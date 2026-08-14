# Raptor & MoarVM-Go

High-performance dynamic language platform, 64-bit virtual machine host, and developer toolchain.

Licensed under the **Artistic License 2.0**.

---

## Subprojects

### 1. [Raptor (`raptor/`)](raptor/README.md)
Raptor is a high-performance procedural execution platform and dynamic language (Perl 5 subset of Raku without OO overhead).
- **Core Runtime**: Pure dynamic typing (`$scalar`, `@array`, `%hash`), Uniform Function Call Syntax (UFCS), C-ABI `struct` records, function pointer closures, and Raku-style dynamic `subset` refinements with `where` predicate dispatching.
- **WebAssembly (Wasm) Go Tour**: 100% client-side in-browser playground (`web/raptor.wasm`, `raptor serve --port 8080`) with Canvas 2D graphics, WebAudio sound synthesizer, DOM manipulation, and interactive Go Tour lessons.
- **RaptorHP Template Server (`raptorhp.exe`)**: PHP-style embedded templating engine and development HTTP server with superglobals (`%_GET`, `%_POST`, `%_SERVER`).
- **Package Manager**: Git-based package management (`raptor init`, `raptor get <repo>`, `raptor install`) cloning dependencies directly into `./raptor_modules/`.
- **PodLit Literate Programming**: Knuth-style literate programming (`raptor weave`, `raptor tangle`, `raptor stitch`).
- **NativeCall C FFI & Raylib Graphics**: 60 FPS desktop hardware GUI graphics (`examples/raylib_game.rp`), PortAudio V19 audio synthesis, and SQLite.
- **Verification & Testing**: Full TAP v13 test producer, `raptor test` harness (like `prove`), Design-by-Contract (`pre`, `post`, `invariant`), and QuickCheck fuzzing.
- **Single-Binary Packager**: Packages scripts into standalone `.exe` binaries (`raptor pack`).

### 2. [MoarVM-Go (`moarvm-go/`)](moarvm-go/README.md)
A high-performance Go host and FFI binding for **MoarVM** (64-bit 6Model JIT virtual machine via `moar.dll`).
- **CompUnit v7 Emitter**: Serializes valid binary bytecode with Serialization Contexts (SC), frame tables, and register descriptors.
- **Metamodel (6Model)**: Pluggable object representations (`P6opaque`, `MVMArray`, `MVMHash`).
- **Declarative Raku Grammars**: Pure-Go pattern matching and Pratt operator precedence parsing engine.
- **Tcl Reference Frontend**: Strict Tcl language implementation with C and Go FFI.

---

## Agent Skills (`.agents/skills/`)
- **[moarvm-language-development](.agents/skills/moarvm-language-development/SKILL.md)**: Guide for agents on building new compiled languages targeting MoarVM.
- **[raptor-language-guide](.agents/skills/raptor-language-guide/SKILL.md)**: Comprehensive guide for agents on the Raptor runtime, C FFI, Wasm, RaptorHP, and PodLit.


