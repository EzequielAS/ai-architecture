package tsparser

func (p *parser) parseExpression() *Node { return p.parseAssignment() }

var assignOps = map[string]bool{
	"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true,
	"**=": true, "&&=": true, "||=": true, "??=": true, "&=": true, "|=": true,
	"^=": true, "<<=": true, ">>=": true, ">>>=": true,
}

func (p *parser) parseAssignment() *Node {
	if arrow := p.tryArrow(); arrow != nil {
		return arrow
	}
	start := p.cur.Start
	left := p.parseBinary(0)

	if p.is("?") { // ternary (the `?.` optional-chain token is distinct)
		p.advance()
		cond := newNode(KConditionalExpression, left.Line)
		cond.Expr = left
		whenTrue := p.parseAssignment()
		p.eat(":")
		whenFalse := p.parseAssignment()
		cond.add(left)
		cond.add(whenTrue)
		cond.add(whenFalse)
		cond.Text = p.slice(start, p.prevEnd)
		return cond
	}

	if p.cur.Type == TPunct && assignOps[p.cur.Value] {
		p.advance()
		right := p.parseAssignment()
		n := newNode(KBinaryExpression, left.Line)
		n.add(left)
		n.add(right)
		n.Text = p.slice(start, p.prevEnd)
		return n
	}
	return left
}

// binaryPrec is the binding power of each binary operator (higher binds tighter).
var binaryPrec = map[string]int{
	"||": 3, "??": 3,
	"&&": 4,
	"|":  5, "^": 6, "&": 7,
	"==": 8, "!=": 8, "===": 8, "!==": 8,
	"<": 9, ">": 9, "<=": 9, ">=": 9, "instanceof": 9, "in": 9,
	"<<": 10, ">>": 10, ">>>": 10,
	"+": 11, "-": 11,
	"*": 12, "/": 12, "%": 12,
	"**": 13,
}

func (p *parser) parseBinary(minPrec int) *Node {
	start := p.cur.Start
	left := p.parseUnary()
	for {
		op := p.cur.Value
		if p.isKw("as") { // `expr as Type` — drop the type, keep the value
			p.advance()
			p.skipType(nil)
			continue
		}
		prec, ok := binaryPrec[op]
		if !ok || prec < minPrec {
			break
		}
		if (op == "<" || op == ">") && p.cur.Type != TPunct {
			break
		}
		p.advance()
		nextMin := prec + 1
		if op == "**" {
			nextMin = prec // right-associative
		}
		right := p.parseBinary(nextMin)
		n := newNode(KBinaryExpression, left.Line)
		n.add(left)
		n.add(right)
		n.Text = p.slice(start, p.prevEnd)
		left = n
	}
	return left
}

var prefixOps = map[string]bool{
	"!": true, "~": true, "+": true, "-": true, "++": true, "--": true,
}

func (p *parser) parseUnary() *Node {
	start := p.cur.Start
	if p.cur.Type == TKeyword {
		switch p.cur.Value {
		case "await":
			p.advance()
			n := newNode(KAwaitExpression, p.lineAt(start))
			n.Expr = p.parseUnary()
			n.add(n.Expr)
			n.Text = p.slice(start, p.prevEnd)
			return n
		case "typeof", "void", "delete", "yield":
			line := p.cur.Line
			p.advance()
			n := newNode(KPrefixUnaryExpression, line)
			n.Expr = p.parseUnary()
			n.add(n.Expr)
			n.Text = p.slice(start, p.prevEnd)
			return n
		}
	}
	if p.cur.Type == TPunct && prefixOps[p.cur.Value] {
		line := p.cur.Line
		p.advance()
		n := newNode(KPrefixUnaryExpression, line)
		n.Expr = p.parseUnary()
		n.add(n.Expr)
		n.Text = p.slice(start, p.prevEnd)
		return n
	}
	return p.parseLeftHandSide()
}

// tryArrow speculatively matches an arrow function, rewinding on a miss.
func (p *parser) tryArrow() *Node {
	start := p.cur.Start
	async := false
	if p.isKw("async") && (p.nxt.Type == TIdent || p.nxt.Value == "(" || p.nxt.Value == "<") && !p.nxt.NlBefore {
		async = true
		p.advance()
	}

	if p.cur.Type == TIdent && p.nxt.Value == "=>" {
		line := p.cur.Line
		param := newNode(KParameter, line)
		param.Name = p.cur.Value
		p.advance()
		return p.finishArrow(start, line, []*Node{param})
	}

	if p.is("<") { // generic arrow: <T>(...) => ...
		save := p.cur.Start
		p.skipTypeParams()
		if !p.is("(") {
			p.resetTo(save)
			p.rewindAsync(async, start)
			return nil
		}
	}

	if p.is("(") && p.parensFollowedByArrow() {
		line := p.cur.Line
		arrow := newNode(KArrowFunction, line)
		arrow.Params = p.parseParams(arrow)
		if p.is(":") {
			p.advance()
			p.skipReturnType(arrow)
		}
		return p.finishArrow(start, line, arrow.Params)
	}

	p.rewindAsync(async, start)
	return nil
}

func (p *parser) rewindAsync(async bool, start int) {
	if async {
		p.resetTo(start)
	}
}

// parensFollowedByArrow scans a balanced `(...)` to see if `=>` follows, directly
// or past a return type annotation.
func (p *parser) parensFollowedByArrow() bool {
	save := p.cur.Start
	p.advance() // (
	depth := 1
	for p.cur.Type != TEOF && depth > 0 {
		switch {
		case p.is("("):
			depth++
		case p.is(")"):
			depth--
		}
		p.advance()
	}
	isArrow := p.is("=>") || (p.is(":") && p.returnTypeFollowedByArrow())
	p.resetTo(save)
	return isArrow
}

// returnTypeFollowedByArrow scans a `: Type` annotation for the `=>` that makes the
// preceding parens an arrow's parameter list. Without it the `:` belongs to a
// ternary — `cond ? (value) : other` — whose parens are just a grouping.
func (p *parser) returnTypeFollowedByArrow() bool {
	p.advance() // :
	depth := 0
	for p.cur.Type != TEOF {
		switch {
		case p.is("("), p.is("["), p.is("{"), p.is("<"):
			depth++
		case p.is(")"), p.is("]"), p.is("}"), p.is(">"):
			if depth == 0 {
				return false
			}
			depth--
		case p.is(">>"):
			depth -= 2
		case depth <= 0 && p.is("=>"):
			return true
		case depth <= 0 && (p.is(";") || p.is(",") || p.is("=")):
			return false
		case depth <= 0 && p.cur.NlBefore && p.typeAtStatementBoundary():
			return false
		}
		p.advance()
	}
	return false
}

func (p *parser) finishArrow(start, line int, params []*Node) *Node {
	arrow := newNode(KArrowFunction, line)
	arrow.Params = params
	for _, pr := range params {
		arrow.add(pr)
	}
	p.eat("=>")
	if p.is("{") {
		arrow.Body = p.parseBlock()
	} else {
		arrow.Body = p.parseAssignment()
	}
	arrow.add(arrow.Body)
	arrow.Text = p.slice(start, p.prevEnd)
	return arrow
}
