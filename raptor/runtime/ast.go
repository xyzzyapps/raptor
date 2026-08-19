package raptor

// Node is the base interface for all AST nodes.
type Node interface {
	node()
}

// Stmt represents an executable statement node.
type Stmt interface {
	Node
	stmt()
}

// Expr represents an evaluatable expression node.
type Expr interface {
	Node
	expr()
}

// Program is the root AST node containing all top-level statements.
type Program struct {
	Stmts []Stmt
}

func (p *Program) node() {}

// Param represents a typed formal parameter in a sub or closure signature.
type Param struct {
	Name         string // e.g. "$x", "@arr", "%hash"
	Type         string // e.g. "Int", "Str", "Num", "Array", "Hash", "Callable", "Any"
	IsOptional   bool
	IsSlurpy     bool   // "*@tail", "*%named"
	Where        Expr   // optional where constraint: { $_ > 0 }
	DestructArr  []Param // e.g. [$head, *@tail]
	DestructHash []Param // e.g. :{:$name, :$age}
}

// GatherExpr represents a gather { ... take $x; ... } generator expression.
type GatherExpr struct {
	Body *BlockStmt
}

func (g *GatherExpr) node() {}
func (g *GatherExpr) expr() {}

// TakeStmt represents a 'take' generator emission statement.
type TakeStmt struct {
	Value Expr
}

func (t *TakeStmt) node() {}
func (t *TakeStmt) stmt() {}
func (t *TakeStmt) expr() {}


// BlockStmt represents a sequence of statements inside braces.
type BlockStmt struct {
	Stmts []Stmt
}

func (b *BlockStmt) node() {}
func (b *BlockStmt) stmt() {}

// VarDeclStmt represents a 'my', 'our', or 'state' variable declaration.
type VarDeclStmt struct {
	Scope string // "my", "our", or "state"
	Type  string // "Int", "Str", etc. (empty for untyped)
	Name  string // "$x", "@a", "%h"
	Where Expr   // optional where constraint: { $_ > 0 }
	Value Expr   // optional initializer
}

func (v *VarDeclStmt) node() {}
func (v *VarDeclStmt) stmt() {}

// PackageDeclStmt represents a 'package Foo;' or 'package Foo { ... }' or 'module Foo;' declaration.
type PackageDeclStmt struct {
	Name   string     // e.g. "Math::Utils", "Foo"
	IsUnit bool       // 'unit module Foo;' / 'unit package Foo;'
	Body   *BlockStmt // optional block scope
}

func (p *PackageDeclStmt) node() {}
func (p *PackageDeclStmt) stmt() {}

// AssignStmt represents variable, index, or hash element assignment.
type AssignStmt struct {
	Target Expr   // VarExpr, IndexExpr, or HashAccessExpr
	Op     string // "=", "+=", "-=", "~="
	Value  Expr
}

func (a *AssignStmt) node() {}
func (a *AssignStmt) stmt() {}
func (a *AssignStmt) expr() {}

// IfStmt represents an if/elsif/else branching statement.
type IfStmt struct {
	Condition  Expr
	ThenBranch *BlockStmt
	ElsifConds []Expr
	ElsifThen  []*BlockStmt
	ElseBranch *BlockStmt
}

func (i *IfStmt) node() {}
func (i *IfStmt) stmt() {}

// UnlessStmt represents an 'unless' condition statement.
type UnlessStmt struct {
	Condition Expr
	Body      *BlockStmt
}

func (u *UnlessStmt) node() {}
func (u *UnlessStmt) stmt() {}

// WhileStmt represents a 'while' or 'until' loop.
type WhileStmt struct {
	IsUntil   bool
	Condition Expr
	Body      *BlockStmt
}

func (w *WhileStmt) node() {}
func (w *WhileStmt) stmt() {}

// ForStmt represents a 'for' list iteration loop.
type ForStmt struct {
	Iterable Expr
	VarName  string // e.g. "$x" from pointy block
	Body     *BlockStmt
}

func (f *ForStmt) node() {}
func (f *ForStmt) stmt() {}

// LoopStmt represents a C-style loop (init; cond; step).
type LoopStmt struct {
	Init Expr
	Cond Expr
	Step Expr
	Body *BlockStmt
}

func (l *LoopStmt) node() {}
func (l *LoopStmt) stmt() {}

// SubsetDeclStmt represents a 'subset Name where { ... }' dynamic refinement declaration.
type SubsetDeclStmt struct {
	Name  string
	Where Expr
}

func (s *SubsetDeclStmt) node() {}
func (s *SubsetDeclStmt) stmt() {}

// SubDeclStmt represents a sub or multi sub declaration.
type SubDeclStmt struct {
	IsMulti bool
	Name    string
	Params  []Param
	Body    *BlockStmt
}

func (s *SubDeclStmt) node() {}
func (s *SubDeclStmt) stmt() {}

// ReturnStmt represents a 'return' statement.
type ReturnStmt struct {
	Value Expr
}

func (r *ReturnStmt) node() {}
func (r *ReturnStmt) stmt() {}

// BreakStmt represents a 'last' loop termination statement.
type BreakStmt struct{}

func (b *BreakStmt) node() {}
func (b *BreakStmt) stmt() {}

// ContinueStmt represents a 'next' loop iteration jump statement.
type ContinueStmt struct{}

func (c *ContinueStmt) node() {}
func (c *ContinueStmt) stmt() {}

// RedoStmt represents a 'redo' restart of the current loop iteration.
type RedoStmt struct{}

func (r *RedoStmt) node() {}
func (r *RedoStmt) stmt() {}

// UseStmt represents a module import (e.g. use Module:from<Perl5>).
type UseStmt struct {
	Module string
	From   string
}

func (u *UseStmt) node() {}
func (u *UseStmt) stmt() {}

// ExprStmt wraps an expression as a statement.
type ExprStmt struct {
	Expr Expr
}

func (e *ExprStmt) node() {}
func (e *ExprStmt) stmt() {}

// LiteralExpr represents integer, float, string, or boolean literals.
type LiteralExpr struct {
	Type  TokenType
	Value any
}

func (l *LiteralExpr) node() {}
func (l *LiteralExpr) expr() {}

// VarExpr represents variable evaluation by name.
type VarExpr struct {
	Name string // "$x", "@a", "%h"
}

func (v *VarExpr) node() {}
func (v *VarExpr) expr() {}

// BinaryExpr represents binary infix expressions.
type BinaryExpr struct {
	Left  Expr
	Op    string
	Right Expr
}

func (b *BinaryExpr) node() {}
func (b *BinaryExpr) expr() {}

// UnaryExpr represents prefix unary operators.
type UnaryExpr struct {
	Op    string
	Right Expr
}

func (u *UnaryExpr) node() {}
func (u *UnaryExpr) expr() {}

// TernaryExpr represents ternary conditional expressions: $cond ?? $then !! $else or $cond ? $then : $else.
type TernaryExpr struct {
	Cond Expr
	Then Expr
	Else Expr
}

func (t *TernaryExpr) node() {}
func (t *TernaryExpr) expr() {}

// CallExpr represents function or sub invocation.
type CallExpr struct {
	Callee Expr
	Args   []Expr
}

func (c *CallExpr) node() {}
func (c *CallExpr) expr() {}

// MethodCallExpr represents method calls on objects.
type MethodCallExpr struct {
	Target Expr
	Method string
	Args   []Expr
}

func (m *MethodCallExpr) node() {}
func (m *MethodCallExpr) expr() {}

// ArrayLiteralExpr represents an array constructor: [1, 2, 3].
type ArrayLiteralExpr struct {
	Elements []Expr
}

func (a *ArrayLiteralExpr) node() {}
func (a *ArrayLiteralExpr) expr() {}

// HashLiteralExpr represents a hash constructor: { "a" => 1, "b" => 2 }.
type HashLiteralExpr struct {
	Pairs [][2]Expr
}

func (h *HashLiteralExpr) node() {}
func (h *HashLiteralExpr) expr() {}

// IndexExpr represents array subscript indexing: @a[0].
type IndexExpr struct {
	Array Expr
	Index Expr
}

func (i *IndexExpr) node() {}
func (i *IndexExpr) expr() {}

// HashAccessExpr represents hash key lookup: %h<key> or %h{"key"}.
type HashAccessExpr struct {
	Hash Expr
	Key  Expr
}

func (h *HashAccessExpr) node() {}
func (h *HashAccessExpr) expr() {}

// ClosureExpr represents an anonymous sub or block closure.
type ClosureExpr struct {
	Params []Param
	Body   *BlockStmt
}

func (c *ClosureExpr) node() {}
func (c *ClosureExpr) expr() {}

// SmartMatchExpr represents the ~~ operator: $val ~~ Int, $val ~~ @list, etc.
type SmartMatchExpr struct {
	Left  Expr
	Right Expr
}

func (s *SmartMatchExpr) node() {}
func (s *SmartMatchExpr) expr() {}

// WhenClause represents a single 'when' arm inside a given block.
type WhenClause struct {
	Match Expr
	Body  *BlockStmt
}

// GivenStmt represents: given $topic { when $match { } ... default { } }
type GivenStmt struct {
	Topic   Expr
	Whens   []WhenClause
	Default *BlockStmt
}

func (g *GivenStmt) node() {}
func (g *GivenStmt) stmt() {}

// EnumValue represents a single named constant in an enum declaration.
type EnumValue struct {
	Name  string
	Index int64
}

// EnumDeclStmt represents: enum Color <Red Green Blue>;
type EnumDeclStmt struct {
	Name   string
	Values []EnumValue
}

func (e *EnumDeclStmt) node() {}
func (e *EnumDeclStmt) stmt() {}

// ChainedCompExpr represents chained comparisons: 1 < $x < 10
// Desugared to: Left op1 Mid && Mid op2 Right (without re-evaluating Mid).
type ChainedCompExpr struct {
	Exprs []Expr   // [left, mid, right, ...]
	Ops   []string // [op1, op2, ...]
}

func (c *ChainedCompExpr) node() {}
func (c *ChainedCompExpr) expr() {}

// InterpStringExpr represents a double-quoted string with interpolation.
// Parts alternate between literal string segments and expression segments.
type InterpStringExpr struct {
	Parts []Expr // LiteralExpr for text, VarExpr / other Expr for interpolated parts
}

func (i *InterpStringExpr) node() {}
func (i *InterpStringExpr) expr() {}

// NativeSubDeclStmt represents: sub Name(params) returns Type is native('lib.dll') is symbol('sym') { * }
type NativeSubDeclStmt struct {
	Name       string
	Params     []Param
	ReturnType string
	Library    string
	Symbol     string
}

func (n *NativeSubDeclStmt) node() {}
func (n *NativeSubDeclStmt) stmt() {}

// CStructField represents a single field in a CStruct or CUnion.
type CStructField struct {
	Name   string
	Type   string
	Offset int
	Size   int
}

// CStructDeclStmt represents: class Name is repr('CStruct' | 'CUnion') { has Type $.field; ... }
type CStructDeclStmt struct {
	Name       string
	IsUnion    bool
	Fields     []CStructField
	FieldIndex map[string]int
	TotalSize  int
	Alignment  int
}

func (c *CStructDeclStmt) node() {}
func (c *CStructDeclStmt) stmt() {}

// AssertStmt represents: assert <condition> [, <message>];
type AssertStmt struct {
	Condition Expr
	Message   Expr // optional custom error message
}

func (a *AssertStmt) node() {}
func (a *AssertStmt) stmt() {}

// VerifyStmt is PRE / POST / INVARIANT / CHECK / ASSERT / TEST / PROPERTY / SUBTEST.
// Expression form:  PRE $b != 0;
// Block form:       PRE { $b != 0 }
// Named+block:      TEST "name" { ... }   PROPERTY "name" ($a, $b) { ... }
type VerifyStmt struct {
	Kind    string     // PRE, POST, INVARIANT, CHECK, ASSERT, TEST, PROPERTY, SUBTEST
	Name    Expr       // TEST / PROPERTY / SUBTEST title
	Params  []Param    // PROPERTY signature
	Cond    Expr       // expression form
	Body    *BlockStmt // block form
	Message Expr       // optional trailing message
}

func (v *VerifyStmt) node() {}
func (v *VerifyStmt) stmt() {}

// AdviceHookStmt represents: before|after|around subName(params) { ... }
type AdviceHookStmt struct {
	Kind       string // "before", "after", "around"
	TargetName string // e.g. "compute"
	Params     []Param
	Body       *BlockStmt
}

func (a *AdviceHookStmt) node() {}
func (a *AdviceHookStmt) stmt() {}

// ModifierKind represents the kind of statement modifier.
type ModifierKind int

const (
	ModIf ModifierKind = iota
	ModUnless
	ModWhile
	ModUntil
	ModFor
	ModGiven
)

// ModifierStmt represents postfix statement modifiers:
// $x = 10 if $cond;
// return $err unless $valid;
// $i++ while $i < 10;
// $i++ until $i >= 10;
// say $_ for @list;
// say $x given $val;
type ModifierStmt struct {
	Kind      ModifierKind
	Target    Stmt // The modified statement (e.g. ExprStmt, AssignStmt, ReturnStmt)
	Condition Expr // The condition or iterable expression
	VarName   string // Optional variable name for 'for', defaults to "$_"
}

func (m *ModifierStmt) node() {}
func (m *ModifierStmt) stmt() {}

// LabelStmt represents a statement label: LABEL: or LABEL: stmt
type LabelStmt struct {
	Name string
	Stmt Stmt
}

func (l *LabelStmt) node() {}
func (l *LabelStmt) stmt() {}

// GotoStmt represents a goto jump: goto LABEL; or goto &subname;
type GotoStmt struct {
	Target string
	IsSub  bool
}

func (g *GotoStmt) node() {}
func (g *GotoStmt) stmt() {}

// RefExpr represents taking a reference: \$scalar, \@arr, \%hash, \&sub
type RefExpr struct {
	Expr Expr
}

func (r *RefExpr) node() {}
func (r *RefExpr) expr() {}

// DerefKind defines the flavor of dereference.
type DerefKind int

const (
	DerefScalar     DerefKind = iota // $$ref or ${$ref}
	DerefArray                       // @$ref or @{$ref}
	DerefHash                        // %$ref or %{$ref}
	DerefCode                        // &$ref or &{$ref}
	DerefArrowArray                  // $ref->[0]
	DerefArrowHash                   // $ref->{"key"}
	DerefArrowCode                   // $ref->("arg")
)

// DerefExpr represents dereferencing expressions.
type DerefExpr struct {
	Kind  DerefKind
	Ref   Expr
	Index Expr   // for DerefArrowArray or DerefArrowHash
	Args  []Expr // for DerefArrowCode
}

func (d *DerefExpr) node() {}
func (d *DerefExpr) expr() {}

// BacktickExpr represents a shell command execution via backticks or qx{}.
type BacktickExpr struct {
	Command Expr
}

func (b *BacktickExpr) node() {}
func (b *BacktickExpr) expr() {}

// RuleDecl represents a rule, token, or regex inside a grammar.
type RuleDecl struct {
	Kind    string // "rule", "token", "regex"
	Name    string
	Pattern string
}

// GrammarDeclStmt represents a grammar declaration: grammar PointGrammar { rule TOP { ... } ... }
type GrammarDeclStmt struct {
	Name  string
	Rules []RuleDecl
}

func (g *GrammarDeclStmt) node() {}
func (g *GrammarDeclStmt) stmt() {}

// StubExpr represents the ... (yada-yada / stub) operator.
type StubExpr struct {
	Message string
}

func (s *StubExpr) node() {}
func (s *StubExpr) expr() {}
func (s *StubExpr) stmt() {}



