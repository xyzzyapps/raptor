---
name: moarvm-language-development
description: Guide and architectural reference for AI agents to develop programming languages targeting MoarVM using the moarvm-go runtime, CompUnit v7 bytecode emitter, 6Model metamodel, and declarative Raku grammars.
---

# Developing Programming Languages with MoarVM-Go

## 1. Overview of MoarVM & moarvm-go

MoarVM is a modern, 64-bit virtual machine built for dynamic and multi-paradigm languages. It features:
- **6Model Object System**: A pluggable metamodel supporting multiple object representations (`P6opaque`, `MVMArray`, `MVMHash`).
- **JIT Compilation**: Type-specializing dynamic JIT compiler targeting x86_64 machine code.
- **Lock-Free Concurrency**: Native OS thread workers, channels, and atomic primitives.
- **`moarvm-go`**: Go host package providing C FFI bindings (`moar.dll`), CompUnit v7 binary bytecode emitter, and declarative Perl 6 / Raku grammar parser.

---

## 2. Embedding MoarVM in Go

### 2.1 Initialization Lifecycle

```go
package main

import (
    "context"
    "fmt"
    "moarvm-go/engine"
)

func main() {
    // 1. Configure engine with dynamic library path
    vm, err := moargo.New(moargo.Config{
        DLLPath:  "bin/moar.dll",
        ProgName: "mylang",
        LibPaths: []string{"lib"},
    })
    if err != nil {
        panic(err)
    }

    // 2. Initialize runtime instance
    ctx := context.Background()
    if err := vm.Init(ctx); err != nil {
        panic(err)
    }
    defer vm.Destroy()

    // 3. Execute bytecode buffer or file
    // vm.RunBytecode(ctx, bytecodeBytes)
    // vm.RunFile(ctx, "program.moarvm")
}
```

---

## 3. Emitting MoarVM CompUnit v7 Bytecode

### 3.1 Binary Container Architecture

A MoarVM Compilation Unit (`.moarvm` format, version 7) contains:
1. **Header (96 bytes)**: Magic number `MOAR\r\n`, version 7, section offsets, and entrypoint frame index.
2. **Serialization Contexts (SC)**: Object graphs, symbols, and type definitions.
3. **Frames Table**: Static descriptors for each subroutine / block.
4. **Callsites Table**: Argument passing metadata for function calls.
5. **String Heap**: Length-prefixed UTF-8 string table.
6. **Bytecode Stream**: 16-bit opcodes and 16-bit register indices.

### 3.2 Constructing a Compilation Unit in Go

```go
import (
    "moarvm-go/engine"
)

// 1. Create new Compilation Unit
cu := moargo.NewCompUnit()

// 2. Define Root Frame (Mainline)
frame := moargo.NewFrame("main", 10) // 10 local registers
frame.SetLocalType(0, moargo.RegInt64)
frame.SetLocalType(1, moargo.RegInt64)
frame.SetLocalType(2, moargo.RegInt64)

// 3. Emit Opcodes
// $0 = 42
frame.EmitOp(moargo.OpConstI64)
frame.EmitReg(0)
frame.EmitInt64(42)

// $1 = 100
frame.EmitOp(moargo.OpConstI64)
frame.EmitReg(1)
frame.EmitInt64(100)

// $2 = $0 + $1
frame.EmitOp(moargo.OpAddI)
frame.EmitReg(2) // destination
frame.EmitReg(0) // left
frame.EmitReg(1) // right

// return $2
frame.EmitOp(moargo.OpReturnI)
frame.EmitReg(2)

// 4. Register frame and serialize binary bytecode
cu.AddFrame(frame)
bytecodeBytes, err := cu.Serialize()
```

---

## 4. Key Opcode Cheatsheet

| Opcode Constant | Opcode ID | Format | Description |
| :--- | :--- | :--- | :--- |
| `OpNoop` | 0 | `noop` | No operation |
| `OpConstI64` | 3 | `const_i64 $dst, int64` | Load 64-bit integer constant |
| `OpConstN64` | 4 | `const_n64 $dst, float64` | Load 64-bit float constant |
| `OpConstS` | 5 | `const_s $dst, str_idx` | Load string from heap |
| `OpAddI` | 7 | `add_i $dst, $a, $b` | 64-bit integer addition |
| `OpSubI` | 8 | `sub_i $dst, $a, $b` | 64-bit integer subtraction |
| `OpMulI` | 9 | `mul_i $dst, $a, $b` | 64-bit integer multiplication |
| `OpDivI` | 10 | `div_i $dst, $a, $b` | 64-bit integer division |
| `OpModI` | 11 | `mod_i $dst, $a, $b` | Integer modulo |
| `OpEqI` | 13 | `eq_i $dst, $a, $b` | Integer equality check ($dst = 1$ or $0$) |
| `OpLtI` | 17 | `lt_i $dst, $a, $b` | Integer less-than check |
| `OpGoto` | 23 | `goto int32_offset` | Unconditional jump |
| `OpIfI` | 24 | `ifi $cond, int32_offset` | Jump if integer condition is non-zero |
| `OpUnlessI` | 25 | `unlessi $cond, int32_offset`| Jump if integer condition is zero |
| `OpPrepArgs` | 40 | `prepargs callsite_idx` | Prepare argument list for invocation |
| `OpArgI` | 41 | `arg_i pos, $reg` | Push integer argument |
| `OpInvoke` | 50 | `invoke $ret, $sub` | Invoke subroutine / frame |
| `OpReturn` | 60 | `return` | Void return |
| `OpReturnI` | 61 | `return_i $reg` | Return 64-bit integer |
| `OpSay` | 80 | `say $reg` | Print value with trailing newline |
| `OpPrint` | 81 | `print $reg` | Print value to stdout |

---

## 5. Declarative Raku Grammar Engine

`moarvm-go/grammar/` provides a parsing engine for building ASTs from declarative `.raku` grammar files.

### 5.1 Grammar Definition (`calc.raku`)

```raku
grammar Calculator {
    rule TOP { <expr> }
    rule expr is optable { ... }
    
    proto token infix:<+> is tighter { <sym> }
    proto token infix:<*> is tighter { <sym> }
    
    token integer { \d+ }
    token term:sym<int> { <integer> }
}
```

### 5.2 Parsing in Go

```go
package main

import (
    "moarvm-go/grammar"
)

func main() {
    g, err := grammar.LoadGrammarFile("calc.raku")
    if err != nil {
        panic(err)
    }

    match := g.Parse("42 + 10 * 2", "TOP")
    if !match.Success {
        panic("Parse failed")
    }
    
    // Walk parse tree and generate MoarVM bytecode
}
```

---

## 6. Blueprint for Implementing a New Language

1. **Syntax Definition**: Write a `.raku` grammar file in `grammar/` defining tokens, rules, and operator precedence tables.
2. **AST Structures**: Create Go structs for AST Nodes (`Program`, `Stmt`, `Expr`, `FnDecl`, `Call`).
3. **Parser / Actions**: Map grammar match reductions to your Go AST Nodes.
4. **Bytecode Compiler**: Walk AST nodes and emit MoarVM Compilation Units (`moargo.NewCompUnit()`) with local register allocations and branch jump offsets.
5. **Execution**: Load `bin/moar.dll` and execute the compiled bytecode.
