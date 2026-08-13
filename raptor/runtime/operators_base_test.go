package raptor

import (
	"testing"
)

func TestOperatorsSuite(t *testing.T) {
	in := NewInterp()

	code := `
# 1. Defined-or and //=
my $undef;
my $v1 = $undef // "fallback";
my $def = "exists";
my $v2 = $def // "fallback";

my $target;
$target //= 42;
$target //= 100;

# 2. Exponentiation
my $pow = 2 ** 10;

# 3. Ternary (Raku & Perl5 styles)
my $t1 = 1 > 0 ?? "yes" !! "no";
my $t2 = 0 > 1 ? "yes" : "no";

# 4. Bitwise numeric
my $band = 0xFF +& 0x0F;
my $bor  = 0xF0 +| 0x0F;
my $bxor = 0xFF +^ 0x0F;
my $bshl = 1 +< 4;
my $bshr = 16 +> 2;

# 5. Repetition
my $strRep = "ab" x 3;
my $arrRep = [1, 2] xx 3;

# 6. Divisibility & Modulo
my $divRes = 25 div 4;
my $modRes = 25 mod 4;
my $isDiv = 25 %% 5;

# 7. Min / Max
my $m1 = 10 min 20;
my $m2 = 10 max 20;

# 8. Regex match
my $mMatch = "hello raptor 123" =~ "\\d+";
my $mNotMatch = "hello" !~ "\\d+";

[$v1, $v2, $target, $pow, $t1, $t2, $band, $bor, $bxor, $bshl, $bshr, $strRep, $arrRep.elems, $divRes, $modRes, $isDiv, $m1, $m2, $mMatch, $mNotMatch];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Operators eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 20 {
		t.Fatalf("expected 20 elements, got %+v", val)
	}

	if val.ArrayVal[0].String() != "fallback" {
		t.Errorf("expected 'fallback', got %q", val.ArrayVal[0].String())
	}
	if val.ArrayVal[1].String() != "exists" {
		t.Errorf("expected 'exists', got %q", val.ArrayVal[1].String())
	}
	if val.ArrayVal[2].IntVal != 42 {
		t.Errorf("expected 42, got %v", val.ArrayVal[2])
	}
	if val.ArrayVal[3].FloatVal != 1024 {
		t.Errorf("expected 1024, got %v", val.ArrayVal[3])
	}
	if val.ArrayVal[4].String() != "yes" {
		t.Errorf("expected 'yes', got %q", val.ArrayVal[4].String())
	}
	if val.ArrayVal[5].String() != "no" {
		t.Errorf("expected 'no', got %q", val.ArrayVal[5].String())
	}
	if val.ArrayVal[6].IntVal != 15 {
		t.Errorf("expected 15, got %v", val.ArrayVal[6])
	}
	if val.ArrayVal[7].IntVal != 255 {
		t.Errorf("expected 255, got %v", val.ArrayVal[7])
	}
	if val.ArrayVal[8].IntVal != 240 {
		t.Errorf("expected 240, got %v", val.ArrayVal[8])
	}
	if val.ArrayVal[9].IntVal != 16 {
		t.Errorf("expected 16, got %v", val.ArrayVal[9])
	}
	if val.ArrayVal[10].IntVal != 4 {
		t.Errorf("expected 4, got %v", val.ArrayVal[10])
	}
	if val.ArrayVal[11].String() != "ababab" {
		t.Errorf("expected 'ababab', got %q", val.ArrayVal[11].String())
	}
	if val.ArrayVal[12].IntVal != 6 {
		t.Errorf("expected 6 elements for [1, 2] xx 3, got %v", val.ArrayVal[12])
	}
	if val.ArrayVal[13].IntVal != 6 {
		t.Errorf("expected 6 for 25 div 4, got %v", val.ArrayVal[13])
	}
	if val.ArrayVal[14].IntVal != 1 {
		t.Errorf("expected 1 for 25 mod 4, got %v", val.ArrayVal[14])
	}
	if !val.ArrayVal[15].IsTrue() {
		t.Errorf("expected true for 25 %% 5, got %v", val.ArrayVal[15])
	}
	if val.ArrayVal[16].IntVal != 10 {
		t.Errorf("expected 10 for min, got %v", val.ArrayVal[16])
	}
	if val.ArrayVal[17].IntVal != 20 {
		t.Errorf("expected 20 for max, got %v", val.ArrayVal[17])
	}
	if !val.ArrayVal[18].IsTrue() {
		t.Errorf("expected true for regex match, got %v", val.ArrayVal[18])
	}
	if !val.ArrayVal[19].IsTrue() {
		t.Errorf("expected true for regex not match, got %v", val.ArrayVal[19])
	}
}

func TestBaseFunctionsSuite(t *testing.T) {
	in := NewInterp()

	code := `
# Environment
setenv("RAPTOR_TEST_VAR", "4242");
my $envRead = getenv("RAPTOR_TEST_VAR");
my $envHashRead = %*ENV<RAPTOR_TEST_VAR>;

# Math
my $a = abs(-42);
my $s = sqrt(144);
my $cl = clamp(150, 0, 100);

# Strings
my $sub = substr("Hello Raptor", 6, 6);
my $sp = split(",", "apple,banana,cherry");
my $j = join(":", $sp);
my $u = uc("raptor");

# Lists
my @list = [10, 20, 30];
push(@list, 40);
my $popped = pop(@list);
my $sh = shift(@list);
unshift(@list, 5);

# Hashes
my %h = {:a => 1, :b => 2};
my @ks = keys(%h);
my @vs = values(%h);
my %inv = invert(%h);

[$envRead, $envHashRead, $a, $s, $cl, $sub, $j, $u, $popped, $sh, @list.elems, %inv<1>];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Base functions eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 12 {
		t.Fatalf("expected 12 elements, got %+v", val)
	}

	if val.ArrayVal[0].String() != "4242" {
		t.Errorf("expected '4242', got %q", val.ArrayVal[0].String())
	}
	if val.ArrayVal[1].String() != "4242" {
		t.Errorf("expected '4242' from %%*ENV, got %q", val.ArrayVal[1].String())
	}
	if val.ArrayVal[2].IntVal != 42 {
		t.Errorf("expected 42, got %v", val.ArrayVal[2])
	}
	if val.ArrayVal[3].FloatVal != 12 {
		t.Errorf("expected 12, got %v", val.ArrayVal[3])
	}
	if val.ArrayVal[4].IntVal != 100 {
		t.Errorf("expected 100, got %v", val.ArrayVal[4])
	}
	if val.ArrayVal[5].String() != "Raptor" {
		t.Errorf("expected 'Raptor', got %q", val.ArrayVal[5].String())
	}
	if val.ArrayVal[6].String() != "apple:banana:cherry" {
		t.Errorf("expected 'apple:banana:cherry', got %q", val.ArrayVal[6].String())
	}
	if val.ArrayVal[7].String() != "RAPTOR" {
		t.Errorf("expected 'RAPTOR', got %q", val.ArrayVal[7].String())
	}
	if val.ArrayVal[8].IntVal != 40 {
		t.Errorf("expected 40, got %v", val.ArrayVal[8])
	}
	if val.ArrayVal[9].IntVal != 10 {
		t.Errorf("expected 10, got %v", val.ArrayVal[9])
	}
	if val.ArrayVal[10].IntVal != 3 {
		t.Errorf("expected 3 elements in @list, got %v", val.ArrayVal[10])
	}
	if val.ArrayVal[11].String() != "a" {
		t.Errorf("expected 'a' from inverted hash, got %q", val.ArrayVal[11].String())
	}
}

func TestAdvancedConcurrencyPrimitives(t *testing.T) {
	in := NewInterp()

	code := `
# Mutex
my $mtx = mutex_create();
mutex_lock($mtx);
mutex_unlock($mtx);

# WaitGroup
my $wg = waitgroup_create();
waitgroup_add($wg, 2);
waitgroup_done($wg);
waitgroup_done($wg);
waitgroup_wait($wg);

# Parallel Map
my $inputs = [1, 2, 3, 4, 5];
my $results = parallel_map($inputs, sub ($x, $idx) {
    return $x * 10;
}, 4);

# Supply / Reactive Stream
my $sup = supply_create();
my $collected = 0;
supply_tap($sup, sub ($val) {
    $collected = $collected + $val;
});

supply_emit($sup, 10);
supply_emit($sup, 20);
supply_done($sup);

[$results[0], $results[4], $collected];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Concurrency eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected 3 results, got %+v", val)
	}

	if val.ArrayVal[0].IntVal != 10 {
		t.Errorf("expected 10, got %v", val.ArrayVal[0])
	}
	if val.ArrayVal[1].IntVal != 50 {
		t.Errorf("expected 50, got %v", val.ArrayVal[1])
	}
	if val.ArrayVal[2].IntVal != 30 {
		t.Errorf("expected 30 for supply collected, got %v", val.ArrayVal[2])
	}
}

func TestRakuSubMainAutoDispatch(t *testing.T) {
	in := NewInterp()
	in.GlobalEnv.Define("@*ARGS", ArrayValue([]*Value{StringValue("World"), StringValue("3")}))

	code := `
sub MAIN($name, $times) {
    return "Greetings " ~ $name ~ " x" ~ $times;
}
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("MAIN dispatch eval failed: %v", err)
	}

	if val.String() != "Greetings World x3" {
		t.Errorf("expected 'Greetings World x3', got %q", val.String())
	}
}
