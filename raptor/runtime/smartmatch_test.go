package raptor

import (
	"testing"
)

func TestSmartMatchPrimitives(t *testing.T) {
	in := NewInterp()
	code := `
my $a = (42 ~~ 42);
my $b = (42 ~~ "Int");
my $c = ("hello" ~~ "Str");
my $d = (42 ~~ "Str");
my $e = ([1, 2, 3] ~~ "Array");
my $f = (2 ~~ [1, 2, 3]);
my $g = (5 ~~ [1, 2, 3]);
[$a, $b, $c, $d, $e, $f, $g];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 7 {
		t.Fatalf("expected array of 7 booleans, got %v", val)
	}

	expected := []bool{true, true, true, false, true, true, false}
	for i, exp := range expected {
		if val.ArrayVal[i].BoolVal != exp {
			t.Errorf("test %d: expected %v, got %v", i, exp, val.ArrayVal[i].BoolVal)
		}
	}
}

func TestGivenWhenDefault(t *testing.T) {
	in := NewInterp()
	code := `
my $score = 85;
my $grade = "";

given $score {
    when 100 {
        $grade = "A+";
    }
    when 85 {
        $grade = "B+";
    }
    default {
        $grade = "F";
    }
}
$grade;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.StrVal != "B+" {
		t.Fatalf("expected 'B+', got %q", val.StrVal)
	}
}

func TestGivenWhenWithArrayMatching(t *testing.T) {
	in := NewInterp()
	code := `
my $day = "Wed";
my $type = "";

given $day {
    when ["Sat", "Sun"] {
        $type = "Weekend";
    }
    when ["Mon", "Tue", "Wed", "Thu", "Fri"] {
        $type = "Weekday";
    }
    default {
        $type = "Unknown";
    }
}
$type;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.StrVal != "Weekday" {
		t.Fatalf("expected 'Weekday', got %q", val.StrVal)
	}
}

func TestChainedComparisons(t *testing.T) {
	in := NewInterp()
	code := `
my $x = 15;
my $a = (10 < $x < 20);
my $b = (10 < $x < 12);
my $c = (5 <= 5 <= 5);
my $d = (1 < 2 < 3 < 4 < 5);
my $e = (1 < 2 < 2 < 4);
[$a, $b, $c, $d, $e];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	expected := []bool{true, false, true, true, false}
	for i, exp := range expected {
		if val.ArrayVal[i].BoolVal != exp {
			t.Errorf("chained test %d: expected %v, got %v", i, exp, val.ArrayVal[i].BoolVal)
		}
	}
}
