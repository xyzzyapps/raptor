# moarvm-go Task & Roadmap (TODO.md)

## Completed Milestones
- [x] MoarVM Go Host Engine (`engine/`) with dynamic Win32 FFI loading `moar.dll`
- [x] Cross-platform POSIX engine support (`engine/vm_posix.go`) and Windows engine (`engine/vm_windows.go`)
- [x] CompUnit v7 Bytecode Emitter (`bytecode.go`, `opcodes.go`) with frame and register allocation
- [x] Full 6Model Serialization Context (`SerializationContext`, `STable`, `Repossession`, binary serialization/deserialization)
- [x] 6Model Metamodel & KnowHOW Object System (`sixmodel.go`)
- [x] Perl 6 / Raku Declarative Grammar & Regex Pattern Matching Engine (`grammar/`)
- [x] 100% Declarative Grammar-Driven Tcl Interpreter (`tcl/tcl.raku`, `tcl_grammar.go`)
- [x] Strict Tcl Language Semantics (verbatim braces, interpolated quotes, command substitutions, variables, escapes)
- [x] Comprehensive Standard Tcl Builtin Command Suite (`builtins.go`)
- [x] C FFI Engine (`cffi.go`) with library loading, function binding, and direct invocations
- [x] Go FFI Engine (`goffi.go`) with runtime reflection function binding
- [x] Elimination of obsolete hand-written parser (`parser.go` deleted)
- [x] Interactive Tcl Shell & REPL (`cmd/tcl/main.go`, `bin/tcl.exe`)
- [x] Standalone MSYS2 / UCRT64 MoarVM compilation script (`vendor/apply_patches_msys.sh`)
- [x] Unit test suites across `engine`, `grammar`, and `tcl` (100% PASSING)
