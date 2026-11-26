//go:build amd64 && !purego

package raptor

func hasAVX2Asm() bool
func avx2SumInt64(ptr *int64, n int) int64
func avx2SumFloat64(ptr *float64, n int) float64
func avx2DotFloat64(a, b *float64, n int) float64
func avx2AddInt64(dst, a, b *int64, n int)

var cpuHasAVX2 = hasAVX2Asm()

// HasAVX2 reports whether hardware 256-bit AVX2 instructions are active on this CPU.
func HasAVX2() bool {
	return cpuHasAVX2
}

// SimdBackendName describes the active SIMD vector backend.
func SimdBackendName() string {
	if cpuHasAVX2 {
		return "x86_64-AVX2-256bit"
	}
	return "x86_64-scalar"
}

// SimdSumInt64 reduces a packed int64 slice using 4x unrolled 256-bit VPADDQ when AVX2 is available.
func SimdSumInt64(xs []int64) int64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	if cpuHasAVX2 && n >= 8 {
		return avx2SumInt64(&xs[0], n)
	}
	var s int64
	for _, v := range xs {
		s += v
	}
	return s
}

// SimdSumFloat64 reduces a packed float64 slice using 4x unrolled 256-bit VADDPD when AVX2 is available.
func SimdSumFloat64(xs []float64) float64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	if cpuHasAVX2 && n >= 8 {
		return avx2SumFloat64(&xs[0], n)
	}
	var s float64
	for _, v := range xs {
		s += v
	}
	return s
}

// SimdDotFloat64 computes the dot product of two float64 slices via AVX2 VMULPD + VADDPD.
func SimdDotFloat64(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	if cpuHasAVX2 && n >= 4 {
		return avx2DotFloat64(&a[0], &b[0], n)
	}
	var s float64
	for i := 0; i < n; i++ {
		s += a[i] * b[i]
	}
	return s
}

// SimdAddInt64 performs element-wise vector addition of two int64 slices via AVX2 VPADDQ.
func SimdAddInt64(a, b []int64) []int64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return nil
	}
	dst := make([]int64, n)
	if cpuHasAVX2 && n >= 4 {
		avx2AddInt64(&dst[0], &a[0], &b[0], n)
		return dst
	}
	for i := 0; i < n; i++ {
		dst[i] = a[i] + b[i]
	}
	return dst
}
