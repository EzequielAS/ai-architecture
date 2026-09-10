package scan

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"quality-gate/internal/tsparser"
)

// Rules follow https://react.dev/learn/you-might-not-need-an-effect. Only the
// objectively detectable cases are implemented; every rule is deliberately
// narrow, because a false positive costs more than a missed finding.
var (
	setterName   = regexp.MustCompile(`^set[A-Z]`)
	parentCbName = regexp.MustCompile(`^on[A-Z]`)

	// Timers match ^set[A-Z] but are not state setters.
	notSetters = map[string]bool{"setTimeout": true, "setInterval": true, "setImmediate": true}

	subscribeMethods = map[string]bool{"addEventListener": true, "subscribe": true}

	// Events that expose ambient browser state, i.e. an external store read.
	// Deliberately excludes UI events (click, resize, keydown), for which an
	// Effect is the right tool.
	storeEvents = map[string]bool{
		"online": true, "offline": true, "storage": true, "hashchange": true,
		"popstate": true, "visibilitychange": true, "languagechange": true,
	}

	// Calls that mean "this Effect fetches data".
	fetchNames = map[string]bool{
		"fetch": true, "axios": true, "ky": true, "got": true,
		"request": true, "superagent": true,
	}

	// Setter arguments that are constants, i.e. a reset rather than a derivation.
	constantKinds = map[tsparser.Kind]bool{
		tsparser.KNullKeyword: true, tsparser.KNumericLiteral: true,
		tsparser.KStringLiteral: true, tsparser.KNoSubstitutionTemplateLiteral: true,
		tsparser.KArrayLiteralExpression: true, tsparser.KObjectLiteralExpression: true,
		tsparser.KTrueKeyword: true, tsparser.KFalseKeyword: true,
	}
)

type effect struct {
	line      int
	callback  *tsparser.Node
	body      *tsparser.Node
	deps      []string       // dependency expressions, as written
	component *tsparser.Node // enclosing function, to scope cross-effect rules
}

// Effects reports useEffect anti-patterns in each file.
func Effects(files []string) []Finding {
	var findings []Finding
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, f := range analyzeEffects(string(data)) {
			f.File = file
			findings = append(findings, f)
		}
	}
	return findings
}

func analyzeEffects(src string) []Finding {
	sf := tsparser.Parse(src)
	effects := collectEffects(sf)
	if len(effects) == 0 {
		return nil
	}

	var findings []Finding
	report := func(rule string, line int, message string) {
		findings = append(findings, Finding{Line: line, Rule: rule, Message: message})
	}

	for _, e := range effects {
		checkEffect(e, report)
	}
	checkChains(effects, setterToState(sf), report)
	return findings
}

// checkEffect emits at most one finding per Effect: the buckets are ordered so
// that a legitimate use (subscription, fetch with cleanup) stops the analysis.
func checkEffect(e effect, report func(rule string, line int, message string)) {
	writesState := len(setterCalls(e.body)) > 0

	// react.dev — "Subscribing to an external store".
	if sub := subscribeCall(e.body); sub != nil {
		if writesState && isExternalStore(sub) {
			report("effect-external-store", sub.Line,
				"Effect assina estado ambiente do browser/store — use useSyncExternalStore")
		}
		return // a real subscription: no other rule applies
	}

	// react.dev — "Fetching data": a refetch needs cleanup or a late response
	// overwrites a newer one.
	if hasFetch(e.body) {
		if writesState && len(e.deps) > 0 && !hasCleanup(e.callback) {
			report("effect-fetch-no-cleanup", e.line,
				"Effect busca dados e faz setState sem cleanup — retorne uma flag `ignore` (ou aborte) para descartar respostas obsoletas")
		}
		return
	}

	// react.dev — "Notifying parent components about state changes".
	if cb := parentCallbackCall(e.body); cb != nil && len(e.deps) > 0 {
		report("effect-notifies-parent", cb.Line,
			fmt.Sprintf("Effect chama o callback do pai %q — chame-o no event handler que originou a mudança", calleeName(cb)))
		return
	}

	// The remaining rules only apply when the Effect does nothing but setState.
	setters, only := onlySetsState(e.body)
	if !only || len(setters) == 0 || len(e.deps) == 0 || readsRef(e.body) {
		return
	}

	// react.dev — "Resetting all state when a prop changes" / "Adjusting some
	// state when a prop changes".
	if allConstant(setters) {
		report("effect-resets-state", e.line,
			"Effect apenas reseta estado quando uma dependência muda — use a prop `key` para remontar, ou calcule o valor durante a renderização")
		return
	}

	// react.dev — "Updating state based on props or state".
	if usesDeps(setters, baseNames(e.deps)) {
		report("effect-derives-state", e.line,
			"Effect deriva estado das dependências — calcule durante a renderização (useMemo se for caro) em vez de usar um Effect")
	}
}

// checkChains flags an Effect whose dependencies read state another Effect in the
// same component writes — react.dev, "Chains of computations".
func checkChains(effects []effect, states map[string]string, report func(rule string, line int, message string)) {
	for _, consumer := range effects {
		if len(consumer.deps) == 0 || len(setterCalls(consumer.body)) == 0 {
			continue // only a cascade of state updates counts as a chain
		}
		reported := map[string]bool{}
		for _, producer := range effects {
			if producer.line == consumer.line || producer.component != consumer.component {
				continue
			}
			for _, state := range statesWritten(producer.body, states) {
				if reported[state] || !dependsOn(consumer.deps, state) {
					continue
				}
				reported[state] = true
				report("effect-chain", consumer.line,
					fmt.Sprintf("Effect depende de %q, que outro Effect atualiza — cada elo adiciona um render; derive na renderização ou trate tudo no mesmo handler", state))
			}
		}
	}
}

// Effect collection ─────────────────────────────────────────────────────────

func collectEffects(sf *tsparser.Node) []effect {
	var out []effect
	for _, call := range sf.GetDescendantsOfKind(tsparser.KCallExpression) {
		if calleeName(call) != "useEffect" || len(call.Args) == 0 {
			continue
		}
		callback := call.Args[0]
		if !isFunction(callback) || callback.Body == nil {
			continue
		}
		e := effect{
			line:      call.Line,
			callback:  callback,
			body:      callback.Body,
			component: enclosingFunction(call),
		}
		if len(call.Args) > 1 && call.Args[1].Kind == tsparser.KArrayLiteralExpression {
			for _, el := range call.Args[1].Elements {
				e.deps = append(e.deps, el.Text)
			}
		}
		out = append(out, e)
	}
	return out
}

func enclosingFunction(n *tsparser.Node) *tsparser.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if isFunction(p) || p.Kind == tsparser.KFunctionDeclaration || p.Kind == tsparser.KMethodDeclaration {
			return p
		}
	}
	return nil
}

func isFunction(n *tsparser.Node) bool {
	return n != nil && (n.Kind == tsparser.KArrowFunction || n.Kind == tsparser.KFunctionExpression)
}

// Structural analysis ───────────────────────────────────────────────────────

// onlySetsState reports whether the Effect body consists exclusively of state
// setter calls, optionally wrapped in `if` guards. Anything else — a variable,
// another call, an assignment — means the Effect does real work, so the
// derive/reset rules stay silent.
func onlySetsState(body *tsparser.Node) (setters []*tsparser.Node, ok bool) {
	switch body.Kind {
	case tsparser.KBlock:
		for _, stmt := range body.Children {
			found, ok := settersInStatement(stmt)
			if !ok {
				return nil, false
			}
			setters = append(setters, found...)
		}
		return setters, true
	case tsparser.KCallExpression: // arrow with an expression body
		if isSetterCall(body) {
			return []*tsparser.Node{body}, true
		}
	}
	return nil, false
}

func settersInStatement(stmt *tsparser.Node) (setters []*tsparser.Node, ok bool) {
	if stmt == nil {
		return nil, true
	}
	switch stmt.Kind {
	case tsparser.KExpressionStatement:
		if isSetterCall(stmt.Expr) {
			return []*tsparser.Node{stmt.Expr}, true
		}
	case tsparser.KIfStatement:
		if len(callsOrSelf(stmt.Expr)) > 0 {
			return nil, false // a call in the condition is work of its own
		}
		for _, branch := range []*tsparser.Node{stmt.Body, stmt.Else} {
			found, ok := settersInStatement(branch)
			if !ok {
				return nil, false
			}
			setters = append(setters, found...)
		}
		return setters, true
	case tsparser.KBlock:
		for _, child := range stmt.Children {
			found, ok := settersInStatement(child)
			if !ok {
				return nil, false
			}
			setters = append(setters, found...)
		}
		return setters, true
	case tsparser.KReturnStatement:
		if stmt.Expr == nil {
			return nil, true // bare early `return`
		}
	}
	return nil, false
}

func isSetterCall(n *tsparser.Node) bool {
	if n == nil || n.Kind != tsparser.KCallExpression || n.Expr == nil {
		return false
	}
	return n.Expr.Kind == tsparser.KIdentifier && isSetter(n.Expr.Text)
}

func isSetter(name string) bool {
	return !notSetters[name] && setterName.MatchString(name)
}

// setterCalls returns every setter call in the body, nested callbacks included.
func setterCalls(body *tsparser.Node) []*tsparser.Node {
	var out []*tsparser.Node
	for _, call := range callsOrSelf(body) {
		if isSetterCall(call) {
			out = append(out, call)
		}
	}
	return out
}

func allConstant(setters []*tsparser.Node) bool {
	for _, call := range setters {
		if len(call.Args) == 0 {
			continue // setX() — still a reset
		}
		arg := call.Args[0]
		if arg.Kind == tsparser.KIdentifier && arg.Text == "undefined" {
			continue
		}
		if !constantKinds[arg.Kind] {
			return false
		}
	}
	return true
}

// usesDeps reports whether a setter argument actually reads one of the
// dependencies — the signal that the value is derived from render values.
func usesDeps(setters []*tsparser.Node, deps map[string]bool) bool {
	for _, call := range setters {
		for _, arg := range call.Args {
			for _, id := range identifiersOrSelf(arg) {
				if deps[id.Name] {
					return true
				}
			}
		}
	}
	return false
}

// baseNames reduces `props.value` and `items[0]` to `props` and `items`.
func baseNames(deps []string) map[string]bool {
	out := map[string]bool{}
	for _, dep := range deps {
		base := dep
		if i := strings.IndexAny(base, ".[?!"); i > 0 {
			base = base[:i]
		}
		if base = strings.TrimSpace(base); base != "" {
			out[base] = true
		}
	}
	return out
}

func readsRef(body *tsparser.Node) bool {
	for _, acc := range body.GetDescendantsOfKind(tsparser.KPropertyAccessExpression) {
		if acc.Name == "current" {
			return true
		}
	}
	return false
}

// Subscriptions, fetching, parent callbacks ─────────────────────────────────

func subscribeCall(body *tsparser.Node) *tsparser.Node {
	for _, call := range callsOrSelf(body) {
		callee := call.Expr
		if callee != nil && callee.Kind == tsparser.KPropertyAccessExpression && subscribeMethods[callee.Name] {
			return call
		}
	}
	return nil
}

// isExternalStore distinguishes reading a store from reacting to an event.
// addEventListener qualifies only for ambient-state events; `subscribe`
// qualifies only when its callback takes no argument and therefore has to
// re-read the store itself (the useSyncExternalStore shape) — an observable
// that pushes a value into the callback is a different pattern.
func isExternalStore(call *tsparser.Node) bool {
	if call.Expr.Name == "addEventListener" {
		return len(call.Args) > 0 && storeEvents[stringValue(call.Args[0])]
	}
	if len(call.Args) == 0 {
		return false
	}
	cb := call.Args[0]
	return isFunction(cb) && len(cb.Params) == 0 && cb.Body != nil && len(setterCalls(cb.Body)) > 0
}

func hasFetch(body *tsparser.Node) bool {
	if len(body.GetDescendantsOfKind(tsparser.KAwaitExpression)) > 0 {
		return true
	}
	for _, call := range callsOrSelf(body) {
		callee := call.Expr
		if callee == nil {
			continue
		}
		if callee.Kind == tsparser.KPropertyAccessExpression && callee.Name == "then" {
			return true
		}
		if callee.Kind == tsparser.KIdentifier && fetchNames[callee.Text] {
			return true
		}
	}
	return false
}

// hasCleanup reports whether the Effect returns a teardown function. An
// expression-bodied arrow is assumed to, since its value is unknown here.
func hasCleanup(callback *tsparser.Node) bool {
	body := callback.Body
	if body == nil || body.Kind != tsparser.KBlock {
		return true
	}
	for _, stmt := range body.Children {
		if stmt.Kind == tsparser.KReturnStatement && isFunction(stmt.Expr) {
			return true
		}
	}
	return false
}

// parentCallbackCall returns an `onSomething(...)` call made directly by the
// Effect. Calls nested in another function are handlers, not notifications.
func parentCallbackCall(body *tsparser.Node) *tsparser.Node {
	for _, call := range callsOrSelf(body) {
		name := calleeName(call)
		if !parentCbName.MatchString(name) || nestedInFunction(call, body) {
			continue
		}
		return call
	}
	return nil
}

func nestedInFunction(n, body *tsparser.Node) bool {
	for p := n.Parent; p != nil && p != body; p = p.Parent {
		if isFunction(p) {
			return true
		}
	}
	return false
}

// State tracking ────────────────────────────────────────────────────────────

// setterToState maps each useState setter to its state name.
func setterToState(sf *tsparser.Node) map[string]string {
	out := map[string]string{}
	for _, decl := range sf.GetDescendantsOfKind(tsparser.KVariableDeclaration) {
		init := decl.Initializer
		if init == nil || init.Kind != tsparser.KCallExpression || calleeName(init) != "useState" {
			continue
		}
		name := decl.NameNode
		if name == nil || name.Kind != tsparser.KArrayBindingPattern || len(name.Elements) < 2 {
			continue
		}
		state, setter := name.Elements[0].Name, name.Elements[1].Name
		if state != "" && setter != "" {
			out[setter] = state
		}
	}
	return out
}

func statesWritten(body *tsparser.Node, states map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, call := range setterCalls(body) {
		state, ok := states[call.Expr.Text]
		if !ok || seen[state] {
			continue
		}
		seen[state] = true
		out = append(out, state)
	}
	return out
}

func dependsOn(deps []string, state string) bool {
	for _, dep := range deps {
		if dep == state || strings.HasPrefix(dep, state+".") || strings.HasPrefix(dep, state+"[") {
			return true
		}
	}
	return false
}

// Node helpers ──────────────────────────────────────────────────────────────

// calleeName is the called name for `f()` and `obj.f()` alike.
func calleeName(call *tsparser.Node) string {
	callee := call.Expr
	if callee == nil {
		return ""
	}
	switch callee.Kind {
	case tsparser.KIdentifier:
		return callee.Text
	case tsparser.KPropertyAccessExpression:
		return callee.Name
	}
	return ""
}

// callsOrSelf and identifiersOrSelf include the node itself, which matters for
// expression-bodied arrows such as `() => setX(y)`.
func callsOrSelf(n *tsparser.Node) []*tsparser.Node {
	return descendantsOrSelf(n, tsparser.KCallExpression)
}

func identifiersOrSelf(n *tsparser.Node) []*tsparser.Node {
	return descendantsOrSelf(n, tsparser.KIdentifier)
}

func descendantsOrSelf(n *tsparser.Node, kind tsparser.Kind) []*tsparser.Node {
	if n == nil {
		return nil
	}
	var out []*tsparser.Node
	if n.Kind == kind {
		out = append(out, n)
	}
	return append(out, n.GetDescendantsOfKind(kind)...)
}

func stringValue(n *tsparser.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind != tsparser.KStringLiteral && n.Kind != tsparser.KNoSubstitutionTemplateLiteral {
		return ""
	}
	s := n.Text
	if len(s) >= 2 {
		if q := s[0]; (q == '\'' || q == '"' || q == '`') && s[len(s)-1] == q {
			return s[1 : len(s)-1]
		}
	}
	return s
}
