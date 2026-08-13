package raptor

import (
	"testing"
)

func TestMultimethodBeforeAfterAdvice(t *testing.T) {
	in := NewInterp()
	code := `
my @trace;

multi sub process(Int $x) {
    @trace.push("int:" ~ $x);
    return $x * 10;
}

multi sub process(Str $s) {
    @trace.push("str:" ~ $s);
    return "processed:" ~ $s;
}

before process(Int $x) {
    @trace.push("before_int:" ~ $x);
}

after process(Int $x) {
    @trace.push("after_int:" ~ $x);
}

before process(Str $s) {
    @trace.push("before_str:" ~ $s);
}

my $r1 = process(5);
my $r2 = process("raku");

[$r1, $r2, @trace];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected array of 3, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 50 {
		t.Errorf("expected 50, got %d", val.ArrayVal[0].IntVal)
	}
	if val.ArrayVal[1].StrVal != "processed:raku" {
		t.Errorf("expected processed:raku, got %s", val.ArrayVal[1].StrVal)
	}
	trace := val.ArrayVal[2]
	expectedTrace := []string{"before_int:5", "int:5", "after_int:5", "before_str:raku", "str:raku"}
	if len(trace.ArrayVal) != len(expectedTrace) {
		t.Fatalf("trace length mismatch: expected %d, got %d (%v)", len(expectedTrace), len(trace.ArrayVal), trace)
	}
	for i, exp := range expectedTrace {
		if trace.ArrayVal[i].StrVal != exp {
			t.Errorf("trace[%d] mismatch: expected %q, got %q", i, exp, trace.ArrayVal[i].StrVal)
		}
	}
}

func TestMultimethodAroundAdvice(t *testing.T) {
	in := NewInterp()
	code := `
multi sub calc(Int $a, Int $b) {
    return $a + $b;
}

multi sub calc(Str $a, Str $b) {
    return $a ~ " & " ~ $b;
}

around calc($orig, Int $a, Int $b) {
    my $res = $orig($a * 2, $b * 2);
    return $res + 1;
}

around calc($orig, Str $a, Str $b) {
    my $res = $orig($a.uc(), $b.uc());
    return "[" ~ $res ~ "]";
}

my $v1 = calc(3, 4);        # (3*2 + 4*2) + 1 = 15
my $v2 = calc("foo", "bar"); # [FOO & BAR]

[$v1, $v2];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected array of 2, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 15 {
		t.Errorf("expected 15, got %d", val.ArrayVal[0].IntVal)
	}
	if val.ArrayVal[1].StrVal != "[FOO & BAR]" {
		t.Errorf("expected [FOO & BAR], got %s", val.ArrayVal[1].StrVal)
	}
}
