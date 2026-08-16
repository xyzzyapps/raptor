# Raptor

[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](raptor/t/)
[![License: Artistic-2.0](https://img.shields.io/badge/License-Artistic_2.0-0298c3.svg)](https://opensource.org/licenses/Artistic-2.0)

A **Perl 5–shaped** language (the dynamic, non-OO subset of Raku): sigils, `$_`, `//`, UFCS, `struct`, `subset`, no `class` / `has`. Go host, optional **MoarVM** backend.

If you know Perl 5, you already know Raptor. Where Perl 5 and Raku disagree, Raptor uses the **Raku spelling**.

**Implemented**

| Perl 5 | Raptor |
| :--- | :--- |
| `my $x`, `my @a`, `my %h` | same sigils |
| `our $x` / `state $x` | same |
| `$_` topic | same (`for`, `given`, bare `say` / `print`) |
| `@ARGV` `%ENV` | `@*ARGS` / `@ARGV`, `%*ENV` / `%ENV` |
| `$$` `$0` `$?` `$!` | same; also `$*PID` `$*PROGRAM` `$*OS` |
| `defined` | `defined($x)`; `Nil` is the undefined value |
| `.` concat | `~` (`~=` too) |
| `x` string repeat | same; `xx` repeats lists |
| `//` `//=` | same |
| `eq ne lt gt le ge` | same; `== != < >` are numeric |
| `<=>` / `cmp` | same |
| `=~` `!~` | same; also smartmatch `~~` |
| `? :` | `?? !!` (also `?:`) |
| `& \| ^ << >>` bitwise | `+&` `+\|` `+^` `+<` `+>` |
| `**` `&&` `\|\|` `and` `or` `not` | same |
| `1 .. 10` | same |
| `1 < $x < 10` | same (chained) |
| `if` `elsif` `else` `unless` | same |
| `while` `until` | same |
| `for` / `foreach (@xs)` | `for @xs { }` or `for @xs -> $x { }` |
| C-style `for (;;)` | `loop (my $i = 0; $i < n; $i++) { }` |
| `say 1 if $ok` | same modifiers (`if` `unless` `while` `until` `for` `given`) |
| `last` / `next` / `redo` | same |
| `given` / `when` / `default` | same (`when` uses `~~`) |
| `goto LABEL` / `goto &sub` | same |
| `sub foo { my $a = shift; }` | `sub foo($a) { }` signatures |
| `@_` | named / slurpy params (`$head, @tail`) |
| `$obj->method($a)` | `$obj.method($a)` **UFCS** (any `sub method($obj, …)`) |
| `sub { }` closures | same |
| `AUTOLOAD` | same (`$AUTOLOAD`) |
| records / methods | `struct` + `multi sub` (UFCS) |
| `@a = (1, 2, 3)` | `@a = [1, 2, 3]` |
| `%h = (k => 1)` | `%h = { "k" => 1 }` |
| `$a[0]` `$h{k}` | `$a[0]` `$h{"k"}` |
| `push pop shift unshift splice` | same |
| `keys` `values` `map` `grep` `sort` `join` `split` `reverse` | same (+ UFCS: `@a.elems()`, `@a.map(...)`) |
| `exists $h{$k}` / `delete $h{$k}` | `exists(%h, $k)` / `delete(%h, $k)` |
| `$#a` | `@a.elems() - 1` |
| `\$s` `\@a` `\%h` `\&f` / `->` | same; `$a->[0]` `$h->{"k"}`; `ref` / `is_ref` |
| `"$x"` / `'lit'` | same interpolation |
| `<<EOF` | same; `<<~EOF` strips indent |
| `length` `uc` `lc` `substr` `index` `ord` `chr` `sprintf` | same; also `chars` `trim` `tc` `fc` |
| `chomp` / `chop` | return a new string (do not mutate) |
| `qw(a b c)` | `[ "a", "b", "c" ]` |
| `m//` | `=~` / `~~`; `regex_engine("samre")` |
| `open my $fh, "<", $p` | `$fh = open($p, "r")` (`<` `>` `>>` ok) |
| `<>` / `readline` | `readline($fh)`; `slurp` / `spurt` for whole files |
| `-e -f -d -s -r -w` | same |
| `` `cmd` `` / `qx` / `system` | same (`$?` / `$!`) |
| `die` `warn` `exit` | same |
| `chdir` `mkdir` `unlink` `rename` | same |
| `package Foo;` | same; `%Foo::` stash |
| `use Foo;` / `require` | same (`lib/`, `raptor_modules/`, `@*INC`) |
| TAP `ok` / `is` / `prove` | built in; `raptor test t/` |
| XS / Inline::C | `is native('lib.dll')` NativeCall |
| `threads` | `start` / `await` / `Channel` |
| CGI / PSGI | RaptorHP `.phtml` — `raptor -S` (like `php -S`) |

**Not implemented:** `local`, `eval STRING` / `eval { }`, `bless` / `@ISA` / Moose / `class` / `has`, prototypes, `wantarray` (no list/scalar context), `s///` / `tr///`.

Raptor is also engineered for the Post-LLM paradigm:

1. **High Token Density & Brief Syntax**: By adopting the concise syntax of Perl 5 and modern Raku (variables using `$` `@` `%` sigils, canonical `Nil`, built-in operators) while completely omitting `class`, `has`, and `is` boilerplate, Raptor maximizes LLM context window efficiency and reduces model hallucinations.
2. **Verification-First by Default**: LLMs generate code rapidly, but need tight, verifiable guardrails. Raptor builds formal Design-by-Contract (`PRE`, `POST`, `INVARIANT`), inline testing (`TEST`), and randomized property-based quickcheck fuzzing (`PROPERTY`) directly into the core grammar.
3. **Continuous Invariant Checking on Every Assignment**: Unlike static types checked once at compile time, every variable in Raptor can hold a dynamic `where` predicate that is rigorously evaluated upon **every mutation and assignment**. If an LLM or runtime mutation produces an invalid state, it is caught immediately at the point of failure.
4. **Uniform Function Call Syntax (UFCS)**: Functional method chaining (`$val.func().transform()`) allows LLMs to construct clean data transformation pipelines without wrapping primitives in artificial classes.
5. **C-ABI Struct Memory & Closures**: Contiguous C-compatible memory layout records (`struct Vector2 { num64 $x; num64 $y; }`) with first-class closure fields provide native C performance without OOP overhead.
6. **Literate Programming with PodLit**: Knuth-style chunk weaving, source tangling, and reverse-stitching allow LLMs to maintain human-readable architectural specifications and executable source code side-by-side.

## Academic & Theoretical Foundations

Raptor's predicate type, continuous contract, and literate programming systems are grounded in seminal computer science research:

- **Robert Bruce Findler (PhD Thesis, Rice University, 2002)**: *Behavioral Software Contracts* (supervised by Matthias Felleisen). Established the mathematical and runtime foundations of higher-order dynamic contracts, continuous invariant monitoring on mutable reference cells, and assignment-level contract enforcement.
- **Predicate Dispatching**: A Unified Theory of Dispatch* (with Craig Kaplan & Craig Chambers). Introduced the formal model for evaluating arbitrary user-defined boolean predicates over variables and parameter values to drive runtime dispatch and enforce state invariants.
- **Patrick Maxim Rondon (PhD Thesis, UC San Diego, 2012)**: *Liquid Types* (supervised by Ranjit Jhala, built on Frank Pfenning's 1991 *Refinement Types for ML*). Refinement types via predicate subtyping where standard base types are enriched with logical predicates evaluated against values and assignments.
- **Johan Hidding (Netherlands eScience Center, 2023)**: *Entangled, a Bidirectional System for Sustainable Literate Programming*, 2023 IEEE 19th International Conference on e-Science (e-Science), pp. 1-10, [DOI: 10.1109/e-Science58273.2023.10254816](https://doi.org/10.1109/e-Science58273.2023.10254816). Formulated the model and grammar for bidirectional, round-trip literate programming where external code modifications are synchronized and reverse-stitched back into literate narrative documents without disturbing narrative prose.


```
cd raptor
go build -mod=mod -o bin/raptor.exe ./cmd/raptor
raptor script.rp          # --go interpreter
raptor --moar script.rp   # CompUnit v7 on moar.dll
raptor test t/
raptor serve              # WASM tour :8080
raptor -S localhost:8000  # PHP-style RaptorHP
```

Full language, CLI, FFI, WASM, and build notes: **[raptor/README.md](raptor/README.md)**.

This tree is three siblings:

| Path | What |
| :--- | :--- |
| **[raptor/](raptor/README.md)** | Language, runtime, TAP, PodLit, WASM tour, RaptorHP, pack |
| **[gcre/](gcre/README.md)** | Grammar Compatible Regular Expressions (Raku ⊂ PEG; `<HOST_name>`) |
| **[moarvm-go/](moarvm-go/README.md)** | MoarVM embed + Tcl frontend (same CompUnit v7 family) |

Parse is **gcre** (`raptor/runtime/raptor.raku`, `moarvm-go/tcl/tcl.raku`). Pratt runs only where the grammar writes `<HOST_stmt>` / `<HOST_expr>`.

Licensed under the **Artistic License 2.0**.

The first prototype was written by **Gemini**. The language was mostly written by Anti Gravity — Gemini 3.6. Library bindings and PodLit were written by Gemini 3.7. Later work was modified by **Grok**.
