package gcre

import (
	"fmt"
	"os"
	"unicode"
)

// DynamicRule is one token/rule/regex compiled from a .raku file.
type DynamicRule struct {
	Name    string
	IsToken bool
	Pat     rx
}

// DynamicGrammarDef is a parsed .raku grammar (no actions).
type DynamicGrammarDef struct {
	Name  string
	Rules []*DynamicRule
}

// LoadGrammarFromFile reads a .raku grammar file.
func LoadGrammarFromFile(path string) (*Grammar, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading grammar file %q: %w", path, err)
	}
	return LoadGrammarFromString(string(content))
}

// LoadGrammarFromString parses a .raku grammar (Pigeon) and builds matchers.
func LoadGrammarFromString(source string) (*Grammar, error) {
	tree, err := Parse("grammar.raku", []byte(source))
	if err != nil {
		return nil, fmt.Errorf("raku grammar: %w", err)
	}
	def, err := walkGrammarFile(tree)
	if err != nil {
		return nil, err
	}
	if len(def.Rules) == 0 {
		return nil, fmt.Errorf("no rules found in grammar")
	}

	g := NewGrammar(def.Name)
	for _, r := range def.Rules {
		isToken := r.IsToken
		pat := r.Pat
		name := r.Name
		g.DefineRule(name, func(g *Grammar, ctx *Context) *Match {
			if !isToken {
				ctx.SkipWS()
			}
			start := ctx.Pos
			m := NewMatch("", start, start, true)
			if pat == nil || !pat.match(g, ctx, m, isToken) {
				ctx.Pos = start
				return NewMatch("", start, start, false)
			}
			m.To = ctx.Pos
			m.Str = string(ctx.Src[start:ctx.Pos])
			m.Ok = true
			return m
		})
	}
	return g, nil
}

// --- matcher (runtime; not a parser) ---

type rx interface {
	match(g *Grammar, ctx *Context, cap *Match, token bool) bool
}

type rxAlt struct{ alts []rx }
type rxSeq struct{ elts []rx }
type rxLit struct{ s string }
type rxAny struct{}
type rxSub struct {
	name    string
	capture bool
	quant   byte
}
type rxCC struct {
	spec  string
	quant byte
}
type rxGrp struct {
	inner rx
	quant byte
}

func applyQuant(q byte, ctx *Context, once func() bool) bool {
	switch q {
	case '*':
		for {
			saved := ctx.Pos
			if !once() {
				return true
			}
			if ctx.Pos <= saved {
				return true
			}
		}
	case '+':
		if !once() {
			return false
		}
		for {
			saved := ctx.Pos
			if !once() {
				return true
			}
			if ctx.Pos <= saved {
				return true
			}
		}
	case '?':
		once()
		return true
	default:
		return once()
	}
}

func (r *rxAlt) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	saved := ctx.Pos
	for _, a := range r.alts {
		ctx.Pos = saved
		if a.match(g, ctx, cap, token) {
			return true
		}
	}
	ctx.Pos = saved
	return false
}

func (r *rxSeq) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	for _, e := range r.elts {
		if !token {
			ctx.SkipWS()
		}
		if !e.match(g, ctx, cap, token) {
			return false
		}
	}
	return true
}

func (r *rxLit) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if !token {
		ctx.SkipWS()
	}
	return g.ExactLit(r.s, ctx).Ok
}

func (r *rxAny) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if ctx.Pos >= len(ctx.Src) {
		return false
	}
	ctx.Pos++
	return true
}

func (r *rxSub) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	once := func() bool {
		if !token {
			ctx.SkipWS()
		}
		m := g.Subrule(r.name, ctx)
		if !m.Ok {
			return false
		}
		if r.capture && cap != nil {
			cap.AddNamed(r.name, m)
		}
		return true
	}
	return applyQuant(r.quant, ctx, once)
}

func (r *rxCC) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	once := func() bool {
		if !token {
			ctx.SkipWS()
		}
		if r.spec == `\s` {
			if ctx.Pos < len(ctx.Src) && unicode.IsSpace(ctx.Src[ctx.Pos]) {
				ctx.Pos++
				return true
			}
			return false
		}
		if r.spec == `\d` {
			return g.Digit(ctx).Ok
		}
		if r.spec == `\w` {
			return g.Word(ctx).Ok
		}
		return g.CharClass(r.spec, ctx).Ok
	}
	return applyQuant(r.quant, ctx, once)
}

func (r *rxGrp) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	once := func() bool {
		return r.inner.match(g, ctx, cap, token)
	}
	return applyQuant(r.quant, ctx, once)
}
