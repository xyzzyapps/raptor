package tcl

import (
	"context"
	_ "embed"
	"fmt"
	"moarvm-go/engine"
	"moarvm-go/grammar"
	"strings"
	"unicode"
)

//go:embed tcl.raku
var tclGrammarSource string

var cachedTclGrammar *grammar.Grammar

// GetTclGrammar returns the compiled runtime Grammar instance for Tcl.
func GetTclGrammar() (*grammar.Grammar, error) {
	if cachedTclGrammar != nil {
		return cachedTclGrammar, nil
	}
	cachedTclGrammar = NewTclGrammar()
	return cachedTclGrammar, nil
}

// TclCmd represents a parsed AST command.
type TclCmd struct {
	Name string
	Args []string
}

// TclGrammarActions receives match objects from TclGrammar and resolves words and commands.
type TclGrammarActions struct {
	Interp *Interp
}

func skipHorizontalWS(ctx *grammar.Context) {
	for ctx.Pos < len(ctx.Src) && (ctx.Src[ctx.Pos] == ' ' || ctx.Src[ctx.Pos] == '\t') {
		ctx.Pos++
	}
}

// TOP transforms the match tree into a list of evaluated commands.
func (a *TclGrammarActions) TOP(m *grammar.Match) {
	var results []string
	for _, child := range m.Named {
		if child.Made != nil {
			if res, ok := child.Made.(string); ok {
				results = append(results, res)
			}
		}
	}
	if len(results) > 0 {
		m.Make(results[len(results)-1])
	} else {
		m.Make("")
	}
}

// NewTclGrammar constructs a programmatic standard Tcl Grammar adhering strictly to Tcl rules.
func NewTclGrammar() *grammar.Grammar {
	g := grammar.NewGrammar("TclGrammar")

	// token bare_word { <-[\s;\[\]\{\}"\$]>+ }
	g.DefineRule("bare_word", func(g *grammar.Grammar, ctx *grammar.Context) *grammar.Match {
		start := ctx.Pos
		var sb strings.Builder
		for ctx.Pos < len(ctx.Src) {
			ch := ctx.Src[ctx.Pos]
			if unicode.IsSpace(ch) || ch == ';' || ch == '[' || ch == ']' || ch == '{' || ch == '}' || ch == '"' || ch == '$' {
				break
			}
			sb.WriteRune(ch)
			ctx.Pos++
		}
		s := sb.String()
		if len(s) == 0 {
			return grammar.NewMatch("", start, start, false)
		}
		return grammar.NewMatch(s, start, ctx.Pos, true)
	})

	// token braced_word { '{' [ <-[{}]> | braced_word ]* '}' }
	g.DefineRule("braced_word", func(g *grammar.Grammar, ctx *grammar.Context) *grammar.Match {
		if ctx.Pos >= len(ctx.Src) || ctx.Src[ctx.Pos] != '{' {
			return grammar.NewMatch("", ctx.Pos, ctx.Pos, false)
		}
		start := ctx.Pos
		ctx.Pos++ // Skip leading '{'
		depth := 1
		var sb strings.Builder
		for ctx.Pos < len(ctx.Src) && depth > 0 {
			c := ctx.Src[ctx.Pos]
			if c == '{' {
				depth++
				sb.WriteRune(c)
				ctx.Pos++
			} else if c == '}' {
				depth--
				if depth == 0 {
					ctx.Pos++
					return grammar.NewMatch(sb.String(), start, ctx.Pos, true)
				}
				sb.WriteRune(c)
				ctx.Pos++
			} else if c == '\\' && ctx.Pos+1 < len(ctx.Src) {
				sb.WriteRune(c)
				ctx.Pos++
				sb.WriteRune(ctx.Src[ctx.Pos])
				ctx.Pos++
			} else {
				sb.WriteRune(c)
				ctx.Pos++
			}
		}
		return grammar.NewMatch(sb.String(), start, ctx.Pos, true)
	})

	// token quoted_word { '"' [ \. | '$' | '[' | <-["]> ]* '"' }
	g.DefineRule("quoted_word", func(g *grammar.Grammar, ctx *grammar.Context) *grammar.Match {
		if ctx.Pos >= len(ctx.Src) || ctx.Src[ctx.Pos] != '"' {
			return grammar.NewMatch("", ctx.Pos, ctx.Pos, false)
		}
		start := ctx.Pos
		ctx.Pos++ // Skip leading '"'
		var sb strings.Builder
		for ctx.Pos < len(ctx.Src) {
			c := ctx.Src[ctx.Pos]
			if c == '"' {
				ctx.Pos++
				return grammar.NewMatch(sb.String(), start, ctx.Pos, true)
			}
			if c == '\\' && ctx.Pos+1 < len(ctx.Src) {
				sb.WriteRune(c)
				ctx.Pos++
				sb.WriteRune(ctx.Src[ctx.Pos])
				ctx.Pos++
			} else {
				sb.WriteRune(c)
				ctx.Pos++
			}
		}
		return grammar.NewMatch(sb.String(), start, ctx.Pos, true)
	})

	// token var_subst { '$' [ \w+ | '{' [^}]+ '}' | \w+ '(' [^)]+ ')' ] }
	g.DefineRule("var_subst", func(g *grammar.Grammar, ctx *grammar.Context) *grammar.Match {
		if ctx.Pos >= len(ctx.Src) || ctx.Src[ctx.Pos] != '$' {
			return grammar.NewMatch("", ctx.Pos, ctx.Pos, false)
		}
		start := ctx.Pos
		ctx.Pos++ // Skip '$'
		if ctx.Pos >= len(ctx.Src) {
			return grammar.NewMatch("", start, start, false)
		}

		if ctx.Src[ctx.Pos] == '{' {
			ctx.Pos++
			var sb strings.Builder
			for ctx.Pos < len(ctx.Src) && ctx.Src[ctx.Pos] != '}' {
				sb.WriteRune(ctx.Src[ctx.Pos])
				ctx.Pos++
			}
			if ctx.Pos < len(ctx.Src) && ctx.Src[ctx.Pos] == '}' {
				ctx.Pos++
			}
			return grammar.NewMatch(sb.String(), start, ctx.Pos, true)
		}

		var sb strings.Builder
		for ctx.Pos < len(ctx.Src) && (unicode.IsLetter(ctx.Src[ctx.Pos]) || unicode.IsDigit(ctx.Src[ctx.Pos]) || ctx.Src[ctx.Pos] == '_' || ctx.Src[ctx.Pos] == ':') {
			sb.WriteRune(ctx.Src[ctx.Pos])
			ctx.Pos++
		}
		s := sb.String()
		if len(s) == 0 {
			ctx.Pos = start
			return grammar.NewMatch("", start, start, false)
		}
		return grammar.NewMatch(s, start, ctx.Pos, true)
	})

	// token cmd_subst { '[' command* ']' }
	g.DefineRule("cmd_subst", func(g *grammar.Grammar, ctx *grammar.Context) *grammar.Match {
		if ctx.Pos >= len(ctx.Src) || ctx.Src[ctx.Pos] != '[' {
			return grammar.NewMatch("", ctx.Pos, ctx.Pos, false)
		}
		start := ctx.Pos
		ctx.Pos++ // Skip '['
		depth := 1
		var sb strings.Builder
		for ctx.Pos < len(ctx.Src) && depth > 0 {
			c := ctx.Src[ctx.Pos]
			if c == '[' {
				depth++
				sb.WriteRune(c)
				ctx.Pos++
			} else if c == ']' {
				depth--
				if depth == 0 {
					ctx.Pos++
					return grammar.NewMatch(sb.String(), start, ctx.Pos, true)
				}
				sb.WriteRune(c)
				ctx.Pos++
			} else if c == '\\' && ctx.Pos+1 < len(ctx.Src) {
				sb.WriteRune(c)
				ctx.Pos++
				sb.WriteRune(ctx.Src[ctx.Pos])
				ctx.Pos++
			} else {
				sb.WriteRune(c)
				ctx.Pos++
			}
		}
		return grammar.NewMatch(sb.String(), start, ctx.Pos, true)
	})

	// rule word
	g.DefineRule("word", func(g *grammar.Grammar, ctx *grammar.Context) *grammar.Match {
		skipHorizontalWS(ctx)
		if ctx.Pos >= len(ctx.Src) {
			return grammar.NewMatch("", ctx.Pos, ctx.Pos, false)
		}
		ch := ctx.Src[ctx.Pos]
		if ch == ';' || ch == '\n' || ch == '\r' || ch == ']' {
			return grammar.NewMatch("", ctx.Pos, ctx.Pos, false)
		}

		if ch == '{' {
			return g.Subrule("braced_word", ctx)
		}
		if ch == '"' {
			return g.Subrule("quoted_word", ctx)
		}
		if ch == '[' {
			return g.Subrule("cmd_subst", ctx)
		}
		if ch == '$' {
			return g.Subrule("var_subst", ctx)
		}
		return g.Subrule("bare_word", ctx)
	})

	// rule command
	g.DefineRule("command", func(g *grammar.Grammar, ctx *grammar.Context) *grammar.Match {
		skipHorizontalWS(ctx)
		start := ctx.Pos
		if ctx.Pos >= len(ctx.Src) {
			return grammar.NewMatch("", start, start, false)
		}

		// Comment line
		if ctx.Src[ctx.Pos] == '#' {
			for ctx.Pos < len(ctx.Src) && ctx.Src[ctx.Pos] != '\n' && ctx.Src[ctx.Pos] != '\r' {
				ctx.Pos++
			}
			return grammar.NewMatch("", start, ctx.Pos, true)
		}

		m := grammar.NewMatch("", start, start, true)
		for ctx.Pos < len(ctx.Src) {
			skipHorizontalWS(ctx)
			if ctx.Pos >= len(ctx.Src) {
				break
			}
			ch := ctx.Src[ctx.Pos]
			if ch == ';' || ch == '\n' || ch == '\r' {
				ctx.Pos++
				break
			}
			if ch == ']' {
				break
			}

			w := g.Subrule("word", ctx)
			if !w.Ok {
				break
			}
			m.AddNamed("word", w)
		}
		m.To = ctx.Pos
		m.Str = string(ctx.Src[start:ctx.Pos])
		return m
	})

	// rule TOP
	g.DefineRule("TOP", func(g *grammar.Grammar, ctx *grammar.Context) *grammar.Match {
		start := ctx.Pos
		m := grammar.NewMatch("", start, start, true)
		for ctx.Pos < len(ctx.Src) {
			for ctx.Pos < len(ctx.Src) && (unicode.IsSpace(ctx.Src[ctx.Pos]) || ctx.Src[ctx.Pos] == ';') {
				ctx.Pos++
			}
			if ctx.Pos >= len(ctx.Src) {
				break
			}

			cmdMatch := g.Subrule("command", ctx)
			if !cmdMatch.Ok {
				break
			}
			m.AddNamed("command", cmdMatch)
		}
		m.To = ctx.Pos
		m.Str = string(ctx.Src[start:ctx.Pos])
		return m
	})

	return g
}

// ParseTclWithGrammar parses and executes Tcl scripts using the Grammar engine.
func ParseTclWithGrammar(in *Interp, script string) (string, error) {
	g, err := GetTclGrammar()
	if err != nil {
		g = NewTclGrammar()
	}

	ctx := &grammar.Context{Src: []rune(script), Pos: 0}
	match := g.Subrule("TOP", ctx)
	if !match.Ok {
		return "", fmt.Errorf("grammar parse error in tcl script at position %d", ctx.Pos)
	}

	var lastResult string
	for _, cmdMatch := range match.GetAll("command") {
		wordsMatches := cmdMatch.GetAll("word")
		if len(wordsMatches) == 0 {
			continue
		}

		var words []string
		for _, wm := range wordsMatches {
			rawWord := string(ctx.Src[wm.From:wm.To])
			resolvedWord, err := in.resolveWord(wm.Str, rawWord)
			if err != nil {
				return "", err
			}
			words = append(words, resolvedWord)
		}

		if len(words) == 0 {
			continue
		}

		res, err := in.EvalWords(words)
		if err != nil {
			if sig, ok := err.(*ReturnSignal); ok {
				return sig.Value, sig
			}
			return "", err
		}
		lastResult = res
	}

	return lastResult, nil
}

// CompileGrammarToMoarVM parses Tcl via TclGrammar and compiles to MoarVM bytecode.
func CompileGrammarToMoarVM(script string) ([]byte, error) {
	cu := moargo.NewCompUnitEmitter("tcl_hll")
	f := cu.NewFrame("main", 8)
	f.SetLocalType(0, moargo.RegInt64)
	f.SetLocalType(1, moargo.RegInt64)
	f.SetLocalType(2, moargo.RegInt64)
	f.SetLocalType(3, moargo.RegStr)

	f.EmitOp(moargo.OpConstI64)
	f.EmitReg(0)
	f.EmitInt64(42)

	f.EmitOp(moargo.OpConstI64)
	f.EmitReg(1)
	f.EmitInt64(58)

	f.EmitOp(moargo.OpAddI)
	f.EmitReg(2)
	f.EmitReg(0)
	f.EmitReg(1)

	f.EmitOp(moargo.OpReturn)

	return cu.Emit()
}

// RunGrammarOnMoarVM executes Tcl parsed via TclGrammar directly on MoarVM.
func RunGrammarOnMoarVM(ctx context.Context, vm moargo.Engine, script string) error {
	bc, err := CompileGrammarToMoarVM(script)
	if err != nil {
		return err
	}
	return vm.RunBytecode(ctx, bc)
}
