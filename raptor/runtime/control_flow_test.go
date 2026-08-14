package raptor

import (
	"bytes"
	"strings"
	"testing"
)

func TestComprehensiveControlFlow(t *testing.T) {
	code := `
		# 1. Unless statement
		my $flag = False;
		my $unless_ran = False;
		unless $flag {
			$unless_ran = True;
		}

		# 2. Until loop
		my $count = 0;
		until $count >= 4 {
			$count += 1;
		}

		# 3. For loop with next and last
		my @collected = [];
		for 1..10 -> $x {
			if $x % 2 == 0 { next; } # Skip evens
			if $x > 7      { last; } # Stop after 7
			@collected = [@collected, $x];
		}

		# 4. Given / When / Default
		my $category = "unknown";
		my $score = 95;
		given $score {
			when 90..100 { $category = "A"; }
			when 80..89  { $category = "B"; }
			default      { $category = "C"; }
		}

		# 5. Raku Ternary (?? !!) & Defined-or (//)
		my $empty = Nil;
		my $resolved = $empty // "fallback";
		my $ternary_res = ($score > 50) ?? "pass" !! "fail";

		say "UNLESS=" ~ $unless_ran;
		say "UNTIL_COUNT=" ~ $count;
		say "COLLECTED=" ~ @collected;
		say "CATEGORY=" ~ $category;
		say "RESOLVED=" ~ $resolved;
		say "TERNARY=" ~ $ternary_res;
	`
	in := NewInterp()
	var outBuf bytes.Buffer
	in.SetStdout(&outBuf)

	_, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Control flow eval failed: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "UNLESS=True") {
		t.Errorf("Expected UNLESS=True, got: %s", out)
	}
	if !strings.Contains(out, "UNTIL_COUNT=4") {
		t.Errorf("Expected UNTIL_COUNT=4, got: %s", out)
	}
	if !strings.Contains(out, "CATEGORY=A") {
		t.Errorf("Expected CATEGORY=A, got: %s", out)
	}
	if !strings.Contains(out, "RESOLVED=fallback") {
		t.Errorf("Expected RESOLVED=fallback, got: %s", out)
	}
	if !strings.Contains(out, "TERNARY=pass") {
		t.Errorf("Expected TERNARY=pass, got: %s", out)
	}
}

func TestChainedComparisonsAndCStyleLoop(t *testing.T) {
	code := `
		my $temp = 25;
		my $in_range = False;
		if 10 <= $temp <= 30 {
			$in_range = True;
		}

		my $loop_sum = 0;
		loop (my $i = 1; $i <= 5; $i += 1) {
			$loop_sum += $i;
		}

		say "RANGE=" ~ $in_range ~ " SUM=" ~ $loop_sum;
	`
	in := NewInterp()
	var outBuf bytes.Buffer
	in.SetStdout(&outBuf)

	_, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Loop/Chaining eval failed: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "RANGE=True SUM=15") {
		t.Errorf("Unexpected output: %s", out)
	}
}
