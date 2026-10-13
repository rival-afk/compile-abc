// Package ast defines the abstract syntax tree of PascalABC programs.
package ast

import "abc/internal/token"

// Node is the common interface of all syntax nodes.
type Node interface {
	Pos() token.Pos
}

type node struct{ At token.Pos }

// Pos implements Node.
func (n node) Pos() token.Pos { return n.At }

// Expr is an expression node.
type Expr interface {
	Node
	exprNode()
}

// Stmt is a statement node.
type Stmt interface {
	Node
	stmtNode()
}

// Decl is a declaration node.
type Decl interface {
	Node
	declNode()
}

// TypeExpr is a type expression node.
type TypeExpr interface {
	Node
	typeNode()
}

// ---------------------------------------------------------------- programs --

// Program is a program module (a .pas file with an optional program header).
type Program struct {
	node
	Name  string // empty for short programs (`##`)
	Uses  []*UsesItem
	Decls []Decl
	Body  *Block
	Short bool
	// Unit/interface parts (nil for plain programs)
	IsUnit           bool
	UnitName         []string
	InterfaceDecls   []Decl
	ImplDecls        []Decl
	Init             *Block
	Fin              *Block
	ZeroBasedStrings bool
}

// UsesItem is one entry of a uses clause.
type UsesItem struct {
	node
	Path []string // unit name, possibly dotted
	File string   // optional `in 'file.pas'`
}

// ------------------------------------------------------------------ types --

// TypeRef references a named type, optionally with generic arguments.
type TypeRef struct {
	node
	Path []string
	Args []TypeExpr
}

// RangeSpec is a single `[lo..hi]` bound of an array type.
type RangeSpec struct {
	node
	Lo Expr
	Hi Expr
}

// ArrayType is `array[r1, r2] of T` or the dynamic `array of T`.
type ArrayType struct {
	node
	Ranges []*RangeSpec // nil/empty for dynamic arrays
	Elem   TypeExpr
}

// SequenceType is `sequence of T`.
type SequenceType struct {
	node
	Elem TypeExpr
}

// SetType is `set of T`.
type SetType struct {
	node
	Elem TypeExpr
}

// PointerType is `^T`.
type PointerType struct {
	node
	Elem TypeExpr
}

// StringType is `string` or `string[n]`.
type StringType struct {
	node
	Len Expr // nil for plain string
}

// FileType is `file of T` or `file`.
type FileType struct {
	node
	Elem TypeExpr // nil for untyped file
}

// EnumType is `(a, b, c)`.
type EnumType struct {
	node
	Members []string
}

// ParamMode describes how a parameter is passed.
type ParamMode int

const (
	ParamValue ParamMode = iota
	ParamVar
	ParamOut
	ParamConst
	ParamParams
)

// Param is a formal parameter declaration.
type Param struct {
	node
	Names  []string
	Type   TypeExpr
	Mode   ParamMode
	Value  Expr     // default value
	Nested []*Param // tuple parameters: `\(a, b)`
}

// ProcType is a procedural type `procedure(a: T): U`.
type ProcType struct {
	node
	IsFunc bool
	Params []*Param
	Result TypeExpr
}

// RecordField is a field of a record/class.
type RecordField struct {
	node
	Names []string
	Type  TypeExpr
	Init  Expr
}

// Visibility of a class member.
type Visibility int

const (
	VisDefault Visibility = iota
	VisPrivate
	VisProtected
	VisPublic
	VisInternal
	VisPublished
)

// RecordType is `record ... end` or `class ... end`.
type RecordType struct {
	node
	IsClass     bool
	IsAuto      bool // `auto class`
	IsSealed    bool
	IsPartial   bool
	IsStatic    bool
	IsInterface bool
	Base        []string
	Members     []Decl
}

// PropertyDecl is a class/record property.
type PropertyDecl struct {
	node
	Name    string
	Type    TypeExpr
	Read    string
	Write   string
	Index   []string
	Default Expr
}

// -------------------------------------------------------------------- decls

// VarDecl is a `var` declaration.
type VarDecl struct {
	node
	Names  []string
	Type   TypeExpr // may be nil when every variable has an initializer
	Inits  []Expr
	Const  bool // used for const sections too (Value in Inits)
	Extern bool
}

// ConstDecl is a `const` declaration.
type ConstDecl struct {
	node
	Name  string
	Type  TypeExpr
	Value Expr
}

// TypeDef is a `type X = ...` declaration.
type TypeDef struct {
	node
	Name string
	Type TypeExpr
	Args []string // generic parameters (`type T<K> = ...`)
}

// ProcKind distinguishes subprogram flavours.
type ProcKind int

const (
	ProcProcedure ProcKind = iota
	ProcFunction
	ProcConstructor
	ProcDestructor
	ProcOperator
)

// ProcDecl is a procedure/function/constructor/destructor declaration.
type ProcDecl struct {
	node
	Kind          ProcKind
	IsClassMethod bool
	BodyExpr      Expr // `procedure p := expr;`
	ExternalName  string
	Name          []string // dotted for qualified names
	TypeArgs      []string // `function f<T>: ...`
	Params        []*Param
	Result        TypeExpr
	Body          *Block // nil for forward/external declarations
	LocalDecls    []Decl // nested var/const/type/procedure declarations
	IsStatic      bool
	// modifiers
	IsVirtual     bool
	IsOverride    bool
	IsAbstract    bool
	IsOverload    bool
	IsForward     bool
	IsExternal    bool
	IsReintroduce bool
	IsExtension   bool
	Visibility    Visibility
	// operator overloading
	OperatorTok token.Kind
	// class members live in RecordType.Members
}

// ---------------------------------------------------------------- statements

// Block is a compound statement with optional local declarations.
type Block struct {
	node
	Decls []Decl
	Stmts []Stmt
}

func (*Block) stmtNode() {}

// Assign is `target := value` (Op may be `+=` etc).
type Assign struct {
	node
	Target Expr
	Op     token.Kind // ColonEq, PlusEq, MinusEq, StarEq, SlashEq
	Value  Expr
}

// IfStmt is a conditional statement.
type IfStmt struct {
	node
	Cond Expr
	Then Stmt
	Else Stmt // may be nil
}

// WhileStmt is a while loop.
type WhileStmt struct {
	node
	Cond Expr
	Body Stmt
}

// LoopStmt is `loop n do stmt` - repeat stmt n times.
type LoopStmt struct {
	node
	Count Expr
	Body  Stmt
}

// RepeatStmt is a repeat..until loop.
type RepeatStmt struct {
	node
	Body []Stmt
	Cond Expr
}

// ForStmt is a for loop.
type ForStmt struct {
	node
	Var       string // loop variable name; empty for `for var i := ...`
	VarDecl   *VarDecl
	From      Expr
	To        Expr
	Downto    bool
	HasStep   bool
	Step      Expr
	Body      Stmt
	IsForeach bool
	Seq       Expr
}

// ForeachStmt is `for x in seq do`.
type ForeachStmt struct {
	node
	Var      string
	VarDecl  *VarDecl
	Seq      Expr
	IndexVar string // `foreach x in a index i do`
	Body     Stmt
}

// CaseBranch is one `label: stmt` arm of a case statement.
type CaseBranch struct {
	node
	Labels []Expr // ranges appear as Binary(DotDot) expressions
	Body   Stmt
}

// CaseStmt is a case statement.
type CaseStmt struct {
	node
	Value    Expr
	Branches []*CaseBranch
	Else     []Stmt
}

// CallStmt is a statement that is just an expression.
type CallStmt struct {
	node
	X Expr
}

// BreakStmt is `break`.
type BreakStmt struct{ node }

// ContinueStmt is `continue`.
type ContinueStmt struct{ node }

// ExitStmt is `exit` or `exit(value)`.
type ExitStmt struct {
	node
	Value Expr
}

// GotoStmt is `goto label`.
type GotoStmt struct {
	node
	Label string
}

// LabelStmt prefixes a statement with `label:`.
type LabelStmt struct {
	node
	Label string
	Stmt  Stmt
}

// Handler is an `on E: T do stmt` arm of an except section.
type Handler struct {
	node
	Var  string
	Type []string
	Body Stmt
}

// TryStmt is try..except/finally.
type TryStmt struct {
	node
	Body     *Block
	Handlers []*Handler
	Else     []Stmt
	Finally  *Block
}

// RaiseStmt is `raise` or `raise e`.
type RaiseStmt struct {
	node
	X Expr
}

// WithStmt is `with a, b do stmt`.
type WithStmt struct {
	node
	Objs []Expr
	Body Stmt
}

// EmptyStmt is a stray `;`.
type EmptyStmt struct{ node }

// LocalDecls carries declarations written inside a statement list
// (`const`, `type`, `var` sections between statements).
type LocalDecls struct {
	node
	Decls []Decl
}

// ExprStmt is an expression evaluated for its side effects.
type ExprStmt struct {
	node
	X Expr
}

// TupleAssign assigns to several targets: `(a, b) := f()`.
type TupleAssign struct {
	node
	Targets []Expr
	Op      token.Kind
	Value   Expr
}

// TupleAssignDecl declares several variables from a tuple:
// `var (a, b) := f()`.
type TupleAssignDecl struct {
	node
	Names []string
	Value Expr
}

// MatchStmt is `match e with <cases> [else ...] end`.
type MatchStmt struct {
	node
	Value Expr
	Cases []*MatchCase
	Else  Stmt // may be nil
}

// MatchCase is one arm of a `match`: `pattern [when cond]: stmt`.
type MatchCase struct {
	node
	Pats []*Pattern
	When Expr // may be nil
	Body Stmt
}

// PatternKind enumerates the forms a pattern may take.
type PatternKind int

const (
	PatWildcard    PatternKind = iota // `_`
	PatVar                            // `x`, `var x`, `x: T`
	PatConst                          // literal or constant name
	PatDeconstruct                    // `Person(a, b)`
	PatTuple                          // `(a, b)`
	PatCollection                     // `[a, b, ..]`
	PatGap                            // `..` inside a collection pattern
)

// Pattern is a pattern used by `match` and by `is T(...)` type tests.
type Pattern struct {
	node
	Kind  PatternKind
	Name  string   // bound name (PatVar) or constructor name (PatDeconstruct)
	IsVar bool     // explicit `var`
	Type  TypeExpr // optional `x: T` annotation
	Lit   Expr     // PatConst
	Elems []*Pattern
}

// AssignExpr is `x := y` (or `x += y`) written where an expression is
// expected, e.g. inside `( ... )`.
type AssignExpr struct {
	node
	Target Expr
	Op     token.Kind // ColonEq, PlusEq, MinusEq, StarEq, SlashEq
	Value  Expr
}

// GenericRef is `Name<T, U>` (also written `Name&<T, U>`).
type GenericRef struct {
	node
	X    Expr
	Args []TypeExpr
}

// YieldExpr is `yield expr` / `yield sequence expr`.
type YieldExpr struct {
	node
	X          Expr
	IsSequence bool
}

// ---------------------------------------------------------------- expressions

// IdentExpr is a (possibly qualified) identifier.
type IdentExpr struct {
	node
	Name []string
}

// IntLit is an integer literal (`123`, `$FF`, `#65`).
type IntLit struct {
	node
	Value  int64
	Text   string
	IsChar bool
}

// BigIntLit is a `123bi` literal; kept as a string until code generation.
type BigIntLit struct {
	node
	Value string
}

// RealLit is a real literal.
type RealLit struct {
	node
	Value float64
	Text  string
}

// StrLit is a string literal.  A one-character literal may also mean a char.
type StrLit struct {
	node
	Value     string
	Format    bool // $'...'
	Multiline bool
}

// NilLit is `nil`.
type NilLit struct{ node }

// BoolLit is `true`/`false` (resolved semantically).
type BoolLit struct {
	node
	Value bool
	Name  string
}

// BinaryExpr is a binary operation.
type BinaryExpr struct {
	node
	Op    token.Kind
	Left  Expr
	Right Expr
}

// UnaryExpr is a prefix operation.
type UnaryExpr struct {
	node
	Op token.Kind
	X  Expr
}

// CallArg is one argument of a call.
type CallArg struct {
	node
	Name     string // named argument, may be empty
	Value    Expr
	Type     TypeExpr // for `default(T)` style arguments
	TypeOnly bool
}

// CallExpr is `f(a, b)`.
type CallExpr struct {
	node
	Fn   Expr
	Args []*CallArg
}

// IndexDim is one dimension of an index or slice expression: `a[i]`,
// `a[lo:hi:step]`, `a[:,k]`, ...
type IndexDim struct {
	Lo      Expr // nil when omitted: `a[:hi]`
	Hi      Expr // nil unless the dimension contains a `:`
	Step    Expr
	IsSlice bool // true when the dimension contains a `:`
}

// IndexExpr is `a[i]`, `a[i, j]` or a slice `a[lo:hi]`.
type IndexExpr struct {
	node
	X    Expr
	Dims []IndexDim
}

// FieldExpr is `a.b`.
type FieldExpr struct {
	node
	X    Expr
	Name string
}

// DerefExpr is `p^`.
type DerefExpr struct {
	node
	X Expr
}

// AddrExpr is `@x`.
type AddrExpr struct {
	node
	X Expr
}

// TypeCastExpr is `x as T`.
type TypeCastExpr struct {
	node
	X    Expr
	Type TypeExpr
}

// TypeTestExpr is `x is T`.
type TypeTestExpr struct {
	node
	X       Expr
	Type    TypeExpr
	Pattern *Pattern // set for `x is T(a, b)`, `x is (a, b)`, `x is [a, b]`
}

// NewExpr is `new T(...)`.
type NewExpr struct {
	node
	Type  TypeExpr
	Args  []*CallArg
	Sizes []Expr // `new T[n]`, `new T[a, b]`
}

// SizeOfExpr is `sizeof(T)`.
type SizeOfExpr struct {
	node
	X Expr // type or expression
}

// TypeOfExpr is `typeof(T)`.
type TypeOfExpr struct {
	node
	Type TypeExpr
}

// SetLit is `[1, 2, 3]` or `[1..10]`.
type SetLit struct {
	node
	Elems []Expr
}

// ArrayLit is the `|a, b, c|` array constructor.
type ArrayLit struct {
	node
	Elems []Expr
}

// LambdaExpr is `function(x: integer): integer := x * 2`.
type LambdaExpr struct {
	node
	Params []*Param
	Result TypeExpr
	Body   Expr // expression body
	Stmts  *Block
}

// IfExpr is `if c then a else b` used as an expression.
type IfExpr struct {
	node
	Cond Expr
	Then Expr
	Else Expr
}

// TupleExpr is `(a, b, c)`.
type TupleExpr struct {
	node
	Elems []Expr
}

// FormatExpr is the field-width syntax used in output calls: `'x':20`,
// `3.1415:10:3`.
type FormatExpr struct {
	node
	Value     Expr
	Width     Expr
	Precision Expr
}

// ToPair is the `a to b` pair used in dictionary literals.
type ToPair struct {
	node
	Left  Expr
	Right Expr
}

// FieldInit is one `name: value` entry of a record constructor.
type FieldInit struct {
	node
	Name  string
	Value Expr
}

// RecordCtorExpr is `(Name: 'x'; Age: 20)`.
type RecordCtorExpr struct {
	node
	Fields []*FieldInit
}

// RangeType is a subrange type `0..100`.
type RangeType struct {
	node
	Lo Expr
	Hi Expr
}

// NullableType is `T?`.
type NullableType struct {
	node
	Elem TypeExpr
}

// RangeExpr is `lo..hi` used inside set literals and slices.
type RangeExpr struct {
	node
	Lo Expr
	Hi Expr
}

func (*IdentExpr) exprNode()    {}
func (*IntLit) exprNode()       {}
func (*BigIntLit) exprNode()    {}
func (*RealLit) exprNode()      {}
func (*StrLit) exprNode()       {}
func (*NilLit) exprNode()       {}
func (*BoolLit) exprNode()      {}
func (*BinaryExpr) exprNode()   {}
func (*UnaryExpr) exprNode()    {}
func (*CallExpr) exprNode()     {}
func (*IndexExpr) exprNode()    {}
func (*FieldExpr) exprNode()    {}
func (*DerefExpr) exprNode()    {}
func (*AddrExpr) exprNode()     {}
func (*TypeCastExpr) exprNode() {}
func (*TypeTestExpr) exprNode() {}
func (*NewExpr) exprNode()      {}
func (*SizeOfExpr) exprNode()   {}
func (*TypeOfExpr) exprNode()   {}
func (*SetLit) exprNode()       {}
func (*ArrayLit) exprNode()     {}
func (*LambdaExpr) exprNode()   {}
func (*IfExpr) exprNode()       {}
func (*RangeExpr) exprNode()    {}

func (*VarDecl) declNode()      {}
func (*ConstDecl) declNode()    {}
func (*TypeDef) declNode()      {}
func (*ProcDecl) declNode()     {}
func (*PropertyDecl) declNode() {}

func (*Assign) stmtNode()       {}
func (*IfStmt) stmtNode()       {}
func (*WhileStmt) stmtNode()    {}
func (*LoopStmt) stmtNode()     {}
func (*RepeatStmt) stmtNode()   {}
func (*ForStmt) stmtNode()      {}
func (*ForeachStmt) stmtNode()  {}
func (*CaseStmt) stmtNode()     {}
func (*CallStmt) stmtNode()     {}
func (*BreakStmt) stmtNode()    {}
func (*ContinueStmt) stmtNode() {}
func (*ExitStmt) stmtNode()     {}
func (*GotoStmt) stmtNode()     {}
func (*LabelStmt) stmtNode()    {}
func (*TryStmt) stmtNode()      {}
func (*RaiseStmt) stmtNode()    {}
func (*WithStmt) stmtNode()     {}
func (*EmptyStmt) stmtNode()    {}

func (*TypeRef) typeNode()      {}
func (*RangeSpec) typeNode()    {}
func (*ArrayType) typeNode()    {}
func (*SequenceType) typeNode() {}
func (*SetType) typeNode()      {}
func (*PointerType) typeNode()  {}
func (*StringType) typeNode()   {}
func (*FileType) typeNode()     {}
func (*EnumType) typeNode()     {}
func (*ProcType) typeNode()     {}
func (*RecordType) typeNode()   {}

func (*RangeType) typeNode()       {}
func (*NullableType) typeNode()    {}
func (*RecordField) declNode()     {}
func (*FormatExpr) exprNode()      {}
func (*ToPair) exprNode()          {}
func (*RecordCtorExpr) exprNode()  {}
func (*FieldInit) exprNode()       {}
func (*TupleExpr) exprNode()       {}
func (*AssignExpr) exprNode()      {}
func (*GenericRef) exprNode()      {}
func (*YieldExpr) exprNode()       {}
func (*LocalDecls) stmtNode()      {}
func (*ExprStmt) stmtNode()        {}
func (*TupleAssign) stmtNode()     {}
func (*TupleAssignDecl) stmtNode() {}
func (*TupleAssignDecl) declNode() {}
func (*MatchStmt) stmtNode()       {}

func (*VarDecl) stmtNode() {}
