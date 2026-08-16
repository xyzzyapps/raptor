package raptor

import (
	"context"
	"encoding/binary"
	"fmt"
	"moarvm-go/engine"
	"strconv"
)


// Compiler compiles Raku5 AST statements into MoarVM bytecode.
type Compiler struct {
	cu      *moargo.CompUnitEmitter
	frame   *moargo.FrameEmitter
	regMap  map[string]uint16
	nextReg uint16
}

// NewCompiler creates a new Raku5 MoarVM bytecode compiler.
func NewCompiler() *Compiler {
	cu := moargo.NewCompUnitEmitter("raku5")
	f := cu.NewFrame("raku5_mainline", 64)
	for i := 0; i < 64; i++ {
		f.SetLocalType(i, moargo.RegInt64)
	}
	return &Compiler{
		cu:      cu,
		frame:   f,
		regMap:  make(map[string]uint16),
		nextReg: 0,
	}
}

func (c *Compiler) allocReg(name string) uint16 {
	if reg, ok := c.regMap[name]; ok {
		return reg
	}
	reg := c.nextReg
	c.nextReg++
	c.regMap[name] = reg
	c.frame.SetLocalType(int(reg), moargo.RegInt64)
	return reg
}

func (c *Compiler) tempReg() uint16 {
	reg := c.nextReg
	c.nextReg++
	c.frame.SetLocalType(int(reg), moargo.RegInt64)
	return reg
}

// CompileAST compiles a list of AST statements into a MoarVM binary compilation unit.
func (c *Compiler) CompileAST(stmts []Stmt) ([]byte, error) {
	for _, stmt := range stmts {
		if err := c.compileStmt(stmt); err != nil {
			return nil, err
		}
	}
	c.frame.EmitOp(moargo.OpReturn)
	return c.cu.Emit()
}

// CompileScript parses and compiles Raku5 source code into MoarVM bytecode.
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
		if s.Value != nil {
			reg := c.allocReg(s.Name)
			valReg, err := c.compileExpr(s.Value)
			if err != nil {
				return err
			}
			regType := c.frame.LocalTypes[valReg]
			c.frame.SetLocalType(int(reg), regType)
			c.frame.EmitOp(moargo.OpSet)
			c.frame.EmitReg(reg)
			c.frame.EmitReg(valReg)
		} else {
			c.allocReg(s.Name)
		}

	case *AssignStmt:
		valReg, err := c.compileExpr(s.Value)
		if err != nil {
			return err
		}
		if varExpr, ok := s.Target.(*VarExpr); ok {
			reg := c.allocReg(varExpr.Name)
			regType := c.frame.LocalTypes[valReg]
			c.frame.SetLocalType(int(reg), regType)
			switch s.Op {
			case "=":
				c.frame.EmitOp(moargo.OpSet)
				c.frame.EmitReg(reg)
				c.frame.EmitReg(valReg)
			case "+=":
				c.frame.EmitOp(moargo.OpAddI)
				c.frame.EmitReg(reg)
				c.frame.EmitReg(reg)
				c.frame.EmitReg(valReg)
			case "-=":
				c.frame.EmitOp(moargo.OpSubI)
				c.frame.EmitReg(reg)
				c.frame.EmitReg(reg)
				c.frame.EmitReg(valReg)
			}
		}

	case *IfStmt:
		condReg, err := c.compileExpr(s.Condition)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpUnlessI)
		c.frame.EmitReg(condReg)
		offsetPos := c.frame.Bytecode.Len()
		c.frame.EmitInt32(0)

		if err := c.compileStmt(s.ThenBranch); err != nil {
			return err
		}
		thenEndPos := int32(c.frame.Bytecode.Len())
		buf := c.frame.Bytecode.Bytes()
		binary.LittleEndian.PutUint32(buf[offsetPos:offsetPos+4], uint32(thenEndPos))

		if s.ElseBranch != nil {
			if err := c.compileStmt(s.ElseBranch); err != nil {
				return err
			}
		}

	case *WhileStmt:
		loopStartPos := int32(c.frame.Bytecode.Len())
		condReg, err := c.compileExpr(s.Condition)
		if err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpUnlessI)
		c.frame.EmitReg(condReg)
		offsetPos := c.frame.Bytecode.Len()
		c.frame.EmitInt32(0)

		if err := c.compileStmt(s.Body); err != nil {
			return err
		}

		c.frame.EmitOp(moargo.OpGoto)
		c.frame.EmitInt32(loopStartPos)

		loopEndPos := int32(c.frame.Bytecode.Len())
		buf := c.frame.Bytecode.Bytes()
		binary.LittleEndian.PutUint32(buf[offsetPos:offsetPos+4], uint32(loopEndPos))

	case *LoopStmt:
		if s.Init != nil {
			if _, err := c.compileExpr(s.Init); err != nil {
				return err
			}
		}
		if s.Cond != nil {
			condReg, err := c.compileExpr(s.Cond)
			if err != nil {
				return err
			}
			c.frame.EmitOp(moargo.OpIfI)
			c.frame.EmitReg(condReg)
			c.frame.EmitInt16(2)
		}
		if err := c.compileStmt(s.Body); err != nil {
			return err
		}
		if s.Step != nil {
			if _, err := c.compileExpr(s.Step); err != nil {
				return err
			}
		}

	case *SubDeclStmt:
		subFrame := c.cu.NewFrame(s.Name, 64)
		for i := 0; i < 64; i++ {
			subFrame.SetLocalType(i, moargo.RegInt64)
		}
		oldFrame := c.frame
		c.frame = subFrame
		for _, param := range s.Params {
			c.allocReg(param.Name)
		}
		if s.Body != nil {
			if err := c.compileStmt(s.Body); err != nil {
				c.frame = oldFrame
				return err
			}
		}
		c.frame.EmitOp(moargo.OpReturn)
		c.frame = oldFrame

	case *ExprStmt:
		if call, ok := s.Expr.(*CallExpr); ok {
			name := ""
			if v, ok := call.Callee.(*VarExpr); ok {
				name = v.Name
			}
			if name == "say" || name == "print" {
				for i, a := range call.Args {
					r, err := c.compileExpr(a)
					if err != nil {
						return err
					}
					if name == "say" && i == len(call.Args)-1 {
						c.frame.EmitOp(moargo.OpSay)
						c.frame.EmitReg(r)
					} else {
						c.frame.EmitOp(moargo.OpPrint)
						c.frame.EmitReg(r)
					}
				}
				if name == "say" && len(call.Args) == 0 {
					r := c.tempReg()
					c.frame.SetLocalType(int(r), moargo.RegStr)
					c.frame.EmitOp(moargo.OpConstS)
					c.frame.EmitReg(r)
					c.frame.EmitString("")
					c.frame.EmitOp(moargo.OpSay)
					c.frame.EmitReg(r)
				}
				return nil
			}
		}
		_, err := c.compileExpr(s.Expr)
		return err

	case *BlockStmt:
		for _, bs := range s.Stmts {
			if err := c.compileStmt(bs); err != nil {
				return err
			}
		}

	case *ReturnStmt:
		if s.Value != nil {
			valReg, err := c.compileExpr(s.Value)
			if err != nil {
				return err
			}
			c.frame.EmitOp(moargo.OpReturnI)
			c.frame.EmitReg(valReg)
		} else {
			c.frame.EmitOp(moargo.OpReturn)
		}
	}
	return nil
}

func (c *Compiler) compileExpr(expr Expr) (uint16, error) {
	switch e := expr.(type) {
	case *LiteralExpr:
		switch e.Type {
		case TokInt:
			val, _ := strconv.ParseInt(fmt.Sprintf("%v", e.Value), 0, 64)
			r := c.tempReg()
			c.frame.SetLocalType(int(r), moargo.RegInt64)
			c.frame.EmitOp(moargo.OpConstI64)
			c.frame.EmitReg(r)
			c.frame.EmitInt64(val)
			return r, nil

		case TokFloat:
			val, _ := strconv.ParseFloat(fmt.Sprintf("%v", e.Value), 64)
			r := c.tempReg()
			c.frame.SetLocalType(int(r), moargo.RegNum64)
			c.frame.EmitOp(moargo.OpConstN64)
			c.frame.EmitReg(r)
			c.frame.EmitInt64(int64(val))
			return r, nil

		case TokString:
			r := c.tempReg()
			c.frame.SetLocalType(int(r), moargo.RegStr)
			c.frame.EmitOp(moargo.OpConstS)
			c.frame.EmitReg(r)
			c.frame.EmitString(fmt.Sprintf("%v", e.Value))
			return r, nil
		}

	case *VarExpr:
		return c.allocReg(e.Name), nil

	case *UnaryExpr:
		targetReg, err := c.compileExpr(e.Right)
		if err != nil {
			return 0, err
		}
		dstReg := c.tempReg()
		switch e.Op {
		case "-":
			c.frame.EmitOp(moargo.OpNegI)
			c.frame.EmitReg(dstReg)
			c.frame.EmitReg(targetReg)
		case "!":
			c.frame.EmitOp(moargo.OpNotI)
			c.frame.EmitReg(dstReg)
			c.frame.EmitReg(targetReg)
		default:
			c.frame.EmitOp(moargo.OpSet)
			c.frame.EmitReg(dstReg)
			c.frame.EmitReg(targetReg)
		}
		return dstReg, nil

	case *TernaryExpr:
		condReg, err := c.compileExpr(e.Cond)
		if err != nil {
			return 0, err
		}
		dstReg := c.tempReg()
		trueReg, err := c.compileExpr(e.Then)
		if err != nil {
			return 0, err
		}
		falseReg, err := c.compileExpr(e.Else)
		if err != nil {
			return 0, err
		}
		c.frame.EmitOp(moargo.OpIfI)
		c.frame.EmitReg(condReg)
		c.frame.EmitInt16(2)
		c.frame.EmitOp(moargo.OpSet)
		c.frame.EmitReg(dstReg)
		c.frame.EmitReg(trueReg)
		c.frame.EmitOp(moargo.OpSet)
		c.frame.EmitReg(dstReg)
		c.frame.EmitReg(falseReg)
		return dstReg, nil

	case *CallExpr:
		var argRegs []uint16
		for _, arg := range e.Args {
			ar, err := c.compileExpr(arg)
			if err != nil {
				return 0, err
			}
			argRegs = append(argRegs, ar)
		}

		calleeName := ""
		if varExpr, ok := e.Callee.(*VarExpr); ok {
			calleeName = varExpr.Name
		}

		if calleeName == "say" || calleeName == "print" {
			for _, ar := range argRegs {
				if calleeName == "say" {
					c.frame.EmitOp(moargo.OpSay)
				} else {
					c.frame.EmitOp(moargo.OpPrint)
				}
				c.frame.EmitReg(ar)
			}
			r := c.tempReg()
			c.frame.EmitOp(moargo.OpConstI64)
			c.frame.EmitReg(r)
			c.frame.EmitInt64(1)
			return r, nil
		}

		dstReg := c.tempReg()
		c.frame.EmitOp(moargo.OpPrepArgs)
		for _, ar := range argRegs {
			c.frame.EmitOp(moargo.OpArgI)
			c.frame.EmitReg(ar)
		}
		c.frame.EmitOp(moargo.OpInvoke)
		c.frame.EmitReg(dstReg)
		return dstReg, nil

	case *ArrayLiteralExpr:
		dstReg := c.tempReg()
		c.frame.SetLocalType(int(dstReg), moargo.RegObj)
		for idx, item := range e.Elements {
			ir, err := c.compileExpr(item)
			if err != nil {
				return 0, err
			}
			c.frame.EmitOp(moargo.OpBindPosI)
			c.frame.EmitReg(dstReg)
			c.frame.EmitInt64(int64(idx))
			c.frame.EmitReg(ir)
		}
		return dstReg, nil

	case *IndexExpr:
		arrReg, err := c.compileExpr(e.Array)
		if err != nil {
			return 0, err
		}
		idxReg, err := c.compileExpr(e.Index)
		if err != nil {
			return 0, err
		}
		dstReg := c.tempReg()
		c.frame.EmitOp(moargo.OpAtPosI)
		c.frame.EmitReg(dstReg)
		c.frame.EmitReg(arrReg)
		c.frame.EmitReg(idxReg)
		return dstReg, nil

	case *BinaryExpr:
		leftReg, err := c.compileExpr(e.Left)
		if err != nil {
			return 0, err
		}
		rightReg, err := c.compileExpr(e.Right)
		if err != nil {
			return 0, err
		}

		dstReg := c.tempReg()
		switch e.Op {
		case "+":
			c.frame.EmitOp(moargo.OpAddI)
		case "-":
			c.frame.EmitOp(moargo.OpSubI)
		case "*":
			c.frame.EmitOp(moargo.OpMulI)
		case "/":
			c.frame.EmitOp(moargo.OpDivI)
		case "%", "mod":
			c.frame.EmitOp(moargo.OpModI)
		case "**":
			c.frame.EmitOp(moargo.OpPowI)
		case "==":
			c.frame.EmitOp(moargo.OpEqI)
		case "!=":
			c.frame.EmitOp(moargo.OpNeI)
		case "<":
			c.frame.EmitOp(moargo.OpLtI)
		case "<=":
			c.frame.EmitOp(moargo.OpLeI)
		case ">":
			c.frame.EmitOp(moargo.OpGtI)
		case ">=":
			c.frame.EmitOp(moargo.OpGeI)
		case "eq":
			c.frame.EmitOp(moargo.OpEqS)
		case "ne":
			c.frame.EmitOp(moargo.OpNeS)
		case "~":
			c.frame.EmitOp(moargo.OpConcatS)
		default:
			c.frame.EmitOp(moargo.OpAddI)
		}
		c.frame.EmitReg(dstReg)
		c.frame.EmitReg(leftReg)
		c.frame.EmitReg(rightReg)
		return dstReg, nil

	case *AssignStmt:
		valReg, err := c.compileExpr(e.Value)
		if err != nil {
			return 0, err
		}
		if varExpr, ok := e.Target.(*VarExpr); ok {
			reg := c.allocReg(varExpr.Name)
			c.frame.EmitOp(moargo.OpSet)
			c.frame.EmitReg(reg)
			c.frame.EmitReg(valReg)
			return reg, nil
		}
	}

	res := c.tempReg()
	return res, nil
}

// CompileAndRun compiles a Raku5 script to MoarVM bytecode and executes it directly on MoarVM.
func CompileAndRun(ctx context.Context, vm moargo.Engine, script string) error {
	compiler := NewCompiler()
	bc, err := compiler.CompileScript(script)
	if err != nil {
		return fmt.Errorf("raku5 compilation failed: %w", err)
	}
	return vm.RunBytecode(ctx, bc)
}
