# Raptor Runtime Performance & "Faster Than C" Architecture Guide

This document details the comparative benchmark results and the four-pillar optimization architecture (**Memory, CPU, SIMD, and Multi-Core Threading**) inspired by Wouter van Oortmerssen's **Goose** language (`aardappel/goose`) that enables **Raptor** to outpace Perl 5 and Raku by orders of magnitude and match or beat idiomatic **C (`gcc -O3`)**.

---

## 1. Test Environment & System Specifications

| Parameter | Specification |
| :--- | :--- |
| **Operating System** | Windows 11 Pro 64-bit (Build 26100) |
| **Processor** | AMD Ryzen 9 5980HX with Radeon Graphics (8 Cores / 16 Threads, Zen 3, AVX2 256-bit) |
| **Memory** | 32 GB DDR4-3200 |
| **C Compiler** | GCC 14.2.0 (`x86_64-w64-mingw32-gcc -O3`) |
| **Perl 5** | Perl v5.40.0 (MSYS2 UCRT64) |
| **Perl 6 / Raku** | Rakudo v2026.05 on MoarVM |
| **Raptor v1.2** | Release Tag `v1.2` (`898d4ab`) |
| **Raptor `--go`** | Go Superinstruction VM + Unboxed `intRecSpec` + Goose DataStacks + Plan 9 AVX2 |
| **Raptor `--cc` (Native)** | `raptor compile --cc`: Goose-style 16 MB Bump-Arena C11 + AVX2 + Radix Sort |

---

## 2. 6-Way Full Benchmark Comparison Table (Mean of 5 Runs)

All measurements are **end-to-end wall-clock times in milliseconds** (process creation + parse/init + execution + teardown). Lower is better.

| Benchmark Kernel | Workload Description | C (`gcc -O3`) | Raptor `--cc` (Native) | Raptor `--go` | Perl 5 | Perl 6 (Raku) | Raptor v1.2 |
| :--- | :--- | :---: | :---: | :---: | :---: | :---: | :---: |
| **`startup`** | Process boot and `say "Hello, World!"` | 7.8 ms | **6.5 ms** | 36.6 ms | 24.8 ms | 229.8 ms | 29.3 ms |
| **`fib`** | Naive recursive `fib(29)` (514,229 calls) | 8.1 ms | **7.5 ms** | **60.3 ms** | 392.8 ms | 719.3 ms | 855.9 ms |
| **`loopsum`** | 1,000,000 integer loop increments (`$total += $i`) | 9.1 ms | **8.1 ms** | **37.4 ms** | 62.0 ms | 515.6 ms | 171.9 ms |
| **`streq`** | 1,000,000 string `eq` and `lt` comparisons | **6.4 ms** | 7.5 ms | **41.1 ms** | 116.2 ms | 427.7 ms | 396.6 ms |
| **`hash`** | 100,000 hash bucket lookups and increments | **6.6 ms** | 7.6 ms | 47.9 ms | 31.7 ms | 323.5 ms | 113.6 ms |
| **`sortnums`** | Sort 50,000 pseudo-random 64-bit integers | 11.4 ms | **9.6 ms** | 39.1 ms | 35.9 ms | 383.6 ms | 65.5 ms |
| **`strcat`** | 50,000 amortized string concatenations | 7.5 ms | **7.2 ms** | 38.9 ms | 24.1 ms | 258.2 ms | 40.0 ms |
| **`arrayops`** | Filter (`%% 3`), map (`+ 1`), sum over 200,000 ints | **7.0 ms** | **7.0 ms** | **36.1 ms** | 36.7 ms | 474.8 ms | 73.3 ms |

> [!NOTE]
> Subtracting Go's fixed binary startup (`36.6 ms`), **`Raptor --go` executes `loopsum` in `0.8 ms`, `arrayops` in `< 0.5 ms`, `strcat` in `2.3 ms`, `sortnums` in `2.5 ms`, `streq` in `4.5 ms`, `hash` in `11.3 ms`, and `fib(29)` in `23.7 ms`**—all without requiring an external C compiler!

---

## 3. Scaled "Faster Than C" Challenge Results

When N is scaled 20x–50x so compute time dwarfs OS `CreateProcess` overhead (~7 ms), Raptor's semantic & data-layout advantages allow it to beat idiomatic `gcc -O3` C code directly:

| Dimension | Benchmark | Idiomatic C (`gcc -O3`) | Raptor | Multiplier |
| :--- | :--- | :---: | :---: | :---: |
| **Algorithmic + Memory Locality** | **`sort_1m`** (Sort 1,000,000 `int64`) | 122.2 ms (`qsort`) | **23.3 ms** (`--cc`)<br>**103.4 ms** (`--go`) | **5.24x faster** (`--cc`)<br>**1.18x faster** (`--go`) |
| **Length-Prefixed Slices vs NUL** | **`streq_20m`** (20M 64-byte string `eq`/`lt`) | 8.5 ms (`strcmp`) | **7.6 ms** (`--cc`) | **1.12x faster** |
| **AVX2 + Multi-Core Threads** | **100M `int64` L3 Reduction** (`2M × 50`) | ~18.5 ms (1-core AVX2) | **18.09 ms** (`simd_sum`)<br>**9.65 ms** (`psum` 16T) | **1.92x faster** (`psum`) |

---

## 4. Architectural Pillars: Goose Insights & How Raptor Beats Idiomatic C

### 1. Memory Performance — Goose-Style Bump Arenas & Segmented DataStacks
Inspired by Wouter van Oortmerssen's **Goose** (`aardappel/goose`):
- **Native `--cc` Bump Arena ([`runtime/cc_backend.go`](runtime/cc_backend.go))**: Whole-program compilation allocates a single 16 MB contiguous bump-pointer arena (`rp_arena_alloc` / `rp_arena_reset(mark)`). Scope exit unwinds allocations in **1 CPU instruction** (`sp = mark`) with zero `malloc`/`realloc`/`free` lock contention or heap fragmentation.
- **Go `--go` DataStacks ([`runtime/datastack.go`](runtime/datastack.go))**: Function locals reside in non-moving `frameSlabs`, ephemeral temporaries bump-allocate in `tempSlabs`, and `Value` is packed to 88 bytes with pointer fields grouped in the first 4 words to minimize GC scan work.
- **Length-Prefixed `RpStr` & `StrTape` vs. NUL-Terminated `char*`**: C's `strlen` and `strcmp` must scan byte-by-byte looking for `\0`. Raptor's `RpStr { const char *data; size_t len; }` rejects unequal-length strings in `O(1)` and compares equal-length slices with wide vector loads.

### 2. CPU Performance — Superinstructions, Unboxed Specialization & Radix Lanes
- **Unboxed Recursive Specialization ([`runtime/supervm.go`](runtime/supervm.go))**: Pure-integer recursive functions like `fib($n)` compile ahead of time into `intRecSpec`, executing entirely in unboxed `int64` machine registers on the Go call stack (`855.9 ms` → `208.2 ms` → **`60.3 ms`** in `--go`, and **`7.5 ms`** in `--cc`).
- **Register-Promoted Fused Loops ([`runtime/supervm.go`](runtime/supervm.go))**: Hot `loop (...)` constructs compile into `fusedLoopPlan` with register-promoted accumulators (`acc += i`), invariant string comparators, and open-addressing multiplicative-hash `flatIntHash` tables, eliminating AST dispatch and boxing inside tight loops.
- **Columnar `O(n)` Counting / LSD Radix Sort ([`runtime/parallel.go`](runtime/parallel.go), [`runtime/cc_backend.go`](runtime/cc_backend.go))**: Because Raptor knows an array's homogeneous `int64` lane (`IntArrayValue` / `RpIntVec`), `.sort()` dispatches to `SortInt64Fast` ($O(n)$ counting sort for compact key spans, 4-pass 16-bit LSD radix sort for general 64-bit integers)—beating C's `qsort` (which must invoke an indirect `int (*cmp)(const void*, const void*)` pointer $O(n \log n)$ times) by **5.24x** at 1M elements.

### 3. SIMD Performance — Native 256-Bit x86-64 AVX2 Kernels
- **Handwritten Plan 9 AVX2 Assembly ([`runtime/internal/simd/simd_amd64.s`](runtime/internal/simd/simd_amd64.s))**: Runtime `CPUID` + `XGETBV` detection activates 4x-unrolled 256-bit `YMM` kernels (`VPADDQ` for 4×`int64`/cycle, `VADDPD` & `VMULPD` for 4×`float64`/cycle), exposed via `simd_sum`, `simd_dot` / `dot`, `simd_add`, `simd_scale`, and `simd_info` (with portable pure-Go fallbacks in [`runtime/internal/simd/simd_generic.go`](runtime/internal/simd/simd_generic.go) for ARM64 and `js/wasm`).

### 4. Thread Performance — Lock-Free Multi-Core Data Parallelism
- **Work-Partitioned Multi-Core Primitives ([`runtime/parallel.go`](runtime/parallel.go))**:
  - `psum(@arr)`: Splits packed `int64`/`float64` lanes across `runtime.NumCPU()` goroutines, each running AVX2 `VPADDQ`/`VADDPD` over its cache-aligned slice (**82.9 GB/s** aggregate L3 throughput on 16 threads).
  - `pmap(&fn, @arr)`: Executes closures in parallel using isolated per-worker `forkWorker()` Goose DataStacks (`datastack.go`) with zero lock contention.
  - `psort(@arr)`: Parallel chunk sort + merge / radix pipeline for large datasets.

---

## 5. How to Reproduce

```powershell
cd raptor

# 1. Build Raptor CLI
go build -mod=mod -o bin/raptor.exe ./cmd/raptor

# 2. Compile C (-O3) and Raptor Native (--cc) binaries
gcc -O3 -o fib_c.exe examples/bench/fib.c
./bin/raptor.exe --cc -o fib_rpcc.exe compile examples/bench/fib.rp

# 3. Compare execution times
Measure-Command { ./fib_c.exe }
Measure-Command { ./fib_rpcc.exe }
Measure-Command { ./bin/raptor.exe --go examples/bench/fib.rp }
Measure-Command { perl examples/bench/fib.pl }
Measure-Command { raku examples/bench/fib.raku }
```
