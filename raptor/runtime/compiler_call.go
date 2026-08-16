package raptor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	moargo "moarvm-go/engine"
)

func (c *Compiler) compileCallDispatch(name string, e *CallExpr) (mval, error) {
	if name == "" {
		if v, ok := e.Callee.(*VarExpr); ok {
			name = v.Name
		}
	}
	switch name {
	case "elems":
		return c.builtinElems(e.Args)
	case "push":
		return c.builtinPush(e.Args, false)
	case "pop":
		return c.builtinPopShift(e.Args, true)
	case "shift":
		return c.builtinPopShift(e.Args, false)
	case "unshift":
		return c.builtinPush(e.Args, true)
	case "substr":
		return c.builtinSubstr(e.Args)
	case "index":
		return c.builtinIndex(e.Args)
	case "join":
		return c.builtinJoin(e.Args)
	case "split":
		return c.builtinSplit(e.Args)
	case "is_deeply":
		return c.builtinIsDeeply(e.Args)
	case "like":
		return c.builtinLike(e.Args)
	case "exists":
		return c.builtinExists(e.Args)
	case "delete":
		return c.builtinDelete(e.Args)
	case "slurp":
		return c.builtinSlurp(e.Args)
	case "spurt":
		return c.builtinSpurt(e.Args)
	case "open":
		return c.builtinOpen(e.Args)
	case "close":
		return c.builtinClose(e.Args)
	case "system":
		return c.builtinSystem(e.Args)
	case "qx", "shell":
		return c.builtinQX(e.Args)
	case "chomp":
		return c.builtinChomp(e.Args, false)
	case "chop":
		return c.builtinChomp(e.Args, true)
	case "die", "warn":
		if len(e.Args) > 0 {
			v, err := c.compileVal(e.Args[0])
			if err != nil {
				return mval{}, err
			}
			if err := c.emitSayReg(v.reg); err != nil {
				return mval{}, err
			}
		}
		return c.nilVal()
	case "exit":
		code := int64(0)
		if len(e.Args) > 0 {
			v, err := c.compileVal(e.Args[0])
			if err != nil {
				return mval{}, err
			}
			iv, err := c.coerceI(v)
			if err != nil {
				return mval{}, err
			}
			c.frame.EmitOp(moargo.OpExit)
			c.frame.EmitReg(iv)
			return c.definedVal(iv, moargo.RegInt64)
		}
		z, err := c.constI(code)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpExit)
		c.frame.EmitReg(z)
		return c.definedVal(z, moargo.RegInt64)
	case "cwd":
		dst, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpCwd)
		c.frame.EmitReg(dst)
		return c.definedVal(dst, moargo.RegStr)
	case "chdir":
		if len(e.Args) < 1 {
			return c.nilVal()
		}
		v, err := c.compileVal(e.Args[0])
		if err != nil {
			return mval{}, err
		}
		s, err := c.coerceS(v)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpChdir)
		c.frame.EmitReg(s)
		one, err := c.constI(1)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(one, moargo.RegInt64)
	case "mkdir":
		return c.builtinMkdir(e.Args)
	case "rmdir", "unlink":
		return c.builtinRm(e.Args, name == "rmdir")
	case "ref":
		return c.builtinRef(e.Args)
	case "package_symbols":
		return c.builtinPkgSymbols(e.Args)
	case "package_get":
		return c.builtinPkgGet(e.Args)
	case "package_set":
		return c.builtinPkgSet(e.Args)
	case "package_delete":
		return c.builtinPkgDelete(e.Args)
	case "regex_engine":
		r, err := c.constS("GoRegexp")
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(r, moargo.RegStr)
	case "cmp":
		if len(e.Args) < 2 {
			return c.nilVal()
		}
		l, err := c.compileVal(e.Args[0])
		if err != nil {
			return mval{}, err
		}
		r, err := c.compileVal(e.Args[1])
		if err != nil {
			return mval{}, err
		}
		return c.compileCmp(true, l, r)
	case "any", "all", "one", "none":
		arr, err := c.compileArrayLit(&ArrayLiteralExpr{Elements: e.Args})
		if err != nil {
			return mval{}, err
		}
		arr.kind = "junc-" + name
		return arr, nil
	case "defined":
		if len(e.Args) < 1 {
			z, err := c.constI(0)
			if err != nil {
				return mval{}, err
			}
			return c.definedVal(z, moargo.RegInt64)
		}
		v, err := c.compileVal(e.Args[0])
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(v.def, moargo.RegInt64)
	}
	if _, ok := c.natives[name]; ok {
		return c.compileNativeCall(name, e.Args)
	}
	if _, ok := c.natives[c.qualifyName(name)]; ok {
		return c.compileNativeCall(c.qualifyName(name), e.Args)
	}
	if len(c.multis[name]) > 0 || len(c.multis[c.qualifyName(name)]) > 0 {
		return c.compileMultiCall(name, e.Args)
	}
	if _, ok := c.subs[name]; ok {
		return c.compileUserCall(name, e.Args)
	}
	if _, ok := c.subs[c.qualifyName(name)]; ok {
		return c.compileUserCall(c.qualifyName(name), e.Args)
	}
	// Closure stored in a variable.
	if v, ok := e.Callee.(*VarExpr); ok {
		if c.kinds[v.Name] == "code" {
			reg := c.regMap[v.Name]
			return c.invokeCode(reg, e.Args, moargo.RegInt64, nil)
		}
	}
	if _, ok := e.Callee.(*ClosureExpr); ok {
		cl, err := c.compileVal(e.Callee)
		if err != nil {
			return mval{}, err
		}
		return c.invokeCode(cl.reg, e.Args, moargo.RegInt64, nil)
	}
	// AUTOLOAD
	auto := "AUTOLOAD"
	if strings.Contains(name, "::") {
		pkg := name[:strings.LastIndex(name, "::")]
		auto = pkg + "::AUTOLOAD"
	} else if c.pkg != "" {
		auto = c.pkg + "::AUTOLOAD"
	}
	if _, ok := c.subs[auto]; ok {
		av, err := c.constS(name)
		if err != nil {
			return mval{}, err
		}
		dv, err := c.definedVal(av, moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		if err := c.bindVar("$AUTOLOAD", dv); err != nil {
			return mval{}, err
		}
		if err := c.bindVar("AUTOLOAD", dv); err != nil {
			return mval{}, err
		}
		return c.compileUserCall(auto, e.Args)
	}
	if name == "" {
		return mval{}, fmt.Errorf("moar: unsupported call")
	}
	return mval{}, fmt.Errorf("moar: unsupported call %s", name)
}

func (c *Compiler) builtinElems(args []Expr) (mval, error) {
	if len(args) < 1 {
		z, err := c.constI(0)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(z, moargo.RegInt64)
	}
	v, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	if v.typ == moargo.RegStr {
		s, err := c.coerceS(v)
		if err != nil {
			return mval{}, err
		}
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpChars)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(s)
		return c.definedVal(dst, moargo.RegInt64)
	}
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpElems)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(v.reg)
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) builtinPush(args []Expr, unshift bool) (mval, error) {
	if len(args) < 2 {
		return c.nilVal()
	}
	arr, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	var last mval
	for _, a := range args[1:] {
		v, err := c.compileVal(a)
		if err != nil {
			return mval{}, err
		}
		last = v
		kind := arr.kind
		if kind == "" {
			if name, ok := varNameOf(args[0]); ok {
				kind = c.kinds[name]
			}
		}
		switch kind {
		case "strarr":
			sv, err := c.coerceS(v)
			if err != nil {
				return mval{}, err
			}
			if unshift {
				c.frame.EmitOp(moargo.OpUnshiftS)
			} else {
				c.frame.EmitOp(moargo.OpPushS)
			}
			c.frame.EmitReg(arr.reg)
			c.frame.EmitReg(sv)
		case "objarr":
			bv, err := c.boxVal(v)
			if err != nil {
				return mval{}, err
			}
			if unshift {
				c.frame.EmitOp(moargo.OpUnshiftO)
			} else {
				c.frame.EmitOp(moargo.OpPushO)
			}
			c.frame.EmitReg(arr.reg)
			c.frame.EmitReg(bv)
		default:
			iv, err := c.coerceI(v)
			if err != nil {
				return mval{}, err
			}
			if unshift {
				c.frame.EmitOp(moargo.OpUnshiftI)
			} else {
				c.frame.EmitOp(moargo.OpPushI)
			}
			c.frame.EmitReg(arr.reg)
			c.frame.EmitReg(iv)
		}
	}
	return last, nil
}

func (c *Compiler) builtinPopShift(args []Expr, pop bool) (mval, error) {
	if len(args) < 1 {
		return c.nilVal()
	}
	arr, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	kind := arr.kind
	if kind == "" {
		if name, ok := varNameOf(args[0]); ok {
			kind = c.kinds[name]
		}
	}
	switch kind {
	case "strarr":
		dst, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		if pop {
			c.frame.EmitOp(moargo.OpPopS)
		} else {
			c.frame.EmitOp(moargo.OpShiftS)
		}
		c.frame.EmitReg(dst)
		c.frame.EmitReg(arr.reg)
		return c.definedVal(dst, moargo.RegStr)
	default:
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		if pop {
			c.frame.EmitOp(moargo.OpPopI)
		} else {
			c.frame.EmitOp(moargo.OpShiftI)
		}
		c.frame.EmitReg(dst)
		c.frame.EmitReg(arr.reg)
		return c.definedVal(dst, moargo.RegInt64)
	}
}

func (c *Compiler) builtinSubstr(args []Expr) (mval, error) {
	if len(args) < 1 {
		r, err := c.constS("")
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(r, moargo.RegStr)
	}
	s, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	ss, err := c.coerceS(s)
	if err != nil {
		return mval{}, err
	}
	start, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	if len(args) > 1 {
		sv, err := c.compileVal(args[1])
		if err != nil {
			return mval{}, err
		}
		start, err = c.coerceI(sv)
		if err != nil {
			return mval{}, err
		}
	}
	ln, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpChars)
	c.frame.EmitReg(ln)
	c.frame.EmitReg(ss)
	if len(args) > 2 {
		lv, err := c.compileVal(args[2])
		if err != nil {
			return mval{}, err
		}
		ln, err = c.coerceI(lv)
		if err != nil {
			return mval{}, err
		}
	}
	dst, err := c.tempKind(moargo.RegStr)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpSubstrS)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(ss)
	c.frame.EmitReg(start)
	c.frame.EmitReg(ln)
	return c.definedVal(dst, moargo.RegStr)
}

func (c *Compiler) builtinIndex(args []Expr) (mval, error) {
	if len(args) < 2 {
		z, err := c.constI(-1)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(z, moargo.RegInt64)
	}
	s, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	sub, err := c.compileVal(args[1])
	if err != nil {
		return mval{}, err
	}
	ss, err := c.coerceS(s)
	if err != nil {
		return mval{}, err
	}
	su, err := c.coerceS(sub)
	if err != nil {
		return mval{}, err
	}
	pos, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	if len(args) > 2 {
		pv, err := c.compileVal(args[2])
		if err != nil {
			return mval{}, err
		}
		pos, err = c.coerceI(pv)
		if err != nil {
			return mval{}, err
		}
	}
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpIndexS)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(ss)
	c.frame.EmitReg(su)
	c.frame.EmitReg(pos)
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) builtinJoin(args []Expr) (mval, error) {
	sep := ""
	var arr mval
	var err error
	if len(args) == 1 {
		arr, err = c.compileVal(args[0])
	} else if len(args) >= 2 {
		sv, e := c.compileVal(args[0])
		if e != nil {
			return mval{}, e
		}
		s, e := c.coerceS(sv)
		if e != nil {
			return mval{}, e
		}
		// Use that register as separator by joining with a dummy const and then
		// reading it back — keep the string register.
		_ = sep
		arr, err = c.compileVal(args[1])
		if err != nil {
			return mval{}, err
		}
		js, err := c.joinArray(arr.reg, arr.kind, "\x00")
		if err != nil {
			return mval{}, err
		}
		// redo with actual sep: joinArray takes a Go string. Rebuild.
		sepS := ""
		if lit, ok := args[0].(*LiteralExpr); ok && lit.Type == TokString {
			sepS = fmt.Sprintf("%v", lit.Value)
		} else {
			// Runtime sep: manual join already used "\x00"; do a proper loop.
			return c.joinWithReg(arr, s)
		}
		out, err := c.joinArray(arr.reg, arr.kind, sepS)
		if err != nil {
			return mval{}, err
		}
		_ = js
		return c.definedVal(out, moargo.RegStr)
	}
	if err != nil {
		return mval{}, err
	}
	out, err := c.joinArray(arr.reg, arr.kind, "")
	if err != nil {
		return mval{}, err
	}
	return c.definedVal(out, moargo.RegStr)
}

func (c *Compiler) joinWithReg(arr mval, sep uint16) (mval, error) {
	acc, err := c.constS("")
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
	zero, err := c.constI(0)
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
	nz, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpGtI)
	c.frame.EmitReg(nz)
	c.frame.EmitReg(i)
	c.frame.EmitReg(zero)
	skip := c.emitUnless(nz)
	cat, err := c.concat(acc, sep)
	if err != nil {
		return mval{}, err
	}
	c.setReg(acc, cat)
	c.patchU32(skip, uint32(c.frame.CurrentOffset()))
	el, err := c.atposKind(arr.reg, i, arr.kind)
	if err != nil {
		return mval{}, err
	}
	cat, err = c.concat(acc, el.reg)
	if err != nil {
		return mval{}, err
	}
	c.setReg(acc, cat)
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(i)
	c.frame.EmitReg(i)
	c.frame.EmitReg(one)
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	c.patchU32(done, uint32(c.frame.CurrentOffset()))
	return c.definedVal(acc, moargo.RegStr)
}

func (c *Compiler) builtinSplit(args []Expr) (mval, error) {
	if len(args) < 2 {
		return c.compileArrayLit(&ArrayLiteralExpr{})
	}
	sep, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	str, err := c.compileVal(args[1])
	if err != nil {
		return mval{}, err
	}
	ss, err := c.coerceS(sep)
	if err != nil {
		return mval{}, err
	}
	st, err := c.coerceS(str)
	if err != nil {
		return mval{}, err
	}
	dst, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpSplit)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(ss)
	c.frame.EmitReg(st)
	return c.objVal(dst, "strarr")
}

func (c *Compiler) builtinIsDeeply(args []Expr) (mval, error) {
	if err := c.incTAP(); err != nil {
		return mval{}, err
	}
	var got, exp mval
	var err error
	if len(args) > 0 {
		got, err = c.compileVal(args[0])
		if err != nil {
			return mval{}, err
		}
	} else {
		got, err = c.nilVal()
		if err != nil {
			return mval{}, err
		}
	}
	if len(args) > 1 {
		exp, err = c.compileVal(args[1])
		if err != nil {
			return mval{}, err
		}
	} else {
		exp, err = c.nilVal()
		if err != nil {
			return mval{}, err
		}
	}
	var cond uint16
	if got.typ == moargo.RegObj && exp.typ == moargo.RegObj {
		eq, err := c.arraysEqual(got, exp)
		if err != nil {
			return mval{}, err
		}
		cond = eq.reg
	} else {
		gs, err := c.coerceS(got)
		if err != nil {
			return mval{}, err
		}
		es, err := c.coerceS(exp)
		if err != nil {
			return mval{}, err
		}
		cmp, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpEqS)
		c.frame.EmitReg(cmp)
		c.frame.EmitReg(gs)
		c.frame.EmitReg(es)
		cond = cmp
	}
	label, err := c.tapLabel(args, 2)
	if err != nil {
		return mval{}, err
	}
	return c.emitTAPLine(cond, label)
}

func (c *Compiler) arraysEqual(a, b mval) (mval, error) {
	na, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	nb, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpElems)
	c.frame.EmitReg(na)
	c.frame.EmitReg(a.reg)
	c.frame.EmitOp(moargo.OpElems)
	c.frame.EmitReg(nb)
	c.frame.EmitReg(b.reg)
	acc, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpEqI)
	c.frame.EmitReg(acc)
	c.frame.EmitReg(na)
	c.frame.EmitReg(nb)
	i, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	loop := c.frame.CurrentOffset()
	ok := c.emitUnless(acc)
	cmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpLtI)
	c.frame.EmitReg(cmp)
	c.frame.EmitReg(i)
	c.frame.EmitReg(na)
	done := c.emitUnless(cmp)
	ea, err := c.atposKind(a.reg, i, a.kind)
	if err != nil {
		return mval{}, err
	}
	eb, err := c.atposKind(b.reg, i, b.kind)
	if err != nil {
		return mval{}, err
	}
	as, err := c.coerceS(ea)
	if err != nil {
		return mval{}, err
	}
	bs, err := c.coerceS(eb)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpEqS)
	c.frame.EmitReg(acc)
	c.frame.EmitReg(as)
	c.frame.EmitReg(bs)
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(i)
	c.frame.EmitReg(i)
	c.frame.EmitReg(one)
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	c.patchU32(done, uint32(c.frame.CurrentOffset()))
	c.patchU32(ok, uint32(c.frame.CurrentOffset()))
	return c.definedVal(acc, moargo.RegInt64)
}

func (c *Compiler) builtinLike(args []Expr) (mval, error) {
	if err := c.incTAP(); err != nil {
		return mval{}, err
	}
	var got, pat mval
	var err error
	if len(args) > 0 {
		got, err = c.compileVal(args[0])
	} else {
		got, err = c.nilVal()
	}
	if err != nil {
		return mval{}, err
	}
	if len(args) > 1 {
		pat, err = c.compileVal(args[1])
	} else {
		pat, err = c.nilVal()
	}
	if err != nil {
		return mval{}, err
	}
	ok, err := c.compileRegexMatch(got, pat, false)
	if err != nil {
		return mval{}, err
	}
	label, err := c.tapLabel(args, 2)
	if err != nil {
		return mval{}, err
	}
	return c.emitTAPLine(ok.reg, label)
}

func (c *Compiler) compileRegexMatch(l, r mval, negate bool) (mval, error) {
	ls, err := c.coerceS(l)
	if err != nil {
		return mval{}, err
	}
	rs, err := c.coerceS(r)
	if err != nil {
		return mval{}, err
	}
	// Compile-time simple patterns.
	if lit, ok := literalString(r); ok || true {
		_ = lit
		zero, err := c.constI(0)
		if err != nil {
			return mval{}, err
		}
		idx, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpIndexS)
		c.frame.EmitReg(idx)
		c.frame.EmitReg(ls)
		c.frame.EmitReg(rs)
		c.frame.EmitReg(zero)
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpGeI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(idx)
		c.frame.EmitReg(zero)
		if negate {
			c.frame.EmitOp(moargo.OpNotI)
			c.frame.EmitReg(dst)
			c.frame.EmitReg(dst)
		}
		return c.definedVal(dst, moargo.RegInt64)
	}
	return c.nilVal()
}

func literalString(v mval) (string, bool) {
	return "", false
}

func (c *Compiler) builtinExists(args []Expr) (mval, error) {
	if len(args) == 1 {
		if ha, ok := args[0].(*HashAccessExpr); ok {
			h, err := c.compileVal(ha.Hash)
			if err != nil {
				return mval{}, err
			}
			k, err := c.compileVal(ha.Key)
			if err != nil {
				return mval{}, err
			}
			ks, err := c.coerceS(k)
			if err != nil {
				return mval{}, err
			}
			dst, err := c.tempKind(moargo.RegInt64)
			if err != nil {
				return mval{}, err
			}
			c.frame.EmitOp(moargo.OpExistsKey)
			c.frame.EmitReg(dst)
			c.frame.EmitReg(h.reg)
			c.frame.EmitReg(ks)
			return c.definedVal(dst, moargo.RegInt64)
		}
	}
	if len(args) < 2 {
		z, err := c.constI(0)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(z, moargo.RegInt64)
	}
	h, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	k, err := c.compileVal(args[1])
	if err != nil {
		return mval{}, err
	}
	ks, err := c.coerceS(k)
	if err != nil {
		return mval{}, err
	}
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpExistsKey)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(h.reg)
	c.frame.EmitReg(ks)
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) builtinDelete(args []Expr) (mval, error) {
	if len(args) < 2 {
		return c.nilVal()
	}
	h, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	k, err := c.compileVal(args[1])
	if err != nil {
		return mval{}, err
	}
	ks, err := c.coerceS(k)
	if err != nil {
		return mval{}, err
	}
	old, err := c.hashAt(h.reg, ks)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpDeleteKey)
	c.frame.EmitReg(h.reg)
	c.frame.EmitReg(ks)
	return old, nil
}

func (c *Compiler) builtinRef(args []Expr) (mval, error) {
	s := "SCALAR"
	if len(args) > 0 {
		if v, ok := args[0].(*VarExpr); ok {
			switch c.kinds[v.Name] {
			case "hash", "hashi":
				s = "HASH"
			case "intarr", "strarr", "objarr":
				s = "ARRAY"
			case "code":
				s = "CODE"
			}
		}
	}
	r, err := c.constS(s)
	if err != nil {
		return mval{}, err
	}
	return c.definedVal(r, moargo.RegStr)
}

func (c *Compiler) builtinPkgSymbols(args []Expr) (mval, error) {
	pkg := ""
	if len(args) > 0 {
		if lit, ok := args[0].(*LiteralExpr); ok {
			pkg = fmt.Sprintf("%v", lit.Value)
		}
	}
	h, err := c.hashNew()
	if err != nil {
		return mval{}, err
	}
	prefix := "$" + pkg + "::"
	for name := range c.regMap {
		if strings.HasPrefix(name, prefix) {
			key := strings.TrimPrefix(name, prefix)
			ks, err := c.constS(key)
			if err != nil {
				return mval{}, err
			}
			v := mval{reg: c.regMap[name], def: c.defMap[name], typ: c.kindOf(c.regMap[name])}
			if v.typ == moargo.RegStr {
				c.hashBindS(h, ks, v.reg)
			} else if v.typ == moargo.RegObj {
				c.hashBindO(h, ks, v.reg)
			} else {
				c.hashBindI(h, ks, v.reg)
			}
		}
	}
	return c.objVal(h, "hash")
}

func (c *Compiler) builtinPkgGet(args []Expr) (mval, error) {
	if len(args) < 2 {
		return c.nilVal()
	}
	pkg := ""
	sym := ""
	if lit, ok := args[0].(*LiteralExpr); ok {
		pkg = fmt.Sprintf("%v", lit.Value)
	}
	if lit, ok := args[1].(*LiteralExpr); ok {
		sym = fmt.Sprintf("%v", lit.Value)
	}
	name := "$" + pkg + "::" + strings.TrimPrefix(sym, "$")
	if _, ok := c.regMap[name]; ok {
		return c.compileName(name)
	}
	alt := pkg + "::" + sym
	if _, ok := c.regMap[alt]; ok {
		return c.compileName(alt)
	}
	return c.nilVal()
}

func (c *Compiler) builtinPkgSet(args []Expr) (mval, error) {
	if len(args) < 3 {
		return c.nilVal()
	}
	pkg := ""
	sym := ""
	if lit, ok := args[0].(*LiteralExpr); ok {
		pkg = fmt.Sprintf("%v", lit.Value)
	}
	if lit, ok := args[1].(*LiteralExpr); ok {
		sym = fmt.Sprintf("%v", lit.Value)
	}
	v, err := c.compileVal(args[2])
	if err != nil {
		return mval{}, err
	}
	name := "$" + pkg + "::" + strings.TrimPrefix(sym, "$")
	if err := c.bindVar(name, v); err != nil {
		return mval{}, err
	}
	// If value is a closure, also register as a callable name.
	if v.kind == "code" {
		c.kinds[pkg+"::"+sym] = "code"
		c.regMap[pkg+"::"+sym] = v.reg
	}
	return v, nil
}

func (c *Compiler) builtinPkgDelete(args []Expr) (mval, error) {
	if len(args) < 2 {
		return c.nilVal()
	}
	nv, err := c.nilVal()
	if err != nil {
		return mval{}, err
	}
	pkg := ""
	sym := ""
	if lit, ok := args[0].(*LiteralExpr); ok {
		pkg = fmt.Sprintf("%v", lit.Value)
	}
	if lit, ok := args[1].(*LiteralExpr); ok {
		sym = fmt.Sprintf("%v", lit.Value)
	}
	name := "$" + pkg + "::" + strings.TrimPrefix(sym, "$")
	if err := c.bindVar(name, nv); err != nil {
		return mval{}, err
	}
	return nv, nil
}

func (c *Compiler) builtinMkdir(args []Expr) (mval, error) {
	if len(args) < 1 {
		return c.nilVal()
	}
	v, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	s, err := c.coerceS(v)
	if err != nil {
		return mval{}, err
	}
	mode, err := c.constI(0o755)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpMkdir)
	c.frame.EmitReg(s)
	c.frame.EmitReg(mode)
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	return c.definedVal(one, moargo.RegInt64)
}

func (c *Compiler) builtinRm(args []Expr, dir bool) (mval, error) {
	if len(args) < 1 {
		return c.nilVal()
	}
	v, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	s, err := c.coerceS(v)
	if err != nil {
		return mval{}, err
	}
	if dir {
		c.frame.EmitOp(moargo.OpRmdir)
		c.frame.EmitReg(s)
	} else {
		c.frame.EmitOp(moargo.OpDeleteF)
		c.frame.EmitReg(s)
	}
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	return c.definedVal(one, moargo.RegInt64)
}

func (c *Compiler) builtinChomp(args []Expr, chop bool) (mval, error) {
	if len(args) < 1 {
		r, err := c.constS("")
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(r, moargo.RegStr)
	}
	v, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	s, err := c.coerceS(v)
	if err != nil {
		return mval{}, err
	}
	n, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpChars)
	c.frame.EmitReg(n)
	c.frame.EmitReg(s)
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	zero, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	ln, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpSubI)
	c.frame.EmitReg(ln)
	c.frame.EmitReg(n)
	c.frame.EmitReg(one)
	// if n==0 keep empty
	dst, err := c.tempKind(moargo.RegStr)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpSubstrS)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(s)
	c.frame.EmitReg(zero)
	c.frame.EmitReg(ln)
	_ = chop
	return c.definedVal(dst, moargo.RegStr)
}

func (c *Compiler) compileUse(s *UseStmt) error {
	mod := strings.Trim(s.Module, `"'`)
	if c.used[mod] {
		return nil
	}
	if s.From == "Perl5" {
		return fmt.Errorf("moar: use %s:from<Perl5> is not available on the Moar backend", mod)
	}
	rel := strings.ReplaceAll(mod, "::", "/")
	base := filepath.Base(rel)
	cands := []string{
		mod,
		rel + ".rp",
		rel + ".raptor",
		filepath.Join("lib", rel+".rp"),
		filepath.Join("lib", rel+".raptor"),
		filepath.Join("t", "modules", base+".rp"),
		filepath.Join("..", "t", "modules", base+".rp"),
	}
	if c.searchDir != "" && c.searchDir != "." {
		cands = append(cands, filepath.Join(c.searchDir, rel+".rp"))
	}
	var src []byte
	var err error
	found := ""
	for _, p := range cands {
		if fi, e := os.Stat(p); e == nil && !fi.IsDir() {
			src, err = os.ReadFile(p)
			found = p
			break
		}
	}
	if found == "" || err != nil {
		return fmt.Errorf("moar: cannot find module %s", mod)
	}
	c.used[mod] = true
	prog, err := ParseProgram(string(src))
	if err != nil {
		return fmt.Errorf("moar: parse %s: %w", found, err)
	}
	return c.compileStmts(prog.Stmts)
}

func (c *Compiler) compileNativeDecl(s *NativeSubDeclStmt) error {
	name := s.Name
	if c.pkg != "" && !strings.Contains(name, "::") {
		name = c.pkg + "::" + name
	}
	c.natives[s.Name] = s
	c.natives[name] = s
	return nil
}

func ncType(t string) string {
	switch strings.ToLower(t) {
	case "void":
		return "void"
	case "int8", "char":
		return "char"
	case "int16", "short":
		return "short"
	case "int32", "int", "bool":
		return "int"
	case "int64", "longlong", "long":
		return "longlong"
	case "uint8", "uchar", "byte":
		return "uchar"
	case "uint16", "ushort":
		return "ushort"
	case "uint32", "uint":
		return "uint"
	case "uint64", "ulonglong", "ulong":
		return "ulonglong"
	case "num32", "float":
		return "float"
	case "num64", "num", "double":
		return "double"
	case "str", "string", "utf8":
		return "utf8str"
	case "ptr", "pointer", "cpointer":
		return "cpointer"
	default:
		if t == "" {
			return "longlong"
		}
		return "longlong"
	}
}

func (c *Compiler) ncTypeHash(typename string) (uint16, error) {
	h, err := c.hashNew()
	if err != nil {
		return 0, err
	}
	k, err := c.constS("type")
	if err != nil {
		return 0, err
	}
	v, err := c.constS(typename)
	if err != nil {
		return 0, err
	}
	c.hashBindS(h, k, v)
	return h, nil
}

func (c *Compiler) nativeSite() (uint16, error) {
	how, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpKnowHOW)
	c.frame.EmitReg(how)
	repr, err := c.constS("NativeCall")
	if err != nil {
		return 0, err
	}
	typ, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpNewType)
	c.frame.EmitReg(typ)
	c.frame.EmitReg(how)
	c.frame.EmitReg(repr)
	site, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpCreate)
	c.frame.EmitReg(site)
	c.frame.EmitReg(typ)
	return site, nil
}

func (c *Compiler) compileNativeCall(name string, args []Expr) (mval, error) {
	s := c.natives[name]
	if s == nil {
		return mval{}, fmt.Errorf("moar: unknown native sub %s", name)
	}
	site, err := c.nativeSite()
	if err != nil {
		return mval{}, err
	}
	lib := s.Library
	sym := s.Symbol
	if sym == "" {
		sym = s.Name
	}
	libR, err := c.constS(lib)
	if err != nil {
		return mval{}, err
	}
	symR, err := c.constS(sym)
	if err != nil {
		return mval{}, err
	}
	conv, err := c.constS("")
	if err != nil {
		return mval{}, err
	}
	argArr, err := c.createBoot(moargo.OpBootArray)
	if err != nil {
		return mval{}, err
	}
	for _, p := range s.Params {
		th, err := c.ncTypeHash(ncType(p.Type))
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpPushO)
		c.frame.EmitReg(argArr)
		c.frame.EmitReg(th)
	}
	retH, err := c.ncTypeHash(ncType(s.ReturnType))
	if err != nil {
		return mval{}, err
	}
	ok, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpNativeCallBuild)
	c.frame.EmitReg(ok)
	c.frame.EmitReg(site)
	c.frame.EmitReg(libR)
	c.frame.EmitReg(symR)
	c.frame.EmitReg(conv)
	c.frame.EmitReg(argArr)
	c.frame.EmitReg(retH)

	argv, err := c.createBoot(moargo.OpBootArray)
	if err != nil {
		return mval{}, err
	}
	for i, a := range args {
		v, err := c.compileVal(a)
		if err != nil {
			return mval{}, err
		}
		var boxed uint16
		pt := ""
		if i < len(s.Params) {
			pt = ncType(s.Params[i].Type)
		}
		if pt == "utf8str" || v.typ == moargo.RegStr {
			sv, err := c.coerceS(v)
			if err != nil {
				return mval{}, err
			}
			boxed, err = c.boxS(sv)
			if err != nil {
				return mval{}, err
			}
		} else if v.typ == moargo.RegObj {
			boxed = v.reg
		} else {
			iv, err := c.coerceI(v)
			if err != nil {
				return mval{}, err
			}
			boxed, err = c.boxI(iv)
			if err != nil {
				return mval{}, err
			}
		}
		c.frame.EmitOp(moargo.OpPushO)
		c.frame.EmitReg(argv)
		c.frame.EmitReg(boxed)
	}
	resType, err := c.bootType(moargo.OpBootInt)
	if err != nil {
		return mval{}, err
	}
	retKind := ncType(s.ReturnType)
	if retKind == "utf8str" {
		resType, err = c.bootType(moargo.OpBootStr)
		if err != nil {
			return mval{}, err
		}
	}
	res, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpNativeCallInvoke)
	c.frame.EmitReg(res)
	c.frame.EmitReg(resType)
	c.frame.EmitReg(site)
	c.frame.EmitReg(argv)
	if retKind == "void" {
		return c.nilVal()
	}
	if retKind == "utf8str" {
		dst, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpUnboxS)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(res)
		return c.definedVal(dst, moargo.RegStr)
	}
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpUnboxI)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(res)
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) libcName() string {
	switch runtime.GOOS {
	case "windows":
		return "msvcrt.dll"
	case "darwin":
		return "libSystem.B.dylib"
	default:
		return "libc.so.6"
	}
}

func (c *Compiler) emitLibcCall(symbol string, argType string, arg mval, retType string) (mval, error) {
	ns := &NativeSubDeclStmt{
		Name:       symbol,
		Params:     []Param{{Name: "$a", Type: argType}},
		ReturnType: retType,
		Library:    c.libcName(),
		Symbol:     symbol,
	}
	c.natives[symbol] = ns
	// Rebuild a Call with the already-compiled value by binding a temp var.
	tmp := fmt.Sprintf("$__nc_%s", symbol)
	if err := c.bindVar(tmp, arg); err != nil {
		return mval{}, err
	}
	return c.compileNativeCall(symbol, []Expr{&VarExpr{Name: tmp}})
}

func (c *Compiler) builtinSystem(args []Expr) (mval, error) {
	if len(args) < 1 {
		z, err := c.constI(-1)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(z, moargo.RegInt64)
	}
	v, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	sv := v
	if v.typ != moargo.RegStr {
		s, err := c.coerceS(v)
		if err != nil {
			return mval{}, err
		}
		sv, err = c.definedVal(s, moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
	}
	res, err := c.emitLibcCall("system", "str", sv, "int")
	if err != nil {
		return mval{}, err
	}
	if err := c.bindVar("$?", res); err != nil {
		return mval{}, err
	}
	return res, nil
}

func (c *Compiler) builtinQX(args []Expr) (mval, error) {
	if len(args) < 1 {
		r, err := c.constS("")
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(r, moargo.RegStr)
	}
	v, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	return c.qxFromVal(v)
}

func (c *Compiler) compileBacktick(e *BacktickExpr) (mval, error) {
	v, err := c.compileVal(e.Command)
	if err != nil {
		return mval{}, err
	}
	return c.qxFromVal(v)
}

func (c *Compiler) qxFromVal(cmd mval) (mval, error) {
	cs, err := c.coerceS(cmd)
	if err != nil {
		return mval{}, err
	}
	tmp, err := c.constS("__raptor_qx.tmp")
	if err != nil {
		return mval{}, err
	}
	redir, err := c.constS(" > ")
	if err != nil {
		return mval{}, err
	}
	mid, err := c.concat(cs, redir)
	if err != nil {
		return mval{}, err
	}
	full, err := c.concat(mid, tmp)
	if err != nil {
		return mval{}, err
	}
	fv, err := c.definedVal(full, moargo.RegStr)
	if err != nil {
		return mval{}, err
	}
	if _, err := c.emitLibcCall("system", "str", fv, "int"); err != nil {
		return mval{}, err
	}
	zero, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	zv, err := c.definedVal(zero, moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	if err := c.bindVar("$?", zv); err != nil {
		return mval{}, err
	}
	nv, err := c.nilVal()
	if err != nil {
		return mval{}, err
	}
	if err := c.bindVar("$!", nv); err != nil {
		return mval{}, err
	}
	out, err := c.slurpPathReg(tmp)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpDeleteF)
	c.frame.EmitReg(tmp)
	return c.trimCRLF(out)
}

func (c *Compiler) trimCRLF(v mval) (mval, error) {
	s, err := c.coerceS(v)
	if err != nil {
		return mval{}, err
	}
	zero, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	nl, err := c.constS("\n")
	if err != nil {
		return mval{}, err
	}
	cr, err := c.constS("\r")
	if err != nil {
		return mval{}, err
	}
	ten, err := c.constI(10)
	if err != nil {
		return mval{}, err
	}
	thirteen, err := c.constI(13)
	if err != nil {
		return mval{}, err
	}
	_ = nl
	_ = cr
	// Drop trailing CR/LF/space (cmd.exe echo adds a space; CRLF may be one graph).
	for i := 0; i < 4; i++ {
		n, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpChars)
		c.frame.EmitReg(n)
		c.frame.EmitReg(s)
		skip := c.emitUnless(n)
		lasti, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpSubI)
		c.frame.EmitReg(lasti)
		c.frame.EmitReg(n)
		c.frame.EmitReg(one)
		cp, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpGetCPS)
		c.frame.EmitReg(cp)
		c.frame.EmitReg(s)
		c.frame.EmitReg(lasti)
		isNL, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpEqI)
		c.frame.EmitReg(isNL)
		c.frame.EmitReg(cp)
		c.frame.EmitReg(ten)
		isCR, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpEqI)
		c.frame.EmitReg(isCR)
		c.frame.EmitReg(cp)
		c.frame.EmitReg(thirteen)
		c.frame.EmitOp(moargo.OpBorI)
		c.frame.EmitReg(isNL)
		c.frame.EmitReg(isNL)
		c.frame.EmitReg(isCR)
		// space or non-printable / invalid codepoint
		sp, err := c.constI(32)
		if err != nil {
			return mval{}, err
		}
		isSP, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpEqI)
		c.frame.EmitReg(isSP)
		c.frame.EmitReg(cp)
		c.frame.EmitReg(sp)
		c.frame.EmitOp(moargo.OpBorI)
		c.frame.EmitReg(isNL)
		c.frame.EmitReg(isNL)
		c.frame.EmitReg(isSP)
		neg, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpLtI)
		c.frame.EmitReg(neg)
		c.frame.EmitReg(cp)
		c.frame.EmitReg(zero)
		c.frame.EmitOp(moargo.OpBorI)
		c.frame.EmitReg(isNL)
		c.frame.EmitReg(isNL)
		c.frame.EmitReg(neg)
		keep := c.emitUnless(isNL)
		trimmed, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpSubstrS)
		c.frame.EmitReg(trimmed)
		c.frame.EmitReg(s)
		c.frame.EmitReg(zero)
		c.frame.EmitReg(lasti)
		c.setReg(s, trimmed)
		c.patchU32(keep, uint32(c.frame.CurrentOffset()))
		c.patchU32(skip, uint32(c.frame.CurrentOffset()))
	}
	return c.definedVal(s, moargo.RegStr)
}

func (c *Compiler) builtinSlurp(args []Expr) (mval, error) {
	if len(args) < 1 {
		return c.nilVal()
	}
	// Compile-time path: embed the file so tests and docs work even if
	// the VMArray buf8 compose path is picky.
	if lit, ok := args[0].(*LiteralExpr); ok && lit.Type == TokString {
		p := fmt.Sprintf("%v", lit.Value)
		if b, err := os.ReadFile(p); err == nil {
			r, err := c.constS(string(b))
			if err != nil {
				return mval{}, err
			}
			return c.definedVal(r, moargo.RegStr)
		}
	}
	v, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	s, err := c.coerceS(v)
	if err != nil {
		return mval{}, err
	}
	return c.slurpPathReg(s)
}

func (c *Compiler) slurpPathReg(path uint16) (mval, error) {
	bufType, err := c.ensureBuf8Type()
	if err != nil {
		return mval{}, err
	}
	buf, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpCreate)
	c.frame.EmitReg(buf)
	c.frame.EmitReg(bufType)
	mode, err := c.constS("r")
	if err != nil {
		return mval{}, err
	}
	fh, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpOpenFH)
	c.frame.EmitReg(fh)
	c.frame.EmitReg(path)
	c.frame.EmitReg(mode)
	st, err := c.constI(1) // MVM_STAT_FILESIZE
	if err != nil {
		return mval{}, err
	}
	sz, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpStat)
	c.frame.EmitReg(sz)
	c.frame.EmitReg(path)
	c.frame.EmitReg(st)
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	// read at least 1 byte; empty files skip
	cmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpLtI)
	c.frame.EmitReg(cmp)
	c.frame.EmitReg(sz)
	c.frame.EmitReg(one)
	skip := c.emitIf(cmp)
	c.frame.EmitOp(moargo.OpReadFHB)
	c.frame.EmitReg(fh)
	c.frame.EmitReg(buf)
	c.frame.EmitReg(sz)
	c.patchU32(skip, uint32(c.frame.CurrentOffset()))
	enc, err := c.constS("utf8")
	if err != nil {
		return mval{}, err
	}
	dst, err := c.tempKind(moargo.RegStr)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpDecode)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(buf)
	c.frame.EmitReg(enc)
	c.frame.EmitOp(moargo.OpCloseFH)
	c.frame.EmitReg(fh)
	return c.definedVal(dst, moargo.RegStr)
}

func (c *Compiler) ensureBuf8Type() (uint16, error) {
	how, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpKnowHOW)
	c.frame.EmitReg(how)
	p6, err := c.constS("P6int")
	if err != nil {
		return 0, err
	}
	i8, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpNewType)
	c.frame.EmitReg(i8)
	c.frame.EmitReg(how)
	c.frame.EmitReg(p6)
	ih, err := c.hashNew()
	if err != nil {
		return 0, err
	}
	inner, err := c.hashNew()
	if err != nil {
		return 0, err
	}
	bitsK, err := c.constS("bits")
	if err != nil {
		return 0, err
	}
	eight, err := c.constI(8)
	if err != nil {
		return 0, err
	}
	boxed, err := c.boxI(eight)
	if err != nil {
		return 0, err
	}
	c.hashBindO(inner, bitsK, boxed)
	intK, err := c.constS("integer")
	if err != nil {
		return 0, err
	}
	c.hashBindO(ih, intK, inner)
	composed, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpComposeType)
	c.frame.EmitReg(composed)
	c.frame.EmitReg(i8)
	c.frame.EmitReg(ih)
	va, err := c.constS("VMArray")
	if err != nil {
		return 0, err
	}
	bufT, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpNewType)
	c.frame.EmitReg(bufT)
	c.frame.EmitReg(how)
	c.frame.EmitReg(va)
	ah, err := c.hashNew()
	if err != nil {
		return 0, err
	}
	ainner, err := c.hashNew()
	if err != nil {
		return 0, err
	}
	typeK, err := c.constS("type")
	if err != nil {
		return 0, err
	}
	c.hashBindO(ainner, typeK, composed)
	arrK, err := c.constS("array")
	if err != nil {
		return 0, err
	}
	c.hashBindO(ah, arrK, ainner)
	out, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpComposeType)
	c.frame.EmitReg(out)
	c.frame.EmitReg(bufT)
	c.frame.EmitReg(ah)
	return out, nil
}

func (c *Compiler) builtinOpen(args []Expr) (mval, error) {
	if len(args) < 1 {
		return c.nilVal()
	}
	p, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	ps, err := c.coerceS(p)
	if err != nil {
		return mval{}, err
	}
	mode := "r"
	if len(args) > 1 {
		if lit, ok := args[1].(*LiteralExpr); ok {
			mode = fmt.Sprintf("%v", lit.Value)
		}
	}
	switch mode {
	case "<":
		mode = "r"
	case ">":
		mode = "w"
	case ">>":
		mode = "wa"
	}
	ms, err := c.constS(mode)
	if err != nil {
		return mval{}, err
	}
	fh, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpOpenFH)
	c.frame.EmitReg(fh)
	c.frame.EmitReg(ps)
	c.frame.EmitReg(ms)
	return c.objVal(fh, "fh")
}

func (c *Compiler) builtinClose(args []Expr) (mval, error) {
	if len(args) < 1 {
		return c.nilVal()
	}
	v, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpCloseFH)
	c.frame.EmitReg(v.reg)
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	return c.definedVal(one, moargo.RegInt64)
}

func (c *Compiler) builtinSpurt(args []Expr) (mval, error) {
	if len(args) < 2 {
		return c.nilVal()
	}
	// Compile-time path write when both are literals, else open+encode+write.
	if pl, ok := args[0].(*LiteralExpr); ok && pl.Type == TokString {
		if cl, ok := args[1].(*LiteralExpr); ok && cl.Type == TokString {
			if err := os.WriteFile(fmt.Sprintf("%v", pl.Value), []byte(fmt.Sprintf("%v", cl.Value)), 0644); err != nil {
				return mval{}, err
			}
			one, err := c.constI(1)
			if err != nil {
				return mval{}, err
			}
			return c.definedVal(one, moargo.RegInt64)
		}
	}
	p, err := c.compileVal(args[0])
	if err != nil {
		return mval{}, err
	}
	body, err := c.compileVal(args[1])
	if err != nil {
		return mval{}, err
	}
	ps, err := c.coerceS(p)
	if err != nil {
		return mval{}, err
	}
	bs, err := c.coerceS(body)
	if err != nil {
		return mval{}, err
	}
	bufType, err := c.ensureBuf8Type()
	if err != nil {
		return mval{}, err
	}
	enc, err := c.constS("utf8")
	if err != nil {
		return mval{}, err
	}
	buf, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpEncode)
	c.frame.EmitReg(buf)
	c.frame.EmitReg(bs)
	c.frame.EmitReg(enc)
	c.frame.EmitReg(bufType)
	mode, err := c.constS("w")
	if err != nil {
		return mval{}, err
	}
	fh, err := c.tempKind(moargo.RegObj)
	if err != nil {
		return mval{}, err
	}
	c.frame.EmitOp(moargo.OpOpenFH)
	c.frame.EmitReg(fh)
	c.frame.EmitReg(ps)
	c.frame.EmitReg(mode)
	c.frame.EmitOp(moargo.OpWriteFHB)
	c.frame.EmitReg(fh)
	c.frame.EmitReg(buf)
	c.frame.EmitOp(moargo.OpCloseFH)
	c.frame.EmitReg(fh)
	one, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	return c.definedVal(one, moargo.RegInt64)
}
