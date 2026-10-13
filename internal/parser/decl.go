package parser

import (
	"strconv"
	"strings"

	"abc/internal/ast"
	"abc/internal/token"
)

func parseInt64(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		if f, err2 := strconv.ParseFloat(s, 64); err2 == nil {
			return int64(f)
		}
	}
	return v
}

func parseHex(s string) (int64, bool) {
	v, err := strconv.ParseInt(s, 16, 64)
	return v, err == nil
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseCharCode decodes `#65`, `#$41` and `#%10000101`.
func parseCharCode(s string) int64 {
	switch {
	case strings.HasPrefix(s, "$"):
		v, _ := strconv.ParseInt(s[1:], 16, 64)
		return v
	case strings.HasPrefix(s, "%"):
		v, _ := strconv.ParseInt(s[1:], 2, 64)
		return v
	default:
		v, _ := strconv.ParseInt(s, 10, 64)
		return v
	}
}

// ------------------------------------------------------------------- types --

// parseType parses a type expression.
func (p *parser) parseType() ast.TypeExpr {
	t := p.parseTypeBase()
	for {
		switch p.kind() {
		case token.Question:
			p.next()
			t = &ast.NullableType{At: t.Pos(), Elem: t}
		case token.Arrow:
			at := p.pos()
			p.next()
			res := p.parseType()
			t = &ast.ProcType{At: at, IsFunc: true,
				Params: []*ast.Param{{At: at, Type: t}}, Result: res}
		default:
			return t
		}
	}
}

func (p *parser) parseTypeBase() ast.TypeExpr {
	at := p.pos()
	switch p.kind() {
	case token.KwArray:
		p.next()
		t := &ast.ArrayType{At: at}
		if p.at(token.LBracket) {
			p.next()
			for {
				r := &ast.RangeSpec{At: p.pos()}
				if !p.at(token.DotDot) && !p.at(token.Comma) && !p.at(token.RBracket) {
					r.Lo = p.parseExpr(false)
					if p.accept(token.DotDot) {
						r.Hi = p.parseExpr(false)
					} else {
						r.Hi = r.Lo
					}
				}
				t.Ranges = append(t.Ranges, r)
				if !p.accept(token.Comma) {
					break
				}
			}
			p.expect(token.RBracket)
		}
		p.expect(token.KwOf)
		t.Elem = p.parseType()
		return t
	case token.KwSet:
		p.next()
		p.expect(token.KwOf)
		return &ast.SetType{At: at, Elem: p.parseType()}
	case token.KwFile:
		p.next()
		if p.at(token.KwOf) {
			p.next()
			return &ast.FileType{At: at, Elem: p.parseType()}
		}
		return &ast.FileType{At: at}
	case token.Caret:
		p.next()
		return &ast.PointerType{At: at, Elem: p.parseType()}
	case token.KwProcedure, token.KwFunction:
		return p.parseProcType()
	case token.KwClass, token.KwRecord, token.KwInterface, token.KwAuto,
		token.KwSealed, token.KwPartial, token.KwStatic:
		return p.parseStructuredType()
	case token.LParen:
		return p.parseParenType()
	case token.Int, token.Minus, token.Plus:
		// subrange type `0..100`
		lo := p.parseRel()
		if p.at(token.DotDot) {
			p.next()
			hi := p.parseRel()
			return &ast.RangeType{At: at, Lo: lo, Hi: hi}
		}
		p.fatalf("expected subrange type, found %s", p.cur())
	case token.KwNew:
		p.next()
		return p.parseTypeBase()
	}

	if p.isIdentLike() || p.at(token.Ident) {
		name := p.ident()
		// `sequence of T`
		if p.at(token.KwOf) && strings.EqualFold(name, "sequence") {
			p.next()
			return &ast.SequenceType{At: at, Elem: p.parseType()}
		}
		// `string[n]`
		if strings.EqualFold(name, "string") && p.at(token.LBracket) && p.stringLenAhead() {
			p.next()
			e := p.parseExpr(false)
			p.expect(token.RBracket)
			return &ast.StringType{At: at, Len: e}
		}
		ref := &ast.TypeRef{At: at, Path: []string{name}}
		for p.at(token.Dot) {
			p.next()
			ref.Path = append(ref.Path, p.anyIdent())
		}
		if p.at(token.Lt) {
			p.next()
			for {
				ref.Args = append(ref.Args, p.parseType())
				if !p.accept(token.Comma) {
					break
				}
			}
			p.expect(token.Gt)
		}
		return ref
	}
	p.fatalf("expected type, found %s", p.cur())
	return nil
}

// parseParenType handles enum types `(a, b, c)`, parenthesised parameter
// lists of procedural types and `(...) -> T` shorthands.
func (p *parser) parseParenType() ast.TypeExpr {
	at := p.pos()
	// lambda-style parameter list `(...) -> T`
	if p.kindAt(1) != token.RParen && p.parenTypeIsProc() {
		p.next()
		params := p.parseParams()
		p.expect(token.RParen)
		p.expect(token.Arrow)
		res := p.parseType()
		return &ast.ProcType{At: at, IsFunc: true, Params: params, Result: res}
	}
	p.next()
	if p.at(token.RParen) {
		p.next()
		if p.at(token.Arrow) {
			p.next()
			return &ast.ProcType{At: at, IsFunc: true, Result: p.parseType()}
		}
		p.fatalf("empty type parentheses")
	}
	e := &ast.EnumType{At: at}
	for {
		e.Members = append(e.Members, p.identName())
		if !p.accept(token.Comma) {
			break
		}
	}
	p.expect(token.RParen)
	if p.at(token.Arrow) {
		p.next()
		res := p.parseType()
		return &ast.ProcType{At: at, IsFunc: true,
			Params: []*ast.Param{{At: at, Type: e}}, Result: res}
	}
	return e
}

// parenTypeIsProc reports whether the parenthesised group contains `name:`
// pairs (i.e. is a parameter list rather than an enumeration).
func (p *parser) parenTypeIsProc() bool {
	depth := 0
	for j := p.i + 1; j < len(p.toks); j++ {
		switch p.toks[j].Kind {
		case token.LParen, token.LBracket:
			depth++
		case token.RParen, token.RBracket:
			if depth == 0 {
				return false
			}
			depth--
		case token.Colon:
			if depth == 0 {
				return true
			}
		case token.EOF:
			return false
		}
	}
	return false
}

// stringLenAhead reports whether the bracketed group that follows `string`
// holds a single expression (`string[80]`) rather than a list of array sizes
// (`new string[12,12]`).
func (p *parser) stringLenAhead() bool {
	if p.kindAt(1) == token.RBracket {
		return false // `new string[](...)`
	}
	depth := 0
	for j := p.i; j < len(p.toks); j++ {
		switch p.toks[j].Kind {
		case token.LBracket:
			depth++
		case token.RBracket:
			depth--
			if depth == 0 {
				return true
			}
		case token.Comma:
			if depth == 1 {
				return false
			}
		case token.EOF:
			return false
		}
	}
	return false
}

func (p *parser) parseProcType() ast.TypeExpr {
	at := p.pos()
	isFunc := p.at(token.KwFunction)
	p.next()
	t := &ast.ProcType{At: at, IsFunc: isFunc}
	if p.at(token.LParen) {
		p.next()
		t.Params = p.parseParams()
		p.expect(token.RParen)
	}
	if p.at(token.Colon) {
		p.next()
		t.Result = p.parseType()
	}
	return t
}

// parseStructuredType parses `class`/`record`/`interface` type declarations.
func (p *parser) parseStructuredType() ast.TypeExpr {
	at := p.pos()
	t := &ast.RecordType{At: at}
	for {
		switch p.kind() {
		case token.KwStatic:
			p.next()
			t.IsStatic = true
		case token.KwAuto:
			p.next()
			t.IsAuto = true
		case token.KwSealed:
			p.next()
			t.IsSealed = true
		case token.KwPartial:
			p.next()
			t.IsPartial = true
		case token.KwClass:
			t.IsClass = true
			p.next()
		case token.KwRecord:
			t.IsClass = false
			p.next()
		case token.KwInterface:
			t.IsClass = true
			t.IsInterface = true
			p.next()
		default:
			goto bases
		}
	}
bases:
	if p.at(token.LParen) {
		p.next()
		for {
			name := strings.Join(p.dottedName(), ".")
			if p.at(token.Lt) {
				// generic base: `class(IComparer<Student>)`
				depth := 0
				for !p.atEOF() {
					tk := p.cur()
					p.next()
					if tk.Kind == token.Lt {
						depth++
					} else if tk.Kind == token.Gt {
						depth--
					}
					name += tk.Text
					if depth == 0 {
						break
					}
				}
			}
			t.Base = append(t.Base, name)
			if !p.accept(token.Comma) {
				break
			}
		}
		p.expect(token.RParen)
	}
	// forward declaration: `TFoo = class;`
	if p.at(token.Semicolon) {
		return t
	}
	vis := ast.VisDefault
	for !p.at(token.KwEnd) && !p.atEOF() {
		switch p.kind() {
		case token.KwWhere:
			// `where T: System.Array,ICloneable;`
			p.next()
			for !p.at(token.Semicolon) && !p.at(token.KwEnd) && !p.atEOF() {
				p.next()
			}
			p.accept(token.Semicolon)
		case token.KwPrivate:
			p.next()
			vis = ast.VisPrivate
			p.accept(token.Semicolon)
		case token.KwProtected:
			p.next()
			vis = ast.VisProtected
			p.accept(token.Semicolon)
		case token.KwPublic:
			p.next()
			vis = ast.VisPublic
			p.accept(token.Semicolon)
		case token.KwInternal:
			p.next()
			vis = ast.VisInternal
			p.accept(token.Semicolon)
		case token.KwConst:
			p.next()
			for _, d := range p.parseConstSection() {
				if c, ok := d.(*ast.ConstDecl); ok {
					t.Members = append(t.Members, c)
				}
			}
		case token.KwType:
			p.next()
			t.Members = append(t.Members, p.parseTypeSection()...)
		case token.KwVar, token.KwThreadvar:
			p.next()
			t.Members = append(t.Members, p.parseVarSection(false)...)
		case token.KwProperty:
			t.Members = append(t.Members, p.parseProperty())
		default:
			// member modifiers: `class arr: ...`, `auto property ...`,
			// `class constructor;`
			for p.at(token.KwClass) || p.at(token.KwStatic) ||
				p.at(token.KwAuto) || p.at(token.KwSealed) || p.at(token.KwPartial) {
				p.next()
			}
			if p.looksLikeProcHeader(0) {
				d := p.parseProcDecl()
				d.Visibility = vis
				t.Members = append(t.Members, d)
				continue
			}
			if p.at(token.KwProperty) {
				t.Members = append(t.Members, p.parseProperty())
				continue
			}
			// field: `a, b: T = init;` or `a := init;`
			f := &ast.RecordField{At: p.pos()}
			f.Names = append(f.Names, p.identName())
			for p.accept(token.Comma) {
				f.Names = append(f.Names, p.identName())
			}
			if p.at(token.Colon) {
				p.next()
				f.Type = p.parseType()
			}
			if p.at(token.ColonEq) {
				p.next()
				f.Init = p.parseExpr(false)
			}
			if f.Type == nil && f.Init == nil {
				p.fatalf("expected ':' in field declaration, found %s", p.cur())
			}
			p.expect(token.Semicolon)
			t.Members = append(t.Members, f)
		}
	}
	p.expect(token.KwEnd)
	return t
}

func (p *parser) parseProperty() ast.Decl {
	at := p.pos()
	p.expect(token.KwProperty)
	pr := &ast.PropertyDecl{At: at}
	pr.Name = p.ident()
	if p.at(token.LBracket) {
		// index property: `property Items[i: integer]: T read Get;`
		p.next()
		for {
			p.accept(token.KwVar)
			p.identName()
			p.expect(token.Colon)
			p.parseType()
			if !p.accept(token.Semicolon) {
				break
			}
		}
		p.expect(token.RBracket)
	}
	if p.at(token.Colon) {
		p.next()
		pr.Type = p.parseType()
	}
	if p.at(token.KwRead) {
		p.next()
		if p.isIdentLike() || p.at(token.Ident) {
			pr.Read = p.ident()
		}
	}
	if p.at(token.KwWrite) {
		p.next()
		if p.isIdentLike() || p.at(token.Ident) {
			pr.Write = p.ident()
		}
	}
	// `property Item[i: integer]: T read Get write Set; default;`
	if p.at(token.Semicolon) && p.kindAt(1) == token.KwDefault {
		p.next()
	}
	if p.at(token.KwDefault) {
		p.next()
		if !p.at(token.Semicolon) {
			pr.Default = p.parseExpr(false)
		}
	}
	p.expect(token.Semicolon)
	return pr
}

// ------------------------------------------------------------ parameters --

// parseParams parses the inside of a parameter list (without parentheses).
func (p *parser) parseParams() []*ast.Param {
	var params []*ast.Param
	for {
		par := &ast.Param{At: p.pos()}
		if p.isIdentLike() && strings.EqualFold(p.cur().Value, "out") {
			p.next()
			par.Mode = ast.ParamOut
		}
		switch p.kind() {
		case token.KwVar:
			p.next()
			par.Mode = ast.ParamVar
		case token.KwConst:
			p.next()
			par.Mode = ast.ParamConst
		case token.KwParams:
			p.next()
			par.Mode = ast.ParamParams
		}
		par.Names = append(par.Names, p.identName())
		for p.accept(token.Comma) {
			par.Names = append(par.Names, p.identName())
		}
		if !p.at(token.Colon) {
			// `procedure(a, b: integer)` shorthand - type comes later? no.
			p.fatalf("expected ':' in parameter list, found %s", p.cur())
		}
		p.expect(token.Colon)
		par.Type = p.parseType()
		if p.at(token.ColonEq) {
			p.next()
			par.Value = p.parseExpr(false)
		}
		params = append(params, par)
		if !p.accept(token.Semicolon) {
			break
		}
	}
	return params
}

// ------------------------------------------------------- subprograms ------

func (p *parser) parseAttributes() {
	if !p.at(token.LBracket) {
		return
	}
	p.next()
	for !p.at(token.RBracket) && !p.atEOF() {
		p.ident()
		if p.at(token.LParen) {
			p.next()
			depth := 1
			for depth > 0 && !p.atEOF() {
				if p.at(token.LParen) {
					depth++
				} else if p.at(token.RParen) {
					depth--
				}
				p.next()
			}
		}
		if !p.accept(token.Comma) {
			break
		}
	}
	p.expect(token.RBracket)
	p.accept(token.Semicolon)
}

// parseProcDecl parses a procedure/function/constructor/destructor/operator
// declaration, including its body when present.
func (p *parser) parseProcDecl() *ast.ProcDecl {
	d := &ast.ProcDecl{At: p.pos()}
	p.parseAttributes()
	for {
		switch p.kind() {
		case token.KwClass:
			p.next()
			d.IsClassMethod = true
		case token.KwStatic:
			p.next()
			d.IsStatic = true
		case token.KwOverload:
			p.next()
			d.IsOverload = true
		case token.KwOverride:
			p.next()
			d.IsOverride = true
		case token.KwVirtual:
			p.next()
			d.IsVirtual = true
		case token.KwAbstract:
			p.next()
			d.IsAbstract = true
		case token.KwReintroduce:
			p.next()
			d.IsReintroduce = true
		case token.KwExtensionMethod:
			p.next()
			d.IsExtension = true
		default:
			goto kw
		}
	}
kw:
	switch p.kind() {
	case token.KwProcedure:
		p.next()
		d.Kind = ast.ProcProcedure
	case token.KwFunction:
		p.next()
		d.Kind = ast.ProcFunction
	case token.KwConstructor:
		p.next()
		d.Kind = ast.ProcConstructor
	case token.KwDestructor:
		p.next()
		d.Kind = ast.ProcDestructor
	case token.KwOperator:
		p.next()
		d.Kind = ast.ProcOperator
	default:
		p.fatalf("expected procedure or function declaration, found %s", p.cur())
	}

	// name
	if p.at(token.KwOperator) {
		p.next()
		d.Kind = ast.ProcOperator
		d.OperatorTok = p.cur().Kind
		d.Name = []string{"operator" + operatorText(p.cur())}
		p.next()
	} else if p.at(token.LParen) || p.at(token.ColonEq) ||
		(p.at(token.Semicolon) && (d.Kind == ast.ProcConstructor || d.Kind == ast.ProcDestructor)) {
		// anonymous constructor/destructor: `constructor (d: integer);`,
		// `constructor;` or a name-less `constructor := exit;`
		d.Name = nil
	} else {
		d.Name = p.dottedName()
	}
	if p.at(token.Lt) {
		p.next()
		for {
			d.TypeArgs = append(d.TypeArgs, p.identName())
			if !p.accept(token.Comma) {
				break
			}
		}
		p.expect(token.Gt)
	}

	if p.at(token.LParen) {
		p.next()
		if !p.at(token.RParen) {
			d.Params = p.parseParams()
		}
		p.expect(token.RParen)
	}
	if p.at(token.Colon) {
		p.next()
		d.Result = p.parseType()
	}

	// `:= body` may follow the header directly
	if p.at(token.ColonEq) {
		p.next()
		if blk, e := p.bodyExprOrBlock(); blk != nil {
			d.Body = blk
		} else {
			d.BodyExpr = e
		}
		p.accept(token.Semicolon)
		return d
	}
	p.expect(token.Semicolon)

	// modifiers after the header
	noBody := false
	for {
		switch p.kind() {
		case token.KwForward:
			p.next()
			d.IsForward = true
			noBody = true
			p.accept(token.Semicolon)
		case token.KwExternal:
			p.next()
			d.IsExternal = true
			noBody = true
			if p.at(token.Str) {
				d.ExternalName = p.next().Value
			}
			if p.at(token.KwName) {
				p.next()
				if p.at(token.Str) {
					d.ExternalName = p.next().Value
				} else {
					d.ExternalName = p.ident()
				}
			}
			p.accept(token.Semicolon)
		case token.KwOverride, token.KwVirtual, token.KwAbstract, token.KwOverload,
			token.KwReintroduce, token.KwExtensionMethod:
			setProcModifier(d, p.next().Kind)
			if d.IsAbstract {
				noBody = true
			}
			p.accept(token.Semicolon)
		default:
			goto body
		}
	}
body:
	if noBody {
		return d
	}
	// local declarations before the body: `var ... const ... procedure ...;`
	save := p.i
	for p.startsDecl() {
		before := p.i
		ds := p.parseDeclSection()
		if p.i == before {
			break
		}
		d.LocalDecls = append(d.LocalDecls, ds...)
	}
	if !p.at(token.KwBegin) && !p.at(token.ColonEq) {
		// no body here - the declarations belong to an enclosing scope
		p.i = save
		d.LocalDecls = nil
	}
	if p.at(token.KwBegin) {
		d.Body = p.parseBlock()
		p.accept(token.Semicolon)
		return d
	}
	if p.at(token.ColonEq) {
		p.next()
		if blk, e := p.bodyExprOrBlock(); blk != nil {
			d.Body = blk
		} else {
			d.BodyExpr = e
		}
		p.accept(token.Semicolon)
		return d
	}
	return d
}

func setProcModifier(d *ast.ProcDecl, k token.Kind) {
	switch k {
	case token.KwOverride:
		d.IsOverride = true
	case token.KwVirtual:
		d.IsVirtual = true
	case token.KwAbstract:
		d.IsAbstract = true
	case token.KwOverload:
		d.IsOverload = true
	case token.KwReintroduce:
		d.IsReintroduce = true
	case token.KwExtensionMethod:
		d.IsExtension = true
	}
}

func operatorText(t token.Token) string {
	switch t.Kind {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Eq, token.Neq,
		token.Lt, token.Gt, token.Lte, token.Gte, token.ColonEq, token.StarStar:
		return t.Text
	}
	if t.Kind == token.Ident {
		return t.Value
	}
	return t.Text
}

// ---------------------------------------------------------------- lambdas --

// parseLambdaParen parses `(x, y) -> expr`.
func (p *parser) parseLambdaParen() ast.Expr {
	at := p.pos()
	p.expect(token.LParen)
	params := p.parseLambdaParamList()
	p.expect(token.RParen)
	p.expect(token.Arrow)
	return p.finishLambda(at, params)
}

// parseLambdaParamList parses lambda parameters: `(x, y)`, `(x: T)`,
// `(x, y: T)`, `(x: T, y: U)` and `(x; y: U)`.
func (p *parser) parseLambdaParamList() []*ast.Param {
	var params []*ast.Param
	if p.at(token.RParen) {
		return params
	}
	for {
		// nested tuple parameter: `\(a, b)`
		if p.at(token.BackslashParen) {
			params = append(params, p.parseTupleParam())
			if !p.accept(token.Semicolon) && !p.accept(token.Comma) {
				break
			}
			continue
		}
		par := &ast.Param{At: p.pos()}
		switch p.kind() {
		case token.KwVar:
			p.next()
			par.Mode = ast.ParamVar
		case token.KwConst:
			p.next()
			par.Mode = ast.ParamConst
		case token.KwParams:
			p.next()
			par.Mode = ast.ParamParams
		}
		par.Names = append(par.Names, p.identName())
		for p.accept(token.Comma) {
			par.Names = append(par.Names, p.identName())
		}
		if p.at(token.Colon) {
			p.next()
			par.Type = p.parseType()
		}
		params = append(params, par)
		if !p.accept(token.Semicolon) && !p.accept(token.Comma) {
			break
		}
	}
	return params
}

// parseTupleParam parses a nested tuple parameter `\(a, b)` of a lambda.
func (p *parser) parseTupleParam() *ast.Param {
	at := p.pos()
	p.expect(token.BackslashParen)
	par := &ast.Param{At: at}
	for !p.at(token.RParen) && !p.atEOF() {
		if p.at(token.BackslashParen) {
			par.Nested = append(par.Nested, p.parseTupleParam())
		} else {
			np := &ast.Param{At: p.pos()}
			np.Names = append(np.Names, p.identName())
			if p.at(token.Colon) {
				p.next()
				np.Type = p.parseType()
			}
			par.Nested = append(par.Nested, np)
		}
		if !p.accept(token.Comma) {
			break
		}
	}
	p.expect(token.RParen)
	return par
}

// finishLambda parses `-> expr` or `begin ... end` after a lambda header.
func (p *parser) finishLambda(at token.Pos, params []*ast.Param) ast.Expr {
	l := &ast.LambdaExpr{At: at, Params: params}
	if p.at(token.Colon) {
		p.next()
		l.Result = p.parseType()
		p.expect(token.ColonEq)
	}
	return p.lambdaBody(l)
}

// parseLambdaKeyword parses `function(...) ... ` / `procedure(...) ...`
// written in expression position.
func (p *parser) parseLambdaKeyword() ast.Expr {
	at := p.pos()
	isFunc := p.at(token.KwFunction)
	p.next()
	l := &ast.LambdaExpr{At: at}
	if p.at(token.LParen) {
		p.next()
		l.Params = p.parseLambdaParamList()
		p.expect(token.RParen)
	}
	if isFunc {
		if p.at(token.Colon) {
			p.next()
			l.Result = p.parseType()
		}
	}
	if p.at(token.ColonEq) {
		p.next()
		if p.at(token.KwBegin) {
			l.Stmts = p.parseBlock()
			return l
		}
		return p.lambdaBody(l)
	}
	// `procedure -> begin ... end` / `function(x): integer -> x*2`
	if p.at(token.Arrow) {
		p.next()
		if p.at(token.KwBegin) {
			l.Stmts = p.parseBlock()
			return l
		}
		return p.lambdaBody(l)
	}
	if p.at(token.KwBegin) {
		l.Stmts = p.parseBlock()
		return l
	}
	// `function(x: integer): integer := x*2` without `:=` is invalid, but
	// allow `function(x) := ...`
	p.fatalf("expected lambda body")
	return nil
}

// lambdaBody parses the body that follows `procedure ... ->` or `:=`:
// either a single expression or a statement list (`-> a[0] := ”`).
func (p *parser) lambdaBody(l *ast.LambdaExpr) *ast.LambdaExpr {
	blk, e := p.bodyExprOrBlock()
	if blk != nil {
		l.Stmts = blk
	} else {
		l.Body = e
	}
	return l
}

// bodyExprOrBlock parses a body written after `:=` or `->`.  It returns a
// block when the body is a statement (`begin`, `if`, `x := ...`) and an
// expression otherwise.
func (p *parser) bodyExprOrBlock() (*ast.Block, ast.Expr) {
	if p.at(token.KwBegin) {
		return p.parseBlock(), nil
	}
	if p.at(token.KwIf) || p.at(token.KwFor) || p.at(token.KwForeach) ||
		p.at(token.KwWhile) || p.at(token.KwRepeat) || p.at(token.KwCase) ||
		p.at(token.KwTry) || p.at(token.KwVar) {
		return p.parseSingleStmtBlock(), nil
	}
	save := p.i
	e := p.parseExpr(false)
	if p.at(token.ColonEq) || p.at(token.PlusEq) || p.at(token.MinusEq) ||
		p.at(token.StarEq) || p.at(token.SlashEq) {
		p.i = save
		return p.parseSingleStmtBlock(), nil
	}
	return nil, e
}

// parseSingleStmtBlock parses one statement as a lambda/short body.  A body
// without `begin` may contain exactly one statement.
func (p *parser) parseSingleStmtBlock() *ast.Block {
	b := &ast.Block{At: p.pos()}
	if s := p.parseStmt(); s != nil {
		b.Stmts = append(b.Stmts, s)
	}
	return b
}
