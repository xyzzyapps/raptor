package raptor

import (
	"fmt"
	"sort"
	"strings"
)

// Dispatcher resolves multiple dispatch candidates for a given argument list.
type Dispatcher struct{}

type candidateMatch struct {
	closure *Closure
	score   int
}

// Resolve selects the best matching candidate closure for the supplied arguments.
func Resolve(candidates []*Closure, args []*Value) (*Closure, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no multi candidates defined")
	}

	var matches []candidateMatch

	for _, cand := range candidates {
		// Arity check
		if len(cand.Params) != len(args) {
			continue
		}

		// Type match and scoring
		matched := true
		score := 0

		for i, param := range cand.Params {
			arg := args[i]
			if !arg.MatchesType(param.Type) {
				matched = false
				break
			}
			score += computeTypeSpecificity(param.Type, arg)
		}

		if matched {
			matches = append(matches, candidateMatch{closure: cand, score: score})
		}
	}

	if len(matches) == 0 {
		var argTypes []string
		for _, a := range args {
			argTypes = append(argTypes, a.TypeName())
		}
		var sigs []string
		for _, c := range candidates {
			var ps []string
			for _, p := range c.Params {
				t := p.Type
				if t == "" {
					t = "Any"
				}
				ps = append(ps, fmt.Sprintf("%s %s", t, p.Name))
			}
			sigs = append(sigs, fmt.Sprintf("(%s)", strings.Join(ps, ", ")))
		}
		return nil, fmt.Errorf("cannot find matching multi candidate for argument types [%s]; available candidates: %s",
			strings.Join(argTypes, ", "), strings.Join(sigs, ", "))
	}

	// Sort matches by descending specificity score
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	// Check for ambiguity: if top two matches have identical score and different closures
	if len(matches) > 1 && matches[0].score == matches[1].score {
		return nil, fmt.Errorf("ambiguous multi dispatch for arguments; multiple candidates matched with equal specificity")
	}

	return matches[0].closure, nil
}

func computeTypeSpecificity(paramType string, arg *Value) int {
	if paramType == "" || paramType == "Any" {
		return 1
	}
	switch paramType {
	case "Int":
		if arg.Type == ValInt {
			return 10
		}
	case "Str":
		if arg.Type == ValString {
			return 10
		}
	case "Num":
		if arg.Type == ValFloat {
			return 10
		}
		if arg.Type == ValInt {
			return 5 // Int converted to Num is less specific than pure Int
		}
	case "Bool":
		if arg.Type == ValBool {
			return 10
		}
	case "Array":
		if arg.Type == ValArray {
			return 10
		}
	case "Hash":
		if arg.Type == ValHash {
			return 10
		}
	case "Callable":
		if arg.Type == ValClosure || arg.Type == ValMultiSub {
			return 10
		}
	}
	return 2
}
