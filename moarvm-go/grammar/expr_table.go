package grammar

import (
	"strings"
)

// Fixity specifies whether an operator is prefix, infix, or postfix.
type Fixity int

const (
	FixityPrefix Fixity = iota
	FixityInfix
	FixityPostfix
)

// Assoc specifies operator associativity.
type Assoc int

const (
	AssocLeft Assoc = iota
	AssocRight
	AssocNon
	AssocChain
)

// Operator defines metadata for an operator.
type Operator struct {
	Symbol string
	Fixity Fixity
	Prec   int
	Assoc  Assoc
}

// OpTable stores operators for the EXPR precedence parser.
type OpTable struct {
	Prefix  map[string]*Operator
	Infix   map[string]*Operator
	Postfix map[string]*Operator
}

func NewOpTable() *OpTable {
	return &OpTable{
		Prefix:  make(map[string]*Operator),
		Infix:   make(map[string]*Operator),
		Postfix: make(map[string]*Operator),
	}
}

func (t *OpTable) AddPrefix(symbol string, prec int) {
	t.Prefix[symbol] = &Operator{
		Symbol: symbol,
		Fixity: FixityPrefix,
		Prec:   prec,
		Assoc:  AssocRight,
	}
}

func (t *OpTable) AddInfix(symbol string, prec int, assoc Assoc) {
	t.Infix[symbol] = &Operator{
		Symbol: symbol,
		Fixity: FixityInfix,
		Prec:   prec,
		Assoc:  assoc,
	}
}

func (t *OpTable) AddPostfix(symbol string, prec int) {
	t.Postfix[symbol] = &Operator{
		Symbol: symbol,
		Fixity: FixityPostfix,
		Prec:   prec,
		Assoc:  AssocLeft,
	}
}

// DefaultRakuOpTable returns a standard Raku operator precedence table.
func DefaultRakuOpTable() *OpTable {
	t := NewOpTable()

	// Exponentiation
	t.AddInfix("**", 100, AssocRight)

	// Prefix unary
	t.AddPrefix("+", 90)
	t.AddPrefix("-", 90)
	t.AddPrefix("!", 90)
	t.AddPrefix("~", 90)
	t.AddPrefix("?", 90)

	// Postfix unary
	t.AddPostfix("++", 95)
	t.AddPostfix("--", 95)

	// Multiplicative
	t.AddInfix("*", 80, AssocLeft)
	t.AddInfix("/", 80, AssocLeft)
	t.AddInfix("%", 80, AssocLeft)

	// Additive & string concat
	t.AddInfix("+", 70, AssocLeft)
	t.AddInfix("-", 70, AssocLeft)
	t.AddInfix("~", 70, AssocLeft)

	// Relational
	t.AddInfix("<=", 60, AssocNon)
	t.AddInfix(">=", 60, AssocNon)
	t.AddInfix("<", 60, AssocNon)
	t.AddInfix(">", 60, AssocNon)
	t.AddInfix("lt", 60, AssocNon)
	t.AddInfix("gt", 60, AssocNon)
	t.AddInfix("le", 60, AssocNon)
	t.AddInfix("ge", 60, AssocNon)

	// Equality
	t.AddInfix("==", 50, AssocChain)
	t.AddInfix("!=", 50, AssocChain)
	t.AddInfix("eq", 50, AssocChain)
	t.AddInfix("ne", 50, AssocChain)

	// Logical AND / OR
	t.AddInfix("&&", 40, AssocLeft)
	t.AddInfix("and", 40, AssocLeft)
	t.AddInfix("||", 30, AssocLeft)
	t.AddInfix("or", 30, AssocLeft)

	// Assignment
	t.AddInfix("=", 10, AssocRight)
	t.AddInfix("+=", 10, AssocRight)
	t.AddInfix("-=", 10, AssocRight)
	t.AddInfix("~=", 10, AssocRight)

	return t
}

// ParseEXPR parses expressions using Precedence-Climbing / Pratt parsing.
func (g *Grammar) ParseEXPR(ctx *Context, termRule string, table *OpTable, minPrec int) *Match {
	ctx.SkipWS()
	start := ctx.Pos

	// 1. Check for Prefix operators
	var prefixOps []*Operator
	for {
		ctx.SkipWS()
		matchedPrefix := false
		for sym, op := range table.Prefix {
			pMatch := g.ExactLit(sym, ctx)
			if pMatch.Ok {
				prefixOps = append(prefixOps, op)
				matchedPrefix = true
				break
			}
		}
		if !matchedPrefix {
			break
		}
	}

	// 2. Parse Base Term
	left := g.Subrule(termRule, ctx)
	if !left.Ok {
		ctx.Pos = start
		return NewMatch("", start, start, false)
	}

	// Wrap prefix operators around term if any
	for i := len(prefixOps) - 1; i >= 0; i-- {
		op := prefixOps[i]
		m := NewMatch(op.Symbol+left.Str, start, ctx.Pos, true)
		m.AddNamed("op", NewMatch(op.Symbol, start, start+len(op.Symbol), true))
		m.AddNamed("term", left)
		left = m
	}

	// 3. Check for Postfix operators
	for {
		ctx.SkipWS()
		matchedPostfix := false
		for sym, op := range table.Postfix {
			pMatch := g.ExactLit(sym, ctx)
			if pMatch.Ok {
				m := NewMatch(left.Str+op.Symbol, left.From, ctx.Pos, true)
				m.AddNamed("op", pMatch)
				m.AddNamed("term", left)
				left = m
				matchedPostfix = true
				break
			}
		}
		if !matchedPostfix {
			break
		}
	}

	// 4. Infix Operator Precedence Loop
	for ctx.Pos < len(ctx.Src) {
		saved := ctx.Pos
		ctx.SkipWS()
		if ctx.Pos >= len(ctx.Src) || ctx.Src[ctx.Pos] == ';' || ctx.Src[ctx.Pos] == ')' || ctx.Src[ctx.Pos] == '}' || ctx.Src[ctx.Pos] == ']' || ctx.Src[ctx.Pos] == ',' {
			ctx.Pos = saved
			break
		}

		// Find best matching infix operator (longer matches first)
		var bestOp *Operator
		var bestMatch *Match
		for sym, op := range table.Infix {
			checkPos := ctx.Pos
			match := g.ExactLit(sym, ctx)
			if match.Ok {
				if bestOp == nil || len(sym) > len(bestOp.Symbol) {
					bestOp = op
					bestMatch = match
				}
			}
			ctx.Pos = checkPos
		}

		if bestOp == nil || bestOp.Prec < minPrec {
			ctx.Pos = saved
			break
		}

		// Advance past the operator
		ctx.Pos += len(bestOp.Symbol)

		// Determine precedence threshold for right-hand side based on associativity
		nextMinPrec := bestOp.Prec + 1
		if bestOp.Assoc == AssocRight {
			nextMinPrec = bestOp.Prec
		}

		right := g.ParseEXPR(ctx, termRule, table, nextMinPrec)
		if !right.Ok {
			ctx.Pos = saved
			break
		}

		// Build combined infix Match node
		combined := NewMatch(strings.TrimSpace(string(ctx.Src[left.From:ctx.Pos])), left.From, ctx.Pos, true)
		combined.AddNamed("left", left)
		combined.AddNamed("op", bestMatch)
		combined.AddNamed("right", right)
		left = combined
	}

	return left
}
