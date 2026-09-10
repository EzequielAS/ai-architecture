package tsparser

func (p *parser) skipTypeParams() {
	if !p.is("<") {
		return
	}
	depth := 0
	for p.cur.Type != TEOF {
		switch {
		case p.is("<"):
			depth++
		case p.is(">"):
			depth--
		case p.is(">>"):
			depth -= 2
		case p.is(">>>"):
			depth -= 3
		}
		p.advance()
		if depth <= 0 {
			return
		}
	}
}

// skipType consumes a type expression, attaching KIndexSignature nodes it finds
// (in `{ [k: string]: T }`) to owner for later descendant queries.
// typeContinuationTokens are the tokens after which the type is still incomplete,
// so a following `{` opens a type literal instead of a function body.
var typeContinuationTokens = map[string]bool{
	"|": true, "&": true, ",": true, "=>": true, "extends": true,
	"keyof": true, "readonly": true, "infer": true, "in": true, "?": true, ":": true,
}

func (p *parser) skipType(owner *Node) { p.skipTypeUntil(owner, false) }

// skipReturnType skips a `: T` annotation that a function body or an arrow follows,
// so a `{` or `=>` closing a complete type ends the annotation instead of being
// swallowed by it.
func (p *parser) skipReturnType(owner *Node) { p.skipTypeUntil(owner, true) }

func (p *parser) skipTypeUntil(owner *Node, stopBeforeBody bool) {
	depthAngle := 0
	complete := false
	for p.cur.Type != TEOF {
		switch {
		case p.is("{"):
			if stopBeforeBody && depthAngle == 0 && complete {
				return
			}
			p.parseTypeLiteral(owner)
			complete = true
			continue
		case p.is("("):
			p.skipParens()
			complete = true
			continue
		case p.is("["):
			p.skipBrackets()
			complete = true
			continue
		case p.is("<"):
			depthAngle++
		case p.is(">"):
			if depthAngle > 0 {
				depthAngle--
			} else {
				return
			}
		case stopBeforeBody && depthAngle == 0 && p.is("=>"):
			return
		case p.is(">>"):
			depthAngle -= 2
		case depthAngle == 0 && (p.is("=") || p.is(";") || p.is(")") || p.is("]") || p.is("}") || p.is(",")):
			return
		case depthAngle == 0 && p.cur.NlBefore && p.typeAtStatementBoundary():
			return
		}
		complete = !typeContinuationTokens[p.cur.Value]
		p.advance()
	}
}

// typeAtStatementBoundary guards against a type run swallowing the next statement.
func (p *parser) typeAtStatementBoundary() bool {
	if p.cur.Type == TKeyword {
		switch p.cur.Value {
		case "const", "let", "var", "function", "class", "interface", "type",
			"enum", "import", "export", "return", "if", "for", "while", "switch":
			return true
		}
	}
	return false
}

// parseTypeLiteral scans a `{ ... }` type body for index signatures.
func (p *parser) parseTypeLiteral(owner *Node) {
	p.advance() // {
	for p.cur.Type != TEOF && !p.is("}") {
		switch {
		case p.is("["):
			p.parseIndexSignature(owner)
		case p.is("{"):
			p.parseTypeLiteral(owner)
		case p.is("("):
			p.skipParens()
		default:
			p.advance()
		}
	}
	p.eat("}")
}

// parseIndexSignature records a `[ ident : keyType ] : valueType` member.
func (p *parser) parseIndexSignature(owner *Node) {
	line := p.cur.Line
	save := p.cur.Start
	p.advance() // [
	if p.cur.Type == TIdent && p.nxt.Value == ":" {
		p.advance() // key name
		p.advance() // :
		for p.cur.Type != TEOF && !p.is("]") {
			p.advance()
		}
		p.eat("]")
		if p.is(":") {
			owner.add(newNode(KIndexSignature, line))
		}
		return
	}
	p.resetTo(save)
	p.skipBrackets()
}

func (p *parser) skipBraces()   { p.skipBalanced("{", "}") }
func (p *parser) skipParens()   { p.skipBalanced("(", ")") }
func (p *parser) skipBrackets() { p.skipBalanced("[", "]") }

func (p *parser) skipBalanced(open, close string) {
	if !p.is(open) {
		return
	}
	depth := 0
	for p.cur.Type != TEOF {
		if p.is(open) {
			depth++
		} else if p.is(close) {
			depth--
		}
		p.advance()
		if depth == 0 {
			return
		}
	}
}
