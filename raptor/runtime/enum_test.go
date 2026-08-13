package raptor

import (
	"testing"
)

func TestEnumAngleBrackets(t *testing.T) {
	in := NewInterp()
	code := `
enum Color <Red Green Blue>;
my $r = Red;
my $g = Green;
my $b = Blue;
[$r, $g, $b];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected array of 3, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 0 || val.ArrayVal[1].IntVal != 1 || val.ArrayVal[2].IntVal != 2 {
		t.Fatalf("expected [0, 1, 2], got [%d, %d, %d]",
			val.ArrayVal[0].IntVal, val.ArrayVal[1].IntVal, val.ArrayVal[2].IntVal)
	}
}

func TestEnumCustomValues(t *testing.T) {
	in := NewInterp()
	code := `
enum Direction (North => 10, South => 20, East => 30, West => 40);
my $n = North;
my $s = South;
$n + $s;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.IntVal != 30 {
		t.Fatalf("expected 30, got %d", val.IntVal)
	}
}
