package raptor

import (
	goruntime "runtime"
	"strings"
	"testing"
)

func TestInternSmallInts(t *testing.T) {
	a := IntValue(0)
	b := IntValue(0)
	if a != b {
		t.Fatal("IntValue(0) should intern")
	}
	if IntValue(256) != IntValue(256) {
		t.Fatal("IntValue(256) should intern")
	}
	if IntValue(-256) != IntValue(-256) {
		t.Fatal("IntValue(-256) should intern")
	}
	if IntValue(257) == IntValue(257) {
		t.Fatal("IntValue(257) should be a unique box")
	}
	if isInternedInt(a) && a.IntVal != 0 {
		t.Fatalf("interned 0 mutated to %d", a.IntVal)
	}
}

func TestInPlaceIntAddAssign(t *testing.T) {
	in := NewInterp()
	val, err := in.Eval(`
my $n = 0;
loop (my $i = 1; $i <= 1000; $i += 1) {
    $n += $i;
}
$n;
`)
	if err != nil {
		t.Fatal(err)
	}
	if val.IntVal != 500500 {
		t.Fatalf("sum 1..1000 = %d", val.IntVal)
	}
	if isInternedInt(IntValue(0)) && IntValue(0).IntVal != 0 {
		t.Fatalf("interned 0 leaked: %d", IntValue(0).IntVal)
	}
}

func TestStrcatAST(t *testing.T) {
	prog, err := ParseProgram("my $s = \"\"; $s = $s ~ \"x\";")
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Stmts) < 2 {
		t.Fatalf("stmts %d", len(prog.Stmts))
	}
	es, ok := prog.Stmts[1].(*ExprStmt)
	if !ok {
		t.Fatalf("stmt1 %T", prog.Stmts[1])
	}
	t.Logf("expr %T", es.Expr)
	if as, ok := es.Expr.(*AssignStmt); ok {
		t.Logf("op=%q target=%T value=%T", as.Op, as.Target, as.Value)
		if be, ok := as.Value.(*BinaryExpr); ok {
			t.Logf("binop=%q left=%T right=%T", be.Op, be.Left, be.Right)
		}
	}
	if be, ok := es.Expr.(*BinaryExpr); ok {
		t.Logf("binop=%q left=%T right=%T", be.Op, be.Left, be.Right)
	}
}

func TestInPlaceStringConcat(t *testing.T) {
	in := NewInterp()
	val, err := in.Eval(`
my $s = "";
loop (my $i = 1; $i <= 200; $i += 1) {
    $s = $s ~ "x";
}
$s ~= "y";
$s.chars();
`)
	if err != nil {
		t.Fatal(err)
	}
	if val.IntVal != 201 {
		t.Fatalf("chars = %d", val.IntVal)
	}
}

func TestIntArrayLane(t *testing.T) {
	in := NewInterp()
	val, err := in.Eval(`
my @nums = [];
loop (my $i = 1; $i <= 20; $i += 1) {
    push(@nums, $i);
}
my @s = @nums.sort();
[@s[0], @s[19], @s.elems()];
`)
	if err != nil {
		t.Fatal(err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("bad result %+v", val)
	}
	if val.ArrayVal[0].IntVal != 1 || val.ArrayVal[1].IntVal != 20 || val.ArrayVal[2].IntVal != 20 {
		t.Fatalf("got %v", val)
	}
}

func TestBenchMemoryChurn(t *testing.T) {
	type row struct {
		name string
		src  string
	}
	rows := []row{
		{"loopsum", `my $total = 0; loop (my $i = 1; $i <= 1000000; $i += 1) { $total += $i; } $total;`},
		{"strcat", `my $s = ""; loop (my $i = 1; $i <= 50000; $i += 1) { $s = $s ~ "x"; } $s.chars();`},
		{"streq", `my $a = "delta"; my $b = "delt" ~ "a"; my $c = 0; loop (my $i = 1; $i <= 1000000; $i += 1) { if $a eq $b { $c = $c + 1; } if $a lt $b { $c = $c - 1; } } $c;`},
		{"sortnums", `my @nums = []; loop (my $i = 1; $i <= 50000; $i += 1) { push(@nums, ($i * 2654435761) % 100000); } my @sorted = @nums.sort(); @sorted.elems();`},
	}
	for _, r := range rows {
		var before, after goruntime.MemStats
		goruntime.GC()
		goruntime.ReadMemStats(&before)
		in := NewInterp()
		var buf strings.Builder
		in.SetStdout(&buf)
		if _, err := in.Eval(r.src); err != nil {
			t.Fatalf("%s: %v", r.name, err)
		}
		goruntime.ReadMemStats(&after)
		t.Logf("%s TotalAlloc=%d MB mallocs=%d liveHeap=%d MB",
			r.name,
			(after.TotalAlloc-before.TotalAlloc)/1e6,
			after.Mallocs-before.Mallocs,
			after.HeapAlloc/1e6)
	}
}

func TestRangeWhenSmartMatch(t *testing.T) {
	in := NewInterp()
	var buf strings.Builder
	in.SetStdout(&buf)
	_, err := in.Eval(`
my $score = 95;
my $cat = "x";
given $score {
    when 90..100 { $cat = "A"; }
    default { $cat = "Z"; }
}
say $cat;
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "A") {
		t.Fatalf("got %q", buf.String())
	}
}
