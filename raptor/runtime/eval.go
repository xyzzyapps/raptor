package raptor

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	goruntime "runtime"
	"strconv"
	"strings"
	"unsafe"

	"gcre"
)

// ReturnSignal unwinds the stack upon reaching 'return'.
type ReturnSignal struct {
	Value *Value
}

func (r *ReturnSignal) Error() string {
	return fmt.Sprintf("return: %s", r.Value.String())
}

// BreakSignal unwinds loop execution on 'last'.
type BreakSignal struct{}

func (b *BreakSignal) Error() string { return "break" }

// ContinueSignal unwinds loop iteration on 'next'.
type ContinueSignal struct{}

func (c *ContinueSignal) Error() string { return "continue" }

// RedoSignal restarts the current loop iteration on 'redo'.
type RedoSignal struct{}

func (r *RedoSignal) Error() string { return "redo" }

// GotoSignal unwinds stack on 'goto'.
type GotoSignal struct {
	Target string
	IsSub  bool
}

func (g *GotoSignal) Error() string {
	return "goto: " + g.Target
}

// Interp is the AST tree-walking evaluation interpreter for Raku5.
type Interp struct {
	GlobalEnv        *Env
	Stdout           io.Writer
	Stderr           io.Writer
	Builtins         map[string]BuiltinFunc
	Structs          map[string]*CStructDeclStmt
	BeforeHooks      map[string][]*AdviceHookStmt
	AfterHooks       map[string][]*AdviceHookStmt
	AroundHooks      map[string][]*AdviceHookStmt
	GatherStack      [][]*Value
	CustomInfixOps   map[string]*Value
	CustomPrefixOps  map[string]*Value
	Packages         map[string]map[string]*Value
	CurrentPackage   string
	StateVars        map[Stmt]*StateCell
	Grammars         map[string]*GrammarDeclStmt
	CompiledGrammars map[string]*gcre.Grammar
	CurrentEnv       *Env
	ds               dataStack
	globals          *globalSlotTable
	hasAdviceHooks   bool
	builtinEpoch     uint32
}

// BuiltinFunc is a standard library function signature in Raku5.
type BuiltinFunc func(in *Interp, args []*Value) (*Value, error)

// NewInterp creates a new Raku5 interpreter.
func NewInterp() *Interp {
	in := &Interp{
		GlobalEnv:       NewEnv(),
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
		Builtins:        make(map[string]BuiltinFunc),
		Structs:         make(map[string]*CStructDeclStmt),
		BeforeHooks:     make(map[string][]*AdviceHookStmt),
		AfterHooks:      make(map[string][]*AdviceHookStmt),
		AroundHooks:     make(map[string][]*AdviceHookStmt),
		CustomInfixOps:  make(map[string]*Value),
		CustomPrefixOps: make(map[string]*Value),
		Packages:        make(map[string]map[string]*Value),
		CurrentPackage:  "main",
		StateVars:        make(map[Stmt]*StateCell),
		Grammars:         make(map[string]*GrammarDeclStmt),
		CompiledGrammars: make(map[string]*gcre.Grammar),
		globals:          newGlobalSlotTable(),
	}
	in.Packages["main"] = make(map[string]*Value)
	in.registerBuiltins()
	in.registerIOBuiltins()
	in.registerFFI()
	in.registerPerl5Bridge()
	in.registerConcurrencyAndAtomics()
	in.registerSocketBuiltins()
	in.registerHTTPBuiltins()
	in.registerWebSocketBuiltins()
	in.registerPortAudioBuiltins()
	in.registerSQLiteBuiltins()
	in.registerJSONBuiltins()
	registerTUIBuiltins(in)
	registerTAPBuiltins(in)
	registerVerificationBuiltins(in)
	in.registerPodLitBuiltins()
	in.registerWebBuiltins()
	in.registerGGMLBuiltins()
	in.registerTinyLLMBuiltins()
	in.registerMoarVMModuleBuiltins()
	in.registerEmbeddedBuiltins()

	// Predefine mathematical constants
	in.GlobalEnv.Define("$pi", FloatValue(math.Pi))
	in.GlobalEnv.Define("$e", FloatValue(math.E))
	in.GlobalEnv.Define("$PI", FloatValue(math.Pi))
	in.GlobalEnv.Define("$E", FloatValue(math.E))
	in.Builtins["pi"] = func(in *Interp, args []*Value) (*Value, error) {
		return FloatValue(math.Pi), nil
	}
	in.Builtins["e"] = func(in *Interp, args []*Value) (*Value, error) {
		return FloatValue(math.E), nil
	}
	in.Builtins["∑"] = func(in *Interp, args []*Value) (*Value, error) {
		var sum float64
		for _, a := range args {
			if a.Type == ValArray {
				if ints := a.intLane(); ints != nil {
					for _, n := range ints {
						sum += float64(n)
					}
				} else if floats := a.floatLane(); floats != nil {
					for _, f := range floats {
						sum += f
					}
				} else {
					for _, e := range a.ArrayVal {
						sum += in.toFloat(e)
					}
				}
			} else {
				sum += in.toFloat(a)
			}
		}
		if sum == float64(int64(sum)) {
			return IntValue(int64(sum)), nil
		}
		return FloatValue(sum), nil
	}
	in.Builtins["∏"] = func(in *Interp, args []*Value) (*Value, error) {
		prod := 1.0
		for _, a := range args {
			if a.Type == ValArray {
				if ints := a.intLane(); ints != nil {
					for _, n := range ints {
						prod *= float64(n)
					}
				} else if floats := a.floatLane(); floats != nil {
					for _, f := range floats {
						prod *= f
					}
				} else {
					for _, e := range a.ArrayVal {
						prod *= in.toFloat(e)
					}
				}
			} else {
				prod *= in.toFloat(a)
			}
		}
		if prod == float64(int64(prod)) {
			return IntValue(int64(prod)), nil
		}
		return FloatValue(prod), nil
	}
	in.Builtins["√"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Sqrt(in.toFloat(args[0]))), nil
	}

	return in
}

func (in *Interp) getVar(ve *VarExpr, env *Env) (*Value, bool) {
	switch ve.SlotKind {
	case SlotLocal:
		if in.ds.curFrame != nil && ve.Slot < len(in.ds.curFrame.ptrs) {
			v := in.ds.curFrame.ptrs[ve.Slot]
			if v != nil {
				return v, true
			}
		}
	case SlotGlobal:
		if in.globals != nil && ve.Slot < in.globals.count {
			v := in.globals.get(ve.Slot)
			if v != nil {
				return v, true
			}
		}
	}
	if v, ok := env.Lookup(ve.Name); ok {
		return v, true
	}
	if env != in.GlobalEnv {
		if v, ok := in.GlobalEnv.Lookup(ve.Name); ok {
			return v, true
		}
	}
	return nil, false
}

func (in *Interp) setVarSlot(kind SlotKind, slot int, val *Value) *Value {
	if kind == SlotLocal {
		f := in.ds.curFrame
		f.setSlot(slot, val)
		return f.ptrs[slot]
	}
	return in.globals.setSlot(slot, val, in.GlobalEnv)
}

func (in *Interp) setVarSlotInt(kind SlotKind, slot int, n int64) *Value {
	if kind == SlotLocal {
		f := in.ds.curFrame
		f.setInt(slot, n)
		return &f.boxes[slot]
	}
	return in.globals.setInt(slot, n, in.GlobalEnv)
}

func (in *Interp) evalIntExprFast(expr Expr, env *Env) (int64, bool) {
	if expr == nil {
		return 0, false
	}
	switch e := expr.(type) {
	case *LiteralExpr:
		if e.Type == TokInt {
			return e.Value.(int64), true
		}
	case *VarExpr:
		if v, ok := in.getVar(e, env); ok && v != nil && v.Type == ValInt {
			return v.IntVal, true
		}
	case *BinaryExpr:
		if len(in.CustomInfixOps) > 0 {
			return 0, false
		}
		li, okL := in.evalIntExprFast(e.Left, env)
		if !okL {
			return 0, false
		}
		ri, okR := in.evalIntExprFast(e.Right, env)
		if !okR {
			return 0, false
		}
		switch e.Op {
		case "+":
			return li + ri, true
		case "-":
			return li - ri, true
		case "*", "×":
			return li * ri, true
		case "div":
			if ri == 0 {
				return 0, false
			}
			return li / ri, true
		case "%", "mod":
			if ri == 0 {
				return 0, false
			}
			return li % ri, true
		case "+&":
			return li & ri, true
		case "+|":
			return li | ri, true
		case "+^":
			return li ^ ri, true
		case "+<":
			return li << ri, true
		case "+>":
			return li >> ri, true
		}
	}
	return 0, false
}

func (in *Interp) evalCondFast(expr Expr, env *Env) (bool, error) {
	if expr == nil {
		return false, nil
	}
	switch e := expr.(type) {
	case *LiteralExpr:
		switch e.Type {
		case TokInt:
			return e.Value.(int64) != 0, nil
		case TokFloat:
			return e.Value.(float64) != 0.0, nil
		case TokString:
			s := e.Value.(string)
			return s != "" && s != "0", nil
		case TokIdent:
			id := e.Value.(string)
			if id == "True" || id == "true" {
				return true, nil
			}
			if id == "False" || id == "false" || id == "Nil" {
				return false, nil
			}
		}
	case *VarExpr:
		if e.Name == "True" || e.Name == "true" {
			return true, nil
		}
		if e.Name == "False" || e.Name == "false" || e.Name == "Nil" {
			return false, nil
		}
		if v, ok := in.getVar(e, env); ok && v != nil {
			return v.IsTrue(), nil
		}
		return false, nil
	case *UnaryExpr:
		if e.Op == "!" || e.Op == "not" {
			c, err := in.evalCondFast(e.Right, env)
			return !c, err
		}
		if e.Op == "?" || e.Op == "so" {
			return in.evalCondFast(e.Right, env)
		}
	case *BinaryExpr:
		if len(in.CustomInfixOps) == 0 {
			switch e.Op {
			case "&&", "and":
				lc, err := in.evalCondFast(e.Left, env)
				if err != nil || !lc {
					return false, err
				}
				return in.evalCondFast(e.Right, env)
			case "||", "or":
				lc, err := in.evalCondFast(e.Left, env)
				if err != nil || lc {
					return lc, err
				}
				return in.evalCondFast(e.Right, env)
			case "<=", ">=", "<", ">", "==":
				if li, okL := in.evalIntExprFast(e.Left, env); okL {
					if ri, okR := in.evalIntExprFast(e.Right, env); okR {
						switch e.Op {
						case "<=":
							return li <= ri, nil
						case ">=":
							return li >= ri, nil
						case "<":
							return li < ri, nil
						case ">":
							return li > ri, nil
						case "==":
							return li == ri, nil
						}
					}
				}
			case "!=":
				if li, okL := in.evalIntExprFast(e.Left, env); okL {
					if ri, okR := in.evalIntExprFast(e.Right, env); okR {
						return li != ri, nil
					}
				}
			case "%%":
				if li, okL := in.evalIntExprFast(e.Left, env); okL {
					if ri, okR := in.evalIntExprFast(e.Right, env); okR && ri != 0 {
						return li%ri == 0, nil
					}
				}
			case "eq", "ne":
				var ls, rs string
				var hasL, hasR bool
				if lit, ok := e.Left.(*LiteralExpr); ok && lit.Type == TokString {
					ls, hasL = lit.Value.(string), true
				} else if ve, ok := e.Left.(*VarExpr); ok {
					if v, ok := in.getVar(ve, env); ok && v != nil && v.Type == ValString {
						ls, hasL = v.StrVal, true
					}
				}
				if lit, ok := e.Right.(*LiteralExpr); ok && lit.Type == TokString {
					rs, hasR = lit.Value.(string), true
				} else if ve, ok := e.Right.(*VarExpr); ok {
					if v, ok := in.getVar(ve, env); ok && v != nil && v.Type == ValString {
						rs, hasR = v.StrVal, true
					}
				}
				if hasL && hasR {
					if e.Op == "eq" {
						return ls == rs, nil
					}
					return ls != rs, nil
				}
			}
		}
	}
	v, err := in.evalExpr(expr, env)
	if err != nil {
		return false, err
	}
	return v.IsTrue(), nil
}

func (in *Interp) execGotoSub(gSig *GotoSignal, env *Env) (*Value, error) {
	subName := strings.TrimPrefix(gSig.Target, "&")
	if callee, ok := env.Lookup(subName); ok {
		return in.InvokeCallable(callee, nil)
	}
	if callee, ok := in.GlobalEnv.Lookup(subName); ok {
		return in.InvokeCallable(callee, nil)
	}
	return nil, fmt.Errorf("goto: subroutine %q not found", subName)
}

func materializeHostReturn(v *Value, depth int) {
	if v == nil || depth > 2 {
		return
	}
	switch v.Type {
	case ValArray:
		if v.ArrayVal == nil && v.arrayLen() <= 1024 {
			v.materializeArray()
		}
		if len(v.ArrayVal) <= 1024 {
			for _, el := range v.ArrayVal {
				materializeHostReturn(el, depth+1)
			}
		}
	case ValHash:
		if len(v.HashVal) <= 1024 {
			for _, el := range v.HashVal {
				materializeHostReturn(el, depth+1)
			}
		}
	}
}

func copyTempSlot(dst, src *Value) *Value {
	if src == nil {
		return valNil
	}
	if (src.flags & flagTempArena) == 0 {
		return src
	}
	switch src.Type {
	case ValNil:
		return valNil
	case ValBool:
		return BoolValue(src.BoolVal)
	case ValInt:
		dst.Type = ValInt
		dst.IntVal = src.IntVal
		dst.flags = flagTempArena
		return dst
	case ValFloat:
		dst.Type = ValFloat
		dst.FloatVal = src.FloatVal
		dst.flags = flagTempArena
		return dst
	case ValString:
		s := src.StrVal
		if src.extKind == extStrBuf {
			s = strings.Clone(s)
		}
		dst.Type = ValString
		dst.StrVal = s
		dst.ext = nil
		dst.extKind = extNone
		dst.flags = flagTempArena
		return dst
	default:
		return escapeValue(src)
	}
}

func isBreak(err error) bool {
	if err == sigBreak {
		return true
	}
	_, ok := err.(*BreakSignal)
	return ok
}

func isContinue(err error) bool {
	if err == sigContinue {
		return true
	}
	_, ok := err.(*ContinueSignal)
	return ok
}

func isRedo(err error) bool {
	if err == sigRedo {
		return true
	}
	_, ok := err.(*RedoSignal)
	return ok
}

func (in *Interp) SetStdout(w io.Writer) {
	in.Stdout = w
}

// SetStderr sets the error stream for the interpreter.
func (in *Interp) SetStderr(w io.Writer) {
	in.Stderr = w
}

func (in *Interp) GetPackage(pkg string) map[string]*Value {
	if pkg == "" {
		pkg = "main"
	}
	if in.Packages == nil {
		in.Packages = make(map[string]map[string]*Value)
	}
	if stash, ok := in.Packages[pkg]; ok {
		return stash
	}
	stash := make(map[string]*Value)
	in.Packages[pkg] = stash
	return stash
}

func (in *Interp) SetPackageSymbol(pkg string, name string, val *Value) {
	val = escapeValue(val)
	stash := in.GetPackage(pkg)
	stash[name] = val
	noSigil := strings.TrimLeft(name, "$@%&")
	if noSigil != "" && noSigil != name {
		stash[noSigil] = val
	}
	if pkg == "main" || pkg == "" {
		in.GlobalEnv.Define(name, val)
		if noSigil != "" && noSigil != name {
			in.GlobalEnv.Define(noSigil, val)
		}
	}
	in.GlobalEnv.Define(pkg+"::"+name, val)
	if noSigil != "" && noSigil != name {
		in.GlobalEnv.Define(pkg+"::"+noSigil, val)
		sigil := name[:1]
		in.GlobalEnv.Define(sigil+pkg+"::"+noSigil, val)
	}
}

func (in *Interp) GetPackageSymbol(pkg string, name string) (*Value, bool) {
	if stash, ok := in.Packages[pkg]; ok {
		if v, exists := stash[name]; exists {
			return v, true
		}
		if v, exists := stash["$"+name]; exists {
			return v, true
		}
		if v, exists := stash["@"+name]; exists {
			return v, true
		}
		if v, exists := stash["%"+name]; exists {
			return v, true
		}
		if v, exists := stash["&"+name]; exists {
			return v, true
		}
	}
	if v, ok := in.GlobalEnv.Lookup(pkg + "::" + name); ok {
		return v, true
	}
	if v, ok := in.GlobalEnv.Lookup(pkg + "::$" + name); ok {
		return v, true
	}
	if v, ok := in.GlobalEnv.Lookup("$" + pkg + "::" + name); ok {
		return v, true
	}
	return nil, false
}

func (in *Interp) lookupAutoload(funcName string, env *Env) (*Value, string, bool) {
	// 1. If funcName contains '::', e.g. "Foo::bar"
	if strings.Contains(funcName, "::") {
		lastIdx := strings.LastIndex(funcName, "::")
		pkg := funcName[:lastIdx]
		if pkgStash, ok := in.Packages[pkg]; ok {
			if autoVal, ok := pkgStash["AUTOLOAD"]; ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
				return autoVal, funcName, true
			}
			if autoVal, ok := pkgStash["&AUTOLOAD"]; ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
				return autoVal, funcName, true
			}
		}
		if autoVal, ok := in.GlobalEnv.Lookup(pkg + "::AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
			return autoVal, funcName, true
		}
		if autoVal, ok := in.GlobalEnv.Lookup("&" + pkg + "::AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
			return autoVal, funcName, true
		}
	}

	// 2. Check current package
	pkg := in.CurrentPackage
	if pkg != "" && pkg != "main" {
		if pkgStash, ok := in.Packages[pkg]; ok {
			if autoVal, ok := pkgStash["AUTOLOAD"]; ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
				return autoVal, pkg + "::" + funcName, true
			}
			if autoVal, ok := pkgStash["&AUTOLOAD"]; ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
				return autoVal, pkg + "::" + funcName, true
			}
		}
		if autoVal, ok := in.GlobalEnv.Lookup(pkg + "::AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
			return autoVal, pkg + "::" + funcName, true
		}
		if autoVal, ok := in.GlobalEnv.Lookup("&" + pkg + "::AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
			return autoVal, pkg + "::" + funcName, true
		}
	}

	// 3. Check lexical env and global env
	if autoVal, ok := env.Lookup("AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
		return autoVal, "main::" + funcName, true
	}
	if autoVal, ok := env.Lookup("&AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
		return autoVal, "main::" + funcName, true
	}
	if autoVal, ok := in.GlobalEnv.Lookup("AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
		return autoVal, "main::" + funcName, true
	}
	if autoVal, ok := in.GlobalEnv.Lookup("&AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
		return autoVal, "main::" + funcName, true
	}
	if autoVal, ok := in.GlobalEnv.Lookup("main::AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
		return autoVal, "main::" + funcName, true
	}
	if autoVal, ok := in.GlobalEnv.Lookup("&main::AUTOLOAD"); ok && (autoVal.Type == ValClosure || autoVal.Type == ValMultiSub) {
		return autoVal, "main::" + funcName, true
	}

	return nil, "", false
}

// Eval parses and evaluates a Raku5 source code string.
func (in *Interp) Eval(source string) (*Value, error) {
	prog, err := ParseProgram(source)
	if err != nil {
		return nil, err
	}
	res, err := in.EvalProgram(prog, in.GlobalEnv)
	if err != nil {
		return nil, err
	}
	res = escapeValue(res)
	materializeHostReturn(res, 0)
	return res, nil
}

// EvalProgram evaluates all top-level statements.
func (in *Interp) EvalProgram(prog *Program, env *Env) (*Value, error) {
	if prog == nil || len(prog.Stmts) == 0 {
		return valNil, nil
	}
	in.ResolveProgram(prog, env == in.GlobalEnv)

	var labelMap map[string]int
	for idx, s := range prog.Stmts {
		if lStmt, ok := s.(*LabelStmt); ok {
			if labelMap == nil {
				labelMap = make(map[string]int)
			}
			labelMap[lStmt.Name] = idx
		}
	}

	var lastVal *Value = valNil
	i := 0
	lastIdx := len(prog.Stmts) - 1
	for i < len(prog.Stmts) {
		stmt := prog.Stmts[i]
		if labelMap == nil && i < lastIdx {
			mark := in.ds.markTemp()
			_, err := in.evalStmt(stmt, env)
			if err == nil {
				in.ds.rewindTemp(mark)
				i++
				continue
			}
			if err == sigReturn {
				rv := in.ds.returnVal
				in.ds.returnVal = nil
				return escapeValue(rv), nil
			}
			if ret, ok := err.(*ReturnSignal); ok {
				return escapeValue(ret.Value), nil
			}
			if gSig, ok := err.(*GotoSignal); ok {
				if gSig.IsSub {
					return in.execGotoSub(gSig, env)
				}
				return nil, fmt.Errorf("goto: label %q not found", gSig.Target)
			}
			return nil, err
		}
		val, err := in.evalStmt(stmt, env)
		if err != nil {
			if err == sigReturn {
				rv := in.ds.returnVal
				in.ds.returnVal = nil
				return escapeValue(rv), nil
			}
			if ret, ok := err.(*ReturnSignal); ok {
				return escapeValue(ret.Value), nil
			}
			if gSig, ok := err.(*GotoSignal); ok {
				if gSig.IsSub {
					return in.execGotoSub(gSig, env)
				}
				if targetIdx, ok := labelMap[gSig.Target]; ok {
					i = targetIdx
					continue
				}
				return nil, fmt.Errorf("goto: label %q not found", gSig.Target)
			}
			return nil, err
		}
		if val != nil {
			lastVal = val
		}
		i++
	}

	// Auto-dispatch to Raku 'sub MAIN' or 'multi sub MAIN' if declared
	if mainVal, ok := in.GlobalEnv.Lookup("MAIN"); ok && (mainVal.Type == ValClosure || mainVal.Type == ValMultiSub) {
		var mainArgs []*Value
		if argsVal, ok := in.GlobalEnv.Lookup("@*ARGS"); ok && argsVal.Type == ValArray {
			mainArgs = argsVal.materializeArray()
		} else if argsVal, ok := in.GlobalEnv.Lookup("@ARGV"); ok && argsVal.Type == ValArray {
			mainArgs = argsVal.materializeArray()
		}
		res, err := in.InvokeCallable(mainVal, mainArgs)
		if err != nil {
			if err == sigReturn {
				rv := in.ds.returnVal
				in.ds.returnVal = nil
				return escapeValue(rv), nil
			}
			if ret, ok := err.(*ReturnSignal); ok {
				return escapeValue(ret.Value), nil
			}
			return nil, err
		}
		if res != nil && res.Type != ValNil {
			lastVal = res
		}
	}

	return lastVal, nil
}

func (in *Interp) evalStmt(stmt Stmt, env *Env) (*Value, error) {
	switch s := stmt.(type) {
	case *SubsetDeclStmt:
		var closure *Closure
		if cExpr, ok := s.Where.(*ClosureExpr); ok {
			closure = &Closure{
				Name:   s.Name,
				Params: []Param{{Name: "$_"}},
				Body:   cExpr.Body,
				Env:    env,
			}
		} else {
			closure = &Closure{
				Name:   s.Name,
				Params: []Param{{Name: "$_"}},
				Body:   &BlockStmt{Stmts: []Stmt{&ReturnStmt{Value: s.Where}}},
				Env:    env,
			}
		}
		val := ClosureValue(closure)
		env.Define(s.Name, val)
		in.GlobalEnv.Define(s.Name, val)
		return NilValue(), nil

	case *PackageDeclStmt:
		if s.Body != nil {
			prevPkg := in.CurrentPackage
			in.CurrentPackage = s.Name
			_ = in.GetPackage(s.Name)
			childEnv := env.NewChild()
			val, err := in.evalBlock(s.Body, childEnv)
			in.CurrentPackage = prevPkg
			return val, err
		}
		in.CurrentPackage = s.Name
		_ = in.GetPackage(s.Name)
		return NilValue(), nil

	case *GrammarDeclStmt:
		if in.Grammars == nil {
			in.Grammars = make(map[string]*GrammarDeclStmt)
		}
		if in.CompiledGrammars == nil {
			in.CompiledGrammars = make(map[string]*gcre.Grammar)
		}
		in.Grammars[s.Name] = s
		rawSrc := s.Source
		if rawSrc == "" {
			var sb strings.Builder
			sb.WriteString("grammar ")
			sb.WriteString(s.Name)
			if s.Parent != "" {
				sb.WriteString(" is ")
				sb.WriteString(s.Parent)
			}
			sb.WriteString(" {\n")
			for _, r := range s.Rules {
				sb.WriteString("    ")
				sb.WriteString(r.Kind)
				sb.WriteString(" ")
				sb.WriteString(r.Name)
				sb.WriteString(" { ")
				sb.WriteString(r.Pattern)
				sb.WriteString(" }\n")
			}
			sb.WriteString("}\n")
			rawSrc = sb.String()
		}
		if cg, err := gcre.LoadGrammarWithParents(rawSrc, in.CompiledGrammars); err == nil && cg != nil {
			in.CompiledGrammars[s.Name] = cg
		}
		grammarHash := make(map[string]*Value)
		grammarHash["name"] = StringValue(s.Name)
		if s.Parent != "" {
			grammarHash["parent"] = StringValue(s.Parent)
		}
		for _, r := range s.Rules {
			grammarHash[r.Name] = StringValue(r.Pattern)
		}
		gVal := HashValue(grammarHash)
		env.Define(s.Name, gVal)
		in.GlobalEnv.Define(s.Name, gVal)
		return gVal, nil

	case *VarDeclStmt:
		if s.SlotKind != SlotNone {
			var initVal *Value = valNil
			if s.Value != nil {
				if n, ok := in.evalIntExprFast(s.Value, env); ok {
					return in.setVarSlotInt(s.SlotKind, s.Slot, n), nil
				}
				v, err := in.evalExpr(s.Value, env)
				if err != nil {
					return nil, err
				}
				initVal = v
			} else if strings.HasPrefix(s.Name, "@") {
				initVal = &Value{Type: ValArray}
			} else if strings.HasPrefix(s.Name, "%") {
				initVal = HashValue(make(map[string]*Value))
			}
			return in.setVarSlot(s.SlotKind, s.Slot, initVal), nil
		}

		if s.Scope == "state" {
			cell, ok := in.StateVars[s]
			if !ok {
				var initVal *Value = NilValue()
				if s.Value != nil {
					v, err := in.evalExpr(s.Value, env)
					if err != nil {
						return nil, err
					}
					initVal = v
				} else if strings.HasPrefix(s.Name, "@") {
					initVal = ArrayValue(nil)
				} else if strings.HasPrefix(s.Name, "%") {
					initVal = HashValue(make(map[string]*Value))
				}
				if in.StateVars == nil {
					in.StateVars = make(map[Stmt]*StateCell)
				}
				cell = &StateCell{Val: initVal}
				in.StateVars[s] = cell
			}
			env.DefineState(s.Name, cell)
			return cell.Val, nil
		}

		if s.Scope == "our" {
			pkg := in.CurrentPackage
			if pkg == "" {
				pkg = "main"
			}
			if s.Value == nil {
				if existing, ok := in.GetPackageSymbol(pkg, s.Name); ok {
					env.Define(s.Name, existing)
					return existing, nil
				}
			}
			var initVal *Value = NilValue()
			if s.Value != nil {
				v, err := in.evalExpr(s.Value, env)
				if err != nil {
					return nil, err
				}
				initVal = v
			} else if strings.HasPrefix(s.Name, "@") {
				initVal = ArrayValue(nil)
			} else if strings.HasPrefix(s.Name, "%") {
				initVal = HashValue(make(map[string]*Value))
			}
			in.SetPackageSymbol(pkg, s.Name, initVal)
			env.Define(s.Name, initVal)
			return initVal, nil
		}

		var initVal *Value = NilValue()
		if s.Value != nil {
			v, err := in.evalExpr(s.Value, env)
			if err != nil {
				return nil, err
			}
			initVal = v
		} else if strings.HasPrefix(s.Name, "@") {
			initVal = ArrayValue(nil)
		} else if strings.HasPrefix(s.Name, "%") {
			initVal = HashValue(make(map[string]*Value))
		}
		if s.Type != "" && initVal.Type != ValNil {
			if subVal, ok := env.Lookup(s.Type); ok && subVal.Type == ValClosure {
				r, err := in.InvokeCallable(subVal, []*Value{initVal})
				if err != nil || !r.IsTrue() {
					return nil, fmt.Errorf("type check failed: dynamic constraint violated for subset %s", s.Type)
				}
			} else if structDecl, ok := in.Structs[s.Type]; ok {
				if initVal.Type != ValCStruct || initVal.CStruct() == nil || initVal.CStruct().Class != structDecl {
					return nil, fmt.Errorf("type check failed in declaration of %s: expected struct %s, got %s", s.Name, s.Type, initVal.TypeName())
				}
			}
		}
		if s.Where != nil && initVal.Type != ValNil {
			whereEnv := env.NewChild()
			whereEnv.Define("$_", initVal)
			whereEnv.Define(s.Name, initVal)
			res, err := in.evalExpr(s.Where, whereEnv)
			if err != nil {
				return nil, fmt.Errorf("where constraint evaluation failed for %s: %w", s.Name, err)
			}
			if res != nil && res.Type == ValClosure {
				r, err := in.InvokeCallable(res, []*Value{initVal})
				if err != nil || !r.IsTrue() {
					return nil, fmt.Errorf("type check failed: where constraint violated for variable %s", s.Name)
				}
			} else if res != nil && !res.IsTrue() {
				return nil, fmt.Errorf("type check failed: where constraint violated for variable %s", s.Name)
			}
		}
		env.DefineTyped(s.Name, initVal, s.Type, s.Where)
		return initVal, nil

	case *AssignStmt:
		if v, ok, err := in.tryInPlaceAssign(s, env); ok {
			return v, err
		}
		val, err := in.evalExpr(s.Value, env)
		if err != nil {
			return nil, err
		}
		return in.evalAssign(s.Target, s.Op, val, env)

	case *SubDeclStmt:
		normName := normalizeOpName(s.Name)
		frameSlots := s.FrameSlots
		paramSlots := s.ParamSlots
		needsEnv := s.NeedsEnv
		fastSimple := s.FastSimple
		noParamChecks := s.NoParamChecks
		if s.Body != nil && !s.Body.Resolved {
			frameSlots, paramSlots, needsEnv, fastSimple, noParamChecks = in.resolveFunction(s.Params, s.Body, nil)
		}
		closure := &Closure{
			Name:          normName,
			Params:        s.Params,
			Body:          s.Body,
			Env:           env,
			IsMulti:       s.IsMulti,
			FrameSlots:    frameSlots,
			ParamSlots:    paramSlots,
			NeedsEnv:      needsEnv,
			FastSimple:    fastSimple,
			NoParamChecks: noParamChecks,
		}
		if s.IsMulti {
			env.RegisterMulti(normName, closure)
			env.RegisterMulti(s.Name, closure)
			in.GlobalEnv.RegisterMulti(normName, closure)
			in.GlobalEnv.RegisterMulti(s.Name, closure)
			if in.CurrentPackage != "" && in.CurrentPackage != "main" {
				in.GlobalEnv.RegisterMulti(in.CurrentPackage+"::"+normName, closure)
				in.GlobalEnv.RegisterMulti("&"+in.CurrentPackage+"::"+normName, closure)
			}
			if s.GlobalSlot >= 0 {
				if mVal, ok := in.GlobalEnv.Lookup(normName); ok {
					in.globals.setSlot(s.GlobalSlot, mVal, in.GlobalEnv)
				}
			}

			rawOp := extractRawOperatorName(s.Name)
			if strings.HasPrefix(s.Name, "infix:") || strings.HasPrefix(s.Name, "infix:<") {
				if mVal, ok := in.GlobalEnv.Lookup(normName); ok {
					in.CustomInfixOps[rawOp] = mVal
				}
			} else if strings.HasPrefix(s.Name, "prefix:") || strings.HasPrefix(s.Name, "prefix:<") {
				if mVal, ok := in.GlobalEnv.Lookup(normName); ok {
					in.CustomPrefixOps[rawOp] = mVal
				}
			}
		} else {
			val := ClosureValue(closure)
			env.Define(normName, val)
			env.Define("&"+normName, val)
			env.Define(s.Name, val)
			in.GlobalEnv.Define(normName, val)
			in.GlobalEnv.Define("&"+normName, val)
			in.GlobalEnv.Define(s.Name, val)
			if s.GlobalSlot >= 0 {
				in.globals.setSlot(s.GlobalSlot, val, in.GlobalEnv)
			}
			if in.CurrentPackage != "" && in.CurrentPackage != "main" {
				in.SetPackageSymbol(in.CurrentPackage, normName, val)
				in.SetPackageSymbol(in.CurrentPackage, "&"+normName, val)
				in.SetPackageSymbol(in.CurrentPackage, s.Name, val)
			}

			rawOp := extractRawOperatorName(s.Name)
			if strings.HasPrefix(s.Name, "infix:") || strings.HasPrefix(s.Name, "infix:<") {
				in.CustomInfixOps[rawOp] = val
			} else if strings.HasPrefix(s.Name, "prefix:") || strings.HasPrefix(s.Name, "prefix:<") {
				in.CustomPrefixOps[rawOp] = val
			}
		}
		return NilValue(), nil

	case *NativeSubDeclStmt:
		normName := normalizeOpName(s.Name)
		in.builtinEpoch++
		libKey := s.Library
		if libKey != "" && libKey != "native" {
			if _, ok := in.Builtins["ffi_load"]; ok {
				res, err := in.Builtins["ffi_load"](in, []*Value{StringValue(libKey)})
				if err == nil && res != nil {
					libKey = res.StrVal
				}
			}
		}

		in.Builtins[normName] = func(in *Interp, callArgs []*Value) (*Value, error) {
			copiedArgs := make([]*Value, len(callArgs))
			copy(copiedArgs, callArgs)
			fArgs := []*Value{
				StringValue(libKey),
				StringValue(s.Symbol),
				StringValue(s.ReturnType),
				ArrayValue(copiedArgs),
			}
			return in.Builtins["ffi_call"](in, fArgs)
		}
		closure := &Closure{Name: normName, Params: s.Params, Body: &BlockStmt{}, Env: env}
		env.Define(normName, ClosureValue(closure))
		env.Define("&"+normName, ClosureValue(closure))
		return NilValue(), nil

	case *CStructDeclStmt:
		in.Structs[s.Name] = s
		typeVal := CStructValue(&CStructInstance{Class: s, Ptr: 0})
		env.Define(s.Name, typeVal)
		in.GlobalEnv.Define(s.Name, typeVal)
		return NilValue(), nil

	case *AssertStmt:
		return in.evalVerify(&VerifyStmt{Kind: "ASSERT", Cond: s.Condition, Message: s.Message}, env)

	case *VerifyStmt:
		return in.evalVerify(s, env)

	case *AdviceHookStmt:
		in.hasAdviceHooks = true
		normTarget := normalizeOpName(s.TargetName)
		switch s.Kind {
		case "before":
			in.BeforeHooks[normTarget] = append(in.BeforeHooks[normTarget], s)
		case "after":
			in.AfterHooks[normTarget] = append(in.AfterHooks[normTarget], s)
		case "around":
			in.AroundHooks[normTarget] = append(in.AroundHooks[normTarget], s)
		}
		return NilValue(), nil

	case *IfStmt:
		condTrue, err := in.evalCondFast(s.Condition, env)
		if err != nil {
			return nil, err
		}
		if condTrue {
			branchEnv := env
			if s.ThenBranch != nil && s.ThenBranch.NeedsBlockEnv {
				branchEnv = env.NewChild()
			}
			return in.evalBlock(s.ThenBranch, branchEnv)
		}
		for i, eCond := range s.ElsifConds {
			ecTrue, err := in.evalCondFast(eCond, env)
			if err != nil {
				return nil, err
			}
			if ecTrue {
				branchEnv := env
				if i < len(s.ElsifThen) && s.ElsifThen[i] != nil && s.ElsifThen[i].NeedsBlockEnv {
					branchEnv = env.NewChild()
				}
				if i < len(s.ElsifThen) {
					return in.evalBlock(s.ElsifThen[i], branchEnv)
				}
				return valNil, nil
			}
		}
		if s.ElseBranch != nil {
			branchEnv := env
			if s.ElseBranch.NeedsBlockEnv {
				branchEnv = env.NewChild()
			}
			return in.evalBlock(s.ElseBranch, branchEnv)
		}
		return valNil, nil

	case *UnlessStmt:
		condTrue, err := in.evalCondFast(s.Condition, env)
		if err != nil {
			return nil, err
		}
		if !condTrue {
			branchEnv := env
			if s.Body != nil && s.Body.NeedsBlockEnv {
				branchEnv = env.NewChild()
			}
			return in.evalBlock(s.Body, branchEnv)
		}
		return valNil, nil

	case *WhileStmt:
		var lastVal *Value = valNil
		lastBox := in.ds.allocTemp()
		iterMark := in.ds.markTemp()
		childEnv := env
		if s.NeedsLoopEnv {
			childEnv = env.NewChild()
		}
		redo := false
		for {
			if !redo {
				condTrue, err := in.evalCondFast(s.Condition, env)
				if err != nil {
					return nil, err
				}
				if s.IsUntil {
					condTrue = !condTrue
				}
				if !condTrue {
					break
				}
			}
			redo = false
			bVal, err := in.evalBlock(s.Body, childEnv)
			if err != nil {
				if err == sigBreak || isBreak(err) {
					break
				}
				if err == sigContinue || isContinue(err) {
					in.ds.rewindTemp(iterMark)
					continue
				}
				if err == sigRedo || isRedo(err) {
					redo = true
					in.ds.rewindTemp(iterMark)
					continue
				}
				return nil, err
			}
			if bVal != nil {
				lastVal = copyTempSlot(lastBox, bVal)
			}
			in.ds.rewindTemp(iterMark)
		}
		return lastVal, nil

	case *ForStmt:
		// Fast-path A: integer range 1..N
		if be, ok := s.Iterable.(*BinaryExpr); ok && be.Op == ".." && len(in.CustomInfixOps) == 0 {
			startI, okStart := in.evalIntExprFast(be.Left, env)
			endI, okEnd := in.evalIntExprFast(be.Right, env)
			if okStart && okEnd {
				if startI > endI {
					return valNil, nil
				}
				var lastVal *Value = valNil
				lastBox := in.ds.allocTemp()
				iterMark := in.ds.markTemp()

				if !s.NeedsLoopEnv && s.VarSlotKind != SlotNone && !s.UsesTopic {
					for n := startI; n <= endI; n++ {
						in.setVarSlotInt(s.VarSlotKind, s.VarSlot, n)
						bVal, err := in.evalBlock(s.Body, env)
						if err != nil {
							if err == sigBreak || isBreak(err) {
								break
							}
							if err == sigContinue || isContinue(err) {
								in.ds.rewindTemp(iterMark)
								continue
							}
							if err == sigRedo || isRedo(err) {
								n--
								in.ds.rewindTemp(iterMark)
								continue
							}
							return nil, err
						}
						if bVal != nil {
							lastVal = copyTempSlot(lastBox, bVal)
						}
						in.ds.rewindTemp(iterMark)
					}
					return lastVal, nil
				}

				loopEnv := env
				if s.NeedsLoopEnv {
					loopEnv = env.NewChild()
				}
				for n := startI; n <= endI; n++ {
					redo := false
					for {
						if !redo {
							if s.VarSlotKind != SlotNone {
								in.setVarSlotInt(s.VarSlotKind, s.VarSlot, n)
							} else if s.VarName != "" {
								loopEnv.Define(s.VarName, IntValue(n))
							}
							if s.UsesTopic {
								loopEnv.Define("$_", IntValue(n))
							}
						}
						redo = false
						bVal, err := in.evalBlock(s.Body, loopEnv)
						if err != nil {
							if err == sigBreak || isBreak(err) {
								return lastVal, nil
							}
							if err == sigContinue || isContinue(err) {
								break
							}
							if err == sigRedo || isRedo(err) {
								redo = true
								in.ds.rewindTemp(iterMark)
								continue
							}
							return nil, err
						}
						if bVal != nil {
							lastVal = copyTempSlot(lastBox, bVal)
						}
						break
					}
					in.ds.rewindTemp(iterMark)
				}
				return lastVal, nil
			}
		}

		iterVal, err := in.evalExpr(s.Iterable, env)
		if err != nil {
			return nil, err
		}

		var lastVal *Value = valNil
		lastBox := in.ds.allocTemp()
		iterMark := in.ds.markTemp()
		loopEnv := env
		if s.NeedsLoopEnv {
			loopEnv = env.NewChild()
		}

		if ints := iterVal.intLane(); ints != nil {
			if !s.NeedsLoopEnv && s.VarSlotKind != SlotNone && !s.UsesTopic {
				for _, n := range ints {
					in.setVarSlotInt(s.VarSlotKind, s.VarSlot, n)
					bVal, err := in.evalBlock(s.Body, env)
					if err != nil {
						if err == sigBreak || isBreak(err) {
							break
						}
						if err == sigContinue || isContinue(err) {
							in.ds.rewindTemp(iterMark)
							continue
						}
						return nil, err
					}
					if bVal != nil {
						lastVal = copyTempSlot(lastBox, bVal)
					}
					in.ds.rewindTemp(iterMark)
				}
				return lastVal, nil
			}
			for _, n := range ints {
				item := IntValue(n)
				if s.VarSlotKind != SlotNone {
					in.setVarSlotInt(s.VarSlotKind, s.VarSlot, n)
				} else if s.VarName != "" {
					loopEnv.Define(s.VarName, item)
				}
				if s.UsesTopic {
					loopEnv.Define("$_", item)
				}
				bVal, err := in.evalBlock(s.Body, loopEnv)
				if err != nil {
					if err == sigBreak || isBreak(err) {
						break
					}
					if err == sigContinue || isContinue(err) {
						in.ds.rewindTemp(iterMark)
						continue
					}
					return nil, err
				}
				if bVal != nil {
					lastVal = copyTempSlot(lastBox, bVal)
				}
				in.ds.rewindTemp(iterMark)
			}
			return lastVal, nil
		}

		var list []*Value
		if iterVal.Type == ValArray {
			list = iterVal.materializeArray()
		} else if iterVal.Type == ValLazySeq && iterVal.LazySeq() != nil {
			list = iterVal.LazySeq().Items
		} else {
			list = []*Value{iterVal}
		}
		for _, item := range list {
			redo := false
			for {
				if !redo {
					if s.VarSlotKind != SlotNone {
						in.setVarSlot(s.VarSlotKind, s.VarSlot, item)
					} else if s.VarName != "" {
						loopEnv.Define(s.VarName, item)
					}
					if s.UsesTopic {
						loopEnv.Define("$_", item)
					}
				}
				redo = false
				bVal, err := in.evalBlock(s.Body, loopEnv)
				if err != nil {
					if err == sigBreak || isBreak(err) {
						return lastVal, nil
					}
					if err == sigContinue || isContinue(err) {
						break
					}
					if err == sigRedo || isRedo(err) {
						redo = true
						in.ds.rewindTemp(iterMark)
						continue
					}
					return nil, err
				}
				if bVal != nil {
					lastVal = copyTempSlot(lastBox, bVal)
				}
				break
			}
			in.ds.rewindTemp(iterMark)
		}
		return lastVal, nil

	case *LoopStmt:
		loopEnv := env
		if s.NeedsLoopEnv {
			loopEnv = env.NewChild()
		}
		if s.Init != nil {
			if _, err := in.evalExpr(s.Init, loopEnv); err != nil {
				return nil, err
			}
		}
		var lastVal *Value = valNil
		lastBox := in.ds.allocTemp()
		iterMark := in.ds.markTemp()
		bodyEnv := loopEnv
		if s.NeedsBodyEnv {
			bodyEnv = loopEnv.NewChild()
		}
		redo := false
		for {
			if !redo {
				if s.Cond != nil {
					condTrue, err := in.evalCondFast(s.Cond, loopEnv)
					if err != nil {
						return nil, err
					}
					if !condTrue {
						break
					}
				}
			}
			redo = false
			bVal, err := in.evalBlock(s.Body, bodyEnv)
			if err != nil {
				if err == sigBreak || isBreak(err) {
					break
				}
				if err == sigContinue || isContinue(err) {
					// step still runs
				} else if err == sigRedo || isRedo(err) {
					redo = true
					in.ds.rewindTemp(iterMark)
					continue
				} else {
					return nil, err
				}
			} else if bVal != nil {
				lastVal = copyTempSlot(lastBox, bVal)
			}
			in.ds.rewindTemp(iterMark)
			if s.Step != nil {
				if _, err := in.evalExpr(s.Step, loopEnv); err != nil {
					return nil, err
				}
			}
		}
		return lastVal, nil

	case *ReturnStmt:
		var retVal *Value = valNil
		if s.Value != nil {
			v, err := in.evalExpr(s.Value, env)
			if err != nil {
				return nil, err
			}
			retVal = v
		}
		in.ds.returnVal = retVal
		return nil, sigReturn

	case *BreakStmt:
		return nil, sigBreak

	case *RedoStmt:
		return nil, sigRedo

	case *ContinueStmt:
		return nil, sigContinue

	case *BlockStmt:
		blockEnv := env
		if s.NeedsBlockEnv {
			blockEnv = env.NewChild()
		}
		return in.evalBlock(s, blockEnv)

	case *UseStmt:
		return in.evalUse(s, env)

	case *ExprStmt:
		return in.evalExpr(s.Expr, env)

	case *GivenStmt:
		topicVal, err := in.evalExpr(s.Topic, env)
		if err != nil {
			return nil, err
		}
		givenEnv := env.NewChild()
		givenEnv.Define("$_", topicVal)
		for _, w := range s.Whens {
			matchVal, err := in.evalExpr(w.Match, givenEnv)
			if err != nil {
				return nil, err
			}
			if in.smartMatch(topicVal, matchVal) {
				return in.evalBlock(w.Body, givenEnv.NewChild())
			}
		}
		if s.Default != nil {
			return in.evalBlock(s.Default, givenEnv.NewChild())
		}
		return NilValue(), nil

	case *EnumDeclStmt:
		for _, v := range s.Values {
			env.Define(v.Name, IntValue(v.Index))
			if s.Name != "" {
				env.Define(s.Name+"::"+v.Name, IntValue(v.Index))
			}
		}
		return NilValue(), nil

	case *LabelStmt:
		if s.Stmt != nil {
			return in.evalStmt(s.Stmt, env)
		}
		return NilValue(), nil

	case *GotoStmt:
		return nil, &GotoSignal{Target: s.Target, IsSub: s.IsSub}

	case *ModifierStmt:
		switch s.Kind {
		case ModIf:
			cVal, err := in.evalExpr(s.Condition, env)
			if err != nil {
				return nil, err
			}
			if cVal.IsTrue() {
				return in.evalStmt(s.Target, env)
			}
			return NilValue(), nil

		case ModUnless:
			cVal, err := in.evalExpr(s.Condition, env)
			if err != nil {
				return nil, err
			}
			if !cVal.IsTrue() {
				return in.evalStmt(s.Target, env)
			}
			return NilValue(), nil

		case ModWhile:
			var lastVal *Value = NilValue()
			redo := false
			for {
				if !redo {
					cVal, err := in.evalExpr(s.Condition, env)
					if err != nil {
						return nil, err
					}
					if !cVal.IsTrue() {
						break
					}
				}
				redo = false
				val, err := in.evalStmt(s.Target, env)
				if err != nil {
					if _, ok := err.(*BreakSignal); ok {
						break
					}
					if _, ok := err.(*ContinueSignal); ok {
						continue
					}
					if _, ok := err.(*RedoSignal); ok {
						redo = true
						continue
					}
					return nil, err
				}
				if val != nil {
					lastVal = val
				}
			}
			return lastVal, nil

		case ModUntil:
			var lastVal *Value = NilValue()
			redo := false
			for {
				if !redo {
					cVal, err := in.evalExpr(s.Condition, env)
					if err != nil {
						return nil, err
					}
					if cVal.IsTrue() {
						break
					}
				}
				redo = false
				val, err := in.evalStmt(s.Target, env)
				if err != nil {
					if _, ok := err.(*BreakSignal); ok {
						break
					}
					if _, ok := err.(*ContinueSignal); ok {
						continue
					}
					if _, ok := err.(*RedoSignal); ok {
						redo = true
						continue
					}
					return nil, err
				}
				if val != nil {
					lastVal = val
				}
			}
			return lastVal, nil

		case ModFor:
			iterVal, err := in.evalExpr(s.Condition, env)
			if err != nil {
				return nil, err
			}
			var items []*Value
			if iterVal.Type == ValArray {
				items = iterVal.materializeArray()
			} else if iterVal.Type == ValLazySeq && iterVal.LazySeq() != nil {
				items = iterVal.LazySeq().Items
			} else if iterVal.Type == ValRef && iterVal.Ref() != nil && iterVal.Ref().Type == ValArray {
				items = iterVal.Ref().materializeArray()
			} else {
				items = []*Value{iterVal}
			}
			varName := s.VarName
			if varName == "" {
				varName = "$_"
			}
			var lastVal *Value = NilValue()
			var loopEnv *Env
			for _, it := range items {
				for {
					loopEnv = env.RecycleChild(loopEnv)
					loopEnv.Define(varName, it)
					loopEnv.Define("$_", it)
					val, err := in.evalStmt(s.Target, loopEnv)
					if err != nil {
						if _, ok := err.(*BreakSignal); ok {
							return lastVal, nil
						}
						if _, ok := err.(*ContinueSignal); ok {
							break
						}
						if _, ok := err.(*RedoSignal); ok {
							continue
						}
						return nil, err
					}
					if val != nil {
						lastVal = val
					}
					break
				}
			}
			return lastVal, nil

		case ModGiven:
			topicVal, err := in.evalExpr(s.Condition, env)
			if err != nil {
				return nil, err
			}
			givenEnv := env.NewChild()
			givenEnv.Define("$_", topicVal)
			return in.evalStmt(s.Target, givenEnv)

		default:
			return in.evalStmt(s.Target, env)
		}

	case *TakeStmt:
		var val *Value = NilValue()
		if s.Value != nil {
			v, err := in.evalExpr(s.Value, env)
			if err != nil {
				return nil, err
			}
			val = escapeValue(v)
		}
		if len(in.GatherStack) > 0 {
			topIdx := len(in.GatherStack) - 1
			in.GatherStack[topIdx] = append(in.GatherStack[topIdx], val)
		}
		return val, nil

	default:
		return NilValue(), nil
	}
}

func (in *Interp) evalBlock(b *BlockStmt, env *Env) (*Value, error) {
	if b == nil || len(b.Stmts) == 0 {
		return valNil, nil
	}
	prev := in.CurrentEnv
	in.CurrentEnv = env
	if b.Resolved && !b.HasLabels {
		var lastVal *Value = valNil
		lastIdx := len(b.Stmts) - 1
		for i, stmt := range b.Stmts {
			if i < lastIdx {
				mark := in.ds.markTemp()
				_, err := in.evalStmt(stmt, env)
				if err == nil {
					in.ds.rewindTemp(mark)
					continue
				}
				in.CurrentEnv = prev
				if gSig, ok := err.(*GotoSignal); ok && gSig.IsSub {
					return in.execGotoSub(gSig, env)
				}
				return nil, err
			}
			val, err := in.evalStmt(stmt, env)
			if err != nil {
				in.CurrentEnv = prev
				if gSig, ok := err.(*GotoSignal); ok && gSig.IsSub {
					return in.execGotoSub(gSig, env)
				}
				return nil, err
			}
			if val != nil {
				lastVal = val
			}
		}
		in.CurrentEnv = prev
		return lastVal, nil
	}

	var labelMap map[string]int
	for idx, s := range b.Stmts {
		if lStmt, ok := s.(*LabelStmt); ok {
			if labelMap == nil {
				labelMap = make(map[string]int)
			}
			labelMap[lStmt.Name] = idx
		}
	}

	var lastVal *Value = valNil
	i := 0
	for i < len(b.Stmts) {
		stmt := b.Stmts[i]
		val, err := in.evalStmt(stmt, env)
		if err != nil {
			in.CurrentEnv = prev
			if gSig, ok := err.(*GotoSignal); ok {
				if gSig.IsSub {
					return in.execGotoSub(gSig, env)
				}
				if targetIdx, ok := labelMap[gSig.Target]; ok {
					in.CurrentEnv = env
					i = targetIdx
					continue
				}
				return nil, gSig
			}
			return nil, err
		}
		if val != nil {
			lastVal = val
		}
		i++
	}
	in.CurrentEnv = prev
	return lastVal, nil
}

func (in *Interp) evalVerify(s *VerifyStmt, env *Env) (*Value, error) {
	kind := s.Kind
	if kind == "" {
		kind = "ASSERT"
	}
	nameVal := StringValue("")
	if s.Name != nil {
		nv, err := in.evalExpr(s.Name, env)
		if err != nil {
			return nil, err
		}
		nameVal = nv
	}

	switch kind {
	case "TEST":
		if os.Getenv("RAPTOR_TEST_MODE") != "1" {
			return NilValue(), nil
		}
		if s.Body == nil {
			return nil, fmt.Errorf("TEST requires a block")
		}
		cl := ClosureValue(&Closure{Name: nameVal.String(), Body: s.Body, Env: env})
		if fn, ok := in.Builtins["subtest"]; ok {
			return fn(in, []*Value{nameVal, cl})
		}
		return in.evalBlock(s.Body, env.NewChild())

	case "SUBTEST":
		if s.Body == nil {
			return nil, fmt.Errorf("SUBTEST requires a block")
		}
		cl := ClosureValue(&Closure{Name: nameVal.String(), Body: s.Body, Env: env})
		if fn, ok := in.Builtins["subtest"]; ok {
			return fn(in, []*Value{nameVal, cl})
		}
		return in.evalBlock(s.Body, env.NewChild())

	case "PROPERTY":
		if s.Body == nil {
			return nil, fmt.Errorf("PROPERTY requires a block")
		}
		cl := ClosureValue(&Closure{Name: nameVal.String(), Params: s.Params, Body: s.Body, Env: env})
		if fn, ok := in.Builtins["PROPERTY"]; ok {
			return fn(in, []*Value{nameVal, cl})
		}
		return nil, fmt.Errorf("PROPERTY builtin not registered")
	}

	var condVal *Value
	var err error
	if s.Body != nil {
		condVal, err = in.evalBlock(s.Body, env.NewChild())
	} else if s.Cond != nil {
		condVal, err = in.evalExpr(s.Cond, env)
	} else {
		condVal = BoolValue(false)
	}
	if err != nil {
		return nil, err
	}
	if condVal != nil && condVal.Type == ValClosure {
		condVal, err = in.InvokeCallable(condVal, nil)
		if err != nil {
			return nil, err
		}
	}
	if condVal == nil || !condVal.IsTrue() {
		msg := "failed"
		switch kind {
		case "PRE":
			msg = "precondition failed"
		case "POST":
			msg = "postcondition failed"
		case "INVARIANT":
			msg = "invariant violation"
		case "CHECK", "ASSERT":
			msg = "assertion failed"
		}
		if s.Message != nil {
			if mVal, merr := in.evalExpr(s.Message, env); merr == nil && mVal != nil {
				msg = mVal.String()
			}
		}
		tag := "AssertionError"
		switch kind {
		case "PRE":
			tag = "PreconditionError"
		case "POST":
			tag = "PostconditionError"
		case "INVARIANT":
			tag = "InvariantError"
		}
		return nil, fmt.Errorf("%s: %s", tag, msg)
	}
	return BoolValue(true), nil
}

func (in *Interp) validateTypeAndWhere(name string, val *Value, env *Env) error {
	if val == nil || val.Type == ValNil {
		return nil
	}
	typeName, whereExpr, ok := env.LookupType(name)
	if !ok {
		return nil
	}
	if typeName != "" {
		if subVal, ok := in.GlobalEnv.Lookup(typeName); ok && subVal.Type == ValClosure {
			r, err := in.InvokeCallable(subVal, []*Value{val})
			if err != nil || !r.IsTrue() {
				return fmt.Errorf("type check failed: dynamic constraint violated for subset %s on variable %s", typeName, name)
			}
		} else if structDecl, ok := in.Structs[typeName]; ok {
			if val.Type != ValCStruct || val.CStruct() == nil || val.CStruct().Class != structDecl {
				return fmt.Errorf("type check failed on %s: expected struct %s, got %s", name, typeName, val.TypeName())
			}
		} else if !val.MatchesType(typeName) {
			return fmt.Errorf("type check failed on %s: expected %s, got %s", name, typeName, val.TypeName())
		}
	}
	if whereExpr != nil {
		whereEnv := env.NewChild()
		whereEnv.Define("$_", val)
		whereEnv.Define(name, val)
		res, err := in.evalExpr(whereExpr, whereEnv)
		if err != nil {
			return fmt.Errorf("where constraint evaluation failed for %s: %w", name, err)
		}
		if res != nil && res.Type == ValClosure {
			r, err := in.InvokeCallable(res, []*Value{val})
			if err != nil || !r.IsTrue() {
				return fmt.Errorf("type check failed: where constraint violated for variable %s", name)
			}
		} else if res != nil && !res.IsTrue() {
			return fmt.Errorf("type check failed: where constraint violated for variable %s", name)
		}
	}
	return nil
}

func (in *Interp) tryInPlaceAssign(s *AssignStmt, env *Env) (*Value, bool, error) {
	ve, ok := s.Target.(*VarExpr)
	if !ok {
		return nil, false, nil
	}

	// Slot-based fast path (SlotLocal or SlotGlobal)
	if ve.SlotKind != SlotNone {
		if s.Op == "+=" || s.Op == "-=" {
			delta, okR := in.evalIntExprFast(s.Value, env)
			if !okR {
				rhs, err := in.evalExpr(s.Value, env)
				if err != nil {
					return nil, true, err
				}
				if rhs.Type == ValInt {
					delta = rhs.IntVal
					okR = true
				}
			}
			if okR {
				if s.Op == "-=" {
					delta = -delta
				}
				if ve.SlotKind == SlotLocal {
					f := in.ds.curFrame
					cur := f.ptrs[ve.Slot]
					if cur.Type == ValInt {
						f.setInt(ve.Slot, cur.IntVal+delta)
						return &f.boxes[ve.Slot], true, nil
					}
				} else {
					cur := in.globals.get(ve.Slot)
					if cur.Type == ValInt {
						return in.globals.setInt(ve.Slot, cur.IntVal+delta, in.GlobalEnv), true, nil
					}
				}
			}
			return nil, false, nil
		}

		if s.Op == "=" {
			if be, ok := s.Value.(*BinaryExpr); ok && (be.Op == "+" || be.Op == "-") {
				if lv, ok := be.Left.(*VarExpr); ok && lv.Name == ve.Name {
					delta, okR := in.evalIntExprFast(be.Right, env)
					if okR {
						if be.Op == "-" {
							delta = -delta
						}
						if ve.SlotKind == SlotLocal {
							f := in.ds.curFrame
							cur := f.ptrs[ve.Slot]
							if cur.Type == ValInt {
								f.setInt(ve.Slot, cur.IntVal+delta)
								return &f.boxes[ve.Slot], true, nil
							}
						} else {
							cur := in.globals.get(ve.Slot)
							if cur.Type == ValInt {
								return in.globals.setInt(ve.Slot, cur.IntVal+delta, in.GlobalEnv), true, nil
							}
						}
					}
				}
			}
			if valI, ok := in.evalIntExprFast(s.Value, env); ok {
				return in.setVarSlotInt(ve.SlotKind, ve.Slot, valI), true, nil
			}
		}

		if s.Op == "~=" || s.Op == "=" {
			var suffix string
			hasSuffix := false
			if s.Op == "~=" {
				rhs, err := in.evalExpr(s.Value, env)
				if err != nil {
					return nil, true, err
				}
				suffix = rhs.String()
				hasSuffix = true
			} else if be, ok := s.Value.(*BinaryExpr); ok && be.Op == "~" {
				if lv, ok := be.Left.(*VarExpr); ok && lv.Name == ve.Name {
					rhs, err := in.evalExpr(be.Right, env)
					if err != nil {
						return nil, true, err
					}
					suffix = rhs.String()
					hasSuffix = true
				}
			}
			if hasSuffix {
				var cur *Value
				if ve.SlotKind == SlotLocal {
					cur = in.ds.curFrame.ptrs[ve.Slot]
				} else {
					cur = in.globals.get(ve.Slot)
				}
				if cur != nil && cur.Type == ValString && !isFlyweight(cur) {
					cur.appendString(suffix)
					return in.setVarSlot(ve.SlotKind, ve.Slot, cur), true, nil
				}
				base := ""
				if cur != nil {
					base = cur.String()
				}
				nv := StringValue(base + suffix)
				return in.setVarSlot(ve.SlotKind, ve.Slot, nv), true, nil
			}
		}
	}

	if tName, wExpr, ok := env.LookupType(ve.Name); ok && (tName != "" || wExpr != nil) {
		return nil, false, nil
	}
	cur, has := env.Lookup(ve.Name)

	// $n += $k / $n += 1 — mutate a unique int box.
	if s.Op == "+=" || s.Op == "-=" {
		rhs, err := in.evalExpr(s.Value, env)
		if err != nil {
			return nil, true, err
		}
		if has && cur.Type == ValInt && rhs.Type == ValInt {
			delta := rhs.IntVal
			if s.Op == "-=" {
				delta = -delta
			}
			if isInternedInt(cur) {
				nv := IntValue(cur.IntVal + delta)
				_ = env.Assign(ve.Name, nv)
				return nv, true, nil
			}
			cur.IntVal += delta
			return cur, true, nil
		}
		return nil, false, nil
	}

	// $n = $n + k  /  $n = $n - k  (same as += / -=)
	if s.Op == "=" {
		if be, ok := s.Value.(*BinaryExpr); ok && (be.Op == "+" || be.Op == "-") {
			if lv, ok := be.Left.(*VarExpr); ok && lv.Name == ve.Name && has && cur.Type == ValInt {
				rhs, err := in.evalExpr(be.Right, env)
				if err != nil {
					return nil, true, err
				}
				if rhs.Type == ValInt {
					delta := rhs.IntVal
					if be.Op == "-" {
						delta = -delta
					}
					if isInternedInt(cur) {
						nv := detachInt(IntValue(cur.IntVal + delta))
						_ = env.Assign(ve.Name, nv)
						return nv, true, nil
					}
					cur.IntVal += delta
					return cur, true, nil
				}
			}
		}
	}

	// $s ~= "x"  or  $s = $s ~ "x"
	var suffix string
	switch s.Op {
	case "~=":
		rhs, err := in.evalExpr(s.Value, env)
		if err != nil {
			return nil, true, err
		}
		suffix = rhs.String()
	case "=":
		be, ok := s.Value.(*BinaryExpr)
		if !ok || be.Op != "~" {
			return nil, false, nil
		}
		lv, ok := be.Left.(*VarExpr)
		if !ok || lv.Name != ve.Name {
			return nil, false, nil
		}
		rhs, err := in.evalExpr(be.Right, env)
		if err != nil {
			return nil, true, err
		}
		suffix = rhs.String()
	default:
		return nil, false, nil
	}
	if !has || cur.Type != ValString || isFlyweight(cur) {
		base := ""
		if has {
			base = cur.String()
		}
		nv := StringValue(base + suffix)
		if err := env.Assign(ve.Name, nv); err != nil {
			env.Define(ve.Name, nv)
		}
		return nv, true, nil
	}
	return cur.appendString(suffix), true, nil
}

func (in *Interp) evalAssign(target Expr, op string, val *Value, env *Env) (*Value, error) {
	switch t := target.(type) {
	case *VarExpr:
		if t.SlotKind != SlotNone {
			if op == "=" {
				return in.setVarSlot(t.SlotKind, t.Slot, val), nil
			}
			if op == "//=" {
				cur, ok := in.getVar(t, env)
				if ok && cur != nil && cur.Type != ValNil {
					return cur, nil
				}
				return in.setVarSlot(t.SlotKind, t.Slot, val), nil
			}
			cur, ok := in.getVar(t, env)
			if !ok || cur == nil {
				cur = valNil
			}
			newVal, err := in.evalBinaryOp(cur, op[:len(op)-1], val)
			if err != nil {
				return nil, err
			}
			return in.setVarSlot(t.SlotKind, t.Slot, newVal), nil
		}

		if strings.HasPrefix(t.Name, "%") && strings.HasSuffix(t.Name, "::") {
			pkg := strings.TrimPrefix(t.Name, "%")
			pkg = strings.TrimPrefix(pkg, "::")
			pkg = strings.TrimSuffix(pkg, "::")
			if val.Type == ValHash && val.HashVal != nil {
				in.Packages[pkg] = val.HashVal
				return val, nil
			}
		}
		if strings.Contains(t.Name, "::") {
			sigil := t.Name[0:1]
			if sigil == "$" || sigil == "@" || sigil == "%" {
				rest := t.Name[1:]
				lastIdx := strings.LastIndex(rest, "::")
				if lastIdx != -1 {
					pkg := rest[:lastIdx]
					sym := sigil + rest[lastIdx+2:]
					in.SetPackageSymbol(pkg, sym, val)
					in.GlobalEnv.Define(t.Name, val)
					return val, nil
				}
			}
		}
		if op == "=" {
			if err := in.validateTypeAndWhere(t.Name, val, env); err != nil {
				return nil, err
			}
			if in.CurrentPackage != "" {
				if _, exists := in.GetPackageSymbol(in.CurrentPackage, t.Name); exists {
					in.SetPackageSymbol(in.CurrentPackage, t.Name, val)
				}
			}
			if err := env.Assign(t.Name, val); err != nil {
				env.Define(t.Name, val)
			}
			return val, nil
		}
		if op == "//=" {
			cur, ok := env.Lookup(t.Name)
			if ok && cur.Type != ValNil {
				return cur, nil
			}
			if err := in.validateTypeAndWhere(t.Name, val, env); err != nil {
				return nil, err
			}
			if err := env.Assign(t.Name, val); err != nil {
				env.Define(t.Name, val)
			}
			return val, nil
		}
		cur, ok := env.Lookup(t.Name)
		if !ok {
			cur = NilValue()
		}
		newVal, err := in.evalBinaryOp(cur, op[:len(op)-1], val)
		if err != nil {
			return nil, err
		}
		if err := in.validateTypeAndWhere(t.Name, newVal, env); err != nil {
			return nil, err
		}
		if err := env.Assign(t.Name, newVal); err != nil {
			env.Define(t.Name, newVal)
		}
		return newVal, nil

	case *IndexExpr:
		arrVal, err := in.evalExpr(t.Array, env)
		if err != nil {
			return nil, err
		}
		if arrVal.Type == ValNil {
			if ve, ok := t.Array.(*VarExpr); ok && strings.HasPrefix(ve.Name, "$") {
				if _, hasS := env.Lookup(ve.Name); !hasS {
					if av, hasA := env.Lookup("@" + ve.Name[1:]); hasA {
						arrVal = av
					}
				}
			}
		}
		if arrVal.Type != ValArray {
			return nil, fmt.Errorf("cannot index non-array type %s", arrVal.TypeName())
		}
		idxVal, err := in.evalExpr(t.Index, env)
		if err != nil {
			return nil, err
		}
		idx := int(in.toInt(idxVal))
		n := arrVal.arrayLen()
		if idx < 0 {
			idx = n + idx
		}
		if idx < 0 {
			return nil, fmt.Errorf("array index out of bounds: %d", idx)
		}
		if op != "=" {
			var cur *Value = NilValue()
			if idx < n {
				cur = arrVal.arrayAt(idx)
			}
			if op == "//=" {
				if cur.Type != ValNil {
					return cur, nil
				}
			} else {
				newVal, err := in.evalBinaryOp(cur, op[:len(op)-1], val)
				if err != nil {
					return nil, err
				}
				val = newVal
			}
		}
		if ints := arrVal.intLane(); ints != nil && val.Type == ValInt {
			for len(ints) <= idx {
				ints = append(ints, 0)
			}
			ints[idx] = val.IntVal
			arrVal.lane().Ints = ints
			return val, nil
		}
		if floats := arrVal.floatLane(); floats != nil && val.Type == ValFloat {
			for len(floats) <= idx {
				floats = append(floats, 0)
			}
			floats[idx] = val.FloatVal
			arrVal.lane().Floats = floats
			return val, nil
		}
		arrVal.materializeArray()
		for len(arrVal.ArrayVal) <= idx {
			arrVal.ArrayVal = append(arrVal.ArrayVal, NilValue())
		}
		arrVal.ArrayVal[idx] = escapeValue(val)
		return val, nil

	case *HashAccessExpr:
		hashVal, err := in.evalExpr(t.Hash, env)
		if err != nil {
			return nil, err
		}
		if hashVal.Type == ValNil {
			if ve, ok := t.Hash.(*VarExpr); ok && strings.HasPrefix(ve.Name, "$") {
				if _, hasS := env.Lookup(ve.Name); !hasS {
					if hv, hasH := env.Lookup("%" + ve.Name[1:]); hasH {
						hashVal = hv
					}
				}
			}
		}
		if hashVal.Type != ValHash {
			return nil, fmt.Errorf("cannot access non-hash type %s", hashVal.TypeName())
		}
		keyVal, err := in.evalExpr(t.Key, env)
		if err != nil {
			return nil, err
		}
		k := keyVal.String()
		if hashVal.HashVal == nil {
			hashVal.HashVal = make(map[string]*Value)
		}
		if op != "=" {
			cur, exists := hashVal.HashVal[k]
			if !exists {
				cur = NilValue()
			}
			if op == "//=" {
				if cur.Type != ValNil {
					return cur, nil
				}
			} else {
				newVal, err := in.evalBinaryOp(cur, op[:len(op)-1], val)
				if err != nil {
					return nil, err
				}
				val = newVal
			}
		}
		hashVal.HashVal[k] = escapeValue(val)
		return val, nil

	case *DerefExpr:
		refVal, err := in.evalExpr(t.Ref, env)
		if err != nil {
			return nil, err
		}
		switch t.Kind {
		case DerefScalar:
			if refVal.Type == ValRef && refVal.Ref() != nil {
				if op == "=" {
					*refVal.Ref() = *escapeValue(val)
					return val, nil
				}
				newVal, err := in.evalBinaryOp(refVal.Ref(), op[:len(op)-1], val)
				if err != nil {
					return nil, err
				}
				*refVal.Ref() = *escapeValue(newVal)
				return newVal, nil
			}
			return nil, fmt.Errorf("cannot assign to non-scalar reference")

		case DerefArrowArray:
			arrVal := refVal
			if arrVal.Type == ValRef && arrVal.Ref() != nil {
				arrVal = arrVal.Ref()
			}
			if arrVal.Type != ValArray {
				return nil, fmt.Errorf("cannot array-index non-array reference of type %s", arrVal.TypeName())
			}
			idxVal, err := in.evalExpr(t.Index, env)
			if err != nil {
				return nil, err
			}
			arrVal.materializeArray()
			idx := int(in.toInt(idxVal))
			if idx < 0 {
				idx = len(arrVal.ArrayVal) + idx
			}
			if idx < 0 {
				return nil, fmt.Errorf("array index out of bounds: %d", idx)
			}
			for len(arrVal.ArrayVal) <= idx {
				arrVal.ArrayVal = append(arrVal.ArrayVal, NilValue())
			}
			if op == "=" {
				arrVal.ArrayVal[idx] = escapeValue(val)
				return val, nil
			}
			cur := arrVal.ArrayVal[idx]
			newVal, err := in.evalBinaryOp(cur, op[:len(op)-1], val)
			if err != nil {
				return nil, err
			}
			arrVal.ArrayVal[idx] = escapeValue(newVal)
			return newVal, nil

		case DerefArrowHash:
			hashVal := refVal
			if hashVal.Type == ValRef && hashVal.Ref() != nil {
				hashVal = hashVal.Ref()
			}
			if hashVal.Type != ValHash {
				return nil, fmt.Errorf("cannot hash-index non-hash reference of type %s", hashVal.TypeName())
			}
			keyVal, err := in.evalExpr(t.Index, env)
			if err != nil {
				return nil, err
			}
			k := keyVal.String()
			if hashVal.HashVal == nil {
				hashVal.HashVal = make(map[string]*Value)
			}
			if op == "=" {
				hashVal.HashVal[k] = val
				return val, nil
			}
			cur, ok := hashVal.HashVal[k]
			if !ok {
				cur = NilValue()
			}
			newVal, err := in.evalBinaryOp(cur, op[:len(op)-1], val)
			if err != nil {
				return nil, err
			}
			hashVal.HashVal[k] = newVal
			return newVal, nil

		default:
			return nil, fmt.Errorf("unsupported dereference assignment")
		}

	case *MethodCallExpr:
		invocantVal, err := in.evalExpr(t.Target, env)
		if err != nil {
			return nil, err
		}
		if invocantVal.Type == ValCStruct && invocantVal.CStruct() != nil && invocantVal.CStruct().Class != nil {
			if idx, ok := invocantVal.CStruct().Class.FieldIndex[t.Method]; ok {
				return in.writeCStructField(invocantVal.CStruct(), invocantVal.CStruct().Class.Fields[idx], val)
			}
		}
		return nil, fmt.Errorf("cannot assign to method %q on type %s", t.Method, invocantVal.TypeName())

	default:
		return nil, fmt.Errorf("invalid assignment target")
	}

}

func (in *Interp) evalExpr(expr Expr, env *Env) (*Value, error) {
	if expr == nil {
		return NilValue(), nil
	}

	switch e := expr.(type) {
	case *LiteralExpr:
		switch e.Type {
		case TokInt:
			return IntValue(e.Value.(int64)), nil
		case TokFloat:
			return FloatValue(e.Value.(float64)), nil
		case TokString:
			return StringValue(e.Value.(string)), nil
		case TokIdent:
			identStr := e.Value.(string)
			if identStr == "Nil" {
				return NilValue(), nil
			}
			if identStr == "True" || identStr == "true" {
				return BoolValue(true), nil
			}
			if identStr == "False" || identStr == "false" {
				return BoolValue(false), nil
			}
			if val, ok := env.Lookup(identStr); ok {
				return val, nil
			}
			return StringValue(identStr), nil
		default:
			return NilValue(), nil
		}

	case *VarExpr:
		if e.SlotKind != SlotNone {
			if val, ok := in.getVar(e, env); ok && val != nil {
				return val, nil
			}
		}
		if e.Name == "Nil" {
			return NilValue(), nil
		}
		if e.Name == "True" || e.Name == "true" {
			return BoolValue(true), nil
		}
		if e.Name == "False" || e.Name == "false" {
			return BoolValue(false), nil
		}
		// Package stash reflection: %Foo:: or %::Foo::
		if strings.HasPrefix(e.Name, "%") && strings.HasSuffix(e.Name, "::") {
			raw := strings.TrimPrefix(e.Name, "%")
			raw = strings.TrimPrefix(raw, "::")
			raw = strings.TrimSuffix(raw, "::")
			stash := in.GetPackage(raw)
			return HashValue(stash), nil
		}
		if val, ok := env.Lookup(e.Name); ok {
			return val, nil
		}
		if val, ok := in.GlobalEnv.Lookup(e.Name); ok {
			return val, nil
		}
		if strings.Contains(e.Name, "::") {
			sigil := e.Name[0:1]
			if sigil == "$" || sigil == "@" || sigil == "%" || sigil == "&" {
				rest := e.Name[1:]
				lastIdx := strings.LastIndex(rest, "::")
				if lastIdx != -1 {
					pkg := rest[:lastIdx]
					sym := sigil + rest[lastIdx+2:]
					if val, ok := in.GetPackageSymbol(pkg, sym); ok {
						return val, nil
					}
					symNoSigil := rest[lastIdx+2:]
					if val, ok := in.GetPackageSymbol(pkg, symNoSigil); ok {
						return val, nil
					}
				}
			} else {
				lastIdx := strings.LastIndex(e.Name, "::")
				if lastIdx != -1 {
					pkg := e.Name[:lastIdx]
					sym := e.Name[lastIdx+2:]
					if val, ok := in.GetPackageSymbol(pkg, sym); ok {
						return val, nil
					}
				}
			}
		}
		if _, ok := in.Builtins[e.Name]; ok {
			return ClosureValue(&Closure{
				Name: e.Name,
				Body: &BlockStmt{},
				Env:  env,
			}), nil
		}
		if !strings.HasPrefix(e.Name, "$") && !strings.HasPrefix(e.Name, "@") && !strings.HasPrefix(e.Name, "%") && !strings.HasPrefix(e.Name, "&") {
			return StringValue(e.Name), nil
		}
		return NilValue(), nil

	case *TernaryExpr:
		condVal, err := in.evalExpr(e.Cond, env)
		if err != nil {
			return nil, err
		}
		if condVal.IsTrue() {
			return in.evalExpr(e.Then, env)
		}
		return in.evalExpr(e.Else, env)

	case *RefExpr:
		if vExpr, ok := e.Expr.(*VarExpr); ok {
			if val, ok := in.getVar(vExpr, env); ok {
				return RefValue(val), nil
			}
			newVal := NilValue()
			env.Define(vExpr.Name, newVal)
			return RefValue(newVal), nil
		}
		targetVal, err := in.evalExpr(e.Expr, env)
		if err != nil {
			return nil, err
		}
		return RefValue(targetVal), nil

	case *DerefExpr:
		refVal, err := in.evalExpr(e.Ref, env)
		if err != nil {
			return nil, err
		}
		switch e.Kind {
		case DerefScalar:
			if refVal.Type == ValRef {
				if refVal.Ref() != nil {
					return refVal.Ref(), nil
				}
				return NilValue(), nil
			}
			return refVal, nil

		case DerefArray:
			arrVal := refVal
			if arrVal.Type == ValRef && arrVal.Ref() != nil {
				arrVal = arrVal.Ref()
			}
			if arrVal.Type == ValArray {
				return arrVal, nil
			}
			if arrVal.Type == ValLazySeq && arrVal.LazySeq() != nil {
				return ArrayValue(arrVal.LazySeq().Items), nil
			}
			return ArrayValue([]*Value{arrVal}), nil

		case DerefHash:
			hashVal := refVal
			if hashVal.Type == ValRef && hashVal.Ref() != nil {
				hashVal = hashVal.Ref()
			}
			if hashVal.Type == ValHash {
				return hashVal, nil
			}
			return hashVal, nil

		case DerefCode:
			codeVal := refVal
			if codeVal.Type == ValRef && codeVal.Ref() != nil {
				codeVal = codeVal.Ref()
			}
			return codeVal, nil

		case DerefArrowArray:
			arrVal := refVal
			if arrVal.Type == ValRef && arrVal.Ref() != nil {
				arrVal = arrVal.Ref()
			}
			if arrVal.Type != ValArray {
				return nil, fmt.Errorf("cannot index non-array reference of type %s", arrVal.TypeName())
			}
			idxVal, err := in.evalExpr(e.Index, env)
			if err != nil {
				return nil, err
			}
			idx := int(in.toInt(idxVal))
			n := arrVal.arrayLen()
			if idx < 0 {
				idx = n + idx
			}
			if idx < 0 || idx >= n {
				return NilValue(), nil
			}
			return arrVal.arrayAt(idx), nil

		case DerefArrowHash:
			hashVal := refVal
			if hashVal.Type == ValRef && hashVal.Ref() != nil {
				hashVal = hashVal.Ref()
			}
			if hashVal.Type != ValHash {
				return nil, fmt.Errorf("cannot access non-hash reference of type %s", hashVal.TypeName())
			}
			keyVal, err := in.evalExpr(e.Index, env)
			if err != nil {
				return nil, err
			}
			k := keyVal.String()
			if hashVal.HashVal == nil {
				return NilValue(), nil
			}
			if val, ok := hashVal.HashVal[k]; ok {
				return val, nil
			}
			return NilValue(), nil

		case DerefArrowCode:
			codeVal := refVal
			if codeVal.Type == ValRef && codeVal.Ref() != nil {
				codeVal = codeVal.Ref()
			}
			base, callArgs := in.ds.pushArgs(len(e.Args))
			for i, a := range e.Args {
				aVal, err := in.evalExpr(a, env)
				if err != nil {
					in.ds.popArgs(base)
					return nil, err
				}
				callArgs[i] = aVal
			}
			res, err := in.InvokeCallable(codeVal, callArgs)
			in.ds.popArgs(base)
			return res, err

		default:
			return refVal, nil
		}

	case *BinaryExpr:
		if v, ok := in.evalBinaryFast(e, env); ok {
			return v, nil
		}
		if e.Op == "//" || e.Op == "orelse" {
			lVal, err := in.evalExpr(e.Left, env)
			if err != nil {
				return nil, err
			}
			if lVal.Type != ValNil {
				return lVal, nil
			}
			return in.evalExpr(e.Right, env)
		}
		if e.Op == "&&" || e.Op == "and" {
			lVal, err := in.evalExpr(e.Left, env)
			if err != nil {
				return nil, err
			}
			if !lVal.IsTrue() {
				return lVal, nil
			}
			return in.evalExpr(e.Right, env)
		}
		if e.Op == "||" || e.Op == "or" {
			lVal, err := in.evalExpr(e.Left, env)
			if err != nil {
				return nil, err
			}
			if lVal.IsTrue() {
				return lVal, nil
			}
			return in.evalExpr(e.Right, env)
		}

		lVal, err := in.evalExpr(e.Left, env)
		if err != nil {
			return nil, err
		}
		rVal, err := in.evalExpr(e.Right, env)
		if err != nil {
			return nil, err
		}
		return in.evalBinaryOp(lVal, e.Op, rVal)

	case *UnaryExpr:
		rVal, err := in.evalExpr(e.Right, env)
		if err != nil {
			return nil, err
		}
		if rVal.Type == ValJunction && rVal.Junction() != nil {
			// Boolean context coercions collapse a Junction to a single Bool.
			if e.Op == "?" || e.Op == "so" {
				return BoolValue(rVal.IsTrue()), nil
			}
			if e.Op == "!" || e.Op == "not" {
				return BoolValue(!rVal.IsTrue()), nil
			}
			var results []*Value
			for _, elem := range rVal.Junction().Values {
				res, err := in.evalUnaryOp(e.Op, elem, env)
				if err != nil {
					return nil, err
				}
				results = append(results, res)
			}
			return JunctionValue(rVal.Junction().Kind, results), nil
		}
		return in.evalUnaryOp(e.Op, rVal, env)

	case *ArrayLiteralExpr:
		n := len(e.Elements)
		if n == 0 {
			return &Value{Type: ValArray}, nil
		}
		base, tmp := in.ds.pushArgs(n)
		allInts := true
		allFloats := true
		allStrs := true
		totalStrBytes := 0
		for i, elExpr := range e.Elements {
			v, err := in.evalExpr(elExpr, env)
			if err != nil {
				in.ds.popArgs(base)
				return nil, err
			}
			tmp[i] = v
			if v.Type != ValInt {
				allInts = false
			}
			if v.Type != ValFloat {
				allFloats = false
			}
			if v.Type != ValString {
				allStrs = false
			} else {
				totalStrBytes += len(v.StrVal)
			}
		}
		if allInts {
			ints := make([]int64, n)
			for i, v := range tmp {
				ints[i] = v.IntVal
			}
			in.ds.popArgs(base)
			return IntArrayValue(ints), nil
		}
		if allFloats {
			floats := make([]float64, n)
			for i, v := range tmp {
				floats[i] = v.FloatVal
			}
			in.ds.popArgs(base)
			return FloatArrayValue(floats), nil
		}
		if allStrs {
			tape := NewStrTape(n, totalStrBytes)
			for _, v := range tmp {
				tape.Append(v.StrVal)
			}
			in.ds.popArgs(base)
			return StrTapeArrayValue(tape), nil
		}
		elems := make([]*Value, n)
		for i, v := range tmp {
			elems[i] = escapeValue(v)
		}
		in.ds.popArgs(base)
		return &Value{Type: ValArray, ArrayVal: elems}, nil

	case *HashLiteralExpr:
		m := make(map[string]*Value)
		for _, pair := range e.Pairs {
			kVal, err := in.evalExpr(pair[0], env)
			if err != nil {
				return nil, err
			}
			vVal, err := in.evalExpr(pair[1], env)
			if err != nil {
				return nil, err
			}
			m[kVal.String()] = escapeValue(vVal)
		}
		return HashValue(m), nil

	case *IndexExpr:
		arrVal, err := in.evalExpr(e.Array, env)
		if err != nil {
			return nil, err
		}
		if arrVal.Type == ValNil {
			if ve, ok := e.Array.(*VarExpr); ok && strings.HasPrefix(ve.Name, "$") {
				if _, hasS := env.Lookup(ve.Name); !hasS {
					if av, hasA := env.Lookup("@" + ve.Name[1:]); hasA {
						arrVal = av
					}
				}
			}
		}
		idxVal, err := in.evalExpr(e.Index, env)
		if err != nil {
			return nil, err
		}
		if arrVal.Type == ValCStruct {
			if fn, ok := env.Lookup("postcircumfix:[ ]"); ok {
				return in.InvokeCallable(fn, []*Value{arrVal, idxVal})
			}
		}
		if arrVal.Type != ValArray {
			return NilValue(), nil
		}
		idx := int(in.toInt(idxVal))
		n := arrVal.arrayLen()
		if idx < 0 {
			idx = n + idx
		}
		if idx < 0 || idx >= n {
			return NilValue(), nil
		}
		return arrVal.arrayAt(idx), nil

	case *HashAccessExpr:
		hVal, err := in.evalExpr(e.Hash, env)
		if err != nil {
			return nil, err
		}
		if hVal.Type == ValNil {
			if ve, ok := e.Hash.(*VarExpr); ok && strings.HasPrefix(ve.Name, "$") {
				if _, hasS := env.Lookup(ve.Name); !hasS {
					if hv, hasH := env.Lookup("%" + ve.Name[1:]); hasH {
						hVal = hv
					}
				}
			}
		}
		kVal, err := in.evalExpr(e.Key, env)
		if err != nil {
			return nil, err
		}
		if hVal.Type == ValCStruct {
			if fn, ok := env.Lookup("postcircumfix:{ }"); ok {
				return in.InvokeCallable(fn, []*Value{hVal, kVal})
			}
			if fn, ok := env.Lookup("postcircumfix:<{ }>"); ok {
				return in.InvokeCallable(fn, []*Value{hVal, kVal})
			}
		}
		if hVal.Type != ValHash || hVal.HashVal == nil {
			return NilValue(), nil
		}
		if val, ok := hVal.HashVal[kVal.String()]; ok {
			return val, nil
		}

		return NilValue(), nil

	case *ClosureExpr:
		frameSlots := e.FrameSlots
		paramSlots := e.ParamSlots
		needsEnv := e.NeedsEnv
		fastSimple := e.FastSimple
		noParamChecks := e.NoParamChecks
		if e.Body != nil && !e.Body.Resolved {
			frameSlots, paramSlots, needsEnv, fastSimple, noParamChecks = in.resolveFunction(e.Params, e.Body, nil)
		}
		return ClosureValue(&Closure{
			Params:        e.Params,
			Body:          e.Body,
			Env:           env,
			FrameSlots:    frameSlots,
			ParamSlots:    paramSlots,
			NeedsEnv:      needsEnv,
			FastSimple:    fastSimple,
			NoParamChecks: noParamChecks,
		}), nil

	case *CallExpr:
		var calleeVal *Value
		var funcName string
		if e.BuiltinChecked && e.BuiltinEpoch == in.builtinEpoch && e.CachedBuiltin != nil {
			base, args := in.ds.pushArgs(len(e.Args))
			for i, argExpr := range e.Args {
				aVal, err := in.evalExpr(argExpr, env)
				if err != nil {
					in.ds.popArgs(base)
					return nil, err
				}
				args[i] = aVal
			}
			res, err := e.CachedBuiltin(in, args)
			in.ds.popArgs(base)
			return res, err
		}
		if v, ok := e.Callee.(*VarExpr); ok {
			funcName = v.Name
			if !e.BuiltinChecked || e.BuiltinEpoch != in.builtinEpoch {
				if builtin, hasBuiltin := in.Builtins[funcName]; hasBuiltin {
					e.CachedBuiltin = builtin
					e.BuiltinChecked = true
					e.BuiltinEpoch = in.builtinEpoch
					base, args := in.ds.pushArgs(len(e.Args))
					for i, argExpr := range e.Args {
						aVal, err := in.evalExpr(argExpr, env)
						if err != nil {
							in.ds.popArgs(base)
							return nil, err
						}
						args[i] = aVal
					}
					res, err := builtin(in, args)
					in.ds.popArgs(base)
					return res, err
				}
				e.BuiltinChecked = true
				e.BuiltinEpoch = in.builtinEpoch
			}
		}

		if e.GlobalSubSlot >= 0 {
			calleeVal = in.globals.get(e.GlobalSubSlot)
		} else {
			cv, err := in.evalExpr(e.Callee, env)
			if (err != nil || cv == nil || cv.Type == ValNil) && funcName != "" {
				if autoVal, qualifiedName, hasAuto := in.lookupAutoload(funcName, env); hasAuto {
					base, args := in.ds.pushArgs(len(e.Args))
					for i, argExpr := range e.Args {
						aVal, aErr := in.evalExpr(argExpr, env)
						if aErr != nil {
							in.ds.popArgs(base)
							return nil, aErr
						}
						args[i] = aVal
					}
					in.GlobalEnv.Define("$AUTOLOAD", StringValue(qualifiedName))
					in.GlobalEnv.Define("AUTOLOAD", StringValue(qualifiedName))
					env.Define("$AUTOLOAD", StringValue(qualifiedName))
					res, err := in.InvokeCallable(autoVal, args)
					in.ds.popArgs(base)
					return res, err
				}
			}
			if err != nil {
				return nil, err
			}
			calleeVal = cv
		}

		base, args := in.ds.pushArgs(len(e.Args))
		for i, argExpr := range e.Args {
			aVal, err := in.evalExpr(argExpr, env)
			if err != nil {
				in.ds.popArgs(base)
				return nil, err
			}
			args[i] = aVal
		}

		if calleeVal != nil && (calleeVal.Type == ValClosure || calleeVal.Type == ValMultiSub || calleeVal.Type == ValCStruct || calleeVal.Type == ValRef) {
			res, err := in.InvokeCallable(calleeVal, args)
			in.ds.popArgs(base)
			return res, err
		}

		if funcName != "" {
			if autoVal, qualifiedName, hasAuto := in.lookupAutoload(funcName, env); hasAuto {
				in.GlobalEnv.Define("$AUTOLOAD", StringValue(qualifiedName))
				in.GlobalEnv.Define("AUTOLOAD", StringValue(qualifiedName))
				env.Define("$AUTOLOAD", StringValue(qualifiedName))
				res, err := in.InvokeCallable(autoVal, args)
				in.ds.popArgs(base)
				return res, err
			}
		}

		res, err := in.InvokeCallable(calleeVal, args)
		in.ds.popArgs(base)
		return res, err

	case *MethodCallExpr:
		invocantVal, err := in.evalExpr(e.Target, env)
		if err != nil {
			return nil, err
		}

		if invocantVal.Type == ValCStruct && invocantVal.CStruct() != nil && invocantVal.CStruct().Class != nil {
			if e.Method == "new" {
				buf := make([]byte, invocantVal.CStruct().Class.TotalSize)
				var ptr uintptr
				if len(buf) > 0 {
					ptr = uintptr(unsafe.Pointer(&buf[0]))
				}
				return CStructValue(&CStructInstance{
					Class:  invocantVal.CStruct().Class,
					Ptr:    ptr,
					Buffer: buf,
				}), nil
			}
			if idx, ok := invocantVal.CStruct().Class.FieldIndex[e.Method]; ok {
				f := invocantVal.CStruct().Class.Fields[idx]
				fieldVal, _ := in.readCStructField(invocantVal.CStruct(), f)
				if fieldVal != nil && fieldVal.Type == ValClosure && len(e.Args) > 0 {
					base, args := in.ds.pushArgs(len(e.Args))
					for i, argExpr := range e.Args {
						aVal, err := in.evalExpr(argExpr, env)
						if err != nil {
							in.ds.popArgs(base)
							return nil, err
						}
						args[i] = aVal
					}
					res, err := in.InvokeCallable(fieldVal, args)
					in.ds.popArgs(base)
					return res, err
				}
				if len(e.Args) == 0 {
					return fieldVal, nil
				} else if len(e.Args) == 1 {
					aVal, err := in.evalExpr(e.Args[0], env)
					if err != nil {
						return nil, err
					}
					return in.writeCStructField(invocantVal.CStruct(), f, aVal)
				}
			}
		}

		if invocantVal.Type == ValPromise && invocantVal.Promise() != nil {
			switch e.Method {
			case "result", "await":
				return invocantVal.Promise().Await()
			case "status":
				return StringValue(invocantVal.Promise().Status), nil
			case "then":
				if len(e.Args) > 0 {
					cbVal, err := in.evalExpr(e.Args[0], env)
					if err != nil {
						return nil, err
					}
					nextP := NewPromise()
					worker := in.forkWorker()
					go func() {
						res, err := invocantVal.Promise().Await()
						if err != nil {
							nextP.Break(err)
						} else {
							nextRes, nextErr := worker.InvokeCallable(cbVal, []*Value{res})
							if nextErr != nil {
								nextP.Break(nextErr)
							} else {
								nextP.Keep(nextRes)
							}
						}
					}()
					return PromiseValue(nextP), nil
				}
			}
		}

		if invocantVal.Type == ValChannel && invocantVal.Channel() != nil {
			switch e.Method {
			case "send":
				if len(e.Args) > 0 {
					sVal, err := in.evalExpr(e.Args[0], env)
					if err != nil {
						return nil, err
					}
					err = invocantVal.Channel().Send(sVal)
					if err != nil {
						return nil, err
					}
					return sVal, nil
				}
			case "receive", "recv", "get":
				return invocantVal.Channel().Receive()
			case "poll":
				return invocantVal.Channel().Poll(), nil
			case "close":
				invocantVal.Channel().Close()
				return NilValue(), nil
			}
		}

		if invocantVal.Type == ValString {
			if invocantVal.StrVal == "Promise" {
				switch e.Method {
				case "start":
					if len(e.Args) > 0 {
						cbVal, err := in.evalExpr(e.Args[0], env)
						if err != nil {
							return nil, err
						}
						return in.Builtins["start"](in, []*Value{cbVal})
					}
				case "new":
					return PromiseValue(NewPromise()), nil
				case "kept":
					p := NewPromise()
					if len(e.Args) > 0 {
						kVal, _ := in.evalExpr(e.Args[0], env)
						p.Keep(kVal)
					} else {
						p.Keep(NilValue())
					}
					return PromiseValue(p), nil
				}
			}
			if invocantVal.StrVal == "Channel" && e.Method == "new" {
				cap := 64
				if len(e.Args) > 0 {
					cVal, _ := in.evalExpr(e.Args[0], env)
					cap = int(in.toInt(cVal))
				}
				return ChannelValue(NewChannel(cap)), nil
			}
		}

		if invocantVal.Type == ValHash && invocantVal.HashVal != nil {
			if (e.Method == "parse" || e.Method == "subparse") && len(e.Args) >= 1 {
				if nv, ok := invocantVal.HashVal["name"]; ok && nv != nil && nv.Type == ValString {
					if cg, exists := in.CompiledGrammars[nv.StrVal]; exists && cg != nil {
						return in.evalGrammarParse(cg, e.Method == "parse", e.Args, env)
					}
				}
			}
			if v, ok := invocantVal.HashVal[e.Method]; ok && len(e.Args) == 0 {
				return v, nil
			}
		}

		base, args := in.ds.pushArgs(len(e.Args) + 1)
		args[0] = invocantVal
		for i, argExpr := range e.Args {
			aVal, err := in.evalExpr(argExpr, env)
			if err != nil {
				in.ds.popArgs(base)
				return nil, err
			}
			args[i+1] = aVal
		}

		// 1. Check built-ins (e.g. .uc, .lc, .elems, .substr, .split, .join, .push, .pop)
		if builtin, ok := in.Builtins[e.Method]; ok {
			res, err := builtin(in, args)
			in.ds.popArgs(base)
			return res, err
		}

		// 2. Check lexical environment for sub or multi sub
		if calleeVal, ok := env.Lookup(e.Method); ok && (calleeVal.Type == ValClosure || calleeVal.Type == ValMultiSub) {
			res, err := in.InvokeCallable(calleeVal, args)
			in.ds.popArgs(base)
			return res, err
		}
		if calleeVal, ok := in.GlobalEnv.Lookup(e.Method); ok && (calleeVal.Type == ValClosure || calleeVal.Type == ValMultiSub) {
			res, err := in.InvokeCallable(calleeVal, args)
			in.ds.popArgs(base)
			return res, err
		}
		if calleeVal, ok := env.Lookup("&" + e.Method); ok && (calleeVal.Type == ValClosure || calleeVal.Type == ValMultiSub) {
			res, err := in.InvokeCallable(calleeVal, args)
			in.ds.popArgs(base)
			return res, err
		}
		if calleeVal, ok := in.GlobalEnv.Lookup("&" + e.Method); ok && (calleeVal.Type == ValClosure || calleeVal.Type == ValMultiSub) {
			res, err := in.InvokeCallable(calleeVal, args)
			in.ds.popArgs(base)
			return res, err
		}

		// 3. Fallback to AUTOLOAD for method call
		if autoVal, qualifiedName, hasAuto := in.lookupAutoload(e.Method, env); hasAuto {
			in.GlobalEnv.Define("$AUTOLOAD", StringValue(qualifiedName))
			in.GlobalEnv.Define("AUTOLOAD", StringValue(qualifiedName))
			env.Define("$AUTOLOAD", StringValue(qualifiedName))
			res, err := in.InvokeCallable(autoVal, args)
			in.ds.popArgs(base)
			return res, err
		}

		in.ds.popArgs(base)
		return nil, fmt.Errorf("method or sub %q not found for invocant type %s", e.Method, invocantVal.TypeName())

	case *SmartMatchExpr:
		lVal, err := in.evalExpr(e.Left, env)
		if err != nil {
			return nil, err
		}
		rVal, err := in.evalExpr(e.Right, env)
		if err != nil {
			return nil, err
		}
		return BoolValue(in.smartMatch(lVal, rVal)), nil

	case *ChainedCompExpr:
		if len(e.Exprs) < 2 || len(e.Ops) < 1 {
			return BoolValue(false), nil
		}
		vals := make([]*Value, len(e.Exprs))
		for i, expr := range e.Exprs {
			v, err := in.evalExpr(expr, env)
			if err != nil {
				return nil, err
			}
			vals[i] = v
		}
		for i, op := range e.Ops {
			cmp, err := in.evalBinaryOp(vals[i], op, vals[i+1])
			if err != nil {
				return nil, err
			}
			if !cmp.IsTrue() {
				return BoolValue(false), nil
			}
		}
		return BoolValue(true), nil

	case *InterpStringExpr:
		var sb strings.Builder
		for _, part := range e.Parts {
			v, err := in.evalExpr(part, env)
			if err != nil {
				return nil, err
			}
			sb.WriteString(v.String())
		}
		return StringValue(sb.String()), nil

	case *GatherExpr:
		in.GatherStack = append(in.GatherStack, make([]*Value, 0))
		_, err := in.evalBlock(e.Body, env.NewChild())
		if err != nil {
			in.GatherStack = in.GatherStack[:len(in.GatherStack)-1]
			return nil, err
		}
		collected := in.GatherStack[len(in.GatherStack)-1]
		in.GatherStack = in.GatherStack[:len(in.GatherStack)-1]
		return LazySeqValue(collected), nil

	case *AssignStmt:
		return in.evalStmt(e, env)

	case *BacktickExpr:
		cmdVal, err := in.evalExpr(e.Command, env)
		if err != nil {
			return nil, err
		}
		cmdStr := cmdVal.String()
		var cmd *exec.Cmd
		if goruntime.GOOS == "windows" {
			cmd = exec.Command("powershell", "-NoProfile", "-Command", cmdStr)
		} else {
			cmd = exec.Command("sh", "-c", cmdStr)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			exitCode := 1
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			in.GlobalEnv.Define("$?", IntValue(int64(exitCode)))
			in.GlobalEnv.Define("$!", StringValue(err.Error()))
			env.Define("$?", IntValue(int64(exitCode)))
			env.Define("$!", StringValue(err.Error()))
		} else {
			in.GlobalEnv.Define("$?", IntValue(0))
			in.GlobalEnv.Define("$!", NilValue())
			env.Define("$?", IntValue(0))
			env.Define("$!", NilValue())
		}
		return StringValue(strings.TrimRight(string(out), "\r\n")), nil

	case *StubExpr:
		return nil, fmt.Errorf("Stub code executed (%s)", e.Message)

	default:
		return NilValue(), nil
	}

}

// smartMatch implements Raku's ~~ smart match semantics.
func (in *Interp) smartMatch(topic *Value, matcher *Value) bool {
	switch matcher.Type {
	case ValInt:
		return topic.Type == ValInt && topic.IntVal == matcher.IntVal
	case ValFloat:
		return topic.Type == ValFloat && topic.FloatVal == matcher.FloatVal
	case ValString:
		// String match: check type name or equality
		typeName := matcher.StrVal
		switch typeName {
		case "Int":
			return topic.Type == ValInt
		case "Str":
			return topic.Type == ValString
		case "Num":
			return topic.Type == ValFloat
		case "Array":
			return topic.Type == ValArray
		case "Hash":
			return topic.Type == ValHash
		case "Bool":
			return topic.Type == ValBool
		default:
			return topic.Type == ValString && topic.StrVal == matcher.StrVal
		}
	case ValArray:
		if ints := matcher.intLane(); ints != nil {
			if topic.Type == ValInt {
				for _, n := range ints {
					if topic.IntVal == n {
						return true
					}
				}
			}
			return false
		}
		if tape := matcher.strLane(); tape != nil {
			if topic.Type == ValString {
				s := topic.StrVal
				for i := 0; i < tape.Len(); i++ {
					if tape.At(i) == s {
						return true
					}
				}
			}
			return false
		}
		for _, elem := range matcher.materializeArray() {
			if in.smartMatch(topic, elem) {
				return true
			}
		}
		return false
	case ValClosure:
		// Invoke the closure as a predicate
		result, err := in.InvokeCallable(matcher, []*Value{topic})
		if err != nil {
			return false
		}
		return result.IsTrue()
	case ValBool:
		return matcher.BoolVal
	case ValJunction:
		j := matcher.Junction()
		if j == nil {
			return false
		}
		switch j.Kind {
		case JunctionAll:
			for _, elem := range j.Values {
				if !in.smartMatch(topic, elem) {
					return false
				}
			}
			return len(j.Values) > 0
		case JunctionAny:
			for _, elem := range j.Values {
				if in.smartMatch(topic, elem) {
					return true
				}
			}
			return false
		case JunctionOne:
			count := 0
			for _, elem := range j.Values {
				if in.smartMatch(topic, elem) {
					count++
				}
			}
			return count == 1
		case JunctionNone:
			for _, elem := range j.Values {
				if in.smartMatch(topic, elem) {
					return false
				}
			}
			return true
		}
		return false
	default:
		return false
	}
}

func (in *Interp) invokeClosureFast(closure *Closure, args []*Value) (*Value, error) {
	callerTempMark := in.ds.markTemp()
	frame := in.ds.pushFrame(closure.FrameSlots, len(args))
	for i := 0; i < len(args) && i < len(closure.ParamSlots); i++ {
		slot := closure.ParamSlots[i]
		if slot < 0 || slot >= len(frame.boxes) {
			continue
		}
		arg := args[i]
		if arg.Type == ValArray || arg.Type == ValHash || arg.Type == ValRef {
			frame.ptrs[slot] = escapeValue(arg)
		} else {
			frame.boxes[slot] = *arg
			frame.boxes[slot].flags = flagStackLocal
			frame.ptrs[slot] = &frame.boxes[slot]
		}
	}
	res, err := in.evalBlock(closure.Body, closure.Env)
	if err != nil {
		if err == sigReturn {
			rv := in.ds.returnVal
			in.ds.returnVal = nil
			in.ds.popFrame(frame)
			return in.promoteReturn(rv, callerTempMark), nil
		}
		in.ds.popFrame(frame)
		if ret, ok := err.(*ReturnSignal); ok {
			return in.promoteReturn(ret.Value, callerTempMark), nil
		}
		return nil, err
	}
	in.ds.popFrame(frame)
	return in.promoteReturn(res, callerTempMark), nil
}

// InvokeCallable resolves candidate dispatch and invokes the closure with advice hooks.
func (in *Interp) InvokeCallable(callee *Value, args []*Value) (*Value, error) {
	if callee == nil {
		return nil, fmt.Errorf("cannot invoke nil callable")
	}

	var closure *Closure
	var subName string

	if callee.Type == ValMultiSub {
		cand, err := in.resolveMultiCandidate(callee.Candidates(), args)
		if err != nil {
			return nil, err
		}
		closure = cand
		subName = cand.Name
	} else if callee.Type == ValClosure {
		closure = callee.Closure()
		if closure != nil {
			subName = closure.Name
		}
	} else {
		return nil, fmt.Errorf("cannot invoke non-callable type %s", callee.TypeName())
	}

	if closure == nil {
		return nil, fmt.Errorf("closure body is empty")
	}

	if builtin, ok := in.Builtins[subName]; ok {
		return builtin(in, args)
	}

	if !in.hasAdviceHooks && closure.FastSimple {
		return in.invokeClosureFast(closure, args)
	}

	callerTempMark := in.ds.markTemp()

	// 1. Run before hooks (if not an unwrapped raw call)
	if !closure.IsRaw {
		if hooks, ok := in.BeforeHooks[subName]; ok {
			for _, hook := range hooks {
				matches := true
				if len(hook.Params) > len(args) {
					matches = false
				} else {
					for i, p := range hook.Params {
						if p.Type != "" && (i >= len(args) || !args[i].MatchesType(p.Type)) {
							matches = false
							break
						}
					}
				}
				if !matches {
					continue
				}
				hookEnv := in.GlobalEnv.NewChild()
				for i, p := range hook.Params {
					if i < len(args) {
						hookEnv.Define(p.Name, args[i])
					}
				}
				_, err := in.evalBlock(hook.Body, hookEnv)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// 2. Run around hooks if present (if not an unwrapped raw call)
	if !closure.IsRaw {
		if aroundHooks, ok := in.AroundHooks[subName]; ok && len(aroundHooks) > 0 {
			for idx := len(aroundHooks) - 1; idx >= 0; idx-- {
				hook := aroundHooks[idx]
				matches := true
				if len(hook.Params) > 1 {
					for i, p := range hook.Params[1:] {
						if p.Type != "" && (i >= len(args) || !args[i].MatchesType(p.Type)) {
							matches = false
							break
						}
					}
				}
				if !matches {
					continue
				}
				hookEnv := in.GlobalEnv.NewChild()
				rawClosure := *closure
				rawClosure.IsRaw = true
				origCallable := ClosureValue(&rawClosure)
				if len(hook.Params) > 0 {
					hookEnv.Define(hook.Params[0].Name, origCallable)
				}
				for i, p := range hook.Params[1:] {
					if i < len(args) {
						hookEnv.Define(p.Name, args[i])
					}
				}
				val, err := in.evalBlock(hook.Body, hookEnv)
				if err != nil {
					if err == sigReturn {
						val = in.ds.returnVal
						in.ds.returnVal = nil
					} else if ret, ok := err.(*ReturnSignal); ok {
						val = ret.Value
					} else {
						return nil, err
					}
				}
				return in.promoteReturn(val, callerTempMark), nil
			}
		}
	}

	// 3. Execute primary closure
	callEnv := closure.Env.NewChild()
	for i, p := range closure.Params {
		var argVal *Value = NilValue()
		if i < len(args) {
			argVal = args[i]
		}
		if p.Type != "" {
			if argVal.Type == ValCStruct && argVal.CStruct() != nil && argVal.CStruct().Class != nil && argVal.CStruct().Class.Name == p.Type {
				// struct matched
			} else if _, ok := in.Structs[p.Type]; ok {
				return nil, fmt.Errorf("type check failed for parameter %s in sub %s: expected struct %s, got %s",
					p.Name, closure.Name, p.Type, argVal.TypeName())
			} else if subVal, ok := in.GlobalEnv.Lookup(p.Type); ok && subVal.Type == ValClosure {
				r, err := in.InvokeCallable(subVal, []*Value{argVal})
				if err != nil || !r.IsTrue() {
					return nil, fmt.Errorf("type check failed for parameter %s in sub %s: expected subset %s, got %s",
						p.Name, closure.Name, p.Type, argVal.TypeName())
				}
			} else if !argVal.MatchesType(p.Type) {
				return nil, fmt.Errorf("type check failed for parameter %s in sub %s: expected %s, got %s",
					p.Name, closure.Name, p.Type, argVal.TypeName())
			}
		}
		if p.Where != nil {
			whereEnv := callEnv.NewChild()
			whereEnv.Define("$_", argVal)
			whereEnv.Define(p.Name, argVal)
			res, err := in.evalExpr(p.Where, whereEnv)
			if err != nil {
				return nil, err
			}
			if res != nil && res.Type == ValClosure {
				r, err := in.InvokeCallable(res, []*Value{argVal})
				if err != nil || !r.IsTrue() {
					return nil, fmt.Errorf("where constraint failed for parameter %s in sub %s", p.Name, closure.Name)
				}
			} else if res != nil && !res.IsTrue() {
				return nil, fmt.Errorf("where constraint failed for parameter %s in sub %s", p.Name, closure.Name)
			}
		}
		callEnv.Define(p.Name, argVal)

		// Array destructuring: [$head, *@tail]
		if len(p.DestructArr) > 0 && (argVal.Type == ValArray || argVal.Type == ValLazySeq) {
			var elements []*Value
			if argVal.Type == ValArray {
				elements = argVal.materializeArray()
			} else if argVal.LazySeq() != nil {
				elements = argVal.LazySeq().Items
			}
			for dIdx, dp := range p.DestructArr {
				if dp.IsSlurpy {
					var tail []*Value
					if dIdx < len(elements) {
						tail = elements[dIdx:]
					}
					callEnv.Define(dp.Name, ArrayValue(tail))
				} else {
					var elemVal *Value = NilValue()
					if dIdx < len(elements) {
						elemVal = elements[dIdx]
					}
					callEnv.Define(dp.Name, elemVal)
				}
			}
		}

		// Hash destructuring: :{:$name, :$age}
		if len(p.DestructHash) > 0 && argVal.Type == ValHash {
			for _, hp := range p.DestructHash {
				key := strings.TrimPrefix(hp.Name, "$")
				key = strings.TrimPrefix(key, ":")
				var val *Value = NilValue()
				if v, ok := argVal.HashVal[key]; ok {
					val = v
				} else if v, ok := argVal.HashVal[":"+key]; ok {
					val = v
				}
				callEnv.Define(hp.Name, val)
			}
		}
	}

	var frame *stackFrame
	if closure.FrameSlots > 0 {
		frame = in.ds.pushFrame(closure.FrameSlots, len(args))
		for i := 0; i < len(args) && i < len(closure.ParamSlots); i++ {
			slot := closure.ParamSlots[i]
			if slot < 0 || slot >= len(frame.boxes) {
				continue
			}
			arg := args[i]
			if arg.Type == ValArray || arg.Type == ValHash || arg.Type == ValRef {
				frame.ptrs[slot] = escapeValue(arg)
			} else {
				frame.boxes[slot] = *arg
				frame.boxes[slot].flags = flagStackLocal
				frame.ptrs[slot] = &frame.boxes[slot]
			}
		}
	}

	val, err := in.evalBlock(closure.Body, callEnv)
	if err != nil {
		if err == sigReturn {
			val = in.ds.returnVal
			in.ds.returnVal = nil
		} else if ret, ok := err.(*ReturnSignal); ok {
			val = ret.Value
		} else {
			if frame != nil {
				in.ds.popFrame(frame)
			}
			return nil, err
		}
	}
	if frame != nil {
		in.ds.popFrame(frame)
	}

	// 4. Run after hooks
	if hooks, ok := in.AfterHooks[subName]; ok {
		for _, hook := range hooks {
			matches := true
			if len(hook.Params) > len(args) {
				matches = false
			} else {
				for i, p := range hook.Params {
					if p.Type != "" && (i >= len(args) || !args[i].MatchesType(p.Type)) {
						matches = false
						break
					}
				}
			}
			if !matches {
				continue
			}
			hookEnv := in.GlobalEnv.NewChild()
			for i, p := range hook.Params {
				if i < len(args) {
					hookEnv.Define(p.Name, args[i])
				}
			}
			_, err := in.evalBlock(hook.Body, hookEnv)
			if err != nil {
				return nil, err
			}
		}
	}

	return in.promoteReturn(val, callerTempMark), nil
}

func (in *Interp) resolveMultiCandidate(candidates []*Closure, args []*Value) (*Closure, error) {
	bestScore := -1
	var bestCand *Closure

	for _, cand := range candidates {
		if len(cand.Params) != len(args) {
			continue
		}
		matches := true
		score := 0
		for i, p := range cand.Params {
			if p.Type != "" {
				if args[i].Type == ValCStruct && args[i].CStruct() != nil && args[i].CStruct().Class != nil && args[i].CStruct().Class.Name == p.Type {
					score += 2
				} else if _, ok := in.Structs[p.Type]; ok {
					matches = false
					break
				} else if subVal, ok := in.GlobalEnv.Lookup(p.Type); ok && subVal.Type == ValClosure {
					r, err := in.InvokeCallable(subVal, []*Value{args[i]})
					if err != nil || !r.IsTrue() {
						matches = false
						break
					}
					score += 2
				} else if p.Type == "Any" || p.Type == "any" {
					score += 1
				} else if args[i].MatchesType(p.Type) {
					score += 2
				} else {
					matches = false
					break
				}
			}
			if p.Where != nil {
				whereEnv := cand.Env.NewChild()
				whereEnv.Define("$_", args[i])
				whereEnv.Define(p.Name, args[i])
				res, err := in.evalExpr(p.Where, whereEnv)
				if err != nil {
					matches = false
					break
				}
				if res != nil && res.Type == ValClosure {
					r, err := in.InvokeCallable(res, []*Value{args[i]})
					if err != nil || !r.IsTrue() {
						matches = false
						break
					}
				} else if res != nil && !res.IsTrue() {
					matches = false
					break
				}
				score += 3
			}
		}
		if matches {
			if score > bestScore {
				bestScore = score
				bestCand = cand
			}
		}
	}

	if bestCand == nil {
		return nil, fmt.Errorf("no matching multi candidate found for arguments")
	}
	return bestCand, nil
}

func (in *Interp) evalBinaryFast(e *BinaryExpr, env *Env) (*Value, bool) {
	if len(in.CustomInfixOps) > 0 {
		return nil, false
	}
	switch e.Op {
	case "+", "-", "*", "div", "%", "mod":
		if res, ok := in.evalIntExprFast(e, env); ok {
			return in.tempInt(res), true
		}
	}

	lv, ok := e.Left.(*VarExpr)
	if !ok {
		return nil, false
	}
	left, ok := in.getVar(lv, env)
	if !ok || left == nil {
		return nil, false
	}
	switch e.Op {
	case "<=", ">=", "<", ">", "==", "!=", "%%":
		if left.Type != ValInt {
			return nil, false
		}
		var ri int64
		switch r := e.Right.(type) {
		case *LiteralExpr:
			if r.Type != TokInt {
				return nil, false
			}
			ri = r.Value.(int64)
		case *VarExpr:
			rv, ok := in.getVar(r, env)
			if !ok || rv == nil || rv.Type != ValInt {
				return nil, false
			}
			ri = rv.IntVal
		default:
			return nil, false
		}
		li := left.IntVal
		switch e.Op {
		case "<=":
			return BoolValue(li <= ri), true
		case ">=":
			return BoolValue(li >= ri), true
		case "<":
			return BoolValue(li < ri), true
		case ">":
			return BoolValue(li > ri), true
		case "==":
			return BoolValue(li == ri), true
		case "!=":
			return BoolValue(li != ri), true
		case "%%":
			if ri == 0 {
				return nil, false
			}
			return BoolValue(li%ri == 0), true
		}
	case "eq", "ne", "lt", "gt", "le", "ge":
		if left.Type != ValString {
			return nil, false
		}
		var rs string
		switch r := e.Right.(type) {
		case *LiteralExpr:
			if r.Type != TokString {
				return nil, false
			}
			rs = r.Value.(string)
		case *VarExpr:
			rv, ok := in.getVar(r, env)
			if !ok || rv == nil || rv.Type != ValString {
				return nil, false
			}
			rs = rv.StrVal
		default:
			return nil, false
		}
		ls := left.StrVal
		switch e.Op {
		case "eq":
			return BoolValue(ls == rs), true
		case "ne":
			return BoolValue(ls != rs), true
		case "lt":
			return BoolValue(ls < rs), true
		case "gt":
			return BoolValue(ls > rs), true
		case "le":
			return BoolValue(ls <= rs), true
		case "ge":
			return BoolValue(ls >= rs), true
		}
	}
	return nil, false
}

func (in *Interp) evalBinaryOp(left *Value, op string, right *Value) (*Value, error) {
	// Junction autothreading
	if left.Type == ValJunction && left.Junction() != nil {
		lj := left.Junction()
		var results []*Value
		for _, elem := range lj.Values {
			res, err := in.evalBinaryOp(elem, op, right)
			if err != nil {
				return nil, err
			}
			results = append(results, res)
		}
		return JunctionValue(lj.Kind, results), nil
	}
	if right.Type == ValJunction && right.Junction() != nil {
		rj := right.Junction()
		var results []*Value
		for _, elem := range rj.Values {
			res, err := in.evalBinaryOp(left, op, elem)
			if err != nil {
				return nil, err
			}
			results = append(results, res)
		}
		return JunctionValue(rj.Kind, results), nil
	}

	// 1. Check for custom operator overloading (e.g. infix:+ or infix:<+>)
	if len(in.CustomInfixOps) > 0 {
		if fn, ok := in.CustomInfixOps[op]; ok {
			if fn.Type == ValMultiSub {
				if cand, err := in.resolveMultiCandidate(fn.Candidates(), []*Value{left, right}); err == nil && cand != nil {
					return in.InvokeCallable(fn, []*Value{left, right})
				}
			} else {
				return in.InvokeCallable(fn, []*Value{left, right})
			}
		}
	}

	// 2. Unicode Set operators
	switch op {
	case "∈": // is element of
		if right.Type == ValArray {
			for _, item := range right.materializeArray() {
				if in.smartMatch(left, item) {
					return BoolValue(true), nil
				}
			}
			return BoolValue(false), nil
		}
	case "∉": // is not element of
		if right.Type == ValArray {
			for _, item := range right.materializeArray() {
				if in.smartMatch(left, item) {
					return BoolValue(false), nil
				}
			}
			return BoolValue(true), nil
		}
	case "∩": // set intersection
		if left.Type == ValArray && right.Type == ValArray {
			var inter []*Value
			rItems := right.materializeArray()
			for _, a := range left.materializeArray() {
				for _, b := range rItems {
					if in.smartMatch(a, b) {
						inter = append(inter, a)
						break
					}
				}
			}
			return ArrayValue(inter), nil
		}
	case "∪": // set union
		if left.Type == ValArray && right.Type == ValArray {
			unionMap := make(map[string]*Value)
			var unionList []*Value
			for _, a := range left.materializeArray() {
				s := a.String()
				if _, ok := unionMap[s]; !ok {
					unionMap[s] = a
					unionList = append(unionList, a)
				}
			}
			for _, b := range right.materializeArray() {
				s := b.String()
				if _, ok := unionMap[s]; !ok {
					unionMap[s] = b
					unionList = append(unionList, b)
				}
			}
			return ArrayValue(unionList), nil
		}
	}

	switch op {
	case "+", "±":
		if left.Type == ValFloat || right.Type == ValFloat {
			return in.tempFloat(in.toFloat(left) + in.toFloat(right)), nil
		}
		return in.tempInt(in.toInt(left) + in.toInt(right)), nil
	case "-":
		if left.Type == ValFloat || right.Type == ValFloat {
			return in.tempFloat(in.toFloat(left) - in.toFloat(right)), nil
		}
		return in.tempInt(in.toInt(left) - in.toInt(right)), nil
	case "*", "×":
		if left.Type == ValFloat || right.Type == ValFloat {
			return in.tempFloat(in.toFloat(left) * in.toFloat(right)), nil
		}
		return in.tempInt(in.toInt(left) * in.toInt(right)), nil
	case "/", "÷":
		rF := in.toFloat(right)
		if rF == 0.0 {
			return nil, fmt.Errorf("division by zero")
		}
		return in.tempFloat(in.toFloat(left) / rF), nil
	case "%":
		rI := in.toInt(right)
		if rI == 0 {
			return nil, fmt.Errorf("modulo by zero")
		}
		return in.tempInt(in.toInt(left) % rI), nil
	case "**":
		return in.tempFloat(math.Pow(in.toFloat(left), in.toFloat(right))), nil

	case "//", "orelse":
		if left.Type != ValNil {
			return left, nil
		}
		return right, nil

	case "xor":
		return BoolValue(left.IsTrue() != right.IsTrue()), nil

	case "<=>":
		lf := in.toFloat(left)
		rf := in.toFloat(right)
		if lf < rf {
			return in.tempInt(-1), nil
		}
		if lf > rf {
			return in.tempInt(1), nil
		}
		return in.tempInt(0), nil

	case "cmp":
		ls := left.String()
		rs := right.String()
		if ls < rs {
			return in.tempInt(-1), nil
		}
		if ls > rs {
			return in.tempInt(1), nil
		}
		return in.tempInt(0), nil

	case "+&":
		return in.tempInt(in.toInt(left) & in.toInt(right)), nil
	case "+|":
		return in.tempInt(in.toInt(left) | in.toInt(right)), nil
	case "+^":
		return in.tempInt(in.toInt(left) ^ in.toInt(right)), nil
	case "+<":
		return in.tempInt(in.toInt(left) << in.toInt(right)), nil
	case "+>":
		return in.tempInt(in.toInt(left) >> in.toInt(right)), nil

	case "div":
		rI := in.toInt(right)
		if rI == 0 {
			return nil, fmt.Errorf("division by zero in div operator")
		}
		return in.tempInt(in.toInt(left) / rI), nil

	case "mod":
		rI := in.toInt(right)
		if rI == 0 {
			return nil, fmt.Errorf("modulo by zero in mod operator")
		}
		return in.tempInt(in.toInt(left) % rI), nil

	case "%%":
		rI := in.toInt(right)
		if rI == 0 {
			return nil, fmt.Errorf("division by zero in %% operator")
		}
		return BoolValue(in.toInt(left)%rI == 0), nil

	case "min":
		if in.compareValues(left, right) <= 0 {
			return left, nil
		}
		return right, nil

	case "max":
		if in.compareValues(left, right) >= 0 {
			return left, nil
		}
		return right, nil

	case "∈":
		return BoolValue(in.smartMatch(left, right)), nil
	case "∉":
		return BoolValue(!in.smartMatch(left, right)), nil
	case "=~":
		pattern := right.String()
		matched, err := RegexMatch(pattern, left.String())
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
		}
		return BoolValue(matched), nil

	case "!~":
		pattern := right.String()
		matched, err := RegexMatch(pattern, left.String())
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
		}
		return BoolValue(!matched), nil

	case "==":
		if left.Type == ValString && right.Type == ValString {
			return BoolValue(left.StrVal == right.StrVal), nil
		}
		return BoolValue(in.toFloat(left) == in.toFloat(right)), nil
	case "!=":
		if left.Type == ValString && right.Type == ValString {
			return BoolValue(left.StrVal != right.StrVal), nil
		}
		return BoolValue(in.toFloat(left) != in.toFloat(right)), nil
	case "<":
		return BoolValue(in.toFloat(left) < in.toFloat(right)), nil
	case "<=":
		return BoolValue(in.toFloat(left) <= in.toFloat(right)), nil
	case ">":
		return BoolValue(in.toFloat(left) > in.toFloat(right)), nil
	case ">=":
		return BoolValue(in.toFloat(left) >= in.toFloat(right)), nil

	case "eq":
		return BoolValue(left.String() == right.String()), nil
	case "ne":
		return BoolValue(left.String() != right.String()), nil
	case "lt":
		return BoolValue(left.String() < right.String()), nil
	case "gt":
		return BoolValue(left.String() > right.String()), nil
	case "le":
		return BoolValue(left.String() <= right.String()), nil
	case "ge":
		return BoolValue(left.String() >= right.String()), nil
	case "~~":
		return BoolValue(in.smartMatch(left, right)), nil

	case "~":
		return StringValue(left.String() + right.String()), nil
	case "x":
		count := int(in.toInt(right))
		if count < 0 {
			count = 0
		}
		return StringValue(strings.Repeat(left.String(), count)), nil

	case "xx":
		count := int(in.toInt(right))
		if count < 0 {
			count = 0
		}
		var list []*Value
		if left.Type == ValArray {
			elems := left.materializeArray()
			for c := 0; c < count; c++ {
				list = append(list, elems...)
			}
		} else {
			for c := 0; c < count; c++ {
				list = append(list, left)
			}
		}
		return ArrayValue(list), nil

	case "..":
		start := in.toInt(left)
		end := in.toInt(right)
		if start > end {
			return IntArrayValue(nil), nil
		}
		n := end - start + 1
		ints := make([]int64, n)
		for i := int64(0); i < n; i++ {
			ints[i] = start + i
		}
		return IntArrayValue(ints), nil

	default:
		// Check for custom operator in GlobalEnv
		if fn, ok := in.GlobalEnv.Lookup("infix:<" + op + ">"); ok {
			return in.InvokeCallable(fn, []*Value{left, right})
		}
		if fn, ok := in.GlobalEnv.Lookup("infix:" + op); ok {
			return in.InvokeCallable(fn, []*Value{left, right})
		}
		return nil, fmt.Errorf("unknown binary operator %q", op)
	}
}

func (in *Interp) evalUnaryOp(op string, rVal *Value, env *Env) (*Value, error) {
	// Custom prefix operator lookup
	if len(in.CustomPrefixOps) > 0 {
		if fn, ok := in.CustomPrefixOps[op]; ok {
			if fn.Type == ValMultiSub {
				if cand, err := in.resolveMultiCandidate(fn.Candidates(), []*Value{rVal}); err == nil && cand != nil {
					return in.InvokeCallable(fn, []*Value{rVal})
				}
			} else {
				return in.InvokeCallable(fn, []*Value{rVal})
			}
		}
	}

	switch op {
	case "!", "not":
		return BoolValue(!rVal.IsTrue()), nil
	case "-":
		if rVal.Type == ValFloat {
			return in.tempFloat(-rVal.FloatVal), nil
		}
		return in.tempInt(-in.toInt(rVal)), nil
	case "+":
		// Numeric context coercion
		switch rVal.Type {
		case ValInt:
			return rVal, nil
		case ValFloat:
			return rVal, nil
		case ValString:
			if f, err := strconv.ParseFloat(rVal.StrVal, 64); err == nil {
				if i, err2 := strconv.ParseInt(rVal.StrVal, 0, 64); err2 == nil {
					return in.tempInt(i), nil
				}
				return in.tempFloat(f), nil
			}
			return in.tempInt(0), nil
		case ValBool:
			if rVal.BoolVal {
				return in.tempInt(1), nil
			}
			return in.tempInt(0), nil
		case ValArray:
			return in.tempInt(int64(rVal.arrayLen())), nil
		case ValHash:
			return in.tempInt(int64(len(rVal.HashVal))), nil
		case ValLazySeq:
			if rVal.LazySeq() != nil {
				return in.tempInt(int64(len(rVal.LazySeq().Items))), nil
			}
			return in.tempInt(0), nil
		case ValNil:
			return in.tempInt(0), nil
		default:
			return in.tempInt(in.toInt(rVal)), nil
		}
	case "~":
		// String context coercion
		switch rVal.Type {
		case ValString:
			return rVal, nil
		case ValInt:
			return StringValue(strconv.FormatInt(rVal.IntVal, 10)), nil
		case ValFloat:
			return StringValue(strconv.FormatFloat(rVal.FloatVal, 'f', -1, 64)), nil
		case ValBool:
			if rVal.BoolVal {
				return StringValue("True"), nil
			}
			return StringValue("False"), nil
		case ValNil:
			return StringValue(""), nil
		case ValArray:
			if ints := rVal.intLane(); ints != nil {
				parts := make([]string, len(ints))
				for i, n := range ints {
					parts[i] = strconv.FormatInt(n, 10)
				}
				return StringValue(strings.Join(parts, " ")), nil
			}
			parts := make([]string, len(rVal.ArrayVal))
			for i, v := range rVal.ArrayVal {
				parts[i] = v.String()
			}
			return StringValue(strings.Join(parts, " ")), nil
		default:
			return StringValue(rVal.String()), nil
		}
	case "?", "so":
		// Boolean context coercion
		return BoolValue(rVal.IsTrue()), nil

	// File test operators
	case "-e":
		_, err := os.Stat(rVal.String())
		return BoolValue(err == nil), nil
	case "-f":
		fi, err := os.Stat(rVal.String())
		return BoolValue(err == nil && !fi.IsDir()), nil
	case "-d":
		fi, err := os.Stat(rVal.String())
		return BoolValue(err == nil && fi.IsDir()), nil
	case "-s":
		fi, err := os.Stat(rVal.String())
		if err != nil {
			return IntValue(0), nil
		}
		return IntValue(fi.Size()), nil
	case "-r":
		f, err := os.Open(rVal.String())
		if err == nil {
			f.Close()
			return BoolValue(true), nil
		}
		return BoolValue(false), nil
	case "-w":
		f, err := os.OpenFile(rVal.String(), os.O_WRONLY, 0)
		if err == nil {
			f.Close()
			return BoolValue(true), nil
		}
		return BoolValue(false), nil

	default:
		return NilValue(), nil
	}
}

func (in *Interp) toInt(v *Value) int64 {

	if v == nil {
		return 0
	}
	switch v.Type {
	case ValInt:
		return v.IntVal
	case ValFloat:
		return int64(v.FloatVal)
	case ValNativePtr:
		return int64(v.PtrVal)
	case ValCStruct:
		return int64(v.PtrVal)
	case ValString:
		i, _ := strconv.ParseInt(v.StrVal, 0, 64)
		return i
	case ValBool:
		if v.BoolVal {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func (in *Interp) toFloat(v *Value) float64 {
	if v == nil {
		return 0.0
	}
	switch v.Type {
	case ValFloat:
		return v.FloatVal
	case ValInt:
		return float64(v.IntVal)
	case ValNativePtr:
		return float64(v.PtrVal)
	case ValCStruct:
		return float64(v.PtrVal)
	case ValString:
		var f float64
		fmt.Sscanf(v.StrVal, "%f", &f)
		return f
	case ValBool:
		if v.BoolVal {
			return 1.0
		}
		return 0.0
	default:
		return 0.0
	}
}

func (in *Interp) readCStructField(inst *CStructInstance, f CStructField) (*Value, error) {
	if inst.Closures != nil {
		if cVal, ok := inst.Closures[f.Name]; ok {
			return cVal, nil
		}
	}
	var ptr unsafe.Pointer
	if len(inst.Buffer) > 0 {
		ptr = unsafe.Pointer(&inst.Buffer[f.Offset])
	} else if inst.Ptr != 0 {
		ptr = unsafe.Pointer(inst.Ptr + uintptr(f.Offset))
	} else {
		return NilValue(), nil
	}
	switch f.Type {
	case "int8", "char":
		return in.tempInt(int64(*(*int8)(ptr))), nil
	case "uint8", "byte":
		return in.tempInt(int64(*(*uint8)(ptr))), nil
	case "int16", "short":
		return in.tempInt(int64(*(*int16)(ptr))), nil
	case "uint16", "WORD":
		return in.tempInt(int64(*(*uint16)(ptr))), nil
	case "int32", "int", "long":
		return in.tempInt(int64(*(*int32)(ptr))), nil
	case "uint32", "uint", "DWORD":
		return in.tempInt(int64(*(*uint32)(ptr))), nil
	case "int64", "Int":
		return in.tempInt(*(*int64)(ptr)), nil
	case "uint64":
		return in.tempInt(int64(*(*uint64)(ptr))), nil
	case "num32", "float32":
		return in.tempFloat(float64(*(*float32)(ptr))), nil
	case "num64", "float64", "Num", "double":
		return in.tempFloat(*(*float64)(ptr)), nil
	case "ptr", "pointer", "Pointer", "OpaquePointer", "Callable", "closure":
		return NativePtrValue(*(*uintptr)(ptr)), nil
	default:
		return in.tempInt(*(*int64)(ptr)), nil
	}
}

func (in *Interp) writeCStructField(inst *CStructInstance, f CStructField, val *Value) (*Value, error) {
	if val.Type == ValClosure {
		if inst.Closures == nil {
			inst.Closures = make(map[string]*Value)
		}
		inst.Closures[f.Name] = val
	}
	var ptr unsafe.Pointer
	if len(inst.Buffer) > 0 {
		ptr = unsafe.Pointer(&inst.Buffer[f.Offset])
	} else if inst.Ptr != 0 {
		ptr = unsafe.Pointer(inst.Ptr + uintptr(f.Offset))
	} else {
		return val, nil
	}
	switch f.Type {
	case "int8", "char":
		*(*int8)(ptr) = int8(in.toInt(val))
	case "uint8", "byte":
		*(*uint8)(ptr) = uint8(in.toInt(val))
	case "int16", "short":
		*(*int16)(ptr) = int16(in.toInt(val))
	case "uint16", "WORD":
		*(*uint16)(ptr) = uint16(in.toInt(val))
	case "int32", "int", "long":
		*(*int32)(ptr) = int32(in.toInt(val))
	case "uint32", "uint", "DWORD":
		*(*uint32)(ptr) = uint32(in.toInt(val))
	case "int64", "Int":
		*(*int64)(ptr) = in.toInt(val)
	case "uint64":
		*(*uint64)(ptr) = uint64(in.toInt(val))
	case "num32", "float32":
		*(*float32)(ptr) = float32(in.toFloat(val))
	case "num64", "float64", "Num", "double":
		*(*float64)(ptr) = in.toFloat(val)
	case "ptr", "pointer", "Pointer", "OpaquePointer", "Callable", "closure":
		*(*uintptr)(ptr) = val.PtrVal
	default:
		*(*int64)(ptr) = in.toInt(val)
	}
	return val, nil
}

func normalizeOpName(name string) string {
	if strings.Contains(name, "<") && strings.HasSuffix(name, ">") {
		idx := strings.Index(name, "<")
		prefix := name[:idx]
		op := name[idx+1 : len(name)-1]
		if !strings.HasSuffix(prefix, ":") {
			prefix += ":"
		}
		return prefix + op
	}
	return name
}

func extractRawOperatorName(name string) string {
	norm := normalizeOpName(name)
	norm = strings.TrimPrefix(norm, "infix:")
	norm = strings.TrimPrefix(norm, "prefix:")
	norm = strings.TrimPrefix(norm, "postfix:")
	return norm
}

func (in *Interp) evalGrammarParse(cg *gcre.Grammar, requireFull bool, argExprs []Expr, env *Env) (*Value, error) {
	inVal, err := in.evalExpr(argExprs[0], env)
	if err != nil {
		return nil, err
	}
	inputStr := inVal.String()
	ruleName := "TOP"
	if len(argExprs) >= 2 {
		rVal, err := in.evalExpr(argExprs[1], env)
		if err != nil {
			return nil, err
		}
		if rVal.Type == ValHash && rVal.HashVal != nil {
			if rv, ok := rVal.HashVal["rule"]; ok && rv != nil {
				ruleName = rv.String()
			}
		} else if s := rVal.String(); s != "" {
			ruleName = s
		}
	}

	m, parseErr := cg.ParseRule(ruleName, inputStr, nil)
	if parseErr != nil || m == nil || !m.Ok {
		env.Define("$/", NilValue())
		in.GlobalEnv.Define("$/", NilValue())
		return NilValue(), nil
	}

	if requireFull {
		runes := []rune(inputStr)
		rem := strings.TrimSpace(string(runes[m.To:]))
		if rem != "" {
			env.Define("$/", NilValue())
			in.GlobalEnv.Define("$/", NilValue())
			return NilValue(), nil
		}
	}

	res := gcreMatchToValue(m)
	env.Define("$/", res)
	in.GlobalEnv.Define("$/", res)
	return res, nil
}

func gcreMatchToValue(m *gcre.Match) *Value {
	if m == nil || !m.Ok {
		return NilValue()
	}
	h := make(map[string]*Value)
	h["Str"] = StringValue(m.Str)
	h["str"] = StringValue(m.Str)
	h["from"] = IntValue(int64(m.From))
	h["to"] = IntValue(int64(m.To))
	h["ok"] = BoolValue(m.Ok)
	if m.Rule != "" {
		h["rule"] = StringValue(m.Rule)
	}
	posVals := make([]*Value, len(m.Pos))
	for i, pm := range m.Pos {
		pv := gcreMatchToValue(pm)
		posVals[i] = pv
		h[strconv.Itoa(i)] = pv
	}
	h["pos"] = ArrayValue(posVals)

	for k, list := range m.List {
		if len(list) == 1 {
			h[k] = gcreMatchToValue(list[0])
		} else if len(list) > 1 {
			elems := make([]*Value, len(list))
			for i, sm := range list {
				elems[i] = gcreMatchToValue(sm)
			}
			h[k] = ArrayValue(elems)
		}
	}
	return HashValue(h)
}
