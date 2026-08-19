package raptor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	moargo "moarvm-go/engine"
)

func TestParseVerifyStatementForms(t *testing.T) {
	src := `
PRE $b != 0;
PRE { $b != 0 }
PRE $b != 0, "no zero";
POST { $res >= 0 }, "non-neg";
INVARIANT $n >= 0;
CHECK 1 + 1 == 2;
ASSERT { True }
TEST "math" { is 1, 1, "one"; }
PROPERTY "commute" ($a, $b) { return ($a + $b) == ($b + $a); }
SUBTEST "inner" { ok True, "yes"; }
`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"PRE", "PRE", "PRE", "POST", "INVARIANT", "CHECK", "ASSERT", "TEST", "PROPERTY", "SUBTEST"}
	var kinds []string
	for _, st := range prog.Stmts {
		v, ok := st.(*VerifyStmt)
		if !ok {
			t.Fatalf("stmt %T is not VerifyStmt", st)
		}
		kinds = append(kinds, v.Kind)
	}
	if len(kinds) != len(want) {
		t.Fatalf("got %d verify stmts %v, want %d %v", len(kinds), kinds, len(want), want)
	}
	for i, k := range want {
		if kinds[i] != k {
			t.Errorf("stmt %d: kind %s, want %s", i, kinds[i], k)
		}
	}
	preBlock := prog.Stmts[1].(*VerifyStmt)
	if preBlock.Body == nil {
		t.Fatal("PRE { } should have a body")
	}
	prop := prog.Stmts[8].(*VerifyStmt)
	if len(prop.Params) != 2 {
		t.Fatalf("PROPERTY params = %d, want 2", len(prop.Params))
	}
}

func TestParseTapStatementForms(t *testing.T) {
	src := `
plan 3;
ok 1 + 1 == 2, "sum";
ok { True }, "block cond";
is 2 * 3, 6, "product";
isnt "a", "b", "diff";
done_testing;
`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Stmts) != 6 {
		t.Fatalf("got %d stmts, want 6", len(prog.Stmts))
	}
	names := []string{"plan", "ok", "ok", "is", "isnt", "done_testing"}
	for i, st := range prog.Stmts {
		es, ok := st.(*ExprStmt)
		if !ok {
			t.Fatalf("stmt %d is %T, want ExprStmt", i, st)
		}
		call, ok := es.Expr.(*CallExpr)
		if !ok {
			t.Fatalf("stmt %d expr is %T, want CallExpr", i, es.Expr)
		}
		if calleeName(call) != names[i] {
			t.Errorf("stmt %d callee %q, want %q", i, calleeName(call), names[i])
		}
	}
	okBlock := prog.Stmts[2].(*ExprStmt).Expr.(*CallExpr)
	if _, ok := okBlock.Args[0].(*ClosureExpr); !ok {
		t.Fatalf("ok { } first arg is %T, want ClosureExpr", okBlock.Args[0])
	}
}

func TestPostStaysCallableIdent(t *testing.T) {
	src := `
sub post($x) { return $x + 1; }
post(41);
`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Stmts) != 2 {
		t.Fatalf("got %d stmts, want 2", len(prog.Stmts))
	}
	if _, ok := prog.Stmts[0].(*SubDeclStmt); !ok {
		t.Fatalf("first stmt is %T, want SubDeclStmt", prog.Stmts[0])
	}
	es, ok := prog.Stmts[1].(*ExprStmt)
	if !ok {
		t.Fatalf("second stmt is %T, want ExprStmt", prog.Stmts[1])
	}
	call, ok := es.Expr.(*CallExpr)
	if !ok {
		t.Fatalf("second expr is %T, want CallExpr", es.Expr)
	}
	if calleeName(call) != "post" {
		t.Fatalf("callee %q, want post", calleeName(call))
	}
	in := NewInterp()
	val, err := in.Eval(src)
	if err != nil {
		t.Fatalf("eval sub post: %v", err)
	}
	if val == nil || val.IntVal != 42 {
		t.Fatalf("sub post(41) = %v, want 42", val)
	}
}

func TestEvalVerifyAndTapNoParen(t *testing.T) {
	src := `
plan 4;
my $b = 4;
PRE $b != 0, "nonzero";
PRE { $b > 0 }
ok 1 + 1 == 2, "sum";
ok { $b > 0 }, "block ok";
is 2 * 3, 6, "product";
done_testing;
`
	in := NewInterp()
	var buf bytes.Buffer
	in.SetStdout(&buf)
	if _, err := in.Eval(src); err != nil {
		t.Fatalf("eval: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"1..4", "ok 1 - sum", "ok 2 - block ok", "ok 3 - product"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\n%s", want, out)
		}
	}
}

func TestLowercasePreIsNotKeyword(t *testing.T) {
	prog, err := ParseProgram("pre $b != 0;")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Stmts) != 1 {
		t.Fatalf("stmts = %d", len(prog.Stmts))
	}
	if _, ok := prog.Stmts[0].(*VerifyStmt); ok {
		t.Fatal("lowercase pre must not be a PRE statement")
	}
}

func TestEvalPreFailure(t *testing.T) {
	src := `
sub boom($b) {
    PRE $b != 0, "divisor cannot be zero";
    return 1 / $b;
}
boom(0);
`
	in := NewInterp()
	_, err := in.Eval(src)
	if err == nil {
		t.Fatal("expected PreconditionError")
	}
	if !strings.Contains(err.Error(), "PreconditionError") {
		t.Fatalf("want PreconditionError, got %v", err)
	}
}

func TestEvalUpdatedTapFiles(t *testing.T) {
	t.Setenv("RAPTOR_TEST_MODE", "1")
	files := []string{
		filepath.Join("..", "t", "01_operators.t"),
		filepath.Join("..", "t", "05_verification_contracts.t"),
		filepath.Join("..", "t", "11_verification_uppercase.t"),
		filepath.Join("..", "t", "24_redo.t"),
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		in := NewInterp()
		var buf bytes.Buffer
		in.SetStdout(&buf)
		if _, err := in.Eval(string(src)); err != nil {
			t.Errorf("%s eval: %v\n%s", f, err, buf.String())
			continue
		}
		out := buf.String()
		if strings.Contains(out, "not ok") {
			t.Errorf("%s TAP failure:\n%s", f, out)
		}
		if !strings.Contains(out, "ok ") {
			t.Errorf("%s produced no TAP ok lines:\n%s", f, out)
		}
		c := NewCompiler()
		if _, err := c.CompileScript(string(src)); err != nil {
			t.Errorf("%s moar compile: %v", f, err)
		}
	}
}

func TestMoarCompileVerifyAndTapNoParen(t *testing.T) {
	src := `
plan 2;
PRE 1 != 0, "nonzero";
ok 1 + 1 == 2, "sum";
ok { True }, "block";
done_testing;
`
	c := NewCompiler()
	if _, err := c.CompileScript(src); err != nil {
		t.Fatalf("moar compile: %v", err)
	}
}

func TestMoarPREAndPrefixCallRun(t *testing.T) {
	if moargo.FindMoarDLL() == "" {
		t.Skip("moar.dll not found")
	}
	src := `
sub pair($x, $y) {
    PRE $x > 0, "x positive";
    PRE { $y > 0 }
    return $x + $y;
}
say pair(2, 3);
say pair 4, 5;
say (pair 6, 7);
`
	in := NewInterp()
	var buf bytes.Buffer
	in.SetStdout(&buf)
	if _, err := in.EvalOnBackend(src, BackendMoar); err != nil {
		t.Fatalf("moar run: %v\n%s", err, buf.String())
	}
	got := strings.Fields(buf.String())
	want := []string{"5", "9", "13"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("moar output %v, want %v\n%s", got, want, buf.String())
	}
}
