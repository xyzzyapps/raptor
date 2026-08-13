package grammar

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

// Match represents a Perl 6 / Raku match object ($/).
type Match struct {
	Str   string
	From  int
	To    int
	Ok    bool
	Named map[string]*Match
	List  map[string][]*Match
	Pos   []*Match
	Made  any // Value set by 'make' ($/.made / $/.ast)
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

// Get returns a single named capture match.
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


// RuleFunc is the function signature for a grammar rule / token match function.
type RuleFunc func(g *Grammar, ctx *Context) *Match

// Context manages current position, source text, and action handler.
type Context struct {
	Src     []rune
	Pos     int
	Actions any
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

// Grammar is a Perl 6 / Raku grammar definition.
type Grammar struct {
	Name  string
	Rules map[string]RuleFunc
}

func NewGrammar(name string) *Grammar {
	return &Grammar{
		Name:  name,
		Rules: make(map[string]RuleFunc),
	}
}

func (g *Grammar) DefineRule(name string, fn RuleFunc) {
	g.Rules[name] = fn
}

// Parse runs the TOP rule of the grammar and invokes action methods.
func (g *Grammar) Parse(input string, actions any) (*Match, error) {
	topRule, ok := g.Rules["TOP"]
	if !ok {
		return nil, fmt.Errorf("grammar %q has no TOP rule", g.Name)
	}

	ctx := &Context{
		Src:     []rune(input),
		Pos:     0,
		Actions: actions,
	}

	m := g.invokeRule("TOP", topRule, ctx)
	if m == nil || !m.Ok {
		return nil, fmt.Errorf("grammar %q parse failed", g.Name)
	}

	return m, nil
}

func (g *Grammar) invokeRule(name string, fn RuleFunc, ctx *Context) *Match {
	startPos := ctx.Pos
	m := fn(g, ctx)
	if m != nil && m.Ok {
		if ctx.Actions != nil {
			g.callAction(name, m, ctx.Actions)
		}
		return m
	}
	ctx.Pos = startPos
	return NewMatch("", startPos, startPos, false)
}

func (g *Grammar) callAction(name string, m *Match, actions any) {
	val := reflect.ValueOf(actions)
	method := val.MethodByName(name)
	if !method.IsValid() && len(name) > 0 {
		capitalized := strings.ToUpper(name[:1]) + name[1:]
		method = val.MethodByName(capitalized)
	}
	if !method.IsValid() {
		upper := strings.ToUpper(name)
		method = val.MethodByName(upper)
	}
	if method.IsValid() && method.Type().NumIn() == 1 {
		method.Call([]reflect.Value{reflect.ValueOf(m)})
	}
}


// Subrule helper: invokes a named rule from the grammar.
func (g *Grammar) Subrule(name string, ctx *Context) *Match {
	fn, ok := g.Rules[name]
	if !ok {
		return NewMatch("", ctx.Pos, ctx.Pos, false)
	}
	return g.invokeRule(name, fn, ctx)
}

// Lit matches exact literal string.
func (g *Grammar) Lit(lit string, ctx *Context) *Match {
	ctx.SkipWS()
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

// Ident matches a Perl 6 identifier (\w+).
func (g *Grammar) Ident(ctx *Context) *Match {
	ctx.SkipWS()
	start := ctx.Pos
	var sb strings.Builder
	for ctx.Pos < len(ctx.Src) && (unicode.IsLetter(ctx.Src[ctx.Pos]) || unicode.IsDigit(ctx.Src[ctx.Pos]) || ctx.Src[ctx.Pos] == '_') {
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
		table = DefaultRakuOpTable()
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

// Digit matches a decimal digit (0..9).
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
	if ctx.Pos < len(ctx.Src) && (ctx.Src[ctx.Pos] == ' ' || ctx.Src[ctx.Pos] == '\t') {
		r := ctx.Src[ctx.Pos]
		ctx.Pos++
		return NewMatch(string(r), ctx.Pos-1, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
}

// Word matches a word character (\w).
func (g *Grammar) Word(ctx *Context) *Match {
	if ctx.Pos < len(ctx.Src) && (unicode.IsLetter(ctx.Src[ctx.Pos]) || unicode.IsDigit(ctx.Src[ctx.Pos]) || ctx.Src[ctx.Pos] == '_') {
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

	negated := strings.HasPrefix(spec, "-") || strings.HasPrefix(spec, "<-")
	clean := strings.TrimPrefix(spec, "<+[")
	clean = strings.TrimPrefix(clean, "<[")
	clean = strings.TrimPrefix(clean, "<-[")
	clean = strings.TrimPrefix(clean, "+[")
	clean = strings.TrimPrefix(clean, "-[")
	clean = strings.TrimSuffix(clean, "]>")
	clean = strings.TrimSuffix(clean, "]")

	matched := false
	runes := []rune(clean)
	for i := 0; i < len(runes); i++ {
		if i+3 < len(runes) && runes[i+1] == '.' && runes[i+2] == '.' {
			startR := runes[i]
			endR := runes[i+3]
			if ch >= startR && ch <= endR {
				matched = true
				break
			}
			i += 3
		} else if runes[i] == ch {
			matched = true
			break
		}
	}

	if negated {
		matched = !matched
	}

	if matched {
		ctx.Pos++
		return NewMatch(string(ch), ctx.Pos-1, ctx.Pos, true)
	}
	return NewMatch("", ctx.Pos, ctx.Pos, false)
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



