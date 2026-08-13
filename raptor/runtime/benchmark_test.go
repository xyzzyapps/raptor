package raptor

import (
	"testing"
)

func BenchmarkIntegerLoop(b *testing.B) {
	code := `
		my $sum = 0;
		for 1..100000 -> $i {
			$sum = $sum + $i;
		}
	`
	for n := 0; n < b.N; n++ {
		in := NewInterp()
		_, err := in.Eval(code)
		if err != nil {
			b.Fatalf("Eval error: %v", err)
		}
	}
}

func BenchmarkStructFieldAccess(b *testing.B) {
	code := `
		struct Point {
			int64 $x;
			int64 $y;
		}
		my $p = Point.new();
		for 1..50000 -> $i {
			$p.x = $i;
			$p.y = $p.x * 2;
		}
	`
	for n := 0; n < b.N; n++ {
		in := NewInterp()
		_, err := in.Eval(code)
		if err != nil {
			b.Fatalf("Eval error: %v", err)
		}
	}
}

func BenchmarkRecursiveFunctionCalls(b *testing.B) {
	code := `
		sub fib($n) {
			if $n <= 1 { return $n; }
			return fib($n - 1) + fib($n - 2);
		}
		my $res = fib(20);
	`
	for n := 0; n < b.N; n++ {
		in := NewInterp()
		_, err := in.Eval(code)
		if err != nil {
			b.Fatalf("Eval error: %v", err)
		}
	}
}

func BenchmarkCustomOperatorOverload(b *testing.B) {
	code := `
		struct Vec2 {
			int64 $x;
			int64 $y;
		}
		multi sub infix:<+>(Vec2 $a, Vec2 $b) {
			my $r = Vec2.new();
			$r.x = $a.x + $b.x;
			$r.y = $a.y + $b.y;
			return $r;
		}
		my $v1 = Vec2.new(); $v1.x = 10; $v1.y = 20;
		my $v2 = Vec2.new(); $v2.x = 5;  $v2.y = 15;
		for 1..20000 -> $i {
			my $v3 = $v1 + $v2;
		}
	`
	for n := 0; n < b.N; n++ {
		in := NewInterp()
		_, err := in.Eval(code)
		if err != nil {
			b.Fatalf("Eval error: %v", err)
		}
	}
}
