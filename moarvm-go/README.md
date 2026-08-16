# moarvm-go

A Go host and FFI binding for **MoarVM** (the 6model 64-bit virtual machine) with a CompUnit v7 bytecode emitter, 6Model types, **[gcre](../gcre)** grammars (Raku subset, PEG-compatible), and a **Tcl** frontend. Pipeline:

**grammar parse → Go compiler emits CompUnit v7 → `moar.exe` / `moar.dll` runs it.**

---

## Pipeline

```
Tcl source
    │
    ▼
Go compiler  (grammar AST → CompUnit v7)
    │
    ▼
moar.dll in-process   (MVM_vm_run_bytecode)
    or moar.exe
```

Ship `moar.dll` (Windows) or `libmoar.so` next to the `tcl` binary. The host looks in the executable directory first, then `MOAR_DLL`, then `bin/moar.dll`.

Parsing is not “evaluate as you tokenize.” The grammar only builds an AST. Semantics live in Go, and those operations **write MoarVM opcodes** (`const_i64`, `add_i`, `if_i`, `say`, `getcode`, `takeclosure`, …). Output comes from running that bytecode. `apply` is a real closure frame; `coroutine` / `yield` are delimited continuations. Commands the compiler cannot emit (`proc`, FFI, `string`, `foreach`, …) still fall back to Go builtins.

---

## Quick Start

### Interactive shell
```powershell
go run ./cmd/tcl
```

### Run a script (parse → bytecode → interpret)
```powershell
go run ./cmd/tcl examples/demo_tcl.tcl
```

### Emit a `.moarvm` file
```powershell
go run ./cmd/tcl -emit-only -o build/demo.moarvm examples/demo_tcl.tcl
```

### Run via bundled `moar.dll`
```powershell
# DLL next to the binary, or:
go run ./cmd/tcl -dll bin/moar.dll examples/demo_tcl.tcl
```

### Other samples
```powershell
go run ./cmd/tcl examples/test_ffi.tcl
go run ./cmd/tcl examples/test_bridge.tcl
```

Native execution needs a complete CompUnit (serialization context, code objects). Homemade units from this emitter are valid enough for the **software interpreter**; `moar.dll` may reject or ignore frames that lack SC data. That is a known binding gap (see SPEC.md).

---

## Embed MoarVM in Go

```go
package main

import (
    "context"
    "fmt"
    "moarvm-go/engine"
)

func main() {
    vm, err := moargo.New(moargo.Config{
        DLLPath:  "build/moarvm/bin/moar.dll",
        ProgName: "myapp",
    })
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    if err := vm.Init(ctx); err != nil {
        panic(err)
    }
    defer vm.Destroy()

    fmt.Printf("MoarVM State: %s\n", vm.State())
}
```

## Compile Tcl from Go

```go
bc, err := tcl.NewCompiler().CompileScript(script)
out, err := moargo.RunNative(bc) // loads moar.dll from the exe dir / MOAR_DLL / bin/
```

---

## Tests

```powershell
go test -v ./...
```

---

## Documentation

- **[SPEC.md](SPEC.md)** — architecture, binding audit, CompUnit layout, gaps.
- **[TUTORIAL.md](TUTORIAL.md)** — Tcl frontend, grammar, compiler ops, running bytecode.
- **[TODO.md](TODO.md)**: Development roadmap and milestones.

## AI credit

The first prototype was written by **Gemini**. Later work was modified by **Grok**.
