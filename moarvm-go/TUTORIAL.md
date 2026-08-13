# Tcl Language Frontend on MoarVM - Architecture & Tutorial (tutorial.md)

## 1. Overview

`moarvm-go/tcl` provides a reference dynamic language frontend for **Tcl** (Tool Command Language). It demonstrates how to build a complete, production-grade dynamic language on top of `moarvm-go` by:
1. Defining the language syntax **100% declaratively** in a `.raku` grammar file (`tcl/tcl.raku`).
2. Executing parsing via the `moarvm-go/grammar` engine.
3. Implementing strict standard Tcl semantics (The Dodekalogue).
4. Providing dynamic **C FFI** and reflection-based **Go FFI**.
5. Emitting native **MoarVM CompUnit v7 bytecode**.

---

## 2. Declarative Grammar (`tcl/tcl.raku`)

The Tcl grammar is written in Perl 6 / Raku grammar notation:

```raku
grammar Tcl {
    rule TOP {
        <command_line>*
    }

    rule command_line {
        | <comment>
        | <command>
    }

    token comment {
        '#' <-[\n]>*
    }

    rule command {
        <word>+
    }

    token word {
        | <braced_word>
        | <quoted_word>
        | <cmd_subst>
        | <var_subst>
        | <bare_word>
    }

    token braced_word {
        '{' <braced_content>* '}'
    }

    token quoted_word {
        '"' <quoted_content>* '"'
    }

    token cmd_subst {
        '[' <command_line>* ']'
    }

    token var_subst {
        '$' [<ident> | '{' <ident> '}' | <ident> '(' <ident> ')']
    }

    token bare_word {
        <-[\s;\[\]\{\}"\$]>+
    }
}
```

The grammar is embedded directly into the Go package via `//go:embed tcl.raku` and compiled into executable parser combinators.

---

## 3. Strict Tcl Syntax & Semantics

The implementation strictly honors standard Tcl rules:

### 3.1 Braces vs Quotes
- **Braces `{...}`**: Verbatim literal strings. No variable expansions, command substitutions, or escape sequences are performed inside braces:
  ```tcl
  set raw {Hello $name [expr 2 + 2]}
  # Result: "Hello $name [expr 2 + 2]"
  ```
- **Quotes `"..."`**: Interpolated strings. Variable substitutions (`$var`), command substitutions (`[...]`), and backslash escapes (`\n`, `\t`, etc.) are evaluated:
  ```tcl
  set name "World"
  set msg "Hello $name [expr 2 + 2]"
  # Result: "Hello World 4"
  ```

### 3.2 Substitutions
- **Variables**: `$var`, `${var}`, and array indices `$arr(key)`.
- **Command Substitutions**: `[command arg1 arg2]` runs the inner command and substitutes its output.
- **Backslashes**: `\n`, `\t`, `\r`, `\"`, `\$`, `\[`, `\{`, `\\`.

---

## 4. Standard Command Reference

### Variables & Introspection
- `set varName ?value?`: Read or write a variable.
- `unset varName ...`: Delete one or more variables.
- `incr varName ?increment?`: Increment an integer variable (default +1).
- `append varName ?value ...?`: Append strings to a variable.
- `global varName ...`: Declare variables as global within a procedure.
- `info exists varName`: Check if a variable exists (returns `1` or `0`).
- `info vars`: List all active variables.
- `info procs`: List all defined procedures.

### Control Flow
- `if {cond} {body} elseif {cond} {body} else {body}`: Conditional execution.
- `while {cond} {body}`: While loop with `break` and `continue` support.
- `for {init} {test} {step} {body}`: For loop with `break` and `continue`.
- `foreach varName list {body}`: Iterate over list items.
- `switch $value { pattern body ... }`: Pattern matching with `default`.
- `proc name args body`: Define a user procedure.
- `return ?value?`: Return a value from a procedure.
- `eval script ...`: Dynamically evaluate a script string.

### String Manipulation
- `string length string`: Return length of string.
- `string index string index`: Get character at index.
- `string range string first last`: Extract substring range (supports `end`).
- `string tolower string`: Convert to lowercase.
- `string toupper string`: Convert to uppercase.
- `string trim string ?chars?`: Trim whitespace or specific characters.
- `string compare s1 s2`: Lexicographical comparison (-1, 0, 1).
- `string equal s1 s2`: Exact equality check (`1` or `0`).
- `string match pattern string`: Glob pattern matching (`*`, `?`).

### List Manipulation
- `list ?arg ...?`: Create a list from arguments.
- `llength list`: Count items in list.
- `lindex list index`: Get item at index.
- `lappend listVar ?val ...?`: Append items to a list variable.
- `lrange list first last`: Extract sub-list range.
- `linsert list index element ...`: Insert elements into list.
- `lreplace list first last element ...`: Replace element range in list.
- `lsearch list pattern`: Search list for glob pattern (returns index or `-1`).
- `join list ?separator?`: Join list elements with separator.
- `split string ?splitChars?`: Split string into list.

### Math Expressions (`expr`)
- Infix operators: `+`, `-`, `*`, `/`, `%`, `**`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `&&`, `||`.
- Math functions: `abs(x)`, `sqrt(x)`, `sin(x)`, `cos(x)`, `tan(x)`, `pow(x, y)`, `ceil(x)`, `floor(x)`, `round(x)`.

---

## 5. C FFI (Foreign Function Interface)

Load and invoke native C dynamic libraries (`.dll` / `.so`) directly in Tcl:

```tcl
# 1. Load dynamic library
set k32 [cffi::load "kernel32.dll"]

# 2. Direct function call
set pid [cffi::call $k32 "GetCurrentProcessId" uint {}]
puts "Current PID: $pid"

# 3. Dynamic function binding to a new Tcl command
set moar [cffi::load "moar.dll"]
cffi::bind $moar "MVM_vm_create_instance" ptr {} mvm_create
cffi::bind $moar "MVM_vm_destroy_instance" void {ptr} mvm_destroy

# Invoke bound native commands
set vm [mvm_create]
puts "Created VM instance at: $vm"
mvm_destroy $vm
```

---

## 6. Go FFI (Reflection Function Binding)

Bind arbitrary Go functions directly into the Tcl interpreter:

```go
package main

import (
    "fmt"
    "moarvm-go/tcl"
)

func main() {
    in := tcl.NewInterp()

    // Register Go function with arbitrary arguments and return types
    _ = tcl.RegisterGoFunc(in, "go_multiply", func(a, b int) int {
        return a * b
    })

    _ = tcl.RegisterGoFunc(in, "go_repeat", func(text string, count int) string {
        var res string
        for i := 0; i < count; i++ {
            res += text
        }
        return res
    })

    out, _ := in.Eval("go_multiply 6 7")
    fmt.Println("Result:", out) // Result: 42

    strOut, _ := in.Eval("go_repeat {abc } 3")
    fmt.Println("Repeated:", strOut) // Repeated: abc abc abc 
}
```

---

## 7. MoarVM Bridge

Control the native MoarVM execution engine directly from Tcl:

```tcl
# Initialize the MoarVM subsystem
moar::init

# Inspect VM state
puts "MoarVM State: [moar::state]"

# Configure program name and arguments
moar::set_prog_name "application.raku"
moar::set_args {--verbose --opt-level=2}

# Execute a compiled bytecode file
moar::run "app.moarvm"

# Cleanly destroy the VM
moar::destroy
```

---

## 8. Running Scripts & REPL

```powershell
# Interactive REPL
go run ./cmd/tcl

# Execute script file
go run ./cmd/tcl examples/demo_tcl.tcl

# Execute C FFI script
go run ./cmd/tcl examples/test_ffi.tcl

# Execute MoarVM bridge script
go run ./cmd/tcl examples/test_bridge.tcl
```
