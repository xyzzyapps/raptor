package raptor

import (
	"testing"
)

func TestStructAndUnion(t *testing.T) {
	in := NewInterp()
	code := `
struct Point {
    int32 $x;
    int32 $y;
}

my $p = Point.new();
$p.x = 120;
$p.y = 240;

union Variant {
    int64 $as_int;
    num64 $as_num;
}

my $v = Variant.new();
$v.as_int = 42;

[$p.x, $p.y, $v.as_int];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected array of 3 elements, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 120 {
		t.Errorf("x mismatch: %d", val.ArrayVal[0].IntVal)
	}
	if val.ArrayVal[1].IntVal != 240 {
		t.Errorf("y mismatch: %d", val.ArrayVal[1].IntVal)
	}
	if val.ArrayVal[2].IntVal != 42 {
		t.Errorf("union int mismatch: %d", val.ArrayVal[2].IntVal)
	}
}

func TestNativeCallSubAndStruct(t *testing.T) {
	in := NewInterp()
	code := `
struct SYSTEMTIME {
    uint16 $wYear;
    uint16 $wMonth;
    uint16 $wDayOfWeek;
    uint16 $wDay;
    uint16 $wHour;
    uint16 $wMinute;
    uint16 $wSecond;
    uint16 $wMilliseconds;
}

sub get_pid() returns uint32 is native('kernel32.dll') is symbol('GetCurrentProcessId') { * }
sub sys_time(SYSTEMTIME $st) returns void is native('kernel32.dll') is symbol('GetSystemTime') { * }

my $pid = get_pid();
my $st = SYSTEMTIME.new();
sys_time($st);

my $year = $st.wYear;
my $month = $st.wMonth;

[$pid, $year, $month];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected array of 3, got %v", val)
	}
	pid := val.ArrayVal[0].IntVal
	year := val.ArrayVal[1].IntVal
	month := val.ArrayVal[2].IntVal

	if pid <= 0 {
		t.Errorf("invalid pid: %d", pid)
	}
	if year < 2024 || year > 2030 {
		t.Errorf("invalid year: %d", year)
	}
	if month < 1 || month > 12 {
		t.Errorf("invalid month: %d", month)
	}
}

func TestWhereTypeRefinements(t *testing.T) {
	in := NewInterp()
	code := `
# 1. Variable with where constraint
my Int $port where { $_ > 1024 && $_ <= 65535 } = 8080;

# 2. Multi sub dispatch with where constraints (Recursive Factorial)
multi sub fact(Int $n where { $n == 0 }) {
    return 1;
}

multi sub fact(Int $n where { $n > 0 }) {
    return $n * fact($n - 1);
}

my $f0 = fact(0);
my $f5 = fact(5);

[$port, $f0, $f5];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected array of 3, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 8080 {
		t.Errorf("port mismatch: %d", val.ArrayVal[0].IntVal)
	}
	if val.ArrayVal[1].IntVal != 1 {
		t.Errorf("fact(0) mismatch: %d", val.ArrayVal[1].IntVal)
	}
	if val.ArrayVal[2].IntVal != 120 {
		t.Errorf("fact(5) mismatch: %d", val.ArrayVal[2].IntVal)
	}
}

func TestAssertInvariants(t *testing.T) {
	in := NewInterp()
	// Test passing assertion
	_, err := in.Eval(`
my $x = 100;
assert $x > 50, "x must be greater than 50";
`)
	if err != nil {
		t.Fatalf("passing assert failed: %v", err)
	}

	// Test failing assertion
	_, err = in.Eval(`
my $y = 10;
assert $y > 50, "y must be greater than 50";
`)
	if err == nil {
		t.Fatalf("expected assertion failure, got nil error")
	}
}

func TestAdviceHooksBeforeAfterAround(t *testing.T) {
	in := NewInterp()
	code := `
my @trace = [];

sub compute($a, $b) {
    push(@trace, "compute");
    return $a + $b;
}

before compute($a, $b) {
    push(@trace, "before");
}

after compute($a, $b) {
    push(@trace, "after");
}

around compute($orig, $a, $b) {
    push(@trace, "around_pre");
    my $res = $orig($a, $b);
    push(@trace, "around_post");
    return $res * 2;
}

my $result = compute(10, 20);
[$result, @trace];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected array of 2, got %v", val)
	}
	result := val.ArrayVal[0].IntVal
	if result != 60 {
		t.Errorf("expected around hook to double result to 60, got %d", result)
	}
}

func TestOperatorOverloadingOnStructs(t *testing.T) {
	in := NewInterp()
	code := `
struct Vec2 {
    int32 $x;
    int32 $y;
}

multi sub infix:<+>(Vec2 $a, Vec2 $b) {
    my $res = Vec2.new();
    $res.x = $a.x + $b.x;
    $res.y = $a.y + $b.y;
    return $res;
}

multi sub prefix:<->(Vec2 $v) {
    my $res = Vec2.new();
    $res.x = 0 - $v.x;
    $res.y = 0 - $v.y;
    return $res;
}

multi sub postcircumfix:<[ ]>(Vec2 $v, Int $idx) {
    if $idx == 0 { return $v.x; }
    return $v.y;
}

my $v1 = Vec2.new();
$v1.x = 10;
$v1.y = 20;

my $v2 = Vec2.new();
$v2.x = 30;
$v2.y = 40;

my $v3 = $v1 + $v2;
my $v4 = -$v1;
my $first = $v3[0];
my $second = $v3[1];

[$v3.x, $v3.y, $v4.x, $v4.y, $first, $second];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 6 {
		t.Fatalf("expected array of 6, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 40 || val.ArrayVal[1].IntVal != 60 {
		t.Errorf("v3 mismatch: %d, %d", val.ArrayVal[0].IntVal, val.ArrayVal[1].IntVal)
	}
	if val.ArrayVal[2].IntVal != -10 || val.ArrayVal[3].IntVal != -20 {
		t.Errorf("v4 mismatch: %d, %d", val.ArrayVal[2].IntVal, val.ArrayVal[3].IntVal)
	}
	if val.ArrayVal[4].IntVal != 40 || val.ArrayVal[5].IntVal != 60 {
		t.Errorf("subscript mismatch: %d, %d", val.ArrayVal[4].IntVal, val.ArrayVal[5].IntVal)
	}
}

func TestUnicodeOperatorsAndConstants(t *testing.T) {
	in := NewInterp()
	code := `
# Unicode relational and math operators
my $is_leq = 3 <= 5;
my $is_geq = 10 >= 4;
my $is_neq = 4 != 5;

# Unicode set operators
my $in_set = 3 ∈ [1, 2, 3];
my $not_in = 5 ∉ [1, 2, 3];
my @inter = [1, 2, 3] ∩ [2, 3, 4];
my @union = [1, 2] ∪ [2, 3];

# Unicode constants
my $circ = 2 * π;

[$is_leq, $is_geq, $is_neq, $in_set, $not_in, @inter.elems(), @union.elems(), $circ > 6.28];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 8 {
		t.Fatalf("expected array of 8, got %v", val)
	}
	for i := 0; i < 5; i++ {
		if !val.ArrayVal[i].IsTrue() {
			t.Errorf("element %d was not true", i)
		}
	}
	if val.ArrayVal[5].IntVal != 2 {
		t.Errorf("intersection elems mismatch: %d", val.ArrayVal[5].IntVal)
	}
	if val.ArrayVal[6].IntVal != 3 {
		t.Errorf("union elems mismatch: %d", val.ArrayVal[6].IntVal)
	}
	if !val.ArrayVal[7].IsTrue() {
		t.Errorf("pi calculation mismatch")
	}
}
