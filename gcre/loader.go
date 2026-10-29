package gcre

import (
	"fmt"
	"os"
	"strings"
)

// DynamicProto is a `proto token/rule/regex name { * }` declaration.
type DynamicProto struct {
	Name    string
	IsToken bool
	Traits  RuleTraits
}

// DynamicRule is one token/rule/regex compiled from a .raku file.
type DynamicRule struct {
	Name     string
	BaseName string
	Sym      string
	HasSym   bool
	IsMulti  bool
	IsToken  bool
	Traits   RuleTraits
	Pat      rx
}

// DynamicGrammarDef is a parsed .raku grammar definition.
type DynamicGrammarDef struct {
	Name   string
	Parent string
	Protos []*DynamicProto
	Rules  []*DynamicRule
}

type protoCandidate struct {
	fullName string
	sym      string
	isToken  bool
	pat      rx
}

type protoGroup struct {
	name    string
	isToken bool
	cands   []protoCandidate
}

type opDeclSpec struct {
	fixity   Fixity
	sym      string
	ruleName string
	traits   RuleTraits
	order    int
}

// LoadGrammarFromFile reads a .raku grammar file.
func LoadGrammarFromFile(path string) (*Grammar, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading grammar file %q: %w", path, err)
	}
	return LoadGrammarFromString(string(content))
}

// LoadGrammarFromString parses one or more .raku grammars and returns the last declared grammar.
func LoadGrammarFromString(source string) (*Grammar, error) {
	return LoadGrammarWithParents(source, nil)
}

// LoadGrammarsFromString parses all grammar declarations in source and returns both the map and last grammar.
func LoadGrammarsFromString(source string) (map[string]*Grammar, *Grammar, error) {
	return loadGrammarsInternal(source, nil)
}

// LoadGrammarWithParents parses source resolving `grammar Child is Parent` against parents and earlier grammars in source.
func LoadGrammarWithParents(source string, parents map[string]*Grammar) (*Grammar, error) {
	_, last, err := loadGrammarsInternal(source, parents)
	return last, err
}

// ParseGrammarDefs parses .raku source into raw DynamicGrammarDef ASTs without compiling matchers.
func ParseGrammarDefs(source string) ([]*DynamicGrammarDef, error) {
	tree, err := Parse("grammar.raku", []byte(source))
	if err != nil {
		return nil, fmt.Errorf("raku grammar: %w", err)
	}
	return walkGrammarFile(tree)
}

func loadGrammarsInternal(source string, extParents map[string]*Grammar) (map[string]*Grammar, *Grammar, error) {
	defs, err := ParseGrammarDefs(source)
	if err != nil {
		return nil, nil, err
	}
	if len(defs) == 0 {
		return nil, nil, fmt.Errorf("no grammars found")
	}

	env := make(map[string]*Grammar, len(defs)+len(extParents))
	for k, v := range extParents {
		env[k] = v
	}

	var last *Grammar
	for _, def := range defs {
		g, err := compileGrammarDef(def, env)
		if err != nil {
			return nil, nil, err
		}
		env[g.Name] = g
		last = g
	}
	return env, last, nil
}

func compileGrammarDef(def *DynamicGrammarDef, env map[string]*Grammar) (*Grammar, error) {
	g := NewGrammar(def.Name)
	if def.Parent != "" {
		parent, ok := env[def.Parent]
		if !ok {
			return nil, fmt.Errorf("grammar %q extends unknown parent grammar %q", def.Name, def.Parent)
		}
		g.Extend(parent)
	}

	for _, pr := range def.Protos {
		if _, exists := g.protos[pr.Name]; !exists {
			g.protos[pr.Name] = &protoGroup{
				name:    pr.Name,
				isToken: pr.IsToken,
			}
		}
		if pr.Traits.IsOptable {
			g.optableRules[pr.Name] = "term"
		}
	}

	for _, r := range def.Rules {
		if r.Traits.IsOptable {
			g.optableRules[r.Name] = extractOptableTermRule(r.Pat)
			continue
		}

		ruleFn := makeSingleRuleFunc(r.IsToken, r.Sym, r.Pat)
		g.DefineRule(r.Name, ruleFn)

		if r.HasSym || r.IsMulti {
			pg, exists := g.protos[r.BaseName]
			if !exists {
				pg = &protoGroup{
					name:    r.BaseName,
					isToken: r.IsToken,
				}
				g.protos[r.BaseName] = pg
			}
			cand := protoCandidate{
				fullName: r.Name,
				sym:      r.Sym,
				isToken:  r.IsToken,
				pat:      r.Pat,
			}
			replaced := false
			for i := range pg.cands {
				if pg.cands[i].fullName == r.Name {
					pg.cands[i] = cand
					replaced = true
					break
				}
			}
			if !replaced {
				pg.cands = append(pg.cands, cand)
			}
		}

		if r.HasSym {
			if fix, ok := fixityByName(r.BaseName); ok {
				customRule := ""
				if !isSimpleSymBody(r.Pat) {
					customRule = r.Name
				}
				spec := &opDeclSpec{
					fixity:   fix,
					sym:      r.Sym,
					ruleName: customRule,
					traits:   r.Traits,
					order:    len(g.opSpecs) + 1,
				}
				replacedOp := false
				for i, existing := range g.opSpecs {
					if existing.fixity == fix && existing.sym == r.Sym {
						spec.order = existing.order
						g.opSpecs[i] = spec
						replacedOp = true
						break
					}
				}
				if !replacedOp {
					g.opSpecs = append(g.opSpecs, spec)
				}
			}
		}
	}

	g.rebuildProtosAndOptable()

	if len(g.Rules) == 0 {
		return nil, fmt.Errorf("no rules found in grammar")
	}
	return g, nil
}

func makeSingleRuleFunc(isToken bool, symVal string, pat rx) RuleFunc {
	hasBOS := startsWithBOS(pat)
	return func(g *Grammar, ctx *Context) *Match {
		if !isToken && !hasBOS {
			ctx.SkipWS()
		}
		start := ctx.Pos
		m := NewMatch("", start, start, true)
		prevSym := ctx.SymVal
		if symVal != "" {
			ctx.SymVal = symVal
		}
		ok := pat != nil && pat.match(g, ctx, m, isToken)
		ctx.SymVal = prevSym
		if !ok {
			ctx.Pos = start
			return NewMatch("", start, start, false)
		}
		m.To = ctx.Pos
		m.Str = string(ctx.Src[start:ctx.Pos])
		m.Ok = true
		return m
	}
}

func (g *Grammar) rebuildProtosAndOptable() {
	for protoName, pg := range g.protos {
		pgCopy := pg
		g.DefineRule(protoName, func(g *Grammar, ctx *Context) *Match {
			startPos := ctx.Pos
			var bestCand *protoCandidate
			var bestMatch *Match
			bestEnd := -1

			for i := range pgCopy.cands {
				cand := &pgCopy.cands[i]
				ctx.Pos = startPos
				if !cand.isToken && !startsWithBOS(cand.pat) {
					ctx.SkipWS()
				}
				candStart := ctx.Pos
				m := NewMatch("", candStart, candStart, true)
				prevSym := ctx.SymVal
				ctx.SymVal = cand.sym
				ok := cand.pat != nil && cand.pat.match(g, ctx, m, cand.isToken)
				ctx.SymVal = prevSym
				if ok && ctx.Pos > bestEnd {
					bestEnd = ctx.Pos
					m.To = ctx.Pos
					m.Str = string(ctx.Src[candStart:ctx.Pos])
					m.Ok = true
					bestCand = cand
					bestMatch = m
				}
			}

			if bestMatch != nil && bestCand != nil {
				ctx.Pos = bestEnd
				bestMatch.Rule = bestCand.fullName
				if bestCand.sym != "" {
					if _, has := bestMatch.Named["sym"]; !has {
						bestMatch.AddNamed("sym", NewMatch(bestCand.sym, bestMatch.From, bestMatch.From, true))
					}
				}
				return bestMatch
			}
			ctx.Pos = startPos
			return NewMatch("", startPos, startPos, false)
		})
	}

	if len(g.opSpecs) > 0 {
		g.OpTable = buildOpTableFromSpecs(g.opSpecs)
	}

	for ruleName, termRule := range g.optableRules {
		tRule := termRule
		g.DefineRule(ruleName, func(g *Grammar, ctx *Context) *Match {
			return g.EXPR(ctx, tRule, g.OpTable)
		})
	}
}

func fixityByName(base string) (Fixity, bool) {
	switch base {
	case "infix":
		return FixityInfix, true
	case "prefix":
		return FixityPrefix, true
	case "postfix":
		return FixityPostfix, true
	default:
		return 0, false
	}
}

func isSimpleSymBody(pat rx) bool {
	sub, ok := pat.(*rxSub)
	return ok && sub.name == "sym" && sub.alias == ""
}

func extractOptableTermRule(pat rx) string {
	if sub, ok := pat.(*rxSub); ok && sub.name != "" {
		return sub.name
	}
	return "term"
}

func parseAssocTrait(s string, fix Fixity) Assoc {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "right":
		return AssocRight
	case "non":
		return AssocNon
	case "chain":
		return AssocChain
	case "left":
		return AssocLeft
	default:
		if fix == FixityPrefix {
			return AssocRight
		}
		return AssocLeft
	}
}

func buildOpTableFromSpecs(specs []*opDeclSpec) *OpTable {
	type opKey struct {
		fix Fixity
		sym string
	}
	prec := make(map[opKey]int, len(specs))
	resolved := make(map[opKey]bool, len(specs))

	for _, sp := range specs {
		k := opKey{sp.fixity, sp.sym}
		if sp.traits.Tighter == nil && sp.traits.Looser == nil && sp.traits.Equiv == nil {
			switch sp.fixity {
			case FixityInfix:
				prec[k] = 10000
			case FixityPrefix:
				prec[k] = 20000
			case FixityPostfix:
				prec[k] = 30000
			}
			resolved[k] = true
		}
	}

	lookupRef := func(ref *TraitOpRef, preferMax bool) (int, bool) {
		if ref == nil {
			return 0, false
		}
		if ref.HasFixity {
			k := opKey{ref.Fixity, ref.Symbol}
			if resolved[k] {
				return prec[k], true
			}
			return 0, false
		}
		found := false
		best := 0
		for _, f := range []Fixity{FixityInfix, FixityPrefix, FixityPostfix} {
			k := opKey{f, ref.Symbol}
			if resolved[k] {
				if !found || (preferMax && prec[k] > best) || (!preferMax && prec[k] < best) {
					best = prec[k]
					found = true
				}
			}
		}
		return best, found
	}

	for pass := 0; pass <= len(specs)+2; pass++ {
		progressed := false
		for _, sp := range specs {
			k := opKey{sp.fixity, sp.sym}
			if resolved[k] {
				continue
			}
			if sp.traits.Equiv != nil {
				if v, ok := lookupRef(sp.traits.Equiv, true); ok {
					prec[k] = v
					resolved[k] = true
					progressed = true
					continue
				}
			}
			if sp.traits.Tighter != nil && sp.traits.Looser != nil {
				lo, ok1 := lookupRef(sp.traits.Tighter, true)
				hi, ok2 := lookupRef(sp.traits.Looser, false)
				if ok1 && ok2 {
					prec[k] = (lo + hi) / 2
					resolved[k] = true
					progressed = true
					continue
				}
			} else if sp.traits.Tighter != nil {
				if lo, ok := lookupRef(sp.traits.Tighter, true); ok {
					prec[k] = lo + 100
					resolved[k] = true
					progressed = true
					continue
				}
			} else if sp.traits.Looser != nil {
				if hi, ok := lookupRef(sp.traits.Looser, false); ok {
					prec[k] = hi - 100
					resolved[k] = true
					progressed = true
					continue
				}
			}
		}
		if !progressed {
			// Anchor any remaining unresolved root cycles at default base and retry once
			anchored := false
			for _, sp := range specs {
				k := opKey{sp.fixity, sp.sym}
				if !resolved[k] {
					prec[k] = 10000
					resolved[k] = true
					anchored = true
					break
				}
			}
			if !anchored {
				break
			}
		}
	}

	table := NewOpTable()
	for _, sp := range specs {
		k := opKey{sp.fixity, sp.sym}
		p := prec[k]
		assoc := parseAssocTrait(sp.traits.Assoc, sp.fixity)
		op := &Operator{
			Symbol:   sp.sym,
			Fixity:   sp.fixity,
			Prec:     p,
			Assoc:    assoc,
			RuleName: sp.ruleName,
			Order:    sp.order,
		}
		switch sp.fixity {
		case FixityPrefix:
			table.Prefix[sp.sym] = op
		case FixityInfix:
			table.Infix[sp.sym] = op
		case FixityPostfix:
			table.Postfix[sp.sym] = op
		}
	}
	return table
}

// --- runtime regex / QRegex matchers ---

type rx interface {
	match(g *Grammar, ctx *Context, cap *Match, token bool) bool
}

type rxNoop struct{}

type rxAlt struct {
	alts       []rx
	sequential bool // true for '||'
}

type rxLTMAlt struct {
	alts []rx
}

type rxSeq struct {
	elts []rx
}

type rxLit struct {
	s string
}

type rxAny struct{}

type rxAnchor struct {
	kind string // "^", "$", "^^", "$$", "<<", ">>", "wb", "!wb", "ww", "!ww"
}

type rxLookahead struct {
	inner rx
	neg   bool
}

type rxLookbehind struct {
	inner rx
	neg   bool
}

type classTermKind int

const (
	classTermBracket classTermKind = iota
	classTermProp
)

type signedClassTerm struct {
	op          byte // '+' or '-'
	kind        classTermKind
	bracketSpec string
	propName    string
	propNeg     bool
}

type rxCharClass struct {
	esc      rune // non-zero for \d, \D, \s, \S, \w, \W, \h, \H, \v, \V, \n, \N, \t, \T, \r, \R
	terms    []signedClassTerm
	outerNeg bool
}

type rxSub struct {
	name    string
	alias   string
	capture bool
}

type rxGrp struct {
	inner rx
}

type rxCap struct {
	inner rx
}

type rxAlias struct {
	name  string
	inner rx
}

type rxGoal struct {
	open     rx
	goal     rx
	inner    rx
	goalDesc string
}

type rxQuant struct {
	inner rx
	q     quantSpec
}

func startsWithBOS(p rx) bool {
	switch t := p.(type) {
	case *rxAnchor:
		return t.kind == "^" || t.kind == "^^"
	case *rxSeq:
		return len(t.elts) > 0 && startsWithBOS(t.elts[0])
	default:
		return false
	}
}

func noPreWS(p rx) bool {
	switch t := p.(type) {
	case *rxNoop, *rxLookbehind:
		return true
	case *rxAnchor:
		switch t.kind {
		case "^", "^^", "$$", ">>", "wb", "!wb", "ww", "!ww":
			return true
		}
		return false
	default:
		return false
	}
}

func (r *rxNoop) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	return true
}

func (r *rxAlt) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	saved := ctx.Pos
	snap := cap.snapshot()
	for _, a := range r.alts {
		ctx.Pos = saved
		cap.restore(snap)
		if a.match(g, ctx, cap, token) {
			return true
		}
	}
	ctx.Pos = saved
	cap.restore(snap)
	return false
}

func (r *rxLTMAlt) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	saved := ctx.Pos
	snap := cap.snapshot()

	bestIdx := -1
	bestEnd := -1

	// Probe all branches to find longest match (ties broken by declaration order).
	for i, a := range r.alts {
		ctx.Pos = saved
		if a.match(g, ctx, nil, token) {
			if ctx.Pos > bestEnd {
				bestEnd = ctx.Pos
				bestIdx = i
			}
		}
	}
	if bestIdx < 0 {
		ctx.Pos = saved
		cap.restore(snap)
		return false
	}
	if cap == nil {
		ctx.Pos = bestEnd
		return true
	}
	// Replay winning branch with captures enabled.
	ctx.Pos = saved
	cap.restore(snap)
	if !r.alts[bestIdx].match(g, ctx, cap, token) {
		ctx.Pos = saved
		cap.restore(snap)
		return false
	}
	return true
}

func (r *rxSeq) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	return r.matchFrom(0, g, ctx, cap, token)
}

func (r *rxSeq) matchFrom(idx int, g *Grammar, ctx *Context, cap *Match, token bool) bool {
	for idx < len(r.elts) {
		e := r.elts[idx]
		if rq, ok := e.(*rxQuant); ok && rq.q.frugal {
			return rq.matchFrugalInSeq(g, ctx, cap, token, func() bool {
				return r.matchFrom(idx+1, g, ctx, cap, token)
			})
		}
		if !token && !noPreWS(e) {
			ctx.SkipWS()
		}
		if !e.match(g, ctx, cap, token) {
			return false
		}
		idx++
	}
	return true
}

func (r *rxLit) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if !token {
		ctx.SkipWS()
	}
	return g.ExactLit(r.s, ctx).Ok
}

func (r *rxAny) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if !token {
		ctx.SkipWS()
	}
	if ctx.Pos >= len(ctx.Src) {
		return false
	}
	ctx.Pos++
	return true
}

func (r *rxAnchor) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	switch r.kind {
	case "^":
		return ctx.Pos == 0
	case "$":
		if !token {
			ctx.SkipWS()
		}
		return ctx.Pos == len(ctx.Src)
	case "^^":
		return ctx.Pos == 0 || ctx.Src[ctx.Pos-1] == '\n'
	case "$$":
		if !token {
			ctx.SkipHorizontalWS()
		}
		return ctx.Pos == len(ctx.Src) || ctx.Src[ctx.Pos] == '\n' || ctx.Src[ctx.Pos] == '\r'
	case "<<":
		if !token {
			ctx.SkipWS()
		}
		prevW := ctx.Pos > 0 && isWordRune(ctx.Src[ctx.Pos-1])
		currW := ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos])
		return !prevW && currW
	case ">>":
		prevW := ctx.Pos > 0 && isWordRune(ctx.Src[ctx.Pos-1])
		currW := ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos])
		return prevW && !currW
	case "wb":
		prevW := ctx.Pos > 0 && isWordRune(ctx.Src[ctx.Pos-1])
		currW := ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos])
		return prevW != currW
	case "!wb":
		prevW := ctx.Pos > 0 && isWordRune(ctx.Src[ctx.Pos-1])
		currW := ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos])
		return prevW == currW
	case "ww":
		prevW := ctx.Pos > 0 && isWordRune(ctx.Src[ctx.Pos-1])
		currW := ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos])
		return prevW && currW
	case "!ww":
		prevW := ctx.Pos > 0 && isWordRune(ctx.Src[ctx.Pos-1])
		currW := ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos])
		return !(prevW && currW)
	}
	return false
}

func (r *rxLookahead) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	saved := ctx.Pos
	savedGoal := ctx.GoalErr
	ok := r.inner.match(g, ctx, nil, token)
	ctx.Pos = saved
	ctx.GoalErr = savedGoal
	if r.neg {
		return !ok
	}
	return ok
}

func (r *rxLookbehind) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	endPos := ctx.Pos
	var matched bool
	if lit, ok := r.inner.(*rxLit); ok {
		matched = g.After(lit.s, ctx).Ok
	} else {
		subCtx := &Context{
			Src:    ctx.Src[:endPos],
			Host:   ctx.Host,
			SymVal: ctx.SymVal,
		}
		for start := endPos; start >= 0; start-- {
			subCtx.Pos = start
			if r.inner.match(g, subCtx, nil, true) && subCtx.Pos == endPos {
				matched = true
				break
			}
		}
	}
	if r.neg {
		return !matched
	}
	return matched
}

func (r *rxCharClass) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if !token {
		ctx.SkipWS()
	}
	if ctx.Pos >= len(ctx.Src) {
		return false
	}
	ch := ctx.Src[ctx.Pos]
	if r.esc != 0 {
		if _, ok := matchEscClass(r.esc, ch); ok {
			ctx.Pos++
			return true
		}
		return false
	}
	inSet := len(r.terms) > 0 && r.terms[0].op == '-'
	for _, t := range r.terms {
		var hit bool
		if t.kind == classTermBracket {
			hit = classContains(t.bracketSpec, ch)
		} else {
			hit = matchUnicodeProp(t.propName, t.propNeg, ch)
		}
		if hit {
			inSet = (t.op == '+')
		}
	}
	if r.outerNeg {
		inSet = !inSet
	}
	if inSet {
		ctx.Pos++
		return true
	}
	return false
}

func (r *rxSub) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if !token {
		ctx.SkipWS()
	}
	m := g.Subrule(r.name, ctx)
	if !m.Ok {
		return false
	}
	if r.capture && cap != nil {
		capName := r.name
		if r.alias != "" {
			capName = r.alias
		}
		cap.AddNamed(capName, m)
	}
	return true
}

func (r *rxGrp) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	return r.inner.match(g, ctx, cap, token)
}

func (r *rxCap) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if !token && !startsWithBOS(r.inner) {
		ctx.SkipWS()
	}
	start := ctx.Pos
	sub := NewMatch("", start, start, true)
	if !r.inner.match(g, ctx, sub, token) {
		ctx.Pos = start
		return false
	}
	sub.To = ctx.Pos
	sub.Str = string(ctx.Src[start:ctx.Pos])
	sub.Ok = true
	if cap != nil {
		cap.AddPositional(sub)
	}
	return true
}

func (r *rxAlias) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if !token && !startsWithBOS(r.inner) {
		ctx.SkipWS()
	}
	start := ctx.Pos
	sub := NewMatch("", start, start, true)
	if !r.inner.match(g, ctx, sub, token) {
		ctx.Pos = start
		return false
	}
	sub.To = ctx.Pos
	sub.Str = string(ctx.Src[start:ctx.Pos])
	sub.Ok = true
	if cap != nil {
		cap.AddNamed(r.name, sub)
	}
	return true
}

func (r *rxGoal) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if !token && !noPreWS(r.open) {
		ctx.SkipWS()
	}
	if !r.open.match(g, ctx, cap, token) {
		return false
	}
	if !token && !noPreWS(r.inner) {
		ctx.SkipWS()
	}
	if !r.inner.match(g, ctx, cap, token) {
		if ctx.GoalErr == "" {
			ctx.GoalErr = fmt.Sprintf("unable to parse expression before goal %s at pos %d", r.goalDesc, ctx.Pos)
		}
		return false
	}
	if !token && !noPreWS(r.goal) {
		ctx.SkipWS()
	}
	if !r.goal.match(g, ctx, cap, token) {
		ctx.GoalErr = fmt.Sprintf("unable to find goal %s at pos %d", r.goalDesc, ctx.Pos)
		return false
	}
	return true
}

func (r *rxQuant) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	if r.q.frugal {
		return r.matchFrugalInSeq(g, ctx, cap, token, func() bool { return true })
	}
	if r.q.sep != nil {
		return r.matchSeparatedGreedy(g, ctx, cap, token)
	}

	initPos := ctx.Pos
	initSnap := cap.snapshot()
	count := 0

	for r.q.max < 0 || count < r.q.max {
		iterPos := ctx.Pos
		iterSnap := cap.snapshot()
		if !token && !noPreWS(r.inner) {
			ctx.SkipWS()
		}
		if !r.inner.match(g, ctx, cap, token) {
			ctx.Pos = iterPos
			cap.restore(iterSnap)
			break
		}
		count++
		if ctx.Pos <= iterPos && count >= r.q.min {
			// Guard against infinite zero-width repetition once minimum count is met
			break
		}
	}

	if count < r.q.min {
		ctx.Pos = initPos
		cap.restore(initSnap)
		return false
	}
	return true
}

func (r *rxQuant) matchSeparatedGreedy(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	initPos := ctx.Pos
	initSnap := cap.snapshot()

	if !token && !noPreWS(r.inner) {
		ctx.SkipWS()
	}
	if !r.inner.match(g, ctx, cap, token) {
		ctx.Pos = initPos
		cap.restore(initSnap)
		return r.q.min == 0
	}

	count := 1
	for r.q.max < 0 || count < r.q.max {
		beforeSepPos := ctx.Pos
		beforeSepSnap := cap.snapshot()

		if !token && !noPreWS(r.q.sep) {
			ctx.SkipWS()
		}
		if !r.q.sep.match(g, ctx, cap, token) {
			ctx.Pos = beforeSepPos
			cap.restore(beforeSepSnap)
			break
		}

		afterSepPos := ctx.Pos
		afterSepSnap := cap.snapshot()

		if !token && !noPreWS(r.inner) {
			ctx.SkipWS()
		}
		if r.inner.match(g, ctx, cap, token) && ctx.Pos > beforeSepPos {
			count++
			continue
		}

		// Item after separator failed:
		if r.q.sepTrail && count >= r.q.min {
			ctx.Pos = afterSepPos
			cap.restore(afterSepSnap)
		} else {
			ctx.Pos = beforeSepPos
			cap.restore(beforeSepSnap)
		}
		break
	}

	if count < r.q.min {
		ctx.Pos = initPos
		cap.restore(initSnap)
		return false
	}
	return true
}

func (r *rxQuant) matchFrugalInSeq(g *Grammar, ctx *Context, cap *Match, token bool, rest func() bool) bool {
	initPos := ctx.Pos
	initSnap := cap.snapshot()
	count := 0

	// 1. Satisfy minimum required repetitions first
	for count < r.q.min {
		if count > 0 && r.q.sep != nil {
			if !token && !noPreWS(r.q.sep) {
				ctx.SkipWS()
			}
			if !r.q.sep.match(g, ctx, cap, token) {
				ctx.Pos = initPos
				cap.restore(initSnap)
				return false
			}
		}
		if !token && !noPreWS(r.inner) {
			ctx.SkipWS()
		}
		if !r.inner.match(g, ctx, cap, token) {
			ctx.Pos = initPos
			cap.restore(initSnap)
			return false
		}
		count++
	}

	// 2. Try matching the rest of the sequence; advance minimally on failure
	for {
		tryPos := ctx.Pos
		trySnap := cap.snapshot()
		if rest() {
			return true
		}
		ctx.Pos = tryPos
		cap.restore(trySnap)

		if r.q.max >= 0 && count >= r.q.max {
			ctx.Pos = initPos
			cap.restore(initSnap)
			return false
		}

		stepPos := ctx.Pos
		if count > 0 && r.q.sep != nil {
			if !token && !noPreWS(r.q.sep) {
				ctx.SkipWS()
			}
			if !r.q.sep.match(g, ctx, cap, token) {
				ctx.Pos = initPos
				cap.restore(initSnap)
				return false
			}
		}
		if !token && !noPreWS(r.inner) {
			ctx.SkipWS()
		}
		if !r.inner.match(g, ctx, cap, token) || ctx.Pos <= stepPos {
			ctx.Pos = initPos
			cap.restore(initSnap)
			return false
		}
		count++
	}
}
