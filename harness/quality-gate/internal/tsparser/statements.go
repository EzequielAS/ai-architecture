package tsparser

func (p *parser) parseStatement(parent *Node) {
	switch {
	case p.cur.Type == TKeyword:
		switch p.cur.Value {
		case "import":
			p.parseImport(parent)
			return
		case "export":
			p.advance()
			p.eat("default")
			p.parseStatement(parent)
			return
		case "const", "let", "var":
			p.parseVariableStatement(parent)
			return
		case "if":
			p.parseIf(parent)
			return
		case "for":
			p.parseFor(parent)
			return
		case "while":
			p.parseWhile(parent)
			return
		case "do":
			p.parseDoWhile(parent)
			return
		case "switch":
			p.parseSwitch(parent)
			return
		case "return", "throw":
			line := p.cur.Line
			p.advance()
			n := newNode(KReturnStatement, line)
			if !p.stmtEnd() {
				n.Expr = p.parseExpression()
				n.add(n.Expr)
			}
			p.eat(";")
			parent.add(n)
			return
		case "function", "async":
			if fn := p.parseFunctionDeclaration(); fn != nil {
				parent.add(fn)
				return
			}
		case "class":
			parent.add(p.parseClass())
			return
		case "interface":
			parent.add(p.parseInterface())
			return
		case "type":
			p.parseTypeAlias(parent)
			return
		case "enum":
			p.skipEnum()
			return
		case "try":
			p.parseTry(parent)
			return
		case "break", "continue":
			p.advance()
			p.eat(";")
			return
		}
	case p.is("{"):
		parent.add(p.parseBlock())
		return
	case p.is(";"):
		p.advance()
		return
	}
	// Expression statement.
	es := newNode(KExpressionStatement, p.cur.Line)
	es.Expr = p.parseExpression()
	es.add(es.Expr)
	p.eat(";")
	parent.add(es)
}

func (p *parser) parseIf(parent *Node) {
	line := p.cur.Line
	p.advance() // if
	n := newNode(KIfStatement, line)
	if p.eat("(") {
		n.Expr = p.parseExpression()
		n.add(n.Expr)
		p.expectClose(")")
	}
	n.Body = p.parseNestedStatement(n)
	if p.isKw("else") {
		p.advance()
		if p.isKw("if") {
			n.Else = p.wrapStatement(func(par *Node) { p.parseIf(par) })
		} else {
			n.Else = p.parseNestedStatement(n)
		}
		n.add(n.Else)
	}
	parent.add(n)
}

// parseNestedStatement parses one statement, attaching it (unwrapped) to owner.
func (p *parser) parseNestedStatement(owner *Node) *Node {
	holder := newNode(KBlock, p.cur.Line)
	before := p.cur.Start
	p.parseStatement(holder)
	if p.cur.Start == before && p.cur.Type != TEOF {
		p.advance()
	}
	if len(holder.Children) == 1 {
		child := holder.Children[0]
		owner.add(child)
		return child
	}
	owner.add(holder)
	return holder
}

func (p *parser) wrapStatement(fn func(*Node)) *Node {
	holder := newNode(KBlock, p.cur.Line)
	fn(holder)
	if len(holder.Children) == 1 {
		return holder.Children[0]
	}
	return holder
}

func (p *parser) parseFor(parent *Node) {
	line := p.cur.Line
	p.advance() // for
	p.eat("await")
	kind := KForStatement
	if p.eat("(") {
		// Scan the header to classify for / for-in / for-of, keeping brace balance.
		depth := 1
		for p.cur.Type != TEOF && depth > 0 {
			switch {
			case p.is("("):
				depth++
			case p.is(")"):
				depth--
			case depth == 1 && p.isKw("of"):
				kind = KForOfStatement
			case depth == 1 && p.isKw("in") && kind == KForStatement:
				kind = KForInStatement
			}
			if depth == 0 {
				break
			}
			p.advance()
		}
		p.expectClose(")")
	}
	n := newNode(kind, line)
	n.Body = p.parseNestedStatement(n)
	parent.add(n)
}

func (p *parser) parseWhile(parent *Node) {
	line := p.cur.Line
	p.advance()
	n := newNode(KWhileStatement, line)
	if p.eat("(") {
		n.Expr = p.parseExpression()
		n.add(n.Expr)
		p.expectClose(")")
	}
	n.Body = p.parseNestedStatement(n)
	parent.add(n)
}

func (p *parser) parseDoWhile(parent *Node) {
	line := p.cur.Line
	p.advance()
	n := newNode(KDoStatement, line)
	n.Body = p.parseNestedStatement(n)
	if p.isKw("while") {
		p.advance()
		if p.eat("(") {
			n.Expr = p.parseExpression()
			n.add(n.Expr)
			p.expectClose(")")
		}
	}
	p.eat(";")
	parent.add(n)
}

func (p *parser) parseSwitch(parent *Node) {
	line := p.cur.Line
	p.advance()
	n := newNode(KSwitchStatement, line)
	if p.eat("(") {
		n.Expr = p.parseExpression()
		n.add(n.Expr)
		p.expectClose(")")
	}
	if p.eat("{") {
		for p.cur.Type != TEOF && !p.is("}") {
			switch {
			case p.isKw("case"):
				cl := newNode(KCaseClause, p.cur.Line)
				p.advance()
				p.parseExpression()
				p.eat(":")
				p.parseClauseBody(cl)
				n.add(cl)
			case p.isKw("default"):
				cl := newNode(KDefaultClause, p.cur.Line)
				p.advance()
				p.eat(":")
				p.parseClauseBody(cl)
				n.add(cl)
			default:
				p.advance()
			}
		}
		p.eat("}")
	}
	parent.add(n)
}

func (p *parser) parseClauseBody(cl *Node) {
	for p.cur.Type != TEOF && !p.isKw("case") && !p.isKw("default") && !p.is("}") {
		before := p.cur.Start
		p.parseStatement(cl)
		if p.cur.Start == before {
			p.advance()
		}
	}
}

func (p *parser) parseTry(parent *Node) {
	p.advance() // try
	if p.is("{") {
		parent.add(p.parseBlock())
	}
	if p.isKw("catch") {
		line := p.cur.Line
		p.advance()
		cc := newNode(KCatchClause, line)
		if p.eat("(") {
			for p.cur.Type != TEOF && !p.is(")") {
				p.advance()
			}
			p.eat(")")
		}
		if p.is("{") {
			cc.Body = p.parseBlock()
			cc.add(cc.Body)
		}
		parent.add(cc)
	}
	if p.isKw("finally") {
		p.advance()
		if p.is("{") {
			parent.add(p.parseBlock())
		}
	}
}
