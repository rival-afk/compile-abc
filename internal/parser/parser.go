// Package parser implements a recursive-descent parser for PascalABC.
package parser

import (
	"fmt"

	"abc/internal/ast"
	"abc/internal/lexer"
	"abc/internal/token"
)

// Error is a syntax error with a source position.
type Error struct {
	File string
	Pos  token.Pos
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Pos.Line, e.Pos.Col, e.Msg)
}

type bail struct{ err error }

type parser struct {
	file string
	toks []token.Token
	i    int
	errs []error
	// set by statements that consume their own trailing `;`
	ateSemi bool
	// >0 while parsing `for` bounds, where `to` is a loop keyword
	noTo       int
	noCSharpOr int
}

// Parse parses a token stream produced by the lexer.
func Parse(res *lexer.Result) (prog *ast.Program, err error) {
	p := &parser{file: res.File, toks: res.Tokens}
	defer func() {
		if r := recover(); r != nil {
			if b, ok := r.(bail); ok {
				prog, err = nil, b.err
				return
			}
			panic(r)
		}
	}()
	prog = p.parseFile(res)
	if len(p.errs) > 0 {
		return nil, p.errs[0]
	}
	return prog, nil
}

// ------------------------------------------------------------------ helpers

func (p *parser) cur() token.Token { return p.toks[p.i] }
func (p *parser) kind() token.Kind { return p.toks[p.i].Kind }
func (p *parser) kindAt(n int) token.Kind {
	if p.i+n < len(p.toks) {
		return p.toks[p.i+n].Kind
	}
	return token.EOF
}
func (p *parser) at(k token.Kind) bool { return p.toks[p.i].Kind == k }
func (p *parser) atEOF() bool          { return p.toks[p.i].Kind == token.EOF }

func (p *parser) atAny(ks ...token.Kind) bool {
	k := p.kind()
	for _, w := range ks {
		if k == w {
			return true
		}
	}
	return false
}

func (p *parser) next() token.Token {
	t := p.toks[p.i]
	if t.Kind != token.EOF {
		p.i++
	}
	return t
}

func (p *parser) accept(k token.Kind) bool {
	if p.at(k) {
		p.next()
		return true
	}
	return false
}

func (p *parser) errorf(pos token.Pos, format string, args ...interface{}) {
	e := &Error{File: p.file, Pos: pos, Msg: fmt.Sprintf(format, args...)}
	p.errs = append(p.errs, e)
	panic(bail{e})
}

func (p *parser) fatalf(format string, args ...interface{}) {
	p.errorf(p.cur().Start, format, args...)
}

func (p *parser) expect(k token.Kind) token.Token {
	if !p.at(k) {
		p.fatalf("expected %s, found %s", token.Text(k), p.cur())
	}
	return p.next()
}

func (p *parser) isIdentLike() bool {
	// contextual keywords that may be used as identifiers
	switch p.kind() {
	case token.Ident, token.Underscore:
		return true
	}
	return token.IsKeyword(p.kind()) && contextualKeyword(p.kind())
}

// contextualKeyword reports whether a keyword may be used as an ordinary
// identifier (the original grammar calls these `non_reserved`).
func contextualKeyword(k token.Kind) bool {
	switch k {
	case token.KwName, token.KwRead, token.KwWrite, token.KwIndex,
		token.KwNew, token.KwOn, token.KwPrivate, token.KwProtected,
		token.KwPublic, token.KwInternal, token.KwDefault, token.KwEvent,
		token.KwLock, token.KwStep, token.KwParams,
		token.KwOverload, token.KwOverride,
		token.KwVirtual, token.KwAbstract, token.KwForward, token.KwExternal,
		token.KwStatic, token.KwSealed, token.KwPartial, token.KwAuto,
		token.KwLoop, token.KwSequence, token.KwYield, token.KwWhere,
		token.KwImplicit, token.KwExplicit, token.KwExtensionMethod,
		token.KwReintroduce, token.KwThreadvar, token.KwResourceString,
		token.KwExports, token.KwTemplate, token.KwNamespace, token.KwLibrary,
		token.KwFile, token.KwSizeOf, token.KwTypeOf:
		return true
	}
	return false
}

// ident parses an identifier, allowing contextual keywords.
func (p *parser) ident() string {
	if p.isIdentLike() {
		t := p.next()
		if t.Kind == token.Ident || t.Kind == token.Underscore {
			return t.Value
		}
		return t.Text
	}
	p.fatalf("expected identifier, found %s", p.cur())
	return ""
}

// identName is like ident but rejects a lone underscore.
func (p *parser) identName() string {
	name := p.ident()
	if name == "_" {
		p.fatalf("'_' is not a valid name here")
	}
	return name
}

// dottedName parses `a.b.c`.
func (p *parser) dottedName() []string {
	names := []string{p.ident()}
	for p.at(token.Dot) {
		p.next()
		names = append(names, p.ident())
	}
	return names
}

func (p *parser) pos() token.Pos { return p.cur().Start }

// ---------------------------------------------------------------- entry --

func (p *parser) parseFile(res *lexer.Result) *ast.Program {
	prog := &ast.Program{ZeroBasedStrings: res.ZeroBasedStrings}
	prog.At = p.pos()

	switch p.kind() {
	case token.ShortProg, token.ShortSFProg:
		p.next()
		prog.Short = true
		if p.at(token.KwUses) {
			prog.Uses = p.parseAllUses()
		}
		prog.Decls = p.parseDeclList(true)
		prog.Body = &ast.Block{At: p.pos(), Stmts: p.parseStmtList(token.EOF)}
		return prog
	case token.KwUnit:
		return p.parseUnit()
	case token.KwLibrary:
		p.next()
		prog.Name = p.ident()
		p.expect(token.Semicolon)
		prog.Decls = p.parseDeclList(false)
		// a library may end without an explicit body: `library L; ... end.`
		if p.at(token.KwBegin) {
			prog.Body = p.parseBlockWithDecls()
		} else {
			prog.Body = &ast.Block{At: p.pos()}
			p.expect(token.KwEnd)
		}
		p.accept(token.Dot)
		return prog
	case token.KwProgram:
		p.next()
		prog.Name = p.ident()
		if p.at(token.LParen) {
			p.next()
			for !p.at(token.RParen) && !p.atEOF() {
				p.ident()
				if !p.accept(token.Comma) {
					break
				}
			}
			p.expect(token.RParen)
		}
		p.expect(token.Semicolon)
	}

	prog.Uses = p.parseAllUses()
	prog.Decls = p.parseDeclList(false)
	if p.atEOF() || p.at(token.Dot) {
		// a file may consist of declarations only (or just comments)
		prog.Body = &ast.Block{At: p.pos()}
	} else {
		prog.Body = p.parseBlockWithDecls()
	}
	p.accept(token.Dot)
	if !p.atEOF() {
		p.fatalf("expected end of file, found %s", p.cur())
	}
	return prog
}

func (p *parser) parseUnit() *ast.Program {
	prog := &ast.Program{IsUnit: true}
	prog.At = p.pos()
	p.expect(token.KwUnit)
	prog.UnitName = p.dottedName()
	p.expect(token.Semicolon)

	if p.at(token.KwInterface) {
		p.next()
		prog.Uses = p.parseAllUses()
		prog.InterfaceDecls = p.parseDeclList(true)
	}
	if p.at(token.KwImplementation) {
		p.next()
		prog.Uses = append(prog.Uses, p.parseAllUses()...)
		prog.ImplDecls = p.parseDeclList(false)
	}
	if p.at(token.KwInitialization) {
		p.next()
		prog.Init = &ast.Block{At: p.pos(),
			Stmts: p.parseStmtList(token.KwFinalization, token.KwEnd, token.Dot, token.EOF)}
	}
	if p.at(token.KwFinalization) {
		p.next()
		prog.Fin = &ast.Block{At: p.pos(),
			Stmts: p.parseStmtList(token.KwEnd, token.Dot, token.EOF)}
	}
	p.accept(token.Dot)
	return prog
}

// parseAllUses parses one or more consecutive `uses ...;` clauses.
func (p *parser) parseAllUses() []*ast.UsesItem {
	var items []*ast.UsesItem
	for p.at(token.KwUses) {
		items = append(items, p.parseUses()...)
	}
	return items
}

// parseUses parses a `uses a, b.c;` clause.
func (p *parser) parseUses() []*ast.UsesItem {
	p.expect(token.KwUses)
	var items []*ast.UsesItem
	for {
		it := &ast.UsesItem{At: p.pos()}
		it.Path = p.dottedName()
		if p.at(token.KwIn) {
			p.next()
			if p.at(token.Str) {
				it.File = p.next().Value
			} else {
				p.fatalf("expected string file name after 'in'")
			}
		}
		items = append(items, it)
		if !p.accept(token.Comma) {
			break
		}
	}
	p.expect(token.Semicolon)
	return items
}

// ------------------------------------------------------- declaration lists --

var declStart = []token.Kind{
	token.KwLabel, token.KwConst, token.KwResourceString, token.KwType,
	token.KwVar, token.KwThreadvar, token.KwProcedure, token.KwFunction,
	token.KwConstructor, token.KwDestructor, token.KwOperator, token.KwClass,
	token.KwStatic, token.KwAuto, token.KwSealed, token.KwPartial,
	token.KwOverload, token.KwOverride, token.KwVirtual, token.KwAbstract,
	token.KwExternal, token.KwExtensionMethod, token.LBracket,
	token.KwImplicit, token.KwExplicit,
}

// parseDeclList parses a list of declarations.  When procOnly is set, only
// procedure/function declarations are accepted (used for short programs).
func (p *parser) parseDeclList(procOnly bool) []ast.Decl {
	var decls []ast.Decl
	for {
		if p.atEOF() {
			break
		}
		if procOnly {
			if !p.atAny(token.KwProcedure, token.KwFunction, token.KwConstructor,
				token.KwDestructor, token.KwOperator, token.KwClass, token.KwStatic,
				token.KwOverload, token.KwOverride, token.KwVirtual, token.KwAbstract,
				token.KwExtensionMethod, token.LBracket) {
				break
			}
			// `class`/`static` may also start an unrelated construct; only
			// treat them as modifiers when followed by a subprogram keyword.
			if p.at(token.KwClass) || p.at(token.KwStatic) || p.at(token.KwOverload) ||
				p.at(token.KwOverride) || p.at(token.KwVirtual) || p.at(token.KwAbstract) ||
				p.at(token.KwExtensionMethod) {
				if !p.looksLikeProcHeader(0) {
					break
				}
			}
			d := p.parseProcDecl()
			decls = append(decls, d)
			continue
		}
		if !p.startsDecl() {
			break
		}
		before := p.i
		d := p.parseDeclSection()
		if p.i == before {
			p.fatalf("expected declaration, found %s", p.cur())
		}
		decls = append(decls, d...)
	}
	return decls
}

func (p *parser) startsDecl() bool {
	k := p.kind()
	for _, w := range declStart {
		if k == w {
			if k == token.KwClass || k == token.KwStatic || k == token.KwAuto ||
				k == token.KwSealed || k == token.KwPartial || k == token.KwOverload ||
				k == token.KwOverride || k == token.KwVirtual || k == token.KwAbstract ||
				k == token.KwExternal || k == token.KwExtensionMethod {
				return p.looksLikeProcHeader(0)
			}
			return true
		}
	}
	return false
}

// looksLikeProcHeader reports whether the token sequence at lookahead n
// begins a procedure/function declaration (`class function ...`, `overload
// procedure ...` etc).
func (p *parser) looksLikeProcHeader(n int) bool {
	for i := n; i < n+6; i++ {
		switch p.kindAt(i) {
		case token.KwProcedure, token.KwFunction, token.KwConstructor, token.KwDestructor, token.KwOperator:
			return true
		case token.KwClass, token.KwStatic, token.KwOverload, token.KwOverride,
			token.KwVirtual, token.KwAbstract, token.KwExtensionMethod,
			token.KwReintroduce, token.KwSealed, token.KwPartial, token.KwAuto,
			token.KwExternal, token.KwForward, token.LBracket:
			continue
		default:
			return false
		}
	}
	return false
}

// parseDeclSection parses one declaration section (`const`, `type`, `var`, ...).
func (p *parser) parseDeclSection() []ast.Decl {
	switch p.kind() {
	case token.KwLabel:
		p.next()
		for {
			if p.at(token.Int) {
				p.next()
			} else {
				p.ident()
			}
			if !p.accept(token.Comma) {
				break
			}
		}
		p.expect(token.Semicolon)
		return nil
	case token.KwConst:
		p.next()
		return p.parseConstSection()
	case token.KwResourceString:
		p.next()
		return p.parseConstSection()
	case token.KwType:
		p.next()
		return p.parseTypeSection()
	case token.KwVar, token.KwThreadvar:
		p.next()
		return p.parseVarSection(false)
	default:
		return []ast.Decl{p.parseProcDecl()}
	}
}

func (p *parser) parseConstSection() []ast.Decl {
	var decls []ast.Decl
	for p.startsConstDecl() {
		d := &ast.ConstDecl{At: p.pos()}
		d.Name = p.ident()
		if p.at(token.Colon) {
			p.next()
			d.Type = p.parseType()
		}
		p.expect(token.Eq)
		d.Value = p.parseExpr(false)
		p.expect(token.Semicolon)
		decls = append(decls, d)
	}
	return decls
}

// startsConstDecl reports whether a const declaration begins here.  The next
// token must be a name followed by `=` or `:` (and must not start a new
// declaration section).
func (p *parser) startsConstDecl() bool {
	if !p.isIdentLike() && p.kind() != token.Ident {
		return false
	}
	k1, k2 := p.kindAt(1), p.kindAt(2)
	if k1 == token.Eq || k1 == token.Colon && k2 == token.Eq || k1 == token.Colon {
		return true
	}
	return false
}

func (p *parser) parseTypeSection() []ast.Decl {
	var decls []ast.Decl
	for {
		// attributes may precede a type name: `[Serializable] My = class ...`
		p.parseAttributes()
		if !p.isIdentLike() && !p.at(token.Ident) {
			break
		}
		// stop if this is not `Name =` or `Name<T> =`
		save := p.i
		name := p.ident()
		var args []string
		if p.at(token.Lt) {
			p.next()
			for {
				args = append(args, p.identName())
				if !p.accept(token.Comma) {
					break
				}
			}
			p.expect(token.Gt)
		}
		if !p.at(token.Eq) {
			p.i = save
			break
		}
		p.expect(token.Eq)
		// `class`/`record`/`interface` may follow, or a type expression
		d := &ast.TypeDef{At: p.pos(), Name: name, Args: args}
		d.Type = p.parseType()
		p.expect(token.Semicolon)
		decls = append(decls, d)
	}
	return decls
}

// parseVarSection parses `name, name: T := init;` declarations.
func (p *parser) parseVarSection(isConst bool) []ast.Decl {
	var decls []ast.Decl
	for p.startsVarDecl() {
		v := &ast.VarDecl{At: p.pos(), Const: isConst}
		if p.at(token.LParen) {
			// `(a, b) := tuple` declaration
			p.next()
			var names []string
			for {
				p.accept(token.KwVar)
				names = append(names, p.identName())
				if !p.accept(token.Comma) {
					break
				}
			}
			p.expect(token.RParen)
			p.expect(token.ColonEq)
			d := &ast.TupleAssignDecl{At: p.pos(), Names: names, Value: p.parseExpr(false)}
			if !p.accept(token.Semicolon) {
				p.fatalf("expected ';' after declaration, found %s", p.cur())
			}
			decls = append(decls, d)
			continue
		}
		v.Names = append(v.Names, p.identName())
		for p.accept(token.Comma) {
			v.Names = append(v.Names, p.identName())
		}
		if p.at(token.Colon) {
			p.next()
			v.Type = p.parseType()
		}
		for p.accept(token.ColonEq) {
			v.Inits = append(v.Inits, p.parseExpr(false))
		}
		if len(v.Inits) == 0 && v.Type == nil {
			p.fatalf("expected type or initializer in variable declaration")
		}
		if !p.accept(token.Semicolon) {
			if !p.atAny(token.KwConst, token.KwType, token.KwVar, token.KwThreadvar) {
				p.fatalf("expected ';' after variable declaration, found %s", p.cur())
			}
		}
		decls = append(decls, v)
	}
	return decls
}

func (p *parser) startsVarDecl() bool {
	if p.at(token.LParen) {
		// `(a, b) := ...` only when it looks like a tuple declaration
		depth := 0
		for j := p.i + 1; j < len(p.toks); j++ {
			switch p.toks[j].Kind {
			case token.LParen:
				depth++
			case token.RParen:
				if depth == 0 {
					return j+1 < len(p.toks) && p.toks[j+1].Kind == token.ColonEq
				}
				depth--
			case token.EOF:
				return false
			}
		}
		return false
	}
	if !p.isIdentLike() && !p.at(token.Ident) {
		return false
	}
	switch p.kindAt(1) {
	case token.Comma, token.Colon, token.ColonEq:
		return true
	}
	return false
}

// ------------------------------------------------------------- statements --

// parseStmtList parses statements until one of the stop tokens is reached.
func (p *parser) parseStmtList(ends ...token.Kind) []ast.Stmt {
	var stmts []ast.Stmt
	for {
		if p.atEOF() {
			break
		}
		if p.atAny(ends...) {
			break
		}
		if p.at(token.Semicolon) {
			p.next()
			continue
		}
		p.ateSemi = false
		s := p.parseStmt()
		if s != nil {
			stmts = append(stmts, s)
		}
		if p.at(token.Semicolon) {
			p.next()
			continue
		}
		if p.ateSemi {
			continue
		}
		if p.atEOF() || p.atAny(ends...) {
			break
		}
		p.fatalf("expected ';' before %s", p.cur())
	}
	return stmts
}

// parseBlock parses `begin ... end`.
func (p *parser) parseBlock() *ast.Block {
	b := &ast.Block{At: p.pos()}
	p.expect(token.KwBegin)
	b.Stmts = p.parseStmtList(token.KwEnd, token.EOF)
	p.expect(token.KwEnd)
	return b
}

// parseBlockWithDecls parses optional declaration sections followed by a
// compound statement.
func (p *parser) parseBlockWithDecls() *ast.Block {
	b := &ast.Block{At: p.pos()}
	b.Decls = p.parseDeclList(false)
	inner := p.parseBlock()
	b.Stmts = inner.Stmts
	return b
}

// blockOrStmt parses either a compound statement or a single statement
// (PascalABC allows `if x then y := 1` without begin/end).
func (p *parser) parseBlockOrStmt() ast.Stmt {
	if p.at(token.KwBegin) {
		return p.parseBlock()
	}
	return p.parseStmt()
}

func (p *parser) parseStmt() ast.Stmt {
	// labeled statement: `10:` or `label:`
	if (p.at(token.Int) || p.isIdentLike()) && p.kindAt(1) == token.Colon {
		save := p.i
		lab := p.cur()
		var name string
		if p.at(token.Int) {
			name = p.next().Value
		} else {
			name = p.ident()
		}
		if p.at(token.Colon) {
			p.next()
			st := &ast.LabelStmt{At: lab.Start, Label: name, Stmt: p.parseStmt()}
			return st
		}
		p.i = save
	}

	switch p.kind() {
	case token.KwBegin:
		return p.parseBlock()
	case token.KwIf:
		return p.parseIf()
	case token.KwCase:
		return p.parseCase()
	case token.KwWhile:
		at := p.pos()
		p.next()
		s := &ast.WhileStmt{At: at}
		s.Cond = p.parseExpr(false)
		p.expect(token.KwDo)
		s.Body = p.parseBlockOrStmt()
		return s
	case token.KwRepeat:
		at := p.pos()
		p.next()
		s := &ast.RepeatStmt{At: at}
		s.Body = p.parseStmtList(token.KwUntil, token.EOF)
		p.expect(token.KwUntil)
		s.Cond = p.parseExpr(false)
		return s
	case token.KwLoop:
		at := p.pos()
		p.next()
		s := &ast.LoopStmt{At: at}
		s.Count = p.parseExpr(false)
		p.expect(token.KwDo)
		s.Body = p.parseBlockOrStmt()
		return s
	case token.KwFor:
		return p.parseFor()
	case token.KwForeach:
		return p.parseForeach()
	case token.KwWith:
		at := p.pos()
		p.next()
		s := &ast.WithStmt{At: at}
		for {
			s.Objs = append(s.Objs, p.parseExpr(false))
			if !p.accept(token.Comma) {
				break
			}
		}
		p.expect(token.KwDo)
		s.Body = p.parseBlockOrStmt()
		return s
	case token.KwTry:
		return p.parseTry()
	case token.KwRaise:
		at := p.pos()
		p.next()
		s := &ast.RaiseStmt{At: at}
		if !p.at(token.Semicolon) && !p.atAny(token.KwEnd, token.KwExcept, token.KwFinally) {
			s.X = p.parseExpr(false)
		}
		return s
	case token.KwGoto:
		at := p.pos()
		p.next()
		name := ""
		if p.at(token.Int) {
			name = p.next().Value
		} else {
			name = p.ident()
		}
		return &ast.GotoStmt{At: at, Label: name}
	case token.KwVar:
		return p.parseVarStatement()
	case token.KwConst, token.KwType, token.KwResourceString, token.KwThreadvar:
		// declarations inside a statement list
		before := p.i
		ds := p.parseDeclSection()
		if p.i == before {
			p.fatalf("expected declaration, found %s", p.cur())
		}
		p.ateSemi = true
		return &ast.LocalDecls{At: p.toks[before].Start, Decls: ds}
	case token.KwInherited:
		at := p.pos()
		p.next()
		call := &ast.CallExpr{At: at, Fn: &ast.IdentExpr{At: at, Name: []string{"inherited"}}}
		if p.isIdentLike() || p.at(token.Ident) {
			id := &ast.IdentExpr{At: p.pos(), Name: p.dottedName()}
			call.Fn = id
		}
		if p.at(token.LParen) {
			call.Args = p.parseArgs()
		}
		return &ast.CallStmt{At: at, X: call}
	case token.KwYield:
		at := p.pos()
		p.next()
		// `yield sequence expr`
		ye := &ast.YieldExpr{At: at, IsSequence: p.accept(token.KwSequence)}
		ye.X = p.parseExpr(false)
		return &ast.ExprStmt{At: at, X: ye}
	case token.KwLock:
		at := p.pos()
		p.next()
		s := &ast.WithStmt{At: at}
		s.Objs = append(s.Objs, p.parseExpr(false))
		p.expect(token.KwDo)
		s.Body = p.parseBlockOrStmt()
		return s
	case token.Semicolon, token.KwEnd, token.KwElse, token.KwUntil,
		token.KwExcept, token.KwFinally, token.EOF:
		return nil
	}

	if p.at(token.KwMatch) {
		return p.parseMatch()
	}

	// expression / assignment statement
	at := p.pos()
	e := p.parseExpr(false)
	switch p.kind() {
	case token.ColonEq, token.PlusEq, token.MinusEq, token.StarEq, token.SlashEq:
		op := p.next().Kind
		// tuple assignment `(a, b) := ...`
		if t, ok := e.(*ast.TupleExpr); ok {
			rhs := p.parseExpr(false)
			return &ast.TupleAssign{At: at, Targets: t.Elems, Op: op, Value: rhs}
		}
		if !isLValue(e) {
			p.errorf(at, "left side cannot be assigned to")
		}
		rhs := p.parseExpr(false)
		return &ast.Assign{At: at, Target: e, Op: op, Value: rhs}
	}
	return &ast.ExprStmt{At: at, X: e}
}

func isLValue(e ast.Expr) bool {
	switch e.(type) {
	case *ast.IdentExpr, *ast.FieldExpr, *ast.IndexExpr, *ast.DerefExpr:
		return true
	}
	return false
}

func (p *parser) parseIf() ast.Stmt {
	at := p.pos()
	p.expect(token.KwIf)
	s := &ast.IfStmt{At: at}
	s.Cond = p.parseExpr(false)
	p.expect(token.KwThen)
	s.Then = p.parseBlockOrStmt()
	if p.at(token.KwElse) {
		p.next()
		s.Else = p.parseBlockOrStmt()
	}
	return s
}

func (p *parser) parseCase() ast.Stmt {
	at := p.pos()
	p.expect(token.KwCase)
	s := &ast.CaseStmt{At: at}
	s.Value = p.parseExpr(false)
	p.expect(token.KwOf)
	for !p.at(token.KwElse) && !p.at(token.KwEnd) && !p.atEOF() {
		br := &ast.CaseBranch{At: p.pos()}
		for {
			br.Labels = append(br.Labels, p.parseRel())
			if !p.accept(token.Comma) {
				break
			}
		}
		p.expect(token.Colon)
		br.Body = p.parseBlockOrStmt()
		s.Branches = append(s.Branches, br)
		if !p.accept(token.Semicolon) {
			break
		}
	}
	if p.at(token.KwElse) {
		p.next()
		s.Else = p.parseStmtList(token.KwEnd, token.EOF)
	}
	p.expect(token.KwEnd)
	return s
}

func (p *parser) parseFor() ast.Stmt {
	at := p.pos()
	p.expect(token.KwFor)
	s := &ast.ForStmt{At: at}
	if p.at(token.KwVar) {
		p.next()
		v := &ast.VarDecl{At: p.pos()}
		v.Names = []string{p.identName()}
		if p.at(token.Colon) {
			p.next()
			v.Type = p.parseType()
		}
		s.VarDecl = v
	} else {
		s.Var = p.identName()
	}
	if p.at(token.KwIn) {
		// `for x in seq do` written with `for`
		p.next()
		s.Seq = p.parseExpr(false)
		s.IsForeach = true
		p.expect(token.KwDo)
		s.Body = p.parseBlockOrStmt()
		return s
	}
	p.expect(token.ColonEq)
	p.noTo++
	s.From = p.parseExpr(false)
	if p.at(token.KwTo) {
		p.next()
	} else if p.at(token.KwDownto) {
		p.next()
		s.Downto = true
	} else {
		p.fatalf("expected 'to' or 'downto', found %s", p.cur())
	}
	s.To = p.parseExpr(false)
	p.noTo--
	if p.at(token.KwStep) {
		p.next()
		s.HasStep = true
		s.Step = p.parseExpr(false)
	}
	p.expect(token.KwDo)
	s.Body = p.parseBlockOrStmt()
	return s
}

func (p *parser) parseForeach() ast.Stmt {
	at := p.pos()
	p.expect(token.KwForeach)
	s := &ast.ForeachStmt{At: at}
	// `foreach (a, b) in ...` / `foreach var (a, b) in ...`
	if p.at(token.LParen) || (p.at(token.KwVar) && p.kindAt(1) == token.LParen) {
		p.accept(token.KwVar)
		p.next() // (
		v := &ast.VarDecl{At: p.pos()}
		for {
			v.Names = append(v.Names, p.identName())
			if !p.accept(token.Comma) {
				break
			}
		}
		p.expect(token.RParen)
		s.VarDecl = v
	} else if p.at(token.KwVar) {
		p.next()
		v := &ast.VarDecl{At: p.pos()}
		v.Names = []string{p.identName()}
		if p.at(token.Colon) {
			p.next()
			v.Type = p.parseType()
		}
		s.VarDecl = v
	} else {
		s.Var = p.identName()
		if p.at(token.Colon) {
			p.next()
			v := &ast.VarDecl{At: p.pos(), Names: []string{s.Var}}
			v.Type = p.parseType()
			s.VarDecl = v
		}
	}
	p.expect(token.KwIn)
	s.Seq = p.parseExpr(false)
	if p.at(token.KwIndex) {
		p.next()
		s.IndexVar = p.identName()
	}
	p.expect(token.KwDo)
	s.Body = p.parseBlockOrStmt()
	return s
}

func (p *parser) parseTry() ast.Stmt {
	at := p.pos()
	p.expect(token.KwTry)
	s := &ast.TryStmt{At: at}
	s.Body = &ast.Block{At: p.pos(),
		Stmts: p.parseStmtList(token.KwExcept, token.KwFinally, token.KwEnd, token.EOF)}
	if p.at(token.KwExcept) {
		p.next()
		for p.at(token.KwOn) {
			h := &ast.Handler{At: p.pos()}
			p.next()
			// `on e: Exception do ...` or `on System.FormatException do ...`
			first := p.dottedName()
			if p.at(token.Colon) {
				p.next()
				h.Var = first[0]
				h.Type = p.dottedName()
			} else {
				h.Type = first
			}
			p.expect(token.KwDo)
			h.Body = p.parseBlockOrStmt()
			s.Handlers = append(s.Handlers, h)
			p.accept(token.Semicolon)
		}
		if len(s.Handlers) == 0 {
			s.Else = p.parseStmtList(token.KwFinally, token.KwEnd, token.EOF)
		}
		if p.at(token.KwFinally) {
			p.next()
			s.Finally = &ast.Block{At: p.pos(),
				Stmts: p.parseStmtList(token.KwEnd, token.EOF)}
		}
	}
	if p.at(token.KwFinally) {
		p.next()
		s.Finally = &ast.Block{At: p.pos(),
			Stmts: p.parseStmtList(token.KwEnd, token.EOF)}
	}
	if !p.at(token.KwEnd) && len(s.Handlers) == 0 && s.Finally == nil {
		p.fatalf("expected 'except' or 'finally' after 'try'")
	}
	p.expect(token.KwEnd)
	return s
}

func (p *parser) parseVarStatement() ast.Stmt {
	at := p.pos()
	p.expect(token.KwVar)
	// `(var a, b) := ...` or `var (a, b) := ...`
	if p.at(token.LParen) {
		p.next()
		var names []string
		for {
			p.accept(token.KwVar)
			names = append(names, p.identName())
			if !p.accept(token.Comma) {
				break
			}
		}
		p.expect(token.RParen)
		p.expect(token.ColonEq)
		v := p.parseExpr(false)
		return &ast.TupleAssignDecl{At: at, Names: names, Value: v}
	}
	v := &ast.VarDecl{At: at}
	v.Names = append(v.Names, p.identName())
	for p.accept(token.Comma) {
		v.Names = append(v.Names, p.identName())
	}
	if p.at(token.Colon) {
		p.next()
		v.Type = p.parseType()
	}
	for p.accept(token.ColonEq) {
		v.Inits = append(v.Inits, p.parseExpr(false))
	}
	if len(v.Inits) == 0 && v.Type == nil {
		p.fatalf("expected type or initializer in var declaration")
	}
	p.expect(token.Semicolon)
	p.ateSemi = true
	return v
}
