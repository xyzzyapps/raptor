package raptor

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAVX2SIMDAndRadixSort(t *testing.T) {
	// Test odd/tail lengths (0, 1, 3, 4, 7, 8, 15, 16, 17, 63, 10003)
	for _, n := range []int{0, 1, 3, 4, 7, 8, 15, 16, 17, 63, 10003} {
		ints := make([]int64, n)
		floatsA := make([]float64, n)
		floatsB := make([]float64, n)
		var wantI int64
		var wantF, wantDot float64
		for i := 0; i < n; i++ {
			iv := int64((i*37 - 500) * (i%3 + 1))
			fvA := float64(i%19) * 0.5
			fvB := float64((i+3)%11) * 0.25
			ints[i] = iv
			floatsA[i] = fvA
			floatsB[i] = fvB
			wantI += iv
			wantF += fvA
			wantDot += fvA * fvB
		}
		if gotI := SimdSumInt64(ints); gotI != wantI {
			t.Fatalf("SimdSumInt64(len=%d) = %d, want %d (avx2=%v)", n, gotI, wantI, HasAVX2())
		}
		if gotF := SimdSumFloat64(floatsA); math.Abs(gotF-wantF) > 1e-9 {
			t.Fatalf("SimdSumFloat64(len=%d) = %v, want %v", n, gotF, wantF)
		}
		if gotDot := SimdDotFloat64(floatsA, floatsB); math.Abs(gotDot-wantDot) > 1e-9 {
			t.Fatalf("SimdDotFloat64(len=%d) = %v, want %v", n, gotDot, wantDot)
		}
		added := SimdAddInt64(ints, ints)
		for i := 0; i < n; i++ {
			if added[i] != ints[i]*2 {
				t.Fatalf("SimdAddInt64 mismatch at %d: got %d, want %d", i, added[i], ints[i]*2)
			}
		}
	}

	// Test Counting Sort path (dense bounded range) & 4-pass 64-bit LSD Radix Sort (wide signed range)
	dense := make([]int64, 5000)
	wide := make([]int64, 5000)
	for i := range dense {
		dense[i] = int64((i*2654435761)%50000) - 25000
		wide[i] = int64(uint64(i)*6364136223846793005 + 1442695040888963407)
	}
	denseWant := slices.Clone(dense)
	wideWant := slices.Clone(wide)
	slices.Sort(denseWant)
	slices.Sort(wideWant)

	SortInt64Fast(dense)
	SortInt64Fast(wide)
	if !slices.Equal(dense, denseWant) {
		t.Fatalf("SortInt64Fast counting sort mismatch")
	}
	if !slices.Equal(wide, wideWant) {
		t.Fatalf("SortInt64Fast 64-bit signed radix sort mismatch")
	}
}

func TestParallelSIMDAndCCBackendParity(t *testing.T) {
	out := runRaptorSnippet(t, `
my @xs = 1 .. 20000;
say simd_sum(@xs);
say psum(@xs);

my @sq = pmap(sub ($x) { return $x * 2; }, [1, 2, 3, 4, 5]);
say join(",", @sq);

my $d = simd_dot([1.5, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0], [2.0, 0.5, 1.0, 3.0, 2.0, 1.0, 0.5, 0.25]);
say $d;
`)
	want := "200010000\n200010000\n2,4,6,8,10\n40.5"
	if out != want {
		t.Fatalf("parallel/simd builtins mismatch:\ngot:\n%s\nwant:\n%s", out, want)
	}

	if _, err := FindNativeCCompiler(); err != nil {
		t.Skipf("skipping C11 backend test: %v", err)
	}

	kernels := []string{"startup.rp", "fib.rp", "loopsum.rp", "streq.rp", "hash.rp", "sortnums.rp", "strcat.rp", "arrayops.rp"}
	for _, name := range kernels {
		p := filepath.Join("..", "examples", "bench", name)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		src := string(b)

		inGo := NewInterp()
		var goBuf bytes.Buffer
		inGo.Stdout = &goBuf
		if _, err := inGo.EvalOnBackend(src, BackendGo); err != nil {
			t.Fatalf("BackendGo failed on %s: %v", name, err)
		}

		inCC := NewInterp()
		var ccBuf bytes.Buffer
		inCC.Stdout = &ccBuf
		if _, err := inCC.EvalOnBackend(src, BackendCC); err != nil {
			t.Fatalf("BackendCC failed on %s: %v", name, err)
		}

		gotGo := strings.TrimSpace(goBuf.String())
		gotCC := strings.TrimSpace(ccBuf.String())
		if gotGo != gotCC {
			t.Fatalf("kernel %s parity mismatch: go=%q vs cc=%q", name, gotGo, gotCC)
		}
	}
}
