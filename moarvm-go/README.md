# moarvm-go

A high-performance Go host and FFI binding for **MoarVM** (the 6model 64-bit virtual machine with JIT compilation) featuring a full CompUnit v7 bytecode emitter, 6Model metamodel definitions, declarative Perl 6 / Raku grammar engine, and a 100% grammar-driven **Tcl** reference language implementation with C & Go FFI.

---

## What is moarvm-go?

`moarvm-go` enables Go developers to:
- **Embed MoarVM**: Load and control the native MoarVM JIT runtime directly from Go applications on Windows (via `moar.dll`).
- **Emit Bytecode**: Generate valid MoarVM CompUnit v7 binary bytecode files with frame registers, string heaps, and serialization contexts.
- **Parse with Raku Grammars**: Define programming language grammars declaratively in `.raku` files and parse them using a pure Go pattern matching engine.
- **Run Dynamic Languages**: Includes a reference implementation of **Tcl** with strict syntax, C FFI, Go FFI, and MoarVM bridge.

---

## Quick Start

### 1. Run the Interactive Tcl Shell
```powershell
go run ./cmd/tcl
```

### 2. Execute Tcl Scripts
```powershell
# Run sample Tcl script
go run ./cmd/tcl examples/demo_tcl.tcl

# Run C FFI demo (calling Win32 & MoarVM APIs)
go run ./cmd/tcl examples/test_ffi.tcl

# Run MoarVM bridge demo
go run ./cmd/tcl examples/test_bridge.tcl
```

### 3. Embed MoarVM in Go
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

### 4. Embed Tcl with Custom Go Functions
```go
package main

import (
    "fmt"
    "moarvm-go/tcl"
)

func main() {
    in := tcl.NewInterp()

    // Expose Go functions directly to Tcl via reflection
    _ = tcl.RegisterGoFunc(in, "go_multiply", func(a, b int) int {
        return a * b
    })

    res, _ := in.Eval("go_multiply 6 7")
    fmt.Println("Result from Tcl:", res) // 42
}
```

---

## Running Tests

```powershell
go test -v ./...
```

---

## Documentation Links

- **[SPEC.md](SPEC.md)**: Engineering System Requirements Specification and Architecture Document for MoarVM Go Host Engine, Bytecode Emitter, 6Model, and Grammar Engine.
- **[TUTORIAL.md](TUTORIAL.md)**: Complete developer guide and command reference for the Tcl language frontend, declarative grammar integration, and C/Go FFI.
- **[TODO.md](TODO.md)**: Development roadmap and milestones.
