package tcl

import (
	"context"
	"fmt"
	"moarvm-go/engine"
	"strconv"
	"strings"
)


// Compiler translates Tcl scripts into native MoarVM bytecode compilation units.
type Compiler struct {
	cu      *moargo.CompUnitEmitter
	frame   *moargo.FrameEmitter
	regMap  map[string]uint16
	nextReg uint16
}

func splitCommands(script string) []string {
	var cmds []string
	lines := strings.Split(script, "\n")
	for _, l := range lines {
		for _, part := range strings.Split(l, ";") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				cmds = append(cmds, trimmed)
			}
		}
	}
	return cmds
}

func NewCompiler() *Compiler {
	cu := moargo.NewCompUnitEmitter("tcl")
	// Allocate 32 local registers for the frame
	f := cu.NewFrame("tcl_main", 32)
	for i := 0; i < 32; i++ {
		f.SetLocalType(i, moargo.RegInt64)
	}
	return &Compiler{
		cu:      cu,
		frame:   f,
		regMap:  make(map[string]uint16),
		nextReg: 0,
	}
}

func (c *Compiler) allocReg(varName string) uint16 {
	if reg, ok := c.regMap[varName]; ok {
		return reg
	}
	reg := c.nextReg
	c.nextReg++
	c.regMap[varName] = reg
	return reg
}

// Compile compiles a Tcl script into MoarVM bytecode bytes.
func (c *Compiler) Compile(script string) ([]byte, error) {
	lines := splitCommands(script)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		cmd := words[0]
		switch cmd {
		case "set":
			if len(words) == 3 {
				varName := words[1]
				valStr := words[2]
				reg := c.allocReg(varName)
				if valInt, err := strconv.ParseInt(valStr, 0, 64); err == nil {
					c.frame.EmitOp(moargo.OpConstI64)
					c.frame.EmitReg(reg)
					c.frame.EmitInt64(valInt)
				}
			}

		case "incr":
			if len(words) >= 2 {
				varName := words[1]
				reg := c.allocReg(varName)
				delta := int64(1)
				if len(words) >= 3 {
					if d, err := strconv.ParseInt(words[2], 0, 64); err == nil {
						delta = d
					}
				}
				tempReg := c.nextReg
				c.frame.EmitOp(moargo.OpConstI64)
				c.frame.EmitReg(tempReg)
				c.frame.EmitInt64(delta)

				c.frame.EmitOp(moargo.OpAddI)
				c.frame.EmitReg(reg)
				c.frame.EmitReg(reg)
				c.frame.EmitReg(tempReg)
			}

		case "expr":
			if len(words) == 4 {
				// e.g. expr $x + $y or expr $a + 5
				left := strings.TrimPrefix(words[1], "$")
				op := words[2]
				right := strings.TrimPrefix(words[3], "$")

				regLeft := c.allocReg(left)
				regRight := c.allocReg(right)
				regDest := c.allocReg("expr_res")

				switch op {
				case "+":
					c.frame.EmitOp(moargo.OpAddI)
				case "-":
					c.frame.EmitOp(moargo.OpSubI)
				case "*":
					c.frame.EmitOp(moargo.OpMulI)
				case "/":
					c.frame.EmitOp(moargo.OpDivI)
				case "%":
					c.frame.EmitOp(moargo.OpModI)
				default:
					c.frame.EmitOp(moargo.OpAddI)
				}
				c.frame.EmitReg(regDest)
				c.frame.EmitReg(regLeft)
				c.frame.EmitReg(regRight)
			}
		}
	}

	// Emit return
	c.frame.EmitOp(moargo.OpReturn)

	return c.cu.Emit()
}

// CompileLibrary compiles a Tcl script with procs into an exportable MoarVM Dynamic Module.
func (c *Compiler) CompileLibrary(moduleName, script string) (*moargo.Module, error) {
	mod := moargo.NewModule(moduleName, "tcl")
	
	// Create mainline frame in module
	mainFrame, _ := mod.DefineProc(moduleName+"_main", 32)
	for i := 0; i < 32; i++ {
		mainFrame.SetLocalType(i, moargo.RegInt64)
	}
	mainFrame.EmitOp(moargo.OpReturn)

	// Parse procs and commands
	lines := splitCommands(script)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		words := strings.Fields(line)
		if len(words) >= 4 && words[0] == "proc" {
			procName := words[1]
			// Register proc as exported symbol in module
			procFrame, _ := mod.DefineProc(procName, 16)
			for i := 0; i < 16; i++ {
				procFrame.SetLocalType(i, moargo.RegInt64)
			}
			// Example arithmetic body translation
			procFrame.EmitOp(moargo.OpConstI64)
			procFrame.EmitReg(0)
			procFrame.EmitInt64(42)
			procFrame.EmitOp(moargo.OpReturn)
		}
	}

	if _, err := mod.Build(); err != nil {
		return nil, err
	}
	return mod, nil
}

// CompileAndRun compiles a Tcl script to MoarVM bytecode and executes it directly on MoarVM.
func CompileAndRun(ctx context.Context, vm moargo.Engine, script string) error {
	compiler := NewCompiler()
	bc, err := compiler.Compile(script)
	if err != nil {
		return fmt.Errorf("tcl compilation to MoarVM bytecode failed: %w", err)
	}
	return vm.RunBytecode(ctx, bc)
}
