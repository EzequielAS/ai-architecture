package tsparser

import "strings"

// TokenType enumerates the lexical categories the parser consumes.
type TokenType int

const (
	TEOF TokenType = iota
	TIdent
	TKeyword
	TNumber
	TString   // '...' or "..."
	TTemplate // `...` (whole span, substitutions included)
	TRegex
	TPunct
)

// Token is a single lexical unit with its source offsets and start line.
type Token struct {
	Type     TokenType
	Value    string
	Line     int
	Start    int  // rune offset of first char
	End      int  // rune offset just past last char
	NlBefore bool // a newline separated this token from the previous one
}

var keywords = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "default": true, "return": true, "function": true,
	"const": true, "let": true, "var": true, "import": true, "export": true,
	"from": true, "new": true, "await": true, "async": true, "yield": true,
	"true": true, "false": true, "null": true, "undefined": true, "typeof": true,
	"instanceof": true, "in": true, "of": true, "class": true, "extends": true,
	"enum": true, "interface": true, "type": true, "catch": true, "try": true,
	"finally": true, "throw": true, "break": true, "continue": true, "void": true,
	"delete": true, "this": true, "super": true, "as": true, "readonly": true,
}

// Lexer turns TS/JS/JSX source into tokens; it is repositionable so the parser
// can hand raw offsets back for JSX scanning.
type Lexer struct {
	src       []rune
	pos       int
	line      int
	prevType  TokenType // prevType/prevValue drive the regex-vs-division heuristic
	prevValue string
}

// NewLexer builds a lexer over the given source.
func NewLexer(src string) *Lexer {
	return &Lexer{src: []rune(src), pos: 0, line: 1}
}

// Reset moves the lexer to an absolute rune offset, recomputing the line.
func (l *Lexer) Reset(pos int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(l.src) {
		pos = len(l.src)
	}
	line := 1
	for i := 0; i < pos; i++ {
		if l.src[i] == '\n' {
			line++
		}
	}
	l.pos = pos
	l.line = line
	l.prevType = TEOF
	l.prevValue = ""
}

func (l *Lexer) peekRune(off int) rune {
	i := l.pos + off
	if i < 0 || i >= len(l.src) {
		return 0
	}
	return l.src[i]
}

// skipTrivia consumes whitespace and comments, reporting whether a newline passed.
func (l *Lexer) skipTrivia() bool {
	nl := false
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			nl = true
			l.line++
			l.pos++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.pos++
		case c == '/' && l.peekRune(1) == '/':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '/' && l.peekRune(1) == '*':
			l.pos += 2
			for l.pos < len(l.src) && !(l.src[l.pos] == '*' && l.peekRune(1) == '/') {
				if l.src[l.pos] == '\n' {
					l.line++
				}
				l.pos++
			}
			l.pos += 2
		default:
			return nl
		}
	}
	return nl
}

func isIdentStart(c rune) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c > 127
}
func isIdentPart(c rune) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
func isDigit(c rune) bool { return c >= '0' && c <= '9' }

// regexAllowed decides, from the previous token, whether `/` starts a regex.
func (l *Lexer) regexAllowed() bool {
	switch l.prevType {
	case TIdent, TNumber, TString, TTemplate, TRegex:
		return false
	case TKeyword:
		// After value keywords a `/` is division; after operator keywords it's regex.
		switch l.prevValue {
		case "this", "super", "true", "false", "null", "undefined":
			return false
		}
		return true
	case TPunct:
		switch l.prevValue {
		case ")", "]", "}":
			return false
		}
		return true
	}
	return true
}

// Next returns the next token, advancing the lexer.
func (l *Lexer) Next() Token {
	nl := l.skipTrivia()
	start := l.pos
	line := l.line
	if l.pos >= len(l.src) {
		return l.emit(Token{Type: TEOF, Line: line, Start: start, End: start, NlBefore: nl})
	}
	c := l.src[l.pos]

	switch {
	case isIdentStart(c):
		for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
			l.pos++
		}
		val := string(l.src[start:l.pos])
		t := TIdent
		if keywords[val] {
			t = TKeyword
		}
		return l.emit(Token{Type: t, Value: val, Line: line, Start: start, End: l.pos, NlBefore: nl})

	case isDigit(c) || (c == '.' && isDigit(l.peekRune(1))):
		for l.pos < len(l.src) {
			d := l.src[l.pos]
			if isIdentPart(d) || d == '.' || ((d == '+' || d == '-') && (l.src[l.pos-1] == 'e' || l.src[l.pos-1] == 'E')) {
				l.pos++
				continue
			}
			break
		}
		return l.emit(Token{Type: TNumber, Value: string(l.src[start:l.pos]), Line: line, Start: start, End: l.pos, NlBefore: nl})

	case c == '\'' || c == '"':
		l.scanString(c)
		return l.emit(Token{Type: TString, Value: string(l.src[start:l.pos]), Line: line, Start: start, End: l.pos, NlBefore: nl})

	case c == '`':
		l.scanTemplate()
		return l.emit(Token{Type: TTemplate, Value: string(l.src[start:l.pos]), Line: line, Start: start, End: l.pos, NlBefore: nl})

	case c == '/' && l.regexAllowed():
		if l.scanRegex() {
			return l.emit(Token{Type: TRegex, Value: string(l.src[start:l.pos]), Line: line, Start: start, End: l.pos, NlBefore: nl})
		}
		l.pos = start // not a regex — fall through to punctuation
	}

	return l.emit(l.scanPunct(start, line, nl))
}

func (l *Lexer) scanString(quote rune) {
	l.pos++ // opening quote
	for l.pos < len(l.src) {
		d := l.src[l.pos]
		if d == '\\' {
			l.pos += 2
			continue
		}
		if d == quote || d == '\n' {
			break
		}
		l.pos++
	}
	if l.pos < len(l.src) && l.src[l.pos] == quote {
		l.pos++
	}
}

func (l *Lexer) scanTemplate() {
	l.pos++ // opening backtick
	depth := 0
	for l.pos < len(l.src) {
		d := l.src[l.pos]
		switch {
		case d == '\\':
			l.pos += 2
			continue
		case d == '\n':
			l.line++
			l.pos++
		case d == '$' && l.peekRune(1) == '{':
			depth++
			l.pos += 2
		case d == '}' && depth > 0:
			depth--
			l.pos++
		case d == '`' && depth == 0:
			l.pos++
			return
		default:
			l.pos++
		}
	}
}

// scanRegex tries to read a /regex/flags literal; returns false if it isn't one.
func (l *Lexer) scanRegex() bool {
	save := l.pos
	l.pos++ // opening slash
	inClass := false
	for l.pos < len(l.src) {
		d := l.src[l.pos]
		if d == '\\' {
			l.pos += 2
			continue
		}
		if d == '\n' {
			l.pos = save
			return false
		}
		if d == '[' {
			inClass = true
		} else if d == ']' {
			inClass = false
		} else if d == '/' && !inClass {
			l.pos++
			for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
				l.pos++
			}
			return true
		}
		l.pos++
	}
	l.pos = save
	return false
}

var multiPunct = []string{
	">>>=", "===", "!==", "**=", "<<=", ">>=", ">>>", "...", "&&=", "||=", "??=",
	"==", "!=", "<=", ">=", "&&", "||", "??", "?.", "=>", "**", "++", "--",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>",
}

func (l *Lexer) scanPunct(start, line int, nl bool) Token {
	rest := string(l.src[l.pos:minInt(l.pos+4, len(l.src))])
	for _, p := range multiPunct {
		if strings.HasPrefix(rest, p) {
			l.pos += len([]rune(p))
			return Token{Type: TPunct, Value: p, Line: line, Start: start, End: l.pos, NlBefore: nl}
		}
	}
	l.pos++
	return Token{Type: TPunct, Value: string(l.src[start:l.pos]), Line: line, Start: start, End: l.pos, NlBefore: nl}
}

func (l *Lexer) emit(t Token) Token {
	if t.Type != TEOF {
		l.prevType = t.Type
		l.prevValue = t.Value
	}
	return t
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
