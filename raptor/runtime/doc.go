// Package raptor implements the Raptor programming language runtime, parser,
// multi-backend compilers, standard library builtins, and PodLit literate engine.
//
// Execution Backends:
//   - BackendGo ("go"): Goose-inspired pointer-free DataStack + fused superinstruction VM
//     with Plan 9 x86-64 AVX2 vector reductions and unboxed int64 fast recursion.
//   - BackendCC ("cc"): Ahead-of-time / cached native C11 + AVX2 + 8-bit LSD Radix Sort
//     transpiler targeting GCC, Clang, or `zig cc` with a 16 MB pointerless bump arena.
//   - BackendMoar ("moar"): Direct MoarVM CompUnit v7 bytecode compiler (`moarvm-go/engine`).
//   - BackendWASM ("wasm"): Client-side WebAssembly runtime bridging HTML5 Canvas 2D,
//     WebGL 3D, WebAudio, WebGPU, and DOM subsystems.
//
// Memory & Value Architecture:
//   - Value is a compact 64-byte tagged struct with rare variants multiplexed onto Value.ext.
//   - Homogeneous arrays store packed unboxed []int64, []float64, or Arrow-style pointerless
//     StrTape buffers inside ArrayLane so Go's garbage collector skips scanning bulk data.
//   - Lexical variables resolve statically to indexed stackFrame / globalSlab slots (`resolve.go`)
//     backed by contiguous DataStack slabs (`datastack.go`) with escape promotion on heap capture.
package raptor
