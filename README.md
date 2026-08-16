# Raptor

[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](raptor/t/)
[![License: Artistic-2.0](https://img.shields.io/badge/License-Artistic_2.0-0298c3.svg)](https://opensource.org/licenses/Artistic-2.0)

A **Perl 5–shaped** language (the dynamic, non-OO subset of Raku): sigils, `$_`, `//`, UFCS, `struct`, `subset`, no `class` / `has`. Go host, optional **MoarVM** backend.

If you know Perl 5, you already know Raptor.

| Perl 5 | Raptor |
| :--- | :--- |
| `my $x`, `my @a`, `my %h` | same |
| `$_` topic | same (`for`, `given`, bare `say`) |
| `.` concat | `~` |
| `x` / `//` / `eq ne lt gt` | same (`//=` too; also `==` numeric) |
| `$obj->method` | `$obj.method()` **UFCS** (any `sub method($obj, …)`) |
| `bless` / `@ISA` | no classes — `struct` + `multi sub` |
| `package Foo;` | same; `%Foo::` stash |
| TAP `ok` / `is` | built in (`raptor test t/`) |
| XS | `is native('lib.dll')` NativeCall |

```
cd raptor
go build -mod=mod -o bin/raptor.exe ./cmd/raptor
raptor script.rp          # --go interpreter
raptor --moar script.rp   # CompUnit v7 on moar.dll
raptor test t/
raptor serve              # WASM tour :8080
raptor -S localhost:8000  # PHP-style RaptorHP
```

Full language, CLI, FFI, WASM, and build notes: **[raptor/README.md](raptor/README.md)**.

This tree is three siblings:

| Path | What |
| :--- | :--- |
| **[raptor/](raptor/README.md)** | Language, runtime, TAP, PodLit, WASM tour, RaptorHP, pack |
| **[gcre/](gcre/README.md)** | Grammar Compatible Regular Expressions (Raku ⊂ PEG; `<HOST_name>`) |
| **[moarvm-go/](moarvm-go/README.md)** | MoarVM embed + Tcl frontend (same CompUnit v7 family) |

Parse is **gcre** (`raptor/runtime/raptor.raku`, `moarvm-go/tcl/tcl.raku`). Pratt runs only where the grammar writes `<HOST_stmt>` / `<HOST_expr>`.

Licensed under the **Artistic License 2.0**.

The first prototype was written by **Gemini**. The language was mostly written by Anti Gravity — Gemini 3.6. Library bindings and PodLit were written by Gemini 3.7. Later work was modified by **Grok**.
