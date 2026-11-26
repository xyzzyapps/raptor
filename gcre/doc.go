// Package gcre implements a Raku / NQP declarative PEG grammar and Pratt
// operator-precedence engine in pure Go.
//
// Supported Raku / QRegex grammar constructs include:
//   - Grammar inheritance (`grammar Child is Parent`), `rule`, `token`, and `regex`
//   - Protoregex multi-dispatch (`proto token term { * }` + `:sym<...>`) with `<sym>` injection
//   - Longest-Token Matching (`|`) alongside ordered PEG alternation (`||`)
//   - Declarative `is optable` Pratt precedence climbers (`is tighter`, `is looser`, `is equiv`, `is assoc`)
//   - Scoped regex adverbs (`:i` ignorecase, `:m` ignoremark, `:s` sigspace, `:r` ratchet)
//   - Conjunctions (`&`, `&&`), lookarounds (`<?before ...>`, `<!before ...>`, `<?after ...>`, `<!after ...>`)
//   - Capture result markers (`<( ... )>`), backtracking cuts (`:`, `::`, `:::`), and self-recursion (`<~~>`)
//   - Positional and named backreferences (`$0`, `$<name>`), tilde goal-matching (`'(' ~ ')' <expr>`),
//     quantifier separators (`%`, `%%`, `** min..max`), and AST reduction via `make()` / ActionHandler.
package gcre
