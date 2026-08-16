package raptor

import (
	"fmt"
	"strings"

	moargo "moarvm-go/engine"
)

func (c *Compiler) qualifyName(name string) string {
	if c.pkg == "" || strings.Contains(name, "::") {
		return name
	}
	if name == "" {
		return name
	}
	if strings.ContainsAny(name[:1], "$@%&") {
		return name[:1] + c.pkg + "::" + name[1:]
	}
	return c.pkg + "::" + name
}

func (c *Compiler) bindQualified(name string) error {
	q := c.qualifyName(name)
	if q == name {
		return nil
	}
	_, err := c.allocReg(q)
	return err
}

func (c *Compiler) pushFrame(name string) int {
	c.stack = append(c.stack, compilerFrame{
		frame:   c.frame,
		regMap:  c.regMap,
		defMap:  c.defMap,
		lexMap:  c.lexMap,
		kinds:   c.kinds,
		bound:   c.bound,
		nextReg: c.nextReg,
		loops:   c.loops,
		tapN:    c.tapN,
		tapInit: c.tapInit,
	})
	parent := c.frame
	f := c.cu.NewFrame(name, moarMaxLocals)
	for i := 0; i < moarMaxLocals; i++ {
		f.SetLocalType(i, moargo.RegInt64)
	}
	if parent != nil {
		f.SetOuter(parent.Index)
	}
	c.frame = f
	c.regMap = make(map[string]uint16)
	c.defMap = make(map[string]uint16)
	c.lexMap = make(map[string]uint16)
	c.kinds = make(map[string]string)
	c.bound = make(map[string]bool)
	c.nextReg = 0
	c.loops = nil
	c.tapInit = false
	return c.cu.NumFrames() - 1
}

func (c *Compiler) popFrame() {
	if len(c.stack) == 0 {
		return
	}
	top := c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]
	c.frame = top.frame
	c.regMap = top.regMap
	c.defMap = top.defMap
	c.lexMap = top.lexMap
	c.kinds = top.kinds
	c.bound = top.bound
	c.nextReg = top.nextReg
	c.loops = top.loops
	c.tapN = top.tapN
	c.tapInit = top.tapInit
}

func (c *Compiler) emitBindLex(lex, outers, src uint16) {
	c.frame.EmitOp(moargo.OpBindLex)
	c.frame.EmitReg(lex)
	c.frame.EmitReg(outers)
	c.frame.EmitReg(src)
}

func (c *Compiler) emitGetLex(dst, lex, outers uint16) {
	c.frame.EmitOp(moargo.OpGetLex)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(lex)
	c.frame.EmitReg(outers)
}

func (c *Compiler) ensureLex(name string) uint16 {
	typ := moargo.RegInt64
	if r, ok := c.regMap[name]; ok {
		typ = c.kindOf(r)
	}
	return c.ensureLexKind(name, typ)
}

func (c *Compiler) ensureLexKind(name string, typ uint16) uint16 {
	if i, ok := c.lexMap[name]; ok {
		return i
	}
	i := c.frame.AddLexical(name, typ)
	c.lexMap[name] = i
	return i
}

func (c *Compiler) bootType(op uint16) (uint16, error) {
	r, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(op)
	c.frame.EmitReg(r)
	return r, nil
}

func (c *Compiler) createBoot(op uint16) (uint16, error) {
	typ, err := c.bootType(op)
	if err != nil {
		return 0, err
	}
	inst, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpCreate)
	c.frame.EmitReg(inst)
	c.frame.EmitReg(typ)
	return inst, nil
}

func (c *Compiler) boxI(v uint16) (uint16, error) {
	t, err := c.bootType(moargo.OpBootInt)
	if err != nil {
		return 0, err
	}
	dst, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpBoxI)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(v)
	c.frame.EmitReg(t)
	return dst, nil
}

func (c *Compiler) boxS(v uint16) (uint16, error) {
	t, err := c.bootType(moargo.OpBootStr)
	if err != nil {
		return 0, err
	}
	dst, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpBoxS)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(v)
	c.frame.EmitReg(t)
	return dst, nil
}

func (c *Compiler) hashNew() (uint16, error) {
	return c.createBoot(moargo.OpBootHash)
}

func (c *Compiler) hashBindS(h, key, val uint16) {
	boxed, err := c.boxS(val)
	if err != nil {
		return
	}
	c.hashBindO(h, key, boxed)
}

func (c *Compiler) hashBindI(h, key, val uint16) {
	boxed, err := c.boxI(val)
	if err != nil {
		return
	}
	c.hashBindO(h, key, boxed)
}

func (c *Compiler) hashBindO(h, key, val uint16) {
	c.frame.EmitOp(moargo.OpBindKeyO)
	c.frame.EmitReg(h)
	c.frame.EmitReg(key)
	c.frame.EmitReg(val)
}

func (c *Compiler) hashAt(h, key uint16) (mval, error) {
	obj, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpAtKeyO)
	c.frame.EmitReg(obj)
	c.frame.EmitReg(h)
	c.frame.EmitReg(key)
	isS, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpIsStr)
	c.frame.EmitReg(isS)
	c.frame.EmitReg(obj)
	dst, err := c.tempKind(moargo.RegStr)
	if err != nil {
		return mval{}, err
	}
	// isstr → unbox_s; otherwise unbox_i and stringify.
	asInt := c.emitUnless(isS)
	c.frame.EmitOp(moargo.OpUnboxS)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(obj)
	end := c.emitGoto()
	c.patchU32(asInt, uint32(c.frame.CurrentOffset()))
	iv, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpUnboxI)
	c.frame.EmitReg(iv)
	c.frame.EmitReg(obj)
	c.frame.EmitOp(moargo.OpCoerceIS)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(iv)
	c.patchU32(end, uint32(c.frame.CurrentOffset()))
	return c.definedVal(dst, moargo.RegStr)
}

func (c *Compiler) compileArrayLit(e *ArrayLiteralExpr) (mval, error) {
	allInt, allStr := true, true
	for _, el := range e.Elements {
		v, err := peekLiteralKind(el)
		if err != nil || v != "int" {
			allInt = false
		}
		if err != nil || v != "str" {
			allStr = false
		}
	}
	kind := "objarr"
	op := moargo.OpBootArray
	if allInt && len(e.Elements) > 0 {
		kind = "intarr"
		op = moargo.OpBootIntArray
	} else if allStr && len(e.Elements) > 0 {
		kind = "strarr"
		op = moargo.OpBootStrArray
	} else if len(e.Elements) == 0 {
		kind = "intarr"
		op = moargo.OpBootIntArray
	}
	arr, err := c.createBoot(op)
	if err != nil {
		return mval{}, err
	}
	for _, el := range e.Elements {
		v, err := c.compileVal(el)
		if err != nil {
			return mval{}, err
		}
		switch kind {
		case "intarr":
			iv, err := c.coerceI(v)
			if err != nil {
				return mval{}, err
			}
			c.frame.EmitOp(moargo.OpPushI)
			c.frame.EmitReg(arr)
			c.frame.EmitReg(iv)
		case "strarr":
			sv, err := c.coerceS(v)
			if err != nil {
				return mval{}, err
			}
			c.frame.EmitOp(moargo.OpPushS)
			c.frame.EmitReg(arr)
			c.frame.EmitReg(sv)
		default:
			boxed, err := c.boxVal(v)
			if err != nil {
				return mval{}, err
			}
			c.frame.EmitOp(moargo.OpPushO)
			c.frame.EmitReg(arr)
			c.frame.EmitReg(boxed)
		}
	}
	return c.objVal(arr, kind)
}

func peekLiteralKind(e Expr) (string, error) {
	switch x := e.(type) {
	case *LiteralExpr:
		switch x.Type {
		case TokInt:
			return "int", nil
		case TokString:
			return "str", nil
		case TokFloat:
			return "num", nil
		}
	}
	return "", fmt.Errorf("not a literal")
}

func (c *Compiler) boxVal(v mval) (uint16, error) {
	switch v.typ {
	case moargo.RegObj:
		return v.reg, nil
	case moargo.RegStr:
		return c.boxS(v.reg)
	default:
		iv, err := c.coerceI(v)
		if err != nil {
			return 0, err
		}
		return c.boxI(iv)
	}
}

func (c *Compiler) objVal(reg uint16, kind string) (mval, error) {
	d, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	return mval{reg: reg, def: d, typ: moargo.RegObj, kind: kind}, nil
}

func (c *Compiler) compileHashLit(e *HashLiteralExpr) (mval, error) {
	h, err := c.hashNew()
	if err != nil {
		return mval{}, err
	}
	for _, pair := range e.Pairs {
		k, err := c.compileVal(pair[0])
		if err != nil {
			return mval{}, err
		}
		ks, err := c.coerceS(k)
		if err != nil {
			return mval{}, err
		}
		val, err := c.compileVal(pair[1])
		if err != nil {
			return mval{}, err
		}
		if val.typ == moargo.RegStr {
			c.hashBindS(h, ks, val.reg)
		} else if val.typ == moargo.RegObj {
			c.hashBindO(h, ks, val.reg)
		} else {
			iv, err := c.coerceI(val)
			if err != nil {
				return mval{}, err
			}
			c.hashBindI(h, ks, iv)
		}
	}
	return c.objVal(h, "hash")
}

func (c *Compiler) compileIndex(e *IndexExpr) (mval, error) {
	arr, err := c.compileVal(e.Array)
	if err != nil {
		return mval{}, err
	}
	idx, err := c.compileVal(e.Index)
	if err != nil {
		return mval{}, err
	}
	ii, err := c.coerceI(idx)
	if err != nil {
		return mval{}, err
	}
	kind := arr.kind
	if kind == "" {
		if name, ok := varNameOf(e.Array); ok {
			kind = c.kinds[name]
		}
	}
	switch kind {
	case "strarr":
		dst, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpAtPosS)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(arr.reg)
		c.frame.EmitReg(ii)
		return c.definedVal(dst, moargo.RegStr)
	case "objarr":
		dst, err := c.tempKind(moargo.RegObj)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpAtPosO)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(arr.reg)
		c.frame.EmitReg(ii)
		return c.objVal(dst, "")
	default:
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpAtPosI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(arr.reg)
		c.frame.EmitReg(ii)
		return c.definedVal(dst, moargo.RegInt64)
	}
}

func varNameOf(e Expr) (string, bool) {
	if v, ok := e.(*VarExpr); ok {
		return v.Name, true
	}
	return "", false
}

func (c *Compiler) compileHashGet(e *HashAccessExpr) (mval, error) {
	h, err := c.compileVal(e.Hash)
	if err != nil {
		return mval{}, err
	}
	k, err := c.compileVal(e.Key)
	if err != nil {
		return mval{}, err
	}
	ks, err := c.coerceS(k)
	if err != nil {
		return mval{}, err
	}
	return c.hashAt(h.reg, ks)
}

func (c *Compiler) compileIndexAssign(e *IndexExpr, op string, val Expr) error {
	arr, err := c.compileVal(e.Array)
	if err != nil {
		return err
	}
	idx, err := c.compileVal(e.Index)
	if err != nil {
		return err
	}
	ii, err := c.coerceI(idx)
	if err != nil {
		return err
	}
	rhs, err := c.compileVal(val)
	if err != nil {
		return err
	}
	// Grow if needed: setelems to idx+1 when idx >= elems.
	n, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpElems)
	c.frame.EmitReg(n)
	c.frame.EmitReg(arr.reg)
	one, err := c.constI(1)
	if err != nil {
		return err
	}
	need, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(need)
	c.frame.EmitReg(ii)
	c.frame.EmitReg(one)
	cmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpGtI)
	c.frame.EmitReg(cmp)
	c.frame.EmitReg(need)
	c.frame.EmitReg(n)
	skip := c.emitUnless(cmp)
	c.frame.EmitOp(moargo.OpSetElemsPos)
	c.frame.EmitReg(arr.reg)
	c.frame.EmitReg(need)
	c.patchU32(skip, uint32(c.frame.CurrentOffset()))

	kind := arr.kind
	if kind == "" {
		if name, ok := varNameOf(e.Array); ok {
			kind = c.kinds[name]
		}
	}
	switch kind {
	case "strarr":
		sv, err := c.coerceS(rhs)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpBindPosS)
		c.frame.EmitReg(arr.reg)
		c.frame.EmitReg(ii)
		c.frame.EmitReg(sv)
	case "objarr":
		bv, err := c.boxVal(rhs)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpBindPosO)
		c.frame.EmitReg(arr.reg)
		c.frame.EmitReg(ii)
		c.frame.EmitReg(bv)
	default:
		iv, err := c.coerceI(rhs)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpBindPosI)
		c.frame.EmitReg(arr.reg)
		c.frame.EmitReg(ii)
		c.frame.EmitReg(iv)
	}
	return nil
}

func (c *Compiler) compileHashAssign(e *HashAccessExpr, op string, val Expr) error {
	h, err := c.compileVal(e.Hash)
	if err != nil {
		return err
	}
	k, err := c.compileVal(e.Key)
	if err != nil {
		return err
	}
	ks, err := c.coerceS(k)
	if err != nil {
		return err
	}
	rhs, err := c.compileVal(val)
	if err != nil {
		return err
	}
	if rhs.typ == moargo.RegStr {
		c.hashBindS(h.reg, ks, rhs.reg)
	} else if rhs.typ == moargo.RegObj {
		c.hashBindO(h.reg, ks, rhs.reg)
	} else {
		iv, err := c.coerceI(rhs)
		if err != nil {
			return err
		}
		c.hashBindI(h.reg, ks, iv)
		if name, ok := varNameOf(e.Hash); ok {
			c.kinds[name] = "hashi"
		}
	}
	return nil
}

func (c *Compiler) compileFieldAssign(e *MethodCallExpr, op string, val Expr) error {
	obj, err := c.compileVal(e.Target)
	if err != nil {
		return err
	}
	rhs, err := c.compileVal(val)
	if err != nil {
		return err
	}
	key, err := c.constS(e.Method)
	if err != nil {
		return err
	}
	if rhs.typ == moargo.RegStr {
		c.hashBindS(obj.reg, key, rhs.reg)
	} else if rhs.typ == moargo.RegObj {
		c.hashBindO(obj.reg, key, rhs.reg)
	} else {
		iv, err := c.coerceI(rhs)
		if err != nil {
			return err
		}
		c.hashBindI(obj.reg, key, iv)
	}
	return nil
}

func (c *Compiler) compileRange(l, r mval) (mval, error) {
	arr, err := c.createBoot(moargo.OpBootIntArray)
	if err != nil {
		return mval{}, err
	}
	i, err := c.coerceI(l)
	if err != nil {
		return mval{}, err
	}
	end, err := c.coerceI(r)
	if err != nil {
		return mval{}, err
	}
	cur, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.setReg(cur, i)
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	loop := c.frame.CurrentOffset()
	cmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpLeI)
	c.frame.EmitReg(cmp)
	c.frame.EmitReg(cur)
	c.frame.EmitReg(end)
	done := c.emitUnless(cmp)
	c.frame.EmitOp(moargo.OpPushI)
	c.frame.EmitReg(arr)
	c.frame.EmitReg(cur)
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(cur)
	c.frame.EmitReg(cur)
	c.frame.EmitReg(one)
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	c.patchU32(done, uint32(c.frame.CurrentOffset()))
	return c.objVal(arr, "intarr")
}

func (c *Compiler) compileXX(l, r mval) (mval, error) {
	count, err := c.coerceI(r)
	if err != nil {
		return mval{}, err
	}
	kind := l.kind
	op := moargo.OpBootIntArray
	if kind == "strarr" || l.typ == moargo.RegStr {
		op = moargo.OpBootStrArray
		if kind == "" {
			kind = "strarr"
		}
	} else if kind == "objarr" {
		op = moargo.OpBootArray
	} else if kind == "" {
		kind = "intarr"
	}
	out, err := c.createBoot(op)
	if err != nil {
		return mval{}, err
	}
	k, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	outer := c.frame.CurrentOffset()
	ocmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpLtI)
	c.frame.EmitReg(ocmp)
	c.frame.EmitReg(k)
	c.frame.EmitReg(count)
	odone := c.emitUnless(ocmp)
	if l.typ == moargo.RegObj && (l.kind == "intarr" || l.kind == "strarr" || l.kind == "objarr") {
		n, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpElems)
		c.frame.EmitReg(n)
		c.frame.EmitReg(l.reg)
		j, err := c.constI(0)
		if err != nil {
			return mval{}, err
		}
		inner := c.frame.CurrentOffset()
		icmp, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpLtI)
		c.frame.EmitReg(icmp)
		c.frame.EmitReg(j)
		c.frame.EmitReg(n)
		idone := c.emitUnless(icmp)
		if err := c.pushElem(out, l.reg, j, l.kind); err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpAddI)
		c.frame.EmitReg(j)
		c.frame.EmitReg(j)
		c.frame.EmitReg(one)
		c.frame.EmitOp(moargo.OpGoto)
		c.frame.EmitInt32(inner)
		c.patchU32(idone, uint32(c.frame.CurrentOffset()))
	} else if l.typ == moargo.RegStr || kind == "strarr" {
		sv, err := c.coerceS(l)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpPushS)
		c.frame.EmitReg(out)
		c.frame.EmitReg(sv)
	} else {
		iv, err := c.coerceI(l)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpPushI)
		c.frame.EmitReg(out)
		c.frame.EmitReg(iv)
	}
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(k)
	c.frame.EmitReg(k)
	c.frame.EmitReg(one)
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(outer)
	c.patchU32(odone, uint32(c.frame.CurrentOffset()))
	return c.objVal(out, kind)
}

func (c *Compiler) pushElem(dst, src, idx uint16, kind string) error {
	switch kind {
	case "strarr":
		v, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpAtPosS)
		c.frame.EmitReg(v)
		c.frame.EmitReg(src)
		c.frame.EmitReg(idx)
		c.frame.EmitOp(moargo.OpPushS)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(v)
	case "objarr":
		v, err := c.tempKind(moargo.RegObj)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpAtPosO)
		c.frame.EmitReg(v)
		c.frame.EmitReg(src)
		c.frame.EmitReg(idx)
		c.frame.EmitOp(moargo.OpPushO)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(v)
	default:
		v, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpAtPosI)
		c.frame.EmitReg(v)
		c.frame.EmitReg(src)
		c.frame.EmitReg(idx)
		c.frame.EmitOp(moargo.OpPushI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(v)
	}
	return nil
}

func (c *Compiler) compileCmp(str bool, l, r mval) (mval, error) {
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	if str || l.typ == moargo.RegStr || r.typ == moargo.RegStr {
		ls, err := c.coerceS(l)
		if err != nil {
			return mval{}, err
		}
		rs, err := c.coerceS(r)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpCmpS)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(ls)
		c.frame.EmitReg(rs)
	} else {
		li, err := c.coerceI(l)
		if err != nil {
			return mval{}, err
		}
		ri, err := c.coerceI(r)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpCmpI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(li)
		c.frame.EmitReg(ri)
	}
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) compileFor(s *ForStmt) error {
	iter, err := c.compileVal(s.Iterable)
	if err != nil {
		return err
	}
	if iter.typ != moargo.RegObj {
		arr, err := c.createBoot(moargo.OpBootIntArray)
		if err != nil {
			return err
		}
		iv, err := c.coerceI(iter)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpPushI)
		c.frame.EmitReg(arr)
		c.frame.EmitReg(iv)
		iter, err = c.objVal(arr, "intarr")
		if err != nil {
			return err
		}
	}
	n, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpElems)
	c.frame.EmitReg(n)
	c.frame.EmitReg(iter.reg)
	i, err := c.constI(0)
	if err != nil {
		return err
	}
	one, err := c.constI(1)
	if err != nil {
		return err
	}
	varname := s.VarName
	if varname == "" {
		varname = "$_"
	}
	c.pushLoop()
	loop := c.frame.CurrentOffset()
	cmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpLtI)
	c.frame.EmitReg(cmp)
	c.frame.EmitReg(i)
	c.frame.EmitReg(n)
	skip := c.emitUnless(cmp)
	el, err := c.atposKind(iter.reg, i, iter.kind)
	if err != nil {
		return err
	}
	if err := c.bindVar(varname, el); err != nil {
		return err
	}
	if varname != "$_" {
		if err := c.bindVar("$_", el); err != nil {
			return err
		}
	}
	if s.Body != nil {
		if err := c.compileStmt(s.Body); err != nil {
			return err
		}
	}
	cont := c.frame.CurrentOffset()
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(i)
	c.frame.EmitReg(i)
	c.frame.EmitReg(one)
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	end := c.frame.CurrentOffset()
	c.patchU32(skip, uint32(end))
	c.popLoop(cont, end)
	return nil
}

func (c *Compiler) atposKind(arr, idx uint16, kind string) (mval, error) {
	switch kind {
	case "strarr":
		dst, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpAtPosS)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(arr)
		c.frame.EmitReg(idx)
		return c.definedVal(dst, moargo.RegStr)
	case "objarr":
		dst, err := c.tempKind(moargo.RegObj)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpAtPosO)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(arr)
		c.frame.EmitReg(idx)
		return c.objVal(dst, "")
	default:
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpAtPosI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(arr)
		c.frame.EmitReg(idx)
		return c.definedVal(dst, moargo.RegInt64)
	}
}

func (c *Compiler) compileGiven(s *GivenStmt) error {
	topic, err := c.compileVal(s.Topic)
	if err != nil {
		return err
	}
	if err := c.bindVar("$_", topic); err != nil {
		return err
	}
	var endPatches []int32
	for _, w := range s.Whens {
		mv, err := c.compileVal(w.Match)
		if err != nil {
			return err
		}
		ok, err := c.compileSmartMatchVals(topic, mv, w.Match)
		if err != nil {
			return err
		}
		skip := c.emitUnless(ok.reg)
		if w.Body != nil {
			if err := c.compileStmt(w.Body); err != nil {
				return err
			}
		}
		endPatches = append(endPatches, c.emitGoto())
		c.patchU32(skip, uint32(c.frame.CurrentOffset()))
	}
	if s.Default != nil {
		if err := c.compileStmt(s.Default); err != nil {
			return err
		}
	}
	end := uint32(c.frame.CurrentOffset())
	for _, p := range endPatches {
		c.patchU32(p, end)
	}
	return nil
}

func (c *Compiler) compileGoto(s *GotoStmt) error {
	if s.IsSub {
		v, err := c.compileCallDispatch(s.Target, &CallExpr{Callee: &VarExpr{Name: s.Target}})
		if err != nil {
			return err
		}
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
	if off, ok := c.labels[s.Target]; ok {
		c.frame.EmitOp(moargo.OpGoto)
		c.frame.EmitInt32(off)
		return nil
	}
	p := c.emitGoto()
	c.gotoPatch[s.Target] = append(c.gotoPatch[s.Target], p)
	return nil
}

func (c *Compiler) compileEnum(s *EnumDeclStmt) error {
	for _, v := range s.Values {
		r, err := c.constI(v.Index)
		if err != nil {
			return err
		}
		dv, err := c.definedVal(r, moargo.RegInt64)
		if err != nil {
			return err
		}
		if err := c.bindVar(v.Name, dv); err != nil {
			return err
		}
		if s.Name != "" {
			if err := c.bindVar(s.Name+"::"+v.Name, dv); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Compiler) compileSmartMatchVals(l, r mval, right Expr) (mval, error) {
	if name := typeNameOf(right); name != "" {
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		want := int64(0)
		switch name {
		case "Int":
			if l.typ == moargo.RegInt64 {
				want = 1
			}
		case "Str":
			if l.typ == moargo.RegStr {
				want = 1
			}
		case "Num":
			if l.typ == moargo.RegNum64 {
				want = 1
			}
		case "Array":
			if l.kind == "intarr" || l.kind == "strarr" || l.kind == "objarr" {
				want = 1
			}
		case "Hash":
			if l.kind == "hash" || l.kind == "hashi" {
				want = 1
			}
		}
		c.frame.EmitOp(moargo.OpConstI64)
		c.frame.EmitReg(dst)
		c.frame.EmitInt64(want)
		return c.definedVal(dst, moargo.RegInt64)
	}
	if r.typ == moargo.RegObj && (r.kind == "intarr" || r.kind == "strarr" || r.kind == "objarr") {
		return c.memberOf(l, r)
	}
	if l.typ == moargo.RegStr || r.typ == moargo.RegStr {
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
		c.frame.EmitOp(moargo.OpEqS)
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
	c.frame.EmitOp(moargo.OpEqI)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(li)
	c.frame.EmitReg(ri)
	return c.definedVal(dst, moargo.RegInt64)
}

func typeNameOf(e Expr) string {
	switch x := e.(type) {
	case *VarExpr:
		switch x.Name {
		case "Int", "Str", "Num", "Array", "Hash", "Bool":
			return x.Name
		}
	case *LiteralExpr:
		if x.Type == TokIdent {
			if s, ok := x.Value.(string); ok {
				switch s {
				case "Int", "Str", "Num", "Array", "Hash", "Bool":
					return s
				}
			}
		}
	}
	return ""
}

func (c *Compiler) memberOf(needle mval, arr mval) (mval, error) {
	acc, err := c.constI(0)
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
	eq, err := c.compileSmartMatchVals(needle, el, nil)
	if err != nil {
		return mval{}, err
	}
	take := c.emitUnless(eq.reg)
	c.setReg(acc, one)
	fin := c.emitGoto()
	c.patchU32(take, uint32(c.frame.CurrentOffset()))
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(i)
	c.frame.EmitReg(i)
	c.frame.EmitReg(one)
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	c.patchU32(done, uint32(c.frame.CurrentOffset()))
	c.patchU32(fin, uint32(c.frame.CurrentOffset()))
	return c.definedVal(acc, moargo.RegInt64)
}

func (c *Compiler) joinArray(arr uint16, kind, sep string) (uint16, error) {
	if kind == "strarr" {
		s, err := c.constS(sep)
		if err != nil {
			return 0, err
		}
		dst, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return 0, err
		}
		c.frame.EmitOp(moargo.OpJoin)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(s)
		c.frame.EmitReg(arr)
		return dst, nil
	}
	acc, err := c.constS("")
	if err != nil {
		return 0, err
	}
	n, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpElems)
	c.frame.EmitReg(n)
	c.frame.EmitReg(arr)
	i, err := c.constI(0)
	if err != nil {
		return 0, err
	}
	one, err := c.constI(1)
	if err != nil {
		return 0, err
	}
	seps, err := c.constS(sep)
	if err != nil {
		return 0, err
	}
	loop := c.frame.CurrentOffset()
	cmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpLtI)
	c.frame.EmitReg(cmp)
	c.frame.EmitReg(i)
	c.frame.EmitReg(n)
	done := c.emitUnless(cmp)
	nz, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return 0, err
	}
	zero, err := c.constI(0)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpGtI)
	c.frame.EmitReg(nz)
	c.frame.EmitReg(i)
	c.frame.EmitReg(zero)
	skipSep := c.emitUnless(nz)
	cat, err := c.concat(acc, seps)
	if err != nil {
		return 0, err
	}
	c.setReg(acc, cat)
	c.patchU32(skipSep, uint32(c.frame.CurrentOffset()))
	el, err := c.atposKind(arr, i, kind)
	if err != nil {
		return 0, err
	}
	cat, err = c.concat(acc, el.reg)
	if err != nil {
		return 0, err
	}
	c.setReg(acc, cat)
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(i)
	c.frame.EmitReg(i)
	c.frame.EmitReg(one)
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	c.patchU32(done, uint32(c.frame.CurrentOffset()))
	return acc, nil
}
