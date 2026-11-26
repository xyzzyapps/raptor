# Raku & Perl 5 Procedural Subset Compatibility in Raptor

This document answers the question: ***"If a Raku user (or Perl 5 user) writes code from the shared procedural Perl 5 / Raku subset in Raptor, how much works out-of-the-box, and what are the exact differences?"***

---

## 1. What Works 100% Identically

Raptor is specifically designed so that procedural Raku and Perl 5 code runs cleanly with minimal or zero translation:

| Category | Supported Constructs in Raptor |
| :--- | :--- |
| **Lexical & Package Variables** | `my $x = 1;`, `our $pkg_var = 2;`, `state $counter = 0;`, `my ($a, $b, @rest) = ...;`, `($a, $b) = ($b, $a);` |
| **Sigils & Indexing** | Raku sigil-invariance (`@arr[0]`, `%hash<key>`, `%hash{"key"}`) **and** Perl 5 scalar-sigil element indexing (`$arr[0]`, `$hash{"key"}`) plus reference arrows (`$ref->[0]`, `$ref->{"k"}`, `$ref->($arg)`). |
| **Operators** | `+`, `-`, `*`, `/`, `div`, `%`, `mod`, `**`, `%%`, `++`, `--` (prefix & postfix, including `%freq{$w}++` auto-vivification), `~` / `~=`, `x`, `xx`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `<=>`, `eq`, `ne`, `lt`, `gt`, `le`, `ge`, `cmp`, `&&`/`and`, `||`/`or`, `xor`, `!`/`not`, `//`/`orelse`, `//=`, `?? !!` **and** `? :`, `+&`, `+|`, `+^`, `+<`, `+>`, `~~`, `=~`, `!~`. |
| **Ranges** | Inclusive `1..10`, exclusive `0..^10`, `0^..10`, `0^..^10`, and prefix `^10` (`0..9`). |
| **Control Flow** | `if / elsif / else`, `unless`, `while`, `until`, `given / when / default`, statement modifiers (`say $_ for @a;`, `return if $done;`), `last`, `next`, `redo`, labels & `goto LABEL` / `goto &sub`. |
| **All Loop Styles** | Raku `for @list -> $x { }`, multi-variable pointy `for @kv -> $k, $v { }`, topic `for @list { say $_ }`, `loop (my $i = 0; $i < 10; $i++) { }`, `loop { }`, Perl 5 `foreach my $x (@list) { }`, and C-style `for (my $i = 0; $i < 10; $i++) { }`. |
| **Subroutines & Dispatch** | `sub f($a, $b = 10, *@rest) { }`, Perl 5 `@_` argument array (`sub f { ... @_ ... }`), anonymous `sub ($x) { }`, pointy blocks `-> $x { }`, `multi sub` typed & `where`-constrained multiple dispatch, `subset`, `PRE`/`POST` contracts, `before`/`after`/`around` advice hooks, and `AUTOLOAD`. |
| **Quoting & Strings** | `'raw'`, `"interpolated $x @a[0] %h<k> {$x + 1}"`, `q[...]`, `qq[...]`, `qw(a b c)`, Raku `<word list>`, heredocs `<<EOF` / `<<'EOF'` / `<<~EOF`, backticks `` `cmd` `` and `qx{cmd}`. |
| **Error Handling & Eval** | `die("msg")`, `warn("msg")`, `try { ... }` (populating `$!` and returning `Nil` on failure, with optional `catch { ... }`), `eval($code)` / `EVAL($code)`. |
| **Grammars & Regexes** | Declarative Raku PEG + LTM `grammar Name [is Parent] { rule TOP { ... } token t { ... } }` with `:i`, `:m`, `:s`/`:!s`, `|` (LTM) vs `||` (ordered PEG), `&` conjunction, `is optable`, `proto`/`:sym<...>`, `<( ... )>` capture markers, `::`/`:::` cuts, `<~~>` self-recursion, `$0`/`$<name>` backreferences, `make($ast)` action tables, `$/`, and `$<name>` capture variables; plus `=~ /pat/` setting `$1, $2...` and `$/`. |
| **Concurrency & Generator** | Raku `gather { take $x }`, `start { ... }`, `await`, `Promise`, `Channel`, and data-parallel `pmap`, `psum`, `psort`. |

---

## 2. Intentional Syntactic & Semantic Differences

When porting arbitrary Raku or Perl 5 code to Raptor, keep these intentional design differences in mind:

### A. Differences vs. Perl 5
1. **String Concatenation vs. Method Calls (`~` vs `.`)**:
   - Like Raku, Raptor uses **`~`** (and `~=`) for string concatenation and **`.`** (or `->`) for method calls / UFCS (`$str.uc()`, `@arr.sort()`). Perl 5's `$a . $b` must be written `$a ~ $b`.
2. **Bitwise Operators Use Raku `+` Prefix**:
   - Numeric bitwise operators follow Raku syntax: `+&` (AND), `+|` (OR), `+^` (XOR), `+<` (left shift), `+>` (right shift), freeing `^` for prefix ranges (`^10`) and `&` for code references (`&fn`).
3. **Regex Substitution & Matching**:
   - Perl 5 `s/pat/repl/g` and `tr/a-z/A-Z/` are expressed cleanly as function/UFCS calls: `$s = subst($s, "pat", "repl")` (or `replace($s, $old, $new)`) rather than mutating `s///` syntax.
4. **No `bless` OOP (`class` / `@ISA`)**:
   - Raptor is **procedural by design** (no OOP class hierarchy or `bless`). Objects/records are idiomatic hashes, closures, `package` namespaces, `multi sub` polymorphic functions, or zero-overhead C-ABI `struct` / `union` layouts.
5. **List Flattening**:
   - Like Raku (and unlike Perl 5), `[1, [2, 3]]` preserves nested arrays rather than auto-flattening; call `flat(@nested)` when Perl 5 list flattening is desired.

### B. Differences vs. Full Raku
1. **Procedural Subset Only (No `class`, `role`, `has $.attr`, `method`)**:
   - Full Raku's MOP/6Model `class` and `role` declarations are intentionally omitted in favor of `sub` / `multi sub` + UFCS (`$val.my_sub()` resolves to `my_sub($val)`!), `struct`, and `subset` / `where` predicates. (Note: inside `grammar`, `rule`, `token`, and `regex` ARE fully supported.)
2. **PEG Ratcheting Semantics by Default in Grammars**:
   - In Raptor's `gcre` grammar engine, rules and tokens follow **deterministic PEG (ratcheting)** semantics + NQP **Longest-Token Matching (`|`)** rather than backtracking NFA across sequence boundaries, matching Raku's `rule` / `token` behavior (the idiomatic fast subset of Raku grammars).
3. **Inline Code Blocks Inside Grammar Rules**:
   - Instead of embedding `{ make ... }` inline inside regex tokens, pass an action hash (`MyGrammar.parse($input, %actions)`) whose closures call `make($val)` and read `$m<subrule><made>` / `$m.ast`, mirroring Raku's clean `Actions` class separation.
4. **Numeric Division**:
   - `/` performs IEEE-754 `float64` (`Num`) division rather than exact `Rat` rational arithmetic; `div` performs exact `int64` integer division.
5. **Hyper / Meta Operators**:
   - Raku hyperoperators (`>>+<<`, `Z+`, `X~`) are written with explicit functional/SIMD primitives (`simd_add(@a, @b)`, `zip(@a, @b)`, `map`, `pmap`, `reduce`).
