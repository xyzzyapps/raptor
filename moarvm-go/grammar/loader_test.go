package grammar

import (
	"testing"
)

func TestLoadGrammarFromString(t *testing.T) {
	grammarSrc := `
# Simple arithmetic grammar in Raku syntax
grammar MiniCalc {
    rule TOP { <statement> }
    rule statement { <ident> '=' <number> ';' }
    token ident { <\w+> }
    token number { <\d+> }
}
`

	g, err := LoadGrammarFromString(grammarSrc)
	if err != nil {
		t.Fatalf("failed loading grammar from string: %v", err)
	}

	if g.Name != "MiniCalc" {
		t.Fatalf("expected grammar name 'MiniCalc', got %q", g.Name)
	}

	// Test parsing with the dynamically created grammar!
	input := "x = 42;"
	match, err := g.Parse(input, nil)
	if err != nil {
		t.Fatalf("dynamic grammar failed parsing input %q: %v", input, err)
	}

	if !match.Ok {
		t.Fatalf("expected match.Ok = true")
	}

	stmtMatch := match.Get("statement")
	if !stmtMatch.Ok {
		t.Fatalf("expected statement subrule to match")
	}
}
