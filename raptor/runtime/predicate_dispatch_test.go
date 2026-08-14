package raptor

import (
	"testing"
)

func TestSubsetDynamicRefinements(t *testing.T) {
	in := NewInterp()

	code := `
# Define named subsets
subset Positive where { $_ > 0 };
subset Even where { $_ % 2 == 0 };
subset PortNumber where { $_ >= 1 && $_ <= 65535 };

# Variable declaration with subset type contract
my Positive $valid = 42;
my Even $evenNum = 100;
my PortNumber $webPort = 8080;

# Smart matching against subsets
my $isEven = 42 ~~ Even;
my $isOddEven = 43 ~~ Even;
my $isPos = 10 ~~ Positive;

[$valid, $evenNum, $webPort, $isEven, $isOddEven, $isPos];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Subset eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 6 {
		t.Fatalf("expected 6 elements, got %+v", val)
	}

	if val.ArrayVal[0].IntVal != 42 {
		t.Errorf("expected 42, got %v", val.ArrayVal[0])
	}
	if val.ArrayVal[1].IntVal != 100 {
		t.Errorf("expected 100, got %v", val.ArrayVal[1])
	}
	if val.ArrayVal[2].IntVal != 8080 {
		t.Errorf("expected 8080, got %v", val.ArrayVal[2])
	}
	if !val.ArrayVal[3].IsTrue() {
		t.Errorf("expected true for 42 ~~ Even")
	}
	if val.ArrayVal[4].IsTrue() {
		t.Errorf("expected false for 43 ~~ Even")
	}
	if !val.ArrayVal[5].IsTrue() {
		t.Errorf("expected true for 10 ~~ Positive")
	}
}

func TestSubsetContractViolationError(t *testing.T) {
	in := NewInterp()

	code := `
subset Positive where { $_ > 0 };
my Positive $invalid = -10;
`
	_, err := in.Eval(code)
	if err == nil {
		t.Fatalf("expected dynamic constraint violation error, got nil")
	}
}

func TestPredicateDispatchingMultiSubs(t *testing.T) {
	in := NewInterp()

	code := `
subset Negative where { $_ < 0 };
subset Zero where { $_ == 0 };
subset Positive where { $_ > 0 };

# Disambiguate multi subs strictly on basis of subset predicates
multi sub describe(Negative $n) {
    return "negative: " ~ $n;
}

multi sub describe(Zero $n) {
    return "zero";
}

multi sub describe(Positive $n) {
    return "positive: " ~ $n;
}

# Inline where clauses for predicate dispatch
multi sub grade($score where { $score >= 90 }) {
    return "A";
}

multi sub grade($score where { $score >= 75 && $score < 90 }) {
    return "B";
}

multi sub grade($score where { $score < 75 }) {
    return "C";
}

my $d1 = describe(-5);
my $d2 = describe(0);
my $d3 = describe(42);

my $g1 = grade(95);
my $g2 = grade(82);
my $g3 = grade(60);

[$d1, $d2, $d3, $g1, $g2, $g3];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Predicate dispatch eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 6 {
		t.Fatalf("expected 6 elements, got %+v", val)
	}

	if val.ArrayVal[0].String() != "negative: -5" {
		t.Errorf("expected 'negative: -5', got %q", val.ArrayVal[0].String())
	}
	if val.ArrayVal[1].String() != "zero" {
		t.Errorf("expected 'zero', got %q", val.ArrayVal[1].String())
	}
	if val.ArrayVal[2].String() != "positive: 42" {
		t.Errorf("expected 'positive: 42', got %q", val.ArrayVal[2].String())
	}
	if val.ArrayVal[3].String() != "A" {
		t.Errorf("expected 'A', got %q", val.ArrayVal[3].String())
	}
	if val.ArrayVal[4].String() != "B" {
		t.Errorf("expected 'B', got %q", val.ArrayVal[4].String())
	}
	if val.ArrayVal[5].String() != "C" {
		t.Errorf("expected 'C', got %q", val.ArrayVal[5].String())
	}
}

func TestStructFunctionPointersAndClosures(t *testing.T) {
	in := NewInterp()

	code := `
struct Button {
    int32 $x;
    int32 $y;
    ptr $onClick;
}

my $btn = Button.new();
$btn.x = 100;
$btn.y = 200;

my $clickedVal = 0;
$btn.onClick = sub ($val) {
    $clickedVal = $val * 2;
    return $clickedVal;
};

my $res = $btn.onClick(21);

[$btn.x, $btn.y, $clickedVal, $res];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Struct function pointer eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 4 {
		t.Fatalf("expected 4 elements, got %+v", val)
	}

	if val.ArrayVal[0].IntVal != 100 {
		t.Errorf("expected x=100, got %v", val.ArrayVal[0])
	}
	if val.ArrayVal[1].IntVal != 200 {
		t.Errorf("expected y=200, got %v", val.ArrayVal[1])
	}
	if val.ArrayVal[2].IntVal != 42 {
		t.Errorf("expected clickedVal=42, got %v", val.ArrayVal[2])
	}
	if val.ArrayVal[3].IntVal != 42 {
		t.Errorf("expected res=42, got %v", val.ArrayVal[3])
	}
}

func TestCustomOperatorsOnStructs(t *testing.T) {
	in := NewInterp()

	code := `
struct Vec2 {
    int64 $x;
    int64 $y;
}

# Overload + for Vec2 + Vec2
multi sub infix:<+>(Vec2 $a, Vec2 $b) {
    my $r = Vec2.new();
    $r.x = $a.x + $b.x;
    $r.y = $a.y + $b.y;
    return $r;
}

# Overload * for Vec2 * scalar
multi sub infix:<*>(Vec2 $v, $scale) {
    my $r = Vec2.new();
    $r.x = $v.x * $scale;
    $r.y = $v.y * $scale;
    return $r;
}

# Overload prefix - for Vec2
multi sub prefix:<->(Vec2 $v) {
    my $r = Vec2.new();
    $r.x = -$v.x;
    $r.y = -$v.y;
    return $r;
}

my $v1 = Vec2.new();
$v1.x = 10; $v1.y = 20;

my $v2 = Vec2.new();
$v2.x = 5; $v2.y = 15;

my $sum = $v1 + $v2;
my $scaled = $v1 * 3;
my $neg = -$v2;

[$sum.x, $sum.y, $scaled.x, $scaled.y, $neg.x, $neg.y];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Custom struct operators eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 6 {
		t.Fatalf("expected 6 elements, got %+v", val)
	}

	if val.ArrayVal[0].IntVal != 15 || val.ArrayVal[1].IntVal != 35 {
		t.Errorf("expected sum (15, 35), got (%v, %v)", val.ArrayVal[0], val.ArrayVal[1])
	}
	if val.ArrayVal[2].IntVal != 30 || val.ArrayVal[3].IntVal != 60 {
		t.Errorf("expected scaled (30, 60), got (%v, %v)", val.ArrayVal[2], val.ArrayVal[3])
	}
	if val.ArrayVal[4].IntVal != -5 || val.ArrayVal[5].IntVal != -15 {
		t.Errorf("expected neg (-5, -15), got (%v, %v)", val.ArrayVal[4], val.ArrayVal[5])
	}
}

func TestContinuousAssignmentPredicateEnforcement(t *testing.T) {
	in := NewInterp()

	codeSuccess := `
subset Positive of Int where { $_ > 0 };
my Positive $balance = 100;
$balance = 250;
$balance += 50;
$balance;
`
	val, err := in.Eval(codeSuccess)
	if err != nil {
		t.Fatalf("expected valid assignment to pass, got error: %v", err)
	}
	if val.IntVal != 300 {
		t.Errorf("expected 300, got %v", val.IntVal)
	}

	codeViolation := `
subset Positive of Int where { $_ > 0 };
my Positive $score = 50;
$score = -10;
`
	_, err = in.Eval(codeViolation)
	if err == nil {
		t.Fatalf("expected assignment of -10 to Positive variable to fail with invariant violation")
	}

	codeWhereViolation := `
my Int $port where { $_ >= 1 && $_ <= 65535 } = 8080;
$port = 99999;
`
	_, err = in.Eval(codeWhereViolation)
	if err == nil {
		t.Fatalf("expected assignment of 99999 to port range variable to fail with invariant violation")
	}
}

