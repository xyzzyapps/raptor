package raptor

import (
	"strconv"
	"strings"
)

// --- 1. Unboxed Pure-Integer Recursive Function Specialization ---

type intRecKind uint8

const (
	irConst intRecKind = iota
	irParam
	irAdd
	irSub
	irMul
	irDiv
	irMod
	irSelfCall
)

type intRecExpr struct {
	kind intRecKind
	imm  int64
	l, r *intRecExpr
}

type intRecSpec struct {
	cmpOp    string // "<", "<=", "==", ">=", ">"
	cmpConst int64
	baseExpr *intRecExpr
	recExpr  *intRecExpr
}

func (e *intRecExpr) eval(n int64, sp *intRecSpec) int64 {
	switch e.kind {
	case irConst:
		return e.imm
	case irParam:
		return n
	case irAdd:
		return e.l.eval(n, sp) + e.r.eval(n, sp)
	case irSub:
		return e.l.eval(n, sp) - e.r.eval(n, sp)
	case irMul:
		return e.l.eval(n, sp) * e.r.eval(n, sp)
	case irDiv:
		d := e.r.eval(n, sp)
		if d == 0 {
			return 0
		}
		return e.l.eval(n, sp) / d
	case irMod:
		d := e.r.eval(n, sp)
		if d == 0 {
			return 0
		}
		return e.l.eval(n, sp) % d
	case irSelfCall:
		arg := e.l.eval(n, sp)
		return sp.exec(arg)
	}
	return 0
}

func (sp *intRecSpec) exec(n int64) int64 {
	var baseHit bool
	switch sp.cmpOp {
	case "<":
		baseHit = n < sp.cmpConst
	case "<=":
		baseHit = n <= sp.cmpConst
	case "==":
		baseHit = n == sp.cmpConst
	case ">=":
		baseHit = n >= sp.cmpConst
	case ">":
		baseHit = n > sp.cmpConst
	}
	if baseHit {
		return sp.baseExpr.eval(n, sp)
	}
	return sp.recExpr.eval(n, sp)
}

func tryCompileIntRec(subName, paramName string, body *BlockStmt) *intRecSpec {
	if body == nil || len(body.Stmts) != 2 || subName == "" || paramName == "" {
		return nil
	}
	ifSt, ok := body.Stmts[0].(*IfStmt)
	if !ok || len(ifSt.ElsifConds) > 0 || ifSt.ElseBranch != nil || ifSt.ThenBranch == nil || len(ifSt.ThenBranch.Stmts) != 1 {
		return nil
	}
	condBin, ok := ifSt.Condition.(*BinaryExpr)
	if !ok {
		return nil
	}
	switch condBin.Op {
	case "<", "<=", "==", ">=", ">":
	default:
		return nil
	}
	lv, ok := condBin.Left.(*VarExpr)
	if !ok || lv.Name != paramName {
		return nil
	}
	rv, ok := condBin.Right.(*LiteralExpr)
	if !ok || rv.Type != TokInt {
		return nil
	}
	var baseAST Expr
	if ret0, ok := ifSt.ThenBranch.Stmts[0].(*ReturnStmt); ok && ret0.Value != nil {
		baseAST = ret0.Value
	} else if ex0, ok := ifSt.ThenBranch.Stmts[0].(*ExprStmt); ok {
		baseAST = ex0.Expr
	} else {
		return nil
	}

	var recAST Expr
	if ret1, ok := body.Stmts[1].(*ReturnStmt); ok && ret1.Value != nil {
		recAST = ret1.Value
	} else if ex1, ok := body.Stmts[1].(*ExprStmt); ok {
		recAST = ex1.Expr
	} else {
		return nil
	}

	baseIR := buildIntRecExpr(baseAST, subName, paramName)
	recIR := buildIntRecExpr(recAST, subName, paramName)
	if baseIR == nil || recIR == nil {
		return nil
	}
	return &intRecSpec{
		cmpOp:    condBin.Op,
		cmpConst: rv.Value.(int64),
		baseExpr: baseIR,
		recExpr:  recIR,
	}
}

func buildIntRecExpr(e Expr, subName, paramName string) *intRecExpr {
	switch x := e.(type) {
	case *LiteralExpr:
		if x.Type == TokInt {
			return &intRecExpr{kind: irConst, imm: x.Value.(int64)}
		}
	case *VarExpr:
		if x.Name == paramName {
			return &intRecExpr{kind: irParam}
		}
	case *BinaryExpr:
		var k intRecKind
		switch x.Op {
		case "+":
			k = irAdd
		case "-":
			k = irSub
		case "*":
			k = irMul
		case "div":
			k = irDiv
		case "%", "mod":
			k = irMod
		default:
			return nil
		}
		l := buildIntRecExpr(x.Left, subName, paramName)
		r := buildIntRecExpr(x.Right, subName, paramName)
		if l == nil || r == nil {
			return nil
		}
		return &intRecExpr{kind: k, l: l, r: r}
	case *CallExpr:
		if cv, ok := x.Callee.(*VarExpr); ok && cv.Name == subName && len(x.Args) == 1 {
			arg := buildIntRecExpr(x.Args[0], subName, paramName)
			if arg != nil {
				return &intRecExpr{kind: irSelfCall, l: arg}
			}
		}
	}
	return nil
}

// --- 2. Fused Superinstruction Loop Executor ---

type fusedValRef struct {
	kind  SlotKind
	slot  int
	isLit bool
	litI  int64
	litS  string
}

type fusedExprOp uint8

const (
	feRef fusedExprOp = iota
	feAdd
	feSub
	feMul
	feMod
	feDiv
	feHashGetInt
)

type fusedExpr struct {
	op   fusedExprOp
	ref  fusedValRef
	arg2 fusedValRef
	l, r *fusedExpr
}

type fusedCondOp uint8

const (
	fcIntModZero fusedCondOp = iota // expr %% imm == 0
	fcIntTruthy                     // ref != 0
	fcIntLt                         // l < r
	fcIntLe                         // l <= r
	fcIntEq                         // l == r
	fcStrEq                         // s1 eq s2
	fcStrLt                         // s1 lt s2
)

type fusedCond struct {
	op   fusedCondOp
	lhs  *fusedExpr
	rhs  *fusedExpr
	s1   fusedValRef
	s2   fusedValRef
	immI int64
}

type fusedStmtOp uint8

const (
	fsSetInt fusedStmtOp = iota
	fsAddInt
	fsSubInt
	fsAppendStr
	fsPushInt
	fsHashSetInt
	fsIf
)

type fusedStmt struct {
	op       fusedStmtOp
	dst      fusedValRef
	keyRef   fusedValRef
	valExpr  *fusedExpr
	strSuffix fusedValRef
	cond     *fusedCond
	thenBody []fusedStmt
	elseBody []fusedStmt
}

type fusedLoopPlan struct {
	supported bool
	iterRef   fusedValRef
	initExpr  *fusedExpr
	limitExpr *fusedExpr
	inclusive bool
	stepDelta int64
	body      []fusedStmt
	intHashes []fusedValRef
}

type flatIntHash struct {
	keys []int64
	vals []int64
	used []bool
	mask int
}

func newFlatIntHash(capHint int) *flatIntHash {
	sz := 2048
	for sz < capHint*2 {
		sz <<= 1
	}
	return &flatIntHash{
		keys: make([]int64, sz),
		vals: make([]int64, sz),
		used: make([]bool, sz),
		mask: sz - 1,
	}
}

func (h *flatIntHash) get(k int64) (int64, bool) {
	idx := int((uint64(k) * 11400714819323198485) >> 52) & h.mask
	for {
		if !h.used[idx] {
			return 0, false
		}
		if h.keys[idx] == k {
			return h.vals[idx], true
		}
		idx = (idx + 1) & h.mask
	}
}

func (h *flatIntHash) set(k, v int64) {
	idx := int((uint64(k) * 11400714819323198485) >> 52) & h.mask
	for {
		if !h.used[idx] {
			h.used[idx] = true
			h.keys[idx] = k
			h.vals[idx] = v
			return
		}
		if h.keys[idx] == k {
			h.vals[idx] = v
			return
		}
		idx = (idx + 1) & h.mask
	}
}

func compileValRef(e Expr) (fusedValRef, bool) {
	switch x := e.(type) {
	case *VarExpr:
		if x.SlotKind != SlotNone {
			return fusedValRef{kind: x.SlotKind, slot: x.Slot}, true
		}
	case *LiteralExpr:
		if x.Type == TokInt {
			return fusedValRef{isLit: true, litI: x.Value.(int64)}, true
		}
		if x.Type == TokString {
			return fusedValRef{isLit: true, litS: x.Value.(string)}, true
		}
	}
	return fusedValRef{}, false
}

func compileFusedExpr(e Expr, hashRefs *[]fusedValRef) (*fusedExpr, bool) {
	if r, ok := compileValRef(e); ok {
		return &fusedExpr{op: feRef, ref: r}, true
	}
	switch x := e.(type) {
	case *BinaryExpr:
		var op fusedExprOp
		switch x.Op {
		case "+":
			op = feAdd
		case "-":
			op = feSub
		case "*":
			op = feMul
		case "%", "mod":
			op = feMod
		case "div":
			op = feDiv
		default:
			return nil, false
		}
		l, okL := compileFusedExpr(x.Left, hashRefs)
		r, okR := compileFusedExpr(x.Right, hashRefs)
		if !okL || !okR {
			return nil, false
		}
		return &fusedExpr{op: op, l: l, r: r}, true
	case *HashAccessExpr:
		hr, okH := compileValRef(x.Hash)
		kr, okK := compileValRef(x.Key)
		if !okH || hr.isLit || !okK {
			return nil, false
		}
		addUniqueRef(hashRefs, hr)
		return &fusedExpr{op: feHashGetInt, ref: hr, arg2: kr}, true
	}
	return nil, false
}

func addUniqueRef(list *[]fusedValRef, r fusedValRef) {
	for _, ex := range *list {
		if ex.kind == r.kind && ex.slot == r.slot {
			return
		}
	}
	*list = append(*list, r)
}

func compileFusedCond(e Expr, hashRefs *[]fusedValRef) (*fusedCond, bool) {
	switch x := e.(type) {
	case *VarExpr:
		if r, ok := compileValRef(x); ok {
			return &fusedCond{op: fcIntTruthy, lhs: &fusedExpr{op: feRef, ref: r}}, true
		}
	case *BinaryExpr:
		switch x.Op {
		case "%%":
			if lit, ok := x.Right.(*LiteralExpr); ok && lit.Type == TokInt && lit.Value.(int64) != 0 {
				l, okL := compileFusedExpr(x.Left, hashRefs)
				if okL {
					return &fusedCond{op: fcIntModZero, lhs: l, immI: lit.Value.(int64)}, true
				}
			}
		case "<", "<=", "==":
			l, okL := compileFusedExpr(x.Left, hashRefs)
			r, okR := compileFusedExpr(x.Right, hashRefs)
			if okL && okR {
				cop := fcIntEq
				if x.Op == "<" {
					cop = fcIntLt
				} else if x.Op == "<=" {
					cop = fcIntLe
				}
				return &fusedCond{op: cop, lhs: l, rhs: r}, true
			}
		case "eq", "lt":
			s1, ok1 := compileValRef(x.Left)
			s2, ok2 := compileValRef(x.Right)
			if ok1 && ok2 {
				cop := fcStrEq
				if x.Op == "lt" {
					cop = fcStrLt
				}
				return &fusedCond{op: cop, s1: s1, s2: s2}, true
			}
		}
	}
	return nil, false
}

func compileFusedStmts(stmts []Stmt, hashRefs *[]fusedValRef) ([]fusedStmt, bool) {
	out := make([]fusedStmt, 0, len(stmts))
	for _, st := range stmts {
		switch s := st.(type) {
		case *VarDeclStmt:
			if s.SlotKind == SlotNone || s.Scope != "my" || s.Type != "" || s.Where != nil || s.Value == nil {
				return nil, false
			}
			fe, ok := compileFusedExpr(s.Value, hashRefs)
			if !ok {
				return nil, false
			}
			out = append(out, fusedStmt{
				op:      fsSetInt,
				dst:     fusedValRef{kind: s.SlotKind, slot: s.Slot},
				valExpr: fe,
			})
		case *AssignStmt:
			if ha, isH := s.Target.(*HashAccessExpr); isH && s.Op == "=" {
				hr, okH := compileValRef(ha.Hash)
				kr, okK := compileValRef(ha.Key)
				fe, okV := compileFusedExpr(s.Value, hashRefs)
				if !okH || hr.isLit || !okK || !okV {
					return nil, false
				}
				addUniqueRef(hashRefs, hr)
				out = append(out, fusedStmt{op: fsHashSetInt, dst: hr, keyRef: kr, valExpr: fe})
				continue
			}
			dst, okD := compileValRef(s.Target)
			if !okD || dst.isLit {
				return nil, false
			}
			switch s.Op {
			case "+=":
				fe, ok := compileFusedExpr(s.Value, hashRefs)
				if !ok {
					return nil, false
				}
				out = append(out, fusedStmt{op: fsAddInt, dst: dst, valExpr: fe})
			case "-=":
				fe, ok := compileFusedExpr(s.Value, hashRefs)
				if !ok {
					return nil, false
				}
				out = append(out, fusedStmt{op: fsSubInt, dst: dst, valExpr: fe})
			case "~=":
				sr, ok := compileValRef(s.Value)
				if !ok {
					return nil, false
				}
				out = append(out, fusedStmt{op: fsAppendStr, dst: dst, strSuffix: sr})
			case "=":
				if be, isB := s.Value.(*BinaryExpr); isB {
					if be.Op == "~" {
						if lv, isV := be.Left.(*VarExpr); isV && lv.SlotKind == dst.kind && lv.Slot == dst.slot {
							sr, ok := compileValRef(be.Right)
							if !ok {
								return nil, false
							}
							out = append(out, fusedStmt{op: fsAppendStr, dst: dst, strSuffix: sr})
							continue
						}
					}
					if be.Op == "+" || be.Op == "-" {
						if lv, isV := be.Left.(*VarExpr); isV && lv.SlotKind == dst.kind && lv.Slot == dst.slot {
							fe, ok := compileFusedExpr(be.Right, hashRefs)
							if !ok {
								return nil, false
							}
							op := fsAddInt
							if be.Op == "-" {
								op = fsSubInt
							}
							out = append(out, fusedStmt{op: op, dst: dst, valExpr: fe})
							continue
						}
					}
				}
				fe, ok := compileFusedExpr(s.Value, hashRefs)
				if !ok {
					return nil, false
				}
				out = append(out, fusedStmt{op: fsSetInt, dst: dst, valExpr: fe})
			default:
				return nil, false
			}
		case *ExprStmt:
			if inc, isInc := s.Expr.(*IncDecExpr); isInc {
				dst, okD := compileValRef(inc.Target)
				if !okD || dst.isLit {
					return nil, false
				}
				op := fsAddInt
				if inc.Op == "--" {
					op = fsSubInt
				}
				out = append(out, fusedStmt{op: op, dst: dst, valExpr: &fusedExpr{op: feRef, ref: fusedValRef{isLit: true, litI: 1}}})
				continue
			}
			if ce, isC := s.Expr.(*CallExpr); isC {
				if cv, isV := ce.Callee.(*VarExpr); isV && cv.Name == "push" && len(ce.Args) == 2 {
					ar, okA := compileValRef(ce.Args[0])
					fe, okV := compileFusedExpr(ce.Args[1], hashRefs)
					if okA && !ar.isLit && okV {
						out = append(out, fusedStmt{op: fsPushInt, dst: ar, valExpr: fe})
						continue
					}
				}
			}
			if as, isA := s.Expr.(*AssignStmt); isA {
				sub, ok := compileFusedStmts([]Stmt{as}, hashRefs)
				if !ok {
					return nil, false
				}
				out = append(out, sub...)
				continue
			}
			return nil, false
		case *IfStmt:
			if len(s.ElsifConds) > 0 || s.ThenBranch == nil {
				return nil, false
			}
			fc, okC := compileFusedCond(s.Condition, hashRefs)
			if !okC {
				return nil, false
			}
			tb, okT := compileFusedStmts(s.ThenBranch.Stmts, hashRefs)
			if !okT {
				return nil, false
			}
			var eb []fusedStmt
			if s.ElseBranch != nil {
				var okE bool
				eb, okE = compileFusedStmts(s.ElseBranch.Stmts, hashRefs)
				if !okE {
					return nil, false
				}
			}
			out = append(out, fusedStmt{op: fsIf, cond: fc, thenBody: tb, elseBody: eb})
		default:
			return nil, false
		}
	}
	return out, true
}

func buildFusedLoopPlan(s *LoopStmt) *fusedLoopPlan {
	plan := &fusedLoopPlan{}
	if s.Init == nil || s.Cond == nil || s.Step == nil || s.Body == nil || s.Body.HasLabels {
		return plan
	}
	initAs, ok := s.Init.(*AssignStmt)
	if !ok || initAs.Op != "=" {
		return plan
	}
	iterRef, ok := compileValRef(initAs.Target)
	if !ok || iterRef.isLit {
		return plan
	}
	var hashRefs []fusedValRef
	initE, ok := compileFusedExpr(initAs.Value, &hashRefs)
	if !ok || len(hashRefs) > 0 {
		return plan
	}

	condBin, ok := s.Cond.(*BinaryExpr)
	if !ok || (condBin.Op != "<=" && condBin.Op != "<") {
		return plan
	}
	condL, ok := compileValRef(condBin.Left)
	if !ok || condL.kind != iterRef.kind || condL.slot != iterRef.slot {
		return plan
	}
	limitE, ok := compileFusedExpr(condBin.Right, &hashRefs)
	if !ok || len(hashRefs) > 0 {
		return plan
	}

	var stepDelta int64
	switch st := s.Step.(type) {
	case *IncDecExpr:
		sr, ok := compileValRef(st.Target)
		if !ok || sr.kind != iterRef.kind || sr.slot != iterRef.slot {
			return plan
		}
		if st.Op == "++" {
			stepDelta = 1
		} else {
			stepDelta = -1
		}
	case *AssignStmt:
		sr, ok := compileValRef(st.Target)
		if !ok || sr.kind != iterRef.kind || sr.slot != iterRef.slot {
			return plan
		}
		if st.Op == "+=" {
			if lit, ok := st.Value.(*LiteralExpr); ok && lit.Type == TokInt {
				stepDelta = lit.Value.(int64)
			}
		} else if st.Op == "=" {
			if be, ok := st.Value.(*BinaryExpr); ok && be.Op == "+" {
				if lv, ok := compileValRef(be.Left); ok && lv.kind == iterRef.kind && lv.slot == iterRef.slot {
					if lit, ok := be.Right.(*LiteralExpr); ok && lit.Type == TokInt {
						stepDelta = lit.Value.(int64)
					}
				}
			}
		}
	}
	if stepDelta <= 0 {
		return plan
	}

	bodyOps, ok := compileFusedStmts(s.Body.Stmts, &hashRefs)
	if !ok || len(hashRefs) > 1 {
		return plan
	}

	plan.supported = true
	plan.iterRef = iterRef
	plan.initExpr = initE
	plan.limitExpr = limitE
	plan.inclusive = condBin.Op == "<="
	plan.stepDelta = stepDelta
	plan.body = bodyOps
	plan.intHashes = hashRefs
	return plan
}

func (in *Interp) tryExecFusedLoop(s *LoopStmt) (*Value, bool) {
	if in.hasAdviceHooks || len(in.CustomInfixOps) > 0 || len(in.CustomPrefixOps) > 0 {
		return nil, false
	}
	if s.fusedPlan == nil {
		s.fusedPlan = buildFusedLoopPlan(s)
	}
	p := s.fusedPlan
	if !p.supported {
		return nil, false
	}

	var fh *flatIntHash
	var targetHashVal *Value
	if len(p.intHashes) == 1 {
		hv := in.readFusedSlotVal(p.intHashes[0])
		if hv == nil || hv.Type != ValHash || len(hv.HashVal) != 0 {
			return nil, false
		}
		targetHashVal = hv
		fh = newFlatIntHash(2048)
	}

	readInt := func(r fusedValRef) (int64, bool) {
		if r.isLit {
			return r.litI, true
		}
		v := in.readFusedSlotVal(r)
		if v == nil {
			return 0, true
		}
		if v.Type == ValInt {
			return v.IntVal, true
		}
		if v.Type == ValNil {
			return 0, true
		}
		return 0, false
	}

	readStr := func(r fusedValRef) (string, bool) {
		if r.isLit {
			return r.litS, true
		}
		v := in.readFusedSlotVal(r)
		if v != nil && v.Type == ValString {
			return v.StrVal, true
		}
		return "", false
	}

	var evalE func(e *fusedExpr) (int64, bool)
	evalE = func(e *fusedExpr) (int64, bool) {
		switch e.op {
		case feRef:
			return readInt(e.ref)
		case feAdd:
			a, okA := evalE(e.l)
			b, okB := evalE(e.r)
			return a + b, okA && okB
		case feSub:
			a, okA := evalE(e.l)
			b, okB := evalE(e.r)
			return a - b, okA && okB
		case feMul:
			a, okA := evalE(e.l)
			b, okB := evalE(e.r)
			return a * b, okA && okB
		case feMod:
			a, okA := evalE(e.l)
			b, okB := evalE(e.r)
			if !okA || !okB || b == 0 {
				return 0, false
			}
			return a % b, true
		case feDiv:
			a, okA := evalE(e.l)
			b, okB := evalE(e.r)
			if !okA || !okB || b == 0 {
				return 0, false
			}
			return a / b, true
		case feHashGetInt:
			k, okK := readInt(e.arg2)
			if !okK || fh == nil {
				return 0, false
			}
			v, _ := fh.get(k)
			return v, true
		}
		return 0, false
	}

	evalC := func(c *fusedCond) (bool, bool) {
		switch c.op {
		case fcIntModZero:
			v, ok := evalE(c.lhs)
			return v%c.immI == 0, ok
		case fcIntTruthy:
			v, ok := evalE(c.lhs)
			return v != 0, ok
		case fcIntLt:
			a, okA := evalE(c.lhs)
			b, okB := evalE(c.rhs)
			return a < b, okA && okB
		case fcIntLe:
			a, okA := evalE(c.lhs)
			b, okB := evalE(c.rhs)
			return a <= b, okA && okB
		case fcIntEq:
			a, okA := evalE(c.lhs)
			b, okB := evalE(c.rhs)
			return a == b, okA && okB
		case fcStrEq:
			s1, ok1 := readStr(c.s1)
			s2, ok2 := readStr(c.s2)
			return s1 == s2, ok1 && ok2
		case fcStrLt:
			s1, ok1 := readStr(c.s1)
			s2, ok2 := readStr(c.s2)
			return s1 < s2, ok1 && ok2
		}
		return false, false
	}

	startVal, okS := evalE(p.initExpr)
	limitVal, okL := evalE(p.limitExpr)
	if !okS || !okL {
		return nil, false
	}
	if !p.inclusive {
		limitVal--
	}

	// Register-promoted fast path 1: single accumulator `$acc += $i` or `$acc += const`
	if len(p.body) == 1 && p.body[0].op == fsAddInt && p.body[0].valExpr.op == feRef {
		dst := p.body[0].dst
		rhs := p.body[0].valExpr.ref
		if dst != p.iterRef {
			acc, okA := readInt(dst)
			if okA {
				if rhs == p.iterRef {
					i := startVal
					for ; i <= limitVal; i += p.stepDelta {
						acc += i
					}
					in.setVarSlotInt(dst.kind, dst.slot, acc)
					in.setVarSlotInt(p.iterRef.kind, p.iterRef.slot, i)
					return valNil, true
				}
				if rhs.isLit {
					delta := rhs.litI
					i := startVal
					for ; i <= limitVal; i += p.stepDelta {
						acc += delta
					}
					in.setVarSlotInt(dst.kind, dst.slot, acc)
					in.setVarSlotInt(p.iterRef.kind, p.iterRef.slot, i)
					return valNil, true
				}
			}
		}
	}

	// Register-promoted fast path 2: loop-invariant string comparison accumulator (`eq` / `lt`)
	if len(p.body) == 2 &&
		p.body[0].op == fsIf && p.body[0].cond.op == fcStrEq && len(p.body[0].thenBody) == 1 && len(p.body[0].elseBody) == 0 &&
		p.body[1].op == fsIf && p.body[1].cond.op == fcStrLt && len(p.body[1].thenBody) == 1 && len(p.body[1].elseBody) == 0 {
		t0 := p.body[0].thenBody[0]
		t1 := p.body[1].thenBody[0]
		if t0.op == fsAddInt && t0.valExpr.op == feRef && t0.valExpr.ref.isLit &&
			t1.op == fsSubInt && t1.valExpr.op == feRef && t1.valExpr.ref.isLit &&
			t0.dst == t1.dst && t0.dst != p.iterRef &&
			p.body[0].cond.s1 == p.body[1].cond.s1 && p.body[0].cond.s2 == p.body[1].cond.s2 {
			s1, ok1 := readStr(p.body[0].cond.s1)
			s2, ok2 := readStr(p.body[0].cond.s2)
			acc, okA := readInt(t0.dst)
			if ok1 && ok2 && okA {
				d0 := t0.valExpr.ref.litI
				d1 := t1.valExpr.ref.litI
				i := startVal
				for ; i <= limitVal; i += p.stepDelta {
					if s1 == s2 {
						acc += d0
					}
					if s1 < s2 {
						acc -= d1
					}
				}
				in.setVarSlotInt(t0.dst.kind, t0.dst.slot, acc)
				in.setVarSlotInt(p.iterRef.kind, p.iterRef.slot, i)
				return valNil, true
			}
		}
	}

	// Register-promoted fast path 3: filter + map + accumulate (`if $i %% K { $acc += $i + C }`)
	if len(p.body) == 1 && p.body[0].op == fsIf && p.body[0].cond.op == fcIntModZero &&
		p.body[0].cond.lhs.op == feRef && p.body[0].cond.lhs.ref == p.iterRef &&
		len(p.body[0].thenBody) == 1 && len(p.body[0].elseBody) == 0 {
		tb := p.body[0].thenBody[0]
		if tb.op == fsAddInt && tb.dst != p.iterRef &&
			tb.valExpr.op == feAdd && tb.valExpr.l.op == feRef && tb.valExpr.l.ref == p.iterRef &&
			tb.valExpr.r.op == feRef && tb.valExpr.r.ref.isLit {
			acc, okA := readInt(tb.dst)
			if okA {
				modK := p.body[0].cond.immI
				addC := tb.valExpr.r.ref.litI
				i := startVal
				for ; i <= limitVal; i += p.stepDelta {
					if i%modK == 0 {
						acc += i + addC
					}
				}
				in.setVarSlotInt(tb.dst.kind, tb.dst.slot, acc)
				in.setVarSlotInt(p.iterRef.kind, p.iterRef.slot, i)
				return valNil, true
			}
		}
	}

	var execStmts func(ops []fusedStmt) bool
	execStmts = func(ops []fusedStmt) bool {
		for idx := range ops {
			st := &ops[idx]
			switch st.op {
			case fsSetInt:
				v, ok := evalE(st.valExpr)
				if !ok {
					return false
				}
				in.setVarSlotInt(st.dst.kind, st.dst.slot, v)
			case fsAddInt:
				cur, okC := readInt(st.dst)
				dv, okD := evalE(st.valExpr)
				if !okC || !okD {
					return false
				}
				in.setVarSlotInt(st.dst.kind, st.dst.slot, cur+dv)
			case fsSubInt:
				cur, okC := readInt(st.dst)
				dv, okD := evalE(st.valExpr)
				if !okC || !okD {
					return false
				}
				in.setVarSlotInt(st.dst.kind, st.dst.slot, cur-dv)
			case fsAppendStr:
				suf, okS := readStr(st.strSuffix)
				if !okS {
					return false
				}
				cur := in.readFusedSlotVal(st.dst)
				if cur != nil && cur.Type == ValString && !isFlyweight(cur) {
					cur.appendString(suf)
				} else {
					base := ""
					if cur != nil {
						base = cur.String()
					}
					nv := &Value{Type: ValString, StrVal: strings.Clone(base + suf)}
					in.setVarSlot(st.dst.kind, st.dst.slot, nv)
				}
			case fsPushInt:
				arr := in.readFusedSlotVal(st.dst)
				if arr == nil || arr.Type != ValArray || arr.ArrayVal != nil {
					return false
				}
				v, ok := evalE(st.valExpr)
				if !ok {
					return false
				}
				l := arr.ensureLane()
				if l.Floats != nil || l.Strs != nil {
					return false
				}
				l.Ints = append(l.Ints, v)
			case fsHashSetInt:
				k, okK := readInt(st.keyRef)
				v, okV := evalE(st.valExpr)
				if !okK || !okV || fh == nil {
					return false
				}
				fh.set(k, v)
			case fsIf:
				condRes, okC := evalC(st.cond)
				if !okC {
					return false
				}
				if condRes {
					if !execStmts(st.thenBody) {
						return false
					}
				} else if len(st.elseBody) > 0 {
					if !execStmts(st.elseBody) {
						return false
					}
				}
			}
		}
		return true
	}

	i := startVal
	for ; i <= limitVal; i += p.stepDelta {
		in.setVarSlotInt(p.iterRef.kind, p.iterRef.slot, i)
		if !execStmts(p.body) {
			return nil, false
		}
	}
	in.setVarSlotInt(p.iterRef.kind, p.iterRef.slot, i)

	if fh != nil && targetHashVal != nil {
		if targetHashVal.HashVal == nil {
			targetHashVal.HashVal = make(map[string]*Value, 1024)
		}
		for idx, used := range fh.used {
			if used {
				kStr := formatIntFast(fh.keys[idx])
				targetHashVal.HashVal[kStr] = IntValue(fh.vals[idx])
			}
		}
	}

	return valNil, true
}

func (in *Interp) readFusedSlotVal(r fusedValRef) *Value {
	if r.kind == SlotLocal {
		if in.ds.curFrame != nil && r.slot < len(in.ds.curFrame.ptrs) {
			return in.ds.curFrame.ptrs[r.slot]
		}
		return nil
	}
	if in.globals != nil && r.slot < in.globals.count {
		return in.globals.get(r.slot)
	}
	return nil
}

var _ = strconv.Itoa
