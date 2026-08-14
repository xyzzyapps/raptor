package raptor

import (
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unsafe"
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

// Interp is the AST tree-walking evaluation interpreter for Raku5.
type Interp struct {
	GlobalEnv       *Env
	Stdout          io.Writer
	Stderr          io.Writer
	Builtins        map[string]BuiltinFunc
	Structs         map[string]*CStructDeclStmt
	BeforeHooks     map[string][]*AdviceHookStmt
	AfterHooks      map[string][]*AdviceHookStmt
	AroundHooks     map[string][]*AdviceHookStmt
	GatherStack     [][]*Value
	CustomInfixOps  map[string]*Value
	CustomPrefixOps map[string]*Value
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
	}
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
	return in
}



func (in *Interp) SetStdout(w io.Writer) {
	in.Stdout = w
}

func (in *Interp) SetStderr(w io.Writer) {
	in.Stderr = w
}

// Eval parses and evaluates a Raku5 source code string.
func (in *Interp) Eval(source string) (*Value, error) {
	lexer := NewLexer(source)
	var tokens []Token
	for {
		tok := lexer.NextToken()
		if tok.Type == TokError {
			return nil, fmt.Errorf("lex error at line %d, col %d: %s", tok.Line, tok.Col, tok.Literal)
		}
		tokens = append(tokens, tok)
		if tok.Type == TokEOF {
			break
		}
	}

	parser := NewParser(tokens)
	prog, err := parser.Parse()
	if err != nil {
		return nil, err
	}

	return in.EvalProgram(prog, in.GlobalEnv)
}



// EvalProgram evaluates all top-level statements.
func (in *Interp) EvalProgram(prog *Program, env *Env) (*Value, error) {
	var lastVal *Value = NilValue()
	for _, stmt := range prog.Stmts {
		val, err := in.evalStmt(stmt, env)
		if err != nil {
			if ret, ok := err.(*ReturnSignal); ok {
				return ret.Value, nil
			}
			return nil, err
		}
		if val != nil {
			lastVal = val
		}
	}

	// Auto-dispatch to Raku 'sub MAIN' or 'multi sub MAIN' if declared
	if mainVal, ok := in.GlobalEnv.Lookup("MAIN"); ok && (mainVal.Type == ValClosure || mainVal.Type == ValMultiSub) {
		var mainArgs []*Value
		if argsVal, ok := in.GlobalEnv.Lookup("@*ARGS"); ok && argsVal.Type == ValArray {
			mainArgs = argsVal.ArrayVal
		} else if argsVal, ok := in.GlobalEnv.Lookup("@ARGV"); ok && argsVal.Type == ValArray {
			mainArgs = argsVal.ArrayVal
		}
		res, err := in.InvokeCallable(mainVal, mainArgs)
		if err != nil {
			if ret, ok := err.(*ReturnSignal); ok {
				return ret.Value, nil
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

	case *VarDeclStmt:
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
				if initVal.Type != ValCStruct || initVal.CStructVal == nil || initVal.CStructVal.Class != structDecl {
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
		val, err := in.evalExpr(s.Value, env)
		if err != nil {
			return nil, err
		}
		return in.evalAssign(s.Target, s.Op, val, env)

	case *SubDeclStmt:
		normName := normalizeOpName(s.Name)
		closure := &Closure{
			Name:    normName,
			Params:  s.Params,
			Body:    s.Body,
			Env:     env,
			IsMulti: s.IsMulti,
		}
		if s.IsMulti {
			env.RegisterMulti(normName, closure)
			env.RegisterMulti(s.Name, closure)
			in.GlobalEnv.RegisterMulti(normName, closure)
			in.GlobalEnv.RegisterMulti(s.Name, closure)

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
			fArgs := []*Value{
				StringValue(libKey),
				StringValue(s.Symbol),
				StringValue(s.ReturnType),
				ArrayValue(callArgs),
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
		condVal, err := in.evalExpr(s.Condition, env)
		if err != nil {
			return nil, err
		}
		if !condVal.IsTrue() {
			msg := "assertion failed"
			if s.Message != nil {
				mVal, err := in.evalExpr(s.Message, env)
				if err == nil && mVal != nil {
					msg = mVal.String()
				}
			}
			return nil, fmt.Errorf("AssertionError: %s", msg)
		}
		return BoolValue(true), nil

	case *AdviceHookStmt:
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
		condVal, err := in.evalExpr(s.Condition, env)
		if err != nil {
			return nil, err
		}
		if condVal.IsTrue() {
			return in.evalBlock(s.ThenBranch, env.NewChild())
		}
		for i, eCond := range s.ElsifConds {
			ecVal, err := in.evalExpr(eCond, env)
			if err != nil {
				return nil, err
			}
			if ecVal.IsTrue() {
				return in.evalBlock(s.ElsifThen[i], env.NewChild())
			}
		}
		if s.ElseBranch != nil {
			return in.evalBlock(s.ElseBranch, env.NewChild())
		}
		return NilValue(), nil

	case *UnlessStmt:
		condVal, err := in.evalExpr(s.Condition, env)
		if err != nil {
			return nil, err
		}
		if !condVal.IsTrue() {
			return in.evalBlock(s.Body, env.NewChild())
		}
		return NilValue(), nil

	case *WhileStmt:
		var lastVal *Value = NilValue()
		childEnv := env.NewChild()
		for {
			cVal, err := in.evalExpr(s.Condition, env)
			if err != nil {
				return nil, err
			}
			condIsTrue := cVal.IsTrue()
			if s.IsUntil {
				condIsTrue = !condIsTrue
			}
			if !condIsTrue {
				break
			}
			bVal, err := in.evalBlock(s.Body, childEnv)
			if err != nil {
				if _, ok := err.(*BreakSignal); ok {
					break
				}
				if _, ok := err.(*ContinueSignal); ok {
					continue
				}
				return nil, err
			}
			lastVal = bVal
		}
		return lastVal, nil

	case *ForStmt:
		iterVal, err := in.evalExpr(s.Iterable, env)
		if err != nil {
			return nil, err
		}
		var list []*Value
		if iterVal.Type == ValArray {
			list = iterVal.ArrayVal
		} else {
			list = []*Value{iterVal}
		}

		var lastVal *Value = NilValue()
		childEnv := env.NewChild()
		for _, item := range list {
			if s.VarName != "" {
				childEnv.Define(s.VarName, item)
			} else {
				childEnv.Define("$_", item)
			}
			bVal, err := in.evalBlock(s.Body, childEnv)
			if err != nil {
				if _, ok := err.(*BreakSignal); ok {
					break
				}
				if _, ok := err.(*ContinueSignal); ok {
					continue
				}
				return nil, err
			}
			lastVal = bVal
		}
		return lastVal, nil

	case *LoopStmt:
		loopEnv := env.NewChild()
		if s.Init != nil {
			if _, err := in.evalExpr(s.Init, loopEnv); err != nil {
				return nil, err
			}
		}
		var lastVal *Value = NilValue()
		bodyEnv := loopEnv.NewChild()
		for {
			if s.Cond != nil {
				cVal, err := in.evalExpr(s.Cond, loopEnv)
				if err != nil {
					return nil, err
				}
				if !cVal.IsTrue() {
					break
				}
			}
			bVal, err := in.evalBlock(s.Body, bodyEnv)
			if err != nil {
				if _, ok := err.(*BreakSignal); ok {
					break
				}
				if _, ok := err.(*ContinueSignal); ok {
					// step will still run
				} else {
					return nil, err
				}
			}
			lastVal = bVal
			if s.Step != nil {
				if _, err := in.evalExpr(s.Step, loopEnv); err != nil {
					return nil, err
				}
			}
		}
		return lastVal, nil

	case *ReturnStmt:
		var retVal *Value = NilValue()
		if s.Value != nil {
			v, err := in.evalExpr(s.Value, env)
			if err != nil {
				return nil, err
			}
			retVal = v
		}
		return nil, &ReturnSignal{Value: retVal}

	case *BlockStmt:
		return in.evalBlock(s, env.NewChild())

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


	case *TakeStmt:
		var val *Value = NilValue()
		if s.Value != nil {
			v, err := in.evalExpr(s.Value, env)
			if err != nil {
				return nil, err
			}
			val = v
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
	var lastVal *Value = NilValue()
	for _, stmt := range b.Stmts {
		val, err := in.evalStmt(stmt, env)
		if err != nil {
			return nil, err
		}
		if val != nil {
			lastVal = val
		}
	}
	return lastVal, nil
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
			if val.Type != ValCStruct || val.CStructVal == nil || val.CStructVal.Class != structDecl {
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

func (in *Interp) evalAssign(target Expr, op string, val *Value, env *Env) (*Value, error) {
	switch t := target.(type) {
	case *VarExpr:
		if op == "=" {
			if err := in.validateTypeAndWhere(t.Name, val, env); err != nil {
				return nil, err
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
		if arrVal.Type != ValArray {
			return nil, fmt.Errorf("cannot index non-array type %s", arrVal.TypeName())
		}
		idxVal, err := in.evalExpr(t.Index, env)
		if err != nil {
			return nil, err
		}
		idx := int(idxVal.IntVal)
		if idx < 0 {
			idx = len(arrVal.ArrayVal) + idx
		}
		if idx < 0 {
			return nil, fmt.Errorf("array index out of bounds: %d", idx)
		}
		for len(arrVal.ArrayVal) <= idx {
			arrVal.ArrayVal = append(arrVal.ArrayVal, NilValue())
		}
		arrVal.ArrayVal[idx] = val
		return val, nil

	case *HashAccessExpr:
		hashVal, err := in.evalExpr(t.Hash, env)
		if err != nil {
			return nil, err
		}
		if hashVal.Type != ValHash {
			return nil, fmt.Errorf("cannot access non-hash type %s", hashVal.TypeName())
		}
		keyVal, err := in.evalExpr(t.Key, env)
		if err != nil {
			return nil, err
		}
		k := keyVal.StrVal
		if hashVal.HashVal == nil {
			hashVal.HashVal = make(map[string]*Value)
		}
		hashVal.HashVal[k] = val
		return val, nil

	case *MethodCallExpr:
		invocantVal, err := in.evalExpr(t.Target, env)
		if err != nil {
			return nil, err
		}
		if invocantVal.Type == ValCStruct && invocantVal.CStructVal != nil && invocantVal.CStructVal.Class != nil {
			if idx, ok := invocantVal.CStructVal.Class.FieldIndex[t.Method]; ok {
				return in.writeCStructField(invocantVal.CStructVal, invocantVal.CStructVal.Class.Fields[idx], val)
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
		if e.Name == "Nil" {
			return NilValue(), nil
		}
		if e.Name == "True" || e.Name == "true" {
			return BoolValue(true), nil
		}
		if e.Name == "False" || e.Name == "false" {
			return BoolValue(false), nil
		}
		if val, ok := env.Lookup(e.Name); ok {
			return val, nil
		}
		if builtin, ok := in.Builtins[e.Name]; ok {
			return ClosureValue(&Closure{
				Name: e.Name,
				Body: &BlockStmt{},
				Env:  env,
			}), nil
			_ = builtin
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

	case *BinaryExpr:
		if e.Op == "//" {
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
		if rVal.Type == ValJunction && rVal.JunctionVal != nil {
			var results []*Value
			for _, elem := range rVal.JunctionVal.Values {
				subU := &UnaryExpr{Op: e.Op, Right: &LiteralExpr{Type: TokString, Value: ""}}
				_ = subU
				res, err := in.evalUnaryOp(e.Op, elem, env)
				if err != nil {
					return nil, err
				}
				results = append(results, res)
			}
			return JunctionValue(rVal.JunctionVal.Kind, results), nil
		}
		return in.evalUnaryOp(e.Op, rVal, env)


	case *ArrayLiteralExpr:
		var elems []*Value
		for _, elExpr := range e.Elements {
			v, err := in.evalExpr(elExpr, env)
			if err != nil {
				return nil, err
			}
			elems = append(elems, v)
		}
		return ArrayValue(elems), nil

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
			m[kVal.StrVal] = vVal
		}
		return HashValue(m), nil

	case *IndexExpr:
		arrVal, err := in.evalExpr(e.Array, env)
		if err != nil {
			return nil, err
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
		idx := int(idxVal.IntVal)
		if idx < 0 {
			idx = len(arrVal.ArrayVal) + idx
		}
		if idx < 0 || idx >= len(arrVal.ArrayVal) {
			return NilValue(), nil
		}
		return arrVal.ArrayVal[idx], nil


	case *HashAccessExpr:
		hVal, err := in.evalExpr(e.Hash, env)
		if err != nil {
			return nil, err
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
		if val, ok := hVal.HashVal[kVal.StrVal]; ok {
			return val, nil
		}

		return NilValue(), nil

	case *ClosureExpr:
		return ClosureValue(&Closure{
			Params: e.Params,
			Body:   e.Body,
			Env:    env,
		}), nil

	case *CallExpr:
		calleeVal, err := in.evalExpr(e.Callee, env)
		if err != nil {
			return nil, err
		}
		if varExpr, ok := e.Callee.(*VarExpr); ok {
			if builtin, hasBuiltin := in.Builtins[varExpr.Name]; hasBuiltin {
				var args []*Value
				for _, argExpr := range e.Args {
					aVal, err := in.evalExpr(argExpr, env)
					if err != nil {
						return nil, err
					}
					args = append(args, aVal)
				}
				return builtin(in, args)
			}
		}

		var args []*Value
		for _, argExpr := range e.Args {
			aVal, err := in.evalExpr(argExpr, env)
			if err != nil {
				return nil, err
			}
			args = append(args, aVal)
		}

		return in.InvokeCallable(calleeVal, args)

	case *MethodCallExpr:
		invocantVal, err := in.evalExpr(e.Target, env)
		if err != nil {
			return nil, err
		}

		if invocantVal.Type == ValCStruct && invocantVal.CStructVal != nil && invocantVal.CStructVal.Class != nil {
			if e.Method == "new" {
				buf := make([]byte, invocantVal.CStructVal.Class.TotalSize)
				ptr := uintptr(unsafe.Pointer(&buf[0]))
				return CStructValue(&CStructInstance{
					Class:    invocantVal.CStructVal.Class,
					Ptr:      ptr,
					Buffer:   buf,
					Closures: make(map[string]*Value),
				}), nil
			}
			if idx, ok := invocantVal.CStructVal.Class.FieldIndex[e.Method]; ok {
				f := invocantVal.CStructVal.Class.Fields[idx]
				fieldVal, _ := in.readCStructField(invocantVal.CStructVal, f)
				if fieldVal != nil && fieldVal.Type == ValClosure && len(e.Args) > 0 {
					var args []*Value
					for _, argExpr := range e.Args {
						aVal, err := in.evalExpr(argExpr, env)
						if err != nil {
							return nil, err
						}
						args = append(args, aVal)
					}
					return in.InvokeCallable(fieldVal, args)
				}
				if len(e.Args) == 0 {
					return fieldVal, nil
				} else if len(e.Args) == 1 {
					aVal, err := in.evalExpr(e.Args[0], env)
					if err != nil {
						return nil, err
					}
					return in.writeCStructField(invocantVal.CStructVal, f, aVal)
				}
			}
		}

		if invocantVal.Type == ValPromise && invocantVal.PromiseVal != nil {
			switch e.Method {
			case "result", "await":
				return invocantVal.PromiseVal.Await()
			case "status":
				return StringValue(invocantVal.PromiseVal.Status), nil
			case "then":
				if len(e.Args) > 0 {
					cbVal, err := in.evalExpr(e.Args[0], env)
					if err != nil {
						return nil, err
					}
					nextP := NewPromise()
					go func() {
						res, err := invocantVal.PromiseVal.Await()
						if err != nil {
							nextP.Break(err)
						} else {
							nextRes, nextErr := in.InvokeCallable(cbVal, []*Value{res})
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

		if invocantVal.Type == ValChannel && invocantVal.ChannelVal != nil {
			switch e.Method {
			case "send":
				if len(e.Args) > 0 {
					sVal, err := in.evalExpr(e.Args[0], env)
					if err != nil {
						return nil, err
					}
					err = invocantVal.ChannelVal.Send(sVal)
					if err != nil {
						return nil, err
					}
					return sVal, nil
				}
			case "receive", "recv", "get":
				return invocantVal.ChannelVal.Receive()
			case "poll":
				return invocantVal.ChannelVal.Poll(), nil
			case "close":
				invocantVal.ChannelVal.Close()
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


		var args []*Value
		// UFCS places invocant as first argument: func(invocant, args...)
		args = append(args, invocantVal)
		for _, argExpr := range e.Args {
			aVal, err := in.evalExpr(argExpr, env)
			if err != nil {
				return nil, err
			}
			args = append(args, aVal)
		}

		// 1. Check built-ins (e.g. .uc, .lc, .elems, .substr, .split, .join, .push, .pop)
		if builtin, ok := in.Builtins[e.Method]; ok {
			return builtin(in, args)
		}

		// 2. Check lexical environment for sub or multi sub
		if calleeVal, ok := env.Lookup(e.Method); ok {
			return in.InvokeCallable(calleeVal, args)
		}
		if calleeVal, ok := env.Lookup("&" + e.Method); ok {
			return in.InvokeCallable(calleeVal, args)
		}

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
		// Check if topic is contained in the array
		for _, elem := range matcher.ArrayVal {
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
		if matcher.JunctionVal == nil {
			return false
		}
		switch matcher.JunctionVal.Kind {
		case JunctionAll:
			for _, elem := range matcher.JunctionVal.Values {
				if !in.smartMatch(topic, elem) {
					return false
				}
			}
			return len(matcher.JunctionVal.Values) > 0
		case JunctionAny:
			for _, elem := range matcher.JunctionVal.Values {
				if in.smartMatch(topic, elem) {
					return true
				}
			}
			return false
		case JunctionOne:
			count := 0
			for _, elem := range matcher.JunctionVal.Values {
				if in.smartMatch(topic, elem) {
					count++
				}
			}
			return count == 1
		case JunctionNone:
			for _, elem := range matcher.JunctionVal.Values {
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


// InvokeCallable resolves candidate dispatch and invokes the closure with advice hooks.
func (in *Interp) InvokeCallable(callee *Value, args []*Value) (*Value, error) {
	if callee == nil {
		return nil, fmt.Errorf("cannot invoke nil callable")
	}

	var closure *Closure
	var subName string

	if callee.Type == ValMultiSub {
		cand, err := in.resolveMultiCandidate(callee.Candidates, args)
		if err != nil {
			return nil, err
		}
		closure = cand
		subName = cand.Name
	} else if callee.Type == ValClosure {
		closure = callee.ClosureVal
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
					if ret, ok := err.(*ReturnSignal); ok {
						val = ret.Value
					} else {
						return nil, err
					}
				}
				return val, nil
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
			if argVal.Type == ValCStruct && argVal.CStructVal != nil && argVal.CStructVal.Class != nil && argVal.CStructVal.Class.Name == p.Type {
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
				elements = argVal.ArrayVal
			} else if argVal.LazySeqVal != nil {
				elements = argVal.LazySeqVal.Items
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


	val, err := in.evalBlock(closure.Body, callEnv)
	if err != nil {
		if ret, ok := err.(*ReturnSignal); ok {
			val = ret.Value
		} else {
			return nil, err
		}
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


	return val, nil
}

func (in *Interp) resolveMultiCandidate(candidates []*Closure, args []*Value) (*Closure, error) {
	var matching []*Closure
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
				if args[i].Type == ValCStruct && args[i].CStructVal != nil && args[i].CStructVal.Class != nil && args[i].CStructVal.Class.Name == p.Type {
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
			matching = append(matching, cand)
			if score > bestScore {
				bestScore = score
				bestCand = cand
			}
		}
	}

	if len(matching) == 0 || bestCand == nil {
		return nil, fmt.Errorf("no matching multi candidate found for arguments")
	}
	return bestCand, nil
}


func (in *Interp) evalBinaryOp(left *Value, op string, right *Value) (*Value, error) {
	// Junction autothreading
	if left.Type == ValJunction && left.JunctionVal != nil {
		var results []*Value
		for _, elem := range left.JunctionVal.Values {
			res, err := in.evalBinaryOp(elem, op, right)
			if err != nil {
				return nil, err
			}
			results = append(results, res)
		}
		return JunctionValue(left.JunctionVal.Kind, results), nil
	}
	if right.Type == ValJunction && right.JunctionVal != nil {
		var results []*Value
		for _, elem := range right.JunctionVal.Values {
			res, err := in.evalBinaryOp(left, op, elem)
			if err != nil {
				return nil, err
			}
			results = append(results, res)
		}
		return JunctionValue(right.JunctionVal.Kind, results), nil
	}

	// 1. Check for custom operator overloading (e.g. infix:+ or infix:<+>)
	if len(in.CustomInfixOps) > 0 {
		if fn, ok := in.CustomInfixOps[op]; ok {
			if fn.Type == ValMultiSub {
				if cand, err := in.resolveMultiCandidate(fn.Candidates, []*Value{left, right}); err == nil && cand != nil {
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
			for _, item := range right.ArrayVal {
				if in.smartMatch(left, item) {
					return BoolValue(true), nil
				}
			}
			return BoolValue(false), nil
		}
	case "∉": // is not element of
		if right.Type == ValArray {
			for _, item := range right.ArrayVal {
				if in.smartMatch(left, item) {
					return BoolValue(false), nil
				}
			}
			return BoolValue(true), nil
		}
	case "∩": // set intersection
		if left.Type == ValArray && right.Type == ValArray {
			var inter []*Value
			for _, a := range left.ArrayVal {
				for _, b := range right.ArrayVal {
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
			for _, a := range left.ArrayVal {
				s := a.String()
				if _, ok := unionMap[s]; !ok {
					unionMap[s] = a
					unionList = append(unionList, a)
				}
			}
			for _, b := range right.ArrayVal {
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
	case "+":
		if left.Type == ValFloat || right.Type == ValFloat {
			return FloatValue(in.toFloat(left) + in.toFloat(right)), nil
		}
		return IntValue(in.toInt(left) + in.toInt(right)), nil
	case "-":
		if left.Type == ValFloat || right.Type == ValFloat {
			return FloatValue(in.toFloat(left) - in.toFloat(right)), nil
		}
		return IntValue(in.toInt(left) - in.toInt(right)), nil
	case "*":
		if left.Type == ValFloat || right.Type == ValFloat {
			return FloatValue(in.toFloat(left) * in.toFloat(right)), nil
		}
		return IntValue(in.toInt(left) * in.toInt(right)), nil
	case "/":
		rF := in.toFloat(right)
		if rF == 0.0 {
			return nil, fmt.Errorf("division by zero")
		}
		return FloatValue(in.toFloat(left) / rF), nil
	case "%":
		rI := in.toInt(right)
		if rI == 0 {
			return nil, fmt.Errorf("modulo by zero")
		}
		return IntValue(in.toInt(left) % rI), nil
	case "**":
		return FloatValue(math.Pow(in.toFloat(left), in.toFloat(right))), nil

	case "//":
		if left.Type != ValNil {
			return left, nil
		}
		return right, nil

	case "+&":
		return IntValue(in.toInt(left) & in.toInt(right)), nil
	case "+|":
		return IntValue(in.toInt(left) | in.toInt(right)), nil
	case "+^":
		return IntValue(in.toInt(left) ^ in.toInt(right)), nil
	case "+<":
		return IntValue(in.toInt(left) << in.toInt(right)), nil
	case "+>":
		return IntValue(in.toInt(left) >> in.toInt(right)), nil

	case "div":
		rI := in.toInt(right)
		if rI == 0 {
			return nil, fmt.Errorf("division by zero in div operator")
		}
		return IntValue(in.toInt(left) / rI), nil

	case "mod":
		rI := in.toInt(right)
		if rI == 0 {
			return nil, fmt.Errorf("modulo by zero in mod operator")
		}
		return IntValue(in.toInt(left) % rI), nil

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

	case "=~":
		pattern := right.String()
		matched, err := regexp.MatchString(pattern, left.String())
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
		}
		return BoolValue(matched), nil

	case "!~":
		pattern := right.String()
		matched, err := regexp.MatchString(pattern, left.String())
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
			for c := 0; c < count; c++ {
				list = append(list, left.ArrayVal...)
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
			return ArrayValue(nil), nil
		}
		elems := make([]*Value, 0, end-start+1)
		for i := start; i <= end; i++ {
			elems = append(elems, IntValue(i))
		}
		return ArrayValue(elems), nil

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
				if cand, err := in.resolveMultiCandidate(fn.Candidates, []*Value{rVal}); err == nil && cand != nil {
					return in.InvokeCallable(fn, []*Value{rVal})
				}
			} else {
				return in.InvokeCallable(fn, []*Value{rVal})
			}
		}
	}

	switch op {
	case "!":
		return BoolValue(!rVal.IsTrue()), nil
	case "-":
		if rVal.Type == ValFloat {
			return FloatValue(-rVal.FloatVal), nil
		}
		return IntValue(-rVal.IntVal), nil
	case "+":
		return rVal, nil

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


func (in *Interp) interpolateString(template string, env *Env) string {
	var sb strings.Builder
	runes := []rune(template)
	n := len(runes)
	i := 0
	for i < n {
		if (runes[i] == '$' || runes[i] == '@' || runes[i] == '%') && i+1 < n && (runes[i+1] == '_' || runes[i+1] == '*' || (runes[i+1] >= 'a' && runes[i+1] <= 'z') || (runes[i+1] >= 'A' && runes[i+1] <= 'Z')) {
			sigil := runes[i]
			i++
			var varName strings.Builder
			varName.WriteRune(sigil)
			for i < n && (runes[i] == '_' || runes[i] == '*' || (runes[i] >= 'a' && runes[i] <= 'z') || (runes[i] >= 'A' && runes[i] <= 'Z') || (runes[i] >= '0' && runes[i] <= '9')) {
				varName.WriteRune(runes[i])
				i++
			}
			if val, ok := env.Lookup(varName.String()); ok {
				sb.WriteString(val.String())
			}
			continue
		}
		sb.WriteRune(runes[i])
		i++
	}
	return sb.String()
}

func (in *Interp) readCStructField(inst *CStructInstance, f CStructField) (*Value, error) {
	if inst.Closures != nil {
		if cVal, ok := inst.Closures[f.Name]; ok {
			return cVal, nil
		}
	}
	if inst.Ptr == 0 {
		return NilValue(), nil
	}
	ptr := unsafe.Pointer(inst.Ptr + uintptr(f.Offset))
	switch f.Type {
	case "int8", "char":
		return IntValue(int64(*(*int8)(ptr))), nil
	case "uint8", "byte":
		return IntValue(int64(*(*uint8)(ptr))), nil
	case "int16", "short":
		return IntValue(int64(*(*int16)(ptr))), nil
	case "uint16", "WORD":
		return IntValue(int64(*(*uint16)(ptr))), nil
	case "int32", "int", "long":
		return IntValue(int64(*(*int32)(ptr))), nil
	case "uint32", "uint", "DWORD":
		return IntValue(int64(*(*uint32)(ptr))), nil
	case "int64", "Int":
		return IntValue(*(*int64)(ptr)), nil
	case "uint64":
		return IntValue(int64(*(*uint64)(ptr))), nil
	case "num32", "float32":
		return FloatValue(float64(*(*float32)(ptr))), nil
	case "num64", "float64", "Num", "double":
		return FloatValue(*(*float64)(ptr)), nil
	case "ptr", "pointer", "Pointer", "OpaquePointer", "Callable", "closure":
		return NativePtrValue(*(*uintptr)(ptr)), nil
	default:
		return IntValue(*(*int64)(ptr)), nil
	}
}

func (in *Interp) writeCStructField(inst *CStructInstance, f CStructField, val *Value) (*Value, error) {
	if val.Type == ValClosure {
		if inst.Closures == nil {
			inst.Closures = make(map[string]*Value)
		}
		inst.Closures[f.Name] = val
	}
	if inst.Ptr == 0 {
		return val, nil
	}
	ptr := unsafe.Pointer(inst.Ptr + uintptr(f.Offset))
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

