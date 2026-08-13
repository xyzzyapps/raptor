package raptor

import (
	"testing"
)

func TestJunctionComparisonsAndConditionals(t *testing.T) {
	in := NewInterp()

	code := `
my $x = 5;
my $is_valid = ($x == any(1, 3, 5, 7));
my $all_match = (all(2, 4, 6) > 0);
my $one_match = ($x == one(5, 10, 15));
my $none_negative = (none(1, 2, 3) < 0);

[$is_valid, $all_match, $one_match, $none_negative];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 4 {
		t.Fatalf("expected 4 elements, got %+v", val)
	}
	for i, v := range val.ArrayVal {
		if !v.IsTrue() {
			t.Errorf("junction check [%d] failed: expected True, got %v", i, v)
		}
	}
}

func TestJunctionInIfStatements(t *testing.T) {
	in := NewInterp()

	code := `
my $score = 85;
my $grade = "F";
if $score > all(80, 70, 60) {
    $grade = "A";
}
$grade;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.String() != "A" {
		t.Fatalf("expected 'A', got %q", val.String())
	}
}

func TestSmartMatchWithJunctions(t *testing.T) {
	in := NewInterp()

	code := `
my $val = 42;
my $matches = ($val ~~ any(10, 20, 42));
my $no_match = ($val ~~ none(1, 2, 3));

[$matches, $no_match];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected 2 elements, got %+v", val)
	}
	if !val.ArrayVal[0].IsTrue() || !val.ArrayVal[1].IsTrue() {
		t.Fatalf("expected [True, True], got %+v", val)
	}
}

func TestArrayParameterDestructuring(t *testing.T) {
	in := NewInterp()

	code := `
sub process_list([$head, *@tail]) {
    return [$head, @tail.elems];
}

my $res = process_list([100, 200, 300, 400]);
$res;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected 2 elements, got %+v", val)
	}
	if val.ArrayVal[0].IntVal != 100 || val.ArrayVal[1].IntVal != 3 {
		t.Fatalf("expected [100, 3], got [%v, %v]", val.ArrayVal[0], val.ArrayVal[1])
	}
}

func TestHashParameterDestructuring(t *testing.T) {
	in := NewInterp()

	code := `
sub greet_user(:{:$name, :$role}) {
    return $name ~ " is " ~ $role;
}

my $u = {:name => "Alice", :role => "Admin"};
my $msg = greet_user($u);
$msg;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.String() != "Alice is Admin" {
		t.Fatalf("expected 'Alice is Admin', got %q", val.String())
	}
}

func TestGatherTakeGenerator(t *testing.T) {
	in := NewInterp()

	code := `
my $seq = gather {
    take 10;
    take 20;
    for 1..3 -> $i {
        take $i * 100;
    }
    take 50;
};

$seq;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValLazySeq || val.LazySeqVal == nil {
		t.Fatalf("expected LazySeq, got %+v", val)
	}
	if len(val.LazySeqVal.Items) != 6 {
		t.Fatalf("expected 6 items, got %d", len(val.LazySeqVal.Items))
	}
	expected := []int64{10, 20, 100, 200, 300, 50}
	for i, exp := range expected {
		if val.LazySeqVal.Items[i].IntVal != exp {
			t.Errorf("item [%d] mismatch: expected %d, got %v", i, exp, val.LazySeqVal.Items[i])
		}
	}
}


func TestCustomInfixAndPrefixOperators(t *testing.T) {
	in := NewInterp()

	code := `
sub infix:<xor>(Int $a, Int $b) {
    if ($a != 0 && $b == 0) || ($a == 0 && $b != 0) {
        return 1;
    }
    return 0;
}

my $t1 = 1 xor 0;
my $t2 = 1 xor 1;

[$t1, $t2];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected 2 elements, got %+v", val)
	}
	if val.ArrayVal[0].IntVal != 1 || val.ArrayVal[1].IntVal != 0 {
		t.Fatalf("expected [1, 0], got [%v, %v]", val.ArrayVal[0], val.ArrayVal[1])
	}
}
