package raptor

import (
	"testing"
)

func TestMapGrepUFCS(t *testing.T) {
	in := NewInterp()
	code := `
sub square($x) {
    return $x * $x;
}

sub is_even($x) {
    return ($x % 2) == 0;
}

my @nums = [1, 2, 3, 4, 5, 6];
my @evens = @nums.grep(&is_even);
my @squares = @evens.map(&square);
@squares;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected array of 3 squares, got %v", val)
	}
	// evens: 2, 4, 6 -> squares: 4, 16, 36
	expected := []int64{4, 16, 36}
	for i, exp := range expected {
		if val.ArrayVal[i].IntVal != exp {
			t.Errorf("elem %d: expected %d, got %d", i, exp, val.ArrayVal[i].IntVal)
		}
	}
}

func TestSortAndReverse(t *testing.T) {
	in := NewInterp()
	code := `
my @data = [50, 10, 40, 20, 30];
my @sorted = @data.sort();
my @rev = @sorted.reverse();
@rev;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 5 {
		t.Fatalf("expected array of 5, got %v", val)
	}
	expected := []int64{50, 40, 30, 20, 10}
	for i, exp := range expected {
		if val.ArrayVal[i].IntVal != exp {
			t.Errorf("elem %d: expected %d, got %d", i, exp, val.ArrayVal[i].IntVal)
		}
	}
}

func TestReduce(t *testing.T) {
	in := NewInterp()
	code := `
sub add($a, $b) {
    return $a + $b;
}

sub multiply($a, $b) {
    return $a * $b;
}

my @items = [1, 2, 3, 4, 5];
my $sum = @items.reduce(&add);
my $prod = @items.reduce(&multiply);
[$sum, $prod];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected array of 2, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 15 {
		t.Errorf("sum: expected 15, got %d", val.ArrayVal[0].IntVal)
	}
	if val.ArrayVal[1].IntVal != 120 {
		t.Errorf("prod: expected 120, got %d", val.ArrayVal[1].IntVal)
	}
}
