# MiniCalc — assignment statements. Valid Raku; PEG-compatible subset.

grammar MiniCalc {
    rule TOP { <statement> }
    rule statement { <ident> '=' <number> ';' }
    token ident { \w+ }
    token number { \d+ }
}
