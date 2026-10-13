// Package token defines lexical tokens of the PascalABC language.
package token

import "fmt"

// Kind enumerates token kinds.
type Kind uint16

const (
	Invalid Kind = iota
	EOF
	Ident      // identifier (or keyword-misused-as-identifier)
	Underscore // a lone '_'
	Int        // 123, 1_000
	BigInt     // 123bi
	Hex        // $FF
	Float      // 1.5, 1e10, 1.5e-3
	Str        // 'text' (possibly length 1 -> char)
	MultiStr   // ''' multi line '''
	FmtStr     // $'format {0}'
	CharLit    // #65, #$41
	Directive  // {$mode ...} and friends (kept as trivia)
	ShortProg  // ##
	ShortSFProg

	// punctuation / operators
	LParen
	RParen
	LBracket
	RBracket
	Comma
	Semicolon
	Colon
	ColonEq // :=
	PlusEq
	MinusEq
	StarEq
	SlashEq
	Dot
	DotDot
	At    // @
	Caret // ^
	Plus
	Minus
	Star
	Slash
	StarStar       // **
	Question       // ?
	QuestionDot    // ?.
	QuestionSq     // ?[
	DblQuestion    // ??
	Amp            // &
	Pipe           // |
	CSharpOr       // ||
	Arrow          // -> or →
	BackslashParen // \(
	Eq
	Neq
	Lt
	Gt
	Lte
	Gte
	LtLt // <<
	GtGt // >>

	// keywords (from PascalABC's Keywords.cs)
	KwBegin
	KwEnd
	KwIf
	KwThen
	KwElse
	KwCase
	KwOf
	KwWhile
	KwDo
	KwRepeat
	KwUntil
	KwFor
	KwTo
	KwDownto
	KwStep
	KwGoto
	KwLabel
	KwVar
	KwConst
	KwType
	KwProcedure
	KwFunction
	KwUses
	KwUnit
	KwInterface
	KwImplementation
	KwInitialization
	KwFinalization
	KwProgram
	KwLibrary
	KwNamespace
	KwRecord
	KwClass
	KwAuto
	KwSealed
	KwPartial
	KwStatic
	KwPrivate
	KwProtected
	KwPublic
	KwInternal
	KwProperty
	KwRead
	KwWrite
	KwIndex
	KwName
	KwConstructor
	KwDestructor
	KwOperator
	KwInherited
	KwNew
	KwNil
	KwNot
	KwAnd
	KwOr
	KwXor
	KwDiv
	KwMod
	KwShl
	KwShr
	KwAs
	KwIn
	KwIs
	KwImplicit
	KwExplicit
	KwSizeOf
	KwTypeOf
	KwWhere
	KwArray
	KwSet
	KwFile
	KwTry
	KwExcept
	KwFinally
	KwRaise
	KwOn
	KwLock
	KwWith
	KwForeach
	KwLoop
	KwSequence
	KwYield
	KwMatch
	KwWhen
	KwAsync
	KwAwait
	KwAbstract
	KwVirtual
	KwOverride
	KwOverload
	KwReintroduce
	KwForward
	KwExtensionMethod
	KwExternal
	KwParams
	KwEvent
	KwDefault
	KwTemplate
	KwExports
	KwResourceString
	KwThreadvar
	KwBeginEndAlias // placeholder, unused
)

type tinfo struct {
	Kind    Kind
	Text    string
	Keyword bool
}

// table maps kinds to their spelling.
var table = map[Kind]tinfo{
	Invalid:     {Invalid, "<invalid>", false},
	EOF:         {EOF, "EOF", false},
	Ident:       {Ident, "identifier", false},
	Underscore:  {Underscore, "_", false},
	Int:         {Int, "integer literal", false},
	BigInt:      {BigInt, "bigint literal", false},
	Hex:         {Hex, "hex literal", false},
	Float:       {Float, "real literal", false},
	Str:         {Str, "string literal", false},
	MultiStr:    {MultiStr, "multiline string", false},
	FmtStr:      {FmtStr, "format string", false},
	CharLit:     {CharLit, "char literal", false},
	Directive:   {Directive, "compiler directive", false},
	ShortProg:   {ShortProg, "##", false},
	ShortSFProg: {ShortSFProg, "###", false},

	LParen:         {LParen, "(", false},
	RParen:         {RParen, ")", false},
	LBracket:       {LBracket, "[", false},
	RBracket:       {RBracket, "]", false},
	Comma:          {Comma, ",", false},
	Semicolon:      {Semicolon, ";", false},
	Colon:          {Colon, ":", false},
	ColonEq:        {ColonEq, ":=", false},
	PlusEq:         {PlusEq, "+=", false},
	MinusEq:        {MinusEq, "-=", false},
	StarEq:         {StarEq, "*=", false},
	SlashEq:        {SlashEq, "/=", false},
	Dot:            {Dot, ".", false},
	DotDot:         {DotDot, "..", false},
	At:             {At, "@", false},
	Caret:          {Caret, "^", false},
	Plus:           {Plus, "+", false},
	Minus:          {Minus, "-", false},
	Star:           {Star, "*", false},
	Slash:          {Slash, "/", false},
	StarStar:       {StarStar, "**", false},
	Question:       {Question, "?", false},
	QuestionDot:    {QuestionDot, "?.", false},
	QuestionSq:     {QuestionSq, "?[", false},
	DblQuestion:    {DblQuestion, "??", false},
	Amp:            {Amp, "&", false},
	Pipe:           {Pipe, "|", false},
	CSharpOr:       {CSharpOr, "||", false},
	Arrow:          {Arrow, "->", false},
	BackslashParen: {BackslashParen, "\\(", false},
	Eq:             {Eq, "=", false},
	Neq:            {Neq, "<>", false},
	Lt:             {Lt, "<", false},
	Gt:             {Gt, ">", false},
	Lte:            {Lte, "<=", false},
	Gte:            {Gte, ">=", false},

	KwBegin:           {KwBegin, "begin", true},
	KwEnd:             {KwEnd, "end", true},
	KwIf:              {KwIf, "if", true},
	KwThen:            {KwThen, "then", true},
	KwElse:            {KwElse, "else", true},
	KwCase:            {KwCase, "case", true},
	KwOf:              {KwOf, "of", true},
	KwWhile:           {KwWhile, "while", true},
	KwDo:              {KwDo, "do", true},
	KwRepeat:          {KwRepeat, "repeat", true},
	KwUntil:           {KwUntil, "until", true},
	KwFor:             {KwFor, "for", true},
	KwTo:              {KwTo, "to", true},
	KwDownto:          {KwDownto, "downto", true},
	KwStep:            {KwStep, "step", true},
	KwGoto:            {KwGoto, "goto", true},
	KwLabel:           {KwLabel, "label", true},
	KwVar:             {KwVar, "var", true},
	KwConst:           {KwConst, "const", true},
	KwType:            {KwType, "type", true},
	KwProcedure:       {KwProcedure, "procedure", true},
	KwFunction:        {KwFunction, "function", true},
	KwUses:            {KwUses, "uses", true},
	KwUnit:            {KwUnit, "unit", true},
	KwInterface:       {KwInterface, "interface", true},
	KwImplementation:  {KwImplementation, "implementation", true},
	KwInitialization:  {KwInitialization, "initialization", true},
	KwFinalization:    {KwFinalization, "finalization", true},
	KwProgram:         {KwProgram, "program", true},
	KwLibrary:         {KwLibrary, "library", true},
	KwNamespace:       {KwNamespace, "namespace", true},
	KwRecord:          {KwRecord, "record", true},
	KwClass:           {KwClass, "class", true},
	KwAuto:            {KwAuto, "auto", true},
	KwSealed:          {KwSealed, "sealed", true},
	KwPartial:         {KwPartial, "partial", true},
	KwStatic:          {KwStatic, "static", true},
	KwPrivate:         {KwPrivate, "private", true},
	KwProtected:       {KwProtected, "protected", true},
	KwPublic:          {KwPublic, "public", true},
	KwInternal:        {KwInternal, "internal", true},
	KwProperty:        {KwProperty, "property", true},
	KwRead:            {KwRead, "read", true},
	KwWrite:           {KwWrite, "write", true},
	KwIndex:           {KwIndex, "index", true},
	KwName:            {KwName, "name", true},
	KwConstructor:     {KwConstructor, "constructor", true},
	KwDestructor:      {KwDestructor, "destructor", true},
	KwOperator:        {KwOperator, "operator", true},
	KwInherited:       {KwInherited, "inherited", true},
	KwNew:             {KwNew, "new", true},
	KwNil:             {KwNil, "nil", true},
	KwNot:             {KwNot, "not", true},
	KwAnd:             {KwAnd, "and", true},
	KwOr:              {KwOr, "or", true},
	KwXor:             {KwXor, "xor", true},
	KwDiv:             {KwDiv, "div", true},
	KwMod:             {KwMod, "mod", true},
	KwShl:             {KwShl, "shl", true},
	KwShr:             {KwShr, "shr", true},
	KwAs:              {KwAs, "as", true},
	KwIn:              {KwIn, "in", true},
	KwIs:              {KwIs, "is", true},
	KwImplicit:        {KwImplicit, "implicit", true},
	KwExplicit:        {KwExplicit, "explicit", true},
	KwSizeOf:          {KwSizeOf, "sizeof", true},
	KwTypeOf:          {KwTypeOf, "typeof", true},
	KwWhere:           {KwWhere, "where", true},
	KwArray:           {KwArray, "array", true},
	KwSet:             {KwSet, "set", true},
	KwFile:            {KwFile, "file", true},
	KwTry:             {KwTry, "try", true},
	KwExcept:          {KwExcept, "except", true},
	KwFinally:         {KwFinally, "finally", true},
	KwRaise:           {KwRaise, "raise", true},
	KwOn:              {KwOn, "on", true},
	KwLock:            {KwLock, "lock", true},
	KwWith:            {KwWith, "with", true},
	KwForeach:         {KwForeach, "foreach", true},
	KwLoop:            {KwLoop, "loop", true},
	KwSequence:        {KwSequence, "sequence", true},
	KwYield:           {KwYield, "yield", true},
	KwMatch:           {KwMatch, "match", true},
	KwWhen:            {KwWhen, "when", true},
	KwAsync:           {KwAsync, "async", true},
	KwAwait:           {KwAwait, "await", true},
	KwAbstract:        {KwAbstract, "abstract", true},
	KwVirtual:         {KwVirtual, "virtual", true},
	KwOverride:        {KwOverride, "override", true},
	KwOverload:        {KwOverload, "overload", true},
	KwReintroduce:     {KwReintroduce, "reintroduce", true},
	KwForward:         {KwForward, "forward", true},
	KwExtensionMethod: {KwExtensionMethod, "extensionmethod", true},
	KwExternal:        {KwExternal, "external", true},
	KwParams:          {KwParams, "params", true},
	KwEvent:           {KwEvent, "event", true},
	KwDefault:         {KwDefault, "default", true},
	KwTemplate:        {KwTemplate, "template", true},
	KwExports:         {KwExports, "exports", true},
	KwResourceString:  {KwResourceString, "resourcestring", true},
	KwThreadvar:       {KwThreadvar, "threadvar", true},
}

// keywords maps lower-cased spelling to kind.
var keywords = func() map[string]Kind {
	m := make(map[string]Kind, 64)
	for k, v := range table {
		if v.Keyword {
			m[v.Text] = k
		}
	}
	// break/continue/exit are ordinary identifiers in PascalABC, but they are
	// reserved for statement purposes in this compiler; keep them as keywords
	// so the parser can recognise `break`/`continue`/`exit` statements directly.
	return m
}()

// Lookup returns the keyword kind for an identifier spelling, or Ident.
func Lookup(name string) Kind {
	if k, ok := keywords[name]; ok {
		return k
	}
	return Ident
}

// Text returns the canonical spelling of a kind.
func Text(k Kind) string {
	if s, ok := table[k]; ok {
		return s.Text
	}
	return fmt.Sprintf("kind(%d)", k)
}

// IsKeyword reports whether k is a keyword kind.
func IsKeyword(k Kind) bool {
	s, ok := table[k]
	return ok && s.Keyword
}

// IsOperator reports whether k is an operator/punctuation token.
func IsOperator(k Kind) bool {
	_, ok := table[k]
	return ok && !s(k).Keyword && k >= LParen && k <= Gte
}

func s(k Kind) tinfo { return table[k] }

// String implements fmt.Stringer for diagnostics.
func (k Kind) String() string { return Text(k) }

// Pos is a byte-offset/line/column triple inside a source file.
type Pos struct {
	Line   int // 1-based
	Col    int // 1-based, counted in runes
	Offset int // 0-based byte offset
}

func (p Pos) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

// Token is a lexical token.
type Token struct {
	Kind  Kind
	Text  string // raw source text of the token
	Value string // decoded value for Str/MultiStr/FmtStr/CharLit; numeric text otherwise
	Start Pos
	End   Pos
}

// String renders the token for diagnostics.
func (t Token) String() string {
	switch t.Kind {
	case EOF:
		return "end of file"
	case Ident:
		return "identifier '" + t.Text + "'"
	case Str, MultiStr:
		return "string literal"
	case Int, Hex, Float, BigInt:
		return "literal '" + t.Text + "'"
	default:
		return "'" + t.Text + "'"
	}
}
