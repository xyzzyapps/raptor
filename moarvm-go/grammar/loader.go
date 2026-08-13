package grammar

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

// DynamicRule represents a parsed rule definition from a .raku file.
type DynamicRule struct {
	Name     string
	IsToken  bool // true if 'token', false if 'rule' (:sigspace)
	Patterns []string
}

// DynamicGrammarDef represents the parsed grammar schema.
type DynamicGrammarDef struct {
	Name  string
	Rules []*DynamicRule
}

// LoadGrammarFromFile reads and parses a .raku grammar file into a runnable *Grammar.
func LoadGrammarFromFile(path string) (*Grammar, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading grammar file %q: %w", path, err)
	}
	return LoadGrammarFromString(string(content))
}

// LoadGrammarFromString parses a grammar definition string into a runnable *Grammar.
func LoadGrammarFromString(source string) (*Grammar, error) {
	def, err := parseGrammarDefinition(source)
	if err != nil {
		return nil, err
	}

	g := NewGrammar(def.Name)
	for _, r := range def.Rules {
		ruleDef := r
		g.DefineRule(ruleDef.Name, func(g *Grammar, ctx *Context) *Match {
			if !ruleDef.IsToken {
				ctx.SkipWS()
			}
			start := ctx.Pos
			m := NewMatch("", start, start, true)

			for _, elem := range ruleDef.Patterns {
				if !ruleDef.IsToken {
					ctx.SkipWS()
				}
				if ctx.Pos >= len(ctx.Src) {
					// Check if element was optional or end
					if elem == "$" {
						break
					}
				}

				// Subrule call <subrule_name>
				if strings.HasPrefix(elem, "<") && strings.HasSuffix(elem, ">") {
					subName := elem[1 : len(elem)-1]
					subName = strings.TrimSuffix(subName, "*")
					subName = strings.TrimSuffix(subName, "+")

					// Standard built-in regex tokens
					var subMatch *Match
					switch subName {
					case "\\d+", "\\d":
						subMatch = g.Number(ctx)
					case "\\w+", "\\w":
						subMatch = g.Ident(ctx)
					case "\\s+", "\\s":
						ctx.SkipWS()
						subMatch = NewMatch(" ", start, ctx.Pos, true)
					default:
						subMatch = g.Subrule(subName, ctx)
					}

					if !subMatch.Ok {
						ctx.Pos = start
						return NewMatch("", start, start, false)
					}
					m.AddNamed(subName, subMatch)
				} else {
					// Literal string match
					lit := elem
					if strings.HasPrefix(lit, "'") && strings.HasSuffix(lit, "'") && len(lit) >= 2 {
						lit = lit[1 : len(lit)-1]
					} else if strings.HasPrefix(lit, "\"") && strings.HasSuffix(lit, "\"") && len(lit) >= 2 {
						lit = lit[1 : len(lit)-1]
					}
					lMatch := g.ExactLit(lit, ctx)
					if !lMatch.Ok {
						ctx.Pos = start
						return NewMatch("", start, start, false)
					}
				}
			}

			m.To = ctx.Pos
			m.Str = string(ctx.Src[start:ctx.Pos])
			return m
		})
	}

	return g, nil
}

func parseGrammarDefinition(source string) (*DynamicGrammarDef, error) {
	lines := strings.Split(source, "\n")
	def := &DynamicGrammarDef{Name: "AnonymousGrammar"}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || len(trimmed) == 0 {
			continue
		}

		// grammar Name {
		if strings.HasPrefix(trimmed, "grammar ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				def.Name = strings.TrimSuffix(parts[1], "{")
				def.Name = strings.TrimSpace(def.Name)
			}
			continue
		}

		// rule Name { ... } or token Name { ... }
		isRule := strings.HasPrefix(trimmed, "rule ")
		isToken := strings.HasPrefix(trimmed, "token ") || strings.HasPrefix(trimmed, "regex ")

		if isRule || isToken {
			bodyStart := strings.Index(trimmed, "{")
			bodyEnd := strings.LastIndex(trimmed, "}")
			if bodyStart == -1 || bodyEnd == -1 || bodyEnd <= bodyStart {
				continue
			}

			header := strings.TrimSpace(trimmed[:bodyStart])
			headerParts := strings.Fields(header)
			if len(headerParts) < 2 {
				continue
			}
			ruleName := headerParts[1]
			body := strings.TrimSpace(trimmed[bodyStart+1 : bodyEnd])

			// Tokenize rule body into elements (<subrule>, 'literal', etc.)
			elems := tokenizeRuleBody(body)
			def.Rules = append(def.Rules, &DynamicRule{
				Name:     ruleName,
				IsToken:  isToken,
				Patterns: elems,
			})
		}
	}

	return def, nil
}

func tokenizeRuleBody(body string) []string {
	var elems []string
	runes := []rune(body)
	i := 0
	for i < len(runes) {
		for i < len(runes) && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= len(runes) {
			break
		}

		// Subrule <...>
		if runes[i] == '<' {
			start := i
			for i < len(runes) && runes[i] != '>' {
				i++
			}
			if i < len(runes) {
				i++ // consume '>'
			}
			elems = append(elems, string(runes[start:i]))
			continue
		}

		// String literal '...' or "..."
		if runes[i] == '\'' || runes[i] == '"' {
			q := runes[i]
			start := i
			i++
			for i < len(runes) && runes[i] != q {
				i++
			}
			if i < len(runes) {
				i++
			}
			elems = append(elems, string(runes[start:i]))
			continue
		}

		// Word or symbol
		start := i
		for i < len(runes) && !unicode.IsSpace(runes[i]) && runes[i] != '<' && runes[i] != '\'' && runes[i] != '"' {
			i++
		}
		elems = append(elems, string(runes[start:i]))
	}
	return elems
}
