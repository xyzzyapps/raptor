# Tcl & NQP Grammars on MoarVM — Tutorial

## 1. What this frontend is

`moarvm-go/tcl` is a Tcl-shaped language that uses **MoarVM constructs** end to end:

| Stage | Where | What |
| :--- | :--- | :--- |
| Parse | [`tcl/tcl.raku`](tcl/tcl.raku) via **[gcre](../gcre)** | Commands and words only. No evaluation. `gcre` is our Go NQP / QRegex declarative grammar engine. |
| Operations | Go (`tcl/compiler.go`) | Each Tcl command is a Go function that **emits MoarVM opcodes**. |
| Image | `engine.CompUnitEmitter` | CompUnit v7 (`.moarvm`): header, SC, frames, callsites, string heap, bytecode. |
| Run | `moar.dll` via Go (`MVM_vm_run_bytecode`) | Same process as the host; ship the DLL with the binary. |

Commands the compiler cannot emit (`proc`, `foreach`, `string`, C/Go FFI, `moar::*`) still run as Go builtins after a failed compile of the whole script. Nested `eval` inside those builtins never re-enters the bytecode compiler, so scopes stay consistent.

---

## 2. Parse (MoarVM-side grammar)

The grammar is declarative Raku / NQP notation, compiled to combinators by `gcre`:

```raku
grammar Tcl {
    rule TOP { <command_line>* }
    rule command { <word>+ }
    token word {
        || <braced_word> || <quoted_word>
        || <cmd_subst> || <var_subst> || <bare_word>
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

`moargo.RunNative(bytecode)` loads the DLL (next to the exe, `MOAR_DLL`, or `bin/moar.dll`) and calls `MVM_vm_run_bytecode`. Output is Moar’s `say`. Pass `-mod=mod` when running from source (because `vendor/MoarVM` holds C sources, not Go modules):

```powershell
go run -mod=mod ./cmd/tcl -emit-only -o out.moarvm examples/demo_tcl.tcl
go run -mod=mod ./cmd/tcl -native -dll build/moarvm/bin/moar.dll examples/demo_tcl.tcl
```

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
go run -mod=mod ./cmd/tcl                          # REPL (Go eval + optional DLL)
go run -mod=mod ./cmd/tcl script.tcl               # parse → bytecode → interpret
go run -mod=mod ./cmd/tcl -emit-only -o x.moarvm script.tcl
go run -mod=mod ./cmd/tcl -native script.tcl
```

---

## 10. Authoring NQP / QRegex Grammars with `gcre`

When building a new language frontend for `moarvm-go`, write your syntax in an NQP-style `.raku` file and load it via [`gcre`](../gcre/README.md) (see [`../gcre/examples/nqp_lang.raku`](../gcre/examples/nqp_lang.raku)):

```raku
grammar ExprCore {
    token EXPR is optable { <term> }

    proto token term { * }
    token term:sym<int>   { \d+ {*} }
    token term:sym<hex>   { '0x' <xdigit>+ {*} }
    token term:sym<group> { '(' ~ ')' [ <.ws> <EXPR> <.ws> ] {*} }

    proto token prefix { * }
    token prefix:sym<-> is tighter<*> { <sym> }

    proto token infix { * }
    token infix:sym<+>  { <sym> }
    token infix:sym<*>  is tighter<+> { <sym> }
    token infix:sym<**> is tighter<*> is assoc<right> { <sym> }
}

grammar MyLang is ExprCore {
    rule TOP { ^ <stmt>+ % ';' ';'? $ }

    proto rule stmt { * }
    rule stmt:sym<let>   { <sym> <lhs=ident> '=' <rhs=EXPR> {*} }
    rule stmt:sym<print> { <sym> <arg=EXPR> {*} }
}
```

### Key NQP Constructs Available in `gcre`

- **Grammar inheritance (`grammar Child is Parent`)**: Inherits rules, `proto` multi-dispatch groups, and `is optable` operators; child `:sym<...>` rules extend inherited protoregexes.
- **Protoregexes & `:sym<...>`**: `proto token|rule|regex name { * }` dispatches across all `name:sym<...>` candidates using Longest-Token Matching (LTM), binding `<sym>` and recording `m.Rule` (e.g., `"stmt:sym<let>"`).
- **LTM (`|`) vs Sequential (`||`)**: `|` picks the longest match across declarative branches; `||` picks the first matching branch left-to-right (`|` binds tighter than `||`).
- **Declarative Operator Tables (`is optable`)**: `token EXPR is optable { <term> }` builds a Pratt precedence parser from `prefix:sym<...>`, `infix:sym<...>`, and `postfix:sym<...>` rules annotated with `is tighter<...>`, `is looser<...>`, `is equiv<...>`, and `is assoc<left|right|non|chain>`.
- **Quantifiers, Separators & Goals**: Counted `** N..M`, frugal `*?` / `+?` / `??`, separated lists `%` / `%%`, and goal matching `'(' ~ ')' <EXPR>` with structured goal error diagnostics.
- **Lookarounds & Character Algebra**: `<?before ...>`, `<!before ...>`, `<?after ...>`, `<!after ...>`, anchors (`^`, `$`, `^^`, `$$`, `<<`, `>>`, `<?wb>`, `<!ww>`), and set algebra (`<[a..z] + :Digit - [aeiou]>`).

### Reducing Matches to ASTs in Go

```go
type Actions struct{}

func (a *Actions) HandleAction(rule string, m *gcre.Match) {
    switch rule {
    case "term:sym<int>":
        n, _ := strconv.ParseInt(m.Str, 10, 64)
        m.Make(n)
    }
}

g, err := gcre.LoadGrammarFromFile("mylang.raku")
match, err := g.Parse("let x = 2 + 3 * 4;", &Actions{})
```

---

## 11. Comparison with Upstream Raku NQP

| Layer | Upstream Raku NQP (`Rakudo/nqp`) | `gcre` + `moarvm-go` |
| :--- | :--- | :--- |
| **Host & Bootstrap** | Self-hosted NQP → QAST → MAST → MoarVM | Pure Go host (`gcre` + `moargo.CompUnitEmitter`) → `moar.dll` |
| **QRegex Surface** | `grammar`/`is`, `proto`/`:sym`, `\|`/`\|\|`, `EXPR` `%optable`, `%`/`%%`, `~`, lookarounds, `<[...]>` set algebra | Supported declaratively in `.raku` files |
| **LTM Engine** | Compiled Thompson NFA state table for declarative prefixes | Direct multi-branch longest-match probing from cursor with capture rollback |
| **Regex Backtracking** | Full `bstack` continuation cursor in `regex`; `:ratchet` in `token`/`rule` | `:ratchet` semantics with frugal lookahead (`*?`/`+?`/`??`) and branch snapshot/restore |
| **Semantic Actions & Codegen** | NQP action classes + `QAST` AST nodes + ~400 `nqp::*` ops | Go `ActionHandler`/struct methods + `<HOST_name>` hatch + direct CompUnit v7 / 6Model emission |
| **Omitted Inline Regex Code** | `{ code }`, `<?{ pred }>`, `:my($x)`, `<$rx>`, `:i`/`:m`, `<( )>`, `&` | Omitted so `.raku` grammars stay pure, declarative, and language-agnostic |
