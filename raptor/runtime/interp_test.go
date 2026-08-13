package raptor

import (
	"testing"
)

func TestStringInterpolationVars(t *testing.T) {
	in := NewInterp()
	code := `
my $name = "Alice";
my $age = 30;
my $msg = "User $name is $age years old.";
$msg;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	expected := "User Alice is 30 years old."
	if val.StrVal != expected {
		t.Fatalf("expected %q, got %q", expected, val.StrVal)
	}
}

func TestStringInterpolationExpressions(t *testing.T) {
	in := NewInterp()
	code := `
my $x = 10;
my $y = 20;
my $msg = "Result: {$x + $y} and double is {$x * 2}.";
$msg;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	expected := "Result: 30 and double is 20."
	if val.StrVal != expected {
		t.Fatalf("expected %q, got %q", expected, val.StrVal)
	}
}

func TestSingleQuoteLiteral(t *testing.T) {
	in := NewInterp()
	code := `
my $name = "Alice";
my $msg = 'User $name is not interpolated.';
$msg;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	expected := "User $name is not interpolated."
	if val.StrVal != expected {
		t.Fatalf("expected %q, got %q", expected, val.StrVal)
	}
}
