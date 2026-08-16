package gcre

import (
	"testing"
)

func TestLoadGrammarFromString(t *testing.T) {
	g, err := LoadGrammarFromFile("examples/minicalc.raku")
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

func TestLoadIndependentJSONLikeGrammar(t *testing.T) {
	g, err := LoadGrammarFromFile("examples/tinyjson.raku")
	if err != nil {
		t.Fatal(err)
	}
	m, err := g.Parse(`"hi"`, nil)
	if err != nil || !m.Ok {
		t.Fatalf("string value: %v ok=%v", err, m != nil && m.Ok)
	}
	m, err = g.Parse(`123`, nil)
	if err != nil || !m.Ok {
		t.Fatalf("number value: %v", err)
	}
}
