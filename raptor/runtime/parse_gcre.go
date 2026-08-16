package raptor

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"gcre"
)

//go:embed raptor.raku
var raptorGrammarSrc string

var (
	raptorGOnce sync.Once
	raptorG     *gcre.Grammar
	raptorGErr  error
)

func init() {
	gcre.RegisterHost("stmt", hostStmt)
	gcre.RegisterHost("expr", hostExpr)
}

type hostLex struct {
	tokens []Token
}

func tokenIndexAt(tokens []Token, pos int) int {
	for i, t := range tokens {
		if t.To > pos || t.Type == TokEOF {
			return i
		}
	}
	if len(tokens) == 0 {
		return 0
	}
	return len(tokens) - 1
}

func parseStmtAt(tokens []Token, pos int) (Stmt, int, error) {
	i := tokenIndexAt(tokens, pos)
	p := NewParser(tokens[i:])
	stmt, err := p.parseStatement()
	if err != nil {
		return nil, 0, err
	}
	return stmt, lastConsumedEnd(p), nil
}

func parseExprAt(tokens []Token, pos int) (Expr, int, error) {
	i := tokenIndexAt(tokens, pos)
	p := NewParser(tokens[i:])
	expr, err := p.parseExpression(0)
	if err != nil {
		return nil, 0, err
	}
	return expr, lastConsumedEnd(p), nil
}

func hostStmt(_ *gcre.Grammar, ctx *gcre.Context, cap *gcre.Match) bool {
	if ctx == nil || ctx.Pos >= len(ctx.Src) {
		return false
	}
	if hl, ok := ctx.Host.(*hostLex); ok && hl != nil && len(hl.tokens) > 0 {
		stmt, end, err := parseStmtAt(hl.tokens, ctx.Pos)
		if err != nil || end <= ctx.Pos {
			return false
		}
		ctx.Pos = end
		cap.Make(stmt)
		return true
	}
	rest := string(ctx.Src[ctx.Pos:])
	stmt, n, err := ParseOneStatement(rest)
	if err != nil || n <= 0 {
		return false
	}
	ctx.Pos += n
	cap.Make(stmt)
	return true
}

func hostExpr(_ *gcre.Grammar, ctx *gcre.Context, cap *gcre.Match) bool {
	if ctx == nil || ctx.Pos >= len(ctx.Src) {
		return false
	}
	if hl, ok := ctx.Host.(*hostLex); ok && hl != nil && len(hl.tokens) > 0 {
		expr, end, err := parseExprAt(hl.tokens, ctx.Pos)
		if err != nil || end <= ctx.Pos {
			return false
		}
		ctx.Pos = end
		cap.Make(expr)
		return true
	}
	rest := string(ctx.Src[ctx.Pos:])
	expr, n, err := ParseOneExpression(rest)
	if err != nil || n <= 0 {
		return false
	}
	ctx.Pos += n
	cap.Make(expr)
	return true
}

func raptorGrammar() (*gcre.Grammar, error) {
	raptorGOnce.Do(func() {
		raptorG, raptorGErr = gcre.LoadGrammarFromString(raptorGrammarSrc)
	})
	return raptorG, raptorGErr
}

// ParseProgram is the only entry: gcre loads raptor.raku.
// Pratt runs only when the grammar names <HOST_stmt> or <HOST_expr>.
func ParseProgram(source string) (*Program, error) {
	return parseProgramGcre(source)
}

func parseProgramGcre(source string) (*Program, error) {
	g, err := raptorGrammar()
	if err != nil {
		return nil, fmt.Errorf("load raptor grammar: %w", err)
	}
	ctx := &gcre.Context{Src: []rune(source), Pos: 0}
	if toks, lexErr := lexAll(source); lexErr == nil {
		ctx.Host = &hostLex{tokens: toks}
	}
	m := g.Subrule("TOP", ctx)
	if m == nil || !m.Ok {
		return nil, fmt.Errorf("parse error at position %d", ctx.Pos)
	}
	rest := strings.TrimSpace(string(ctx.Src[ctx.Pos:]))
	if rest != "" && !allComments(rest) {
		return nil, fmt.Errorf("parse error: unconsumed input at position %d: %q", ctx.Pos, clip(rest, 60))
	}
	var stmts []Stmt
	for _, sm := range m.GetAll("statement") {
		st := walkStatement(sm)
		if st != nil {
			stmts = append(stmts, st)
		}
	}
	return &Program{Stmts: stmts}, nil
}

func allComments(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t != "" && !strings.HasPrefix(t, "#") {
			return false
		}
	}
	return true
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func firstNamed(m *gcre.Match, names ...string) (*gcre.Match, string) {
	if m == nil {
		return nil, ""
	}
	for _, n := range names {
		sub := m.Get(n)
		if sub != nil && sub.Ok {
			return sub, n
		}
	}
	return nil, ""
}

func walkStatement(m *gcre.Match) Stmt {
	if m == nil || !m.Ok {
		return nil
	}
	if hs := m.Get("HOST_stmt"); hs != nil && hs.Ok {
		if st, ok := hs.Made.(Stmt); ok {
			return st
		}
	}
	if c := m.Get("comment"); c != nil && c.Ok && !hasOtherStmt(m) {
		return nil
	}
	if s := m.Get("var_decl"); s != nil && s.Ok {
		return applyMod(walkVarDecl(s), s)
	}
	if s := m.Get("sub_decl"); s != nil && s.Ok {
		return walkSubDecl(s)
	}
	if s := m.Get("if_stmt"); s != nil && s.Ok {
		return walkIf(s)
	}
	if s := m.Get("unless_stmt"); s != nil && s.Ok {
		return &UnlessStmt{Condition: walkExpr(s.Get("expression")), Body: walkBlock(s.Get("block"))}
	}
	if s := m.Get("while_stmt"); s != nil && s.Ok {
		return &WhileStmt{Condition: walkExpr(s.Get("expression")), Body: walkBlock(s.Get("block"))}
	}
	if s := m.Get("until_stmt"); s != nil && s.Ok {
		return &WhileStmt{IsUntil: true, Condition: walkExpr(s.Get("expression")), Body: walkBlock(s.Get("block"))}
	}
	if s := m.Get("for_stmt"); s != nil && s.Ok {
		vn := "$_"
		if v := s.Get("var"); v != nil && v.Ok {
			vn = v.Str
		}
		return &ForStmt{Iterable: walkExpr(s.Get("expression")), VarName: vn, Body: walkBlock(s.Get("block"))}
	}
	if s := m.Get("loop_stmt"); s != nil && s.Ok {
		return walkLoop(s)
	}
	if s := m.Get("given_stmt"); s != nil && s.Ok {
		return walkGiven(s)
	}
	if s := m.Get("subset_decl"); s != nil && s.Ok {
		return walkSubset(s)
	}
	if s := m.Get("struct_decl"); s != nil && s.Ok {
		return walkStruct(s)
	}
	if s := m.Get("enum_decl"); s != nil && s.Ok {
		return walkEnum(s)
	}
	if s := m.Get("return_stmt"); s != nil && s.Ok {
		return applyMod(&ReturnStmt{Value: walkExpr(s.Get("expression"))}, s)
	}
	if s := m.Get("last_stmt"); s != nil && s.Ok {
		return &BreakStmt{}
	}
	if s := m.Get("next_stmt"); s != nil && s.Ok {
		return &ContinueStmt{}
	}
	if s := m.Get("redo_stmt"); s != nil && s.Ok {
		return applyMod(&RedoStmt{}, s)
	}
	if s := m.Get("goto_stmt"); s != nil && s.Ok {
		t := s.Get("goto_target")
		name := ""
		isSub := false
		if t != nil && t.Ok {
			name = strings.TrimPrefix(t.Str, "&")
			isSub = strings.HasPrefix(t.Str, "&")
		}
		return &GotoStmt{Target: name, IsSub: isSub}
	}
	if s := m.Get("take_stmt"); s != nil && s.Ok {
		return &TakeStmt{Value: walkExpr(s.Get("expression"))}
	}
	if s := m.Get("assert_stmt"); s != nil && s.Ok {
		exprs := s.GetAll("expression")
		var cond, msg Expr
		if len(exprs) > 0 {
			cond = walkExpr(exprs[0])
		}
		if len(exprs) > 1 {
			msg = walkExpr(exprs[1])
		}
		return &AssertStmt{Condition: cond, Message: msg}
	}
	if s := m.Get("use_stmt"); s != nil && s.Ok {
		mod := ""
		if n := s.Get("colon_name"); n != nil && n.Ok {
			mod = n.Str
		}
		from := ""
		if strings.Contains(s.Str, ":from<") {
			i := strings.Index(s.Str, ":from<")
			rest := s.Str[i+6:]
			if j := strings.Index(rest, ">"); j >= 0 {
				from = rest[:j]
			}
		}
		return &UseStmt{Module: mod, From: from}
	}
	if s := m.Get("package_decl"); s != nil && s.Ok {
		name := ""
		if n := s.Get("colon_name"); n != nil && n.Ok {
			name = n.Str
		}
		return &PackageDeclStmt{
			Name:   name,
			IsUnit: strings.Contains(s.Str, "unit"),
			Body:   walkBlock(s.Get("block")),
		}
	}
	if s := m.Get("grammar_decl"); s != nil && s.Ok {
		return walkGrammarDecl(s)
	}
	if s := m.Get("advice_stmt"); s != nil && s.Ok {
		kind := "before"
		st := strings.TrimSpace(s.Str)
		for _, k := range []string{"before", "after", "around"} {
			if strings.HasPrefix(st, k) {
				kind = k
				break
			}
		}
		nm := ""
		if n := s.Get("name"); n != nil && n.Ok {
			nm = n.Str
		}
		return &AdviceHookStmt{Kind: kind, TargetName: nm, Params: walkParams(s.Get("sig")), Body: walkBlock(s.Get("block"))}
	}
	if s := m.Get("label_stmt"); s != nil && s.Ok {
		nm := ""
		if n := s.Get("name"); n != nil && n.Ok {
			nm = n.Str
		}
		inner := walkStatement(s.Get("statement"))
		return &LabelStmt{Name: nm, Stmt: inner}
	}
	if s := m.Get("block"); s != nil && s.Ok && m.Get("expr_stmt") != nil && !m.Get("expr_stmt").Ok || (s != nil && s.Ok && (m.Get("expr_stmt") == nil || !m.Get("expr_stmt").Ok)) {
		if m.Get("expr_stmt") == nil || !m.Get("expr_stmt").Ok {
			if s != nil && s.Ok {
				return walkBlock(s)
			}
		}
	}
	if s := m.Get("expr_stmt"); s != nil && s.Ok {
		ex := walkExpr(s.Get("expression"))
		if ex == nil {
			if lp := s.Get("listop"); lp != nil && lp.Ok {
				ex = walkListop(lp)
			}
		}
		return applyMod(&ExprStmt{Expr: ex}, s)
	}
	// bare expression fallback
	if e := m.Get("expression"); e != nil && e.Ok {
		return &ExprStmt{Expr: walkExpr(e)}
	}
	return nil
}

func hasOtherStmt(m *gcre.Match) bool {
	for _, n := range []string{"var_decl", "sub_decl", "if_stmt", "expr_stmt", "block"} {
		if s := m.Get(n); s != nil && s.Ok {
			return true
		}
	}
	return false
}

func applyMod(stmt Stmt, m *gcre.Match) Stmt {
	if stmt == nil || m == nil {
		return stmt
	}
	mod := m.Get("modifier")
	if mod == nil || !mod.Ok {
		return stmt
	}
	kind := ModIf
	s := strings.TrimSpace(mod.Str)
	switch {
	case strings.HasPrefix(s, "unless"):
		kind = ModUnless
	case strings.HasPrefix(s, "while"):
		kind = ModWhile
	case strings.HasPrefix(s, "until"):
		kind = ModUntil
	case strings.HasPrefix(s, "for"):
		kind = ModFor
	case strings.HasPrefix(s, "given"):
		kind = ModGiven
	}
	return &ModifierStmt{Kind: kind, Target: stmt, Condition: walkExpr(mod.Get("expression")), VarName: "$_"}
}

func walkVarDecl(m *gcre.Match) Stmt {
	scope := "my"
	if s := m.Get("scope"); s != nil && s.Ok {
		scope = s.Str
	}
	typ := ""
	if t := m.Get("typename"); t != nil && t.Ok {
		typ = t.Str
	}
	name := ""
	if v := m.Get("var"); v != nil && v.Ok {
		name = v.Str
	}
	var where Expr
	if w := m.Get("where_clause"); w != nil && w.Ok {
		if b := w.Get("block"); b != nil && b.Ok {
			where = &ClosureExpr{Body: walkBlock(b)}
		} else {
			where = walkExpr(w.Get("expression"))
		}
	}
	var init Expr
	// assignment is expression after =
	if strings.Contains(m.Str, "=") {
		if e := m.Get("expression"); e != nil && e.Ok {
			init = walkExpr(e)
		}
	}
	return &VarDeclStmt{Scope: scope, Type: typ, Name: name, Where: where, Value: init}
}

func walkSubDecl(m *gcre.Match) Stmt {
	isMulti := strings.HasPrefix(strings.TrimSpace(m.Str), "multi")
	name := ""
	if n := m.Get("sub_name"); n != nil && n.Ok {
		name = strings.TrimSpace(n.Str)
		if u := n.Get("uni_name"); u != nil && u.Ok {
			name = u.Str
		} else if nn := n.Get("name"); nn != nil && nn.Ok && !strings.Contains(n.Str, "infix") && !strings.Contains(n.Str, "prefix") {
			name = nn.Str
		}
	}
	params := walkParams(m.Get("sig"))
	ret := ""
	if t := m.Get("typename"); t != nil && t.Ok {
		ret = t.Str
	}
	if nt := m.Get("native_trait"); nt != nil && nt.Ok {
		lib := ""
		sym := ""
		strs := nt.GetAll("string")
		if len(strs) > 0 {
			lib = unquote(strs[0].Str)
		}
		if len(strs) > 1 {
			sym = unquote(strs[1].Str)
		}
		// also check all native_trait
		for _, t := range m.GetAll("native_trait") {
			ss := t.GetAll("string")
			if len(ss) > 0 && lib == "" {
				lib = unquote(ss[0].Str)
			}
			if len(ss) > 1 {
				sym = unquote(ss[1].Str)
			}
		}
		return &NativeSubDeclStmt{Name: name, Params: params, ReturnType: ret, Library: lib, Symbol: sym}
	}
	if strings.Contains(m.Str, "is native") {
		lib := extractQuotedAfter(m.Str, "native")
		sym := extractQuotedAfter(m.Str, "symbol")
		return &NativeSubDeclStmt{Name: name, Params: params, ReturnType: ret, Library: lib, Symbol: sym}
	}
	return &SubDeclStmt{IsMulti: isMulti, Name: name, Params: params, Body: walkBlock(m.Get("block"))}
}

func extractQuotedAfter(s, key string) string {
	i := strings.Index(s, key)
	if i < 0 {
		return ""
	}
	rest := s[i+len(key):]
	q := strings.IndexAny(rest, `"'`)
	if q < 0 {
		return ""
	}
	quote := rest[q]
	rest = rest[q+1:]
	e := strings.IndexByte(rest, quote)
	if e < 0 {
		return ""
	}
	return rest[:e]
}

func walkParams(m *gcre.Match) []Param {
	if m == nil || !m.Ok {
		return nil
	}
	pl := m.Get("param_list")
	if pl == nil || !pl.Ok {
		pl = m
	}
	var out []Param
	for _, p := range pl.GetAll("param") {
		if inner := p.Get("param_list"); inner != nil && inner.Ok && strings.HasPrefix(strings.TrimSpace(p.Str), "[") {
			var ds []Param
			for _, ip := range inner.GetAll("param") {
				ds = append(ds, walkOneParam(ip))
			}
			out = append(out, Param{Name: fmt.Sprintf("$_destruct_arr_%d", len(out)), Type: "Array", DestructArr: ds})
			continue
		}
		out = append(out, walkOneParam(p))
	}
	return out
}

func walkOneParam(p *gcre.Match) Param {
	name := ""
	if v := p.Get("var"); v != nil && v.Ok {
		name = v.Str
	}
	typ := ""
	if t := p.Get("typename"); t != nil && t.Ok {
		typ = t.Str
	}
	slurpy := strings.Contains(p.Str, "*")
	var where Expr
	if w := p.Get("where_clause"); w != nil && w.Ok {
		if b := w.Get("block"); b != nil && b.Ok {
			where = &ClosureExpr{Body: walkBlock(b)}
		} else {
			where = walkExpr(w.Get("expression"))
		}
	}
	return Param{Name: name, Type: typ, IsSlurpy: slurpy, Where: where}
}

func walkIf(m *gcre.Match) Stmt {
	exprs := m.GetAll("expression")
	blocks := m.GetAll("block")
	st := &IfStmt{}
	if len(exprs) > 0 {
		st.Condition = walkExpr(exprs[0])
	}
	if len(blocks) > 0 {
		st.ThenBranch = walkBlock(blocks[0])
	}
	// elsif: remaining expr/block pairs except else
	hasElse := strings.Contains(m.Str, "else")
	nElsif := len(exprs) - 1
	if hasElse && len(blocks) > len(exprs) {
		// else has block but no expr
	}
	for i := 1; i < len(exprs) && i < len(blocks); i++ {
		st.ElsifConds = append(st.ElsifConds, walkExpr(exprs[i]))
		st.ElsifThen = append(st.ElsifThen, walkBlock(blocks[i]))
		nElsif--
	}
	if hasElse && len(blocks) > 1+len(st.ElsifThen) {
		st.ElseBranch = walkBlock(blocks[len(blocks)-1])
	}
	return st
}

func walkLoop(m *gcre.Match) Stmt {
	body := walkBlock(m.Get("block"))
	st := &LoopStmt{Body: body}
	parts := m.GetAll("loop_part")
	if len(parts) == 0 {
		exprs := m.GetAll("expression")
		if len(exprs) >= 1 {
			st.Init = walkExpr(exprs[0])
		}
		if len(exprs) >= 2 {
			st.Cond = walkExpr(exprs[1])
		}
		if len(exprs) >= 3 {
			st.Step = walkExpr(exprs[2])
		}
		return st
	}
	asExpr := func(p *gcre.Match) Expr {
		if p == nil || !p.Ok {
			return nil
		}
		if p.Get("scope") != nil && p.Get("scope").Ok {
			name := ""
			if v := p.Get("var"); v != nil && v.Ok {
				name = v.Str
			}
			return &AssignStmt{Target: &VarExpr{Name: name}, Op: "=", Value: walkExpr(p.Get("expression"))}
		}
		return walkExpr(p.Get("expression"))
	}
	if len(parts) >= 1 {
		st.Init = asExpr(parts[0])
	}
	if len(parts) >= 2 {
		st.Cond = asExpr(parts[1])
	}
	if len(parts) >= 3 {
		st.Step = asExpr(parts[2])
	}
	return st
}

func walkGiven(m *gcre.Match) Stmt {
	st := &GivenStmt{Topic: walkExpr(m.Get("expression"))}
	gb := m.Get("given_block")
	if gb == nil || !gb.Ok {
		return st
	}
	for _, w := range gb.GetAll("when_clause") {
		st.Whens = append(st.Whens, WhenClause{Match: walkExpr(w.Get("expression")), Body: walkBlock(w.Get("block"))})
	}
	if d := gb.Get("default_clause"); d != nil && d.Ok {
		st.Default = walkBlock(d.Get("block"))
	}
	return st
}

func walkSubset(m *gcre.Match) Stmt {
	name := ""
	if n := m.Get("name"); n != nil && n.Ok {
		name = n.Str
	}
	var where Expr
	if w := m.Get("where_clause"); w != nil && w.Ok {
		if b := w.Get("block"); b != nil && b.Ok {
			where = &ClosureExpr{Body: walkBlock(b)}
		} else {
			where = walkExpr(w.Get("expression"))
		}
	}
	return &SubsetDeclStmt{Name: name, Where: where}
}

func walkStruct(m *gcre.Match) Stmt {
	name := ""
	if n := m.Get("name"); n != nil && n.Ok {
		name = n.Str
	}
	isUnion := strings.HasPrefix(strings.TrimSpace(m.Str), "union")
	st := &CStructDeclStmt{Name: name, IsUnion: isUnion, FieldIndex: map[string]int{}}
	off := 0
	for i, f := range m.GetAll("struct_field") {
		typ := ""
		vn := ""
		if n := f.Get("name"); n != nil && n.Ok {
			typ = n.Str
		}
		if v := f.Get("var"); v != nil && v.Ok {
			vn = strings.TrimLeft(v.Str, "$@%")
		}
		sz := 8
		st.Fields = append(st.Fields, CStructField{Name: vn, Type: typ, Offset: off, Size: sz})
		st.FieldIndex[vn] = i
		if !isUnion {
			off += sz
		}
	}
	st.TotalSize = off
	if isUnion && len(st.Fields) > 0 {
		st.TotalSize = 8
	}
	st.Alignment = 8
	return st
}

func walkEnum(m *gcre.Match) Stmt {
	names := m.GetAll("name")
	en := ""
	if len(names) > 0 {
		en = names[0].Str
	}
	var vals []EnumValue
	pairs := m.GetAll("hash_pair")
	if len(pairs) > 0 {
		for _, p := range pairs {
			nm := ""
			if n := p.Get("name"); n != nil && n.Ok {
				nm = n.Str
			}
			idx := int64(len(vals))
			es := p.GetAll("expression")
			if len(es) > 0 {
				if lit, ok := walkExpr(es[len(es)-1]).(*LiteralExpr); ok {
					switch v := lit.Value.(type) {
					case int64:
						idx = v
					case int:
						idx = int64(v)
					}
				}
			}
			vals = append(vals, EnumValue{Name: nm, Index: idx})
		}
		return &EnumDeclStmt{Name: en, Values: vals}
	}
	for i, n := range names {
		if i == 0 {
			continue
		}
		vals = append(vals, EnumValue{Name: n.Str, Index: int64(i - 1)})
	}
	return &EnumDeclStmt{Name: en, Values: vals}
}

func walkGrammarDecl(m *gcre.Match) Stmt {
	name := ""
	if n := m.Get("name"); n != nil && n.Ok {
		name = n.Str
	}
	var rules []RuleDecl
	for _, r := range m.GetAll("g_rule") {
		kind := "token"
		st := strings.TrimSpace(r.Str)
		for _, k := range []string{"token", "rule", "regex"} {
			if strings.HasPrefix(st, k) {
				kind = k
				break
			}
		}
		rn := ""
		if n := r.Get("name"); n != nil && n.Ok {
			rn = n.Str
		}
		pat := r.Str
		if i := strings.Index(pat, "{"); i >= 0 {
			pat = strings.TrimSpace(pat[i+1:])
			pat = strings.TrimSuffix(pat, "}")
		}
		rules = append(rules, RuleDecl{Kind: kind, Name: rn, Pattern: strings.TrimSpace(pat)})
	}
	return &GrammarDeclStmt{Name: name, Rules: rules}
}

func walkBlock(m *gcre.Match) *BlockStmt {
	if m == nil || !m.Ok {
		return &BlockStmt{}
	}
	var stmts []Stmt
	for _, s := range m.GetAll("statement") {
		if st := walkStatement(s); st != nil {
			stmts = append(stmts, st)
		}
	}
	return &BlockStmt{Stmts: stmts}
}

func walkExpr(m *gcre.Match) Expr {
	if m == nil || !m.Ok {
		return nil
	}
	if he := m.Get("HOST_expr"); he != nil && he.Ok {
		if e, ok := he.Made.(Expr); ok {
			return e
		}
	}
	if a := m.Get("assign_expr"); a != nil && a.Ok {
		return walkAssign(a)
	}
	if t := m.Get("ternary"); t != nil && t.Ok {
		return walkTernary(t)
	}
	if lp := m.Get("listop"); lp != nil && lp.Ok {
		return walkListop(lp)
	}
	return walkAssign(m)
}

func walkAssign(m *gcre.Match) Expr {
	if m == nil || !m.Ok {
		return nil
	}
	if pf := m.Get("postfix"); pf != nil && pf.Ok {
		if opm := m.Get("assign_op"); opm != nil && opm.Ok {
			return &AssignStmt{Target: walkPostfix(pf), Op: strings.TrimSpace(opm.Str), Value: walkAssign(m.Get("assign_expr"))}
		}
	}
	if t := m.Get("ternary"); t != nil && t.Ok {
		return walkTernary(t)
	}
	return walkTernary(m)
}

func walkTernary(m *gcre.Match) Expr {
	if m == nil || !m.Ok {
		return walkOr(m)
	}
	cond := walkOr(m.Get("or_expr"))
	exprs := m.GetAll("expression")
	if len(exprs) >= 2 {
		return &TernaryExpr{Cond: cond, Then: walkExpr(exprs[0]), Else: walkExpr(exprs[1])}
	}
	if cond != nil {
		return cond
	}
	return walkOr(m)
}

func firstOrSelf(m *gcre.Match, name string) *gcre.Match {
	if m == nil {
		return nil
	}
	if s := m.Get(name); s != nil && s.Ok {
		return s
	}
	return m
}

func walkBinLayer(m *gcre.Match, childName, opName string, child func(*gcre.Match) Expr) Expr {
	if m == nil || !m.Ok {
		return nil
	}
	kids := m.GetAll(childName)
	if len(kids) == 0 {
		if c := m.Get(childName); c != nil && c.Ok {
			return child(c)
		}
		return child(m)
	}
	left := child(kids[0])
	ops := m.GetAll(opName)
	for i := 1; i < len(kids); i++ {
		op := ""
		if i-1 < len(ops) {
			op = strings.TrimSpace(ops[i-1].Str)
		}
		right := child(kids[i])
		if op == "~~" {
			left = &SmartMatchExpr{Left: left, Right: right}
		} else {
			left = &BinaryExpr{Left: left, Op: op, Right: right}
		}
	}
	return left
}

func walkOr(m *gcre.Match) Expr {
	return walkBinLayer(firstOrSelf(m, "or_expr"), "and_expr", "or_op", walkAnd)
}

func walkAnd(m *gcre.Match) Expr {
	return walkBinLayer(firstOrSelf(m, "and_expr"), "cmp_expr", "and_op", walkCmp)
}

func walkCmp(m *gcre.Match) Expr {
	m = firstOrSelf(m, "cmp_expr")
	kids := m.GetAll("add_expr")
	if len(kids) == 0 {
		return walkAdd(m.Get("add_expr"))
	}
	if len(kids) == 1 {
		return walkAdd(kids[0])
	}
	ops := m.GetAll("cmp_op")
	if len(kids) > 2 {
		var exprs []Expr
		var opStrs []string
		for _, k := range kids {
			exprs = append(exprs, walkAdd(k))
		}
		for _, o := range ops {
			opStrs = append(opStrs, strings.TrimSpace(o.Str))
		}
		return &ChainedCompExpr{Exprs: exprs, Ops: opStrs}
	}
	left := walkAdd(kids[0])
	op := ""
	if len(ops) > 0 {
		op = strings.TrimSpace(ops[0].Str)
	}
	right := walkAdd(kids[1])
	if op == "=" {
		return &AssignStmt{Target: left, Op: "=", Value: right}
	}
	if op == "~~" {
		return &SmartMatchExpr{Left: left, Right: right}
	}
	return &BinaryExpr{Left: left, Op: op, Right: right}
}

func walkAdd(m *gcre.Match) Expr {
	return walkBinLayer(firstOrSelf(m, "add_expr"), "mul_expr", "add_op", walkMul)
}

func walkMul(m *gcre.Match) Expr {
	return walkBinLayer(firstOrSelf(m, "mul_expr"), "pow_expr", "mul_op", walkPow)
}

func walkPow(m *gcre.Match) Expr {
	if m == nil || !m.Ok {
		return nil
	}
	// Do not firstOrSelf into the recursive tail capture named pow_expr.
	left := walkRange(m.Get("range_expr"))
	if left == nil {
		if inner := m.Get("pow_expr"); inner != nil && inner.Ok && inner.Get("range_expr") != nil && inner.Get("range_expr").Ok && inner.Get("starstar") != nil && inner.Get("starstar").Ok {
			return walkPow(inner)
		}
		if inner := m.Get("pow_expr"); inner != nil && inner.Ok {
			if re := inner.Get("range_expr"); re != nil && re.Ok {
				return walkPow(inner)
			}
		}
	}
	if ss := m.Get("starstar"); ss != nil && ss.Ok {
		if p := m.Get("pow_expr"); p != nil && p.Ok {
			return &BinaryExpr{Left: left, Op: "**", Right: walkPow(p)}
		}
	}
	if left != nil {
		return left
	}
	return walkRange(m)
}

func walkRange(m *gcre.Match) Expr {
	m = firstOrSelf(m, "range_expr")
	kids := m.GetAll("prefix_expr")
	if len(kids) == 0 {
		return walkPrefix(m.Get("prefix_expr"))
	}
	if len(kids) == 1 {
		return walkPrefix(kids[0])
	}
	return &BinaryExpr{Left: walkPrefix(kids[0]), Op: "..", Right: walkPrefix(kids[1])}
}

func walkPrefix(m *gcre.Match) Expr {
	if m == nil || !m.Ok {
		return nil
	}
	ops := m.GetAll("prefix_op")
	inner := walkPostfix(m.Get("postfix"))
	for i := len(ops) - 1; i >= 0; i-- {
		op := strings.TrimSpace(ops[i].Str)
		if op == "\\" {
			inner = &RefExpr{Expr: inner}
		} else if op == "√" {
			inner = &CallExpr{Callee: &VarExpr{Name: "sqrt"}, Args: []Expr{inner}}
		} else {
			inner = &UnaryExpr{Op: op, Right: inner}
		}
	}
	return inner
}

func walkPostfix(m *gcre.Match) Expr {
	if m == nil || !m.Ok {
		return nil
	}
	left := walkPrimary(m.Get("primary"))
	for _, t := range m.GetAll("postfix_tail") {
		s := strings.TrimSpace(t.Str)
		if strings.HasPrefix(s, ".") {
			meth := ""
			if n := t.Get("name"); n != nil && n.Ok {
				meth = n.Str
			}
			left = &MethodCallExpr{Target: left, Method: meth, Args: walkArglist(t.Get("arglist"))}
			continue
		}
		if strings.HasPrefix(s, "->[") {
			left = &DerefExpr{Kind: DerefArrowArray, Ref: left, Index: walkExpr(t.Get("expression"))}
			continue
		}
		if strings.HasPrefix(s, "->{") {
			left = &DerefExpr{Kind: DerefArrowHash, Ref: left, Index: walkExpr(t.Get("expression"))}
			continue
		}
		if strings.HasPrefix(s, "->(") {
			left = &DerefExpr{Kind: DerefArrowCode, Ref: left, Args: walkArglist(t.Get("arglist"))}
			continue
		}
		if strings.HasPrefix(s, "->") {
			meth := ""
			if n := t.Get("name"); n != nil && n.Ok {
				meth = n.Str
			}
			left = &MethodCallExpr{Target: left, Method: meth, Args: walkArglist(t.Get("arglist"))}
			continue
		}
		if strings.HasPrefix(s, "[") {
			left = &IndexExpr{Array: left, Index: walkExpr(t.Get("expression"))}
			continue
		}
		if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "<") {
			var key Expr
			if e := t.Get("expression"); e != nil && e.Ok {
				key = walkExpr(e)
			} else if n := t.Get("name"); n != nil && n.Ok {
				key = &LiteralExpr{Type: TokString, Value: n.Str}
			}
			left = &HashAccessExpr{Hash: left, Key: key}
			continue
		}
		if strings.HasPrefix(s, "(") {
			left = &CallExpr{Callee: left, Args: walkArglist(t.Get("arglist"))}
		}
	}
	return left
}

func walkArglist(m *gcre.Match) []Expr {
	if m == nil || !m.Ok {
		return nil
	}
	var out []Expr
	for _, e := range m.GetAll("expression") {
		if x := walkExpr(e); x != nil {
			out = append(out, x)
		}
	}
	return out
}

func walkListop(m *gcre.Match) Expr {
	name := ""
	if n := m.Get("listop_name"); n != nil && n.Ok {
		name = n.Str
	}
	var args []Expr
	for _, e := range m.GetAll("expression") {
		args = append(args, walkExpr(e))
	}
	return &CallExpr{Callee: &VarExpr{Name: name}, Args: args}
}

func walkPrimary(m *gcre.Match) Expr {
	if m == nil || !m.Ok {
		return nil
	}
	if g := m.Get("gather_expr"); g != nil && g.Ok {
		return &GatherExpr{Body: walkBlock(g.Get("block"))}
	}
	if s := m.Get("start_expr"); s != nil && s.Ok {
		return &CallExpr{Callee: &VarExpr{Name: "start"}, Args: []Expr{&ClosureExpr{Body: walkBlock(s.Get("block"))}}}
	}
	if a := m.Get("anon_sub"); a != nil && a.Ok {
		return &ClosureExpr{Params: walkParams(a.Get("sig")), Body: walkBlock(a.Get("block"))}
	}
	if a := m.Get("array_lit"); a != nil && a.Ok {
		return &ArrayLiteralExpr{Elements: walkArglist(a.Get("arglist"))}
	}
	if h := m.Get("hash_or_block"); h != nil && h.Ok {
		pairs := h.GetAll("hash_pair")
		if len(pairs) > 0 {
			hl := &HashLiteralExpr{}
			for _, p := range pairs {
				es := p.GetAll("expression")
				if len(es) >= 2 {
					hl.Pairs = append(hl.Pairs, [2]Expr{walkExpr(es[0]), walkExpr(es[1])})
					continue
				}
				if n := p.Get("name"); n != nil && n.Ok {
					var val Expr
					if len(es) == 1 {
						val = walkExpr(es[0])
					} else {
						val = &VarExpr{Name: "$" + n.Str}
					}
					hl.Pairs = append(hl.Pairs, [2]Expr{&LiteralExpr{Type: TokString, Value: n.Str}, val})
				}
			}
			return hl
		}
		if b := h.Get("block"); b != nil && b.Ok {
			return &ClosureExpr{Body: walkBlock(b)}
		}
	}
	if p := m.Get("paren"); p != nil && p.Ok {
		return walkExpr(p.Get("expression"))
	}
	if n := m.Get("number"); n != nil && n.Ok {
		return parseNumber(n.Str)
	}
	if s := m.Get("string"); s != nil && s.Ok {
		return parseString(s.Str)
	}
	if b := m.Get("backtick"); b != nil && b.Ok {
		raw := b.Str
		cmd := raw
		if strings.HasPrefix(raw, "`") {
			cmd = strings.Trim(raw, "`")
		} else if strings.HasPrefix(raw, "qx{") {
			cmd = strings.TrimSuffix(strings.TrimPrefix(raw, "qx{"), "}")
		}
		return &BacktickExpr{Command: &LiteralExpr{Type: TokString, Value: cmd}}
	}
	if v := m.Get("var"); v != nil && v.Ok {
		return &VarExpr{Name: v.Str}
	}
	if lp := m.Get("listop"); lp != nil && lp.Ok {
		return walkListop(lp)
	}
	if u := m.Get("uni_name"); u != nil && u.Ok {
		return &VarExpr{Name: u.Str}
	}
	if n := m.Get("bare_name"); n != nil && n.Ok {
		nm := n.Str
		if nm == "True" || nm == "true" {
			return &LiteralExpr{Type: TokIdent, Value: "True"}
		}
		if nm == "False" || nm == "false" {
			return &LiteralExpr{Type: TokIdent, Value: "False"}
		}
		if nm == "Nil" {
			return &LiteralExpr{Type: TokIdent, Value: "Nil"}
		}
		return &VarExpr{Name: nm}
	}
	st := strings.TrimSpace(m.Str)
	switch st {
	case "...":
		return &StubExpr{Message: "..."}
	case "True", "true":
		return &LiteralExpr{Type: TokIdent, Value: "True"}
	case "False", "false":
		return &LiteralExpr{Type: TokIdent, Value: "False"}
	case "Nil":
		return &LiteralExpr{Type: TokIdent, Value: "Nil"}
	}
	return &LiteralExpr{Type: TokIdent, Value: st}
}

func parseNumber(s string) Expr {
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, ".eE") && !strings.HasPrefix(s, "0x") {
		v, _ := strconv.ParseFloat(s, 64)
		return &LiteralExpr{Type: TokFloat, Value: v}
	}
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		v, _ = strconv.ParseInt(s, 10, 64)
	}
	return &LiteralExpr{Type: TokInt, Value: v}
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			inner := s[1 : len(s)-1]
			inner = strings.ReplaceAll(inner, `\"`, `"`)
			inner = strings.ReplaceAll(inner, `\\`, `\`)
			inner = strings.ReplaceAll(inner, `\n`, "\n")
			inner = strings.ReplaceAll(inner, `\t`, "\t")
			return inner
		}
	}
	return s
}

func parseString(s string) Expr {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "q[") {
		return &LiteralExpr{Type: TokString, Value: strings.TrimSuffix(strings.TrimPrefix(s, "q["), "]")}
	}
	if strings.HasPrefix(s, "qq[") {
		return interpOrLit(strings.TrimSuffix(strings.TrimPrefix(s, "qq["), "]"))
	}
	if len(s) >= 2 && s[0] == '\'' {
		return &LiteralExpr{Type: TokString, Value: s[1 : len(s)-1]}
	}
	if len(s) >= 2 && s[0] == '"' {
		return interpOrLit(unquote(s))
	}
	return &LiteralExpr{Type: TokString, Value: unquote(s)}
}

func interpOrLit(inner string) Expr {
	if !strings.Contains(inner, "$") && !strings.Contains(inner, "@") {
		return &LiteralExpr{Type: TokString, Value: inner}
	}
	var parts []Expr
	var buf strings.Builder
	rs := []rune(inner)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '$' || rs[i] == '@' || rs[i] == '%' {
			if buf.Len() > 0 {
				parts = append(parts, &LiteralExpr{Type: TokString, Value: buf.String()})
				buf.Reset()
			}
			j := i + 1
			for j < len(rs) && (isIdentRune(rs[j]) || rs[j] == '*' || rs[j] == '?' || rs[j] == '!' || rs[j] == '_') {
				j++
			}
			parts = append(parts, &VarExpr{Name: string(rs[i:j])})
			i = j - 1
			continue
		}
		buf.WriteRune(rs[i])
	}
	if buf.Len() > 0 {
		parts = append(parts, &LiteralExpr{Type: TokString, Value: buf.String()})
	}
	if len(parts) == 1 {
		if lit, ok := parts[0].(*LiteralExpr); ok {
			return lit
		}
	}
	return &InterpStringExpr{Parts: parts}
}

func isIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
}
