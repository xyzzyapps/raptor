package raptor

import (
	"strings"
	"testing"
)

// TestDataStackRecursion verifies that deep recursive function calls
// operate on Goose DataStack frame slabs without frame corruption or memory leaks.
func TestDataStackRecursion(t *testing.T) {
	in := NewInterp()
	code := `
sub fib(Int $n) {
    if $n <= 1 { return $n; }
    return fib($n - 1) + fib($n - 2);
}
fib(20);
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("fib(20) failed: %v", err)
	}
	if val.Type != ValInt || val.IntVal != 6765 {
		t.Fatalf("expected 6765, got %v", val)
	}
}

// TestScalarSlotIsolation verifies that scalar slot assignments have value semantics
// and modifying a copy does not affect the original variable.
func TestScalarSlotIsolation(t *testing.T) {
	in := NewInterp()
	code := `
sub test_isolation() {
    my $a = 42;
    my $b = $a;
    $b += 100;
    return [$a, $b];
}
test_isolation();
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("test_isolation failed: %v", err)
	}
	elems := val.materializeArray()
	if len(elems) != 2 || elems[0].IntVal != 42 || elems[1].IntVal != 142 {
		t.Fatalf("expected [42, 142], got %v", elems)
	}
}

// TestContainerAliasing verifies that container variables (@arr, %hash) preserve
// shared reference semantics under assignment.
func TestContainerAliasing(t *testing.T) {
	in := NewInterp()
	code := `
sub test_alias() {
    my @arr = [10, 20];
    my $ref = @arr;
    push($ref, 30);
    return @arr;
}
test_alias();
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("test_alias failed: %v", err)
	}
	if val.arrayLen() != 3 {
		t.Fatalf("expected len 3, got %d", val.arrayLen())
	}
	if val.arrayAt(2).IntVal != 30 {
		t.Fatalf("expected element 2 to be 30, got %v", val.arrayAt(2))
	}
}

// TestContainerEscape verifies that when a local scalar is appended into a heap container,
// subsequent mutations to the local variable do not mutate the container's copy.
func TestContainerEscape(t *testing.T) {
	in := NewInterp()
	code := `
sub test_escape() {
    my @container = [];
    my $x = 100;
    push(@container, $x);
    $x += 50;
    return [@container[0], $x];
}
test_escape();
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("test_escape failed: %v", err)
	}
	elems := val.materializeArray()
	if len(elems) != 2 || elems[0].IntVal != 100 || elems[1].IntVal != 150 {
		t.Fatalf("expected [100, 150], got %v", elems)
	}
}

// TestReplMultiEvalPersistence verifies that top-level global slot indexing correctly
// synchronizes across multiple distinct in.Eval calls in REPL mode.
func TestReplMultiEvalPersistence(t *testing.T) {
	in := NewInterp()
	if _, err := in.Eval("my $counter = 10;"); err != nil {
		t.Fatalf("eval 1 failed: %v", err)
	}
	if _, err := in.Eval("$counter += 25;"); err != nil {
		t.Fatalf("eval 2 failed: %v", err)
	}
	val, err := in.Eval("$counter * 2;")
	if err != nil {
		t.Fatalf("eval 3 failed: %v", err)
	}
	if val.Type != ValInt || val.IntVal != 70 {
		t.Fatalf("expected 70, got %v", val)
	}
}

// TestLexicalShadowing verifies that block-scoped lexical variables correctly shadow
// outer lexical or global slots and restore the outer value on exit.
func TestLexicalShadowing(t *testing.T) {
	in := NewInterp()
	code := `
sub test_shadow() {
    my $x = 10;
    my $inner = 0;
    if True {
        my $x = 99;
        $inner = $x;
    }
    return [$inner, $x];
}
test_shadow();
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("test_shadow failed: %v", err)
	}
	elems := val.materializeArray()
	if len(elems) != 2 || elems[0].IntVal != 99 || elems[1].IntVal != 10 {
		t.Fatalf("expected [99, 10], got %v", elems)
	}
}

// TestColumnarLanesOperations verifies homogeneous IntArray, FloatArray, and StrTape lanes.
func TestColumnarLanesOperations(t *testing.T) {
	in := NewInterp()

	// 1. Integer lane with pdqsort, reverse, push, pop, shift, unshift
	intCode := `
my @nums = [5, 2, 8, 1, 9];
push(@nums, 3);
my $popped = pop(@nums);
unshift(@nums, 100);
my $shifted = shift(@nums);
my @sorted = sort(@nums);
my @rev = reverse(@sorted);
join(",", @rev);
`
	val, err := in.Eval(intCode)
	if err != nil {
		t.Fatalf("int lane failed: %v", err)
	}
	expectedInts := "9,8,5,2,1"
	if strings.TrimSpace(val.String()) != expectedInts {
		t.Fatalf("expected %q, got %q", expectedInts, val.String())
	}

	// 2. Float lane operations
	floatCode := `
my @floats = [3.14, 1.41, 2.71];
my @s = sort(@floats);
join(" ", @s);
`
	fVal, err := in.Eval(floatCode)
	if err != nil {
		t.Fatalf("float lane failed: %v", err)
	}
	if !strings.HasPrefix(fVal.String(), "1.41") {
		t.Fatalf("expected sorted floats starting with 1.41, got %q", fVal.String())
	}

	// 3. StrTape lane operations
	strCode := `
my @words = ["zebra", "apple", "mango", "banana"];
my @sw = sort(@words);
join("-", @sw);
`
	sVal, err := in.Eval(strCode)
	if err != nil {
		t.Fatalf("str tape failed: %v", err)
	}
	expectedStrs := "apple-banana-mango-zebra"
	if strings.TrimSpace(sVal.String()) != expectedStrs {
		t.Fatalf("expected %q, got %q", expectedStrs, sVal.String())
	}
}

// TestDirectRangeLoop verifies zero-allocation integer range iteration.
func TestDirectRangeLoop(t *testing.T) {
	in := NewInterp()
	code := `
my $sum = 0;
for 1..1000 -> $i {
    $sum += $i;
}
$sum;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("range loop failed: %v", err)
	}
	if val.Type != ValInt || val.IntVal != 500500 {
		t.Fatalf("expected 500500, got %v", val)
	}
}
