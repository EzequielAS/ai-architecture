package tsparser

// parseJSX parses a JSX element/fragment starting at the current `<`, then
// repositions the token lexer just past it.
func (p *parser) parseJSX() *Node {
	start := p.cur.Start
	node, end := p.jsxElementAt(start)
	p.resetTo(end)
	return node
}

func (p *parser) runeAt(i int) rune {
	if i < 0 || i >= len(p.src) {
		return 0
	}
	return p.src[i]
}

func (p *parser) skipWS(i int) int {
	for i < len(p.src) {
		c := p.src[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		break
	}
	return i
}

func isJSXNameChar(c rune) bool {
	return c == '_' || c == '$' || c == '.' || c == ':' || c == '-' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func (p *parser) readJSXName(i int) (string, int) {
	i = p.skipWS(i)
	start := i
	for i < len(p.src) && isJSXNameChar(p.src[i]) {
		i++
	}
	return string(p.src[start:i]), i
}

// jsxElementAt parses one element/fragment at offset i (pointing at `<`).
func (p *parser) jsxElementAt(i int) (*Node, int) {
	line := p.lineAt(i)
	i++ // past '<'
	i = p.skipWS(i)

	if p.runeAt(i) == '>' { // fragment <> ... </>
		frag := newNode(KJsxFragment, line)
		i++
		children, ni := p.jsxChildren(i)
		for _, c := range children {
			frag.add(c)
		}
		return frag, p.skipClosingTag(ni)
	}

	_, i = p.readJSXName(i)
	attrs, i := p.jsxAttributes(i)
	i = p.skipWS(i)

	if p.runeAt(i) == '/' && p.runeAt(i+1) == '>' { // self-closing
		el := newNode(KJsxSelfClosingElement, line)
		el.Elements = attrs
		for _, a := range attrs {
			el.add(a)
		}
		return el, i + 2
	}

	if p.runeAt(i) == '>' {
		i++
		el := newNode(KJsxElement, line)
		opening := newNode(KJsxOpeningElement, line)
		opening.Elements = attrs
		for _, a := range attrs {
			opening.add(a)
		}
		el.add(opening)
		children, ni := p.jsxChildren(i)
		for _, c := range children {
			el.add(c)
		}
		return el, p.skipClosingTag(ni)
	}

	// Malformed — surface the opening element and stop.
	opening := newNode(KJsxOpeningElement, line)
	opening.Elements = attrs
	for _, a := range attrs {
		opening.add(a)
	}
	return opening, i
}

// jsxAttributes reads attributes until `>` or `/>`.
func (p *parser) jsxAttributes(i int) ([]*Node, int) {
	var attrs []*Node
	for {
		i = p.skipWS(i)
		c := p.runeAt(i)
		if c == 0 || c == '>' || (c == '/' && p.runeAt(i+1) == '>') {
			return attrs, i
		}
		if c == '{' { // spread attribute {...x}
			_, ni := p.parseJSXExprAt(i)
			sp := newNode(KJsxSpreadAttribute, p.lineAt(i))
			attrs = append(attrs, sp)
			i = ni
			continue
		}
		nameStart := i
		name, ni := p.readJSXName(i)
		if name == "" {
			return attrs, ni + 1
		}
		i = p.skipWS(ni)
		attr := newNode(KJsxAttribute, p.lineAt(nameStart))
		if p.runeAt(i) == '=' {
			i = p.skipWS(i + 1)
			switch p.runeAt(i) {
			case '{':
				expr, ni := p.parseJSXExprAt(i)
				je := newNode(KJsxExpression, p.lineAt(i))
				je.Expr = expr
				if expr != nil {
					je.add(expr)
				}
				attr.Initializer = je
				attr.add(je)
				i = ni
			case '"', '\'':
				val, ni := p.readJSXString(i)
				lit := newNode(KStringLiteral, p.lineAt(i))
				lit.Text = val
				attr.Initializer = lit
				attr.add(lit)
				i = ni
			}
		}
		attrs = append(attrs, attr)
	}
}

func (p *parser) readJSXString(i int) (string, int) {
	quote := p.src[i]
	start := i
	i++
	for i < len(p.src) && p.src[i] != quote {
		i++
	}
	if i < len(p.src) {
		i++
	}
	return string(p.src[start:i]), i
}

// jsxChildren scans children until the matching closing tag, returning at its `<`.
func (p *parser) jsxChildren(i int) ([]*Node, int) {
	var children []*Node
	for i < len(p.src) {
		for i < len(p.src) && p.src[i] != '<' && p.src[i] != '{' {
			i++
		}
		if i >= len(p.src) {
			break
		}
		if p.src[i] == '{' {
			expr, ni := p.parseJSXExprAt(i)
			container := newNode(KJsxExpression, p.lineAt(i))
			container.Expr = expr
			if expr != nil {
				container.add(expr)
			}
			children = append(children, container)
			i = ni
			continue
		}
		if p.runeAt(i+1) == '/' { // closing tag
			break
		}
		child, ni := p.jsxElementAt(i)
		children = append(children, child)
		i = ni
	}
	return children, i
}

// parseJSXExprAt token-parses a `{ expr }` container at offset i and returns the
// inner expression plus the offset just past the closing `}`.
func (p *parser) parseJSXExprAt(i int) (*Node, int) {
	p.resetTo(i)
	p.advance() // consume '{'
	for p.is("...") {
		p.advance()
	}
	var expr *Node
	if !p.is("}") {
		expr = p.parseAssignment()
	}
	for p.cur.Type != TEOF && !p.is("}") {
		p.advance()
	}
	end := len(p.src)
	if p.is("}") {
		end = p.cur.End
	}
	return expr, end
}

func (p *parser) skipClosingTag(i int) int {
	for i < len(p.src) && p.src[i] != '>' {
		i++
	}
	if i < len(p.src) {
		i++
	}
	return i
}
