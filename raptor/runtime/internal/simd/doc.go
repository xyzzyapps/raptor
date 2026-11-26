// Package simd provides hardware-accelerated 256-bit AVX2 vector kernels on amd64
// (with 4-way unrolled scalar fallbacks on non-amd64 and WebAssembly targets).
//
// Keeping the Plan 9 assembly routines in this dedicated internal package (free of
// `import "C"`) allows `raptor/runtime` to combine CGO dynamic library loading (`dlopen`)
// and native AVX2 assembly when cross-compiling with `zig cc`.
package simd
