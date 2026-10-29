# gcre — Grammar Compatible Regular Expressions (NQP / QRegex + PEG Engine)

**gcre** is a pure-Go declarative grammar and regular-expression engine implementing the **NQP (Not Quite Perl / QRegex)** and **Raku** grammar subset used by `raptor` and `moarvm-go`.

Authors write `.raku` grammar files with zero embedded target-language action code; semantic actions live cleanly in Go structs or `ActionHandler` implementations.

## Supported NQP / Raku Constructs

| Category | Syntax in `.raku` | Semantics |
| :--- | :--- | :--- |
| **Grammars & Inheritance** | `grammar Child is Parent { ... }` | Inherits rules, `proto` multi-dispatch tables, and `is optable` operator tables; late-bound subrule dispatch |
| **Rule Kinds** | `token`, `rule`, `regex` | `token`/`regex` match exact characters (`:ratchet`); `rule` skips inter-atom whitespace (`:sigspace`) |
| **Protoregexes & `:sym`** | `proto token op { * }`<br>`token op:sym<+> { <sym> }`<br>`token op:sym«>=» { <sym> }`<br>`token op:sym[=] { <sym> }` | Longest-Token Matching (LTM) across all `:sym` / `multi` candidates; binds `<sym>` / `<.sym>`, sets `m.Rule` and `m.Get("sym")` |
| **LTM vs Sequential Alt** | `a \| ab` vs `a \|\| ab` | `\|` performs LTM across declarative branches (and all branches in `is ltm` rules); `\|\|` performs left-to-right first-match (`\|` binds tighter than `\|\|`) |
| **Declarative Optables** | `token EXPR is optable { <term> }`<br>`is tighter<+>` / `is looser<*>` / `is equiv<->`<br>`is assoc<left\|right\|non\|chain>` | Builds a Pratt / precedence-climbing `OpTable` from `prefix:sym<...>`, `infix:sym<...>`, and `postfix:sym<...>` declarations |
| **Quantifiers** | `*`, `+`, `?`, `** N`, `** N..M`, `** N..*` | Greedy `:ratchet` repetition with rollback of partial captures |
| **Frugal Quantifiers** | `*?`, `+?`, `??`, `** N..M?` | Minimal repetition advancing until the remainder of the sequence succeeds |
| **Separated Lists** | `<item>+ % ','`<br>`<item>+ %% ','` | `%` matches separated lists (no trailing separator); `%%` allows an optional trailing separator |
| **Goal Matching** | `'(' ~ ')' <EXPR>` | Matches opener, inner atom, and closing goal; records a structured goal diagnostic on `Context.GoalErr` if the goal fails |
| **Captures & Aliases** | `<subrule>`, `<.subrule>`<br>`$<lhs>=<expr>`, `<lhs=expr>`<br>`( ... )`, `[ ... ]` | Named captures (`m.Get`, `m.GetAll`), silent subrules (`<.foo>`), named aliases, positional captures (`m.At(0)`), non-capturing groups |
| **Lookarounds** | `<?before pat>`, `<!before pat>`<br>`<?after pat>`, `<!after pat>`<br>`<?rule>`, `<!rule>` | Zero-width positive and negative lookahead and lookbehind assertions |
| **Anchors & Boundaries** | `^`, `$`, `^^`, `$$`<br>`<<` / `«`, `>>` / `»`<br>`<?wb>`, `<!wb>`, `<?ww>`, `<!ww>` | BOS, EOS, BOL, EOL, left/right word boundaries, word-boundary and within-word assertions |
| **CharClass Set Algebra** | `<[a..z]>`, `<-[0..9]>`<br>`<[a..z] + :Digit - [aeiou]>` | Bracketed ranges, hex endpoints `\x[20]..\x[7e]`, union `+`, difference `-`, negation `<-...>` |
| **Unicode Properties** | `<:Lu>`, `<:Ll>`, `<:L>`, `<:Digit>`, `<:N>`, `<:Space>`, `<:Punct>`, `<:Hex>`, `<:!Digit>` | Unicode general categories and POSIX/Raku character properties |
| **Regex Escapes** | `\d \D \s \S \w \W \h \H \v \V \n \N \t \T \r \R \x[NN]` | Positive and uppercase-negated character classes plus hex codepoints |
| **Built-in NQP Subrules** | `<ident>`, `<ws>`, `<alpha>`, `<digit>`, `<alnum>`, `<xdigit>`, `<space>`, `<blank>`, `<wb>`, `<ww>` | Built-in QRegex rules when not overridden by the grammar (`<ws>` implements NQP `<!ww> \s*`) |
| **Action Stubs** | `{*}` | NQP action-dispatch marker (no-op in `.raku`, dispatched via Go `Actions` / `ActionHandler`) |
| **Organic Host Hatch** | `<HOST_name>` / `<.HOST_name>` | Invokes a registered Go `HostFunc` (`RegisterHost("name", fn)`) at the current rune cursor |

## Go API

```go
import "gcre"

g, err := gcre.LoadGrammarFromString(src) // or LoadGrammarFromFile("lang.raku")
m, err := g.Parse(input, actions)         // or g.ParseRule("expr", input, actions)
```

- Multi-grammar & inheritance loading:
  - `LoadGrammarFromString(src)` compiles all grammars in `src` and returns the last grammar.
  - `LoadGrammarsFromString(src)` returns `(map[string]*Grammar, *Grammar, error)`.
  - `LoadGrammarWithParents(src, parents)` resolves `grammar Child is Parent` against pre-compiled parent grammars.
  - `g.Extend(parent)` programmatically inherits rules, protoregexes, and optable operators.
- Action dispatch:
  - Implement `ActionHandler` (`HandleAction(ruleName string, m *Match)`) to receive exact `:sym<...>` rule names (e.g. `"infix:sym<+>"`), or pass any Go struct with exported methods matching rule names.

## Formal Meta-Grammar (`raku.peg`)

The complete meta-grammar accepted by `gcre` is specified in [`raku.peg`](raku.peg):

```peg
GrammarFile    <- _ GrammarDecl (_ GrammarDecl)* _ EOF
GrammarDecl    <- "grammar" !NameCont _ NameTok ParentSpec? _ "{" _ GrammarMember* _ "}"
ParentSpec     <- _ "is" !NameCont _ NameTok
GrammarMember  <- ProtoDecl / RuleDecl / ";" _
ProtoDecl      <- "proto" !NameCont _ RuleKind _ NameTok RuleTrait* _ "{" _ ProtoBody _ "}" _
ProtoBody      <- "{" _ "*" _ "}" / "*" / "<...>" / "..." / ""
RuleDecl       <- ("multi" !NameCont _)? RuleKind _ RuleName RuleTrait* _ "{" _ RxAlt? _ "}" _
RuleKind       <- ("token" / "regex" / "rule") !NameCont
RuleName       <- NameTok (":sym" (SymAngle / SymFrench / SymTexas / SymBracket))?
RuleTrait      <- _ "is" !NameCont _ NameTok TraitArg?
RxAlt          <- (_ "||")? _ RxLTMAlt (_ "||" _ RxLTMAlt)*
RxLTMAlt       <- (_ "|" !"|")? _ RxSeq (_ "|" !"|" _ RxSeq)*
RxSeq          <- RxItem (_ RxItem)*
RxItem         <- NamedCap / PlainAtom (_ "~" _ PlainAtom _ RxItem)?
NamedCap       <- "$<" NameTok ">" _ "=" _ PlainAtom
PlainAtom      <- RxPrimary QuantSpec? (_ ("%%" / "%") _ RxPrimary QuantSpec?)?
QuantSpec      <- _ "**" _ CountRange "?"? / "*?" / "+?" / "??" / "*" / "+" / "?"
```

## Upstream Raku NQP (QRegex) vs `gcre`

| Dimension | Upstream Raku NQP (`Rakudo/nqp`) | `gcre` (Go NQP / QRegex Engine) |
| :--- | :--- | :--- |
| **Bootstrap & Host** | Self-hosted in NQP → QAST → MAST → MoarVM / JVM / JS | Pure Go (`pigeon` PEG metagrammar → matcher combinators); zero runtime C/VM dependencies |
| **Grammar Inheritance & Protos** | `grammar Child is Parent`, `proto token/rule/regex { * }`, `:sym<...>` | Same declarative syntax & inheritance (`LoadGrammarWithParents`, `Extend`) |
| **LTM (`\|`) vs Sequential (`\|\|`)** | Compiles declarative prefixes into an NFA table run before imperative tails | Probes declarative branches (and `is ltm` rules) from current cursor and commits longest match |
| **Operator Precedence (`EXPR`)** | `HLL::Grammar::EXPR` + `%optable` (`is tighter`, `is looser`, `is equiv`, `is assoc`) | Declarative `is optable` + same traits compiled to a deterministic Pratt `OpTable` |
| **Backtracking Model** | Full continuation stack (`bstack`) in `regex`; `:ratchet` in `token`/`rule` | `:ratchet` greedy/frugal quantifiers with `Match` capture snapshot/restore on branch failure |
| **Semantic Actions** | NQP action classes (`method foo($/) { make ... }`) + inline `{ ... }` code blocks | Go `ActionHandler` or reflection methods (`func (a *Actions) Foo(m *Match)`) + `<HOST_name>` |
| **Omitted Regex Features** | Inline `{ code }`, `<?{ pred }>`, `:my($x)`, `<$interp>`, `:i`/`:m`, `<( )>`, `&` | Intentionally omitted to keep `.raku` grammars declarative and host-agnostic |

## Examples

- [`examples/nqp_lang.raku`](examples/nqp_lang.raku) — Multi-grammar inheritance, `proto`/`:sym<...>`, `is optable`, `%`, `~` goal matching, lookarounds, and character class set algebra
- [`examples/minicalc.raku`](examples/minicalc.raku) — Expression grammar
- [`examples/tinyjson.raku`](examples/tinyjson.raku) — JSON grammar

## AI credit

The first prototype was written by **Gemini**. Later work was modified by **Grok** and audited/extended with NQP/QRegex constructs.
