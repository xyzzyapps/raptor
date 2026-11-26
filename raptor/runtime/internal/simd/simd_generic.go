//go:build !amd64 || purego

package simd

// HasAVX2 reports false on non-amd64 / WASM targets.
func HasAVX2() bool {
	return false
}

// BackendName describes the active SIMD vector backend.
func BackendName() string {
	return "generic-unrolled"
}

// SumInt64 reduces a packed int64 slice with 4-way scalar unrolling.
func SumInt64(xs []int64) int64 {
	var s0, s1, s2, s3 int64
	i := 0
	n := len(xs)
	for ; i+3 < n; i += 4 {
		s0 += xs[i]
		s1 += xs[i+1]
		s2 += xs[i+2]
		s3 += xs[i+3]
	}
	sum := (s0 + s1) + (s2 + s3)
	for ; i < n; i++ {
		sum += xs[i]
	}
	return sum
}

// SumFloat64 reduces a packed float64 slice with 4-way scalar unrolling.
func SumFloat64(xs []float64) float64 {
	var s0, s1, s2, s3 float64
	i := 0
	n := len(xs)
	for ; i+3 < n; i += 4 {
		s0 += xs[i]
		s1 += xs[i+1]
		s2 += xs[i+2]
		s3 += xs[i+3]
	}
	sum := (s0 + s1) + (s2 + s3)
	for ; i < n; i++ {
		sum += xs[i]
	}
	return sum
}

// DotFloat64 computes dot product on generic targets.
func DotFloat64(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var s0, s1 float64
	i := 0
	for ; i+1 < n; i += 2 {
		s0 += a[i] * b[i]
		s1 += a[i+1] * b[i+1]
	}
	sum := s0 + s1
	if i < n {
		sum += a[i] * b[i]
	}
	return sum
}

// AddInt64 computes element-wise int64 slice sum on generic targets.
func AddInt64(a, b []int64) []int64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return nil
	}
	dst := make([]int64, n)
	for i := 0; i < n; i++ {
		dst[i] = a[i] + b[i]
	}
	return dst
}
