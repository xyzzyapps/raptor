# TinyJSON — numbers or quoted strings. Valid Raku; PEG-compatible subset.

grammar TinyJSON {
    token TOP { <value> }
    token value { | <number> | <string> }
    token number { \d+ }
    token string { '"' <-["]>* '"' }
}
