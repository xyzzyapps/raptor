package tcl

import (
	"fmt"
	"gcre"
	"strings"
	"unicode"
)

// WordKind classifies a parsed Tcl word.
type WordKind int

const (
	WordBare WordKind = iota
	WordBrace
	WordQuote
	WordVar
	WordCmd
)

// Word is a parsed Tcl word with its original spelling.
type Word struct {
	Kind  WordKind
	Raw   string
	Inner string
}

// Command is a parsed Tcl command (one or more words).
type Command struct {
	Words []Word
}

// ParseTclAST tokenizes a script with the Tcl grammar into commands.
// This is the MoarVM-side parse: no evaluation, no substitution.
func ParseTclAST(script string) ([]Command, error) {
	g, err := GetTclGrammar()
	if err != nil {
		return nil, err
	}
	ctx := &gcre.Context{Src: []rune(script), Pos: 0}
	match := g.Subrule("TOP", ctx)
	if !match.Ok {
		return nil, fmt.Errorf("grammar parse error in tcl script at position %d", ctx.Pos)
	}

	var cmds []Command
	for _, cmdMatch := range collectCommands(match) {
		if cmdMatch == nil || !cmdMatch.Ok {
			continue
		}
		wordsMatches := cmdMatch.GetAll("word")
		if len(wordsMatches) == 0 {
			continue
		}
		var words []Word
		for _, wm := range wordsMatches {
			raw := string(ctx.Src[wm.From:wm.To])
			words = append(words, classifyWord(raw, wm.Str))
		}
		if len(words) > 0 {
			cmds = append(cmds, Command{Words: words})
		}
	}
	return cmds, nil
}

func collectCommands(m *gcre.Match) []*gcre.Match {
	if m == nil || !m.Ok {
		return nil
	}
	var out []*gcre.Match
	if cmds := m.GetAll("command"); len(cmds) > 0 {
		// Prefer direct command captures, but still walk nested command_line
		// so TOP { <command_line>* } is handled.
		direct := true
		for _, c := range cmds {
			if c != nil && c.Ok {
				out = append(out, c)
			}
		}
		if len(out) > 0 && direct {
			return out
		}
	}
	for _, line := range m.GetAll("command_line") {
		out = append(out, collectCommands(line)...)
	}
	return out
}

func classifyWord(raw, inner string) Word {
	raw = trimWS(raw)
	w := Word{Raw: raw, Inner: inner}
	if len(raw) == 0 {
		w.Kind = WordBare
		return w
	}
	switch raw[0] {
	case '{':
		w.Kind = WordBrace
		if len(raw) >= 2 {
			w.Inner = raw[1 : len(raw)-1]
		}
	case '"':
		w.Kind = WordQuote
		if len(raw) >= 2 {
			w.Inner = raw[1 : len(raw)-1]
		}
	case '[':
		w.Kind = WordCmd
		if len(raw) >= 2 {
			w.Inner = raw[1 : len(raw)-1]
		}
	case '$':
		w.Kind = WordVar
		name := w.Inner
		if name == "" || strings.HasPrefix(name, "$") {
			name = raw
		}
		if strings.HasPrefix(name, "$") {
			name = name[1:]
		}
		if len(name) >= 2 && name[0] == '{' && name[len(name)-1] == '}' {
			name = name[1 : len(name)-1]
		}
		w.Inner = name
	default:
		w.Kind = WordBare
		if w.Inner == "" {
			w.Inner = raw
		}
	}
	return w
}

func trimWS(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}

func (w Word) isIdentStart() bool {
	if w.Inner == "" {
		return false
	}
	r := rune(w.Inner[0])
	return unicode.IsLetter(r) || r == '_'
}
