package parser

import (
	"strings"

	"abc/internal/ast"
	"abc/internal/token"
)

// parseExpr parses a full expression.  When allowColon is set the trailing
// field-width syntax (`'x':20:3`) is recognised.
func (p *parser) parseExpr(allowColon bool) ast.Expr {
	e := p.parseTo()
	if allowColon && p.at(token.Colon) {
		p.next()
		fe := &ast.FormatExpr{At: e.Pos(), Value: e}
		if p.startsExpr() {
			fe.Width = p.parseRel()
		}
		if p.at(token.Colon) {
			p.next()
			if p.startsExpr() {
				fe.Precision = p.parseRel()
			}
		}
		return fe
	}
	return e
}

// startsExpr reports whether the current token can begin an expression.
func (p *parser) startsExpr() bool {
	switch p.kind() {
	case token.Ident, token.Underscore, token.Int, token.Hex, token.Float,
		token.BigInt, token.Str, token.MultiStr, token.FmtStr, token.CharLit,
		token.LParen, token.LBracket, token.Pipe, token.Minus, token.Plus, token.At,
		token.BackslashParen,
		token.Caret, token.Star, token.Slash, token.Eq, token.Lt, token.Gt,
		token.Neq, token.Lte, token.Gte, token.Colon:
		return true
	}
	if token.IsKeyword(p.kind()) {
		switch p.kind() {
		case token.KwNil, token.KwNot, token.KwNew, token.KwSizeOf, token.KwTypeOf,
			token.KwDefault, token.KwIf,
			token.KwFunction, token.KwProcedure, token.KwInherited, token.KwArray,
			token.KwCase, token.KwIn, token.KwIs, token.KwAs, token.KwOr,
			token.KwAnd, token.KwXor, token.KwDiv, token.KwMod, token.KwShl,
			token.KwShr, token.KwClass, token.KwRecord, token.KwOperator:
			return true
		}
	}
	return false
}

// parseTo parses `a to b` pairs (lowest precedence).
func (p *parser) parseTo() ast.Expr {
	left := p.parseTernary()
	for p.noTo == 0 && p.at(token.KwTo) {
		at := p.pos()
		p.next()
		right := p.parseTernary()
		left = &ast.ToPair{At: at, Left: left, Right: right}
	}
	return left
}

// parseTernary handles `if c then a else b` and `c ? a : b` expressions.
func (p *parser) parseTernary() ast.Expr {
	if p.at(token.KwIf) {
		return p.parseIfExpr()
	}
	x := p.parseNullCoalesce()
	if p.at(token.Question) {
		at := p.pos()
		p.next()
		thenE := p.parseTernary()
		p.expect(token.Colon)
		elseE := p.parseTernary()
		return &ast.IfExpr{At: at, Cond: x, Then: thenE, Else: elseE}
	}
	return x
}

func (p *parser) parseIfExpr() ast.Expr {
	at := p.pos()
	p.expect(token.KwIf)
	e := &ast.IfExpr{At: at}
	e.Cond = p.parseTernary()
	p.expect(token.KwThen)
	e.Then = p.parseTernary()
	p.expect(token.KwElse)
	e.Else = p.parseTernary()
	return e
}

func (p *parser) parseNullCoalesce() ast.Expr {
	left := p.parseRel()
	for p.at(token.DblQuestion) {
		at := p.pos()
		p.next()
		right := p.parseRel()
		left = &ast.BinaryExpr{At: at, Op: token.DblQuestion, Left: left, Right: right}
	}
	return left
}

func (p *parser) isRelOp() bool {
	switch p.kind() {
	case token.Eq, token.Neq, token.Lt, token.Gt, token.Lte, token.Gte, token.KwIn:
		return true
	case token.KwNot:
		return p.kindAt(1) == token.KwIn
	}
	return false
}

// parseRel parses relational expressions; it is also used for case labels
// (where a top-level `:` must not be consumed).
func (p *parser) parseRel() ast.Expr {
	left := p.parseRange()
	for p.isRelOp() {
		at := p.pos()
		op := p.next().Kind
		if op == token.KwNot {
			p.expect(token.KwIn)
			op = token.KwIn
			// `not in` is rewritten as a negated membership test
			right := p.parseRelOperand()
			inner := &ast.BinaryExpr{At: at, Op: token.KwIn, Left: left, Right: right}
			left = &ast.UnaryExpr{At: at, Op: token.KwNot, X: inner}
			continue
		}
		right := p.parseRelOperand()
		left = &ast.BinaryExpr{At: at, Op: op, Left: left, Right: right}
	}
	return left
}

func (p *parser) parseRelOperand() ast.Expr {
	if p.at(token.KwIf) {
		return p.parseIfExpr()
	}
	return p.parseRange()
}

func (p *parser) parseRange() ast.Expr {
	left := p.parseAdd()
	for p.at(token.DotDot) {
		at := p.pos()
		p.next()
		right := p.parseAdd()
		left = &ast.BinaryExpr{At: at, Op: token.DotDot, Left: left, Right: right}
	}
	return left
}

func (p *parser) isAddOp() bool {
	switch p.kind() {
	case token.Plus, token.Minus, token.KwOr, token.KwXor:
		return true
	case token.CSharpOr:
		// inside a matrix literal `|| a, b | c, d ||` the `||` closes the
		// literal instead of acting as a logical or
		return p.noCSharpOr == 0
	}
	return false
}

func (p *parser) parseAdd() ast.Expr {
	left := p.parseMul()
	for p.isAddOp() {
		at := p.pos()
		op := p.next().Kind
		right := p.parseMul()
		left = &ast.BinaryExpr{At: at, Op: op, Left: left, Right: right}
	}
	return left
}

func (p *parser) isMulOp() bool {
	switch p.kind() {
	case token.Star, token.Slash, token.KwDiv, token.KwMod,
		token.KwShl, token.KwShr, token.KwAnd:
		return true
	}
	return false
}

func (p *parser) parseMul() ast.Expr {
	left := p.parseTerm()
	for p.isMulOp() {
		at := p.pos()
		op := p.next().Kind
		right := p.parseTerm()
		left = &ast.BinaryExpr{At: at, Op: op, Left: left, Right: right}
	}
	return left
}

// parseTerm handles `as` and `is`, which bind tighter than `*`.
func (p *parser) parseTerm() ast.Expr {
	x := p.parseUnary()
	for p.at(token.KwAs) || p.at(token.KwIs) {
		at := p.pos()
		isOp := p.at(token.KwIs)
		p.next()
		if isOp {
			// `x is (a, b)` and `x is [a, b]` are pattern tests
			if p.at(token.LParen) {
				x = &ast.TypeTestExpr{At: at, X: x, Pattern: p.parseTuplePattern(p.pos())}
				continue
			}
			if p.at(token.LBracket) {
				x = &ast.TypeTestExpr{At: at, X: x, Pattern: p.parseCollectionPattern(p.pos())}
				continue
			}
			typ := p.parseType()
			tt := &ast.TypeTestExpr{At: at, X: x, Type: typ}
			// `x is Person(var name, var age)`
			if p.at(token.LParen) {
				tt.Pattern = &ast.Pattern{At: at, Kind: ast.PatDeconstruct,
					Type: typ, Elems: p.parsePatternArgs()}
			}
			x = tt
			continue
		}
		typ := p.parseType()
		x = &ast.TypeCastExpr{At: at, X: x, Type: typ}
	}
	return x
}

func (p *parser) parseUnary() ast.Expr {
	switch p.kind() {
	case token.Plus, token.Minus:
		at := p.pos()
		op := p.next().Kind
		x := p.parseUnary()
		return &ast.UnaryExpr{At: at, Op: op, X: x}
	case token.KwNot:
		at := p.pos()
		p.next()
		x := p.parsePostfix()
		return &ast.UnaryExpr{At: at, Op: token.KwNot, X: x}
	case token.Caret:
		at := p.pos()
		p.next()
		x := p.parsePostfix()
		return &ast.DerefExpr{At: at, X: x}
	}
	base := p.parsePostfix()
	if p.at(token.StarStar) {
		at := p.pos()
		p.next()
		right := p.parseUnary()
		return &ast.BinaryExpr{At: at, Op: token.StarStar, Left: base, Right: right}
	}
	return base
}

// parsePostfix parses calls, indexing, field access and dereference.
func (p *parser) parsePostfix() ast.Expr {
	if p.at(token.At) {
		at := p.pos()
		p.next()
		x := p.parsePostfix()
		return &ast.AddrExpr{At: at, X: x}
	}
	x := p.parsePrimary()
	// adjacent string/character literals are one literal: 'a'#10#10'b'
	if p.atAny(token.Str, token.MultiStr, token.CharLit) {
		if s, ok := plainStrLit(x); ok {
			x = p.joinLiterals(s)
		}
	}
	for {
		switch p.kind() {
		case token.LParen:
			call := &ast.CallExpr{At: x.Pos(), Fn: x}
			call.Args = p.parseArgs()
			x = call
		case token.LBracket:
			x = p.parseIndex(x)
		case token.Dot:
			p.next()
			name := p.anyIdent()
			x = &ast.FieldExpr{At: x.Pos(), X: x, Name: name}
		case token.QuestionDot:
			p.next()
			name := p.anyIdent()
			x = &ast.FieldExpr{At: x.Pos(), X: x, Name: name}
		case token.Arrow:
			p.next()
			name := p.anyIdent()
			x = &ast.FieldExpr{At: x.Pos(), X: x, Name: name}
		case token.Caret:
			at := p.pos()
			p.next()
			x = &ast.DerefExpr{At: at, X: x}
		case token.Amp:
			// `Foo&<T>` - `&` escapes the type argument list
			if p.kindAt(1) != token.Lt {
				return x
			}
			at := p.pos()
			p.next()
			x = &ast.GenericRef{At: at, X: x, Args: p.parseTypeArgs()}
		case token.QuestionSq:
			at := p.pos()
			p.next()
			idx := p.parseExpr(true)
			p.expect(token.RBracket)
			d := ast.IndexDim{}
			if fe, ok := idx.(*ast.FormatExpr); ok {
				d.IsSlice = true
				d.Lo = fe.Value
				d.Hi = fe.Width
				d.Step = fe.Precision
			} else {
				d.Lo = idx
			}
			x = &ast.IndexExpr{At: at, X: x, Dims: []ast.IndexDim{d}}
		default:
			return x
		}
	}
}

// plainStrLit reports whether x is a non-format string literal.  A character
// literal is converted to a one-character string so that it can be joined
// with adjacent literals.
func plainStrLit(x ast.Expr) (*ast.StrLit, bool) {
	if s, ok := x.(*ast.StrLit); ok && !s.Format {
		return s, true
	}
	if c, ok := x.(*ast.IntLit); ok && c.IsChar {
		return &ast.StrLit{At: c.At, Value: string(rune(c.Value))}, true
	}
	return nil, false
}

// joinLiterals merges any following string and character literals into s:
// `'text'#10#10'more'` is a single string.
func (p *parser) joinLiterals(s *ast.StrLit) ast.Expr {
	for {
		switch p.kind() {
		case token.Str, token.MultiStr:
			s.Value += p.next().Value
		case token.CharLit:
			s.Value += string(rune(parseCharCode(p.next().Value)))
		default:
			return s
		}
	}
}

// parseIndex parses `a[i, j]`, slices `a[lo:hi:step]` and mixed forms
// such as `a[:, k]` or `a[k, :]`.
func (p *parser) parseIndex(x ast.Expr) ast.Expr {
	at := p.pos()
	p.expect(token.LBracket)
	ie := &ast.IndexExpr{At: at, X: x}
	if p.at(token.RBracket) {
		p.next()
		return ie
	}
	for {
		d := ast.IndexDim{}
		if !p.at(token.Colon) && !p.at(token.RBracket) {
			d.Lo = p.parseExpr(false)
		}
		if p.at(token.Colon) {
			p.next()
			d.IsSlice = true
			if !p.at(token.Colon) && !p.at(token.Comma) && !p.at(token.RBracket) {
				d.Hi = p.parseExpr(false)
			}
		}
		if p.at(token.Colon) {
			p.next()
			d.IsSlice = true
			if !p.at(token.Comma) && !p.at(token.RBracket) {
				d.Step = p.parseExpr(false)
			}
		}
		ie.Dims = append(ie.Dims, d)
		if !p.accept(token.Comma) {
			break
		}
	}
	p.expect(token.RBracket)
	return ie
}

// parseArgs parses a parenthesised argument list.
func (p *parser) parseArgs() []*ast.CallArg {
	p.expect(token.LParen)
	var args []*ast.CallArg
	if p.at(token.RParen) {
		p.next()
		return args
	}
	for {
		a := &ast.CallArg{At: p.pos()}
		if (p.isIdentLike() || p.at(token.Ident)) && p.kindAt(1) == token.ColonEq {
			a.Name = p.ident()
			p.next() // :=
			a.Value = p.parseExpr(false)
		} else {
			a.Value = p.parseExpr(true)
		}
		args = append(args, a)
		if !p.accept(token.Comma) {
			break
		}
	}
	p.expect(token.RParen)
	return args
}

// anyIdent accepts any identifier-like token, including reserved words
// (used after `.` and for member names).
func (p *parser) anyIdent() string {
	if p.atEOF() {
		p.fatalf("expected identifier, found end of file")
	}
	t := p.next()
	if t.Kind == token.Ident || t.Kind == token.Underscore {
		return t.Value
	}
	return strings.ToLower(t.Text)
}

// isLambdaAhead reports whether the parenthesis group starting at the current
// token is a lambda parameter list (`(x, y) -> ...`).
func (p *parser) isLambdaAhead() bool {
	if !p.at(token.LParen) {
		return false
	}
	depth := 0
	for j := p.i; j < len(p.toks); j++ {
		switch p.toks[j].Kind {
		case token.LParen, token.LBracket:
			depth++
		case token.RParen, token.RBracket:
			depth--
			if depth == 0 {
				if j+1 < len(p.toks) && p.toks[j+1].Kind == token.Arrow {
					// ensure there is something between `(` and `)`
					return j > p.i
				}
				return false
			}
		case token.EOF:
			return false
		}
	}
	return false
}

// isRecordCtorAhead reports whether the parenthesised group is a record
// constructor: `(Name: 'x'; Age: 20)`.
func (p *parser) isRecordCtorAhead() bool {
	if !p.at(token.LParen) {
		return false
	}
	k1 := p.kindAt(1)
	ident := k1 == token.Ident || (token.IsKeyword(k1) && contextualKeyword(k1))
	if !ident || p.kindAt(2) != token.Colon {
		return false
	}
	k3 := p.kindAt(3)
	return k3 != token.Eq && k3 != token.ColonEq
}

func (p *parser) parsePrimary() ast.Expr {
	at := p.pos()
	switch p.kind() {
	case token.Int:
		t := p.next()
		return &ast.IntLit{At: at, Value: parseInt64(t.Value), Text: t.Value}
	case token.Hex:
		t := p.next()
		v, ok := parseHex(t.Value)
		if !ok {
			p.errorf(at, "invalid hexadecimal literal %q", t.Text)
		}
		return &ast.IntLit{At: at, Value: v, Text: t.Text}
	case token.CharLit:
		t := p.next()
		return &ast.IntLit{At: at, Value: parseCharCode(t.Value), Text: t.Text, IsChar: true}
	case token.Float:
		t := p.next()
		return &ast.RealLit{At: at, Value: parseFloat(t.Value), Text: t.Text}
	case token.BigInt:
		t := p.next()
		return &ast.BigIntLit{At: at, Value: t.Value}
	case token.Str, token.MultiStr, token.FmtStr:
		t := p.next()
		return &ast.StrLit{At: at, Value: t.Value, Format: t.Kind == token.FmtStr,
			Multiline: t.Kind == token.MultiStr}
	case token.KwNil:
		p.next()
		return &ast.NilLit{At: at}
	case token.KwInherited:
		p.next()
		names := []string{"inherited"}
		if p.isIdentLike() || p.at(token.Ident) {
			names = append(names, p.ident())
		}
		x := &ast.IdentExpr{At: at, Name: names}
		if p.at(token.LParen) {
			return &ast.CallExpr{At: at, Fn: x, Args: p.parseArgs()}
		}
		return x
	case token.KwNew:
		return p.parseNew()
	case token.KwSizeOf:
		if p.kindAt(1) == token.LParen {
			p.next()
			p.expect(token.LParen)
			e := &ast.SizeOfExpr{At: at}
			e.X = p.parseTypeOrExprForSizeof()
			p.expect(token.RParen)
			return e
		}
	case token.KwTypeOf:
		if p.kindAt(1) == token.LParen {
			p.next()
			p.expect(token.LParen)
			e := &ast.TypeOfExpr{At: at, Type: p.parseType()}
			p.expect(token.RParen)
			return e
		}
	case token.KwDefault:
		p.next()
		p.expect(token.LParen)
		typ := p.parseType()
		p.expect(token.RParen)
		return &ast.CallExpr{At: at, Fn: &ast.IdentExpr{At: at, Name: []string{"default"}},
			Args: []*ast.CallArg{{At: at, TypeOnly: true, Type: typ}}}
	case token.KwFunction, token.KwProcedure:
		return p.parseLambdaKeyword()
	case token.BackslashParen:
		// `\(x, y) -> ...`
		p.next()
		params := p.parseLambdaParamList()
		p.expect(token.RParen)
		p.expect(token.Arrow)
		return p.finishLambda(at, params)
	case token.CSharpOr:
		// matrix literal `|| 1,2,3 | 4,5,6 ||`
		p.next()
		al := &ast.ArrayLit{At: at}
		for !p.at(token.CSharpOr) && !p.atEOF() {
			if p.at(token.Pipe) {
				p.next() // row opener
			}
			var row []ast.Expr
			for !p.at(token.Pipe) && !p.at(token.CSharpOr) && !p.atEOF() {
				p.noCSharpOr++
				el := p.parseExpr(true)
				p.noCSharpOr--
				row = append(row, el)
				if !p.accept(token.Comma) {
					break
				}
			}
			if len(row) > 0 {
				al.Elems = append(al.Elems, &ast.ArrayLit{At: p.pos(), Elems: row})
			}
			if p.at(token.Pipe) {
				p.next() // row closer
			}
			p.accept(token.Comma)
		}
		p.expect(token.CSharpOr)
		return al
	case token.LParen:
		if p.isLambdaAhead() {
			return p.parseLambdaParen()
		}
		if p.isRecordCtorAhead() {
			return p.parseRecordCtor()
		}
		// `(var a, var b) := ...`
		if p.kindAt(1) == token.KwVar || p.kindAt(1) == token.KwConst {
			p.next()
			var elems []ast.Expr
			for {
				p.accept(token.KwVar)
				p.accept(token.KwConst)
				elems = append(elems, &ast.IdentExpr{At: p.pos(), Name: p.dottedName()})
				if !p.accept(token.Comma) {
					break
				}
			}
			p.expect(token.RParen)
			return &ast.TupleExpr{At: at, Elems: elems}
		}
		p.next()
		if p.at(token.RParen) {
			p.fatalf("empty parentheses are not an expression")
		}
		first := p.parseExpr(true)
		// `(x := y)` / `(sum += x)` - an assignment used as an expression
		if p.at(token.ColonEq) || p.at(token.PlusEq) || p.at(token.MinusEq) ||
			p.at(token.StarEq) || p.at(token.SlashEq) {
			op := p.next().Kind
			rhs := p.parseExpr(true)
			p.expect(token.RParen)
			return &ast.AssignExpr{At: at, Target: first, Op: op, Value: rhs}
		}
		if p.at(token.Comma) {
			elems := []ast.Expr{first}
			for p.accept(token.Comma) {
				elems = append(elems, p.parseExpr(true))
			}
			p.expect(token.RParen)
			return &ast.TupleExpr{At: at, Elems: elems}
		}
		p.expect(token.RParen)
		return first
	case token.LBracket:
		p.next()
		sl := &ast.SetLit{At: at}
		for {
			if p.at(token.RBracket) {
				break
			}
			sl.Elems = append(sl.Elems, p.parseExpr(false))
			if !p.accept(token.Comma) {
				break
			}
		}
		p.expect(token.RBracket)
		return sl
	case token.Pipe:
		p.next()
		al := &ast.ArrayLit{At: at}
		for {
			if p.at(token.Pipe) {
				break
			}
			al.Elems = append(al.Elems, p.parseExpr(false))
			if !p.accept(token.Comma) {
				break
			}
		}
		p.expect(token.Pipe)
		return al
	case token.KwCase:
		// case expression used as a value
		return p.parseCaseExpr()
	}

	// identifier (or contextual keyword used as a name)
	if p.isIdentLike() || p.at(token.Ident) {
		// arrow lambda with a single parameter: `x -> expr`
		if p.kindAt(1) == token.Arrow {
			name := p.ident()
			p.next() // ->
			l := &ast.LambdaExpr{At: at,
				Params: []*ast.Param{{At: at, Names: []string{name}}}}
			if p.at(token.KwBegin) {
				l.Stmts = p.parseBlock()
				return l
			}
			if p.at(token.Colon) {
				p.next()
				l.Result = p.parseType()
				p.expect(token.ColonEq)
			}
			return p.lambdaBody(l)
		}
		name := p.ident()
		if strings.EqualFold(name, "string") && p.at(token.LBracket) {
			p.next()
			e := p.parseExpr(false)
			p.expect(token.RBracket)
			return &ast.CallExpr{At: at, Fn: &ast.IdentExpr{At: at, Name: []string{"string"}},
				Args: []*ast.CallArg{{At: at, Value: e}}}
		}
		return &ast.IdentExpr{At: at, Name: []string{name}}
	}

	p.fatalf("expected expression, found %s", p.cur())
	return nil
}

func (p *parser) parseNew() ast.Expr {
	at := p.pos()
	p.expect(token.KwNew)
	e := &ast.NewExpr{At: at}
	e.Type = p.parseType()
	if p.at(token.LParen) {
		e.Args = p.parseArgs()
	} else if p.at(token.LBracket) {
		p.next()
		// `new T[]` - the rank is inferred; `new T[10,20]` - several sizes
		if !p.at(token.RBracket) {
			for {
				e.Sizes = append(e.Sizes, p.parseExpr(false))
				if !p.accept(token.Comma) {
					break
				}
			}
		}
		p.expect(token.RBracket)
		if p.at(token.LParen) {
			e.Args = p.parseArgs()
		}
	}
	return e
}

func (p *parser) parseCaseExpr() ast.Expr {
	at := p.pos()
	p.expect(token.KwCase)
	// `case expr of a: x; b: y end` used as an expression
	x := p.parseExpr(false)
	_ = x
	p.errorf(at, "case expressions are not supported yet")
	return nil
}

// parseTypeOrExprForSizeof accepts either a type name or an expression.
func (p *parser) parseTypeOrExprForSizeof() ast.Expr {
	return p.parseExpr(false)
}

func (p *parser) parseRecordCtor() ast.Expr {
	at := p.pos()
	p.expect(token.LParen)
	rc := &ast.RecordCtorExpr{At: at}
	for {
		f := &ast.FieldInit{At: p.pos()}
		f.Name = p.ident()
		p.expect(token.Colon)
		f.Value = p.parseExpr(true)
		rc.Fields = append(rc.Fields, f)
		if p.at(token.Semicolon) {
			p.next()
			if p.at(token.RParen) {
				break
			}
			continue
		}
		break
	}
	p.expect(token.RParen)
	return rc
}
