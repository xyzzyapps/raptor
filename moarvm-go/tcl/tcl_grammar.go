package tcl

import (
	"context"
	_ "embed"
	"fmt"
	"gcre"
	"moarvm-go/engine"
)

//go:embed tcl.raku
var tclGrammarSource string

var cachedTclGrammar *gcre.Grammar

// GetTclGrammar returns the grammar compiled from tcl.raku (source of truth).
func GetTclGrammar() (*gcre.Grammar, error) {
	if cachedTclGrammar != nil {
		return cachedTclGrammar, nil
	}
	g, err := gcre.LoadGrammarFromString(tclGrammarSource)
	if err != nil {
		return nil, err
	}
	cachedTclGrammar = g
	return cachedTclGrammar, nil
}

// TclCmd represents a parsed AST command.
type TclCmd struct {
	Name string
	Args []string
}

// ParseTclWithGrammar parses and executes Tcl scripts using the Grammar engine.
func ParseTclWithGrammar(in *Interp, script string) (string, error) {
	g, err := GetTclGrammar()
	if err != nil {
		return "", err
	}

	ctx := &gcre.Context{Src: []rune(script), Pos: 0}
	match := g.Subrule("TOP", ctx)
	if !match.Ok {
		return "", fmt.Errorf("grammar parse error in tcl script at position %d", ctx.Pos)
	}

	var lastResult string
	for _, cmdMatch := range collectCommands(match) {
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

// CompileGrammarToMoarVM parses Tcl via the grammar and compiles to MoarVM bytecode.
func CompileGrammarToMoarVM(script string) ([]byte, error) {
	return NewCompiler().CompileScript(script)
}

// RunGrammarOnMoarVM executes Tcl parsed via TclGrammar directly on MoarVM.
func RunGrammarOnMoarVM(ctx context.Context, vm moargo.Engine, script string) error {
	bc, err := CompileGrammarToMoarVM(script)
	if err != nil {
		return err
	}
	return vm.RunBytecode(ctx, bc)
}
