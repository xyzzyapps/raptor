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
    rule TOP { <statement>* <HOST_legacy_rest>? }
    rule statement { <var_decl> | <if_stmt> | ... }
}
```

```go
gcre.RegisterHost("legacy_rest", func(g *gcre.Grammar, ctx *gcre.Context, cap *gcre.Match) bool {
    // parse leftover input; advance ctx.Pos; optionally cap.Make(ast)
    return true
})
```

That is still PEG (ordered choice) and still valid Raku angle syntax
(`<HOST_legacy_rest>`). Pigeon does not need action blocks. The host
supplies LTM-like or Pratt parsing only where the grammar names it.
