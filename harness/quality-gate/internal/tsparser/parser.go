package tsparser

// Parse builds a tolerant AST for TS/JS/JSX source. It never fails: unrecognized
// syntax is skipped so the scanners can still read the parts they understand.
func Parse(src string) *Node {
	p := &parser{lex: NewLexer(src), src: []rune(src)}
	p.advance()
	p.advance()
	sf := newNode(KSourceFile, 1)
	for p.cur.Type != TEOF {
		before := p.cur.Start
		p.parseStatement(sf)
		if p.cur.Start == before && p.cur.Type != TEOF {
			p.advance() // guarantee forward progress
		}
	}
	return sf
}

type parser struct {
	lex     *Lexer
	src     []rune
	cur     Token
	nxt     Token
	prevEnd int // End offset of the most recently consumed token
}

func (p *parser) advance() {
	if p.cur.Type != TEOF {
		p.prevEnd = p.cur.End
	}
	p.cur = p.nxt
	p.nxt = p.lex.Next()
}

func (p *parser) resetTo(offset int) {
	p.lex.Reset(offset)
	p.cur = p.lex.Next()
	p.nxt = p.lex.Next()
}

func (p *parser) is(v string) bool   { return p.cur.Value == v }
func (p *parser) isKw(v string) bool { return p.cur.Type == TKeyword && p.cur.Value == v }

// eat consumes the current token if it matches v.
func (p *parser) eat(v string) bool {
	if p.cur.Value == v {
		p.advance()
		return true
	}
	return false
}

func (p *parser) expectClose(v string) {
	if p.is(v) {
		p.advance()
	}
}

func (p *parser) stmtEnd() bool {
	return p.cur.Type == TEOF || p.is(";") || p.is("}") || p.cur.NlBefore
}

func (p *parser) slice(start, end int) string {
	if start < 0 || end > len(p.src) || start >= end {
		return ""
	}
	return string(p.src[start:end])
}

func (p *parser) lineAt(offset int) int {
	line := 1
	for i := 0; i < offset && i < len(p.src); i++ {
		if p.src[i] == '\n' {
			line++
		}
	}
	return line
}

func (p *parser) parseBlock() *Node {
	line := p.cur.Line
	p.advance() // {
	b := newNode(KBlock, line)
	for p.cur.Type != TEOF && !p.is("}") {
		before := p.cur.Start
		p.parseStatement(b)
		if p.cur.Start == before && !p.is("}") {
			p.advance()
		}
	}
	p.eat("}")
	return b
}

func (p *parser) parseIdentifier() *Node {
	n := newNode(KIdentifier, p.cur.Line)
	n.Name = p.cur.Value
	n.Text = p.cur.Value
	p.advance()
	return n
}

func unquote(s string) string {
	if len(s) >= 2 {
		q := s[0]
		if (q == '\'' || q == '"' || q == '`') && s[len(s)-1] == q {
			return s[1 : len(s)-1]
		}
	}
	return s
}
