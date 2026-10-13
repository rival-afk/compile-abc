// Package lexer turns PascalABC source text into tokens.
package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"abc/internal/token"
)

// Directive is a {$...} compiler directive kept as trivia.
type Directive struct {
	Name   string   // upper-case name without '$'
	Params []string // remaining words
	Pos    token.Pos
	PosEnd token.Pos
}

// Result of scanning a source file.
type Result struct {
	File             string
	Tokens           []token.Token
	Directives       []Directive
	ZeroBasedStrings bool
	AppType          string // {$apptype console|window|...}
}

type condState struct {
	name    string // symbol tested by $ifdef/$ifndef
	inElse  bool
	taken   bool // some branch of this conditional has been taken already
	active  bool // currently emitting text
	parentA bool // parent was active
}

type scanner struct {
	file    string
	src     []byte
	off     int
	line    int
	col     int
	defines map[string]bool
	stack   []*condState
	res     *Result
}

// Scan tokenizes the whole file.
func Scan(file string, src []byte) (*Result, error) {
	src = stripBOM(src)
	s := &scanner{
		file:    file,
		src:     src,
		line:    1,
		col:     1,
		defines: map[string]bool{},
		res:     &Result{File: file},
	}
	for {
		tok, err := s.next()
		if err != nil {
			return s.res, err
		}
		s.res.Tokens = append(s.res.Tokens, tok)
		if tok.Kind == token.EOF {
			break
		}
	}
	if len(s.stack) != 0 {
		return s.res, s.errorAt(s.pos(), "unterminated {$%s}", s.stack[len(s.stack)-1].name)
	}
	return s.res, nil
}

func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

func (s *scanner) pos() token.Pos {
	return token.Pos{Line: s.line, Col: s.col, Offset: s.off}
}

func (s *scanner) errorAt(p token.Pos, format string, args ...interface{}) error {
	return fmt.Errorf("%s:%d:%d: %s", s.file, p.Line, p.Col, fmt.Sprintf(format, args...))
}

// excluding reports whether the current position lies inside a disabled
// conditional-compilation region.
func (s *scanner) excluding() bool {
	for _, c := range s.stack {
		if !c.active {
			return true
		}
	}
	return false
}

func (s *scanner) atEOF() bool { return s.off >= len(s.src) }

func (s *scanner) peek(n int) byte {
	if s.off+n < len(s.src) {
		return s.src[s.off+n]
	}
	return 0
}

func (s *scanner) runeAt(off int) (rune, int) {
	if off >= len(s.src) {
		return 0, 0
	}
	r, size := utf8.DecodeRune(s.src[off:])
	return r, size
}

func (s *scanner) advance() {
	if s.atEOF() {
		return
	}
	r, size := utf8.DecodeRune(s.src[s.off:])
	s.off += size
	if r == '\n' {
		s.line++
		s.col = 1
	} else {
		s.col++
	}
}

// advanceN advances n bytes, in whole runes where possible. Callers pass byte
// counts (byte distances or the byte size of the current rune).
func (s *scanner) advanceN(n int) {
	for i := 0; i < n && !s.atEOF(); {
		r, size := utf8.DecodeRune(s.src[s.off:])
		if size <= 0 {
			size = 1
		}
		if i+size > n {
			size = n - i
		}
		s.off += size
		i += size
		if r == '\n' {
			s.line++
			s.col = 1
		} else {
			s.col++
		}
	}
}

// text returns raw source text for [start,end).
func (s *scanner) text(start int) string { return string(s.src[start:s.off]) }

// next returns the next token, skipping comments and directives.
func (s *scanner) next() (token.Token, error) {
	for {
		s.skipSpaceAndComments()
		if s.atEOF() {
			return s.eofToken(), nil
		}
		start := s.pos()
		startOff := s.off

		// Conditional compilation regions are handled while skipping.
		if s.excluding() {
			// Only directives are meaningful here.
			if _, handled, err := s.tryDirective(start, startOff); handled {
				if err != nil {
					return token.Token{}, err
				}
				continue
			}
			s.advance()
			continue
		}

		if _, handled, err := s.tryDirective(start, startOff); handled {
			if err != nil {
				return token.Token{}, err
			}
			continue
		}
		return s.scanToken(start, startOff)
	}
}

func (s *scanner) eofToken() token.Token {
	p := s.pos()
	return token.Token{Kind: token.EOF, Text: "", Start: p, End: p}
}

// skipSpaceAndComments consumes whitespace, comments and non-conditional
// directives.  Conditional directives are processed by tryDirective from next().
func (s *scanner) skipSpaceAndComments() {
	for !s.atEOF() {
		c := s.peek(0)
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\v' || c == '\f':
			s.advance()
		case c == '/' && s.peek(1) == '/':
			for !s.atEOF() && s.peek(0) != '\n' {
				s.advance()
			}
		case c == '(' && s.peek(1) == '*':
			// `(*$directive*)` is handled as a directive below.
			if s.peek(2) == '$' {
				return
			}
			s.advance()
			s.advance()
			for !s.atEOF() {
				if s.peek(0) == '*' && s.peek(1) == ')' {
					s.advance()
					s.advance()
					break
				}
				s.advance()
			}
		case c == '{' && s.peek(1) != '$':
			s.advance()
			for !s.atEOF() && s.peek(0) != '}' {
				s.advance()
			}
			if !s.atEOF() {
				s.advance()
			}
		default:
			return
		}
	}
}

// tryDirective handles `{$...}` and `(*$...*)`.  It reports whether the
// directive was recognised at the current position.
func (s *scanner) tryDirective(start token.Pos, startOff int) (token.Token, bool, error) {
	var bodyStart int
	var closer string
	switch {
	case s.peek(0) == '{' && s.peek(1) == '$':
		bodyStart = s.off + 2
		closer = "}"
	case s.peek(0) == '(' && s.peek(1) == '*' && s.peek(2) == '$':
		bodyStart = s.off + 3
		closer = "*)"
	default:
		return token.Token{}, false, nil
	}
	i := bodyStart
	for i+1 < len(s.src) {
		if closer == "}" && (s.src[i] == '}' || s.src[i] == '\n') {
			break
		}
		if closer == "*)" && s.src[i] == '*' && s.src[i+1] == ')' {
			break
		}
		if closer == "}" && s.src[i] == '\n' {
			break
		}
		i++
	}
	if i >= len(s.src) || (closer == "}" && s.src[i] != '}') && !(closer == "*)" && i+1 < len(s.src) && s.src[i] == '*' && s.src[i+1] == ')') {
		return token.Token{}, true, s.errorAt(start, "unterminated compiler directive")
	}
	body := string(s.src[bodyStart:i])
	end := i + 1
	if closer == "*)" {
		end = i + 2
	}
	s.advanceN(end - s.off)
	endPos := s.pos()

	name, params := splitDirective(body)
	up := strings.ToUpper(name)

	s.res.Directives = append(s.res.Directives, Directive{Name: up, Params: params, Pos: start, PosEnd: endPos})

	if err := s.applyDirective(up, params, start, endPos); err != nil {
		return token.Token{}, true, err
	}
	_ = startOff
	return token.Token{}, true, nil
}

func splitDirective(body string) (string, []string) {
	body = strings.TrimSpace(body)
	name := body
	rest := ""
	for i, r := range body {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			name = body[:i]
			rest = strings.TrimSpace(body[i:])
			break
		}
	}
	var params []string
	if rest != "" {
		params = strings.Fields(rest)
	}
	return name, params
}

func (s *scanner) applyDirective(name string, params []string, start, end token.Pos) error {
	cur := s.stack
	top := func() *condState {
		if len(cur) == 0 {
			return nil
		}
		return cur[len(cur)-1]
	}
	insideDisabled := s.excluding()

	switch name {
	case "DEFINE":
		if insideDisabled {
			return nil
		}
		if len(params) > 0 {
			s.defines[strings.ToLower(params[0])] = true
		}
	case "UNDEF":
		if insideDisabled {
			return nil
		}
		if len(params) > 0 {
			delete(s.defines, strings.ToLower(params[0]))
		}
	case "IFDEF", "IFNDEF":
		sym := ""
		if len(params) > 0 {
			sym = params[0]
		}
		_, defined := s.defines[strings.ToLower(sym)]
		taken := (name == "IFDEF") == defined
		parentActive := !insideDisabled
		st := &condState{name: sym, active: parentActive && taken, parentA: parentActive}
		s.stack = append(s.stack, st)
	case "ELSE":
		st := top()
		if st == nil {
			return s.errorAt(start, "unexpected $else")
		}
		if st.inElse {
			return s.errorAt(start, "duplicate $else")
		}
		st.inElse = true
		st.active = st.parentA && !st.active
	case "ENDIF":
		st := top()
		if st == nil {
			return s.errorAt(start, "unexpected $endif")
		}
		s.stack = s.stack[:len(s.stack)-1]
	case "ZEROBASEDSTRINGS":
		if !insideDisabled {
			v := true
			if len(params) > 0 {
				switch strings.ToLower(params[0]) {
				case "off", "false", "0":
					v = false
				}
			}
			s.res.ZeroBasedStrings = v
		}
	case "APPTYPE":
		if !insideDisabled && len(params) > 0 {
			s.res.AppType = strings.ToLower(params[0])
		}
	}
	return nil
}

// scanToken produces one token for the current position.
func (s *scanner) scanToken(start token.Pos, startOff int) (token.Token, error) {
	c := s.peek(0)

	if s.identStart(s.off) || ((c == '&' || c == '!') && s.identStart(s.off+1)) {
		return s.scanIdent(start, startOff), nil
	}
	if isDigit(c) {
		return s.scanNumber(start, startOff)
	}
	switch c {
	case '$':
		if s.peek(1) == '\'' {
			return s.scanFormatString(start, startOff)
		}
		if isHexDigit(s.peek(1)) {
			return s.scanHex(start, startOff), nil
		}
		return token.Token{}, s.errorAt(start, "unexpected '$'")
	case '\'':
		if s.multilineStringAhead() {
			return s.scanMultiString(start, startOff)
		}
		return s.scanString(start, startOff)
	case '#':
		return s.scanSharp(start, startOff)
	}
	return s.scanOperator(start, startOff)
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
func isHexDigit(b byte) bool {
	return isDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func isIdentStartByte(b byte) bool {
	if b == '_' {
		return true
	}
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// identStart reports whether a token starting at off is an identifier.
func (s *scanner) identStart(off int) bool {
	if off >= len(s.src) {
		return false
	}
	b := s.src[off]
	if b == '_' {
		return true
	}
	if b < 0x80 {
		return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
	}
	r, _ := utf8.DecodeRune(s.src[off:])
	return unicode.IsLetter(r)
}

func (s *scanner) scanIdent(start token.Pos, startOff int) token.Token {
	// optional & or ! escape prefix
	if s.peek(0) == '&' || s.peek(0) == '!' {
		s.advance()
	}
	for !s.atEOF() {
		r, size := s.runeAt(s.off)
		if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			break
		}
		s.advanceN(size)
	}
	txt := s.text(startOff)
	if txt == "_" {
		return token.Token{Kind: token.Underscore, Text: txt, Value: txt, Start: start, End: s.pos()}
	}
	kind := token.Lookup(strings.ToLower(txt))
	return token.Token{Kind: kind, Text: txt, Value: txt, Start: start, End: s.pos()}
}

func identPart(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func (s *scanner) scanNumber(start token.Pos, startOff int) (token.Token, error) {
	kind := token.Int
	scanDigits := func() {
		for !s.atEOF() {
			c := s.peek(0)
			if isDigit(c) || c == '_' {
				s.advance()
				continue
			}
			break
		}
	}
	scanDigits()

	// fractional part
	if s.peek(0) == '.' && s.peek(1) != '.' && (isDigit(s.peek(1)) || isExpStart(s, s.off+1)) {
		s.advance() // '.'
		if isDigit(s.peek(0)) {
			scanDigits()
		}
		kind = token.Float
	}
	// exponent
	if s.peek(0) == 'e' || s.peek(0) == 'E' {
		n := 1
		if s.peek(n) == '+' || s.peek(n) == '-' {
			n++
		}
		if isDigit(s.peek(n)) {
			kind = token.Float
			s.advanceN(n)
			scanDigits()
		}
	}
	// bigint suffix
	if (s.peek(0) == 'b' || s.peek(0) == 'B') && (s.peek(1) == 'i' || s.peek(1) == 'I') {
		s.advanceN(2)
		kind = token.BigInt
	}
	txt := strings.ReplaceAll(s.text(startOff), "_", "")
	return token.Token{Kind: kind, Text: s.text(startOff), Value: txt, Start: start, End: s.pos()}, nil
}

// isExpStart reports whether off points at an exponent marker followed by an
// exponent, so that `1.e5` is a real number.
func isExpStart(s *scanner, off int) bool {
	if off >= len(s.src) {
		return false
	}
	c := s.src[off]
	if c != 'e' && c != 'E' {
		return false
	}
	i := off + 1
	if i < len(s.src) && (s.src[i] == '+' || s.src[i] == '-') {
		i++
	}
	return i < len(s.src) && isDigit(s.src[i])
}

func (s *scanner) scanHex(start token.Pos, startOff int) token.Token {
	s.advance() // $
	for !s.atEOF() && (isHexDigit(s.peek(0)) || s.peek(0) == '_') {
		s.advance()
	}
	txt := strings.ReplaceAll(s.text(startOff)[1:], "_", "")
	return token.Token{Kind: token.Hex, Text: s.text(startOff), Value: txt, Start: start, End: s.pos()}
}

func (s *scanner) scanString(start token.Pos, startOff int) (token.Token, error) {
	s.advance() // opening quote
	var b strings.Builder
	for {
		if s.atEOF() || s.peek(0) == '\n' {
			return token.Token{}, s.errorAt(start, "unterminated string literal")
		}
		if s.peek(0) == '\'' {
			if s.peek(1) == '\'' {
				b.WriteByte('\'')
				s.advanceN(2)
				continue
			}
			s.advance()
			break
		}
		r, size := s.runeAt(s.off)
		b.WriteRune(r)
		s.advanceN(size)
	}
	return token.Token{Kind: token.Str, Text: s.text(startOff), Value: b.String(), Start: start, End: s.pos()}, nil
}

// multilineStringAhead reports whether the current position starts a
// `”'<spaces><newline>...”'` verbatim string literal.
func (s *scanner) multilineStringAhead() bool {
	i := s.off
	if i+3 > len(s.src) {
		return false
	}
	if s.src[i] != '\'' || s.src[i+1] != '\'' || s.src[i+2] != '\'' {
		return false
	}
	i += 3
	for i < len(s.src) && s.src[i] == ' ' {
		i++
	}
	if i < len(s.src) && s.src[i] == '\r' {
		i++
	}
	return i < len(s.src) && s.src[i] == '\n'
}

// lineEnd is the offset of the next line terminator at or after off.
func (s *scanner) lineEnd(off int) int {
	e := off
	for e < len(s.src) && s.src[e] != '\n' && s.src[e] != '\r' {
		e++
	}
	return e
}

// lineIndent is the number of leading spaces and tabs of a line.
func lineIndent(line string) int {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// stripIndent removes up to n leading spaces and tabs from a line.
func stripIndent(line string, n int) string {
	i := 0
	for i < len(line) && i < n && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[i:]
}

// scanMultiString scans a verbatim multiline string literal: three quotes,
// optional spaces, a newline, the content and a closing line that starts with
// three quotes.  The opening spaces and newline are dropped, the content is
// taken verbatim - except that the smallest indentation of the content lines,
// which has to be equal to the indentation of the closing line, is stripped
// from every content line - and the newline directly before the closing quotes
// is not part of the value.
func (s *scanner) scanMultiString(start token.Pos, startOff int) (token.Token, error) {
	s.advanceN(3) // opening quotes
	for s.peek(0) == ' ' {
		s.advance()
	}
	if s.peek(0) == '\r' {
		s.advance()
	}
	if s.peek(0) == '\n' {
		s.advance()
	}
	var lines []string
	for {
		if s.atEOF() {
			return token.Token{}, s.errorAt(start, "unterminated multiline string literal")
		}
		lineStart := s.off
		line := string(s.src[lineStart:s.lineEnd(lineStart)])
		rest := strings.TrimLeft(line, " \t")
		indent := lineIndent(line)
		if strings.HasPrefix(rest, "'''") {
			minIndent := -1
			for _, l := range lines {
				if n := lineIndent(l); minIndent < 0 || n < minIndent {
					minIndent = n
				}
			}
			if minIndent < 0 {
				minIndent = indent // empty literal
			}
			if indent != minIndent {
				return token.Token{}, s.errorAt(start, "Multiline string contains inconsistent indents")
			}
			s.advanceN(indent + 3) // the closing quotes
			var b strings.Builder
			for i, l := range lines {
				if i > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(stripIndent(l, minIndent))
			}
			return token.Token{Kind: token.MultiStr, Text: s.text(startOff),
				Value: b.String(), Start: start, End: s.pos()}, nil
		}
		if strings.Contains(rest, "'''") {
			return token.Token{}, s.errorAt(start,
				"There should be no non-whitespace characters before the closing quotes of the multiline string")
		}
		lines = append(lines, line)
		s.advanceN(s.lineEnd(s.off) - s.off)
		if s.peek(0) == '\r' {
			s.advance()
		}
		if s.peek(0) == '\n' {
			s.advance()
		}
	}
}

func (s *scanner) scanFormatString(start token.Pos, startOff int) (token.Token, error) {
	s.advanceN(2) // $'
	var b strings.Builder
	for {
		if s.atEOF() || s.peek(0) == '\n' {
			return token.Token{}, s.errorAt(start, "unterminated format string")
		}
		if s.peek(0) == '\'' {
			if s.peek(1) == '\'' {
				b.WriteByte('\'')
				s.advanceN(2)
				continue
			}
			s.advance()
			break
		}
		r, size := s.runeAt(s.off)
		b.WriteRune(r)
		s.advanceN(size)
	}
	return token.Token{Kind: token.FmtStr, Text: s.text(startOff), Value: b.String(), Start: start, End: s.pos()}, nil
}

func (s *scanner) scanSharp(start token.Pos, startOff int) (token.Token, error) {
	if s.peek(1) == '#' {
		// ## / ### short program markers
		n := 2
		for s.peek(n) == '#' {
			n++
		}
		if s.peek(n) == ' ' || s.peek(n) == '\t' || s.peek(n) == '\r' || s.peek(n) == '\n' {
			s.advanceN(n)
			kind := token.ShortProg
			if n >= 3 {
				kind = token.ShortSFProg
			}
			return token.Token{Kind: kind, Text: s.text(startOff), Start: start, End: s.pos()}, nil
		}
		return token.Token{}, s.errorAt(start, "unexpected '##'")
	}
	if isDigit(s.peek(1)) {
		s.advance() // #
		for !s.atEOF() && (isDigit(s.peek(0)) || s.peek(0) == '_') {
			s.advance()
		}
		return token.Token{Kind: token.CharLit, Text: s.text(startOff), Value: s.text(startOff)[1:], Start: start, End: s.pos()}, nil
	}
	if s.peek(1) == '$' && isHexDigit(s.peek(2)) {
		s.advanceN(2)
		for !s.atEOF() && (isHexDigit(s.peek(0)) || s.peek(0) == '_') {
			s.advance()
		}
		v := strings.ReplaceAll(s.text(startOff)[2:], "_", "")
		return token.Token{Kind: token.CharLit, Text: s.text(startOff), Value: "$" + v, Start: start, End: s.pos()}, nil
	}
	if s.peek(1) == '%' {
		s.advanceN(2)
		for !s.atEOF() && (s.peek(0) == '0' || s.peek(0) == '1' || s.peek(0) == '_') {
			s.advance()
		}
		return token.Token{Kind: token.CharLit, Text: s.text(startOff), Value: "%" + strings.ReplaceAll(s.text(startOff)[2:], "_", ""), Start: start, End: s.pos()}, nil
	}
	if isIdentStartByte(s.peek(1)) {
		// legacy `#directive` (e.g. #I) - treated as trivia
		s.advance()
		for !s.atEOF() {
			r, size := s.runeAt(s.off)
			if !identPart(r) {
				break
			}
			s.advanceN(size)
		}
		return token.Token{Kind: token.Directive, Text: s.text(startOff), Start: start, End: s.pos()}, nil
	}
	return token.Token{}, s.errorAt(start, "unexpected '#'")
}

var operators = []struct {
	text string
	kind token.Kind
}{
	{"+=", token.PlusEq},
	{"-=", token.MinusEq},
	{"*=", token.StarEq},
	{"/=", token.SlashEq},
	{":=", token.ColonEq},
	{"**", token.StarStar},
	{"<>", token.Neq},
	{"<=", token.Lte},
	{">=", token.Gte},
	{"??", token.DblQuestion},
	{"?.", token.QuestionDot},
	{"?[", token.QuestionSq},
	{"->", token.Arrow},
	{"||", token.CSharpOr},
	{"..", token.DotDot},
	{"\\(", token.BackslashParen},
	{"+", token.Plus},
	{"-", token.Minus},
	{"*", token.Star},
	{"/", token.Slash},
	{"=", token.Eq},
	{"<", token.Lt},
	{">", token.Gt},
	{"?", token.Question},
	{"@", token.At},
	{"^", token.Caret},
	{"&", token.Amp},
	{"|", token.Pipe},
	{",", token.Comma},
	{";", token.Semicolon},
	{":", token.Colon},
	{".", token.Dot},
	{"(", token.LParen},
	{")", token.RParen},
	{"[", token.LBracket},
	{"]", token.RBracket},
}

func (s *scanner) scanOperator(start token.Pos, startOff int) (token.Token, error) {
	// Unicode arrow
	if r, size := s.runeAt(s.off); r == '→' {
		s.advanceN(size)
		return token.Token{Kind: token.Arrow, Text: s.text(startOff), Value: "->", Start: start, End: s.pos()}, nil
	}
	rest := s.src[s.off:]
	for _, op := range operators {
		if len(rest) >= len(op.text) && string(rest[:len(op.text)]) == op.text {
			s.advanceN(len(op.text))
			return token.Token{Kind: op.kind, Text: op.text, Value: op.text, Start: start, End: s.pos()}, nil
		}
	}
	r, _ := s.runeAt(s.off)
	return token.Token{}, s.errorAt(start, "unexpected character %q", string(r))
}
