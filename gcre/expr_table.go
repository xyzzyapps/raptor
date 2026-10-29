package gcre

import (
	"fmt"
	"sort"
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
	Symbol   string
	Fixity   Fixity
	Prec     int
	Assoc    Assoc
	RuleName string // Optional grammar rule (e.g. "infix:sym<and>") for custom token matching
	Order    int    // Declaration order tie-breaker
}

// OpTable stores operators for the EXPR precedence parser.
type OpTable struct {
	Prefix  map[string]*Operator
	Infix   map[string]*Operator
	Postfix map[string]*Operator
	seq     int
}

func NewOpTable() *OpTable {
	return &OpTable{
		Prefix:  make(map[string]*Operator),
		Infix:   make(map[string]*Operator),
		Postfix: make(map[string]*Operator),
	}
}

func (t *OpTable) Clone() *OpTable {
	if t == nil {
		return NewOpTable()
	}
	c := NewOpTable()
	c.seq = t.seq
	for k, v := range t.Prefix {
		cp := *v
		c.Prefix[k] = &cp
	}
	for k, v := range t.Infix {
		cp := *v
		c.Infix[k] = &cp
	}
	for k, v := range t.Postfix {
		cp := *v
		c.Postfix[k] = &cp
	}
	return c
}

func (t *OpTable) AddPrefix(symbol string, prec int) {
	t.seq++
	t.Prefix[symbol] = &Operator{
		Symbol: symbol,
		Fixity: FixityPrefix,
		Prec:   prec,
		Assoc:  AssocRight,
		Order:  t.seq,
	}
}

func (t *OpTable) AddInfix(symbol string, prec int, assoc Assoc) {
	t.seq++
	t.Infix[symbol] = &Operator{
		Symbol: symbol,
		Fixity: FixityInfix,
		Prec:   prec,
		Assoc:  assoc,
		Order:  t.seq,
	}
}

func (t *OpTable) AddPostfix(symbol string, prec int) {
	t.seq++
	t.Postfix[symbol] = &Operator{
		Symbol: symbol,
		Fixity: FixityPostfix,
		Prec:   prec,
		Assoc:  AssocLeft,
		Order:  t.seq,
	}
}

// DefaultRakuOpTable returns a standard Raku operator precedence table.
func DefaultRakuOpTable() *OpTable {
	t := NewOpTable()

	// Exponentiation
	t.AddInfix("**", 100, AssocRight)

	// Postfix unary
	t.AddPostfix("++", 95)
	t.AddPostfix("--", 95)

	// Prefix unary
	t.AddPrefix("+", 90)
	t.AddPrefix("-", 90)
	t.AddPrefix("!", 90)
	t.AddPrefix("~", 90)
	t.AddPrefix("?", 90)

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

func sortedOps(m map[string]*Operator) []*Operator {
	ops := make([]*Operator, 0, len(m))
	for _, op := range m {
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool {
		ri := len([]rune(ops[i].Symbol))
		rj := len([]rune(ops[j].Symbol))
		if ri != rj {
			return ri > rj
		}
		if ops[i].Order != ops[j].Order {
			return ops[i].Order < ops[j].Order
		}
		return ops[i].Symbol < ops[j].Symbol
	})
	return ops
}

func (g *Grammar) matchBestOp(ctx *Context, ops map[string]*Operator) (*Operator, *Match) {
	if len(ops) == 0 || ctx.Pos >= len(ctx.Src) {
		return nil, nil
	}
	savedPos := ctx.Pos
	savedActions := ctx.Actions
	// Suppress action side effects during speculative LTM operator probing;
	// ParseEXPR invokes the winning operator's action once combined.
	ctx.Actions = nil
	defer func() { ctx.Actions = savedActions }()

	var bestOp *Operator
	var bestMatch *Match
	bestLen := -1

	for _, op := range sortedOps(ops) {
		ctx.Pos = savedPos
		var m *Match
		if op.RuleName != "" {
			if fn, ok := g.Rules[op.RuleName]; ok {
				prevSym := ctx.SymVal
				ctx.SymVal = op.Symbol
				m = fn(g, ctx)
				ctx.SymVal = prevSym
			} else {
				m = g.ExactLit(op.Symbol, ctx)
			}
		} else {
			m = g.ExactLit(op.Symbol, ctx)
		}
		if m != nil && m.Ok {
			mlen := m.To - m.From
			if mlen > bestLen {
				bestLen = mlen
				bestOp = op
				bestMatch = m
			}
		}
	}
	ctx.Pos = savedPos
	return bestOp, bestMatch
}

// ParseEXPR parses expressions using Precedence-Climbing / Pratt parsing.
func (g *Grammar) ParseEXPR(ctx *Context, termRule string, table *OpTable, minPrec int) *Match {
	if table == nil {
		if g.OpTable != nil {
			table = g.OpTable
		} else {
			table = DefaultRakuOpTable()
		}
	}
	ctx.SkipWS()
	start := ctx.Pos

	var left *Match

	// 1. Check for a Prefix operator (LTM) and bind its operand at prefix precedence.
	if prefOp, prefMatch := g.matchBestOp(ctx, table.Prefix); prefOp != nil {
		ctx.Pos = prefMatch.To
		operand := g.ParseEXPR(ctx, termRule, table, prefOp.Prec)
		if !operand.Ok {
			ctx.Pos = start
			return NewMatch("", start, start, false)
		}
		m := NewMatch(strings.TrimSpace(string(ctx.Src[start:ctx.Pos])), start, ctx.Pos, true)
		m.AddNamed("op", prefMatch)
		m.AddNamed("sym", NewMatch(prefOp.Symbol, prefMatch.From, prefMatch.To, true))
		m.AddNamed("term", operand)
		if ctx.Actions != nil {
			g.callAction("prefix:sym<"+prefOp.Symbol+">", m, ctx.Actions)
			if m.Made == nil {
				g.callAction("EXPR", m, ctx.Actions)
			}
			if m.Made == nil {
				g.callAction("expr", m, ctx.Actions)
			}
		}
		left = m
	} else {
		// 2. Parse Base Term
		left = g.Subrule(termRule, ctx)
		if !left.Ok {
			ctx.Pos = start
			return NewMatch("", start, start, false)
		}
	}

	// 3. Check for Postfix operators
	for {
		saved := ctx.Pos
		ctx.SkipWS()
		postOp, postMatch := g.matchBestOp(ctx, table.Postfix)
		if postOp == nil || postOp.Prec < minPrec {
			ctx.Pos = saved
			break
		}
		ctx.Pos = postMatch.To
		m := NewMatch(strings.TrimSpace(string(ctx.Src[left.From:ctx.Pos])), left.From, ctx.Pos, true)
		m.AddNamed("op", postMatch)
		m.AddNamed("sym", NewMatch(postOp.Symbol, postMatch.From, postMatch.To, true))
		m.AddNamed("term", left)
		if ctx.Actions != nil {
			g.callAction("postfix:sym<"+postOp.Symbol+">", m, ctx.Actions)
			if m.Made == nil {
				g.callAction("EXPR", m, ctx.Actions)
			}
			if m.Made == nil {
				g.callAction("expr", m, ctx.Actions)
			}
		}
		left = m
	}

	// 4. Infix Operator Precedence Loop
	var prevInfix *Operator
	for ctx.Pos < len(ctx.Src) {
		saved := ctx.Pos
		ctx.SkipWS()
		if ctx.Pos >= len(ctx.Src) {
			ctx.Pos = saved
			break
		}

		bestOp, bestMatch := g.matchBestOp(ctx, table.Infix)
		if bestOp == nil {
			ctx.Pos = saved
			break
		}

		if prevInfix != nil && prevInfix.Assoc == AssocNon && bestOp.Assoc == AssocNon && prevInfix.Prec == bestOp.Prec {
			ctx.GoalErr = fmt.Sprintf("operators %q and %q are non-associative and cannot be chained", prevInfix.Symbol, bestOp.Symbol)
			ctx.Pos = start
			return NewMatch("", start, start, false)
		}

		if bestOp.Prec < minPrec {
			ctx.Pos = saved
			break
		}

		// Advance past the matched operator runes
		ctx.Pos = bestMatch.To

		// Determine precedence threshold for right-hand side based on associativity
		nextMinPrec := bestOp.Prec + 1
		if bestOp.Assoc == AssocRight {
			nextMinPrec = bestOp.Prec
		}

		right := g.ParseEXPR(ctx, termRule, table, nextMinPrec)
		if !right.Ok {
			if ctx.GoalErr != "" {
				ctx.Pos = start
				return NewMatch("", start, start, false)
			}
			ctx.Pos = saved
			break
		}

		// Build combined infix Match node
		combined := NewMatch(strings.TrimSpace(string(ctx.Src[left.From:ctx.Pos])), left.From, ctx.Pos, true)
		combined.AddNamed("left", left)
		combined.AddNamed("op", bestMatch)
		combined.AddNamed("sym", NewMatch(bestOp.Symbol, bestMatch.From, bestMatch.To, true))
		combined.AddNamed("right", right)
		if ctx.Actions != nil {
			g.callAction("infix:sym<"+bestOp.Symbol+">", combined, ctx.Actions)
			if combined.Made == nil {
				g.callAction("EXPR", combined, ctx.Actions)
			}
			if combined.Made == nil {
				g.callAction("expr", combined, ctx.Actions)
			}
		}
		left = combined
		prevInfix = bestOp
	}

	return left
}
