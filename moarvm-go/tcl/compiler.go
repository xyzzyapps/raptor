package tcl

import (
	"context"
	"fmt"
	"moarvm-go/engine"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Compiler translates a grammar-parsed Tcl AST into a MoarVM compilation unit.
// Command semantics live in Go; the product is MoarVM bytecode.
type frameCtx struct {
	frame   *moargo.FrameEmitter
	regMap  map[string]uint16
	nextReg uint16
	lastReg uint16
	lastStr bool
}

type Compiler struct {
	cu       *moargo.CompUnitEmitter
	frame    *moargo.FrameEmitter
	regMap   map[string]uint16
	nextReg  uint16
	lastReg  uint16
	lastStr  bool
	stack      []frameCtx
	coro       map[string]bool
	lambdaN    int
	procFrame  map[string]int
	didSay     bool
	needResult bool
	coroLex    map[string][3]uint16
	lists      map[string][]string
	lastList   []string
	strConst   map[string]string
	strNote    string
	varType    map[string]uint16
}

func NewCompiler() *Compiler {
	cu := moargo.NewCompUnitEmitter("tcl")
	f := cu.NewFrame("tcl_main", 256)
	for i := 0; i < 256; i++ {
		f.SetLocalType(i, moargo.RegInt64)
	}
	// string-capable slots for puts / concat
	f.SetLocalType(200, moargo.RegStr)
	f.SetLocalType(201, moargo.RegStr)
	f.SetLocalType(202, moargo.RegStr)
	f.SetLocalType(203, moargo.RegStr)
	return &Compiler{
		cu:      cu,
		frame:   f,
		regMap:  make(map[string]uint16),
		nextReg: 0,
		coro:      make(map[string]bool),
		procFrame:  make(map[string]int),
		needResult: true,
		coroLex:    make(map[string][3]uint16),
		lists:      make(map[string][]string),
		strConst:   make(map[string]string),
		varType:    make(map[string]uint16),
	}
}

func (c *Compiler) pushFrame(name string, params []string) int {
	c.stack = append(c.stack, frameCtx{
		frame: c.frame, regMap: c.regMap, nextReg: c.nextReg,
		lastReg: c.lastReg, lastStr: c.lastStr,
	})
	parent := c.frame
	f := c.cu.NewFrame(name, 256)
	for i := 0; i < 256; i++ {
		f.SetLocalType(i, moargo.RegInt64)
	}
	if parent != nil {
		f.SetOuter(parent.Index)
	}
	if strings.HasPrefix(name, "proc_") {
		f.SetOuter(f.Index)
	}
	c.frame = f
	c.regMap = make(map[string]uint16)
	c.nextReg = 0
	c.lastReg = 0
	c.lastStr = false
	for _, p := range params {
		c.allocReg(p)
	}
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
	c.nextReg = top.nextReg
	c.lastReg = top.lastReg
	c.lastStr = top.lastStr
}

func (c *Compiler) allocReg(varName string) uint16 {
	if varName != "" {
		if reg, ok := c.regMap[varName]; ok {
			return reg
		}
	}
	reg := c.nextReg
	c.nextReg++
	if varName != "" {
		c.regMap[varName] = reg
	}
	return reg
}

func (c *Compiler) tempReg() uint16 {
	return c.allocReg("")
}

func (c *Compiler) CompUnit() *moargo.CompUnitEmitter {
	return c.cu
}

// CompileScript parses with the Tcl grammar and emits MoarVM bytecode.
func (c *Compiler) CompileScript(script string) ([]byte, error) {
	cmds, err := ParseTclAST(script)
	if err != nil {
		return nil, err
	}
	if err := c.emitCommands(cmds); err != nil {
		return nil, err
	}
	if c.needResult && !c.didSay {
		c.emitSayLast()
	}
	c.frame.EmitOp(moargo.OpReturn)
	return c.cu.Emit()
}

func (c *Compiler) emitSayLast() {
	s := c.tempReg()
	c.frame.SetLocalType(int(s), moargo.RegStr)
	if c.lastStr {
		c.frame.EmitOp(moargo.OpSet)
		c.frame.EmitReg(s)
		c.frame.EmitReg(c.lastReg)
	} else {
		c.frame.EmitOp(moargo.OpCoerceIS)
		c.frame.EmitReg(s)
		c.frame.EmitReg(c.lastReg)
	}
	c.frame.EmitOp(moargo.OpSay)
	c.frame.EmitReg(s)
}

func (c *Compiler) emitCommands(cmds []Command) error {
	for _, cmd := range cmds {
		if err := c.emitCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (c *Compiler) emitCommand(cmd Command) error {
	if len(cmd.Words) == 0 {
		return nil
	}
	name := cmd.Words[0].Inner
	if cmd.Words[0].Kind == WordVar || cmd.Words[0].Kind == WordCmd {
		return fmt.Errorf("dynamic command names are not compiled to bytecode")
	}
	args := cmd.Words[1:]
	switch name {
	case "set":
		return c.emitSet(args)
	case "incr":
		return c.emitIncr(args)
	case "expr":
		return c.emitExpr(args)
	case "puts":
		return c.emitPuts(args)
	case "if":
		return c.emitIf(args)
	case "while":
		return c.emitWhile(args)
	case "for":
		return c.emitFor(args)
	case "list":
		return c.emitList(args)
	case "llength":
		return c.emitLlength(args)
	case "lindex":
		return c.emitLindex(args)
	case "return":
		return c.emitReturn(args)
	case "apply":
		return c.emitApply(args)
	case "yield":
		return c.emitYield(args)
	case "coroutine":
		return c.emitCoroutine(args)
	case "proc":
		return c.emitProc(args)
	case "string":
		return c.emitStringCmd(args)
	case "foreach":
		return c.emitForeach(args)
	case "switch":
		return c.emitSwitch(args)
	case "lappend":
		return c.emitLappend(args)
	case "lrange":
		return c.emitLrange(args)
	default:
		if idx, ok := c.procFrame[name]; ok {
			return c.emitCallFrame(idx, args)
		}
		if c.coro[name] {
			return c.emitCoroResume(name, args)
		}
		return fmt.Errorf("command %q cannot be compiled to MoarVM bytecode", name)
	}
}

func (c *Compiler) emitSet(args []Word) error {
	if len(args) == 1 {
		reg := c.allocReg(args[0].Inner)
		c.lastReg = reg
		return nil
	}
	if len(args) != 2 {
		return fmt.Errorf("wrong # args: should be \"set varName ?newValue?\"")
	}
	dest := c.allocReg(args[0].Inner)
	c.lastList = nil
	if err := c.emitWord(args[1], dest); err != nil {
		return err
	}
	if c.lastList != nil {
		c.lists[args[0].Inner] = c.lastList
	}
	if args[1].Kind == WordBrace || args[1].Kind == WordQuote || args[1].Kind == WordBare {
		c.strConst[args[0].Inner] = args[1].Inner
	} else if c.strNote != "" {
		c.strConst[args[0].Inner] = c.strNote
	}
	if c.lastStr {
		c.frame.SetLocalType(int(dest), moargo.RegStr)
		c.varType[args[0].Inner] = moargo.RegStr
	} else {
		c.varType[args[0].Inner] = moargo.RegInt64
	}
	c.lastReg = dest
	c.didSay = false
	return nil
}

func (c *Compiler) emitIncr(args []Word) error {
	if len(args) < 1 {
		return fmt.Errorf("wrong # args: should be \"incr varName ?increment?\"")
	}
	reg := c.allocReg(args[0].Inner)
	if len(args) == 1 {
		c.frame.EmitOp(moargo.OpIncI)
		c.frame.EmitReg(reg)
		c.lastReg = reg
		return nil
	}
	delta := c.tempReg()
	if err := c.emitWord(args[1], delta); err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpAddI)
	c.frame.EmitReg(reg)
	c.frame.EmitReg(reg)
	c.frame.EmitReg(delta)
	c.lastReg = reg
	return nil
}

func (c *Compiler) emitExpr(args []Word) error {
	if len(args) == 0 {
		return fmt.Errorf("wrong # args: should be \"expr arg ?arg ...?\"")
	}
	// Flatten expr words: either a single braced/quoted expression or tokens.
	tokens := exprTokens(args)
	dest := c.tempReg()
	if err := c.compileExprTokens(tokens, dest); err != nil {
		return err
	}
	c.lastReg = dest
	c.lastStr = false
	return nil
}

func exprTokens(args []Word) []Word {
	if len(args) == 1 && (args[0].Kind == WordBrace || args[0].Kind == WordQuote || strings.ContainsAny(args[0].Inner, " \t")) {
		return tokenizeExpr(args[0].Inner)
	}
	return args
}

func tokenizeExpr(s string) []Word {
	var out []Word
	i := 0
	for i < len(s) {
		for i < len(s) && unicode.IsSpace(rune(s[i])) {
			i++
		}
		if i >= len(s) {
			break
		}
		if s[i] == '$' {
			j := i + 1
			for j < len(s) && (unicode.IsLetter(rune(s[j])) || unicode.IsDigit(rune(s[j])) || s[j] == '_') {
				j++
			}
			out = append(out, Word{Kind: WordVar, Raw: s[i:j], Inner: s[i+1 : j]})
			i = j
			continue
		}
		if s[i] == '[' {
			depth := 1
			j := i + 1
			for j < len(s) && depth > 0 {
				if s[j] == '[' {
					depth++
				} else if s[j] == ']' {
					depth--
				}
				j++
			}
			inner := s[i+1 : j-1]
			out = append(out, Word{Kind: WordCmd, Raw: s[i:j], Inner: inner})
			i = j
			continue
		}
		// two-char operators
		if i+1 < len(s) {
			two := s[i : i+2]
			if two == "==" || two == "!=" || two == "<=" || two == ">=" || two == "&&" || two == "||" {
				out = append(out, Word{Kind: WordBare, Raw: two, Inner: two})
				i += 2
				continue
			}
		}
		ch := s[i]
		if strings.ContainsRune("+-*/%<>=", rune(ch)) {
			out = append(out, Word{Kind: WordBare, Raw: string(ch), Inner: string(ch)})
			i++
			continue
		}
		j := i
		for j < len(s) && !unicode.IsSpace(rune(s[j])) && !strings.ContainsRune("+-*/%<>=$[", rune(s[j])) {
			j++
		}
		out = append(out, Word{Kind: WordBare, Raw: s[i:j], Inner: s[i:j]})
		i = j
	}
	return out
}

func (c *Compiler) emitInt(w Word, dest uint16) error {
	tmp := c.tempReg()
	if err := c.emitWord(w, tmp); err != nil {
		return err
	}
	c.frame.SetLocalType(int(dest), moargo.RegInt64)
	if c.lastStr {
		c.frame.EmitOp(moargo.OpCoerceSI)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(tmp)
	} else {
		c.frame.EmitOp(moargo.OpSet)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(tmp)
	}
	c.lastStr = false
	c.lastReg = dest
	return nil
}

func (c *Compiler) compileExprTokens(tokens []Word, dest uint16) error {
	if len(tokens) == 1 {
		return c.emitInt(tokens[0], dest)
	}
	if len(tokens) == 3 {
		left := c.tempReg()
		right := c.tempReg()
		if err := c.emitInt(tokens[0], left); err != nil {
			return err
		}
		if err := c.emitInt(tokens[2], right); err != nil {
			return err
		}
		op := tokens[1].Inner
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
		default:
			return fmt.Errorf("unsupported expr operator %q", op)
		}
		c.frame.EmitReg(dest)
		c.frame.EmitReg(left)
		c.frame.EmitReg(right)
		return nil
	}
	// left-fold a + b + c ...
	if len(tokens) >= 3 && (len(tokens)-1)%2 == 0 {
		if err := c.emitInt(tokens[0], dest); err != nil {
			return err
		}
		for i := 1; i+1 < len(tokens); i += 2 {
			rhs := c.tempReg()
			if err := c.emitInt(tokens[i+1], rhs); err != nil {
				return err
			}
			switch tokens[i].Inner {
			case "+":
				c.frame.EmitOp(moargo.OpAddI)
			case "-":
				c.frame.EmitOp(moargo.OpSubI)
			case "*":
				c.frame.EmitOp(moargo.OpMulI)
			case "/":
				c.frame.EmitOp(moargo.OpDivI)
			default:
				return fmt.Errorf("unsupported expr operator %q", tokens[i].Inner)
			}
			c.frame.EmitReg(dest)
			c.frame.EmitReg(dest)
			c.frame.EmitReg(rhs)
		}
		return nil
	}
	return fmt.Errorf("unsupported expr form")
}

func (c *Compiler) emitPuts(args []Word) error {
	if len(args) != 1 {
		return fmt.Errorf("wrong # args: should be \"puts string\"")
	}
	dest := c.tempReg()
	c.frame.SetLocalType(int(dest), moargo.RegStr)
	if err := c.emitWordAsStr(args[0], dest); err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpSay)
	c.frame.EmitReg(dest)
	c.lastReg = dest
	c.lastStr = true
	c.didSay = true
	return nil
}

func (c *Compiler) emitIf(args []Word) error {
	// if cond body [elseif cond body]* [else body]
	type branch struct {
		cond []Word
		body string
	}
	var branches []branch
	i := 0
	for i < len(args) {
		if i == 0 {
			if i+1 >= len(args) {
				return fmt.Errorf("wrong # args: if")
			}
			branches = append(branches, branch{cond: []Word{args[i]}, body: args[i+1].Inner})
			i += 2
			continue
		}
		if args[i].Inner == "elseif" && i+2 < len(args) {
			branches = append(branches, branch{cond: []Word{args[i+1]}, body: args[i+2].Inner})
			i += 3
			continue
		}
		if args[i].Inner == "else" && i+1 < len(args) {
			branches = append(branches, branch{body: args[i+1].Inner})
			i += 2
			continue
		}
		return fmt.Errorf("malformed if")
	}

	var endPatches []int32
	for _, b := range branches {
		var skipPatch int32 = -1
		if len(b.cond) > 0 {
			condReg := c.tempReg()
			if err := c.compileExprTokens(exprTokens(b.cond), condReg); err != nil {
				return err
			}
			c.frame.EmitOp(moargo.OpUnlessI)
			c.frame.EmitReg(condReg)
			skipPatch = c.frame.CurrentOffset()
			c.frame.EmitInt32(0)
		}
		bodyCmds, err := ParseTclAST(b.body)
		if err != nil {
			return err
		}
		if err := c.emitCommands(bodyCmds); err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpGoto)
		endPatches = append(endPatches, c.frame.CurrentOffset())
		c.frame.EmitInt32(0)
		if skipPatch >= 0 {
			c.patchU32(skipPatch, uint32(c.frame.CurrentOffset()))
		}
	}
	end := uint32(c.frame.CurrentOffset())
	for _, p := range endPatches {
		c.patchU32(p, end)
	}
	return nil
}

func (c *Compiler) emitWhile(args []Word) error {
	if len(args) != 2 {
		return fmt.Errorf("wrong # args: should be \"while test body\"")
	}
	loop := c.frame.CurrentOffset()
	condReg := c.tempReg()
	if err := c.compileExprTokens(exprTokens([]Word{args[0]}), condReg); err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpUnlessI)
	c.frame.EmitReg(condReg)
	endPatch := c.frame.CurrentOffset()
	c.frame.EmitInt32(0)

	bodyCmds, err := ParseTclAST(args[1].Inner)
	if err != nil {
		return err
	}
	if err := c.emitCommands(bodyCmds); err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	c.patchU32(endPatch, uint32(c.frame.CurrentOffset()))
	return nil
}

func (c *Compiler) emitFor(args []Word) error {
	if len(args) != 4 {
		return fmt.Errorf("wrong # args: should be \"for start test next body\"")
	}
	startCmds, err := ParseTclAST(args[0].Inner)
	if err != nil {
		return err
	}
	if err := c.emitCommands(startCmds); err != nil {
		return err
	}
	loop := c.frame.CurrentOffset()
	condReg := c.tempReg()
	if err := c.compileExprTokens(exprTokens([]Word{args[1]}), condReg); err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpUnlessI)
	c.frame.EmitReg(condReg)
	endPatch := c.frame.CurrentOffset()
	c.frame.EmitInt32(0)

	bodyCmds, err := ParseTclAST(args[3].Inner)
	if err != nil {
		return err
	}
	if err := c.emitCommands(bodyCmds); err != nil {
		return err
	}
	nextCmds, err := ParseTclAST(args[2].Inner)
	if err != nil {
		return err
	}
	if err := c.emitCommands(nextCmds); err != nil {
		return err
	}
	c.frame.EmitOp(moargo.OpGoto)
	c.frame.EmitInt32(loop)
	c.patchU32(endPatch, uint32(c.frame.CurrentOffset()))
	return nil
}

func (c *Compiler) emitList(args []Word) error {
	allLit := true
	var lits []string
	for _, a := range args {
		if a.Kind == WordBare || a.Kind == WordBrace || a.Kind == WordQuote {
			lits = append(lits, a.Inner)
		} else {
			allLit = false
		}
	}
	if len(args) == 0 {
		dest := c.tempReg()
		c.emitConstS(dest, "")
		c.lastReg = dest
		c.lastStr = true
		c.lastList = []string{}
		return nil
	}
	dest := c.tempReg()
	if err := c.emitWordAsStr(args[0], dest); err != nil {
		return err
	}
	c.frame.SetLocalType(int(dest), moargo.RegStr)
	for i := 1; i < len(args); i++ {
		space := c.tempReg()
		c.emitConstS(space, " ")
		tmp := c.tempReg()
		c.frame.SetLocalType(int(tmp), moargo.RegStr)
		c.frame.EmitOp(moargo.OpConcatS)
		c.frame.EmitReg(tmp)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(space)
		elem := c.tempReg()
		if err := c.emitWordAsStr(args[i], elem); err != nil {
			return err
		}
		c.frame.SetLocalType(int(dest), moargo.RegStr)
		c.frame.EmitOp(moargo.OpConcatS)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(tmp)
		c.frame.EmitReg(elem)
	}
	c.lastReg = dest
	c.lastStr = true
	if allLit {
		c.lastList = lits
	}
	return nil
}

func (c *Compiler) emitLlength(args []Word) error {
	// Compile-time length when the argument is a literal list; otherwise
	// materialize the string and count words at interpret time via a
	// const of the split — we emit a runtime walk by compiling to a
	// host-evaluated constant only when the word is a brace/bare literal.
	if len(args) != 1 {
		return fmt.Errorf("wrong # args: should be \"llength list\"")
	}
	if items, ok := c.knownList(args[0]); ok {
		dest := c.tempReg()
		c.emitConstI(dest, int64(len(items)))
		c.lastReg = dest
		c.lastStr = false
		return nil
	}
	return fmt.Errorf("llength of unknown list cannot be compiled")
}

func (c *Compiler) knownStr(w Word) (string, bool) {
	if w.Kind == WordBrace || w.Kind == WordBare || w.Kind == WordQuote {
		return w.Inner, true
	}
	if w.Kind == WordVar {
		s, ok := c.strConst[w.Inner]
		return s, ok
	}
	return "", false
}

func (c *Compiler) knownList(w Word) ([]string, bool) {
	if w.Kind == WordBrace || w.Kind == WordBare || w.Kind == WordQuote {
		return SplitTclList(w.Inner), true
	}
	if w.Kind == WordVar {
		if items, ok := c.lists[w.Inner]; ok {
			return items, true
		}
	}
	return nil, false
}

func (c *Compiler) emitLindex(args []Word) error {
	if len(args) < 2 {
		return fmt.Errorf("wrong # args: should be \"lindex list index\"")
	}
	items, ok := c.knownList(args[0])
	if !ok {
		return fmt.Errorf("lindex of unknown list cannot be compiled")
	}
	idx := 0
	if n, err := strconv.Atoi(args[1].Inner); err == nil {
		idx = n
	}
	dest := c.tempReg()
	val := ""
	if idx >= 0 && idx < len(items) {
		val = items[idx]
	}
	c.emitConstS(dest, val)
	c.lastReg = dest
	c.lastStr = true
	return nil
}

func (c *Compiler) emitReturn(args []Word) error {
	if len(args) == 0 {
		c.frame.EmitOp(moargo.OpReturn)
		return nil
	}
	dest := c.tempReg()
	if err := c.emitWord(args[0], dest); err != nil {
		return err
	}
	if c.lastStr {
		c.frame.SetLocalType(int(dest), moargo.RegStr)
		c.frame.EmitOp(moargo.OpReturnS)
	} else {
		c.frame.EmitOp(moargo.OpReturnI)
	}
	c.frame.EmitReg(dest)
	c.lastReg = dest
	return nil
}

func (c *Compiler) emitWord(w Word, dest uint16) error {
	switch w.Kind {
	case WordBrace:
		if n, err := strconv.ParseInt(strings.TrimSpace(w.Inner), 0, 64); err == nil && !strings.ContainsAny(w.Inner, " \t") {
			c.emitConstI(dest, n)
			c.lastStr = false
			return nil
		}
		c.emitConstS(dest, w.Inner)
		c.lastStr = true
		return nil
	case WordBare:
		if n, err := strconv.ParseInt(w.Inner, 0, 64); err == nil {
			c.emitConstI(dest, n)
			c.lastStr = false
			return nil
		}
		c.emitConstS(dest, w.Inner)
		c.lastStr = true
		return nil
	case WordVar:
		src := c.allocReg(w.Inner)
		if c.varType[w.Inner] == moargo.RegStr {
			c.frame.SetLocalType(int(src), moargo.RegStr)
			c.frame.SetLocalType(int(dest), moargo.RegStr)
			c.lastStr = true
		} else {
			c.lastStr = false
		}
		c.frame.EmitOp(moargo.OpSet)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(src)
		c.lastReg = dest
		return nil
	case WordQuote:
		return c.emitQuoted(w.Inner, dest)
	case WordCmd:
		cmds, err := ParseTclAST(w.Inner)
		if err != nil {
			return err
		}
		if err := c.emitCommands(cmds); err != nil {
			return err
		}
		if c.lastStr {
			c.frame.SetLocalType(int(dest), moargo.RegStr)
		}
		c.frame.EmitOp(moargo.OpSet)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(c.lastReg)
		return nil
	default:
		c.emitConstS(dest, w.Inner)
		c.lastStr = true
		return nil
	}
}

func (c *Compiler) emitQuoted(s string, dest uint16) error {
	c.frame.SetLocalType(int(dest), moargo.RegStr)
	// Split into literal / $var / [cmd] pieces and concat_s them.
	var pieces []uint16
	var lit strings.Builder
	flushLit := func() {
		if lit.Len() == 0 {
			return
		}
		r := c.tempReg()
		c.emitConstS(r, lit.String())
		pieces = append(pieces, r)
		lit.Reset()
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if ch == '$' {
			flushLit()
			j := i + 1
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			name := string(runes[i+1 : j])
			r := c.tempReg()
			if err := c.emitWordAsStr(Word{Kind: WordVar, Inner: name, Raw: "$" + name}, r); err != nil {
				return err
			}
			pieces = append(pieces, r)
			i = j - 1
			continue
		}
		if ch == '[' {
			flushLit()
			depth := 1
			j := i + 1
			for j < len(runes) && depth > 0 {
				if runes[j] == '[' {
					depth++
				} else if runes[j] == ']' {
					depth--
				}
				j++
			}
			inner := string(runes[i+1 : j-1])
			cmds, err := ParseTclAST(inner)
			if err != nil {
				return err
			}
			if err := c.emitCommands(cmds); err != nil {
				return err
			}
			r := c.tempReg()
			c.frame.SetLocalType(int(r), moargo.RegStr)
			if c.lastStr {
				c.frame.EmitOp(moargo.OpSet)
				c.frame.EmitReg(r)
				c.frame.EmitReg(c.lastReg)
			} else {
				c.frame.EmitOp(moargo.OpCoerceIS)
				c.frame.EmitReg(r)
				c.frame.EmitReg(c.lastReg)
			}
			pieces = append(pieces, r)
			i = j - 1
			continue
		}
		if ch == '\\' && i+1 < len(runes) {
			i++
			switch runes[i] {
			case 'n':
				lit.WriteByte('\n')
			case 't':
				lit.WriteByte('\t')
			default:
				lit.WriteRune(runes[i])
			}
			continue
		}
		lit.WriteRune(ch)
	}
	flushLit()
	if len(pieces) == 0 {
		c.emitConstS(dest, "")
		c.lastStr = true
		return nil
	}
	c.frame.SetLocalType(int(dest), moargo.RegStr)
	c.frame.EmitOp(moargo.OpSet)
	c.frame.EmitReg(dest)
	c.frame.EmitReg(pieces[0])
	for i := 1; i < len(pieces); i++ {
		tmp := c.tempReg()
		c.frame.SetLocalType(int(tmp), moargo.RegStr)
		c.frame.EmitOp(moargo.OpConcatS)
		c.frame.EmitReg(tmp)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(pieces[i])
		c.frame.EmitOp(moargo.OpSet)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(tmp)
	}
	c.lastStr = true
	return nil
}

func parseLambdaSpec(spec string) (params []string, body string, err error) {
	parts := SplitTclList(spec)
	if len(parts) < 2 {
		return nil, "", fmt.Errorf("apply: lambda must be {arglist body}")
	}
	params = SplitTclList(parts[0])
	if len(params) == 1 && params[0] == "" {
		params = nil
	}
	body = parts[1]
	return params, body, nil
}

func (c *Compiler) emitLambda(params []string, body string) (uint16, error) {
	c.lambdaN++
	idx := c.pushFrame(fmt.Sprintf("lambda_%d", c.lambdaN), params)
	n := int16(len(params))
	c.frame.EmitOp(moargo.OpCheckArity)
	c.frame.EmitInt16(n)
	c.frame.EmitInt16(n)
	for i := range params {
		c.frame.EmitOp(moargo.OpParamRpI)
		c.frame.EmitReg(uint16(i))
		c.frame.EmitInt16(int16(i))
	}
	bodyCmds, err := ParseTclAST(body)
	if err != nil {
		c.popFrame()
		return 0, err
	}
	if err := c.emitCommands(bodyCmds); err != nil {
		c.popFrame()
		return 0, err
	}
	if c.lastStr {
		c.frame.EmitOp(moargo.OpReturnS)
		c.frame.EmitReg(c.lastReg)
	} else {
		c.frame.EmitOp(moargo.OpReturnI)
		c.frame.EmitReg(c.lastReg)
	}
	c.popFrame()

	code := c.tempReg()
	c.frame.SetLocalType(int(code), moargo.RegObj)
	c.frame.EmitGetCode(code, uint16(idx))
	clos := c.tempReg()
	c.frame.SetLocalType(int(clos), moargo.RegObj)
	c.frame.EmitOp(moargo.OpTakeClosure)
	c.frame.EmitReg(clos)
	c.frame.EmitReg(code)
	return clos, nil
}

func (c *Compiler) emitApply(args []Word) error {
	if len(args) < 1 {
		return fmt.Errorf("wrong # args: should be \"apply {arglist body} ?arg ...?\"")
	}
	params, body, err := parseLambdaSpec(args[0].Inner)
	if err != nil {
		return err
	}
	if len(args)-1 != len(params) {
		return fmt.Errorf("wrong # args: apply expected %d argument(s)", len(params))
	}
	clos, err := c.emitLambda(params, body)
	if err != nil {
		return err
	}
	argRegs := make([]uint16, len(params))
	for i := range params {
		argRegs[i] = c.tempReg()
		if err := c.emitWord(args[i+1], argRegs[i]); err != nil {
			return err
		}
	}
	dest := c.tempReg()
	flags := []uint8{moargo.CallArgObj}
	regs := []uint16{clos}
	for _, r := range argRegs {
		flags = append(flags, moargo.CallArgInt)
		regs = append(regs, r)
	}
	cs := c.cu.AddCallsite(flags)
	c.frame.SetLocalType(int(dest), moargo.RegInt64)
	c.frame.EmitDispatchI(dest, "boot-code", cs, regs...)
	c.lastReg = dest
	c.lastStr = false
	c.didSay = false
	return nil
}

func (c *Compiler) emitWordAsStr(w Word, dest uint16) error {
	c.frame.SetLocalType(int(dest), moargo.RegStr)
	tmp := c.tempReg()
	if err := c.emitWord(w, tmp); err != nil {
		return err
	}
	if c.lastStr {
		c.frame.SetLocalType(int(tmp), moargo.RegStr)
		c.frame.EmitOp(moargo.OpSet)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(tmp)
	} else {
		c.frame.EmitOp(moargo.OpCoerceIS)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(tmp)
	}
	c.lastStr = true
	c.lastReg = dest
	return nil
}

func mustParseInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 0, 64)
	return n
}

func (c *Compiler) emitBindLex(lex uint16, outers uint16, src uint16) {
	c.frame.EmitOp(moargo.OpBindLex)
	c.frame.EmitReg(lex)
	c.frame.EmitReg(outers)
	c.frame.EmitReg(src)
}

func (c *Compiler) emitGetLex(dst uint16, lex uint16, outers uint16) {
	c.frame.EmitOp(moargo.OpGetLex)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(lex)
	c.frame.EmitReg(outers)
}

func (c *Compiler) ensureCoroLex(name string) (cont, val, tag uint16) {
	if slots, ok := c.coroLex[name]; ok {
		return slots[0], slots[1], slots[2]
	}
	main := c.cuFrame0()
	cont = main.AddLexical("coro_cont_"+name, moargo.RegObj)
	val = main.AddLexical("coro_val_"+name, moargo.RegStr)
	tag = main.AddLexical("coro_tag_"+name, moargo.RegObj)
	c.coroLex[name] = [3]uint16{cont, val, tag}
	return
}

func (c *Compiler) cuFrame0() *moargo.FrameEmitter {
	return c.cu.FrameAt(0)
}

func (c *Compiler) emitYieldHandler(name string, contLex, valLex uint16) int {
	c.lambdaN++
	idx := c.pushFrame("yield_handler_"+name, nil)
	c.frame.EmitOp(moargo.OpCheckArity)
	c.frame.EmitInt16(1)
	c.frame.EmitInt16(1)
	k := uint16(0)
	c.frame.SetLocalType(0, moargo.RegObj)
	c.frame.EmitOp(moargo.OpParamRpO)
	c.frame.EmitReg(k)
	c.frame.EmitInt16(0)
	c.emitBindLex(contLex, 1, k)
	v := uint16(1)
	c.frame.SetLocalType(1, moargo.RegStr)
	c.emitGetLex(v, valLex, 1)
	c.frame.EmitOp(moargo.OpReturnS)
	c.frame.EmitReg(v)
	c.popFrame()
	return idx
}

func (c *Compiler) emitYield(args []Word) error {
	// Requires an enclosing coroutine so we know which lexicals to use.
	// The last registered coro name is the one being compiled.
	name := c.currentCoroName()
	if name == "" {
		return fmt.Errorf("yield called outside of a coroutine")
	}
	contLex, valLex, tagLex := c.ensureCoroLex(name)
	val := c.tempReg()
	if len(args) == 0 {
		c.emitConstS(val, "")
	} else if err := c.emitWordAsStr(args[0], val); err != nil {
		return err
	}
	c.emitBindLex(valLex, 1, val)
	hIdx := c.emitYieldHandler(name, contLex, valLex)
	h := c.tempReg()
	c.frame.SetLocalType(int(h), moargo.RegObj)
	c.frame.EmitGetCode(h, uint16(hIdx))
	hc := c.tempReg()
	c.frame.SetLocalType(int(hc), moargo.RegObj)
	c.frame.EmitOp(moargo.OpTakeClosure)
	c.frame.EmitReg(hc)
	c.frame.EmitReg(h)
	tag := c.tempReg()
	c.frame.SetLocalType(int(tag), moargo.RegObj)
	c.emitGetLex(tag, tagLex, 1)
	z := c.tempReg()
	c.emitConstI(z, 0)
	dst := c.tempReg()
	c.frame.SetLocalType(int(dst), moargo.RegObj)
	c.frame.EmitOp(moargo.OpContinuationControl)
	c.frame.EmitReg(dst)
	c.frame.EmitReg(z)
	c.frame.EmitReg(tag)
	c.frame.EmitReg(hc)
	c.lastReg = dst
	c.lastStr = true
	c.didSay = false
	return nil
}

func (c *Compiler) currentCoroName() string {
	for n := range c.coro {
		return n
	}
	return ""
}

func (c *Compiler) emitCoroutine(args []Word) error {
	if len(args) < 2 {
		return fmt.Errorf("wrong # args: should be \"coroutine name command ?arg ...?\"")
	}
	name := args[0].Inner
	if args[1].Inner != "apply" || len(args) < 3 {
		return fmt.Errorf("coroutine requires apply {arglist body}")
	}
	params, body, err := parseLambdaSpec(args[2].Inner)
	if err != nil {
		return err
	}
	if len(params) != 0 {
		return fmt.Errorf("bytecode coroutine apply must be zero-argument")
	}
	c.coro[name] = true
	// Pre-declare lexicals on the mainline before any bindlex.
	c.ensureCoroLex(name)
	_, _, tagLex := c.ensureCoroLex(name)
	tag := c.tempReg()
	c.frame.SetLocalType(int(tag), moargo.RegObj)
	c.frame.EmitOp(moargo.OpNull)
	c.frame.EmitReg(tag)
	c.emitBindLex(tagLex, 0, tag)
	clos, err := c.emitLambda(params, body)
	if err != nil {
		return err
	}
	dest := c.tempReg()
	c.frame.SetLocalType(int(dest), moargo.RegObj)
	c.frame.EmitOp(moargo.OpContinuationReset)
	c.frame.EmitReg(dest)
	c.frame.EmitReg(tag)
	c.frame.EmitReg(clos)
	out := c.tempReg()
	c.frame.SetLocalType(int(out), moargo.RegStr)
	c.frame.EmitOp(moargo.OpUnboxS)
	c.frame.EmitReg(out)
	c.frame.EmitReg(dest)
	c.lastReg = out
	c.lastStr = true
	c.didSay = false
	return nil
}

func (c *Compiler) emitResumeThunk(arg Word) (uint16, error) {
	c.lambdaN++
	idx := c.pushFrame(fmt.Sprintf("resume_thunk_%d", c.lambdaN), nil)
	c.frame.EmitOp(moargo.OpCheckArity)
	c.frame.EmitInt16(0)
	c.frame.EmitInt16(0)
	r := uint16(0)
	c.frame.SetLocalType(0, moargo.RegStr)
	if err := c.emitWordAsStr(arg, r); err != nil {
		c.popFrame()
		return 0, err
	}
	c.frame.EmitOp(moargo.OpReturnS)
	c.frame.EmitReg(r)
	c.popFrame()
	th := c.tempReg()
	c.frame.SetLocalType(int(th), moargo.RegObj)
	c.frame.EmitGetCode(th, uint16(idx))
	cl := c.tempReg()
	c.frame.SetLocalType(int(cl), moargo.RegObj)
	c.frame.EmitOp(moargo.OpTakeClosure)
	c.frame.EmitReg(cl)
	c.frame.EmitReg(th)
	return cl, nil
}

func (c *Compiler) emitCoroResume(name string, args []Word) error {
	contLex, _, _ := c.ensureCoroLex(name)
	k := c.tempReg()
	c.frame.SetLocalType(int(k), moargo.RegObj)
	c.emitGetLex(k, contLex, 0)
	var arg Word
	if len(args) == 0 {
		arg = Word{Kind: WordBrace, Inner: ""}
	} else {
		arg = args[0]
	}
	th, err := c.emitResumeThunk(arg)
	if err != nil {
		return err
	}
	dest := c.tempReg()
	c.frame.SetLocalType(int(dest), moargo.RegObj)
	c.frame.EmitOp(moargo.OpContinuationInvoke)
	c.frame.EmitReg(dest)
	c.frame.EmitReg(k)
	c.frame.EmitReg(th)
	out := c.tempReg()
	c.frame.SetLocalType(int(out), moargo.RegStr)
	c.frame.EmitOp(moargo.OpUnboxS)
	c.frame.EmitReg(out)
	c.frame.EmitReg(dest)
	c.lastReg = out
	c.lastStr = true
	c.didSay = false
	return nil
}

func (c *Compiler) emitProc(args []Word) error {
	if len(args) != 3 {
		return fmt.Errorf("wrong # args: should be \"proc name args body\"")
	}
	name := args[0].Inner
	params := SplitTclList(args[1].Inner)
	if len(params) == 1 && params[0] == "" {
		params = nil
	}
	body := args[2].Inner
	c.lambdaN++
	idx := c.pushFrame("proc_"+name, params)
	c.procFrame[name] = idx
	n := int16(len(params))
	c.frame.EmitOp(moargo.OpCheckArity)
	c.frame.EmitInt16(n)
	c.frame.EmitInt16(n)
	for i := range params {
		c.frame.EmitOp(moargo.OpParamRpI)
		c.frame.EmitReg(uint16(i))
		c.frame.EmitInt16(int16(i))
	}
	bodyCmds, err := ParseTclAST(body)
	if err != nil {
		c.popFrame()
		return err
	}
	if err := c.emitCommands(bodyCmds); err != nil {
		c.popFrame()
		return err
	}
	if c.lastStr {
		c.frame.EmitOp(moargo.OpReturnS)
		c.frame.EmitReg(c.lastReg)
	} else {
		c.frame.EmitOp(moargo.OpReturnI)
		c.frame.EmitReg(c.lastReg)
	}
	c.popFrame()
	c.lastReg = 0
	c.lastStr = false
	return nil
}

func (c *Compiler) emitCallFrame(idx int, args []Word) error {
	clos := c.tempReg()
	c.frame.SetLocalType(int(clos), moargo.RegObj)
	c.frame.EmitGetCode(clos, uint16(idx))
	flags := []uint8{moargo.CallArgObj}
	regs := []uint16{clos}
	for _, a := range args {
		r := c.tempReg()
		if err := c.emitWord(a, r); err != nil {
			return err
		}
		flags = append(flags, moargo.CallArgInt)
		regs = append(regs, r)
	}
	cs := c.cu.AddCallsite(flags)
	dest := c.tempReg()
	c.frame.EmitDispatchI(dest, "boot-code", cs, regs...)
	c.lastReg = dest
	c.lastStr = false
	c.didSay = false
	return nil
}

func (c *Compiler) emitStringCmd(args []Word) error {
	if len(args) < 2 {
		return fmt.Errorf("wrong # args: string")
	}
	sub := args[0].Inner
	if known, ok := c.knownStr(args[1]); ok {
		switch sub {
		case "length":
			dest := c.tempReg()
			c.emitConstI(dest, int64(len(known)))
			c.lastReg = dest
			c.lastStr = false
			c.didSay = false
			return nil
		case "toupper":
			dest := c.tempReg()
			c.emitConstS(dest, strings.ToUpper(known))
			c.lastReg = dest
			c.lastStr = true
			c.lastList = nil
			c.strNote = strings.ToUpper(known)
			c.didSay = false
			return nil
		case "tolower":
			dest := c.tempReg()
			c.emitConstS(dest, strings.ToLower(known))
			c.lastReg = dest
			c.lastStr = true
			c.strNote = strings.ToLower(known)
			c.didSay = false
			return nil
		case "trim":
			dest := c.tempReg()
			c.emitConstS(dest, strings.TrimSpace(known))
			c.lastReg = dest
			c.lastStr = true
			c.strNote = strings.TrimSpace(known)
			c.didSay = false
			return nil
		case "range":
			if len(args) < 4 {
				return fmt.Errorf("string range needs 3 args")
			}
			first, _ := strconv.Atoi(args[2].Inner)
			last, _ := strconv.Atoi(args[3].Inner)
			runes := []rune(known)
			if first < 0 {
				first = 0
			}
			if last >= len(runes) {
				last = len(runes) - 1
			}
			s := ""
			if first <= last {
				s = string(runes[first : last+1])
			}
			dest := c.tempReg()
			c.emitConstS(dest, s)
			c.lastReg = dest
			c.lastStr = true
			c.strNote = s
			c.didSay = false
			return nil
		case "equal":
			rhs, ok := c.knownStr(args[2])
			if !ok {
				break
			}
			dest := c.tempReg()
			var n int64
			if known == rhs {
				n = 1
			}
			c.emitConstI(dest, n)
			c.lastReg = dest
			c.lastStr = false
			c.didSay = false
			return nil
		}
	}
	src := c.tempReg()
	if err := c.emitWordAsStr(args[1], src); err != nil {
		return err
	}
	switch sub {
	case "length":
		dest := c.tempReg()
		c.frame.EmitOp(moargo.OpChars)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(src)
		c.lastReg = dest
		c.lastStr = false
	case "toupper":
		dest := c.tempReg()
		c.frame.SetLocalType(int(dest), moargo.RegStr)
		c.frame.EmitOp(moargo.OpUC)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(src)
		c.lastReg = dest
		c.lastStr = true
	case "tolower":
		dest := c.tempReg()
		c.frame.SetLocalType(int(dest), moargo.RegStr)
		c.frame.EmitOp(moargo.OpLC)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(src)
		c.lastReg = dest
		c.lastStr = true
	case "range":
		if len(args) < 4 {
			return fmt.Errorf("string range needs 3 args")
		}
		first := c.tempReg()
		last := c.tempReg()
		if err := c.emitWord(args[2], first); err != nil {
			return err
		}
		if err := c.emitWord(args[3], last); err != nil {
			return err
		}
		// substr_s dest, src, start, length — convert last to length
		one := c.tempReg()
		c.emitConstI(one, 1)
		ln := c.tempReg()
		c.frame.EmitOp(moargo.OpSubI)
		c.frame.EmitReg(ln)
		c.frame.EmitReg(last)
		c.frame.EmitReg(first)
		c.frame.EmitOp(moargo.OpAddI)
		c.frame.EmitReg(ln)
		c.frame.EmitReg(ln)
		c.frame.EmitReg(one)
		dest := c.tempReg()
		c.frame.SetLocalType(int(dest), moargo.RegStr)
		c.frame.EmitOp(moargo.OpSubstrS)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(src)
		c.frame.EmitReg(first)
		c.frame.EmitReg(ln)
		c.lastReg = dest
		c.lastStr = true
	case "equal":
		if len(args) < 3 {
			return fmt.Errorf("string equal needs 2 strings")
		}
		rhs := c.tempReg()
		if err := c.emitWordAsStr(args[2], rhs); err != nil {
			return err
		}
		dest := c.tempReg()
		c.frame.EmitOp(moargo.OpEqS)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(src)
		c.frame.EmitReg(rhs)
		c.lastReg = dest
		c.lastStr = false
	case "trim":
		if known, ok := c.knownStr(args[1]); ok {
			dest := c.tempReg()
			c.emitConstS(dest, strings.TrimSpace(known))
			c.lastReg = dest
			c.lastStr = true
			break
		}
		c.lastReg = src
		c.lastStr = true
	default:
		return fmt.Errorf("string %s cannot be compiled to MoarVM bytecode", sub)
	}
	c.didSay = false
	return nil
}

func (c *Compiler) emitForeach(args []Word) error {
	if len(args) != 3 {
		return fmt.Errorf("wrong # args: should be \"foreach varName list body\"")
	}
	items := SplitTclList(args[1].Inner)
	for _, item := range items {
		reg := c.allocReg(args[0].Inner)
		c.emitConstS(reg, item)
		c.varType[args[0].Inner] = moargo.RegStr
		c.strConst[args[0].Inner] = item
		bodyCmds, err := ParseTclAST(args[2].Inner)
		if err != nil {
			return err
		}
		if err := c.emitCommands(bodyCmds); err != nil {
			return err
		}
	}
	return nil
}

func (c *Compiler) emitSwitch(args []Word) error {
	if len(args) < 2 {
		return fmt.Errorf("wrong # args: switch")
	}
	valR := c.tempReg()
	if err := c.emitWordAsStr(args[0], valR); err != nil {
		return err
	}
	arms := SplitTclList(args[len(args)-1].Inner)
	var endPatches []int32
	for i := 0; i+1 < len(arms); i += 2 {
		pat := arms[i]
		body := arms[i+1]
		var skip int32 = -1
		if pat != "default" {
			patR := c.tempReg()
			c.emitConstS(patR, pat)
			eq := c.tempReg()
			c.frame.EmitOp(moargo.OpEqS)
			c.frame.EmitReg(eq)
			c.frame.EmitReg(valR)
			c.frame.EmitReg(patR)
			c.frame.EmitOp(moargo.OpUnlessI)
			c.frame.EmitReg(eq)
			skip = c.frame.CurrentOffset()
			c.frame.EmitInt32(0)
		}
		bodyCmds, err := ParseTclAST(body)
		if err != nil {
			return err
		}
		if err := c.emitCommands(bodyCmds); err != nil {
			return err
		}
		c.frame.EmitOp(moargo.OpGoto)
		endPatches = append(endPatches, c.frame.CurrentOffset())
		c.frame.EmitInt32(0)
		if skip >= 0 {
			c.patchU32(skip, uint32(c.frame.CurrentOffset()))
		}
	}
	end := uint32(c.frame.CurrentOffset())
	for _, p := range endPatches {
		c.patchU32(p, end)
	}
	return nil
}

func (c *Compiler) emitLappend(args []Word) error {
	if len(args) < 2 {
		return fmt.Errorf("wrong # args: lappend")
	}
	dest := c.allocReg(args[0].Inner)
	for i := 1; i < len(args); i++ {
		sp := c.tempReg()
		c.emitConstS(sp, " ")
		elem := c.tempReg()
		if err := c.emitWordAsStr(args[i], elem); err != nil {
			return err
		}
		tmp := c.tempReg()
		c.frame.SetLocalType(int(tmp), moargo.RegStr)
		c.frame.SetLocalType(int(dest), moargo.RegStr)
		c.frame.EmitOp(moargo.OpConcatS)
		c.frame.EmitReg(tmp)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(sp)
		c.frame.EmitOp(moargo.OpConcatS)
		c.frame.EmitReg(dest)
		c.frame.EmitReg(tmp)
		c.frame.EmitReg(elem)
	}
	if items, ok := c.lists[args[0].Inner]; ok {
		for i := 1; i < len(args); i++ {
			items = append(items, args[i].Inner)
		}
		c.lists[args[0].Inner] = items
	}
	c.lastReg = dest
	c.lastStr = true
	c.didSay = false
	return nil
}

func (c *Compiler) emitLrange(args []Word) error {
	if len(args) != 3 {
		return fmt.Errorf("wrong # args: lrange")
	}
	if items, ok := c.knownList(args[0]); ok {
		first, _ := strconv.Atoi(args[1].Inner)
		last, _ := strconv.Atoi(args[2].Inner)
		if first < 0 {
			first = 0
		}
		if last >= len(items) {
			last = len(items) - 1
		}
		var out []string
		if first <= last && first < len(items) {
			out = items[first : last+1]
		}
		dest := c.tempReg()
		c.emitConstS(dest, strings.Join(out, " "))
		c.lastReg = dest
		c.lastStr = true
		c.didSay = false
		return nil
	}
	return fmt.Errorf("lrange of variable cannot be compiled")
}

func (c *Compiler) emitConstI(dest uint16, v int64) {
	c.frame.EmitOp(moargo.OpConstI64)
	c.frame.EmitReg(dest)
	c.frame.EmitInt64(v)
}

func (c *Compiler) emitConstS(dest uint16, s string) {
	c.frame.SetLocalType(int(dest), moargo.RegStr)
	c.frame.EmitOp(moargo.OpConstS)
	c.frame.EmitReg(dest)
	c.frame.EmitString(s)
}

func (c *Compiler) patchU32(offset int32, val uint32) {
	b := c.frame.Bytecode.Bytes()
	b[offset] = byte(val)
	b[offset+1] = byte(val >> 8)
	b[offset+2] = byte(val >> 16)
	b[offset+3] = byte(val >> 24)
}

// CompileAndRun compiles a Tcl script to MoarVM bytecode and executes it on the VM.
func CompileAndRun(ctx context.Context, vm moargo.Engine, script string) error {
	compiler := NewCompiler()
	bc, err := compiler.CompileScript(script)
	if err != nil {
		return fmt.Errorf("tcl compilation to MoarVM bytecode failed: %w", err)
	}
	return vm.RunBytecode(ctx, bc)
}

// WriteCompUnit writes compiled bytecode to a .moarvm file.
func WriteCompUnit(path string, bytecode []byte) error {
	return os.WriteFile(path, bytecode, 0o644)
}
