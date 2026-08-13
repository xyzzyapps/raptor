package raptor

import (
	"os/exec"
	"testing"
)

// BenchmarkArithmeticLoop tests raw arithmetic loop speed in Raku5.
func BenchmarkArithmeticLoop(b *testing.B) {
	code := `
my $sum = 0;
for 1..1000 -> $i {
    $sum = $sum + $i;
}
$sum;
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in := NewInterp()
		_, err := in.Eval(code)
		if err != nil {
			b.Fatalf("eval failed: %v", err)
		}
	}
}

// BenchmarkMultipleDispatch tests multiple dispatch resolution throughput.
func BenchmarkMultipleDispatch(b *testing.B) {
	code := `
multi sub calc(Int $a, Int $b) {
    return $a + $b;
}

multi sub calc(Str $a, Str $b) {
    return $a ~ $b;
}

my $sum = 0;
for 1..100 -> $i {
    $sum = calc($sum, $i);
}
$sum;
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in := NewInterp()
		_, err := in.Eval(code)
		if err != nil {
			b.Fatalf("eval failed: %v", err)
		}
	}
}

// BenchmarkFunctionalMapGrep tests functional pipelines with UFCS.
func BenchmarkFunctionalMapGrep(b *testing.B) {
	code := `
sub dbl($x) { return $x * 2; }
sub gt10($x) { return $x > 10; }

my @list = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
my @res = @list.map(&dbl).grep(&gt10);
@res;
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in := NewInterp()
		_, err := in.Eval(code)
		if err != nil {
			b.Fatalf("eval failed: %v", err)
		}
	}
}

// BenchmarkComparisonWithPerl compares Raku5 execution with Perl 5 process execution if perl is on system.
func BenchmarkComparisonWithPerl(b *testing.B) {
	_, err := exec.LookPath("perl")
	if err != nil {
		b.Skip("perl executable not found in PATH")
	}

	b.Run("Perl5_Process", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cmd := exec.Command("perl", "-e", "my $s=0; for my $i (1..1000) { $s += $i; } print $s;")
			out, err := cmd.Output()
			if err != nil {
				b.Fatalf("perl failed: %v, out: %s", err, out)
			}
		}
	})

	b.Run("Raku5_InProcess", func(b *testing.B) {
		code := "my $s=0; for 1..1000 -> $i { $s = $s + $i; } $s;"
		for i := 0; i < b.N; i++ {
			in := NewInterp()
			_, err := in.Eval(code)
			if err != nil {
				b.Fatalf("raku5 failed: %v", err)
			}
		}
	})
}
