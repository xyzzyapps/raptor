# Tcl on MoarVM — Tutorial

## 1. What this frontend is

`moarvm-go/tcl` is a Tcl-shaped language that uses **MoarVM constructs** end to end:

| Stage | Where | What |
| :--- | :--- | :--- |
| Parse | [`tcl/tcl.raku`](tcl/tcl.raku) via **[gcre](../gcre)** | Commands and words only. No evaluation. gcre is a Raku subset that is PEG-compatible. |
| Operations | Go (`tcl/compiler.go`) | Each Tcl command is a Go function that **emits MoarVM opcodes**. |
| Image | `engine.CompUnitEmitter` | CompUnit v7 (`.moarvm`): header, frames, string heap, bytecode. |
| Run | `moar.dll` via Go (`MVM_vm_run_bytecode`) | Same process as the host; ship the DLL with the binary. |

Commands the compiler cannot emit (`proc`, `foreach`, `string`, C/Go FFI, `moar::*`) still run as Go builtins after a failed compile of the whole script. Nested `eval` inside those builtins never re-enters the bytecode compiler, so scopes stay consistent.

---

## 2. Parse (MoarVM-side grammar)

The grammar is declarative Raku notation, compiled to combinators:

```raku
grammar Tcl {
    rule TOP { <command_line>* }
    rule command { <word>+ }
    token word {
        | <braced_word> | <quoted_word>
        | <cmd_subst> | <var_subst> | <bare_word>
    }
}
```

`ParseTclAST(script)` returns `[]Command` of `Word` values (`bare`, `brace`, `quote`, `$var`, `[cmd]`). Substitutions are **not** applied at parse time. Braces stay literal; quotes and command words are compiled to `const_s` / nested command emission.

---

## 3. Operations emit bytecode

Go operations allocate registers and write official MoarVM ops (`vendor/MoarVM/src/core/ops.h`):

| Tcl | MoarVM ops |
| :--- | :--- |
| `set x 3` | `const_i64 $r, 3` |
| `incr x` | `inc_i $r` |
| `expr $a + $b` | `add_i $dst, $a, $b` (also `- * / % == != < <= > >=`) |
| `puts $msg` | `const_s` / `concat_s` / `say` |
| `if` / `while` / `for` | `unless_i` + `goto` (absolute frame offsets) |
| `list a b` | `const_s` + `concat_s` |
| `apply {args body} ...` | extra frame + `getcode` + `takeclosure` + `dispatch_*` `boot-code` |
| `coroutine` / `yield` | `continuationreset` / `continuationcontrol` / `continuationinvoke` |
| `llength` / `lindex` | compile-time list values or a compile error |

Example compiled shape for `set a 50; set b 12; expr $a + $b`:

```
const_i64 r0, 50
const_i64 r1, 12
add_i     r2, r0, r1
return_i  r2
```

The compiler does **not** interpret arithmetic in Go. It only chooses opcodes.

---

## 4. Running the bytecode

### In-process Moar (`moar.dll`)

`moargo.RunNative(bytecode)` loads the DLL (next to the exe, `MOAR_DLL`, or `bin/moar.dll`) and calls `MVM_vm_run_bytecode`. Output is Moar’s `say`. CLI:

```powershell
go run ./cmd/tcl -o out.moarvm examples/demo_tcl.tcl
go run ./cmd/tcl -native -dll build/moarvm/bin/moar.dll examples/demo_tcl.tcl
```

Native run requires a fuller CompUnit (SC, code objects). Until those are emitted, treat `-native` as experimental.

---

## 5. Tcl syntax the grammar honors

- **Braces `{...}`**: literal, no `$` / `[]` / escapes.
- **Quotes `"..."`**: compiled interpolations (`concat_s` of literals, vars, nested commands).
- **`$var` / `${var}`**: register load (`set`).
- **`[command ...]`**: nested compile of the inner AST.

---

## 6. Commands

### Compiled to bytecode

`set`, `incr`, `expr` (infix), `puts`, `if` / `elseif` / `else`, `while`, `for`, `list`, `llength`, `lindex`, `return`, `apply`, `yield`, `coroutine` (zero-arg `apply` body).

`apply` compiles the lambda to its own static frame. `coroutine` / `yield` use the same continuation model as MoarVM (`continuationreset` / capture / `continuationinvoke`): yield is one-shot; the next `name` call resumes and the value passed in becomes the result of `yield`. Scripts that mix these with `proc` / FFI still run on the Go fallback (`apply` as a nested scope, coroutines as a goroutine + channels).

### Go builtins (fallback when the script uses them)

`proc`, `foreach`, `switch`, `eval`, `string *`, `lappend` / `lrange` / …, `info`, `break` / `continue`, `cffi::*`, Go FFI, `moar::*`.

---

## 7. C FFI and Go FFI

These stay Go operations (they cannot be honest MoarVM opcodes without a native extop). They force the **fallback** interpreter for that script.

```tcl
set k32 [cffi::load "kernel32.dll"]
set pid [cffi::call $k32 "GetCurrentProcessId" uint {}]
```

```go
_ = tcl.RegisterGoFunc(in, "go_multiply", func(a, b int) int { return a * b })
```

---

## 8. MoarVM bridge (`moar::*`)

Lifecycle of an embedded instance (not the Tcl program’s own bytecode):

```tcl
moar::init
puts [moar::state]
moar::set_prog_name "application.raku"
moar::run "app.moarvm"
moar::destroy
```

---

## 9. CLI

```powershell
go run ./cmd/tcl                          # REPL (Go eval + optional DLL)
go run ./cmd/tcl script.tcl               # parse → bytecode → interpret
go run ./cmd/tcl -emit-only -o x.moarvm script.tcl
go run ./cmd/tcl -native script.tcl
```
