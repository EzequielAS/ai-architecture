package tsparser

func (p *parser) parseImport(parent *Node) {
	line := p.cur.Line
	p.advance() // import
	spec := ""
	for p.cur.Type != TEOF && !p.is(";") && !p.cur.NlBefore {
		if p.cur.Type == TString {
			spec = unquote(p.cur.Value)
		}
		p.advance()
	}
	p.eat(";")
	n := newNode(KImportDeclaration, line)
	n.Name = spec // module specifier value
	parent.add(n)
}

func (p *parser) parseVariableStatement(parent *Node) {
	line := p.cur.Line
	p.advance() // const/let/var
	vs := newNode(KVariableStatement, line)
	for {
		vs.add(p.parseVariableDeclaration())
		if !p.eat(",") {
			break
		}
	}
	p.eat(";")
	parent.add(vs)
}

func (p *parser) parseVariableDeclaration() *Node {
	line := p.cur.Line
	start := p.cur.Start
	d := newNode(KVariableDeclaration, line)
	d.NameNode = p.parseBindingName(d)
	d.add(d.NameNode)
	p.eat("!") // definite-assignment
	if p.is(":") {
		p.advance()
		p.skipType(d)
	}
	if p.eat("=") {
		d.Initializer = p.parseAssignment()
		d.add(d.Initializer)
	}
	d.Text = p.slice(start, p.prevEnd)
	return d
}

func (p *parser) parseBindingName(owner *Node) *Node {
	switch {
	case p.is("["):
		line := p.cur.Line
		p.advance()
		bp := newNode(KArrayBindingPattern, line)
		for p.cur.Type != TEOF && !p.is("]") {
			if p.is(",") {
				p.advance()
				continue
			}
			el := p.parseBindingElement()
			bp.Elements = append(bp.Elements, el)
			bp.add(el)
			if !p.is("]") {
				p.eat(",")
			}
		}
		p.eat("]")
		return bp
	case p.is("{"):
		line := p.cur.Line
		bp := newNode(KBindingElement, line) // object binding, treated opaquely
		p.skipBraces()
		return bp
	default:
		return p.parseIdentifier()
	}
}

func (p *parser) parseBindingElement() *Node {
	p.eat("...")
	el := p.parseBindingName(nil)
	if p.eat(":") { // aliased destructure: { a: b }
		el = p.parseBindingName(nil)
	}
	if p.eat("=") {
		p.parseAssignment() // default value
	}
	return el
}

func (p *parser) parseFunctionDeclaration() *Node {
	line := p.cur.Line
	start := p.cur.Start
	p.eat("async")
	if !p.isKw("function") {
		p.resetTo(start) // `async` not followed by function → expression; rewind
		return nil
	}
	p.advance() // function
	p.eat("*")
	fn := newNode(KFunctionDeclaration, line)
	if p.cur.Type == TIdent {
		fn.Name = p.cur.Value
		p.advance()
	}
	p.skipTypeParams()
	fn.Params = p.parseParams(fn)
	if p.is(":") {
		p.advance()
		p.skipReturnType(fn)
	}
	if p.is("{") {
		fn.Body = p.parseBlock()
		fn.add(fn.Body)
	}
	return fn
}

func (p *parser) parseParams(owner *Node) []*Node {
	var params []*Node
	if !p.eat("(") {
		return params
	}
	for p.cur.Type != TEOF && !p.is(")") {
		if p.is(",") {
			p.advance()
			continue
		}
		params = append(params, p.parseParam(owner))
		if !p.is(")") {
			p.eat(",")
		}
	}
	p.expectClose(")")
	return params
}

func (p *parser) parseParam(owner *Node) *Node {
	pr := newNode(KParameter, p.cur.Line)
	for p.is("@") || p.isKw("public") || p.isKw("private") || p.isKw("protected") || p.isKw("readonly") {
		p.advance()
	}
	p.eat("...")
	pr.NameNode = p.parseBindingName(pr)
	if pr.NameNode != nil {
		pr.Name = pr.NameNode.Name
	}
	p.eat("?")
	if p.is(":") {
		p.advance()
		p.skipType(pr)
	}
	if p.eat("=") {
		p.parseAssignment()
	}
	if owner != nil {
		owner.add(pr)
	}
	return pr
}

func (p *parser) parseClass() *Node {
	line := p.cur.Line
	p.advance() // class
	c := newNode(KUnknown, line)
	if p.cur.Type == TIdent {
		p.advance()
	}
	p.skipTypeParams()
	// `implements` is a contextual keyword, so it arrives as an identifier.
	for p.isKw("extends") || p.is("implements") {
		p.advance()
		p.parseLeftHandSide() // base expr
		for p.eat(",") {
			p.parseLeftHandSide()
		}
	}
	if p.is("{") {
		p.parseClassBody(c)
	}
	return c
}

// parseClassBody scans members loosely, capturing method bodies and property
// initializers so functions/effects inside them still reach the AST.
func (p *parser) parseClassBody(c *Node) {
	p.advance() // {
	for p.cur.Type != TEOF && !p.is("}") {
		switch {
		case p.is(";"):
			p.advance()
		case p.is("("):
			m := newNode(KMethodDeclaration, p.cur.Line)
			m.Params = p.parseParams(m)
			if p.is(":") {
				p.advance()
				p.skipReturnType(m)
			}
			if p.is("{") {
				m.Body = p.parseBlock()
				m.add(m.Body)
			}
			c.add(m)
		case p.is("="):
			p.advance()
			c.add(p.parseAssignment())
			p.eat(";")
		case p.is("{"):
			c.add(p.parseBlock())
		default:
			p.advance()
		}
	}
	p.eat("}")
}

func (p *parser) parseInterface() *Node {
	line := p.cur.Line
	p.advance() // interface
	n := newNode(KUnknown, line)
	if p.cur.Type == TIdent {
		p.advance()
	}
	p.skipTypeParams()
	for p.isKw("extends") {
		p.advance()
		p.skipType(n)
		for p.eat(",") {
			p.skipType(n)
		}
	}
	if p.is("{") {
		p.parseTypeLiteral(n)
	}
	return n
}

func (p *parser) parseTypeAlias(parent *Node) {
	line := p.cur.Line
	p.advance() // type
	n := newNode(KUnknown, line)
	if p.cur.Type == TIdent {
		p.advance()
	}
	p.skipTypeParams()
	if p.eat("=") {
		p.skipType(n)
	}
	p.eat(";")
	parent.add(n)
}

func (p *parser) skipEnum() {
	p.advance() // enum
	if p.cur.Type == TIdent {
		p.advance()
	}
	if p.is("{") {
		p.skipBraces()
	}
}
