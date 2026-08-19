package raptor

import (
	"bytes"
	"strings"
	"testing"

	moargo "moarvm-go/engine"
)

func firstCall(t *testing.T, src string) *CallExpr {
	t.Helper()
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	if len(prog.Stmts) != 1 {
		t.Fatalf("%q: %d stmts, want 1", src, len(prog.Stmts))
	}
	es, ok := prog.Stmts[0].(*ExprStmt)
	if !ok {
		t.Fatalf("%q: stmt is %T", src, prog.Stmts[0])
	}
	call, ok := es.Expr.(*CallExpr)
	if !ok {
		t.Fatalf("%q: expr is %T, want CallExpr", src, es.Expr)
	}
	return call
}

func TestPrefixCallFormsParseEqual(t *testing.T) {
	forms := []string{
		`pair($a, $b)`,
		`pair $a, $b`,
		`(pair $a, $b)`,
	}
	for _, src := range forms {
		call := firstCall(t, src)
		if calleeName(call) != "pair" {
			t.Errorf("%s: callee %q, want pair", src, calleeName(call))
		}
		if len(call.Args) != 2 {
			t.Errorf("%s: %d args, want 2", src, len(call.Args))
			continue
		}
		a, ok := call.Args[0].(*VarExpr)
		if !ok || a.Name != "$a" {
			t.Errorf("%s: arg0 = %#v, want $a", src, call.Args[0])
		}
		b, ok := call.Args[1].(*VarExpr)
		if !ok || b.Name != "$b" {
			t.Errorf("%s: arg1 = %#v, want $b", src, call.Args[1])
		}
	}
}

func TestPrefixCallFormsEvalEqual(t *testing.T) {
	src := `
sub pair($x, $y) { return $x + $y; }
my $a = pair(1, 2);
my $b = pair 1, 2;
my $c = (pair 1, 2);
my $d = pair(1, (pair 3, 4));
my $e = pair 1, pair 3, 4;
my $f = (pair 1, (pair 3, 4));
say $a; say $b; say $c; say $d; say $e; say $f;
`
	in := NewInterp()
	var buf bytes.Buffer
	in.SetStdout(&buf)
	if _, err := in.Eval(src); err != nil {
		t.Fatalf("eval: %v\n%s", err, buf.String())
	}
	got := strings.Fields(buf.String())
	want := []string{"3", "3", "3", "8", "8", "8"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, want %v\n%s", got, want, buf.String())
	}
}

func TestPrefixCallGroupingUnchanged(t *testing.T) {
	src := `(1 + 2)`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("parse grouping: %v", err)
	}
	es := prog.Stmts[0].(*ExprStmt)
	if _, ok := es.Expr.(*BinaryExpr); !ok {
		t.Fatalf("(1 + 2) is %T, want BinaryExpr", es.Expr)
	}

	src = `($a + $b)`
	prog, err = ParseProgram(src)
	if err != nil {
		t.Fatalf("parse ($a + $b): %v", err)
	}
	es = prog.Stmts[0].(*ExprStmt)
	if _, ok := es.Expr.(*BinaryExpr); !ok {
		t.Fatalf("($a + $b) is %T, want BinaryExpr", es.Expr)
	}

	src = `fib($n - 1)`
	prog, err = ParseProgram(src)
	if err != nil {
		t.Fatalf("parse fib($n - 1): %v", err)
	}
	call := prog.Stmts[0].(*ExprStmt).Expr.(*CallExpr)
	if calleeName(call) != "fib" || len(call.Args) != 1 {
		t.Fatalf("fib($n - 1) = %s/%d", calleeName(call), len(call.Args))
	}
	if _, ok := call.Args[0].(*BinaryExpr); !ok {
		t.Fatalf("fib arg is %T, want BinaryExpr", call.Args[0])
	}
}

func TestMoarPrefixCallRuns(t *testing.T) {
	if moargo.FindMoarDLL() == "" {
		t.Skip("moar.dll not found")
	}
	src := `
sub pair($x, $y) { return $x + $y; }
say pair(1, 2);
say pair 3, 4;
say (pair 5, 6);
`
	in := NewInterp()
	var buf bytes.Buffer
	in.SetStdout(&buf)
	if _, err := in.EvalOnBackend(src, BackendMoar); err != nil {
		t.Fatalf("moar run: %v\n%s", err, buf.String())
	}
	got := strings.Fields(buf.String())
	want := []string{"3", "7", "11"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("moar output %v, want %v\n%s", got, want, buf.String())
	}
}
