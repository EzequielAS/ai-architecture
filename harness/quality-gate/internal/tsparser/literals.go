package tsparser

import "strings"

func (p *parser) parsePrimary() *Node {
	start := p.cur.Start
	line := p.cur.Line

	switch p.cur.Type {
	case TNumber:
		return p.literal(KNumericLiteral)
	case TString:
		return p.literal(KStringLiteral)
	case TTemplate:
		if strings.Contains(p.cur.Value, "${") {
			return p.literal(KTemplateExpression)
		}
		return p.literal(KNoSubstitutionTemplateLiteral)
	case TRegex:
		return p.literal(KUnknown)
	case TKeyword:
		switch p.cur.Value {
		case "true":
			return p.literal(KTrueKeyword)
		case "false":
			return p.literal(KFalseKeyword)
		case "null":
			return p.literal(KNullKeyword)
		case "function", "async":
			if fn := p.parseFunctionExpression(); fn != nil {
				return fn
			}
			return p.literal(KIdentifier)
		case "class":
			p.advance()
			c := newNode(KUnknown, line)
			if p.cur.Type == TIdent {
				p.advance()
			}
			if p.is("{") {
				p.parseClassBody(c)
			}
			return c
		default:
			return p.literal(KIdentifier) // this / super / undefined / etc.
		}
	case TIdent:
		return p.parseIdentifier()
	case TPunct:
		switch p.cur.Value {
		case "(":
			p.advance()
			inner := p.parseExpression()
			p.expectClose(")")
			n := newNode(KParenthesizedExpression, line)
			n.Expr = inner
			n.add(inner)
			n.Text = p.slice(start, p.prevEnd)
			return n
		case "[":
			return p.parseArrayLiteral()
		case "{":
			return p.parseObjectLiteral()
		case "<":
			return p.parseJSX()
		}
	}
	return p.literal(KUnknown) // unknown token — consume to keep progressing
}

func (p *parser) literal(kind Kind) *Node {
	n := newNode(kind, p.cur.Line)
	n.Text = p.cur.Value
	if kind == KIdentifier {
		n.Name = p.cur.Value
	}
	p.advance()
	return n
}

func (p *parser) parseFunctionExpression() *Node {
	start := p.cur.Start
	line := p.cur.Line
	p.eat("async")
	if !p.isKw("function") {
		p.resetTo(start)
		return nil
	}
	p.advance() // function
	p.eat("*")
	fn := newNode(KFunctionExpression, line)
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
	fn.Text = p.slice(start, p.prevEnd)
	return fn
}

func (p *parser) parseArrayLiteral() *Node {
	start := p.cur.Start
	line := p.cur.Line
	p.advance() // [
	n := newNode(KArrayLiteralExpression, line)
	for p.cur.Type != TEOF && !p.is("]") {
		if p.is(",") {
			p.advance()
			continue
		}
		var el *Node
		if p.is("...") {
			p.advance()
			sp := newNode(KSpreadElement, p.cur.Line)
			sp.Expr = p.parseAssignment()
			sp.add(sp.Expr)
			el = sp
		} else {
			el = p.parseAssignment()
		}
		n.Elements = append(n.Elements, el)
		n.add(el)
		if !p.is("]") {
			p.eat(",")
		}
	}
	p.expectClose("]")
	n.Text = p.slice(start, p.prevEnd)
	return n
}

func (p *parser) parseObjectLiteral() *Node {
	start := p.cur.Start
	line := p.cur.Line
	p.advance() // {
	n := newNode(KObjectLiteralExpression, line)
	for p.cur.Type != TEOF && !p.is("}") {
		if p.is(",") {
			p.advance()
			continue
		}
		if p.is("...") {
			p.advance()
			sp := newNode(KSpreadElement, p.cur.Line)
			sp.Expr = p.parseAssignment()
			sp.add(sp.Expr)
			n.Elements = append(n.Elements, sp)
			n.add(sp)
			p.eat(",")
			continue
		}
		n.add(p.parseObjectMember())
		p.eat(",")
	}
	p.expectClose("}")
	n.Text = p.slice(start, p.prevEnd)
	return n
}

// parseObjectMember reads one property/method/shorthand and returns its value node.
func (p *parser) parseObjectMember() *Node {
	line := p.cur.Line
	// get/set accessor: skip the modifier so the real key parses next.
	if (p.is("get") || p.is("set")) && p.nxt.Type == TIdent {
		p.advance()
	}
	p.eat("async")
	p.eat("*")

	if p.is("[") { // computed key
		p.advance()
		p.parseExpression()
		p.expectClose("]")
	} else {
		p.advance() // literal/identifier key
	}
	p.eat("?")

	pa := newNode(KPropertyAssignment, line)
	switch {
	case p.is("("): // method
		m := newNode(KFunctionExpression, line)
		m.Params = p.parseParams(m)
		if p.is(":") {
			p.advance()
			p.skipReturnType(m)
		}
		if p.is("{") {
			m.Body = p.parseBlock()
			m.add(m.Body)
		}
		pa.Expr = m
		pa.add(m)
	case p.is(":"):
		p.advance()
		pa.Expr = p.parseAssignment()
		pa.add(pa.Expr)
	}
	return pa
}
