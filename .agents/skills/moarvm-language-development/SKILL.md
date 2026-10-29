---
name: moarvm-language-development
description: Guide and architectural reference for AI agents to develop programming languages targeting MoarVM using the moarvm-go runtime, CompUnit v7 bytecode emitter, 6Model metamodel, and declarative NQP / QRegex grammars via gcre.
---

# Developing Programming Languages with MoarVM-Go & gcre

## 1. Overview of MoarVM, `moarvm-go`, and `gcre`

MoarVM is a 64-bit virtual machine built for dynamic and multi-paradigm languages. It features:
- **6Model Object System**: Pluggable metamodel supporting representations such as `P6opaque`, `VMArray`, and `MVMHash`.
- **JIT Compilation**: Dynamic type-specializing JIT targeting x86_64 machine code.
- **Continuations & Concurrency**: Delimited continuations (`continuationreset` / `continuationcontrol` / `continuationinvoke`), native OS thread workers, and channels.
- **`moarvm-go`**: Go host package providing C FFI bindings (`moar.dll`), CompUnit v7 binary bytecode emitter (`CompUnitEmitter`), and a Tcl compiler frontend.
- **`gcre`**: Pure-Go declarative NQP / QRegex grammar engine (`grammar ... is Parent`, `proto` / `:sym<...>`, LTM `|` vs `||`, `is optable`, `%` / `%%`, `~` goal matching, lookarounds, character class set algebra).

> **Note:** Always pass `-mod=mod` when running `go build`, `go run`, or `go test` in `moarvm-go/` and `raptor/` (`moarvm-go/vendor/MoarVM` contains C sources rather than Go vendor modules).

---

## 2. Embedding MoarVM in Go

### 2.1 Initialization Lifecycle

```go
package main

import (
    "context"
    "moarvm-go/engine"
)

func main() {
    vm, err := moargo.New(moargo.Config{
        DLLPath:  "bin/moar.dll",
        ProgName: "mylang",
        LibPaths: []string{"lib"},
    })
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    if err := vm.Init(ctx); err != nil {
        panic(err)
    }
    defer vm.Destroy()

    // Execute in-memory CompUnit v7 bytecode or .moarvm file:
    // _ = vm.RunBytecode(ctx, bytecodeBytes)
    // _ = vm.RunFile(ctx, "program.moarvm")
}
```

---

## 3. Emitting MoarVM CompUnit v7 Bytecode

### 3.1 Binary Container Architecture

A MoarVM Compilation Unit (`.moarvm` format, version 7) contains:
1. **Header (96 bytes)**: Magic `MOARVM\r\n`, version 7, section counts/offsets, and entrypoint frame index.
2. **Serialization Contexts (SC)**: Object graphs, symbols, and type definitions.
3. **Frames Table**: Static descriptors, local register types, and lexical slots for each routine/closure.
4. **Callsites Table**: Argument flags (`CallArgObj`, `CallArgInt`, `CallArgNum`, `CallArgStr`).
5. **String Heap**: UTF-8 string table (index 0 is HLL name).
6. **Bytecode Stream**: Official 16-bit MoarVM opcodes (`vendor/MoarVM/src/core/ops.h`).

### 3.2 Constructing a Compilation Unit in Go

```go
import "moarvm-go/engine"

cu := moargo.NewCompUnitEmitter("mylang")
frame := cu.NewFrame("main", 3)
frame.SetLocalType(0, moargo.RegInt64)
frame.SetLocalType(1, moargo.RegInt64)
frame.SetLocalType(2, moargo.RegInt64)

// r0 = 42; r1 = 100; r2 = r0 + r1; return r2
frame.EmitOp(moargo.OpConstI64)
frame.EmitReg(0)
frame.EmitInt64(42)

frame.EmitOp(moargo.OpConstI64)
frame.EmitReg(1)
frame.EmitInt64(100)

frame.EmitOp(moargo.OpAddI)
frame.EmitReg(2)
frame.EmitReg(0)
frame.EmitReg(1)

frame.EmitOp(moargo.OpReturnI)
frame.EmitReg(2)

bytecodeBytes, err := cu.Emit()
```

---

## 4. Official MoarVM Opcode Cheatsheet (`moarvm-go/engine/opcodes.go`)

| Opcode Constant | Official ID | Format | Description |
| :--- | :--- | :--- | :--- |
| `OpNoOp` | 0 | `noop` | No operation |
| `OpConstI64` | 4 | `const_i64 $dst, int64` | Load 64-bit integer constant |
| `OpConstN64` | 6 | `const_n64 $dst, float64` | Load 64-bit float constant |
| `OpConstS` | 7 | `const_s $dst, str_idx` | Load string from heap |
| `OpGoto` | 23 | `goto uint32_offset` | Unconditional frame-offset jump |
| `OpIfI` | 24 | `if_i $cond, uint32_offset` | Jump if integer condition is non-zero |
| `OpUnlessI` | 25 | `unless_i $cond, uint32_offset` | Jump if integer condition is zero |
| `OpGetLex` / `OpBindLex` | 35 / 36 | `getlex` / `bindlex` | Read / write lexical slot across outer frames |
| `OpPrepArgs` / `OpInvoke` | 40 / 49 | `prepargs cs_idx` / `invoke` | Prepare callsite and invoke code object |
| `OpReturnI` / `OpReturn` | 51 / 55 | `return_i $reg` / `return` | Return `int64` or void |
| `OpEqI` / `OpLtI` | 56 / 58 | `eq_i` / `lt_i $dst, $a, $b` | 64-bit integer comparisons |
| `OpAddI` / `OpSubI` / `OpMulI` / `OpDivI` | 63 / 64 / 65 / 66 | `add_i $dst, $a, $b` | 64-bit integer arithmetic |
| `OpGetCode` / `OpTakeClosure` | 159 / 162 | `getcode` / `takeclosure` | Reference frame and instantiate closure |
| `OpConcatS` | 208 | `concat_s $dst, $a, $b` | Concatenate strings |
| `OpPrint` / `OpSay` | 494 / 495 | `print $reg` / `say $reg` | Write string to stdout (without / with newline) |
| `OpDispatchV`..`OpDispatchO` | 826..830 | `dispatch_*` | Unified MoarVM boot-code / callsite dispatch |

---

## 5. Declarative NQP / QRegex Grammar Engine (`gcre`)

`gcre` compiles declarative `.raku` grammar files into matcher combinators and Pratt operator tables.

### 5.1 Grammar Definition (`calc.raku`)

```raku
grammar Calculator {
    rule TOP { ^ <EXPR> $ }
    token EXPR is optable { <term> }

    proto token term { * }
    token term:sym<int>   { \d+ {*} }
    token term:sym<group> { '(' ~ ')' [ <.ws> <EXPR> <.ws> ] {*} }

    proto token prefix { * }
    token prefix:sym<-> is tighter<*> { <sym> }

    proto token infix { * }
    token infix:sym<+>  { <sym> }
    token infix:sym<*>  is tighter<+> { <sym> }
    token infix:sym<**> is tighter<*> is assoc<right> { <sym> }
}
```

### 5.2 Parsing & Reducing in Go

```go
package main

import "gcre"

type CalcActions struct{}

func (a *CalcActions) HandleAction(rule string, m *gcre.Match) {
    // Reduce matched subtrees via m.Make(astNode)
}

func main() {
    g, err := gcre.LoadGrammarFromFile("calc.raku")
    if err != nil {
        panic(err)
    }

    match, err := g.Parse("42 + 10 * 2 ** 3", &CalcActions{})
    if err != nil || !match.Ok {
        panic(err)
    }
}
```

---

## 6. Blueprint for Implementing a New Language

1. **Syntax Definition**: Write a declarative `.raku` grammar using `gcre` (`grammar`, `proto`/`:sym<...>`, `is optable`, `%`/`%%`, `~`, lookarounds).
2. **AST Structures**: Define Go AST nodes (`Program`, `Stmt`, `Expr`, `FnDecl`, `Call`).
3. **Actions / Host Hatch**: Map match reductions to Go AST nodes via `gcre.ActionHandler` (and optional `<HOST_name>` callbacks if stateful lexing is needed).
4. **Bytecode Compiler**: Walk AST nodes and emit MoarVM Compilation Units (`moargo.NewCompUnitEmitter`) with typed local registers, lexical slots, and frame jump offsets.
5. **Execution**: Execute in-process via `moargo.RunNative(bc)` / `vm.RunBytecode(ctx, bc)` on `moar.dll`.

## AI credit

The first prototype was written by **Gemini**. Later work was modified by **Grok**.
