package gcre

import (
	"fmt"
	"strings"
)

// PEG ⊂ Raku spelling (1-1). Authors write only this; no actions in .raku.
//
//	PEG              Raku
//	A / B            | A | B
//	A B              A B
//	A* A+ A?         A* A+ A?
//	[ e ]            [ e ]
//	'lit' / "lit"    'lit' / "lit"
//	.                .
//	[a-z]            <[a..z]>
//	[^a-z]           <-[a..z]>
//	&e / !e          (not in this subset)
//	name <- e        token name { e }   (or rule / regex)

func walkGrammarFile(v any) (*DynamicGrammarDef, error) {
	seq, ok := v.([]any)
	if !ok || len(seq) < 2 {
		return nil, fmt.Errorf("raku: expected GrammarFile sequence")
	}
	return walkGrammarDecl(seq[1])
}

func walkGrammarDecl(v any) (*DynamicGrammarDef, error) {
	seq, ok := v.([]any)
	if !ok || len(seq) < 8 {
		return nil, fmt.Errorf("raku: expected GrammarDecl")
	}
	name := joinText(seq[2])
	var rules []*DynamicRule
	ruleIdx := 6
	if len(seq) > 7 {
		// "grammar" _ Name _ "{" _ Rules _ "}"
		ruleIdx = 6
	}
	for _, r := range asList(seq[ruleIdx]) {
		dr, err := walkRuleDecl(r)
		if err != nil {
			return nil, err
		}
		if dr != nil {
			rules = append(rules, dr)
		}
	}
	if name == "" {
		name = "AnonymousGrammar"
	}
	return &DynamicGrammarDef{Name: name, Rules: rules}, nil
}

func walkRuleDecl(v any) (*DynamicRule, error) {
	seq, ok := v.([]any)
	if !ok || len(seq) < 7 {
		return nil, fmt.Errorf("raku: expected RuleDecl")
	}
	kind := joinText(seq[0])
	name := joinText(seq[2])
	pat, err := walkRxAlt(seq[6])
	if err != nil {
		return nil, fmt.Errorf("rule %s: %w", name, err)
	}
	return &DynamicRule{
		Name:    name,
		IsToken: kind != "rule",
		Pat:     pat,
	}, nil
}

func walkRxAlt(v any) (rx, error) {
	seq := asList(v)
	// (_ "|")? _ RxSeq (_ "|" _ RxSeq)*
	if len(seq) < 4 {
		return walkRxSeq(v)
	}
	head, err := walkRxSeq(seq[2])
	if err != nil {
		return nil, err
	}
	alts := []rx{head}
	for _, t := range asList(seq[3]) {
		pair := asList(t)
		if len(pair) < 4 {
			continue
		}
		s, err := walkRxSeq(pair[3])
		if err != nil {
			return nil, err
		}
		alts = append(alts, s)
	}
	if len(alts) == 1 {
		return alts[0], nil
	}
	return &rxAlt{alts: alts}, nil
}

func walkRxSeq(v any) (rx, error) {
	seq := asList(v)
	if len(seq) == 0 {
		return &rxSeq{}, nil
	}
	// RxAtom (_ RxAtom)*
	first, err := walkRxAtom(seq[0])
	if err != nil {
		return nil, err
	}
	elts := []rx{first}
	if len(seq) > 1 {
		for _, item := range asList(seq[1]) {
			pair := asList(item)
			if len(pair) == 0 {
				continue
			}
			p, err := walkRxAtom(pair[len(pair)-1])
			if err != nil {
				return nil, err
			}
			if p != nil {
				elts = append(elts, p)
			}
		}
	}
	if len(elts) == 1 {
		return elts[0], nil
	}
	return &rxSeq{elts: elts}, nil
}

func walkRxAtom(v any) (rx, error) {
	seq := asList(v)
	if len(seq) < 1 {
		return nil, fmt.Errorf("raku: empty atom")
	}
	p, err := walkRxPrimary(seq[0])
	if err != nil {
		return nil, err
	}
	q := byte(' ')
	if len(seq) > 1 && seq[1] != nil {
		qs := joinText(seq[1])
		if qs != "" {
			q = qs[0]
		}
	}
	return applyRxQuant(p, q), nil
}

func walkRxPrimary(v any) (rx, error) {
	switch x := v.(type) {
	case []byte:
		return walkRxPrimaryText(string(x))
	case string:
		return walkRxPrimaryText(x)
	case []any:
		if len(x) == 0 {
			return nil, fmt.Errorf("raku: empty primary")
		}
		// RxGroup: "[", _, alt, _, "]"
		if joinText(x[0]) == "[" && len(x) >= 5 {
			inner, err := walkRxAlt(x[2])
			if err != nil {
				return nil, err
			}
			return &rxGrp{inner: inner, quant: ' '}, nil
		}
		// RxAngle: "<", DotOpt, AngleBody, ">"
		if joinText(x[0]) == "<" && len(x) >= 4 {
			return walkRxAngle(x)
		}
		// RxSq / RxDq: quote, chars, quote
		if q := joinText(x[0]); (q == "'" || q == `"`) && len(x) >= 3 {
			return &rxLit{s: joinText(x[1])}, nil
		}
		// RxEsc: "\", "d"  or "\", "."
		if joinText(x[0]) == `\` && len(x) >= 2 {
			esc := joinText(x[1])
			if esc != "" && strings.ContainsRune("ntrsdwNTRSDW", rune(esc[0])) {
				return &rxCC{spec: `\` + strings.ToLower(esc[:1]), quant: ' '}, nil
			}
			if esc != "" {
				return &rxLit{s: string(unescapeRune(esc))}, nil
			}
		}
		return walkRxPrimaryText(joinText(x))
	default:
		return walkRxPrimaryText(joinText(v))
	}
}

func walkRxAngle(x []any) (rx, error) {
	dot := joinText(x[1])
	capture := dot != "."
	body := x[2]
	// ClassSpec: optional "-", "[", inner, "]"
	if bs := asList(body); len(bs) >= 3 && joinText(bs[len(bs)-3]) == "[" || (len(bs) >= 4 && joinText(bs[1]) == "[") {
		neg := false
		inner := ""
		if joinText(bs[0]) == "-" {
			neg = true
			inner = joinText(bs[2])
		} else if joinText(bs[0]) == "[" {
			inner = joinText(bs[1])
		} else {
			inner = joinText(bs[2])
		}
		spec := "<[" + inner + "]>"
		if neg {
			spec = "<-[" + inner + "]>"
		}
		return &rxCC{spec: spec, quant: ' '}, nil
	}
	name := joinText(body)
	if name == "" {
		return nil, fmt.Errorf("raku: empty <subrule>")
	}
	return &rxSub{name: name, capture: capture, quant: ' '}, nil
}

func walkRxPrimaryText(s string) (rx, error) {
	switch s {
	case ".":
		return &rxAny{}, nil
	case "#":
		return &rxLit{s: "#"}, nil
	case ";":
		return &rxLit{s: ";"}, nil
	case "$":
		return &rxLit{s: "$"}, nil
	default:
		if s == "" {
			return nil, fmt.Errorf("raku: empty primary text")
		}
		return &rxLit{s: s}, nil
	}
}

func asList(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return []any{v}
}

func joinText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, e := range x {
			b.WriteString(joinText(e))
		}
		return b.String()
	default:
		return fmt.Sprint(x)
	}
}
