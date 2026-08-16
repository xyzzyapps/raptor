package raptor

import (
	"context"
	"fmt"
	"math"
	"moarvm-go/engine"
	"strconv"
)

const moarMaxLocals = 256

// Compiler emits MoarVM CompUnit v7 bytecode for a Raptor program.
// It does not fall back to the Go interpreter. Unsupported AST is an error.
type Compiler struct {
	cu      *moargo.CompUnitEmitter
	frame   *moargo.FrameEmitter
	regMap  map[string]uint16
	defMap  map[string]uint16
	nextReg uint16
	loops   []loopCtx
	tapN    uint16
	tapInit bool
}

type loopCtx struct {
	breaks []int32
	conts  []int32
	contAt int32
}

type mval struct {
	reg uint16
	def uint16
	typ uint16
}

// NewCompiler creates a new Raptor → MoarVM compiler.
func NewCompiler() *Compiler {
	cu := moargo.NewCompUnitEmitter("raptor")
	f := cu.NewFrame("raptor_mainline", moarMaxLocals)
	for i := 0; i < moarMaxLocals; i++ {
		f.SetLocalType(i, moargo.RegInt64)
	}
	return &Compiler{
		cu:      cu,
		frame:   f,
		regMap:  make(map[string]uint16),
		defMap:  make(map[string]uint16),
		nextReg: 0,
	}
}

func (c *Compiler) allocReg(name string) (uint16, error) {
	if reg, ok := c.regMap[name]; ok {
		return reg, nil
	}
	reg, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return 0, err
	}
	def, err := c.constI(0)
	if err != nil {
		return 0, err
	}
	c.regMap[name] = reg
	c.defMap[name] = def
	return reg, nil
}

func (c *Compiler) tempKind(kind uint16) (uint16, error) {
	if int(c.nextReg) >= moarMaxLocals {
		return 0, fmt.Errorf("moar: too many locals (max %d)", moarMaxLocals)
	}
	reg := c.nextReg
	c.nextReg++
	c.frame.SetLocalType(int(reg), kind)
	return reg, nil
}

func (c *Compiler) tempReg() (uint16, error) {
	return c.tempKind(moargo.RegInt64)
}

func (c *Compiler) kindOf(reg uint16) uint16 {
	if int(reg) < len(c.frame.LocalTypes) {
		return c.frame.LocalTypes[reg]
	}
	return moargo.RegInt64
}

func (c *Compiler) patchU32(offset int32, val uint32) {
	b := c.frame.Bytecode.Bytes()
	b[offset] = byte(val)
	b[offset+1] = byte(val >> 8)
	b[offset+2] = byte(val >> 16)
	b[offset+3] = byte(val >> 24)
}

func (c *Compiler) emitGoto() int32 {
	c.frame.EmitOp(moargo.OpGoto)
	off := c.frame.CurrentOffset()
	c.frame.EmitInt32(0)
	return off
}

func (c *Compiler) emitUnless(reg uint16) int32 {
	c.frame.EmitOp(moargo.OpUnlessI)
	c.frame.EmitReg(reg)
	off := c.frame.CurrentOffset()
	c.frame.EmitInt32(0)
	return off
}

func (c *Compiler) emitIf(reg uint16) int32 {
	c.frame.EmitOp(moargo.OpIfI)
	c.frame.EmitReg(reg)
	off := c.frame.CurrentOffset()
	c.frame.EmitInt32(0)
	return off
}

func (c *Compiler) constI(n int64) (uint16, error) {
	r, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpConstI64)
	c.frame.EmitReg(r)
	c.frame.EmitInt64(n)
	return r, nil
}

func (c *Compiler) constS(s string) (uint16, error) {
	r, err := c.tempKind(moargo.RegStr)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpConstS)
	c.frame.EmitReg(r)
	c.frame.EmitString(s)
	return r, nil
}

func (c *Compiler) setReg(dst, src uint16) {
	c.frame.EmitOp(moargo.OpSet)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(src)
}

// emitMove copies src into dst, coercing when the static register types differ.
func (c *Compiler) emitMove(dst, src uint16) error {
	dt, st := c.kindOf(dst), c.kindOf(src)
	if dt == st {
		c.setReg(dst, src)
		return nil
	}
	if dt == moargo.RegStr && st == moargo.RegInt64 {
		c.frame.EmitOp(moargo.OpCoerceIS)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(src)
		return nil
	}
	if dt == moargo.RegStr && st == moargo.RegNum64 {
		c.frame.EmitOp(moargo.OpCoerceNS)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(src)
		return nil
	}
	if dt == moargo.RegInt64 && st == moargo.RegStr {
		c.frame.EmitOp(moargo.OpCoerceSI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(src)
		return nil
	}
	return fmt.Errorf("moar: cannot move reg type %d into %d", st, dt)
}

func (c *Compiler) bindVar(name string, v mval) error {
	reg, err := c.allocReg(name)
	if err != nil {
		return err
	}
	// First bind may retarget an unused int slot to the value's type.
	if c.kindOf(reg) != v.typ {
		c.frame.SetLocalType(int(reg), v.typ)
	}
	if err := c.emitMove(reg, v.reg); err != nil {
		return err
	}
	return c.emitMove(c.defMap[name], v.def)
}

func (c *Compiler) coerceS(v mval) (uint16, error) {
	switch v.typ {
	case moargo.RegStr:
		return v.reg, nil
	case moargo.RegNum64:
		r, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return 0, err
		}
		c.frame.EmitOp(moargo.OpCoerceNS)
		c.frame.EmitReg(r)
		c.frame.EmitReg(v.reg)
		return r, nil
	default:
		r, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return 0, err
		}
		c.frame.EmitOp(moargo.OpCoerceIS)
		c.frame.EmitReg(r)
		c.frame.EmitReg(v.reg)
		return r, nil
	}
}

func (c *Compiler) emitSayReg(reg uint16) error {
	s, err := c.coerceS(mval{reg: reg, typ: c.kindOf(reg)})
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpSay)
	c.frame.EmitReg(s)
	return nil
}

func (c *Compiler) emitPrintReg(reg uint16) error {
	s, err := c.coerceS(mval{reg: reg, typ: c.kindOf(reg)})
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpPrint)
	c.frame.EmitReg(s)
	return nil
}

func (c *Compiler) concat(a, b uint16) (uint16, error) {
	as, err := c.coerceS(mval{reg: a, typ: c.kindOf(a)})
	if err != nil {
		return 0, err
	}
	bs, err := c.coerceS(mval{reg: b, typ: c.kindOf(b)})
	if err != nil {
		return 0, err
	}
	dst, err := c.tempKind(moargo.RegStr)
	if err != nil {
		return 0, err
	}
	c.frame.EmitOp(moargo.OpConcatS)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(as)
	c.frame.EmitReg(bs)
	return dst, nil
}

func (c *Compiler) ensureTAP() error {
	if c.tapInit {
		return nil
	}
	n, err := c.constI(0)
	if err != nil {
		return err
	}
	c.tapN = n
	c.tapInit = true
	return nil
}

func (c *Compiler) incTAP() error {
	if err := c.ensureTAP(); err != nil {
		return err
	}
	one, err := c.constI(1)
	if err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(c.tapN)
	c.frame.EmitReg(c.tapN)
	c.frame.EmitReg(one)
	return nil
}

func unsupported(node any) error {
	return fmt.Errorf("moar: unsupported construct %T", node)
}

// CompileAST compiles statements into a MoarVM compilation unit.
func (c *Compiler) CompileAST(stmts []Stmt) ([]byte, error) {
	for _, stmt := range stmts {
		if err := c.compileStmt(stmt); err != nil {
			return nil, err
		}
	}
	c.frame.EmitOp(moargo.OpReturn)
	return c.cu.Emit()
}

// CompileScript parses and compiles Raptor source to MoarVM bytecode.
func (c *Compiler) CompileScript(source string) ([]byte, error) {
	prog, err := ParseProgram(source)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}
	return c.CompileAST(prog.Stmts)
}

func (c *Compiler) compileStmt(stmt Stmt) error {
	switch s := stmt.(type) {
	case *VarDeclStmt:
		if s.Where != nil {
			return unsupported(s.Where)
		}
		if _, err := c.allocReg(s.Name); err != nil {
			return err
		}
		if s.Value == nil {
			return nil
		}
		v, err := c.compileVal(s.Value)
		if err != nil {
			return err
		}
		return c.bindVar(s.Name, v)

	case *AssignStmt:
		return c.compileAssign(s)

	case *IfStmt:
		return c.compileIf(s)

	case *UnlessStmt:
		cond, err := c.compileVal(s.Condition)
		if err != nil {
			return err
		}
		skip := c.emitIf(cond.reg)
		if err := c.compileStmt(s.Body); err != nil {
			return err
		}
		c.patchU32(skip, uint32(c.frame.CurrentOffset()))
		return nil

	case *WhileStmt:
		return c.compileWhile(s)

	case *LoopStmt:
		return c.compileLoop(s)

	case *ModifierStmt:
		return c.compileModifier(s)

	case *BlockStmt:
		for _, bs := range s.Stmts {
			if err := c.compileStmt(bs); err != nil {
				return err
			}
		}
		return nil

	case *LabelStmt:
		if s.Stmt != nil {
			return c.compileStmt(s.Stmt)
		}
		return nil

	case *BreakStmt:
		if len(c.loops) == 0 {
			return fmt.Errorf("moar: last outside loop")
		}
		off := c.emitGoto()
		i := len(c.loops) - 1
		c.loops[i].breaks = append(c.loops[i].breaks, off)
		return nil

	case *ContinueStmt:
		if len(c.loops) == 0 {
			return fmt.Errorf("moar: next outside loop")
		}
		off := c.emitGoto()
		i := len(c.loops) - 1
		c.loops[i].conts = append(c.loops[i].conts, off)
		return nil

	case *ReturnStmt:
		if s.Value == nil {
			c.frame.EmitOp(moargo.OpReturn)
			return nil
		}
		v, err := c.compileVal(s.Value)
		if err != nil {
			return err
		}
		switch v.typ {
		case moargo.RegStr:
			c.frame.EmitOp(moargo.OpReturnS)
		case moargo.RegNum64:
			c.frame.EmitOp(moargo.OpReturnN)
		default:
			c.frame.EmitOp(moargo.OpReturnI)
		}
		c.frame.EmitReg(v.reg)
		return nil

	case *ExprStmt:
		return c.compileExprStmt(s)

	case *AssertStmt:
		cond, err := c.compileVal(s.Condition)
		if err != nil {
			return err
		}
		okJ := c.emitIf(cond.reg)
		msg := "assert failed"
		if s.Message != nil {
			mv, err := c.compileVal(s.Message)
			if err != nil {
				return err
			}
			if err := c.emitSayReg(mv.reg); err != nil {
				return err
			}
		} else {
			r, err := c.constS(msg)
			if err != nil {
				return err
			}
			if err := c.emitSayReg(r); err != nil {
				return err
			}
		}
		c.patchU32(okJ, uint32(c.frame.CurrentOffset()))
		return nil

	default:
		return unsupported(stmt)
	}
}

func (c *Compiler) compileAssign(s *AssignStmt) error {
	ve, ok := s.Target.(*VarExpr)
	if !ok {
		return unsupported(s.Target)
	}
	reg, err := c.allocReg(ve.Name)
	if err != nil {
		return err
	}
	rhs, err := c.compileVal(s.Value)
	if err != nil {
		return err
	}
	switch s.Op {
	case "=":
		return c.bindVar(ve.Name, rhs)
	case "+=":
		c.frame.EmitOp(moargo.OpAddI)
		c.frame.EmitReg(reg)
		c.frame.EmitReg(reg)
		c.frame.EmitReg(rhs.reg)
		one, err := c.constI(1)
		if err != nil {
			return err
		}
		c.setReg(c.defMap[ve.Name], one)
	case "-=":
		c.frame.EmitOp(moargo.OpSubI)
		c.frame.EmitReg(reg)
		c.frame.EmitReg(reg)
		c.frame.EmitReg(rhs.reg)
	case "*=":
		c.frame.EmitOp(moargo.OpMulI)
		c.frame.EmitReg(reg)
		c.frame.EmitReg(reg)
		c.frame.EmitReg(rhs.reg)
	case "/=":
		c.frame.EmitOp(moargo.OpDivI)
		c.frame.EmitReg(reg)
		c.frame.EmitReg(reg)
		c.frame.EmitReg(rhs.reg)
	case "~=":
		cat, err := c.concat(reg, rhs.reg)
		if err != nil {
			return err
		}
		c.setReg(reg, cat)
	case "//=":
		useR := c.emitUnless(c.defMap[ve.Name])
		end := c.emitGoto()
		c.patchU32(useR, uint32(c.frame.CurrentOffset()))
		if err := c.bindVar(ve.Name, rhs); err != nil {
			return err
		}
		c.patchU32(end, uint32(c.frame.CurrentOffset()))
	default:
		return fmt.Errorf("moar: unsupported assign op %q", s.Op)
	}
	return nil
}

func (c *Compiler) compileIf(s *IfStmt) error {
	var endPatches []int32
	cond, err := c.compileVal(s.Condition)
	if err != nil {
		return err
	}
	skip := c.emitUnless(cond.reg)
	if err := c.compileStmt(s.ThenBranch); err != nil {
		return err
	}
	endPatches = append(endPatches, c.emitGoto())
	c.patchU32(skip, uint32(c.frame.CurrentOffset()))
	for i, ec := range s.ElsifConds {
		econd, err := c.compileVal(ec)
		if err != nil {
			return err
		}
		es := c.emitUnless(econd.reg)
		if err := c.compileStmt(s.ElsifThen[i]); err != nil {
			return err
		}
		endPatches = append(endPatches, c.emitGoto())
		c.patchU32(es, uint32(c.frame.CurrentOffset()))
	}
	if s.ElseBranch != nil {
		if err := c.compileStmt(s.ElseBranch); err != nil {
			return err
		}
	}
	end := uint32(c.frame.CurrentOffset())
	for _, p := range endPatches {
		c.patchU32(p, end)
	}
	return nil
}

func (c *Compiler) pushLoop() {
	c.loops = append(c.loops, loopCtx{})
}

func (c *Compiler) popLoop(contAt, end int32) {
	i := len(c.loops) - 1
	lc := c.loops[i]
	c.loops = c.loops[:i]
	for _, p := range lc.breaks {
		c.patchU32(p, uint32(end))
	}
	for _, p := range lc.conts {
		c.patchU32(p, uint32(contAt))
	}
}

func (c *Compiler) compileWhile(s *WhileStmt) error {
	c.pushLoop()
	loop := c.frame.CurrentOffset()
	cond, err := c.compileVal(s.Condition)
	if err != nil {
		return err
	}
	var skip int32
	if s.IsUntil {
		skip = c.emitIf(cond.reg)
	} else {
		skip = c.emitUnless(cond.reg)
	}
	if err := c.compileStmt(s.Body); err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	end := c.frame.CurrentOffset()
	c.patchU32(skip, uint32(end))
	c.popLoop(loop, end)
	return nil
}

func (c *Compiler) compileLoop(s *LoopStmt) error {
	if s.Init != nil {
		if _, err := c.compileVal(s.Init); err != nil {
			return err
		}
	}
	c.pushLoop()
	loop := c.frame.CurrentOffset()
	var skip int32 = -1
	if s.Cond != nil {
		cond, err := c.compileVal(s.Cond)
		if err != nil {
			return err
		}
		skip = c.emitUnless(cond.reg)
	}
	if err := c.compileStmt(s.Body); err != nil {
		return err
	}
	cont := c.frame.CurrentOffset()
	if s.Step != nil {
		if _, err := c.compileVal(s.Step); err != nil {
			return err
		}
	}
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	end := c.frame.CurrentOffset()
	if skip >= 0 {
		c.patchU32(skip, uint32(end))
	}
	c.popLoop(cont, end)
	return nil
}

func (c *Compiler) compileModifier(s *ModifierStmt) error {
	switch s.Kind {
	case ModIf:
		cond, err := c.compileVal(s.Condition)
		if err != nil {
			return err
		}
		skip := c.emitUnless(cond.reg)
		if err := c.compileStmt(s.Target); err != nil {
			return err
		}
		c.patchU32(skip, uint32(c.frame.CurrentOffset()))
		return nil
	case ModUnless:
		cond, err := c.compileVal(s.Condition)
		if err != nil {
			return err
		}
		skip := c.emitIf(cond.reg)
		if err := c.compileStmt(s.Target); err != nil {
			return err
		}
		c.patchU32(skip, uint32(c.frame.CurrentOffset()))
		return nil
	case ModWhile:
		ws := &WhileStmt{Condition: s.Condition, Body: &BlockStmt{Stmts: []Stmt{s.Target}}}
		return c.compileWhile(ws)
	case ModUntil:
		ws := &WhileStmt{IsUntil: true, Condition: s.Condition, Body: &BlockStmt{Stmts: []Stmt{s.Target}}}
		return c.compileWhile(ws)
	default:
		return unsupported(s)
	}
}

func (c *Compiler) compileExprStmt(s *ExprStmt) error {
	if call, ok := s.Expr.(*CallExpr); ok {
		name := calleeName(call)
		switch name {
		case "say", "print":
			return c.compileSayPrint(name == "say", call.Args)
		case "plan", "ok", "is", "isnt", "done_testing":
			_, err := c.compileCall(call)
			return err
		}
	}
	_, err := c.compileVal(s.Expr)
	return err
}

func calleeName(call *CallExpr) string {
	if v, ok := call.Callee.(*VarExpr); ok {
		return v.Name
	}
	return ""
}

func (c *Compiler) compileSayPrint(nl bool, args []Expr) error {
	if len(args) == 0 {
		empty, err := c.constS("")
		if err != nil {
			return err
		}
		if nl {
			return c.emitSayReg(empty)
		}
		return c.emitPrintReg(empty)
	}
	var acc uint16
	for i, a := range args {
		v, err := c.compileVal(a)
		if err != nil {
			return err
		}
		if i == 0 {
			acc, err = c.coerceS(v)
			if err != nil {
				return err
			}
			continue
		}
		acc, err = c.concat(acc, v.reg)
		if err != nil {
			return err
		}
	}
	if nl {
		return c.emitSayReg(acc)
	}
	return c.emitPrintReg(acc)
}

func (c *Compiler) compileVal(expr Expr) (mval, error) {
	if expr == nil {
		return c.nilVal()
	}
	switch e := expr.(type) {
	case *LiteralExpr:
		return c.compileLit(e)
	case *VarExpr:
		return c.compileVar(e)
	case *UnaryExpr:
		return c.compileUnary(e)
	case *BinaryExpr:
		return c.compileBinary(e)
	case *TernaryExpr:
		return c.compileTernary(e)
	case *CallExpr:
		return c.compileCall(e)
	case *AssignStmt:
		if err := c.compileAssign(e); err != nil {
			return mval{}, err
		}
		if ve, ok := e.Target.(*VarExpr); ok {
			return mval{reg: c.regMap[ve.Name], def: c.defMap[ve.Name], typ: c.kindOf(c.regMap[ve.Name])}, nil
		}
		return mval{}, unsupported(e)
	case *InterpStringExpr:
		return c.compileInterp(e)
	case *ChainedCompExpr:
		return c.compileChained(e)
	default:
		return mval{}, unsupported(expr)
	}
}

func (c *Compiler) nilVal() (mval, error) {
	r, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	d, err := c.constI(0)
	if err != nil {
		return mval{}, err
	}
	return mval{reg: r, def: d, typ: moargo.RegInt64}, nil
}

func (c *Compiler) definedVal(reg uint16, typ uint16) (mval, error) {
	d, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	return mval{reg: reg, def: d, typ: typ}, nil
}

func (c *Compiler) compileLit(e *LiteralExpr) (mval, error) {
	switch e.Type {
	case TokInt:
		val, _ := strconv.ParseInt(fmt.Sprintf("%v", e.Value), 0, 64)
		if v, ok := e.Value.(int64); ok {
			val = v
		}
		r, err := c.constI(val)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(r, moargo.RegInt64)
	case TokFloat:
		val, _ := strconv.ParseFloat(fmt.Sprintf("%v", e.Value), 64)
		if v, ok := e.Value.(float64); ok {
			val = v
		}
		r, err := c.tempKind(moargo.RegNum64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpConstN64)
		c.frame.EmitReg(r)
		c.frame.EmitInt64(int64(math.Float64bits(val)))
		return c.definedVal(r, moargo.RegNum64)
	case TokString:
		r, err := c.constS(fmt.Sprintf("%v", e.Value))
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(r, moargo.RegStr)
	case TokIdent:
		ident, _ := e.Value.(string)
		return c.compileName(ident)
	default:
		return mval{}, fmt.Errorf("moar: unsupported literal %v", e.Type)
	}
}

func (c *Compiler) compileVar(e *VarExpr) (mval, error) {
	return c.compileName(e.Name)
}

func (c *Compiler) compileName(name string) (mval, error) {
	switch name {
	case "Nil":
		return c.nilVal()
	case "True", "true":
		r, err := c.constI(1)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(r, moargo.RegInt64)
	case "False", "false":
		r, err := c.constI(0)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(r, moargo.RegInt64)
	}
	reg, err := c.allocReg(name)
	if err != nil {
		return mval{}, err
	}
	return mval{reg: reg, def: c.defMap[name], typ: c.kindOf(reg)}, nil
}

func (c *Compiler) compileUnary(e *UnaryExpr) (mval, error) {
	v, err := c.compileVal(e.Right)
	if err != nil {
		return mval{}, err
	}
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	switch e.Op {
	case "-":
		c.frame.EmitOp(moargo.OpNegI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(v.reg)
	case "!", "not":
		c.frame.EmitOp(moargo.OpNotI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(v.reg)
	case "+":
		c.setReg(dst, v.reg)
	case "+^":
		c.frame.EmitOp(moargo.OpBnotI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(v.reg)
	default:
		return mval{}, fmt.Errorf("moar: unsupported unary %q", e.Op)
	}
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) compileBinary(e *BinaryExpr) (mval, error) {
	switch e.Op {
	case "&&", "and":
		return c.compileAnd(e)
	case "||", "or":
		return c.compileOr(e)
	case "//", "orelse":
		return c.compileDefinedOr(e)
	}

	l, err := c.compileVal(e.Left)
	if err != nil {
		return mval{}, err
	}
	r, err := c.compileVal(e.Right)
	if err != nil {
		return mval{}, err
	}

	if e.Op == "~" {
		cat, err := c.concat(l.reg, r.reg)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(cat, moargo.RegStr)
	}
	if e.Op == "x" {
		dst, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		ls, err := c.coerceS(l)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpRepeatS)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(ls)
		c.frame.EmitReg(r.reg)
		return c.definedVal(dst, moargo.RegStr)
	}
	if e.Op == "%%" {
		mod, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpModI)
		c.frame.EmitReg(mod)
		c.frame.EmitReg(l.reg)
		c.frame.EmitReg(r.reg)
		zero, err := c.constI(0)
		if err != nil {
			return mval{}, err
		}
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpEqI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(mod)
		c.frame.EmitReg(zero)
		return c.definedVal(dst, moargo.RegInt64)
	}
	if e.Op == "min" || e.Op == "max" {
		return c.compileMinMax(e.Op == "min", l, r)
	}

	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	op, strOp, ok := binOp(e.Op)
	if !ok {
		return mval{}, fmt.Errorf("moar: unsupported infix %q", e.Op)
	}
	if strOp {
		ls, err := c.coerceS(l)
		if err != nil {
			return mval{}, err
		}
		rs, err := c.coerceS(r)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(op)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(ls)
		c.frame.EmitReg(rs)
		return c.definedVal(dst, moargo.RegInt64)
	}
	c.frame.EmitOp(op)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(l.reg)
	c.frame.EmitReg(r.reg)
	return c.definedVal(dst, moargo.RegInt64)
}

func binOp(op string) (uint16, bool, bool) {
	switch op {
	case "+":
		return moargo.OpAddI, false, true
	case "-":
		return moargo.OpSubI, false, true
	case "*":
		return moargo.OpMulI, false, true
	case "/", "div":
		return moargo.OpDivI, false, true
	case "%", "mod":
		return moargo.OpModI, false, true
	case "**":
		return moargo.OpPowI, false, true
	case "==":
		return moargo.OpEqI, false, true
	case "!=":
		return moargo.OpNeI, false, true
	case "<":
		return moargo.OpLtI, false, true
	case "<=":
		return moargo.OpLeI, false, true
	case ">":
		return moargo.OpGtI, false, true
	case ">=":
		return moargo.OpGeI, false, true
	case "+&":
		return moargo.OpBandI, false, true
	case "+|":
		return moargo.OpBorI, false, true
	case "+^":
		return moargo.OpBxorI, false, true
	case "+<":
		return moargo.OpBlshiftI, false, true
	case "+>":
		return moargo.OpBrshiftI, false, true
	case "eq":
		return moargo.OpEqS, true, true
	case "ne":
		return moargo.OpNeS, true, true
	case "lt":
		return moargo.OpLtS, true, true
	case "gt":
		return moargo.OpGtS, true, true
	case "le":
		return moargo.OpLeS, true, true
	case "ge":
		return moargo.OpGeS, true, true
	}
	return 0, false, false
}

func (c *Compiler) compileMinMax(min bool, l, r mval) (mval, error) {
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	cmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	if min {
		c.frame.EmitOp(moargo.OpLtI)
	} else {
		c.frame.EmitOp(moargo.OpGtI)
	}
	c.frame.EmitReg(cmp)
	c.frame.EmitReg(l.reg)
	c.frame.EmitReg(r.reg)
	// cmp true → left is the min/max
	takeR := c.emitUnless(cmp)
	if err := c.emitMove(dst, l.reg); err != nil {
		return mval{}, err
	}
	end := c.emitGoto()
	c.patchU32(takeR, uint32(c.frame.CurrentOffset()))
	if err := c.emitMove(dst, r.reg); err != nil {
		return mval{}, err
	}
	c.patchU32(end, uint32(c.frame.CurrentOffset()))
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) compileAnd(e *BinaryExpr) (mval, error) {
	l, err := c.compileVal(e.Left)
	if err != nil {
		return mval{}, err
	}
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	if err := c.emitMove(dst, l.reg); err != nil {
		return mval{}, err
	}
	skip := c.emitUnless(l.reg)
	r, err := c.compileVal(e.Right)
	if err != nil {
		return mval{}, err
	}
	if err := c.emitMove(dst, r.reg); err != nil {
		return mval{}, err
	}
	c.patchU32(skip, uint32(c.frame.CurrentOffset()))
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) compileOr(e *BinaryExpr) (mval, error) {
	l, err := c.compileVal(e.Left)
	if err != nil {
		return mval{}, err
	}
	dst, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	if err := c.emitMove(dst, l.reg); err != nil {
		return mval{}, err
	}
	skip := c.emitIf(l.reg)
	r, err := c.compileVal(e.Right)
	if err != nil {
		return mval{}, err
	}
	if err := c.emitMove(dst, r.reg); err != nil {
		return mval{}, err
	}
	c.patchU32(skip, uint32(c.frame.CurrentOffset()))
	return c.definedVal(dst, moargo.RegInt64)
}

func (c *Compiler) compileDefinedOr(e *BinaryExpr) (mval, error) {
	l, err := c.compileVal(e.Left)
	if err != nil {
		return mval{}, err
	}
	// Result is always a string so both branches share one register type.
	dst, err := c.tempKind(moargo.RegStr)
	if err != nil {
		return mval{}, err
	}
	def, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	skip := c.emitIf(l.def)
	r, err := c.compileVal(e.Right)
	if err != nil {
		return mval{}, err
	}
	if err := c.emitMove(dst, r.reg); err != nil {
		return mval{}, err
	}
	c.setReg(def, r.def)
	end := c.emitGoto()
	c.patchU32(skip, uint32(c.frame.CurrentOffset()))
	if err := c.emitMove(dst, l.reg); err != nil {
		return mval{}, err
	}
	c.setReg(def, l.def)
	c.patchU32(end, uint32(c.frame.CurrentOffset()))
	return mval{reg: dst, def: def, typ: moargo.RegStr}, nil
}

func (c *Compiler) compileTernary(e *TernaryExpr) (mval, error) {
	cond, err := c.compileVal(e.Cond)
	if err != nil {
		return mval{}, err
	}
	// String dest: both branches can be int or str without a mixed set.
	dst, err := c.tempKind(moargo.RegStr)
	if err != nil {
		return mval{}, err
	}
	els := c.emitUnless(cond.reg)
	th, err := c.compileVal(e.Then)
	if err != nil {
		return mval{}, err
	}
	if err := c.emitMove(dst, th.reg); err != nil {
		return mval{}, err
	}
	end := c.emitGoto()
	c.patchU32(els, uint32(c.frame.CurrentOffset()))
	el, err := c.compileVal(e.Else)
	if err != nil {
		return mval{}, err
	}
	if err := c.emitMove(dst, el.reg); err != nil {
		return mval{}, err
	}
	c.patchU32(end, uint32(c.frame.CurrentOffset()))
	return c.definedVal(dst, moargo.RegStr)
}

func (c *Compiler) compileChained(e *ChainedCompExpr) (mval, error) {
	if len(e.Exprs) < 2 || len(e.Ops) != len(e.Exprs)-1 {
		return mval{}, fmt.Errorf("moar: bad chained comparison")
	}
	acc, err := c.constI(1)
	if err != nil {
		return mval{}, err
	}
	for i, op := range e.Ops {
		bin := &BinaryExpr{Left: e.Exprs[i], Op: op, Right: e.Exprs[i+1]}
		v, err := c.compileBinary(bin)
		if err != nil {
			return mval{}, err
		}
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		// acc = acc && v  (v is 0/1)
		c.frame.EmitOp(moargo.OpMulI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(acc)
		c.frame.EmitReg(v.reg)
		acc = dst
	}
	return c.definedVal(acc, moargo.RegInt64)
}

func (c *Compiler) compileInterp(e *InterpStringExpr) (mval, error) {
	if len(e.Parts) == 0 {
		r, err := c.constS("")
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(r, moargo.RegStr)
	}
	first, err := c.compileVal(e.Parts[0])
	if err != nil {
		return mval{}, err
	}
	acc, err := c.coerceS(first)
	if err != nil {
		return mval{}, err
	}
	for _, p := range e.Parts[1:] {
		v, err := c.compileVal(p)
		if err != nil {
			return mval{}, err
		}
		acc, err = c.concat(acc, v.reg)
		if err != nil {
			return mval{}, err
		}
	}
	return c.definedVal(acc, moargo.RegStr)
}

func (c *Compiler) compileCall(e *CallExpr) (mval, error) {
	name := calleeName(e)
	switch name {
	case "say":
		if err := c.compileSayPrint(true, e.Args); err != nil {
			return mval{}, err
		}
		return c.nilVal()
	case "print":
		if err := c.compileSayPrint(false, e.Args); err != nil {
			return mval{}, err
		}
		return c.nilVal()
	case "plan":
		return c.compilePlan(e.Args)
	case "ok":
		return c.compileOK(e.Args)
	case "is":
		return c.compileIs(e.Args, true)
	case "isnt":
		return c.compileIs(e.Args, false)
	case "done_testing":
		one, err := c.constI(1)
		if err != nil {
			return mval{}, err
		}
		return c.definedVal(one, moargo.RegInt64)
	case "abs":
		if len(e.Args) < 1 {
			return c.nilVal()
		}
		v, err := c.compileVal(e.Args[0])
		if err != nil {
			return mval{}, err
		}
		dst, err := c.tempKind(moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
		c.frame.EmitOp(moargo.OpAbsI)
		c.frame.EmitReg(dst)
		c.frame.EmitReg(v.reg)
		return c.definedVal(dst, moargo.RegInt64)
	case "chars", "length":
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
	case "uc", "lc":
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
		dst, err := c.tempKind(moargo.RegStr)
		if err != nil {
			return mval{}, err
		}
		if name == "uc" {
			c.frame.EmitOp(moargo.OpUC)
		} else {
			c.frame.EmitOp(moargo.OpLC)
		}
		c.frame.EmitReg(dst)
		c.frame.EmitReg(s)
		return c.definedVal(dst, moargo.RegStr)
	case "int", "Int":
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
		if v.typ == moargo.RegInt64 {
			return v, nil
		}
		if v.typ == moargo.RegStr {
			dst, err := c.tempKind(moargo.RegInt64)
			if err != nil {
				return mval{}, err
			}
			c.frame.EmitOp(moargo.OpCoerceSI)
			c.frame.EmitReg(dst)
			c.frame.EmitReg(v.reg)
			return c.definedVal(dst, moargo.RegInt64)
		}
		return v, nil
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
	default:
		return mval{}, fmt.Errorf("moar: unsupported call %s", name)
	}
}

func (c *Compiler) compilePlan(args []Expr) (mval, error) {
	n := int64(0)
	if len(args) > 0 {
		v, err := c.compileVal(args[0])
		if err != nil {
			return mval{}, err
		}
		pref, err := c.constS("1..")
		if err != nil {
			return mval{}, err
		}
		line, err := c.concat(pref, v.reg)
		if err != nil {
			return mval{}, err
		}
		if err := c.emitSayReg(line); err != nil {
			return mval{}, err
		}
		return c.definedVal(v.reg, moargo.RegInt64)
	}
	z, err := c.constI(n)
	if err != nil {
		return mval{}, err
	}
	return c.definedVal(z, moargo.RegInt64)
}

func (c *Compiler) tapLabel(args []Expr, idx int) (uint16, error) {
	if len(args) <= idx {
		return c.constS("")
	}
	v, err := c.compileVal(args[idx])
	if err != nil {
		return 0, err
	}
	dash, err := c.constS(" - ")
	if err != nil {
		return 0, err
	}
	return c.concat(dash, v.reg)
}

func (c *Compiler) compileOK(args []Expr) (mval, error) {
	if err := c.incTAP(); err != nil {
		return mval{}, err
	}
	cond := mval{}
	var err error
	if len(args) > 0 {
		cond, err = c.compileVal(args[0])
		if err != nil {
			return mval{}, err
		}
	} else {
		z, err := c.constI(0)
		if err != nil {
			return mval{}, err
		}
		cond, err = c.definedVal(z, moargo.RegInt64)
		if err != nil {
			return mval{}, err
		}
	}
	label, err := c.tapLabel(args, 1)
	if err != nil {
		return mval{}, err
	}
	return c.emitTAPLine(cond.reg, label)
}

func (c *Compiler) compileIs(args []Expr, wantEq bool) (mval, error) {
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
	cmp, err := c.tempKind(moargo.RegInt64)
	if err != nil {
		return mval{}, err
	}
	gs, err := c.coerceS(got)
	if err != nil {
		return mval{}, err
	}
	es, err := c.coerceS(exp)
	if err != nil {
		return mval{}, err
	}
	if wantEq {
		c.frame.EmitOp(moargo.OpEqS)
	} else {
		c.frame.EmitOp(moargo.OpNeS)
	}
	c.frame.EmitReg(cmp)
	c.frame.EmitReg(gs)
	c.frame.EmitReg(es)
	label, err := c.tapLabel(args, 2)
	if err != nil {
		return mval{}, err
	}
	return c.emitTAPLine(cmp, label)
}

func (c *Compiler) emitTAPLine(cond, label uint16) (mval, error) {
	okP, err := c.constS("ok ")
	if err != nil {
		return mval{}, err
	}
	notP, err := c.constS("not ok ")
	if err != nil {
		return mval{}, err
	}
	pref, err := c.tempKind(moargo.RegStr)
	if err != nil {
		return mval{}, err
	}
	els := c.emitUnless(cond)
	c.setReg(pref, okP)
	end := c.emitGoto()
	c.patchU32(els, uint32(c.frame.CurrentOffset()))
	c.setReg(pref, notP)
	c.patchU32(end, uint32(c.frame.CurrentOffset()))
	mid, err := c.concat(pref, c.tapN)
	if err != nil {
		return mval{}, err
	}
	line, err := c.concat(mid, label)
	if err != nil {
		return mval{}, err
	}
	if err := c.emitSayReg(line); err != nil {
		return mval{}, err
	}
	return c.definedVal(cond, moargo.RegInt64)
}

// CompileAndRun compiles a Raptor script to MoarVM bytecode and executes it.
func CompileAndRun(ctx context.Context, vm moargo.Engine, script string) error {
	compiler := NewCompiler()
	bc, err := compiler.CompileScript(script)
	if err != nil {
		return fmt.Errorf("raptor compilation failed: %w", err)
	}
	return vm.RunBytecode(ctx, bc)
}
