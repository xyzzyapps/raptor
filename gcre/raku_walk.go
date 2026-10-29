package gcre

import (
	"fmt"
	"strconv"
	"strings"
)

// TraitOpRef identifies an operator referenced by is tighter / is looser / is equiv.
type TraitOpRef struct {
	Symbol    string
	HasFixity bool
	Fixity    Fixity
}

// RuleTraits holds declarative NQP/Raku traits attached to a rule/token/proto.
type RuleTraits struct {
	IsOptable bool
	IsLTM     bool
	Tighter   *TraitOpRef
	Looser    *TraitOpRef
	Equiv     *TraitOpRef
	Assoc     string
}

func walkGrammarFile(v any) ([]*DynamicGrammarDef, error) {
	seq, ok := v.([]any)
	if !ok || len(seq) < 3 {
		return nil, fmt.Errorf("raku: expected GrammarFile sequence")
	}
	first, err := walkGrammarDecl(seq[1])
	if err != nil {
		return nil, err
	}
	defs := []*DynamicGrammarDef{first}
	for _, item := range asList(seq[2]) {
		pair := asList(item)
		if len(pair) < 2 {
			continue
		}
		d, err := walkGrammarDecl(pair[1])
		if err != nil {
			return nil, err
		}
		defs = append(defs, d)
	}
	return defs, nil
}

func walkGrammarDecl(v any) (*DynamicGrammarDef, error) {
	seq, ok := v.([]any)
	if !ok || len(seq) < 11 {
		return nil, fmt.Errorf("raku: expected GrammarDecl")
	}
	name := joinText(seq[3])
	if name == "" {
		name = "AnonymousGrammar"
	}
	parent := ""
	if seq[4] != nil {
		ps := asList(seq[4])
		if len(ps) >= 5 {
			parent = joinText(ps[4])
		}
	}

	def := &DynamicGrammarDef{
		Name:   name,
		Parent: parent,
	}

	for _, m := range asList(seq[8]) {
		ms := asList(m)
		if len(ms) == 0 {
			continue
		}
		head := joinText(ms[0])
		if head == ";" {
			continue
		}
		if head == "proto" && len(ms) >= 14 {
			pr, err := walkProtoDecl(ms)
			if err != nil {
				return nil, err
			}
			def.Protos = append(def.Protos, pr)
			continue
		}
		dr, err := walkRuleDecl(ms)
		if err != nil {
			return nil, err
		}
		if dr != nil {
			def.Rules = append(def.Rules, dr)
		}
	}
	return def, nil
}

func walkProtoDecl(seq []any) (*DynamicProto, error) {
	kind := joinText(seq[3])
	name := joinText(seq[5])
	traits := walkTraits(seq[6])
	return &DynamicProto{
		Name:    name,
		IsToken: kind != "rule",
		Traits:  traits,
	}, nil
}

func walkRuleDecl(seq []any) (*DynamicRule, error) {
	if len(seq) < 12 {
		return nil, fmt.Errorf("raku: expected RuleDecl")
	}
	isMulti := seq[0] != nil
	kind := joinText(seq[1])
	fullName, baseName, symVal, hasSym := walkRuleName(seq[3])
	traits := walkTraits(seq[4])

	var pat rx = &rxNoop{}
	if seq[8] != nil {
		var err error
		pat, err = walkRxAlt(seq[8])
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", fullName, err)
		}
	}
	if traits.IsLTM {
		pat = forceLTM(pat)
	}
	return &DynamicRule{
		Name:     fullName,
		BaseName: baseName,
		Sym:      symVal,
		HasSym:   hasSym,
		IsMulti:  isMulti,
		IsToken:  kind != "rule",
		Traits:   traits,
		Pat:      pat,
	}, nil
}

func walkRuleName(v any) (fullName, baseName, symVal string, hasSym bool) {
	rn := asList(v)
	if len(rn) == 0 {
		return "", "", "", false
	}
	baseName = joinText(rn[0])
	if len(rn) > 1 && rn[1] != nil {
		sa := asList(rn[1])
		if len(sa) >= 2 {
			sd := asList(sa[1])
			if len(sd) >= 3 {
				symVal = joinText(sd[1])
				return baseName + ":sym<" + symVal + ">", baseName, symVal, true
			}
		}
	}
	return baseName, baseName, "", false
}

func walkTraits(v any) RuleTraits {
	var tr RuleTraits
	for _, item := range asList(v) {
		rt := asList(item)
		if len(rt) < 6 {
			continue
		}
		key := joinText(rt[4])
		arg := ""
		if rt[5] != nil {
			ta := asList(rt[5])
			if len(ta) >= 3 {
				arg = strings.TrimSpace(joinText(ta[1]))
			}
		}
		switch key {
		case "optable", "parsed":
			tr.IsOptable = true
		case "ltm":
			tr.IsLTM = true
		case "tighter":
			ref := parseTraitOpRef(arg)
			tr.Tighter = &ref
		case "looser":
			ref := parseTraitOpRef(arg)
			tr.Looser = &ref
		case "equiv":
			ref := parseTraitOpRef(arg)
			tr.Equiv = &ref
		case "assoc":
			tr.Assoc = cleanTraitQuotes(arg)
		}
	}
	return tr
}

func cleanTraitQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"')) {
		return s[1 : len(s)-1]
	}
	return s
}

func parseTraitOpRef(raw string) TraitOpRef {
	s := cleanTraitQuotes(raw)
	s = strings.TrimPrefix(s, "&")
	var ref TraitOpRef
	for _, pfx := range []struct {
		p string
		f Fixity
	}{
		{"infix:sym", FixityInfix},
		{"infix:", FixityInfix},
		{"prefix:sym", FixityPrefix},
		{"prefix:", FixityPrefix},
		{"postfix:sym", FixityPostfix},
		{"postfix:", FixityPostfix},
	} {
		if strings.HasPrefix(s, pfx.p) {
			ref.HasFixity = true
			ref.Fixity = pfx.f
			rest := strings.TrimSpace(s[len(pfx.p):])
			rest = trimSymDelims(rest)
			ref.Symbol = cleanTraitQuotes(rest)
			return ref
		}
	}
	ref.Symbol = cleanTraitQuotes(trimSymDelims(s))
	return ref
}

func trimSymDelims(s string) string {
	if strings.HasPrefix(s, "<<") && strings.HasSuffix(s, ">>") && len(s) >= 4 {
		return s[2 : len(s)-2]
	}
	if strings.HasPrefix(s, "«") && strings.HasSuffix(s, "»") && len(s) > len("«»") {
		return strings.TrimSuffix(strings.TrimPrefix(s, "«"), "»")
	}
	if len(s) >= 2 && ((s[0] == '<' && s[len(s)-1] == '>') || (s[0] == '[' && s[len(s)-1] == ']')) {
		return s[1 : len(s)-1]
	}
	return s
}

func walkRxAlt(v any) (rx, error) {
	seq := asList(v)
	// (_ "||")? _ RxLTMAlt (_ "||" _ RxLTMAlt)*
	if len(seq) < 4 {
		return walkRxLTMAlt(v)
	}
	head, err := walkRxLTMAlt(seq[2])
	if err != nil {
		return nil, err
	}
	alts := []rx{head}
	for _, t := range asList(seq[3]) {
		pair := asList(t)
		if len(pair) < 4 {
			continue
		}
		s, err := walkRxLTMAlt(pair[3])
		if err != nil {
			return nil, err
		}
		alts = append(alts, s)
	}
	if len(alts) == 1 {
		return alts[0], nil
	}
	return &rxAlt{alts: alts, sequential: true}, nil
}

func walkRxLTMAlt(v any) (rx, error) {
	seq := asList(v)
	// (_ "|" !"|")? _ RxSeq (_ "|" !"|" _ RxSeq)*
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
		if len(pair) < 5 {
			continue
		}
		s, err := walkRxSeq(pair[4])
		if err != nil {
			return nil, err
		}
		alts = append(alts, s)
	}
	if len(alts) == 1 {
		return alts[0], nil
	}
	allDecl := true
	for _, a := range alts {
		if !isPureDeclarative(a) {
			allDecl = false
			break
		}
	}
	if allDecl {
		return &rxLTMAlt{alts: alts}, nil
	}
	return &rxAlt{alts: alts, sequential: false}, nil
}

func walkRxSeq(v any) (rx, error) {
	seq := asList(v)
	if len(seq) == 0 {
		return &rxNoop{}, nil
	}
	// RxItem (_ RxItem)*
	first, err := walkRxItem(seq[0])
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
			p, err := walkRxItem(pair[len(pair)-1])
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

func walkRxItem(v any) (rx, error) {
	x := asList(v)
	if len(x) == 0 {
		return &rxNoop{}, nil
	}
	// NamedCap: "$<", NameTok, ">", _, "=", _, PlainAtom
	if len(x) == 7 && joinText(x[0]) == "$<" {
		alias := joinText(x[1])
		atom, err := walkPlainAtom(x[6])
		if err != nil {
			return nil, err
		}
		return applyNamedAlias(alias, atom), nil
	}
	// GoalOrAtom: PlainAtom, GoalTail?
	if len(x) == 2 {
		first, err := walkPlainAtom(x[0])
		if err != nil {
			return nil, err
		}
		if x[1] != nil {
			gt := asList(x[1])
			// _, "~", _, PlainAtom, _, RxItem
			if len(gt) >= 6 {
				goal, err := walkPlainAtom(gt[3])
				if err != nil {
					return nil, err
				}
				inner, err := walkRxItem(gt[5])
				if err != nil {
					return nil, err
				}
				return &rxGoal{
					open:     first,
					goal:     goal,
					inner:    inner,
					goalDesc: strings.TrimSpace(joinText(gt[3])),
				}, nil
			}
		}
		return first, nil
	}
	return walkPlainAtom(v)
}

func applyNamedAlias(alias string, atom rx) rx {
	switch t := atom.(type) {
	case *rxSub:
		cp := *t
		cp.alias = alias
		cp.capture = true
		return &cp
	case *rxHost:
		cp := *t
		cp.alias = alias
		cp.capture = true
		return &cp
	case *rxQuant:
		if sub, ok := t.inner.(*rxSub); ok {
			cpSub := *sub
			cpSub.alias = alias
			cpSub.capture = true
			cpQ := *t
			cpQ.inner = &cpSub
			return &cpQ
		}
		if h, ok := t.inner.(*rxHost); ok {
			cpH := *h
			cpH.alias = alias
			cpH.capture = true
			cpQ := *t
			cpQ.inner = &cpH
			return &cpQ
		}
		return &rxAlias{name: alias, inner: atom}
	default:
		return &rxAlias{name: alias, inner: atom}
	}
}

func walkPlainAtom(v any) (rx, error) {
	pa := asList(v)
	if len(pa) == 0 {
		return nil, fmt.Errorf("raku: empty atom")
	}
	prim, err := walkRxPrimary(pa[0])
	if err != nil {
		return nil, err
	}
	var q quantSpec
	if len(pa) > 1 && pa[1] != nil {
		q, err = walkQuantSpec(pa[1])
		if err != nil {
			return nil, err
		}
	}
	if len(pa) > 2 && pa[2] != nil {
		sc := asList(pa[2])
		// _, SepOp, _, SepAtom
		if len(sc) >= 4 {
			sepOp := joinText(sc[1])
			sepAtom, err := walkSepAtom(sc[3])
			if err != nil {
				return nil, err
			}
			q.sep = sepAtom
			q.sepTrail = sepOp == "%%"
		}
	}
	return wrapQuant(prim, q), nil
}

func walkSepAtom(v any) (rx, error) {
	sa := asList(v)
	if len(sa) == 0 {
		return nil, fmt.Errorf("raku: empty separator")
	}
	prim, err := walkRxPrimary(sa[0])
	if err != nil {
		return nil, err
	}
	var q quantSpec
	if len(sa) > 1 && sa[1] != nil {
		q, err = walkQuantSpec(sa[1])
		if err != nil {
			return nil, err
		}
	}
	return wrapQuant(prim, q), nil
}

func walkQuantSpec(v any) (quantSpec, error) {
	// StarStarQuant: _, "**", _, CountRange, FrugalOpt
	if ss := asList(v); len(ss) == 5 && joinText(ss[1]) == "**" {
		minV, maxV, err := walkCountRange(ss[3])
		if err != nil {
			return quantSpec{}, err
		}
		return quantSpec{
			present: true,
			min:     minV,
			max:     maxV,
			frugal:  joinText(ss[4]) == "?",
		}, nil
	}
	switch joinText(v) {
	case "*":
		return quantSpec{present: true, min: 0, max: -1}, nil
	case "+":
		return quantSpec{present: true, min: 1, max: -1}, nil
	case "?":
		return quantSpec{present: true, min: 0, max: 1}, nil
	case "*?":
		return quantSpec{present: true, min: 0, max: -1, frugal: true}, nil
	case "+?":
		return quantSpec{present: true, min: 1, max: -1, frugal: true}, nil
	case "??":
		return quantSpec{present: true, min: 0, max: 1, frugal: true}, nil
	default:
		return quantSpec{}, nil
	}
}

func walkCountRange(v any) (int, int, error) {
	cr := asList(v)
	if len(cr) == 5 && joinText(cr[2]) == ".." {
		lo, err := strconv.Atoi(joinText(cr[0]))
		if err != nil {
			return 0, 0, err
		}
		ub := strings.TrimSpace(joinText(cr[4]))
		if ub == "*" {
			return lo, -1, nil
		}
		hi, err := strconv.Atoi(ub)
		if err != nil {
			return 0, 0, err
		}
		return lo, hi, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(joinText(v)))
	if err != nil {
		return 0, 0, err
	}
	return n, n, nil
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
		h0 := joinText(x[0])
		// ActionStub: "{", _, "*", _, "}"
		if h0 == "{" && len(x) == 5 {
			return &rxNoop{}, nil
		}
		// RxGroup: "[", _, optAlt, _, "]"
		if h0 == "[" && len(x) == 5 {
			if x[2] == nil {
				return &rxNoop{}, nil
			}
			inner, err := walkRxAlt(x[2])
			if err != nil {
				return nil, err
			}
			return &rxGrp{inner: inner}, nil
		}
		// RxCapture: "(", _, optAlt, _, ")"
		if h0 == "(" && len(x) == 5 {
			var inner rx = &rxNoop{}
			if x[2] != nil {
				var err error
				inner, err = walkRxAlt(x[2])
				if err != nil {
					return nil, err
				}
			}
			return &rxCap{inner: inner}, nil
		}
		// Lookahead / Lookbehind: "<?" / "<!", _, "before" / "after", !NameCont, _, RxAlt, _, ">"
		if (h0 == "<?" || h0 == "<!") && len(x) == 8 {
			neg := h0 == "<!"
			dir := joinText(x[2])
			inner, err := walkRxAlt(x[5])
			if err != nil {
				return nil, err
			}
			if dir == "before" {
				return &rxLookahead{inner: inner, neg: neg}, nil
			}
			return &rxLookbehind{inner: inner, neg: neg}, nil
		}
		// StdAngle: "<", AnglePrefix, AngleBody, ">"
		if h0 == "<" && len(x) == 4 {
			return walkStdAngle(x)
		}
		// RxSq / RxDq
		if h0 == "'" && len(x) >= 3 {
			return &rxLit{s: unescapeSq(joinText(x[1]))}, nil
		}
		if h0 == `"` && len(x) >= 3 {
			return &rxLit{s: unescapeDq(joinText(x[1]))}, nil
		}
		// HexEsc: `\x[`, hex, `]`
		if h0 == `\x[` && len(x) >= 3 {
			return &rxLit{s: decodeHexBracket(joinText(x[1]))}, nil
		}
		// ClassEsc / AnyEsc: `\`, char
		if h0 == `\` && len(x) >= 2 {
			esc := joinText(x[1])
			if esc != "" && strings.ContainsRune("ntrsdwhvNTRSDWHV", rune(esc[0])) {
				return &rxCharClass{esc: rune(esc[0])}, nil
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

func walkStdAngle(x []any) (rx, error) {
	prefix := joinText(x[1]) // "?", "!", ".", or ""
	body := x[2]
	ab := asList(body)
	bodyStr := joinText(body)

	// 1. SubruleAlias: NameTok "=" (HostName / NameTok)
	if len(ab) == 3 && joinText(ab[1]) == "=" {
		alias := joinText(ab[0])
		target := joinText(ab[2])
		if strings.HasPrefix(target, "HOST_") {
			return &rxHost{name: strings.TrimPrefix(target, "HOST_"), alias: alias, capture: prefix != "."}, nil
		}
		return &rxSub{name: target, alias: alias, capture: prefix != "."}, nil
	}

	// 2. HostName: "HOST_" NameTok
	if strings.HasPrefix(bodyStr, "HOST_") {
		hName := strings.TrimPrefix(bodyStr, "HOST_")
		if prefix == "?" {
			return &rxLookahead{inner: &rxHost{name: hName, capture: false}, neg: false}, nil
		}
		if prefix == "!" {
			return &rxLookahead{inner: &rxHost{name: hName, capture: false}, neg: true}, nil
		}
		return &rxHost{name: hName, capture: prefix != "."}, nil
	}

	// 3. CharClassExpr: starts with '[', '+', '-', or ':'
	if bodyStr != "" && strings.ContainsRune("[+-:", rune(bodyStr[0])) {
		terms, err := walkCharClassExpr(body)
		if err != nil {
			return nil, err
		}
		return &rxCharClass{terms: terms, outerNeg: prefix == "!"}, nil
	}

	// 4. Plain subrule / assertion NameTok
	if bodyStr == "" {
		return nil, fmt.Errorf("raku: empty <subrule>")
	}
	if bodyStr == "wb" {
		if prefix == "!" {
			return &rxAnchor{kind: "!wb"}, nil
		}
		return &rxAnchor{kind: "wb"}, nil
	}
	if bodyStr == "ww" {
		if prefix == "!" {
			return &rxAnchor{kind: "!ww"}, nil
		}
		return &rxAnchor{kind: "ww"}, nil
	}
	if prefix == "?" {
		return &rxLookahead{inner: &rxSub{name: bodyStr, capture: false}, neg: false}, nil
	}
	if prefix == "!" {
		return &rxLookahead{inner: &rxSub{name: bodyStr, capture: false}, neg: true}, nil
	}
	return &rxSub{name: bodyStr, capture: prefix != "."}, nil
}

func walkCharClassExpr(v any) ([]signedClassTerm, error) {
	ce := asList(v)
	if len(ce) < 2 {
		return nil, fmt.Errorf("raku: invalid character class")
	}
	first, err := walkClassFirstTerm(ce[0])
	if err != nil {
		return nil, err
	}
	terms := []signedClassTerm{first}
	for _, item := range asList(ce[1]) {
		// _, ClassSetOp, _, ClassTerm
		row := asList(item)
		if len(row) < 4 {
			continue
		}
		op := byte('+')
		if joinText(row[1]) == "-" {
			op = '-'
		}
		term, err := walkClassTerm(op, row[3])
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	return terms, nil
}

func walkClassFirstTerm(v any) (signedClassTerm, error) {
	ft := asList(v)
	// Alt 1: optSign, "[", ClassInner, "]"  (len == 4 && joinText(ft[1]) == "[")
	if len(ft) == 4 && joinText(ft[1]) == "[" {
		op := byte('+')
		if joinText(ft[0]) == "-" {
			op = '-'
		}
		return signedClassTerm{
			op:          op,
			kind:        classTermBracket,
			bracketSpec: joinText(ft[2]),
		}, nil
	}
	// Alt 2: ClassSign, PropSpec  (len == 2 && (joinText(ft[0]) == "+" || joinText(ft[0]) == "-"))
	if len(ft) == 2 && (joinText(ft[0]) == "+" || joinText(ft[0]) == "-") {
		op := byte('+')
		if joinText(ft[0]) == "-" {
			op = '-'
		}
		return walkPropSpec(op, ft[1]), nil
	}
	// Alt 3: PropSpec  (":", opt"!", NameTok)
	return walkPropSpec('+', v), nil
}

func walkClassTerm(op byte, v any) (signedClassTerm, error) {
	ct := asList(v)
	if len(ct) == 3 && joinText(ct[0]) == "[" {
		return signedClassTerm{
			op:          op,
			kind:        classTermBracket,
			bracketSpec: joinText(ct[1]),
		}, nil
	}
	return walkPropSpec(op, v), nil
}

func walkPropSpec(op byte, v any) signedClassTerm {
	ps := asList(v)
	// ":", opt"!", NameTok
	neg := false
	name := ""
	if len(ps) >= 3 {
		neg = joinText(ps[1]) == "!"
		name = joinText(ps[2])
	} else {
		raw := strings.TrimPrefix(joinText(v), ":")
		if strings.HasPrefix(raw, "!") {
			neg = true
			raw = raw[1:]
		}
		name = raw
	}
	return signedClassTerm{
		op:       op,
		kind:     classTermProp,
		propName: name,
		propNeg:  neg,
	}
}

func walkRxPrimaryText(s string) (rx, error) {
	switch s {
	case ".":
		return &rxAny{}, nil
	case "^":
		return &rxAnchor{kind: "^"}, nil
	case "$":
		return &rxAnchor{kind: "$"}, nil
	case "^^":
		return &rxAnchor{kind: "^^"}, nil
	case "$$":
		return &rxAnchor{kind: "$$"}, nil
	case "<<", "«":
		return &rxAnchor{kind: "<<"}, nil
	case ">>", "»":
		return &rxAnchor{kind: ">>"}, nil
	case "#":
		return &rxLit{s: "#"}, nil
	case ";":
		return &rxLit{s: ";"}, nil
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
