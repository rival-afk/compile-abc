package parser

import (
	"strings"

	"abc/internal/ast"
	"abc/internal/token"
)

// parseMatch parses `match e with <pattern>: <stmt>; ... [else ...] end`.
func (p *parser) parseMatch() ast.Stmt {
	at := p.pos()
	p.expect(token.KwMatch)
	s := &ast.MatchStmt{At: at}
	s.Value = p.parseExpr(false)
	p.expect(token.KwWith)
	for {
		if p.at(token.KwElse) || p.at(token.KwEnd) || p.atEOF() {
			break
		}
		c := &ast.MatchCase{At: p.pos()}
		for {
			c.Pats = append(c.Pats, p.parsePattern(false))
			if !p.accept(token.Comma) {
				break
			}
		}
		if p.at(token.KwWhen) {
			p.next()
			c.When = p.parseExpr(false)
		}
		p.expect(token.Colon)
		c.Body = p.parseStmt()
		s.Cases = append(s.Cases, c)
		if !p.accept(token.Semicolon) {
			break
		}
	}
	p.accept(token.Semicolon)
	if p.at(token.KwElse) {
		p.next()
		s.Else = &ast.Block{At: p.pos(),
			Stmts: p.parseStmtList(token.KwEnd, token.EOF)}
	}
	p.expect(token.KwEnd)
	return s
}

// parsePattern parses a single pattern.  When inArgs is set a bare
// identifier binds a variable (as inside `T(x, y)`), otherwise it denotes a
// constant.
func (p *parser) parsePattern(inArgs bool) *ast.Pattern {
	at := p.pos()
	switch p.kind() {
	case token.Underscore:
		p.next()
		return &ast.Pattern{At: at, Kind: ast.PatWildcard}
	case token.KwVar:
		p.next()
		return p.parseVarPattern(at)
	case token.LParen:
		return p.parseTuplePattern(at)
	case token.LBracket:
		return p.parseCollectionPattern(at)
	}
	if p.isIdentLike() || p.at(token.Ident) {
		path := p.dottedName()
		name := strings.Join(path, ".")
		if p.at(token.Lt) {
			p.skipTypeArgs()
		}
		if p.at(token.LParen) {
			return &ast.Pattern{At: at, Kind: ast.PatDeconstruct,
				Name: name, Elems: p.parsePatternArgs()}
		}
		if inArgs {
			el := &ast.Pattern{At: at, Kind: ast.PatVar, Name: name}
			if p.at(token.Colon) {
				p.next()
				el.Type = p.parseType()
			}
			return el
		}
		return &ast.Pattern{At: at, Kind: ast.PatConst, Name: name}
	}
	if p.startsExpr() {
		return &ast.Pattern{At: at, Kind: ast.PatConst, Lit: p.parseUnary()}
	}
	p.fatalf("expected pattern, found %s", p.cur())
	return nil
}

// parseVarPattern parses the tail of `var name [: T]`.
func (p *parser) parseVarPattern(at token.Pos) *ast.Pattern {
	el := &ast.Pattern{At: at, Kind: ast.PatVar, IsVar: true}
	el.Name = p.identName()
	if p.at(token.Colon) {
		p.next()
		el.Type = p.parseType()
	}
	return el
}

// parseTuplePattern parses `(a, b, _)`.
func (p *parser) parseTuplePattern(at token.Pos) *ast.Pattern {
	p.expect(token.LParen)
	el := &ast.Pattern{At: at, Kind: ast.PatTuple}
	for !p.at(token.RParen) && !p.atEOF() {
		el.Elems = append(el.Elems, p.parsePattern(true))
		if !p.accept(token.Comma) {
			break
		}
	}
	p.expect(token.RParen)
	return el
}

// parseCollectionPattern parses `[a, b, ..]`.
func (p *parser) parseCollectionPattern(at token.Pos) *ast.Pattern {
	p.expect(token.LBracket)
	el := &ast.Pattern{At: at, Kind: ast.PatCollection}
	for !p.at(token.RBracket) && !p.atEOF() {
		if p.at(token.DotDot) {
			p.next()
			el.Elems = append(el.Elems, &ast.Pattern{At: p.pos(), Kind: ast.PatGap})
		} else {
			el.Elems = append(el.Elems, p.parsePattern(true))
		}
		if !p.accept(token.Comma) {
			break
		}
	}
	p.expect(token.RBracket)
	return el
}

// parsePatternArgs parses the out-parameter list of `T(...)`; the items are
// separated by `,` or `;`.
func (p *parser) parsePatternArgs() []*ast.Pattern {
	p.expect(token.LParen)
	var out []*ast.Pattern
	for !p.at(token.RParen) && !p.atEOF() {
		out = append(out, p.parsePattern(true))
		if !p.accept(token.Comma) && !p.accept(token.Semicolon) {
			break
		}
	}
	p.expect(token.RParen)
	return out
}

// skipTypeArgs consumes a balanced `<...>` group.
func (p *parser) skipTypeArgs() {
	p.expect(token.Lt)
	depth := 1
	for !p.atEOF() {
		switch p.kind() {
		case token.Lt:
			depth++
		case token.Gt:
			depth--
		}
		p.next()
		if depth == 0 {
			return
		}
	}
	p.fatalf("expected '>', found %s", p.cur())
}

// parseTypeArgs parses `<T, U>`.
func (p *parser) parseTypeArgs() []ast.TypeExpr {
	p.expect(token.Lt)
	var args []ast.TypeExpr
	for !p.at(token.Gt) && !p.atEOF() {
		args = append(args, p.parseType())
		if !p.accept(token.Comma) {
			break
		}
	}
	p.expect(token.Gt)
	return args
}
