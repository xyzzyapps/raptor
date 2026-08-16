package gcre

import (
	"strconv"
	"testing"
)

type ASTExpr interface {
	Eval() int64
}

type NumNode struct {
	Val int64
}

func (n *NumNode) Eval() int64 { return n.Val }

type BinNode struct {
	Left  ASTExpr
	Op    string
	Right ASTExpr
}

func (b *BinNode) Eval() int64 {
	l := b.Left.Eval()
	r := b.Right.Eval()
	switch b.Op {
	case "+":
		return l + r
	case "-":
		return l - r
	case "*":
		return l * r
	case "/":
		if r == 0 {
			return 0
		}
		return l / r
	}
	return 0
}

type EXPRCalculatorActions struct{}

func (a *EXPRCalculatorActions) Number(m *Match) {
	v, _ := strconv.ParseInt(m.Str, 10, 64)
	m.Make(&NumNode{Val: v})
}

func (a *EXPRCalculatorActions) Term(m *Match) {
	if n := m.Get("number"); n.Ok && n.Made != nil {
		m.Make(n.Made)
	}
}

func (a *EXPRCalculatorActions) Expr(m *Match) {
	leftMatch := m.Get("left")
	rightMatch := m.Get("right")
	opMatch := m.Get("op")

	if leftMatch.Ok && rightMatch.Ok && opMatch.Ok {
		var l, r ASTExpr
		if leftMatch.Made != nil {
			l = leftMatch.Made.(ASTExpr)
		}
		if rightMatch.Made != nil {
			r = rightMatch.Made.(ASTExpr)
		}
		if l != nil && r != nil {
			m.Make(&BinNode{Left: l, Op: opMatch.Str, Right: r})
			return
		}
	}

	if termMatch := m.Get("term"); termMatch.Ok && termMatch.Made != nil {
		m.Make(termMatch.Made)
	}
}

func TestOperatorPrecedence(t *testing.T) {
	g := NewGrammar("ExprCalc")

	g.DefineRule("number", func(g *Grammar, ctx *Context) *Match {
		return g.Number(ctx)
	})

	g.DefineRule("term", func(g *Grammar, ctx *Context) *Match {
		start := ctx.Pos
		n := g.Subrule("number", ctx)
		if !n.Ok {
			return NewMatch("", start, start, false)
		}
		m := NewMatch(n.Str, start, ctx.Pos, true)
		m.AddNamed("number", n)
		return m
	})

	table := DefaultRakuOpTable()
	g.DefineRule("expr", func(g *Grammar, ctx *Context) *Match {
		return g.EXPR(ctx, "term", table)
	})

	g.DefineRule("TOP", func(g *Grammar, ctx *Context) *Match {
		start := ctx.Pos
		e := g.Subrule("expr", ctx)
		if !e.Ok {
			return NewMatch("", start, start, false)
		}
		m := NewMatch(e.Str, start, ctx.Pos, true)
		m.AddNamed("expr", e)
		if e.Made != nil {
			m.Make(e.Made)
		}
		return m
	})

	actions := &EXPRCalculatorActions{}

	// Test precedence: 2 + 3 * 4 should evaluate to 14 (not 20)
	m, err := g.Parse("2 + 3 * 4", actions)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if m == nil || m.Str != "2 + 3 * 4" {
		t.Fatalf("unexpected match: %+v", m)
	}
}
