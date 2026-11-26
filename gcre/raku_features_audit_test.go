package gcre

import (
	"testing"
)

func TestRakuAdverbsIgnoreCaseAndIgnoreMark(t *testing.T) {
	src := `
grammar AdverbGram {
    token kw :i { 'select' }
    token cafe :m { 'cafe' }
    token resume_im :i :m { 'resume' }
    token sql_alt { [ :i 'select' | 'from' | 'where' ] $ }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatalf("LoadGrammarFromString failed: %v", err)
	}

	for _, in := range []string{"select", "SELECT", "SeLeCt"} {
		m, err := g.ParseRule("kw", in, nil)
		if err != nil || !m.Ok || m.Str != in {
			t.Fatalf("expected :i match for %q, got m=%v err=%v", in, m, err)
		}
	}

	// NFC precomposed "café" and NFD decomposed "cafe\u0301"
	for _, in := range []string{"cafe", "caf\u00e9", "cafe\u0301"} {
		m, err := g.ParseRule("cafe", in, nil)
		if err != nil || !m.Ok || m.Str != in {
			t.Fatalf("expected :m match for %q, got m=%v err=%v", in, m, err)
		}
	}

	// Combined :i :m on "RÉSUMÉ" (NFC) and "Re\u0301sume\u0301" (NFD)
	for _, in := range []string{"RESUME", "R\u00c9SUM\u00c9", "Re\u0301sume\u0301"} {
		m, err := g.ParseRule("resume_im", in, nil)
		if err != nil || !m.Ok || m.Str != in {
			t.Fatalf("expected :i :m match for %q, got m=%v err=%v", in, m, err)
		}
	}

	// Inline scoped :i propagating across sibling '|' branches
	for _, in := range []string{"SELECT", "FrOm", "wHeRe"} {
		m, err := g.ParseRule("sql_alt", in, nil)
		if err != nil || !m.Ok {
			t.Fatalf("expected inline :i sibling propagation for %q, got err=%v", in, err)
		}
	}
}

func TestRakuInlineSigspaceToggle(t *testing.T) {
	src := `
grammar SigGram {
    token TOP { ^ 'id' [ :s '=' <digit>+ ] $ }
    rule NoSpacePair { ^ 'key' [ :!s ':' <ident> ] $ }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatalf("LoadGrammarFromString failed: %v", err)
	}
	if m, err := g.Parse("id = 42", nil); err != nil || !m.Ok {
		t.Fatalf("expected inline :s inside token to skip spaces: %v", err)
	}
	if m, err := g.ParseRule("NoSpacePair", "key:val", nil); err != nil || !m.Ok {
		t.Fatalf("expected inline :!s inside rule to match without space: %v", err)
	}
	if m, err := g.ParseRule("NoSpacePair", "key : val", nil); err == nil && m != nil && m.Ok {
		t.Fatalf("expected inline :!s inside rule to reject spaces around ':'")
	}
}

func TestRakuCaptureMarkersCutsRecursionAndBackrefs(t *testing.T) {
	src := `
grammar AdvancedPEG {
    # Capture markers <( ... )> narrow result span
    token quoted_inner { '"' <( <-["]>+ )> '"' }

    # Group/alt cut :: prevents trying later alternatives after commit
    token committed_stmt {
        [ 'let' >> :: \s+ <ident> '=' <digit>+ || 'let_fallback' .* ]
    }

    # Rule cut ::: aborts entire rule immediately on failure after :::
    token rule_cut {
        [ 'fn' ::: \s+ <ident> || 'fn_other' ]
    }

    # Self-recursion <~~> for balanced parentheses
    token balanced { '(' [ <-[()]>+ | <~~> ]* ')' }

    # Dynamic backreferences: positional $0 and named $<tag>
    token dup_word { (<ident>) \s+ $0 $ }
    token xml_elem { '<' $<tag>=<ident> '>' $<body_txt>=<-[<]>* '</' $<tag> '>' }

    # Word boundary <|w> / <!|w>, quoted subrule <'lit'>, ** ^N quantifier, conjunction &
    token three_alpha_no_if { ^ [ <alpha> ** 2..4 & <!before 'if' $> .* ] <|w> $ }
    token upto_three { ^ 'a' ** ^4 $ }
    token angle_lit { ^ <'hello'> <!|w> <'world'> $ }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatalf("LoadGrammarFromString failed: %v", err)
	}

	// 1. <( ... )> capture span marker
	m, err := g.ParseRule("quoted_inner", `"raptor_peg"`, nil)
	if err != nil || !m.Ok || m.Str != "raptor_peg" {
		t.Fatalf("expected <( ... )> to narrow Match.Str to 'raptor_peg', got %q (err=%v)", m.GetStr(), err)
	}

	// 2. :: cut operator
	if _, err := g.ParseRule("committed_stmt", "let x=10", nil); err != nil {
		t.Fatalf("expected valid 'let x=10' to match: %v", err)
	}
	if m2, err := g.ParseRule("committed_stmt", "let_fallback 123", nil); err != nil || !m2.Ok {
		t.Fatalf("expected 'let_fallback 123' (before cut) to match fallback: %v", err)
	}
	if m3, err := g.ParseRule("committed_stmt", "let broken", nil); err == nil && m3 != nil && m3.Ok {
		t.Fatalf("expected 'let broken' after :: cut to fail without falling back")
	}

	// 3. ::: rule cut
	if m4, err := g.ParseRule("rule_cut", "fn_other", nil); err == nil && m4 != nil && m4.Ok {
		t.Fatalf("expected 'fn' prefix followed by ::: cut to abort entire rule on 'fn_other'")
	}

	// 4. <~~> self-recursion
	if m5, err := g.ParseRule("balanced", "((a)(b(c)))", nil); err != nil || !m5.Ok || m5.Str != "((a)(b(c)))" {
		t.Fatalf("expected <~~> balanced parens match, got m=%v err=%v", m5, err)
	}

	// 5. Positional ($0) and Named ($<tag>) backreferences
	if m6, err := g.ParseRule("dup_word", "raku raku", nil); err != nil || !m6.Ok {
		t.Fatalf("expected $0 backref to match 'raku raku': %v", err)
	}
	if m7, err := g.ParseRule("dup_word", "raku perl", nil); err == nil && m7 != nil && m7.Ok {
		t.Fatalf("expected $0 backref to reject 'raku perl'")
	}
	m8, err := g.ParseRule("xml_elem", "<b>bold</b>", nil)
	if err != nil || !m8.Ok || m8.Get("tag").Str != "b" || m8.Get("body_txt").Str != "bold" {
		t.Fatalf("expected $<tag> xml_elem match, got m=%v err=%v", m8, err)
	}
	if m9, err := g.ParseRule("xml_elem", "<b>bold</i>", nil); err == nil && m9 != nil && m9.Ok {
		t.Fatalf("expected $<tag> xml_elem to reject mismatched closing tag")
	}

	// 6. Conjunction &, <|w>, ** ^4, <'lit'>, <!|w>
	if _, err := g.ParseRule("three_alpha_no_if", "for", nil); err != nil {
		t.Fatalf("expected conjunction to accept 'for': %v", err)
	}
	if m10, err := g.ParseRule("three_alpha_no_if", "if", nil); err == nil && m10 != nil && m10.Ok {
		t.Fatalf("expected conjunction to reject 'if'")
	}
	if _, err := g.ParseRule("upto_three", "aaa", nil); err != nil {
		t.Fatalf("expected ** ^4 (0..3) to accept 'aaa': %v", err)
	}
	if m11, err := g.ParseRule("upto_three", "aaaa", nil); err == nil && m11 != nil && m11.Ok {
		t.Fatalf("expected ** ^4 (0..3) to reject 'aaaa'")
	}
	if _, err := g.ParseRule("angle_lit", "helloworld", nil); err != nil {
		t.Fatalf("expected <'hello'> <!|w> <'world'> to match 'helloworld': %v", err)
	}
}

func (m *Match) GetStr() string {
	if m == nil {
		return ""
	}
	return m.Str
}
