package raptor

import (
	"testing"
)

func TestUFCSBuiltins(t *testing.T) {
	in := NewInterp()
	code := `
my Str $str = "hello";
my $upper = $str.uc();
my $elemCount = [10, 20, 30, 40].elems();
$upper ~ " count=" ~ $elemCount;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.StrVal != "HELLO count=4" {
		t.Fatalf("expected 'HELLO count=4', got %q", val.StrVal)
	}
}

func TestUFCSMultipleDispatch(t *testing.T) {
	in := NewInterp()
	code := `
multi sub double(Int $x) {
    return $x * 2;
}

multi sub double(Str $s) {
    return $s ~ $s;
}

my $d1 = 21.double();
my $d2 = "abc".double();
$d1 ~ " " ~ $d2;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.StrVal != "42 abcabc" {
		t.Fatalf("expected '42 abcabc', got %q", val.StrVal)
	}
}

func TestUFCSChaining(t *testing.T) {
	in := NewInterp()
	code := `
multi sub addSuffix(Str $s, Str $suf) {
    return $s ~ $suf;
}

my $res = "raku".uc().addSuffix("5");
$res;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.StrVal != "RAKU5" {
		t.Fatalf("expected 'RAKU5', got %q", val.StrVal)
	}
}
