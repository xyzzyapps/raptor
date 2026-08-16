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
