# gcre — Grammar Compatible Regular Expressions

**gcre** is a Go library for **grammar-compatible regular expressions**: a
**subset of Raku** (Perl 6) grammar/regex notation that is **PEG-compatible**.

You write a `.raku` file. You do not write Pigeon `.peg` or embed code in the
grammar. The subset is spelled so every construct is valid Raku *and* has a
1-to-1 PEG reading:

| PEG | Raku (gcre) |
| :--- | :--- |
| `name <- e` | `token name { e }` (or `rule` / `regex`) |
| `e1 / e2` | `\| e1 \| e2` |
| `e1 e2` | `e1 e2` |
| `e* e+ e?` | same |
| `[ e ]` | `[ e ]` |
| `'x'` / `"x"` | same |
| `.` | `.` |
| `[a-z]` | `<[a..z]>` |
| `[^xyz]` | `<-[xyz]>` |
| call `A` | `<A>` / `<.A>` |

Not in this subset: action blocks, protoregexes, LTM, `&e`/`!e` as PEG
predicates, `{n,m}`. Those are either full Raku or extra PEG, not both.

## Use

```go
import "gcre"

g, err := gcre.LoadGrammarFromString(src) // or LoadGrammarFromFile("lang.raku")
m, err := g.Parse(input, nil)
```

Examples (not Tcl):

- [`examples/minicalc.raku`](examples/minicalc.raku)
- [`examples/tinyjson.raku`](examples/tinyjson.raku)

Regenerate the `.raku` *file* parser (maintainers only):

```
go generate
```

That runs Pigeon on `raku.peg`. Language authors still only edit `.raku`.

The `.raku` subset stays **1-to-1 with Pigeon PEG** (no action blocks, no LTM).

### Organic host hatch (`<HOST_name>`)

A unique gcre feature: a subrule whose name starts with `HOST_` is **not**
a grammar rule. It is a hook the Go host registers:

```raku
grammar Raptor {
    rule TOP { <statement>* }
    rule statement { <comment> | <HOST_stmt> | <var_decl> | ... }
    rule expression { <assign_expr> | <HOST_expr> }
}
```

```go
gcre.RegisterHost("stmt", parseOneStatement)
gcre.RegisterHost("expr", parseOneExpression)
```

That is still PEG (ordered choice) and still valid Raku angle syntax
(`<HOST_stmt>`). Pigeon does not need action blocks. The host supplies
Pratt parsing only where the grammar names it. There is no second
full-file parser pass.
