package raptor

import (
	"fmt"
	"reflect"
	"strings"

	moargo "moarvm-go/engine"
)

func (c *Compiler) compileSubDecl(s *SubDeclStmt) error {
	name := s.Name
	if c.pkg != "" && !strings.Contains(name, "::") {
		name = c.pkg + "::" + name
	}
	if s.IsMulti {
		c.multis[s.Name] = append(c.multis[s.Name], s)
		c.multis[name] = append(c.multis[name], s)
		name = fmt.Sprintf("%s__multi_%d", name, len(c.multis[name]))
	}
	kinds := make([]string, len(s.Params))
	for i, p := range s.Params {
		kinds[i] = inferParamKind(p, s.Body)
	}
	ret := c.inferReturnType(s.Body)
	free := freeVars(s.Body, s.Params)
	c.captureOuter(free)
	idx := c.pushFrame("sub_" + strings.ReplaceAll(name, ":", "_"))
	c.subs[s.Name] = idx
	c.subs[name] = idx
	if strings.HasPrefix(s.Name, "&") {
		c.subs[strings.TrimPrefix(s.Name, "&")] = idx
	} else {
		c.subs["&"+s.Name] = idx
		c.subs["&"+name] = idx
	}
	c.subArity[name] = len(s.Params)
	c.subArity[s.Name] = len(s.Params)
	c.subRet[name] = ret
	c.subRet[s.Name] = ret
	c.subParams[name] = kinds
	c.subParams[s.Name] = kinds

	if err := c.loadOuterVars(free); err != nil {
		c.popFrame()
		return err
	}
	n := int16(len(s.Params))
	c.frame.EmitOp(moargo.OpCheckArity)
	c.frame.EmitInt16(n)
	c.frame.EmitInt16(n)
	for i, p := range s.Params {
		reg, err := c.allocReg(p.Name)
		if err != nil {
			c.popFrame()
			return err
		}
		switch kinds[i] {
		case "str":
			c.frame.SetLocalType(int(reg), moargo.RegStr)
			c.frame.EmitOp(moargo.OpParamRpS)
		case "obj":
			c.frame.SetLocalType(int(reg), moargo.RegObj)
			c.frame.EmitOp(moargo.OpParamRpO)
		default:
			c.frame.EmitOp(moargo.OpParamRpI)
		}
		c.frame.EmitReg(reg)
		c.frame.EmitInt16(int16(i))
		one, err := c.constI(1)
		if err != nil {
			c.popFrame()
			return err
		}
		c.setReg(c.defMap[p.Name], one)
		if p.Where != nil {
			if err := c.checkSubset("", p.Where, p.Name); err != nil {
				c.popFrame()
				return err
			}
		}
	}
	if err := c.compileFrameBody(s.Body, ret); err != nil {
		c.popFrame()
		return err
	}
	c.popFrame()
	code, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return err
	}
	c.frame.EmitGetCode(code, uint16(idx))
	clos, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpTakeClosure)
	c.frame.EmitReg(clos)
	c.frame.EmitReg(code)
	c.subClos[s.Name] = clos
	c.subClos[name] = clos
	c.subClos["&"+s.Name] = clos
	lex := c.ensureLexKind("&"+name, moargo.RegObj)
	c.emitBindLex(lex, 0, clos)
	c.subLex[s.Name] = lex
	c.subLex[name] = lex
	c.subLex["&"+s.Name] = lex
	return nil
}

func inferParamKind(p Param, body *BlockStmt) string {
	if strings.HasPrefix(p.Name, "@") || strings.HasPrefix(p.Name, "%") ||
		p.Type == "Array" || p.Type == "Hash" || p.Type == "Callable" {
		return "obj"
	}
	if p.Type == "Str" || p.Type == "string" || p.Type == "str" {
		return "str"
	}
	if p.Type == "Int" || p.Type == "int" || p.Type == "int32" || p.Type == "int64" {
		return "int"
	}
	if body != nil && usesAsString(body, p.Name) {
		return "str"
	}
	return "int"
}

func (c *Compiler) inferReturnType(body *BlockStmt) uint16 {
	if body == nil {
		return moargo.RegInt64
	}
	if returnsString(body) {
		return moargo.RegStr
	}
	if t := gotoSubTarget(body); t != "" {
		if r, ok := c.subRet[t]; ok && r != 0 {
			return r
		}
		if r, ok := c.subRet[strings.TrimPrefix(t, "&")]; ok && r != 0 {
			return r
		}
	}
	return moargo.RegInt64
}

func gotoSubTarget(n Node) string {
	found := ""
	walkNode(n, func(x Node) {
		if g, ok := x.(*GotoStmt); ok && g.IsSub {
			found = g.Target
		}
	})
	return found
}

func (c *Compiler) compileFrameBody(body *BlockStmt, ret uint16) error {
	if body == nil || len(body.Stmts) == 0 {
		return c.emitDefaultReturn(ret)
	}
	last := body.Stmts[len(body.Stmts)-1]
	for _, st := range body.Stmts[:len(body.Stmts)-1] {
		if err := c.compileStmt(st); err != nil {
			return err
		}
	}
	if es, ok := last.(*ExprStmt); ok {
		v, err := c.compileVal(es.Expr)
		if err != nil {
			return err
		}
		return c.emitReturnVal(v)
	}
	if err := c.compileStmt(last); err != nil {
		return err
	}
	return c.emitDefaultReturn(ret)
}

func (c *Compiler) emitReturnVal(v mval) error {
	switch v.typ {
	case moargo.RegStr:
		c.frame.EmitOp(moargo.OpReturnS)
	case moargo.RegNum64:
		c.frame.EmitOp(moargo.OpReturnN)
	case moargo.RegObj:
		c.frame.EmitOp(moargo.OpReturnO)
	default:
		c.frame.EmitOp(moargo.OpReturnI)
	}
	c.frame.EmitReg(v.reg)
	return nil
}

func (c *Compiler) emitDefaultReturn(ret uint16) error {
	if ret == moargo.RegStr {
		empty, err := c.constS("")
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpReturnS)
		c.frame.EmitReg(empty)
		return nil
	}
	z, err := c.constI(0)
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpReturnI)
	c.frame.EmitReg(z)
	return nil
}

func usesAsString(n Node, name string) bool {
	found := false
	walkNode(n, func(x Node) {
		switch e := x.(type) {
		case *BinaryExpr:
			if e.Op == "~" || e.Op == "eq" || e.Op == "ne" || e.Op == "lt" || e.Op == "gt" || e.Op == "le" || e.Op == "ge" || e.Op == "x" {
				if v, ok := e.Left.(*VarExpr); ok && v.Name == name {
					found = true
				}
				if v, ok := e.Right.(*VarExpr); ok && v.Name == name {
					found = true
				}
			}
		case *CallExpr:
			fn := calleeName(e)
			if fn == "chars" || fn == "uc" || fn == "lc" || fn == "substr" || fn == "index" || fn == "split" {
				for _, a := range e.Args {
					if v, ok := a.(*VarExpr); ok && v.Name == name {
						found = true
					}
				}
			}
		}
	})
	return found
}

func returnsString(n Node) bool {
	found := false
	walkNode(n, func(x Node) {
		if r, ok := x.(*ReturnStmt); ok && r.Value != nil {
			switch e := r.Value.(type) {
			case *LiteralExpr:
				if e.Type == TokString {
					found = true
				}
			case *BinaryExpr:
				if e.Op == "~" || e.Op == "x" {
					found = true
				}
			case *InterpStringExpr:
				found = true
			}
		}
	})
	return found
}

func walkNode(n Node, fn func(Node)) {
	if n == nil {
		return
	}
	if rv := reflect.ValueOf(n); rv.Kind() == reflect.Ptr && rv.IsNil() {
		return
	}
	fn(n)
	switch x := n.(type) {
	case *BlockStmt:
		for _, s := range x.Stmts {
			walkNode(s, fn)
		}
	case *Program:
		for _, s := range x.Stmts {
			walkNode(s, fn)
		}
	case *VarDeclStmt:
		walkNode(x.Value, fn)
		walkNode(x.Where, fn)
	case *AssignStmt:
		walkNode(x.Target, fn)
		walkNode(x.Value, fn)
	case *IfStmt:
		walkNode(x.Condition, fn)
		walkNode(x.ThenBranch, fn)
		for _, e := range x.ElsifConds {
			walkNode(e, fn)
		}
		for _, b := range x.ElsifThen {
			walkNode(b, fn)
		}
		walkNode(x.ElseBranch, fn)
	case *UnlessStmt:
		walkNode(x.Condition, fn)
		walkNode(x.Body, fn)
	case *WhileStmt:
		walkNode(x.Condition, fn)
		walkNode(x.Body, fn)
	case *ForStmt:
		walkNode(x.Iterable, fn)
		walkNode(x.Body, fn)
	case *LoopStmt:
		walkNode(x.Init, fn)
		walkNode(x.Cond, fn)
		walkNode(x.Step, fn)
		walkNode(x.Body, fn)
	case *ReturnStmt:
		walkNode(x.Value, fn)
	case *ExprStmt:
		walkNode(x.Expr, fn)
	case *BinaryExpr:
		walkNode(x.Left, fn)
		walkNode(x.Right, fn)
	case *UnaryExpr:
		walkNode(x.Right, fn)
	case *TernaryExpr:
		walkNode(x.Cond, fn)
		walkNode(x.Then, fn)
		walkNode(x.Else, fn)
	case *CallExpr:
		walkNode(x.Callee, fn)
		for _, a := range x.Args {
			walkNode(a, fn)
		}
	case *MethodCallExpr:
		walkNode(x.Target, fn)
		for _, a := range x.Args {
			walkNode(a, fn)
		}
	case *ArrayLiteralExpr:
		for _, e := range x.Elements {
			walkNode(e, fn)
		}
	case *HashLiteralExpr:
		for _, p := range x.Pairs {
			walkNode(p[0], fn)
			walkNode(p[1], fn)
		}
	case *IndexExpr:
		walkNode(x.Array, fn)
		walkNode(x.Index, fn)
	case *HashAccessExpr:
		walkNode(x.Hash, fn)
		walkNode(x.Key, fn)
	case *InterpStringExpr:
		for _, p := range x.Parts {
			walkNode(p, fn)
		}
	case *ChainedCompExpr:
		for _, e := range x.Exprs {
			walkNode(e, fn)
		}
	case *GivenStmt:
		walkNode(x.Topic, fn)
		for _, w := range x.Whens {
			walkNode(w.Match, fn)
			walkNode(w.Body, fn)
		}
		walkNode(x.Default, fn)
	case *ModifierStmt:
		walkNode(x.Target, fn)
		walkNode(x.Condition, fn)
	case *TakeStmt:
		walkNode(x.Value, fn)
	case *GatherExpr:
		walkNode(x.Body, fn)
	case *SmartMatchExpr:
		walkNode(x.Left, fn)
		walkNode(x.Right, fn)
	case *SubDeclStmt:
		walkNode(x.Body, fn)
	case *ClosureExpr:
		walkNode(x.Body, fn)
	}
}

func freeVars(body Node, params []Param) []string {
	locals := map[string]bool{}
	for _, p := range params {
		locals[p.Name] = true
	}
	out := map[string]bool{}
	walkNode(body, func(n Node) {
		switch x := n.(type) {
		case *VarDeclStmt:
			locals[x.Name] = true
		case *VarExpr:
			if !locals[x.Name] && x.Name != "Nil" && x.Name != "True" && x.Name != "true" && x.Name != "False" && x.Name != "false" {
				out[x.Name] = true
			}
		}
	})
	var names []string
	for n := range out {
		names = append(names, n)
	}
	return names
}

func (c *Compiler) captureOuter(names []string) {
	for _, name := range names {
		if _, err := c.allocReg(name); err != nil {
			continue
		}
		if name == "$AUTOLOAD" && !c.bound[name] {
			c.frame.SetLocalType(int(c.regMap[name]), moargo.RegStr)
		}
		c.ensureLex(name)
		if int(c.lexMap[name]) < len(c.frame.Lexicals) && c.frame.Lexicals[c.lexMap[name]].Type == c.kindOf(c.regMap[name]) {
			c.emitBindLex(c.lexMap[name], 0, c.regMap[name])
		}
	}
}

func (c *Compiler) loadOuterVars(names []string) error {
	for _, name := range names {
		if _, ok := c.regMap[name]; ok {
			continue
		}
		outers := uint16(1)
		for i := len(c.stack) - 1; i >= 0; i-- {
			if lex, ok := c.stack[i].lexMap[name]; ok {
				typ := moargo.RegInt64
				if r, ok := c.stack[i].regMap[name]; ok {
					if int(r) < len(c.stack[i].frame.LocalTypes) {
						typ = c.stack[i].frame.LocalTypes[r]
					}
				}
				dst, err := c.tempKind(typ)
				if err != nil {
					return err
				}
				c.emitGetLex(dst, lex, outers)
				c.regMap[name] = dst
				def, err := c.constI(1)
				if err != nil {
					return err
				}
				c.defMap[name] = def
				if k, ok := c.stack[i].kinds[name]; ok {
					c.kinds[name] = k
				}
				break
			}
			outers++
		}
	}
	return nil
}

func (c *Compiler) compileClosure(e *ClosureExpr) (mval, error) {
	free := freeVars(e.Body, e.Params)
	c.captureOuter(free)
	c.lambdaN++
	name := fmt.Sprintf("lambda_%d", c.lambdaN)
	kinds := make([]string, len(e.Params))
	for i, p := range e.Params {
		kinds[i] = inferParamKind(p, e.Body)
	}
	ret := c.inferReturnType(e.Body)
	idx := c.pushFrame(name)
	if err := c.loadOuterVars(free); err != nil {
		c.popFrame()
		return mval{}, err
	}
	n := int16(len(e.Params))
	c.frame.EmitOp(moargo.OpCheckArity)
	c.frame.EmitInt16(n)
	c.frame.EmitInt16(n)
	for i, p := range e.Params {
		reg, err := c.allocReg(p.Name)
		if err != nil {
			c.popFrame()
			return mval{}, err
		}
		switch kinds[i] {
		case "str":
			c.frame.SetLocalType(int(reg), moargo.RegStr)
			c.frame.EmitOp(moargo.OpParamRpS)
		case "obj":
			c.frame.SetLocalType(int(reg), moargo.RegObj)
			c.frame.EmitOp(moargo.OpParamRpO)
		default:
			c.frame.EmitOp(moargo.OpParamRpI)
		}
		c.frame.EmitReg(reg)
		c.frame.EmitInt16(int16(i))
	}
	if err := c.compileFrameBody(e.Body, ret); err != nil {
		c.popFrame()
		return mval{}, err
	}
	c.popFrame()
	code, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitGetCode(code, uint16(idx))
	clos, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpTakeClosure)
	c.frame.EmitReg(clos)
	c.frame.EmitReg(code)
	c.subRet[name] = ret
	c.subParams[name] = kinds
	v, err := c.objVal(clos, "code")
	if err != nil {
		return mval{}, err
	}
	v.kind = "code"
	return v, nil
}

func (c *Compiler) compileMethod(e *MethodCallExpr) (mval, error) {
	if e.Method == "new" {
		if v, ok := e.Target.(*VarExpr); ok {
			if st, ok := c.structs[v.Name]; ok {
				return c.structNew(st)
			}
			// Type name as ident
			if st, ok := c.structs[fmt.Sprintf("%v", v.Name)]; ok {
				return c.structNew(st)
			}
		}
		if lit, ok := e.Target.(*LiteralExpr); ok {
			if name, ok := lit.Value.(string); ok {
				if st, ok := c.structs[name]; ok {
					return c.structNew(st)
				}
			}
		}
		h, err := c.hashNew()
		if err != nil {
			return mval{}, err
		}
		return c.objVal(h, "struct")
	}
	obj, err := c.compileVal(e.Target)
	if err != nil {
		return mval{}, err
	}
	// Struct / hash field
	if len(e.Args) == 0 {
		key, err := c.constS(e.Method)
		if err != nil {
			return mval{}, err
		}
		return c.hashAt(obj.reg, key)
	}
	if len(e.Args) == 1 && (obj.kind == "struct" || obj.kind == "hash" || obj.kind == "hashi") {
		key, err := c.constS(e.Method)
		if err != nil {
			return mval{}, err
		}
		rhs, err := c.compileVal(e.Args[0])
		if err != nil {
			return mval{}, err
		}
		iv, err := c.coerceI(rhs)
		if err != nil {
			return mval{}, err
		}
		c.hashBindI(obj.reg, key, iv)
		return rhs, nil
	}
	// UFCS: method(obj, args...)
	args := append([]Expr{e.Target}, e.Args...)
	return c.compileCallDispatch(e.Method, &CallExpr{Callee: &VarExpr{Name: e.Method}, Args: args})
}

func (c *Compiler) structNew(st *CStructDeclStmt) (mval, error) {
	h, err := c.hashNew()
	if err != nil {
		return mval{}, err
	}
	for _, f := range st.Fields {
		key, err := c.constS(f.Name)
		if err != nil {
			return mval{}, err
		}
		z, err := c.constI(0)
		if err != nil {
			return mval{}, err
		}
		c.hashBindI(h, key, z)
	}
	return c.objVal(h, "struct")
}

func (c *Compiler) compileSubsetDecl(s *SubsetDeclStmt) error {
	c.subsets[s.Name] = s.Where
	return nil
}

func (c *Compiler) checkSubset(typ string, where Expr, varName string) error {
	pred := where
	if pred == nil && typ != "" {
		pred = c.subsets[typ]
	}
	if pred == nil {
		return nil
	}
	cur, err := c.compileName(varName)
	if err != nil {
		return err
	}
	if err := c.bindVar("$_", cur); err != nil {
		return err
	}
	var cond mval
	if cl, ok := pred.(*ClosureExpr); ok {
		// Evaluate body; last expression is the predicate.
		if cl.Body != nil {
			for i, st := range cl.Body.Stmts {
				if i == len(cl.Body.Stmts)-1 {
					if es, ok := st.(*ExprStmt); ok {
						cond, err = c.compileVal(es.Expr)
						if err != nil {
							return err
						}
						continue
					}
					if rs, ok := st.(*ReturnStmt); ok && rs.Value != nil {
						cond, err = c.compileVal(rs.Value)
						if err != nil {
							return err
						}
						continue
					}
				}
				if err := c.compileStmt(st); err != nil {
					return err
				}
			}
		}
		if cond.reg == 0 {
			cond, err = c.compileVal(cl)
			if err != nil {
				return err
			}
		}
	} else {
		cond, err = c.compileVal(pred)
		if err != nil {
			return err
		}
	}
	okJ := c.emitIf(cond.reg)
	msg, err := c.constS("subset " + typ + " failed for " + varName)
	if err != nil {
		return err
	}
	if err := c.emitSayReg(msg); err != nil {
		return err
	}
	c.patchU32(okJ, uint32(c.frame.CurrentOffset()))
	return nil
}

func (c *Compiler) compileStructDecl(s *CStructDeclStmt) error {
	c.structs[s.Name] = s
	return nil
}

func (c *Compiler) compilePackage(s *PackageDeclStmt) error {
	if s.Body != nil {
		prev := c.pkg
		c.pkg = s.Name
		err := c.compileStmts(s.Body.Stmts)
		c.pkg = prev
		return err
	}
	c.pkg = s.Name
	return nil
}

func (c *Compiler) compileTake(s *TakeStmt) error {
	if len(c.gathers) == 0 {
		return fmt.Errorf("moar: take outside gather")
	}
	v, err := c.compileVal(s.Value)
	if err != nil {
		return err
	}
	arr := c.gathers[len(c.gathers)-1]
	if v.typ == moargo.RegStr {
		c.frame.EmitOp(moargo.OpPushS)
		c.frame.EmitReg(arr)
		c.frame.EmitReg(v.reg)
	} else if v.typ == moargo.RegObj {
		c.frame.EmitOp(moargo.OpPushO)
		c.frame.EmitReg(arr)
		c.frame.EmitReg(v.reg)
	} else {
		iv, err := c.coerceI(v)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpPushI)
		c.frame.EmitReg(arr)
		c.frame.EmitReg(iv)
	}
	return nil
}

func (c *Compiler) compileGather(e *GatherExpr) (mval, error) {
	arr, err := c.createBoot(moargo.OpBootIntArray)
	if err != nil {
		return mval{}, err
	}
	c.gathers = append(c.gathers, arr)
	if e.Body != nil {
		if err := c.compileStmt(e.Body); err != nil {
			c.gathers = c.gathers[:len(c.gathers)-1]
			return mval{}, err
		}
	}
	c.gathers = c.gathers[:len(c.gathers)-1]
	return c.objVal(arr, "intarr")
}

func (c *Compiler) compileGrammar(s *GrammarDeclStmt) error {
	c.grammars[s.Name] = s
	h, err := c.hashNew()
	if err != nil {
		return err
	}
	nameK, err := c.constS("name")
	if err != nil {
		return err
	}
	nameV, err := c.constS(s.Name)
	if err != nil {
		return err
	}
	c.hashBindS(h, nameK, nameV)
	for _, r := range s.Rules {
		k, err := c.constS(r.Name)
		if err != nil {
			return err
		}
		v, err := c.constS(r.Pattern)
		if err != nil {
			return err
		}
		c.hashBindS(h, k, v)
	}
	dv, err := c.objVal(h, "hash")
	if err != nil {
		return err
	}
	return c.bindVar(s.Name, dv)
}

func (c *Compiler) compileDeref(e *DerefExpr) (mval, error) {
	inner, err := c.compileVal(e.Ref)
	if err != nil {
		return mval{}, err
	}
	switch e.Kind {
	case DerefScalar, DerefArray, DerefHash, DerefCode:
		return inner, nil
	case DerefArrowArray:
		return c.compileIndex(&IndexExpr{Array: e.Ref, Index: e.Index})
	case DerefArrowHash:
		return c.compileHashGet(&HashAccessExpr{Hash: e.Ref, Key: e.Index})
	case DerefArrowCode:
		return c.compileCallDispatch("", &CallExpr{Callee: e.Ref, Args: e.Args})
	default:
		return inner, nil
	}
}

func (c *Compiler) invokeCode(codeReg uint16, args []Expr, retTyp uint16, paramKinds []string) (mval, error) {
	flags := []uint8{moargo.CallArgObj}
	regs := []uint16{codeReg}
	for i, a := range args {
		v, err := c.compileVal(a)
		if err != nil {
			return mval{}, err
		}
		kind := ""
		if i < len(paramKinds) {
			kind = paramKinds[i]
		}
		switch kind {
		case "str":
			sv, err := c.coerceS(v)
			if err != nil {
				return mval{}, err
			}
			flags = append(flags, moargo.CallArgStr)
			regs = append(regs, sv)
		case "obj":
			if v.typ != moargo.RegObj {
				bv, err := c.boxVal(v)
				if err != nil {
					return mval{}, err
				}
				v.reg = bv
			}
			flags = append(flags, moargo.CallArgObj)
			regs = append(regs, v.reg)
		default:
			if v.typ == moargo.RegStr {
				flags = append(flags, moargo.CallArgStr)
				regs = append(regs, v.reg)
			} else if v.typ == moargo.RegObj {
				flags = append(flags, moargo.CallArgObj)
				regs = append(regs, v.reg)
			} else {
				iv, err := c.coerceI(v)
				if err != nil {
					return mval{}, err
				}
				flags = append(flags, moargo.CallArgInt)
				regs = append(regs, iv)
			}
		}
	}
	cs := c.cu.AddCallsite(flags)
	if retTyp == moargo.RegStr {
		dst, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitDispatchS(dst, "boot-code", cs, regs...)
		return c.definedVal(dst, moargo.RegStr)
	}
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitDispatchI(dst, "boot-code", cs, regs...)
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) compileUserCall(name string, args []Expr) (mval, error) {
	idx, ok := c.subs[name]
	if !ok {
		idx, ok = c.subs[c.qualifyName(name)]
	}
	if !ok && c.pkg != "" {
		idx, ok = c.subs[c.pkg+"::"+name]
	}
	if !ok {
		return mval{}, fmt.Errorf("moar: unknown sub %s", name)
	}
	_ = idx
	clos, err := c.loadSubClos(name)
	if err != nil {
		return mval{}, err
	}
	ret := c.subRet[name]
	if ret == 0 {
		ret = c.subRet[c.qualifyName(name)]
	}
	kinds := c.subParams[name]
	if kinds == nil {
		kinds = c.subParams[c.qualifyName(name)]
	}
	return c.invokeCode(clos, args, ret, kinds)
}

func (c *Compiler) loadSubClos(name string) (uint16, error) {
	if r, ok := c.subClos[name]; ok && len(c.stack) == 0 {
		return r, nil
	}
	q := c.qualifyName(name)
	lex, ok := c.subLex[name]
	if !ok {
		lex, ok = c.subLex[q]
	}
	if !ok {
		lex, ok = c.subLex["&"+name]
	}
	if !ok {
		// Fall back to getcode+takeclosure in this frame.
		idx, found := c.subs[name]
		if !found {
			idx, found = c.subs[q]
		}
		if !found {
			return 0, fmt.Errorf("moar: unknown sub %s", name)
		}
		code, err := c.tempKind(moargo.RegObj)
		if err != nil {
			return 0, err
		}
		c.frame.EmitGetCode(code, uint16(idx))
		clos, err := c.tempKind(moargo.RegObj)
		if err != nil {
			return 0, err
		}
		c.frame.EmitOp(moargo.OpTakeClosure)
		c.frame.EmitReg(clos)
		c.frame.EmitReg(code)
		return clos, nil
	}
	dst, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.emitGetLex(dst, lex, uint16(len(c.stack)))
	return dst, nil
}

func (c *Compiler) compileMultiCall(name string, args []Expr) (mval, error) {
	cands := c.multis[name]
	if len(cands) == 0 {
		cands = c.multis[c.qualifyName(name)]
	}
	// Evaluate args once.
	compiled := make([]mval, len(args))
	var err error
	for i, a := range args {
		compiled[i], err = c.compileVal(a)
		if err != nil {
			return mval{}, err
		}
	}
	var end []int32
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	for _, cand := range cands {
		// Bind params and test where.
		okReg, err := c.constI(1)
		if err != nil {
			return mval{}, err
		}
		for i, p := range cand.Params {
			if i >= len(compiled) {
				z, err := c.constI(0)
				if err != nil {
					return mval{}, err
				}
				c.setReg(okReg, z)
				break
			}
			if err := c.bindVar(p.Name, compiled[i]); err != nil {
				return mval{}, err
			}
			if p.Where != nil {
				if err := c.bindVar("$_", compiled[i]); err != nil {
					return mval{}, err
				}
				var cond mval
				if cl, ok := p.Where.(*ClosureExpr); ok && cl.Body != nil && len(cl.Body.Stmts) > 0 {
					if es, ok := cl.Body.Stmts[0].(*ExprStmt); ok {
						cond, err = c.compileVal(es.Expr)
					} else {
						cond, err = c.compileVal(p.Where)
					}
				} else {
					cond, err = c.compileVal(p.Where)
				}
				if err != nil {
					return mval{}, err
				}
				c.frame.EmitOp(moargo.OpMulI)
				c.frame.EmitReg(okReg)
				c.frame.EmitReg(okReg)
				c.frame.EmitReg(cond.reg)
			}
		}
		skip := c.emitUnless(okReg)
		qname := cand.Name
		if c.pkg != "" && !strings.Contains(qname, "::") {
			qname = c.pkg + "::" + qname
		}
		// Find the frame for this candidate: first matching name__multi_N
		// Fall back to name.
		mv, err := c.compileUserCall(cand.Name, args)
		if err != nil {
			// try generated multi name
			found := false
			for n := range c.subs {
				if strings.HasPrefix(n, qname+"__multi_") || strings.HasPrefix(n, cand.Name+"__multi_") {
					mv, err = c.compileUserCall(n, args)
					if err == nil {
						found = true
						break
					}
				}
			}
			if !found {
				return mval{}, err
			}
		}
		if err := c.emitMove(dst, mv.reg); err != nil {
			return mval{}, err
		}
		end = append(end, c.emitGoto())
		c.patchU32(skip, uint32(c.frame.CurrentOffset()))
	}
	fin := uint32(c.frame.CurrentOffset())
	for _, p := range end {
		c.patchU32(p, fin)
	}
	return c.definedVal(dst, moargo.RegInt64)
}

func junctionOf(e Expr) (string, []Expr, bool) {
	call, ok := e.(*CallExpr)
	if !ok {
		return "", nil, false
	}
	name := calleeName(call)
	switch name {
	case "any", "all", "one", "none":
		return name, call.Args, true
	}
	return "", nil, false
}

func (c *Compiler) compileJunctionCmp(op string, other Expr, kind string, args []Expr, junctionOnLeft bool) (mval, error) {
	// Build comparison of `other` against each arg, then reduce with kind.
	ov, err := c.compileVal(other)
	if err != nil {
		return mval{}, err
	}
	// If a single array arg, iterate it.
	if len(args) == 1 {
		if _, ok := args[0].(*VarExpr); ok {
			arr, err := c.compileVal(args[0])
			if err != nil {
				return mval{}, err
			}
			if arr.typ == moargo.RegObj {
				return c.juncArray(op, ov, arr, kind, junctionOnLeft)
			}
		}
	}
	init := int64(0)
	if kind == "all" || kind == "none" {
		init = 1
	}
	acc, err := c.constI(init)
	if err != nil {
		return mval{}, err
	}
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	zero, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	for _, a := range args {
		av, err := c.compileVal(a)
		if err != nil {
			return mval{}, err
		}
		cmp, err := c.cmpVals(op, ov, av, junctionOnLeft)
		if err != nil {
			return mval{}, err
		}
		switch kind {
		case "any":
			c.frame.EmitOp(moargo.OpBorI)
			c.frame.EmitReg(acc)
			c.frame.EmitReg(acc)
			c.frame.EmitReg(cmp.reg)
		case "all":
			c.frame.EmitOp(moargo.OpMulI)
			c.frame.EmitReg(acc)
			c.frame.EmitReg(acc)
			c.frame.EmitReg(cmp.reg)
		case "none":
			not, err := c.tempKind(moargo.RegInt64)
			if err != nil {
				return mval{}, err
			}
			c.frame.EmitOp(moargo.OpNotI)
			c.frame.EmitReg(not)
			c.frame.EmitReg(cmp.reg)
			c.frame.EmitOp(moargo.OpMulI)
			c.frame.EmitReg(acc)
			c.frame.EmitReg(acc)
			c.frame.EmitReg(not)
		case "one":
			// acc += cmp; later acc == 1
			c.frame.EmitOp(moargo.OpAddI)
			c.frame.EmitReg(acc)
			c.frame.EmitReg(acc)
			c.frame.EmitReg(cmp.reg)
			_ = one
			_ = zero
		}
	}
	if kind == "one" {
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpEqI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(one)
		acc = dst
	}
	return c.definedVal(acc, moargo.RegInt64)
}

func (c *Compiler) juncArray(op string, ov mval, arr mval, kind string, jleft bool) (mval, error) {
	init := int64(0)
	if kind == "all" || kind == "none" {
		init = 1
	}
	acc, err := c.constI(init)
	if err != nil {
		return mval{}, err
	}
	n, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpElems)
	c.frame.EmitReg(n)
	c.frame.EmitReg(arr.reg)
	i, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	loop := c.frame.CurrentOffset()
	cmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpLtI)
	c.frame.EmitReg(cmp)
	c.frame.EmitReg(i)
	c.frame.EmitReg(n)
	done := c.emitUnless(cmp)
	el, err := c.atposKind(arr.reg, i, arr.kind)
	if err != nil {
		return mval{}, err
	}
	cv, err := c.cmpVals(op, ov, el, jleft)
	if err != nil {
		return mval{}, err
	}
	switch kind {
	case "any":
		c.frame.EmitOp(moargo.OpBorI)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(cv.reg)
	case "all":
		c.frame.EmitOp(moargo.OpMulI)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(cv.reg)
	case "none":
		not, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpNotI)
		c.frame.EmitReg(not)
		c.frame.EmitReg(cv.reg)
		c.frame.EmitOp(moargo.OpMulI)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(not)
	case "one":
		c.frame.EmitOp(moargo.OpAddI)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(cv.reg)
	}
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(i)
	c.frame.EmitReg(i)
	c.frame.EmitReg(one)
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	c.patchU32(done, uint32(c.frame.CurrentOffset()))
	if kind == "one" {
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpEqI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(one)
		acc = dst
	}
	return c.definedVal(acc, moargo.RegInt64)
}

func (c *Compiler) cmpVals(op string, a, b mval, swap bool) (mval, error) {
	l, r := a, b
	if swap {
		l, r = b, a
	}
	if op == "~~" {
		return c.compileSmartMatchVals(l, r, nil)
	}
	if op == "eq" || op == "ne" || op == "lt" || op == "gt" || op == "le" || op == "ge" {
		ls, err := c.coerceS(l)
		if err != nil {
			return mval{}, err
		}
		rs, err := c.coerceS(r)
		if err != nil {
			return mval{}, err
		}
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		code, _, ok := binOp(op)
		if !ok {
			return mval{}, fmt.Errorf("moar: bad junction op %s", op)
		}
		c.frame.EmitOp(code)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(ls)
		c.frame.EmitReg(rs)
		return c.definedVal(dst, moargo.RegInt64)
	}
	li, err := c.coerceI(l)
	if err != nil {
		return mval{}, err
	}
	ri, err := c.coerceI(r)
	if err != nil {
		return mval{}, err
	}
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	code, _, ok := binOp(op)
	if !ok {
		return mval{}, fmt.Errorf("moar: bad junction op %s", op)
	}
	c.frame.EmitOp(code)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(li)
	c.frame.EmitReg(ri)
	return c.definedVal(dst, moargo.RegInt64)
}
