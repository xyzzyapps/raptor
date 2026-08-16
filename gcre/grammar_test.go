package gcre

import (
	"strings"
	"testing"
)



type CalcNode struct {
	Left  int64
	Op    string
	Right int64
}

type CalcActions struct{}

func (a *CalcActions) TOP(m *Match) {
	exprMatch := m.Get("expr")
	if exprMatch.Made != nil {
		m.Make(exprMatch.Made)
	}
}

func (a *CalcActions) Expr(m *Match) {
	num1 := m.Get("num1").Str
	op := m.Get("op").Str
	num2 := m.Get("num2").Str


	var l, r int64
	for _, ch := range num1 {
		l = l*10 + int64(ch-'0')
	}
	for _, ch := range num2 {
		r = r*10 + int64(ch-'0')
	}

	m.Make(&CalcNode{Left: l, Op: op, Right: r})
}

func TestGrammarActions(t *testing.T) {
	g := NewGrammar("CalcGrammar")

	g.DefineRule("op", func(g *Grammar, ctx *Context) *Match {
		ctx.SkipWS()
		if ctx.Pos < len(ctx.Src) && (ctx.Src[ctx.Pos] == '+' || ctx.Src[ctx.Pos] == '-' || ctx.Src[ctx.Pos] == '*') {
			ch := string(ctx.Src[ctx.Pos])
			ctx.Pos++
			return NewMatch(ch, ctx.Pos-1, ctx.Pos, true)
		}
		return NewMatch("", ctx.Pos, ctx.Pos, false)
	})

	g.DefineRule("expr", func(g *Grammar, ctx *Context) *Match {
		start := ctx.Pos
		n1 := g.Number(ctx)
		if !n1.Ok {
			return NewMatch("", start, start, false)
		}
		op := g.Subrule("op", ctx)
		if !op.Ok {
			ctx.Pos = start
			return NewMatch("", start, start, false)
		}
		n2 := g.Number(ctx)
		if !n2.Ok {
			ctx.Pos = start
			return NewMatch("", start, start, false)
		}

		m := NewMatch("", start, ctx.Pos, true)
		m.AddNamed("num1", n1)
		m.AddNamed("op", op)
		m.AddNamed("num2", n2)
		m.Str = string(ctx.Src[start:ctx.Pos])
		return m
	})

	g.DefineRule("TOP", func(g *Grammar, ctx *Context) *Match {
		start := ctx.Pos
		e := g.Subrule("expr", ctx)
		if !e.Ok {
			return NewMatch("", start, start, false)
		}
		m := NewMatch("", start, ctx.Pos, true)
		m.AddNamed("expr", e)
		m.Str = e.Str
		return m
	})

	actions := &CalcActions{}
	match, err := g.Parse("100 + 42", actions)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if match.Made == nil {
		t.Fatalf("expected match.Made to be populated by action")
	}

	calc, ok := match.Made.(*CalcNode)
	if !ok {
		t.Fatalf("expected *CalcNode, got %T", match.Made)
	}

	if calc.Left != 100 || calc.Op != "+" || calc.Right != 42 {
		t.Fatalf("unexpected AST: %+v", calc)
	}
}

func TestLookaroundsAndCharClasses(t *testing.T) {
	g := NewGrammar("LookaroundGrammar")

	g.DefineRule("word_alpha", func(g *Grammar, ctx *Context) *Match {
		start := ctx.Pos
		var sb strings.Builder
		for {
			m := g.Alpha(ctx)
			if !m.Ok {
				break
			}
			sb.WriteString(m.Str)
		}
		if sb.Len() == 0 {
			return NewMatch("", start, start, false)
		}
		return NewMatch(sb.String(), start, ctx.Pos, true)
	})

	g.DefineRule("keyword_let", func(g *Grammar, ctx *Context) *Match {
		start := ctx.Pos
		lit := g.Lit("let", ctx)
		if !lit.Ok {
			return NewMatch("", start, start, false)
		}
		// Positive lookahead: must be followed by space
		if before := g.Before("space_rule", ctx); !before.Ok {
			ctx.Pos = start
			return NewMatch("", start, start, false)
		}
		return NewMatch("let", start, ctx.Pos, true)
	})

	g.DefineRule("space_rule", func(g *Grammar, ctx *Context) *Match {
		return g.Space(ctx)
	})

	// Test lookahead
	ctx1 := &Context{Src: []rune("let x = 10"), Pos: 0}
	m1 := g.Subrule("keyword_let", ctx1)
	if !m1.Ok || m1.Str != "let" || ctx1.Pos != 3 {
		t.Fatalf("expected 'let' with lookahead, got match=%v pos=%d", m1.Ok, ctx1.Pos)
	}

	ctx2 := &Context{Src: []rune("letter"), Pos: 0}
	m2 := g.Subrule("keyword_let", ctx2)
	if m2.Ok {
		t.Fatalf("expected 'letter' to fail 'let' keyword match with lookahead")
	}

	// Test character classes: <[a..z0..9_]>
	ctx3 := &Context{Src: []rune("a5_#"), Pos: 0}
	m3_1 := g.CharClass("<[a..z0..9_]>", ctx3)
	m3_2 := g.CharClass("<[a..z0..9_]>", ctx3)
	m3_3 := g.CharClass("<[a..z0..9_]>", ctx3)
	m3_4 := g.CharClass("<[a..z0..9_]>", ctx3)

	if !m3_1.Ok || m3_1.Str != "a" {
		t.Errorf("expected 'a', got %v", m3_1)
	}
	if !m3_2.Ok || m3_2.Str != "5" {
		t.Errorf("expected '5', got %v", m3_2)
	}
	if !m3_3.Ok || m3_3.Str != "_" {
		t.Errorf("expected '_', got %v", m3_3)
	}
	if m3_4.Ok {
		t.Errorf("expected '#' to fail char class, got ok")
	}
}

func TestListWithSepAndPositional(t *testing.T) {
	g := NewGrammar("ListGrammar")

	g.DefineRule("item", func(g *Grammar, ctx *Context) *Match {
		return g.Ident(ctx)
	})

	g.DefineRule("comma", func(g *Grammar, ctx *Context) *Match {
		return g.Lit(",", ctx)
	})

	g.DefineRule("ident_list", func(g *Grammar, ctx *Context) *Match {
		return g.ListWithSep("item", "comma", ctx)
	})

	ctx := &Context{Src: []rune("alpha, beta, gamma, delta"), Pos: 0}
	m := g.Subrule("ident_list", ctx)
	if !m.Ok {
		t.Fatalf("expected list match ok")
	}

	items := m.GetAll("item")
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
	if m.At(0).Str != "alpha" || m.At(1).Str != "beta" || m.At(2).Str != "gamma" || m.At(3).Str != "delta" {
		t.Fatalf("positional capture mismatch: [%s, %s, %s, %s]",
			m.At(0).Str, m.At(1).Str, m.At(2).Str, m.At(3).Str)
	}
}

