<p align="center">
  <img src="assets/logo_wt.png" alt="Raptor Language Logo" width="160" />
</p>

# Raptor 

Raptor is a dynamic language - roughly a Perl5 subset of Raku targeting **MoarVM** in go. It supports go FFI, C FFI, refinement types, predicate types, multimethods, defensive programming, literate programming  and standalone binary packaging (`raptor pack`).

It comes with a few sample libraries to aid in development and testing

1. **Charmbracelet TUI & Terminal Styling Engine**:
   Lip Gloss ANSI 24-bit TrueColor styling (`tui_style`), framed boxes (`tui_box`), data tables (`tui_table`), progress bars (`tui_progress`), terminal markdown rendering (`tui_markdown`), and Bubble Tea state-machine event loops (`tui_app_run`).
2. **PortAudio Sound Engine & Waveform Synthesizer**:
    Real-time audio engine integration (`pa_init`, `pa_terminate`, `pa_device_count`, `pa_device_info`, `pa_sine_wave`).
3. **Native SQLite Database & Fast JSON**:
    Embedded database support (`sqlite_open`, `sqlite_exec`, `sqlite_query`, `sqlite_close`) and JSON serialization (`to_json`, `from_json`).
4. **Procedural Sockets Networking (TCP & UDP)**:
   Fast, low-level socket primitives: `tcp_listen`, `tcp_accept`, `tcp_connect`, `tcp_send`, `tcp_recv`, `tcp_close`, `udp_bind`, `udp_send`, `udp_recv`, and `tcp_close`.
5. **Sockets-Based HTTP/1.1 & RFC 6455 WebSockets**:
    Direct protocol implementation over sockets: `http_get`, `http_post`, `http_server_start`, `ws_frame_text`, `ws_parse_frame`.
6. **Raylib 5.5 Hardware Graphics Engine Integration**:
    60 FPS desktop window rendering with Windows x64 ABI struct-by-value packing and UTF-8 string marshalling ([examples/raylib_game.rp](examples/raylib_game.rp)).

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
2. **Perl5 TAP Testing Framework & Test Harness (`raptor test`)**:
   Standard Test Anything Protocol v13 producer (`plan`, `ok`, `is`, `isnt`, `is_deeply`, `like`, `unlike`, `cmp_ok`, `subtest`, `done_testing`) and test harness runner (like `prove`).
3. **Verification-Friendly Architecture**:
   Zero-overhead inline tests (`TEST "desc" { ... }`), Design-by-Contract (`pre`, `post`, `invariant`), and Property-Based QuickCheck fuzzing (`property "name", sub ($a, $b) { ... }`).
4. **PodLit Literate Programming Subsystem (Weave, Tangle, Mangle & Stitch) & `raptor doc`**:
    Knuth-style literate programming via extended POD: weave Markdown docs (`raptor weave`), tangle executable source code with macro chunk expansion (`raptor tangle`), reverse-tangle modified code back into POD (`raptor stitch`), apply mangle filters, and directly execute `.pod` documents ([examples/literate_game.pod](examples/literate_game.pod)).
   Comprehensive manual documentation in `docs/` rendered directly in the terminal with ANSI colors: `raptor doc operators`, `raptor doc subsets`, `raptor doc tui`, `raptor doc structs`.
5. **C-ABI Struct Records & Function Pointers**:
   C memory layout records (`struct Point { int32 $x; int32 $y; }`) with first-class function pointer / closure fields (`$btn.onClick = sub ($v) { ... }; $btn.onClick(42)`).
6. **Dynamic Refinement Types (`subset`) & Predicate Dispatching**:
   Enforce dynamic value invariants using Raku-style `subset` and `where` predicates (`subset Positive where { $_ > 0 }`, `multi sub fib($n where { $n <= 1 })`).
7. **Custom Operator Overloading on Structs**:
   Seamless multi-dispatch overloading for structs (`multi sub infix:<+>(Vec2 $a, Vec2 $b)`, `multi sub prefix:<->(Vec2 $v)`).
8. **Comprehensive Perl5 & Raku Operator Suite**:
    Defined-or (`//`, `//=`), Exponentiation (`**`), Ternary (`?? !!`, `? :`), Bitwise numeric (`+&`, `+|`, `+^`, `+<`, `+>`), Repetition (`x`, `xx`), Divisibility (`div`, `mod`, `%%`), Min/Max (`min`, `max`), File tests (`-e`, `-f`, `-d`, `-s`, `-r`, `-w`), and Regex (`=~`, `!~`).
9. **Advanced Concurrency & Atomics**:
    `Mutex`, `Semaphore`, `WaitGroup`, `parallel_map`, reactive `Supply` streams, `Promise` (`start { ... }`), `Channel`, and hardware atomic primitives.
10. **Standalone Binary Packaging & Bundled DLLs**:
    Package any script into a self-contained `.exe` executable (`raptor pack`), with `moar.dll`, `libraylib.dll`, and `sqlite3.dll` residing directly in `bin/` for zero-dependency portability.
11. **WebAssembly (Wasm) In-Browser IDE & REPL**:
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

