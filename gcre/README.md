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
Hosts that need syntax outside that subset (Raptor) must keep a
non-Pigeon escape hatch rather than extending `raku.peg` in ways Pigeon
cannot generate.
