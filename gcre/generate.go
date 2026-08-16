package gcre

// Regenerates the .raku file parser. Authors still write .raku, not .peg.
//go:generate go run github.com/mna/pigeon@v1.3.0 -o raku_parser.go raku.peg


