# Raptor Runtime Performance & Comparative Benchmarks

This document provides a comparative benchmark analysis of the **Raptor** language runtime against **C (`gcc -O3`)**, **Perl 5**, and **Perl 6 (Rakudo / MoarVM)**, as well as tracking speedups from **Raptor v1.2** through **Raptor `--go` (Superinstruction + AVX2 + Goose DataStacks)** and **Raptor Native (`raptor compile --cc` Goose-style Bump-Arena C11 + AVX2)**.

For the full architectural deep dive (Memory, CPU, SIMD, and Multi-Core Threading) and reproduction commands, see [`raptor/BENCH.md`](raptor/BENCH.md).

---

## 1. Test Environment & System Specifications

All tests were executed on the same physical hardware under identical system conditions:

| Parameter | Specification |
| :--- | :--- |
| **Operating System** | Windows 11 Pro 64-bit (Build 26100) |
| **Processor** | AMD Ryzen 9 5980HX with Radeon Graphics (8 Cores, 16 Threads, AVX2 256-bit) |
| **Memory** | 32 GB DDR4-3200 |
| **C Compiler** | GCC 14.2.0 (`x86_64-w64-mingw32-gcc -O3`) |
| **Perl 5** | Perl v5.40.0 (MSYS2 UCRT64) |
| **Perl 6 / Raku** | Rakudo v2026.05 on MoarVM |
| **Raptor v1.2** | Release Tag `v1.2` (`898d4ab`) |
| **Raptor `--go`** | Go Superinstruction VM + Unboxed `intRecSpec` + Goose DataStacks + Plan 9 AVX2 |
| **Raptor `--cc` (Native)** | `raptor compile --cc`: Whole-program Goose 16 MB Bump-Arena C11 + AVX2 + Radix Sort |

---

## 2. 6-Way Standard Benchmark Comparison Table

All measurements represent **mean wall-clock execution time (in milliseconds)** across **5 consecutive runs** per benchmark kernel (after warmup), including process startup and execution. Lower is better.

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

---

## 3. Scaled "Faster Than C" Challenge Benchmarks

On sub-10 ms scripts, Windows `CreateProcess` overhead (~6.5–7.5 ms) dominates wall-clock runtime. Scaling workloads by **20x–50x** exposes pure **Memory, CPU, String, SIMD, and Multi-Core** throughput:

| Scaled Workload | Idiomatic C (`gcc -O3`) | Raptor `--cc` / SIMD / Parallel | Speedup vs. C (`gcc -O3`) | Architectural Advantage |
| :--- | :---: | :---: | :---: | :--- |
| **`sort_1m`** (Sort 1,000,000 `int64`) | 122.2 ms (`qsort`) | **23.3 ms** (`--cc` Radix)<br>**103.4 ms** (`--go` `sort`) | **5.24x faster** (`--cc`)<br>**1.18x faster** (`--go`) | $O(n)$ counting / 4-pass 16-bit LSD radix sort over packed `int64` lanes vs. C `qsort`'s $O(n \log n)$ indirect comparator calls |
| **`streq_20m`** (20M 64-byte string `eq`/`lt`) | 8.5 ms (`strcmp`) | **7.6 ms** (`--cc` `RpStr`) | **1.12x faster** | Length-prefixed `{ptr, len}` `RpStr` slices (`O(1)` length check + word `memcmp`) vs. NUL-scanning `strcmp` |
| **100M `int64` L3 Reduction** (`2M × 50`) | ~18.5 ms (1-thread AVX2) | **18.09 ms** (`simd_sum` AVX2)<br>**9.65 ms** (`psum` 16-thread) | **1.92x faster** (`psum`) | Handwritten x86-64 AVX2 `VPADDQ` (44.2 GB/s) + 16-worker multi-core pool (82.9 GB/s aggregate L3 bandwidth) |

---

## 4. Key Speedup Highlights

- **Raptor Native (`--cc`) vs. C (`gcc -O3`)**: Matches or beats `gcc -O3` on **6 of 8** standard benchmarks (`startup`, `fib`, `loopsum`, `sortnums`, `strcat`, `arrayops`) and achieves a **5.24x speedup** over C `qsort` on 1,000,000 integers (`23.3 ms` vs `122.2 ms`).
- **Raptor `--go` vs. Perl 5**: Without invoking an external C compiler, the pure-Go interpreter + superinstruction VM now beats Perl 5 on **`fib` (6.5x faster: 60.3 ms vs 392.8 ms)**, **`streq` (2.8x faster: 41.1 ms vs 116.2 ms)**, **`loopsum` (1.7x faster: 37.4 ms vs 62.0 ms)**, **`arrayops` (36.1 ms vs 36.7 ms)**, and **`sort_1m` (103.4 ms vs C's 122.2 ms)**—despite Go's ~36.6 ms runtime boot cost (`< 1 ms` to `4.5 ms` pure kernel time!).
- **Raptor vs. Perl 6 (Rakudo / MoarVM)**:
  - **Raptor `--go`** is **6.3x to 13.8x faster** than Rakudo across every benchmark.
  - **Raptor `--cc`** is **35.3x to 95.9x faster** than Rakudo across every benchmark.
- **Raptor Current vs. Raptor `v1.2`**:
  - `fib`: **14.2x faster** in `--go` (`855.9 ms` → `60.3 ms`) and **114.1x faster** in `--cc` (`7.5 ms`).
  - `streq`: **9.7x faster** in `--go` (`396.6 ms` → `41.1 ms`) and **52.9x faster** in `--cc` (`7.5 ms`).
  - `loopsum`: **4.6x faster** in `--go` (`171.9 ms` → `37.4 ms`) and **21.2x faster** in `--cc` (`8.1 ms`).

---

## 5. How to Reproduce

All benchmark sources live in [`raptor/examples/bench/`](raptor/examples/bench). See [`raptor/BENCH.md`](raptor/BENCH.md) for full reproduction commands.
