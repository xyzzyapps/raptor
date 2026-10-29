package gcre

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// Match represents a Perl 6 / Raku / NQP match object ($/).
type Match struct {
	Str   string
	From  int
	To    int
	Ok    bool
	Rule  string // Specific rule/sym branch that produced this match (e.g. "stmt:sym<let>")
	Named map[string]*Match
	List  map[string][]*Match
	Pos   []*Match
	Made  any // Value set by 'make' ($/.made / $/.ast)
}

type matchSnap struct {
	posLen int
	made   any
	named  map[string]*Match
	list   map[string]int
}

func (m *Match) snapshot() matchSnap {
	if m == nil {
		return matchSnap{}
	}
	s := matchSnap{
		posLen: len(m.Pos),
		made:   m.Made,
	}
	if len(m.Named) > 0 {
		s.named = make(map[string]*Match, len(m.Named))
		for k, v := range m.Named {
			s.named[k] = v
		}
	}
	if len(m.List) > 0 {
		s.list = make(map[string]int, len(m.List))
		for k, v := range m.List {
			s.list[k] = len(v)
		}
	}
	return s
}

func (m *Match) restore(s matchSnap) {
	if m == nil {
		return
	}
	m.Pos = m.Pos[:s.posLen]
	m.Made = s.made
	if s.named == nil {
		for k := range m.Named {
			delete(m.Named, k)
		}
	} else {
		for k := range m.Named {
			if prev, ok := s.named[k]; ok {
				m.Named[k] = prev
			} else {
				delete(m.Named, k)
			}
		}
		for k, v := range s.named {
			m.Named[k] = v
		}
	}
	if s.list == nil {
		for k := range m.List {
			delete(m.List, k)
		}
	} else {
		for k := range m.List {
			if n, ok := s.list[k]; ok {
				m.List[k] = m.List[k][:n]
			} else {
				delete(m.List, k)
			}
		}
	}
}

func NewMatch(str string, from int, to int, ok bool) *Match {
	return &Match{
		Str:   str,
		From:  from,
		To:    to,
		Ok:    ok,
		Named: make(map[string]*Match),
		List:  make(map[string][]*Match),
	}
}

// Make sets the AST node / payload on the match object.
func (m *Match) Make(val any) {
	m.Made = val
}

// Get returns a single named capture match (the latest match if repeated, or first in List).
func (m *Match) Get(name string) *Match {
	if m.Named != nil {
		if sub, ok := m.Named[name]; ok {
			return sub
		}
	}
	if m.List != nil {
		if list, ok := m.List[name]; ok && len(list) > 0 {
			return list[0]
		}
	}
	return NewMatch("", 0, 0, false)
}

// GetAll returns all matches for a repeated named subrule.
func (m *Match) GetAll(name string) []*Match {
	if m.List != nil {
		if list, ok := m.List[name]; ok {
			return list
		}
	}
	if m.Named != nil {
		if sub, ok := m.Named[name]; ok {
			return []*Match{sub}
		}
	}
	return nil
}

// AddNamed records a named subrule match.
func (m *Match) AddNamed(name string, sub *Match) {
	if m.Named == nil {
		m.Named = make(map[string]*Match)
	}
	m.Named[name] = sub
	if m.List == nil {
		m.List = make(map[string][]*Match)
	}
	m.List[name] = append(m.List[name], sub)
}

// AddPositional records a positional capture match ($0, $1, $2...).
func (m *Match) AddPositional(sub *Match) {
	m.Pos = append(m.Pos, sub)
}

// At returns the positional capture at index ($0, $1, ...).
func (m *Match) At(index int) *Match {
	if index >= 0 && index < len(m.Pos) {
		return m.Pos[index]
	}
	return NewMatch("", 0, 0, false)
}

// ActionHandler is an optional interface that action objects may implement to
// receive dynamic rule names including NQP :sym<...> variants (e.g. "infix:sym<+>").
type ActionHandler interface {
	HandleAction(ruleName string, m *Match)
}

// RuleFunc is the function signature for a grammar rule / token match function.
type RuleFunc func(g *Grammar, ctx *Context) *Match

// Context manages current position, source text, and action handler.
type Context struct {
	Src     []rune
	Pos     int
	Actions any
	// Host is an opaque pointer for HOST_* matchers (e.g. a shared token cursor).
	Host any
	// SymVal holds the active :sym<...> symbol string when inside a multi/sym rule.
	SymVal string
	// GoalErr holds the diagnostic message from a failed '~' goal expectation or non-associative op.
	GoalErr string
}

func (c *Context) IsAtEnd() bool {
	return c.Pos >= len(c.Src)
}

func (c *Context) Peek() rune {
	if c.Pos < len(c.Src) {
		return c.Src[c.Pos]
	}
	return 0
}

func (c *Context) Advance() rune {
	if c.Pos < len(c.Src) {
		r := c.Src[c.Pos]
		c.Pos++
		return r
	}
	return 0
}

func (c *Context) SkipWS() {
	for c.Pos < len(c.Src) && unicode.IsSpace(c.Src[c.Pos]) {
		c.Pos++
	}
}

func (c *Context) SkipHorizontalWS() {
	for c.Pos < len(c.Src) && isHorizontalSpace(c.Src[c.Pos]) {
		c.Pos++
	}
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isHorizontalSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\u00A0' || (r != '\n' && r != '\r' && unicode.Is(unicode.Zs, r))
}

func isVerticalSpace(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', '\u0085', '\u2028', '\u2029':
		return true
	}
	return false
}

// Grammar is a Perl 6 / Raku / NQP grammar definition.
type Grammar struct {
	Name         string
	Parent       string
	Rules        map[string]RuleFunc
	OpTable      *OpTable
	protos       map[string]*protoGroup
	opSpecs      []*opDeclSpec
	optableRules map[string]string // ruleName -> termRule
}

func NewGrammar(name string) *Grammar {
	return &Grammar{
		Name:         name,
		Rules:        make(map[string]RuleFunc),
		protos:       make(map[string]*protoGroup),
		optableRules: make(map[string]string),
	}
}

// Extend inherits rules, protoregex candidates, and optable declarations from parent.
func (g *Grammar) Extend(parent *Grammar) {
	if parent == nil {
		return
	}
	g.Parent = parent.Name
	for k, fn := range parent.Rules {
		if _, exists := g.Rules[k]; !exists {
			g.Rules[k] = fn
		}
	}
	for k, pg := range parent.protos {
		cloned := &protoGroup{
			name:    pg.name,
			isToken: pg.isToken,
			cands:   make([]protoCandidate, len(pg.cands)),
		}
		copy(cloned.cands, pg.cands)
		g.protos[k] = cloned
	}
	if len(parent.opSpecs) > 0 {
		base := make([]*opDeclSpec, len(parent.opSpecs))
		for i, sp := range parent.opSpecs {
			cp := *sp
			base[i] = &cp
		}
		g.opSpecs = append(base, g.opSpecs...)
	}
	for k, term := range parent.optableRules {
		if _, exists := g.optableRules[k]; !exists {
			g.optableRules[k] = term
		}
	}
	g.rebuildProtosAndOptable()
}

func (g *Grammar) DefineRule(name string, fn RuleFunc) {
	g.Rules[name] = fn
}

// Parse runs the TOP rule of the grammar and invokes action methods.
func (g *Grammar) Parse(input string, actions any) (*Match, error) {
	return g.ParseRule("TOP", input, actions)
}

// Subparse runs TOP without requiring full input consumption (identical entry to Parse unless TOP anchors with $).
func (g *Grammar) Subparse(input string, actions any) (*Match, error) {
	return g.ParseRule("TOP", input, actions)
}

// ParseRule runs a named rule of the grammar on input and invokes action methods.
func (g *Grammar) ParseRule(ruleName string, input string, actions any) (*Match, error) {
	fn, ok := g.Rules[ruleName]
	if !ok {
		return nil, fmt.Errorf("grammar %q has no %s rule", g.Name, ruleName)
	}

	ctx := &Context{
		Src:     []rune(input),
		Pos:     0,
		Actions: actions,
	}

	m := g.invokeRule(ruleName, fn, ctx)
	if m == nil || !m.Ok {
		if ctx.GoalErr != "" {
			return nil, fmt.Errorf("grammar %q parse failed: %s", g.Name, ctx.GoalErr)
		}
		return nil, fmt.Errorf("grammar %q parse failed", g.Name)
	}

	return m, nil
}

func (g *Grammar) invokeRule(name string, fn RuleFunc, ctx *Context) *Match {
	startPos := ctx.Pos
	m := fn(g, ctx)
	if m != nil && m.Ok {
		if m.Rule == "" {
			m.Rule = name
		}
		if ctx.Actions != nil {
			g.callAction(m.Rule, m, ctx.Actions)
		}
		return m
	}
	ctx.Pos = startPos
	return NewMatch("", startPos, startPos, false)
}

func (g *Grammar) callAction(name string, m *Match, actions any) {
	if actions == nil {
		return
	}
	if ah, ok := actions.(ActionHandler); ok {
		ah.HandleAction(name, m)
		return
	}
	val := reflect.ValueOf(actions)
	if !val.IsValid() {
		return
	}
	candidates := []string{name}
	if len(name) > 0 {
		candidates = append(candidates, strings.ToUpper(name[:1])+name[1:], strings.ToUpper(name))
	}
	if idx := strings.Index(name, ":sym<"); idx > 0 {
		base := name[:idx]
		sym := strings.TrimSuffix(name[idx+5:], ">")
		sanitized := sanitizeIdent(base) + "_sym_" + sanitizeIdent(sym)
		if len(sanitized) > 0 {
			candidates = append(candidates, sanitized, strings.ToUpper(sanitized[:1])+sanitized[1:])
		}
		candidates = append(candidates, base, strings.ToUpper(base[:1])+base[1:], strings.ToUpper(base))
	} else if strings.ContainsRune(name, '-') {
		san := strings.ReplaceAll(name, "-", "_")
		candidates = append(candidates, san, strings.ToUpper(san[:1])+san[1:])
	}
	for _, cand := range candidates {
		method := val.MethodByName(cand)
		if method.IsValid() && method.Type().NumIn() == 1 {
			method.Call([]reflect.Value{reflect.ValueOf(m)})
			return
		}
	}
}

func sanitizeIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			fmt.Fprintf(&b, "x%x", r)
		}
	}
	return b.String()
}

// Subrule helper: invokes a named rule from the grammar, or a built-in NQP assertion/character rule.
func (g *Grammar) Subrule(name string, ctx *Context) *Match {
	if name == "sym" && ctx.SymVal != "" {
		return g.ExactLit(ctx.SymVal, ctx)
	}
	if fn, ok := g.Rules[name]; ok {
		return g.invokeRule(name, fn, ctx)
	}
	return g.builtinSubrule(name, ctx)
}

func (g *Grammar) builtinSubrule(name string, ctx *Context) *Match {
	start := ctx.Pos
	switch name {
	case "ident":
		if ctx.Pos < len(ctx.Src) && (unicode.IsLetter(ctx.Src[ctx.Pos]) || ctx.Src[ctx.Pos] == '_') {
			ctx.Pos++
			for ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos]) {
				ctx.Pos++
			}
			return NewMatch(string(ctx.Src[start:ctx.Pos]), start, ctx.Pos, true)
		}
		return NewMatch("", start, start, false)
	case "alpha":
		if ctx.Pos < len(ctx.Src) && (unicode.IsLetter(ctx.Src[ctx.Pos]) || ctx.Src[ctx.Pos] == '_') {
			r := ctx.Src[ctx.Pos]
			ctx.Pos++
			return NewMatch(string(r), start, ctx.Pos, true)
		}
		return NewMatch("", start, start, false)
	case "digit":
		return g.Digit(ctx)
	case "alnum":
		if ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos]) {
			r := ctx.Src[ctx.Pos]
			ctx.Pos++
			return NewMatch(string(r), start, ctx.Pos, true)
		}
		return NewMatch("", start, start, false)
	case "xdigit":
		if ctx.Pos < len(ctx.Src) && isHexDigit(ctx.Src[ctx.Pos]) {
			r := ctx.Src[ctx.Pos]
			ctx.Pos++
			return NewMatch(string(r), start, ctx.Pos, true)
		}
		return NewMatch("", start, start, false)
	case "upper":
		if ctx.Pos < len(ctx.Src) && unicode.IsUpper(ctx.Src[ctx.Pos]) {
			r := ctx.Src[ctx.Pos]
			ctx.Pos++
			return NewMatch(string(r), start, ctx.Pos, true)
		}
		return NewMatch("", start, start, false)
	case "lower":
		if ctx.Pos < len(ctx.Src) && unicode.IsLower(ctx.Src[ctx.Pos]) {
			r := ctx.Src[ctx.Pos]
			ctx.Pos++
			return NewMatch(string(r), start, ctx.Pos, true)
		}
		return NewMatch("", start, start, false)
	case "space":
		return g.Space(ctx)
	case "blank":
		return g.Blank(ctx)
	case "cntrl":
		if ctx.Pos < len(ctx.Src) && unicode.IsControl(ctx.Src[ctx.Pos]) {
			r := ctx.Src[ctx.Pos]
			ctx.Pos++
			return NewMatch(string(r), start, ctx.Pos, true)
		}
		return NewMatch("", start, start, false)
	case "punct":
		if ctx.Pos < len(ctx.Src) && unicode.IsPunct(ctx.Src[ctx.Pos]) {
			r := ctx.Src[ctx.Pos]
			ctx.Pos++
			return NewMatch(string(r), start, ctx.Pos, true)
		}
		return NewMatch("", start, start, false)
	case "ws":
		// NQP default token ws: <!ww> \s*
		prevW := ctx.Pos > 0 && isWordRune(ctx.Src[ctx.Pos-1])
		currW := ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos])
		if prevW && currW {
			return NewMatch("", start, start, false)
		}
		ctx.SkipWS()
		return NewMatch(string(ctx.Src[start:ctx.Pos]), start, ctx.Pos, true)
	case "wb":
		prevW := ctx.Pos > 0 && isWordRune(ctx.Src[ctx.Pos-1])
		currW := ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos])
		if prevW != currW {
			return NewMatch("", start, start, true)
		}
		return NewMatch("", start, start, false)
	case "ww":
		prevW := ctx.Pos > 0 && isWordRune(ctx.Src[ctx.Pos-1])
		currW := ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos])
		if prevW && currW {
			return NewMatch("", start, start, true)
		}
		return NewMatch("", start, start, false)
	}
	return NewMatch("", start, start, false)
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// Lit matches exact literal string after skipping leading whitespace.
func (g *Grammar) Lit(lit string, ctx *Context) *Match {
	ctx.SkipWS()
	return g.ExactLit(lit, ctx)
}

// ExactLit matches literal string without skipping whitespace.
func (g *Grammar) ExactLit(lit string, ctx *Context) *Match {
	start := ctx.Pos
	runes := []rune(lit)
	for _, r := range runes {
		if ctx.Pos >= len(ctx.Src) || ctx.Src[ctx.Pos] != r {
			ctx.Pos = start
			return NewMatch("", start, start, false)
		}
		ctx.Pos++
	}
	return NewMatch(lit, start, ctx.Pos, true)
}

// Ident matches a Perl 6 identifier (\w+) after skipping whitespace.
func (g *Grammar) Ident(ctx *Context) *Match {
	ctx.SkipWS()
	start := ctx.Pos
	var sb strings.Builder
	for ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos]) {
		sb.WriteRune(ctx.Src[ctx.Pos])
		ctx.Pos++
	}
	s := sb.String()
	if len(s) == 0 {
		return NewMatch("", start, start, false)
	}
	return NewMatch(s, start, ctx.Pos, true)
}

// Number matches integer or float literal.
func (g *Grammar) Number(ctx *Context) *Match {
	ctx.SkipWS()
	start := ctx.Pos
	var sb strings.Builder
	for ctx.Pos < len(ctx.Src) && (unicode.IsDigit(ctx.Src[ctx.Pos]) || ctx.Src[ctx.Pos] == '.') {
		sb.WriteRune(ctx.Src[ctx.Pos])
		ctx.Pos++
	}
	s := sb.String()
	if len(s) == 0 {
		return NewMatch("", start, start, false)
	}
	return NewMatch(s, start, ctx.Pos, true)
}

// StringLit matches single or double quoted strings.
func (g *Grammar) StringLit(ctx *Context) *Match {
	ctx.SkipWS()
	start := ctx.Pos
	if ctx.Pos >= len(ctx.Src) {
		return NewMatch("", start, start, false)
	}
	quote := ctx.Src[ctx.Pos]
	if quote != '"' && quote != '\'' {
		return NewMatch("", start, start, false)
	}
	ctx.Pos++
	var sb strings.Builder
	for ctx.Pos < len(ctx.Src) {
		ch := ctx.Src[ctx.Pos]
		if ch == quote {
			ctx.Pos++
			return NewMatch(sb.String(), start, ctx.Pos, true)
		}
		if ch == '\\' && ctx.Pos+1 < len(ctx.Src) {
			ctx.Pos++
			sb.WriteRune(ctx.Src[ctx.Pos])
			ctx.Pos++
			continue
		}
		sb.WriteRune(ch)
		ctx.Pos++
	}
	ctx.Pos = start
	return NewMatch("", start, start, false)
}

// EXPR invokes the Operator Precedence Parser with the given term rule and operator table.
func (g *Grammar) EXPR(ctx *Context, termRule string, table *OpTable) *Match {
	if table == nil {
		if g.OpTable != nil {
			table = g.OpTable
		} else {
			table = DefaultRakuOpTable()
		}
	}
	return g.ParseEXPR(ctx, termRule, table, 0)
}

// Before implements positive lookahead: <?before <rule>>
func (g *Grammar) Before(rule string, ctx *Context) *Match {
	saved := ctx.Pos
	m := g.Subrule(rule, ctx)
	ctx.Pos = saved // zero-width assertion: do not consume input
	if m.Ok {
		return NewMatch("", saved, saved, true)
	}
	return NewMatch("", saved, saved, false)
}

// NotBefore implements negative lookahead: <!before <rule>>
func (g *Grammar) NotBefore(rule string, ctx *Context) *Match {
	saved := ctx.Pos
	m := g.Subrule(rule, ctx)
	ctx.Pos = saved
	if !m.Ok {
		return NewMatch("", saved, saved, true)
	}
	return NewMatch("", saved, saved, false)
}

// After implements positive lookbehind: <?after <lit>>
func (g *Grammar) After(lit string, ctx *Context) *Match {
	runes := []rune(lit)
	length := len(runes)
	if ctx.Pos < length {
		return NewMatch("", ctx.Pos, ctx.Pos, false)
	}
	start := ctx.Pos - length
	for i := 0; i < length; i++ {
		if ctx.Src[start+i] != runes[i] {
			return NewMatch("", ctx.Pos, ctx.Pos, false)
		}
	}
	return NewMatch("", ctx.Pos, ctx.Pos, true)
}

// NotAfter implements negative lookbehind: <!after <lit>>
func (g *Grammar) NotAfter(lit string, ctx *Context) *Match {
	m := g.After(lit, ctx)
	if !m.Ok {
		return NewMatch("", ctx.Pos, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
}

// Alpha matches a unicode or ascii alphabetic letter.
func (g *Grammar) Alpha(ctx *Context) *Match {
	if ctx.Pos < len(ctx.Src) && unicode.IsLetter(ctx.Src[ctx.Pos]) {
		r := ctx.Src[ctx.Pos]
		ctx.Pos++
		return NewMatch(string(r), ctx.Pos-1, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
}

// Digit matches a decimal digit.
func (g *Grammar) Digit(ctx *Context) *Match {
	if ctx.Pos < len(ctx.Src) && unicode.IsDigit(ctx.Src[ctx.Pos]) {
		r := ctx.Src[ctx.Pos]
		ctx.Pos++
		return NewMatch(string(r), ctx.Pos-1, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
}

// Alnum matches alphabetic letter or digit.
func (g *Grammar) Alnum(ctx *Context) *Match {
	if ctx.Pos < len(ctx.Src) && (unicode.IsLetter(ctx.Src[ctx.Pos]) || unicode.IsDigit(ctx.Src[ctx.Pos])) {
		r := ctx.Src[ctx.Pos]
		ctx.Pos++
		return NewMatch(string(r), ctx.Pos-1, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
}

// Space matches any whitespace character.
func (g *Grammar) Space(ctx *Context) *Match {
	if ctx.Pos < len(ctx.Src) && unicode.IsSpace(ctx.Src[ctx.Pos]) {
		r := ctx.Src[ctx.Pos]
		ctx.Pos++
		return NewMatch(string(r), ctx.Pos-1, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
}

// Blank matches horizontal whitespace (space or tab).
func (g *Grammar) Blank(ctx *Context) *Match {
	if ctx.Pos < len(ctx.Src) && isHorizontalSpace(ctx.Src[ctx.Pos]) {
		r := ctx.Src[ctx.Pos]
		ctx.Pos++
		return NewMatch(string(r), ctx.Pos-1, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
}

// Word matches a word character (\w).
func (g *Grammar) Word(ctx *Context) *Match {
	if ctx.Pos < len(ctx.Src) && isWordRune(ctx.Src[ctx.Pos]) {
		r := ctx.Src[ctx.Pos]
		ctx.Pos++
		return NewMatch(string(r), ctx.Pos-1, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
}

// CharClass matches Raku character class specifications like "<[a..z0..9_]>" or "<-[0..9]>".
func (g *Grammar) CharClass(spec string, ctx *Context) *Match {
	if ctx.Pos >= len(ctx.Src) {
		return NewMatch("", ctx.Pos, ctx.Pos, false)
	}
	ch := ctx.Src[ctx.Pos]

	negated := strings.Contains(spec, "<-") || (strings.HasPrefix(spec, "-") && !strings.HasPrefix(spec, "<["))
	clean := spec
	clean = strings.TrimPrefix(clean, "<")
	clean = strings.TrimSuffix(clean, ">")
	clean = strings.TrimPrefix(clean, "+[")
	clean = strings.TrimPrefix(clean, "-[")
	clean = strings.TrimPrefix(clean, "[")
	clean = strings.TrimSuffix(clean, "]")

	matched := classContains(clean, ch)
	if negated {
		matched = !matched
	}

	if matched {
		ctx.Pos++
		return NewMatch(string(ch), ctx.Pos-1, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
}

func matchEscClass(esc rune, ch rune) (handled bool, matched bool) {
	switch esc {
	case 's':
		return true, unicode.IsSpace(ch)
	case 'S':
		return true, !unicode.IsSpace(ch)
	case 'h':
		return true, isHorizontalSpace(ch)
	case 'H':
		return true, !isHorizontalSpace(ch)
	case 'v':
		return true, isVerticalSpace(ch)
	case 'V':
		return true, !isVerticalSpace(ch)
	case 'n':
		return true, ch == '\n'
	case 'N':
		return true, ch != '\n'
	case 't':
		return true, ch == '\t'
	case 'T':
		return true, ch != '\t'
	case 'r':
		return true, ch == '\r'
	case 'R':
		return true, ch != '\r'
	case 'd':
		return true, unicode.IsDigit(ch)
	case 'D':
		return true, !unicode.IsDigit(ch)
	case 'w':
		return true, isWordRune(ch)
	case 'W':
		return true, !isWordRune(ch)
	}
	return false, false
}

func matchUnicodeProp(prop string, negated bool, ch rune) bool {
	var ok bool
	switch prop {
	case "Lu", "Upper", "Uppercase_Letter":
		ok = unicode.Is(unicode.Lu, ch) || unicode.IsUpper(ch)
	case "Ll", "Lower", "Lowercase_Letter":
		ok = unicode.Is(unicode.Ll, ch) || unicode.IsLower(ch)
	case "Lt", "Titlecase_Letter":
		ok = unicode.Is(unicode.Lt, ch)
	case "Lm", "Modifier_Letter":
		ok = unicode.Is(unicode.Lm, ch)
	case "Lo", "Other_Letter":
		ok = unicode.Is(unicode.Lo, ch)
	case "L", "Letter", "Alpha", "Alphabetic":
		ok = unicode.IsLetter(ch)
	case "Nd", "Digit", "Decimal_Number":
		ok = unicode.IsDigit(ch)
	case "Nl", "Letter_Number":
		ok = unicode.Is(unicode.Nl, ch)
	case "No", "Other_Number":
		ok = unicode.Is(unicode.No, ch)
	case "N", "Number":
		ok = unicode.IsNumber(ch)
	case "M", "Mark":
		ok = unicode.IsMark(ch)
	case "P", "Punct", "Punctuation":
		ok = unicode.IsPunct(ch)
	case "S", "Symbol":
		ok = unicode.IsSymbol(ch)
	case "Z", "Separator":
		ok = unicode.Is(unicode.Z, ch)
	case "Zs":
		ok = unicode.Is(unicode.Zs, ch)
	case "Space", "White_Space":
		ok = unicode.IsSpace(ch)
	case "Blank":
		ok = isHorizontalSpace(ch)
	case "C", "Other", "Cntrl", "Control":
		ok = unicode.IsControl(ch)
	case "Alnum":
		ok = unicode.IsLetter(ch) || unicode.IsDigit(ch)
	case "Word":
		ok = isWordRune(ch)
	case "Hex", "XDigit", "ASCII_Hex_Digit":
		ok = isHexDigit(ch)
	case "ASCII":
		ok = ch >= 0 && ch <= 0x7F
	}
	if negated {
		return !ok
	}
	return ok
}

func decodeClassAtom(runes []rune, i int) (r rune, isClass bool, classEsc rune, nextIdx int) {
	if runes[i] == '\\' && i+1 < len(runes) {
		if runes[i+1] == 'x' && i+2 < len(runes) && runes[i+2] == '[' {
			end := i + 3
			for end < len(runes) && runes[end] != ']' {
				end++
			}
			if end < len(runes) {
				hexStr := strings.TrimSpace(string(runes[i+3 : end]))
				if v, err := strconv.ParseInt(hexStr, 16, 32); err == nil {
					return rune(v), false, 0, end + 1
				}
			}
		}
		esc := runes[i+1]
		switch esc {
		case 'n':
			return '\n', false, 0, i + 2
		case 't':
			return '\t', false, 0, i + 2
		case 'r':
			return '\r', false, 0, i + 2
		case 's', 'S', 'h', 'H', 'v', 'V', 'N', 'T', 'R', 'd', 'D', 'w', 'W':
			return 0, true, esc, i + 2
		default:
			return esc, false, 0, i + 2
		}
	}
	return runes[i], false, 0, i + 1
}

func classContains(spec string, ch rune) bool {
	runes := []rune(spec)
	i := 0
	for i < len(runes) {
		lo, isClass, classEsc, next := decodeClassAtom(runes, i)
		if isClass {
			if _, ok := matchEscClass(classEsc, ch); ok {
				return true
			}
			i = next
			continue
		}
		// Check for '..' range: e.g. a..z or \x[20]..\x[7e]
		j := next
		for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
			j++
		}
		if j+1 < len(runes) && runes[j] == '.' && runes[j+1] == '.' {
			k := j + 2
			for k < len(runes) && (runes[k] == ' ' || runes[k] == '\t') {
				k++
			}
			if k < len(runes) {
				hi, hiIsClass, _, afterHi := decodeClassAtom(runes, k)
				if !hiIsClass {
					if ch >= lo && ch <= hi {
						return true
					}
					i = afterHi
					continue
				}
			}
		}
		if ch == lo {
			return true
		}
		i = next
	}
	return false
}

// ListWithSep matches modified quantified lists: <item> % <sep>
func (g *Grammar) ListWithSep(itemRule string, sepRule string, ctx *Context) *Match {
	start := ctx.Pos
	first := g.Subrule(itemRule, ctx)
	if !first.Ok {
		return NewMatch("", start, start, false)
	}
	m := NewMatch("", start, ctx.Pos, true)
	m.AddNamed(itemRule, first)
	m.AddPositional(first)

	for ctx.Pos < len(ctx.Src) {
		saved := ctx.Pos
		ctx.SkipWS()
		sep := g.Subrule(sepRule, ctx)
		if !sep.Ok {
			ctx.Pos = saved
			break
		}
		next := g.Subrule(itemRule, ctx)
		if !next.Ok {
			ctx.Pos = saved
			break
		}
		m.AddNamed(sepRule, sep)
		m.AddNamed(itemRule, next)
		m.AddPositional(next)
	}

	m.To = ctx.Pos
	m.Str = string(ctx.Src[start:ctx.Pos])
	return m
}
