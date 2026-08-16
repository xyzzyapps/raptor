package gcre

import "testing"

func TestHostHatchConsumesRest(t *testing.T) {
	RegisterHost("echo", func(g *Grammar, ctx *Context, cap *Match) bool {
		if ctx.Pos >= len(ctx.Src) {
			return false
		}
		cap.Make(string(ctx.Src[ctx.Pos:]))
		ctx.Pos = len(ctx.Src)
		return true
	})
	src := `
grammar G {
    rule TOP { 'hi' <HOST_echo>? }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatal(err)
	}
	m, err := g.Parse("hi leftover", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Ok {
		t.Fatal("expected match")
	}
	hm := m.Get("HOST_echo")
	if hm == nil || !hm.Ok {
		t.Fatalf("expected HOST_echo capture, made=%v", m.Made)
	}
}
