<p align="center">
  <img src="assets/logo.png" alt="Raptor Language Logo" width="160" />
</p>

# Raptor - High-Performance Language Runtime for MoarVM & Native Systems

Raptor is a high-performance, strictly non-OO procedural execution platform and language runtime designed as a **pure dynamic language runtime (Perl5 subset of Raku with no OO)** targeting **MoarVM** (64-bit JIT, 6model virtual machine), native OS dynamic libraries via C FFI, TCP/UDP sockets, real-time audio synthesis (PortAudio), SQLite, Charmbracelet terminal styling, dynamic refinement types with `subset`, and standalone binary packaging (`raptor pack`).

Licensed under the **Artistic License 2.0**.

---

## Standalone Executables (`raptor/bin/`)

| Executable | Description |
| :--- | :--- |
| **`bin/raptor.exe`** | Unified CLI runner, interactive REPL, MoarVM compiler, TAP test harness (`raptor test`), terminal documentation reader (`raptor doc`), and standalone binary packager (`raptor pack`) |
| **`bin/demo_app.exe`** | Standalone packaged executable containing embedded Raptor application and runtime |

---

## Key Features

1. **Pure Dynamic Typing & No-OO Architecture**:
   Built on dynamic variables (`$scalar`, `@array`, `%hash`), subroutines, modules, first-class functions, and C-structs without `class`, `has`, or `is` keyword overhead.
2. **Charmbracelet TUI & Terminal Styling Engine**:
   Lip Gloss ANSI 24-bit TrueColor styling (`tui_style`), framed boxes (`tui_box`), data tables (`tui_table`), progress bars (`tui_progress`), terminal markdown rendering (`tui_markdown`), and Bubble Tea state-machine event loops (`tui_app_run`).
3. **Perl5 TAP Testing Framework & Test Harness (`raptor test`)**:
   Standard Test Anything Protocol v13 producer (`plan`, `ok`, `is`, `isnt`, `is_deeply`, `like`, `unlike`, `cmp_ok`, `subtest`, `done_testing`) and test harness runner (like `prove`).
4. **Verification-Friendly Architecture**:
   Zero-overhead inline tests (`TEST "desc" { ... }`), Design-by-Contract (`pre`, `post`, `invariant`), and Property-Based QuickCheck fuzzing (`property "name", sub ($a, $b) { ... }`).
5. **Perl5-Style Markdown Documentation Suite & `raptor doc`**:
   Comprehensive manual documentation in `docs/` rendered directly in the terminal with ANSI colors: `raptor doc operators`, `raptor doc subsets`, `raptor doc tui`, `raptor doc structs`.
6. **C-ABI Struct Records & Function Pointers**:
   C memory layout records (`struct Point { int32 $x; int32 $y; }`) with first-class function pointer / closure fields (`$btn.onClick = sub ($v) { ... }; $btn.onClick(42)`).
7. **Dynamic Refinement Types (`subset`) & Predicate Dispatching**:
   Enforce dynamic value invariants using Raku-style `subset` and `where` predicates (`subset Positive where { $_ > 0 }`, `multi sub fib($n where { $n <= 1 })`).
8. **Custom Operator Overloading on Structs**:
   Seamless multi-dispatch overloading for structs (`multi sub infix:<+>(Vec2 $a, Vec2 $b)`, `multi sub prefix:<->(Vec2 $v)`).
9. **Procedural Sockets Networking (TCP & UDP)**:
   Fast, low-level socket primitives: `tcp_listen`, `tcp_accept`, `tcp_connect`, `tcp_send`, `tcp_recv`, `tcp_close`, `udp_bind`, `udp_send`, `udp_recv`, and `tcp_close`.
10. **Sockets-Based HTTP/1.1 & RFC 6455 WebSockets**:
    Direct protocol implementation over sockets: `http_get`, `http_post`, `http_server_start`, `ws_frame_text`, `ws_parse_frame`.
11. **PortAudio Sound Engine & Waveform Synthesizer**:
    Real-time audio engine integration (`pa_init`, `pa_terminate`, `pa_device_count`, `pa_device_info`, `pa_sine_wave`).
12. **Native SQLite Database & Fast JSON**:
    Embedded database support (`sqlite_open`, `sqlite_exec`, `sqlite_query`, `sqlite_close`) and JSON serialization (`to_json`, `from_json`).
13. **Comprehensive Perl5 & Raku Operator Suite**:
    Defined-or (`//`, `//=`), Exponentiation (`**`), Ternary (`?? !!`, `? :`), Bitwise numeric (`+&`, `+|`, `+^`, `+<`, `+>`), Repetition (`x`, `xx`), Divisibility (`div`, `mod`, `%%`), Min/Max (`min`, `max`), File tests (`-e`, `-f`, `-d`, `-s`, `-r`, `-w`), and Regex (`=~`, `!~`).
14. **Advanced Concurrency & Atomics**:
    `Mutex`, `Semaphore`, `WaitGroup`, `parallel_map`, reactive `Supply` streams, `Promise` (`start { ... }`), `Channel`, and hardware atomic primitives.
15. **Raylib 5.5 Hardware Graphics Engine Integration**:
    60 FPS desktop window rendering with Windows x64 ABI struct-by-value packing and UTF-8 string marshalling ([examples/raylib_game.rp](examples/raylib_game.rp)).
16. **PodLit Literate Programming Subsystem (Weave, Tangle, Mangle & Stitch)**:
    Knuth-style literate programming via extended POD: weave Markdown docs (`raptor weave`), tangle executable source code with macro chunk expansion (`raptor tangle`), reverse-tangle modified code back into POD (`raptor stitch`), apply mangle filters, and directly execute `.pod` documents ([examples/literate_game.pod](examples/literate_game.pod)).
17. **Standalone Binary Packaging & Bundled DLLs**:
    Package any script into a self-contained `.exe` executable (`raptor pack`), with `moar.dll`, `libraylib.dll`, and `sqlite3.dll` residing directly in `bin/` for zero-dependency portability.
18. **WebAssembly (Wasm) In-Browser IDE & REPL**:
    Run 100% client-side in the browser via `web/raptor.wasm`, featuring interactive REPL prompt, code playground, presets, and live PodLit literate inspector (`raptor serve`).

---

## Quick Start

```powershell
# 1. Run the comprehensive feature showcase
.\bin\raptor.exe run examples/demo_showcase.rp

# 2. Launch the WebAssembly In-Browser REPL & Playground
.\bin\raptor.exe serve --port 8080
# Open http://localhost:8080/ in your browser!

# 3. Run the interactive 60 FPS Raylib desktop game GUI window
.\bin\raptor.exe run examples/raylib_game.rp

# 4. Weave, tangle, and stitch a Literate POD document
.\bin\raptor.exe weave examples/literate_game.pod -o docs/literate_game.md
.\bin\raptor.exe tangle examples/literate_game.pod -o src/
.\bin\raptor.exe stitch examples/literate_game.pod src/lib/LiterateTypes.rp
.\bin\raptor.exe run examples/literate_game.pod

# 5. Run the performance benchmark suite
.\bin\raptor.exe run examples/benchmark.rp

# 6. Run the TAP test harness across all test suites (like prove)
.\bin\raptor.exe test t/

# 7. Read terminal documentation manual with ANSI styling
.\bin\raptor.exe doc operators
.\bin\raptor.exe doc 01_introduction

# 8. Package into a standalone executable (.exe)
.\bin\raptor.exe pack examples/raylib_game.rp -o bin/raylib_game.exe
.\bin\raptor.exe pack examples/demo_showcase.rp -o bin/demo_app.exe

# 9. Execute the standalone executable directly
.\bin\raylib_game.exe
.\bin\demo_app.exe

# 10. Start the interactive terminal REPL
.\bin\raptor.exe

# 11. Run the PHP-Style Template Server (RaptorHP)
.\bin\raptorhp.exe -r '<h1><?= "Hello from RaptorHP" ?></h1>'
.\bin\raptorhp.exe -S localhost:8000

# 12. Run full Go test suite across all subsystems
go test -v ./...
```

---

## Performance & Comparison to Perl 5

| Metric / Dimension | Perl 5 | Raptor (`.rp`) | Performance Advantage |
| :--- | :--- | :--- | :--- |
| **Object / Record Memory Layout** | Hash references (`bless { x => 10, y => 20 }`) with SV/HV tables (~120+ bytes/obj) | Contiguous C-ABI struct memory (`struct Point { int64 $x; int64 $y; }`, 16 bytes) | **7.5x lower memory footprint**; $O(1)$ direct offset lookup bypassing hash buckets |
| **Concurrency & Threading** | `ithreads` (heavy copy-on-write process cloning with serialization) | Native Goroutines / OS threads with lock-free Channels, Promises, and hardware Atomics | **Zero-copy shared memory concurrency** with low-overhead inter-thread communication |
| **C FFI (Foreign Function Interface)** | XS extensions requiring C code compilation, `typemap`, and dynamic loading | Direct `is native('lib.dll')` with Windows x64 register packing | **Zero C compilation steps**; instant 60 FPS desktop rendering via Raylib FFI |
| **Virtual Machine & JIT** | Opcode tree interpreter walk (`OP*` tree) without core JIT | MoarVM 64-bit register VM with type-specializing JIT compiler | **Hardware native code execution** on hot code paths |
| **Dynamic Invariant Validation** | Manual `die unless ...` or heavy module wrappers | Native `subset` refinement types and multi-sub Predicate Dispatch | **Built-in syntax-level contract checks** with zero boilerplate |
| **Tight Arithmetic Loops** | ~0.8s - 1.2s per 1,000,000 loop ops | ~1.0s tree-walk / sub-millisecond MoarVM JIT execution | **Comparable or superior execution speed** with static Flyweight value reuse |

### Benchmark Numbers (`examples/benchmark.rp` on AMD Ryzen 9 5980HX):
- **1,000,000 Integer Calculations**: ~1.0 second (AST tree-walk)
- **500,000 C-Struct Field Mutations**: < 0.1 second ($O(1)$ offset indexing)
- **Recursive Fibonacci `fib(24)`**: < 0.1 second (fast call dispatch)
