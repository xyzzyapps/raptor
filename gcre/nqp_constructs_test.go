package gcre

import (
	"strconv"
	"strings"
	"testing"
)

func TestNQPProtoAndSymLTM(t *testing.T) {
	src := `
grammar OpLang {
    rule TOP { ^ <op>+ $ }
    proto token op { * }
    token op:sym<+>   { <sym> }
    token op:sym<+=>  { <sym> }
    token op:sym<++>  { <sym> }
    token op:sym«>=»  { <sym> }
    token op:sym<<>> >> { <sym> }
    token op:sym[=]   { <sym> }
}
`
	// Note: test angle, french «», and bracket [] :sym delimiters:
	src = `
grammar OpLang {
    rule TOP { ^ <op>+ $ }
    proto token op { {*} }
    token op:sym<+>   { <sym> }
    token op:sym<+=>  { <sym> }
    token op:sym<++>  { <sym> }
    token op:sym«>=»  { <sym> }
    token op:sym[=]   { <sym> }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatalf("LoadGrammarFromString failed: %v", err)
	}

	m, err := g.Parse("+= ++ + >= =", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ops := m.GetAll("op")
	want := []string{"+=", "++", "+", ">=", "="}
	if len(ops) != len(want) {
		t.Fatalf("expected %d ops, got %d", len(want), len(ops))
	}
	for i, w := range want {
		if ops[i].Str != w {
			t.Errorf("op[%d].Str = %q, want %q", i, ops[i].Str, w)
		}
		if ops[i].Get("sym").Str != w {
			t.Errorf("op[%d]<sym> = %q, want %q", i, ops[i].Get("sym").Str, w)
		}
		wantRule := "op:sym<" + w + ">"
		if ops[i].Rule != wantRule {
			t.Errorf("op[%d].Rule = %q, want %q", i, ops[i].Rule, wantRule)
		}
	}
}

func TestNQPLTMVsSequentialAlternationAndCaptureRollback(t *testing.T) {
	src := `
grammar AltTest {
    token ltm_op { '+' | '+=' | '++' }
    token seq_op { '+' || '+=' || '++' }
    rule  rollback { ^ [ <first=ident> ':' | <second=ident> '=' ] <digit>+ $ }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// '|' uses LTM on declarative branches -> picks '+=' even though '+' is listed first.
	m1, err := g.ParseRule("ltm_op", "+=", nil)
	if err != nil || m1.Str != "+=" {
		t.Fatalf("ltm_op on '+=': got %q (err=%v), want '+='", m1.Str, err)
	}

	// '||' uses sequential first-match -> picks '+' immediately.
	m2, err := g.ParseRule("seq_op", "+=", nil)
	if err != nil || m2.Str != "+" {
		t.Fatalf("seq_op on '+=': got %q (err=%v), want '+'", m2.Str, err)
	}

	// Backtracking from branch 1 (<first=ident> ':') must roll back 'first' capture!
	m3, err := g.ParseRule("rollback", "alpha = 42", nil)
	if err != nil {
		t.Fatalf("rollback parse failed: %v", err)
	}
	if m3.Get("first").Ok {
		t.Fatalf("expected stale capture 'first' to be rolled back on failed branch, got %q", m3.Get("first").Str)
	}
	if m3.Get("second").Str != "alpha" {
		t.Fatalf("expected 'second' capture 'alpha', got %q", m3.Get("second").Str)
	}
}

type nqpCalcActions struct{}

func (a *nqpCalcActions) HandleAction(rule string, m *Match) {
	switch {
	case rule == "TOP":
		m.Make(m.Get("EXPR").Made)
	case rule == "term:sym<num>":
		n, _ := strconv.ParseInt(m.Str, 10, 64)
		m.Make(n)
	case rule == "term:sym<paren>":
		m.Make(m.Get("EXPR").Made)
	case strings.HasPrefix(rule, "prefix:sym<"):
		v := m.Get("term").Made.(int64)
		if m.Get("sym").Str == "-" {
			m.Make(-v)
		} else {
			m.Make(v)
		}
	case strings.HasPrefix(rule, "postfix:sym<"):
		v := m.Get("term").Made.(int64)
		if m.Get("sym").Str == "!" {
			res := int64(1)
			for i := int64(2); i <= v; i++ {
				res *= i
			}
			m.Make(res)
		}
	case strings.HasPrefix(rule, "infix:sym<"):
		l := m.Get("left").Made.(int64)
		r := m.Get("right").Made.(int64)
		switch m.Get("sym").Str {
		case "+":
			m.Make(l + r)
		case "-":
			m.Make(l - r)
		case "*":
			m.Make(l * r)
		case "/":
			m.Make(l / r)
		case "**":
			res := int64(1)
			for i := int64(0); i < r; i++ {
				res *= l
			}
			m.Make(res)
		case "<":
			if l < r {
				m.Make(int64(1))
			} else {
				m.Make(int64(0))
			}
		case "==":
			if l == r {
				m.Make(int64(1))
			} else {
				m.Make(int64(0))
			}
		case "and":
			if l != 0 && r != 0 {
				m.Make(int64(1))
			} else {
				m.Make(int64(0))
			}
		}
	}
}

func TestNQPDeclarativeOptableTraitsAndGoal(t *testing.T) {
	src := `
grammar NQPCalc {
    rule TOP { ^ <EXPR> $ }
    token EXPR is optable { <term> }

    proto token term { * }
    token term:sym<num>   { \d+ }
    rule  term:sym<paren> { '(' ~ ')' <EXPR> }

    proto token infix { * }
    token infix:sym<+>   { <sym> }
    token infix:sym<->   is equiv<+> { <sym> }
    token infix:sym<*>   is tighter<+> { <sym> }
    token infix:sym</>   is equiv<*> { <sym> }
    token infix:sym<**>  is tighter<*> is assoc<right> { <sym> }
    token infix:sym«<»   is looser<+> is assoc<non> { <sym> }
    token infix:sym<==>  is looser«<» is assoc<chain> { <sym> }
    token infix:sym<and> is looser<==> { <sym> >> }

    proto token prefix { * }
    token prefix:sym<->  is tighter<*> is looser<**> { <sym> }

    proto token postfix { * }
    token postfix:sym<!> is tighter<**> { <sym> }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatalf("load NQPCalc failed: %v", err)
	}
	act := &nqpCalcActions{}

	cases := []struct {
		expr string
		want int64
	}{
		{"2 + 3 * 4", 14},
		{"(2 + 3) * 4", 20},
		{"10 - 3 - 2", 5},       // left-associative
		{"2 ** 3 ** 2", 512},    // right-associative: 2 ** (3 ** 2)
		{"-2 ** 3", -8},         // prefix '-' is looser than '**': -(2 ** 3)
		{"-3! + 10", 4},         // postfix '!' tighter than '**' and '-': -(6) + 10 = 4
		{"1 < 2 and 3 < 4", 1},  // non-assoc '<' inside looser 'and'
		{"5 == 5", 1},
	}

	for _, tc := range cases {
		m, err := g.Parse(tc.expr, act)
		if err != nil {
			t.Fatalf("Parse(%q) failed: %v", tc.expr, err)
		}
		got, ok := m.Made.(int64)
		if !ok || got != tc.want {
			t.Errorf("Parse(%q).Made = %v (%T), want %d", tc.expr, m.Made, m.Made, tc.want)
		}
	}

	// Non-associative '<' must reject chaining '1 < 2 < 3'!
	if _, err := g.Parse("1 < 2 < 3", act); err == nil {
		t.Fatalf("expected non-associative '1 < 2 < 3' to fail")
	}

	// Goal '~' missing closing ')' must fail with goal diagnostic!
	if _, err := g.Parse("(2 + 3", act); err == nil || !strings.Contains(err.Error(), "goal") {
		t.Fatalf("expected goal diagnostic error for unclosed '(', got: %v", err)
	}

	// Word-boundary on 'and' (<sym> >>): "1 ander" must fail!
	if _, err := g.Parse("1 ander", act); err == nil {
		t.Fatalf("expected '1 ander' to fail word boundary on infix:sym<and>")
	}
}

func TestNQPQuantifiersSeparatorsAndCaptures(t *testing.T) {
	src := `
grammar QuantCap {
    token ipv4 { ^ [ $<octet>=[\d ** 1..3] ] ** 4 % '.' $ }
    token c_comment { '/*' .*? '*/' }
    rule  comments { ^ <c_comment>+ $ }
    rule  strict_list { ^ '[' $<item>=<ident>* % ',' ']' $ }
    rule  trail_list  { ^ '[' $<item>=<ident>+ %% ',' ']' $ }
    rule  pos_caps    { ^ ( <ident> ) '=' ( \d+ ) $ }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatalf("load QuantCap failed: %v", err)
	}

	// IPv4: 4 octets of 1..3 digits separated by '.'
	mIP, err := g.ParseRule("ipv4", "192.168.0.1", nil)
	if err != nil {
		t.Fatalf("ipv4 valid failed: %v", err)
	}
	octets := mIP.GetAll("octet")
	if len(octets) != 4 || octets[0].Str != "192" || octets[3].Str != "1" {
		t.Fatalf("unexpected octets: %+v", octets)
	}
	if _, err := g.ParseRule("ipv4", "192.168.0", nil); err == nil {
		t.Fatalf("expected 3-octet IPv4 to fail ** 4")
	}
	if _, err := g.ParseRule("ipv4", "192.168.0.1234", nil); err == nil {
		t.Fatalf("expected 4-digit octet to fail ** 1..3")
	}

	// Frugal .*? must not eat across two comments!
	mCom, err := g.ParseRule("comments", "/* one */ /* two */", nil)
	if err != nil {
		t.Fatalf("frugal comments failed: %v", err)
	}
	cs := mCom.GetAll("c_comment")
	if len(cs) != 2 || cs[0].Str != "/* one */" || cs[1].Str != "/* two */" {
		t.Fatalf("frugal quantifier ate too much: %+v", cs)
	}

	// Strict '%' vs trailing '%%'
	if mEmpty, err := g.ParseRule("strict_list", "[]", nil); err != nil || len(mEmpty.GetAll("item")) != 0 {
		t.Fatalf("strict_list [] failed: %v", err)
	}
	if mList, err := g.ParseRule("strict_list", "[a, b, c]", nil); err != nil || len(mList.GetAll("item")) != 3 {
		t.Fatalf("strict_list [a, b, c] failed: %v", err)
	}
	if _, err := g.ParseRule("strict_list", "[a, b, ]", nil); err == nil {
		t.Fatalf("expected strict '%%' to reject trailing comma")
	}
	if mTrail, err := g.ParseRule("trail_list", "[a, b, ]", nil); err != nil || len(mTrail.GetAll("item")) != 2 {
		t.Fatalf("expected '%%%%' to allow trailing comma, err=%v", err)
	}

	// Positional captures $0, $1
	mPos, err := g.ParseRule("pos_caps", "port = 8080", nil)
	if err != nil {
		t.Fatalf("pos_caps failed: %v", err)
	}
	if mPos.At(0).Str != "port" || mPos.At(1).Str != "8080" {
		t.Fatalf("positional captures mismatch: $0=%q $1=%q", mPos.At(0).Str, mPos.At(1).Str)
	}
}

func TestNQPLookaroundsAnchorsCharClassAlgebraAndUnicode(t *testing.T) {
	src := `
grammar RichRegex {
    token kw_if      { ^ << 'if' >> }
    token px_num     { \d+ <?before 'px'> }
    token not_assign { '=' <!before '='> }
    token unescaped_quote { <!after '\\'> '"' }
    token after_px   { <?after \d+> 'px' }
    token consonants { ^ <[a..z] - [aeiou]>+ $ }
    token cap_word   { ^ <:Lu> <:Ll>+ $ }
    token hex_esc    { ^ \x[41] \x[03B1] \h+ \D+ $ }
    token multiline  { [ ^^ $<line>=\N+ $$ \n? ]+ }
    token nqp_ws_var { ^ 'my' <.ws> <ident> $ }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatalf("load RichRegex failed: %v", err)
	}

	if _, err := g.ParseRule("kw_if", "if (x)", nil); err != nil {
		t.Errorf("kw_if should match 'if (x)': %v", err)
	}
	if _, err := g.ParseRule("kw_if", "iffy", nil); err == nil {
		t.Errorf("kw_if should NOT match 'iffy'")
	}

	if m, err := g.ParseRule("px_num", "42px", nil); err != nil || m.Str != "42" {
		t.Errorf("px_num should match '42' without consuming 'px': m=%+v err=%v", m, err)
	}
	if _, err := g.ParseRule("px_num", "42em", nil); err == nil {
		t.Errorf("px_num should reject '42em'")
	}

	if _, err := g.ParseRule("not_assign", "= 1", nil); err != nil {
		t.Errorf("not_assign should match '= 1': %v", err)
	}
	if _, err := g.ParseRule("not_assign", "== 1", nil); err == nil {
		t.Errorf("not_assign should reject '== 1'")
	}

	// Lookbehind tests at non-zero offset
	ctx1 := &Context{Src: []rune(`\"`), Pos: 1}
	if g.Subrule("unescaped_quote", ctx1).Ok {
		t.Errorf("unescaped_quote should fail after backslash")
	}
	ctx2 := &Context{Src: []rune(`a"`), Pos: 1}
	if !g.Subrule("unescaped_quote", ctx2).Ok {
		t.Errorf("unescaped_quote should succeed after 'a'")
	}
	ctx3 := &Context{Src: []rune(`120px`), Pos: 3}
	if !g.Subrule("after_px", ctx3).Ok {
		t.Errorf("after_px should succeed after digits")
	}

	// Character class subtraction <[a..z] - [aeiou]>
	if _, err := g.ParseRule("consonants", "rhythm", nil); err != nil {
		t.Errorf("consonants should match 'rhythm': %v", err)
	}
	if _, err := g.ParseRule("consonants", "raptor", nil); err == nil {
		t.Errorf("consonants should reject vowel in 'raptor'")
	}

	// Unicode general categories <:Lu> <:Ll>+
	if _, err := g.ParseRule("cap_word", "Raptor", nil); err != nil {
		t.Errorf("cap_word should match 'Raptor': %v", err)
	}
	if _, err := g.ParseRule("cap_word", "raptor", nil); err == nil {
		t.Errorf("cap_word should reject lowercase 'raptor'")
	}

	// \x[41]='A', \x[03B1]='α', \h+, \D+
	if _, err := g.ParseRule("hex_esc", "Aα \t xyz", nil); err != nil {
		t.Errorf("hex_esc failed: %v", err)
	}
	if _, err := g.ParseRule("hex_esc", "Aα 123", nil); err == nil {
		t.Errorf("hex_esc should reject digits under \\D+")
	}

	// Multiline ^^ / $$ and \N+
	mLines, err := g.ParseRule("multiline", "first line\nsecond line", nil)
	if err != nil || len(mLines.GetAll("line")) != 2 {
		t.Fatalf("multiline failed: err=%v lines=%v", err, mLines.GetAll("line"))
	}

	// Built-in NQP <.ws> (<!ww> \s*): requires whitespace between word chars!
	if _, err := g.ParseRule("nqp_ws_var", "my count", nil); err != nil {
		t.Errorf("nqp_ws_var should accept 'my count': %v", err)
	}
	if _, err := g.ParseRule("nqp_ws_var", "mycount", nil); err == nil {
		t.Errorf("nqp_ws_var should reject 'mycount' via <!ww> in <.ws>")
	}
}

func TestNQPGrammarInheritanceExtendsProtosAndOptable(t *testing.T) {
	src := `
grammar BaseExpr {
    rule TOP { ^ <EXPR> $ }
    token EXPR is optable { <term> }

    proto token term { * }
    token term:sym<num> { \d+ }

    proto token infix { * }
    token infix:sym<+> { <sym> }
    token infix:sym<*> is tighter<+> { <sym> }
}

grammar ChildExpr is BaseExpr {
    token term:sym<hex> { '0x' <xdigit>+ }
    token infix:sym<**> is tighter<*> is assoc<right> { <sym> }
}
`
	g, err := LoadGrammarFromString(src)
	if err != nil {
		t.Fatalf("load inherited grammar failed: %v", err)
	}
	if g.Name != "ChildExpr" || g.Parent != "BaseExpr" {
		t.Fatalf("unexpected grammar metadata: Name=%q Parent=%q", g.Name, g.Parent)
	}

	m, err := g.Parse("0x10 + 2 * 3 ** 2", nil)
	if err != nil {
		t.Fatalf("ChildExpr parse failed: %v", err)
	}
	if m.Get("EXPR").Get("sym").Str != "+" {
		t.Fatalf("expected top-level operator '+' in '0x10 + (2 * (3 ** 2))', got %q", m.Get("EXPR").Get("sym").Str)
	}
	rhs := m.Get("EXPR").Get("right")
	if rhs.Get("sym").Str != "*" || rhs.Get("right").Get("sym").Str != "**" {
		t.Fatalf("unexpected precedence tree in ChildExpr: rhs=%q", rhs.Str)
	}
}

func TestExampleNQPLangGrammar(t *testing.T) {
	g, err := LoadGrammarFromFile("examples/nqp_lang.raku")
	if err != nil {
		t.Fatalf("failed loading examples/nqp_lang.raku: %v", err)
	}
	if g.Name != "MiniNQP" || g.Parent != "ExprCore" {
		t.Fatalf("unexpected grammar metadata: Name=%q Parent=%q", g.Name, g.Parent)
	}

	prog := `
		let base = 0x10 + 2 * 3 ** 2;
		if (base == 34 and max(1, 2, 3) == 3) {
			return -2 ** 3 + base;
		}
	`
	m, err := g.Parse(prog, nil)
	if err != nil || !m.Ok {
		t.Fatalf("MiniNQP failed to parse valid program: %v", err)
	}
	stmts := m.GetAll("stmt")
	if len(stmts) != 2 {
		t.Fatalf("expected 2 top-level statements, got %d", len(stmts))
	}
	if stmts[0].Rule != "stmt:sym<let>" || stmts[0].Get("lhs").Str != "base" {
		t.Fatalf("unexpected first stmt: rule=%q lhs=%q", stmts[0].Rule, stmts[0].Get("lhs").Str)
	}
	if stmts[1].Rule != "stmt:sym<if>" || stmts[1].Get("cond").Get("sym").Str != "and" {
		t.Fatalf("unexpected second stmt: rule=%q cond.sym=%q", stmts[1].Rule, stmts[1].Get("cond").Get("sym").Str)
	}

	// Non-associative `==` chaining must be rejected
	if bad, err := g.Parse("let x = 1 == 2 == 3;", nil); err == nil && bad.Ok {
		t.Fatalf("expected non-associative '==' chain to fail")
	}

	// Unclosed goal '(' ~ ')' should mention ')'
	if _, err := g.Parse("let x = (1 + 2;", nil); err == nil || !strings.Contains(err.Error(), "')'") {
		t.Fatalf("expected goal error mentioning ')', got: %v", err)
	}
}
