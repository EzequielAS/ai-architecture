package tsparser

// parseLeftHandSide parses a primary expression plus any member/call/index tail.
func (p *parser) parseLeftHandSide() *Node {
	start := p.cur.Start

	if p.isKw("new") {
		line := p.cur.Line
		p.advance()
		inner := p.parseLeftHandSide()
		n := newNode(KNewExpression, line)
		n.Expr = inner
		n.add(inner)
		n.Text = p.slice(start, p.prevEnd)
		return n
	}

	node := p.parsePrimary()

	for {
		switch {
		case p.is(".") || p.is("?."):
			optional := p.is("?.")
			p.advance()
			if optional && (p.is("(") || p.is("[")) {
				node = p.wrapAccessTail(start, node, optional)
				continue
			}
			acc := newNode(KPropertyAccessExpression, node.Line)
			acc.Expr = node
			acc.Name = p.cur.Value
			acc.add(node)
			p.advance()
			acc.Text = p.slice(start, p.prevEnd)
			node = acc

		case p.is("["):
			p.advance()
			idx := p.parseExpression()
			p.expectClose("]")
			acc := newNode(KElementAccessExpression, node.Line)
			acc.Expr = node
			acc.add(node)
			if idx != nil {
				acc.add(idx)
			}
			acc.Text = p.slice(start, p.prevEnd)
			node = acc

		case p.is("("):
			call := newNode(KCallExpression, node.Line)
			call.Expr = node
			call.add(node)
			call.Args = p.parseArgs(call)
			call.Text = p.slice(start, p.prevEnd)
			node = call

		case p.is("<"):
			if !p.typeArgsThenCall() {
				return node
			}
			// positioned at "(" — loop continues into the call case

		case p.is("!") && !p.cur.NlBefore:
			p.advance() // non-null assertion

		case (p.is("++") || p.is("--")) && !p.cur.NlBefore:
			p.advance() // postfix increment/decrement

		case p.cur.Type == TTemplate:
			p.advance() // tagged template — keep the callee node

		default:
			return node
		}
	}
}

// wrapAccessTail handles the `?.(` and `?.[` optional forms after the `?.` token.
func (p *parser) wrapAccessTail(start int, node *Node, optional bool) *Node {
	if p.is("(") {
		call := newNode(KCallExpression, node.Line)
		call.Expr = node
		call.add(node)
		call.Args = p.parseArgs(call)
		call.Text = p.slice(start, p.prevEnd)
		return call
	}
	// "["
	p.advance()
	idx := p.parseExpression()
	p.expectClose("]")
	acc := newNode(KElementAccessExpression, node.Line)
	acc.Expr = node
	acc.add(node)
	if idx != nil {
		acc.add(idx)
	}
	acc.Text = p.slice(start, p.prevEnd)
	return acc
}

func (p *parser) parseArgs(call *Node) []*Node {
	var args []*Node
	p.advance() // (
	for p.cur.Type != TEOF && !p.is(")") {
		if p.is(",") {
			p.advance()
			continue
		}
		var arg *Node
		if p.is("...") {
			line := p.cur.Line
			p.advance()
			sp := newNode(KSpreadElement, line)
			sp.Expr = p.parseAssignment()
			sp.add(sp.Expr)
			arg = sp
		} else {
			arg = p.parseAssignment()
		}
		args = append(args, arg)
		call.add(arg)
		if !p.is(")") {
			p.eat(",")
		}
	}
	p.expectClose(")")
	return args
}

// typeArgsThenCall consumes `<...>` type arguments only when a call `(` follows,
// disambiguating `foo<T>(x)` from the `<` comparison operator.
func (p *parser) typeArgsThenCall() bool {
	save := p.cur.Start
	p.advance() // <
	depth := 1
	for p.cur.Type != TEOF && depth > 0 {
		switch {
		case p.is("<"):
			depth++
		case p.is(">"):
			depth--
		case p.is(">>"):
			depth -= 2
		case p.is(">>>"):
			depth -= 3
		case p.is(";") || p.is("{") || p.is("&&") || p.is("||"):
			p.resetTo(save)
			return false
		}
		p.advance()
		if depth <= 0 {
			break
		}
	}
	if depth <= 0 && p.is("(") {
		return true
	}
	p.resetTo(save)
	return false
}
