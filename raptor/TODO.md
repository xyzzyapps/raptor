# Raptor Task & Roadmap (TODO.md)

## Active Milestones
- [x] License: Artistic License 2.0 (`raptor/LICENSE`, `moarvm-go/LICENSE`)
- [x] Charmbracelet TUI & Styling Engine (Lip Gloss ANSI styling, box framing, tables, progress bars, terminal markdown, Bubble Tea state machines)
- [x] Full Codebase Edge-Case Audit (safe division, negative array indexing, nil coercion safety, negative numeric literal parsing)
- [x] Perl5-Style TAP Testing Framework (`plan`, `ok`, `is`, `isnt`, `is_deeply`, `like`, `unlike`, `cmp_ok`, `subtest`, `done_testing`)
- [x] `raptor test` CLI Test Harness (like `prove`) with colored results and summary reports
- [x] Verification-Friendly Subsystem: Zero-overhead inline `TEST` blocks, Design-by-Contract (`pre`, `post`, `invariant`), and QuickCheck Property Fuzzing
- [x] Perl5-Style Markdown Documentation Suite (`docs/01_introduction.md` through `docs/09_testing_and_verification.md`)
- [x] Terminal Documentation Reader: `raptor doc <topic>` (or `raptor perldoc <topic>`) with markdown styling
- [x] Primary File Extensions: `.rp` and `.raptor` (`examples/raylib_game.rp`, `examples/demo_showcase.rp`, `lib/*.rp`)
- [x] Standalone Single-Binary Packager & Executables (`bin/raptor.exe`, `bin/raylib_game.exe`, `bin/demo_app.exe`)
- [x] Raylib 5.5 Desktop GUI Window & Game Engine (`examples/raylib_game.rp`, `lib/Raylib.rp`, `bin/raylib_game.exe`)
- [x] High-Performance Engine Optimizations: Value Flyweight singletons, Fast-Path Operator Dispatch, Loop Frame Recycling, O(1) Struct Field Tables, and Benchmark Suite (`examples/benchmark.rp`, `runtime/benchmark_test.go`)
- [x] PodLit Literate Programming Subsystem: Weave to Markdown, Tangle to Source Code, Mangle transformations, Stitch reverse-tangling, and direct `.pod` execution (`runtime/podlit.go`, `examples/literate_game.pod`, `docs/10_literate_programming.md`, `t/06_podlit_literate.t`)
- [x] Bundled Runtime Dynamic Libraries in `bin/` (`bin/moar.dll`, `bin/libraylib.dll`, `bin/sqlite3.dll`) for zero-dependency standalone execution
- [x] WebAssembly (Wasm) Compilation Target & In-Browser Interactive REPL/IDE (`cmd/wasm/main.go`, `web/raptor.wasm`, `web/index.html`, `web/style.css`, `web/app.js`, `raptor serve`)
- [x] MoarVM 64-Bit Full AST Bytecode Compilation & JIT Engine (`moarvm-go/engine`, `runtime/compiler.go`, `moar.dll`)

## Completed Core Runtime Milestones
- [x] Pure Dynamic Typing (Removal of `class`, `has`, `is` keywords and static type annotations)
- [x] `struct` as Sole Compound Record Type with C-ABI Memory Layouts and C FFI
- [x] Struct Function Pointers & Closures (`$struct.fn = sub (...) { ... }; $struct.fn(args...)`)
- [x] Named Dynamic Refinement Types with Raku `subset` (`subset Positive where { $_ > 0 }`)
- [x] Predicate Dispatching & Multiple Dispatch on `where` Predicates and `subset` Types
- [x] Custom Operator Overloading on Structs (`multi sub infix:<+>(Vec2 $a, Vec2 $b)`)
- [x] Raylib Windows Drawing & C FFI ABI Refinement (UTF-8 strings, small struct value packing)
- [x] Procedural Sockets for TCP and UDP programming (`runtime/socket.go`, `lib/Socket.raku`)
- [x] Sockets-based HTTP Client/Server and RFC 6455 WebSocket Framing (`runtime/http.go`, `runtime/websocket.go`)
- [x] PortAudio Sound Engine Integration & Synthesizer (`runtime/portaudio.go`, `lib/PortAudio.raku`)
- [x] Native SQLite Database Support (`runtime/sqlite.go`)
- [x] Fast JSON Serialization and Deserialization (`runtime/json.go`)
- [x] Comprehensive Perl5 & Raku Base Functions (Math, String, Filesystem, List, Hash, System)
- [x] Predefined Environment Globals (`%*ENV`, `%ENV`, `@*ARGS`, `@ARGV`, `$*PID`, `$*CWD`, `$*OS`)
- [x] Raku `sub MAIN` and `multi sub MAIN` CLI Entry Point & Parameter Dispatch
- [x] Perl5 & Raku Operator Suite (`//`, `//=`, `**`, `?? !!`, `? :`, `+&`, `+|`, `+^`, `+<`, `+>`, `x`, `xx`, `div`, `mod`, `%%`, `min`, `max`, `-e`, `-f`, `-d`, `-s`, `=~`, `!~`)
- [x] Advanced Concurrency Primitives (Mutex, Semaphore, WaitGroup, Parallel Map, Channel Select, Supply)
- [x] Quantum Autothreading Junctions (`all`, `any`, `one`, `none`) with comparison and logical autothreading
- [x] Deep Pattern Matching & Signature Destructuring for arrays (`[$head, *@tail]`) and hashes (`:{:$name, :$role}`)
