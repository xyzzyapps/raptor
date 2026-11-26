package raptor

import "raptor/runtime/internal/simd"

// HasAVX2 reports whether hardware 256-bit AVX2 instructions are active on this CPU.
func HasAVX2() bool {
	return simd.HasAVX2()
}

// SimdBackendName describes the active SIMD vector backend.
func SimdBackendName() string {
	return simd.BackendName()
}

// SimdSumInt64 reduces a packed int64 slice using AVX2 VPADDQ when available.
func SimdSumInt64(xs []int64) int64 {
	return simd.SumInt64(xs)
}

// SimdSumFloat64 reduces a packed float64 slice using AVX2 VADDPD when available.
func SimdSumFloat64(xs []float64) float64 {
	return simd.SumFloat64(xs)
}

// SimdDotFloat64 computes the dot product of two float64 slices via AVX2 VMULPD + VADDPD.
func SimdDotFloat64(a, b []float64) float64 {
	return simd.DotFloat64(a, b)
}

// SimdAddInt64 performs element-wise vector addition of two int64 slices via AVX2 VPADDQ.
func SimdAddInt64(a, b []int64) []int64 {
	return simd.AddInt64(a, b)
}
