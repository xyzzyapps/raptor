package raptor

import "strings"

func isFastSlotCandidate(name string) bool {
	if len(name) < 2 {
		return false
	}
	sigil := name[0]
	if sigil != '$' && sigil != '@' && sigil != '%' {
		return false
	}
	c1 := name[1]
	if c1 == '*' || c1 == '?' {
		return false
	}
	if strings.Contains(name, "::") {
		return false
	}
	switch name {
	case "$_", "$/", "$!", "$?", "$0", "$AUTOLOAD", "@ARGV", "%ENV":
		return false
	}
	return true
}

type hazardScanner struct {
	nonFastNames  map[string]bool
	arrayBases    map[string]bool
	hashBases     map[string]bool
	scalarNames   map[string]bool
	nestedDepth   int
	inPackageDecl bool
}

func newHazardScanner() *hazardScanner {
	return &hazardScanner{
		nonFastNames: make(map[string]bool),
		arrayBases:   make(map[string]bool),
		hashBases:    make(map[string]bool),
		scalarNames:  make(map[string]bool),
	}
}

func (hs *hazardScanner) scanParams(params []Param) {
	for _, p := range params {
		if hs.nestedDepth > 0 && p.Name != "" {
			// Inner closure parameter itself is fine inside inner closure, don't mark outer unless referenced.
		}
		if p.Where != nil || len(p.DestructArr) > 0 || len(p.DestructHash) > 0 || p.IsSlurpy {
			if p.Name != "" {
				hs.nonFastNames[p.Name] = true
			}
			for _, dp := range p.DestructArr {
				if dp.Name != "" {
					hs.nonFastNames[dp.Name] = true
				}
			}
			for _, hp := range p.DestructHash {
				if hp.Name != "" {
					hs.nonFastNames[hp.Name] = true
				}
			}
		}
		if p.Where != nil {
			hs.nestedDepth++
			hs.scanExpr(p.Where)
			hs.nestedDepth--
		}
		if strings.HasPrefix(p.Name, "@") && len(p.Name) > 1 {
			hs.arrayBases[p.Name[1:]] = true
		} else if strings.HasPrefix(p.Name, "%") && len(p.Name) > 1 {
			hs.hashBases[p.Name[1:]] = true
		}
	}
}

func (hs *hazardScanner) finalizeCrossSigils() {
	for base := range hs.arrayBases {
		if hs.scalarNames["$"+base] {
			hs.nonFastNames["@"+base] = true
			hs.nonFastNames["$"+base] = true
		}
	}
	for base := range hs.hashBases {
		if hs.scalarNames["$"+base] {
			hs.nonFastNames["%"+base] = true
			hs.nonFastNames["$"+base] = true
		}
	}
}

func (hs *hazardScanner) recordDeclName(name string) {
	if strings.HasPrefix(name, "@") && len(name) > 1 {
		hs.arrayBases[name[1:]] = true
	} else if strings.HasPrefix(name, "%") && len(name) > 1 {
		hs.hashBases[name[1:]] = true
	}
	if hs.inPackageDecl {
		hs.nonFastNames[name] = true
	}
}

func (hs *hazardScanner) scanStmts(stmts []Stmt) {
	for _, s := range stmts {
		hs.scanStmt(s)
	}
}

func (hs *hazardScanner) scanBlock(b *BlockStmt) {
	if b == nil {
		return
	}
	hs.scanStmts(b.Stmts)
}

func (hs *hazardScanner) scanStmt(stmt Stmt) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *BlockStmt:
		hs.scanBlock(s)
	case *VarDeclStmt:
		hs.recordDeclName(s.Name)
		if s.Scope != "my" || s.Type != "" || s.Where != nil || hs.nestedDepth > 0 {
			if s.Scope != "my" || s.Type != "" || s.Where != nil {
				hs.nonFastNames[s.Name] = true
			}
		}
		if s.Value != nil {
			hs.scanExpr(s.Value)
		}
		if s.Where != nil {
			hs.nestedDepth++
			hs.scanExpr(s.Where)
			hs.nestedDepth--
		}
	case *PackageDeclStmt:
		prev := hs.inPackageDecl
		hs.inPackageDecl = true
		hs.scanBlock(s.Body)
		hs.inPackageDecl = prev
	case *AssignStmt:
		if s.IsMyDecl {
			if ve, ok := s.Target.(*VarExpr); ok {
				hs.recordDeclName(ve.Name)
			}
		}
		hs.scanExpr(s.Target)
		hs.scanExpr(s.Value)
	case *IfStmt:
		hs.scanExpr(s.Condition)
		hs.scanBlock(s.ThenBranch)
		for i, c := range s.ElsifConds {
			hs.scanExpr(c)
			if i < len(s.ElsifThen) {
				hs.scanBlock(s.ElsifThen[i])
			}
		}
		hs.scanBlock(s.ElseBranch)
	case *UnlessStmt:
		hs.scanExpr(s.Condition)
		hs.scanBlock(s.Body)
	case *WhileStmt:
		hs.scanExpr(s.Condition)
		hs.scanBlock(s.Body)
	case *ForStmt:
		if s.VarName != "" {
			hs.recordDeclName(s.VarName)
		}
		hs.scanExpr(s.Iterable)
		hs.scanBlock(s.Body)
	case *LoopStmt:
		hs.scanExpr(s.Init)
		hs.scanExpr(s.Cond)
		hs.scanExpr(s.Step)
		hs.scanBlock(s.Body)
	case *SubsetDeclStmt:
		hs.nestedDepth++
		hs.scanExpr(s.Where)
		hs.nestedDepth--
	case *SubDeclStmt:
		hs.nestedDepth++
		hs.scanParams(s.Params)
		hs.scanBlock(s.Body)
		hs.nestedDepth--
	case *ReturnStmt:
		hs.scanExpr(s.Value)
	case *ExprStmt:
		hs.scanExpr(s.Expr)
	case *GivenStmt:
		hs.scanExpr(s.Topic)
		for _, w := range s.Whens {
			hs.scanExpr(w.Match)
			hs.scanBlock(w.Body)
		}
		hs.scanBlock(s.Default)
	case *AssertStmt:
		hs.scanExpr(s.Condition)
		hs.scanExpr(s.Message)
	case *VerifyStmt:
		hs.scanExpr(s.Name)
		hs.scanExpr(s.Cond)
		hs.scanExpr(s.Message)
		if s.Body != nil {
			hs.nestedDepth++
			hs.scanParams(s.Params)
			hs.scanBlock(s.Body)
			hs.nestedDepth--
		}
	case *AdviceHookStmt:
		hs.nestedDepth++
		hs.scanParams(s.Params)
		hs.scanBlock(s.Body)
		hs.nestedDepth--
	case *ModifierStmt:
		hs.scanExpr(s.Condition)
		hs.scanStmt(s.Target)
	case *LabelStmt:
		hs.scanStmt(s.Stmt)
	case *TakeStmt:
		hs.scanExpr(s.Value)
	}
}

func (hs *hazardScanner) scanExpr(expr Expr) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *VarExpr:
		if strings.HasPrefix(e.Name, "$") && len(e.Name) > 1 {
			hs.scalarNames[e.Name] = true
		}
		if hs.nestedDepth > 0 {
			hs.nonFastNames[e.Name] = true
		}
	case *BinaryExpr:
		hs.scanExpr(e.Left)
		hs.scanExpr(e.Right)
	case *UnaryExpr:
		hs.scanExpr(e.Right)
	case *TernaryExpr:
		hs.scanExpr(e.Cond)
		hs.scanExpr(e.Then)
		hs.scanExpr(e.Else)
	case *CallExpr:
		hs.scanExpr(e.Callee)
		for _, a := range e.Args {
			hs.scanExpr(a)
		}
	case *MethodCallExpr:
		hs.scanExpr(e.Target)
		for _, a := range e.Args {
			hs.scanExpr(a)
		}
	case *ArrayLiteralExpr:
		for _, el := range e.Elements {
			hs.scanExpr(el)
		}
	case *HashLiteralExpr:
		for _, p := range e.Pairs {
			hs.scanExpr(p[0])
			hs.scanExpr(p[1])
		}
	case *IndexExpr:
		hs.scanExpr(e.Array)
		hs.scanExpr(e.Index)
	case *HashAccessExpr:
		hs.scanExpr(e.Hash)
		hs.scanExpr(e.Key)
	case *ClosureExpr:
		hs.nestedDepth++
		hs.scanParams(e.Params)
		hs.scanBlock(e.Body)
		hs.nestedDepth--
	case *SmartMatchExpr:
		hs.scanExpr(e.Left)
		hs.scanExpr(e.Right)
	case *ChainedCompExpr:
		for _, ex := range e.Exprs {
			hs.scanExpr(ex)
		}
	case *InterpStringExpr:
		for _, pt := range e.Parts {
			hs.scanExpr(pt)
		}
	case *GatherExpr:
		hs.scanBlock(e.Body)
	case *AssignStmt:
		hs.scanStmt(e)
	case *RefExpr:
		if ve, ok := e.Expr.(*VarExpr); ok {
			hs.nonFastNames[ve.Name] = true
		}
		hs.scanExpr(e.Expr)
	case *DerefExpr:
		hs.scanExpr(e.Ref)
		hs.scanExpr(e.Index)
		for _, a := range e.Args {
			hs.scanExpr(a)
		}
	case *BacktickExpr:
		hs.scanExpr(e.Command)
	}
}

type slotBinding struct {
	kind SlotKind
	slot int
}

type slotResolver struct {
	in            *Interp
	isFunc        bool
	nonFastNames  map[string]bool
	scopes        []map[string]slotBinding
	topSubSlots   map[string]int
	localSlotCnt  int
	inPackageDecl bool
}

func (sr *slotResolver) pushScope() {
	sr.scopes = append(sr.scopes, make(map[string]slotBinding, 4))
}

func (sr *slotResolver) popScope() {
	sr.scopes = sr.scopes[:len(sr.scopes)-1]
}

func (sr *slotResolver) lookup(name string) (slotBinding, bool) {
	for i := len(sr.scopes) - 1; i >= 0; i-- {
		if b, ok := sr.scopes[i][name]; ok {
			return b, true
		}
	}
	return slotBinding{}, false
}

func (sr *slotResolver) declareShadowNone(name string) {
	if name == "" {
		return
	}
	sr.scopes[len(sr.scopes)-1][name] = slotBinding{kind: SlotNone, slot: -1}
}

func (sr *slotResolver) allocSlot(name string) slotBinding {
	if !isFastSlotCandidate(name) || sr.nonFastNames[name] || sr.inPackageDecl {
		b := slotBinding{kind: SlotNone, slot: -1}
		sr.scopes[len(sr.scopes)-1][name] = b
		return b
	}
	if sr.isFunc {
		idx := sr.localSlotCnt
		sr.localSlotCnt++
		b := slotBinding{kind: SlotLocal, slot: idx}
		sr.scopes[len(sr.scopes)-1][name] = b
		return b
	}
	var idx int
	if len(sr.scopes) == 1 {
		idx = sr.in.globals.ensureSlot(name, sr.in.GlobalEnv)
	} else {
		idx = sr.in.globals.allocScopedSlot()
	}
	b := slotBinding{kind: SlotGlobal, slot: idx}
	sr.scopes[len(sr.scopes)-1][name] = b
	return b
}

// ResolveProgram performs compile-time lexical slot indexing and scope escape analysis.
func (in *Interp) ResolveProgram(prog *Program, isGlobalEnv bool) {
	if prog == nil || prog.Resolved {
		return
	}
	prog.Resolved = true
	if !isGlobalEnv {
		// Dynamic eval inside a non-global Env: leave top-level as SlotNone, still resolve inner functions.
		sr := &slotResolver{
			in:           in,
			isFunc:       false,
			nonFastNames: make(map[string]bool),
			topSubSlots:  make(map[string]int),
		}
		sr.inPackageDecl = true // disables top-level SlotGlobal while still resolving SubDeclStmt/ClosureExpr
		sr.pushScope()
		sr.resolveStmts(prog.Stmts)
		sr.popScope()
		return
	}

	// Synchronize any previously allocated global slots with GlobalEnv in case host Go code
	// or a dynamic/closure-bearing REPL snippet mutated GlobalEnv.bindings directly.
	for name, slot := range in.globals.slotMap {
		if cur, ok := in.GlobalEnv.bindings[name]; ok && cur != nil && cur != in.globals.get(slot) {
			in.globals.setSlot(slot, cur, in.GlobalEnv)
		}
	}

	hs := newHazardScanner()
	for name := range in.globals.slotMap {
		if strings.HasPrefix(name, "@") && len(name) > 1 {
			hs.arrayBases[name[1:]] = true
		} else if strings.HasPrefix(name, "%") && len(name) > 1 {
			hs.hashBases[name[1:]] = true
		}
		if (in.GlobalEnv.types != nil && in.GlobalEnv.types[name] != "") ||
			(in.GlobalEnv.wheres != nil && in.GlobalEnv.wheres[name] != nil) ||
			(in.GlobalEnv.stateBindings != nil && in.GlobalEnv.stateBindings[name] != nil) {
			hs.nonFastNames[name] = true
		}
	}
	hs.scanStmts(prog.Stmts)
	hs.finalizeCrossSigils()

	topSubSlots := make(map[string]int)
	hasPkgOrNative := make(map[string]bool)
	for _, st := range prog.Stmts {
		switch s := st.(type) {
		case *NativeSubDeclStmt:
			norm := normalizeOpName(s.Name)
			hasPkgOrNative[norm] = true
		case *PackageDeclStmt:
			// Leave subs declared inside packages off direct GlobalSubSlot.
		}
	}
	for name, slot := range in.globals.slotMap {
		if len(name) > 0 && name[0] != '$' && name[0] != '@' && name[0] != '%' && name[0] != '&' {
			if _, isB := in.Builtins[name]; !isB && !hasPkgOrNative[name] && !strings.Contains(name, "::") {
				topSubSlots[name] = slot
			}
		}
	}
	for _, st := range prog.Stmts {
		if s, ok := st.(*SubDeclStmt); ok {
			norm := normalizeOpName(s.Name)
			if _, isB := in.Builtins[norm]; !isB && !hasPkgOrNative[norm] && !strings.Contains(norm, "::") {
				slot := in.globals.ensureSlot(norm, in.GlobalEnv)
				topSubSlots[norm] = slot
				s.GlobalSlot = slot
			} else {
				s.GlobalSlot = -1
			}
		}
	}

	sr := &slotResolver{
		in:           in,
		isFunc:       false,
		nonFastNames: hs.nonFastNames,
		topSubSlots:  topSubSlots,
	}
	sr.pushScope()
	for name, slot := range in.globals.slotMap {
		if isFastSlotCandidate(name) && !hs.nonFastNames[name] {
			if _, ok := in.GlobalEnv.bindings[name]; ok {
				sr.scopes[0][name] = slotBinding{kind: SlotGlobal, slot: slot}
			}
		}
	}
	sr.resolveStmts(prog.Stmts)
	sr.popScope()
}

func (in *Interp) resolveFunction(params []Param, body *BlockStmt, topSubSlots map[string]int) (frameSlots int, paramSlots []int, needsEnv bool, fastSimple bool, noParamChecks bool) {
	hs := newHazardScanner()
	hs.scanParams(params)
	hs.scanBlock(body)
	hs.finalizeCrossSigils()

	sr := &slotResolver{
		in:           in,
		isFunc:       true,
		nonFastNames: hs.nonFastNames,
		topSubSlots:  topSubSlots,
	}
	sr.pushScope()

	paramSlots = make([]int, len(params))
	allParamsFast := true
	noChecks := true
	for i, p := range params {
		if p.Type != "" {
			noChecks = false
		}
		if p.Where != nil || len(p.DestructArr) > 0 || len(p.DestructHash) > 0 || p.IsSlurpy || p.Name == "" {
			paramSlots[i] = -1
			allParamsFast = false
			if p.Name != "" {
				sr.declareShadowNone(p.Name)
			}
			for _, dp := range p.DestructArr {
				sr.declareShadowNone(dp.Name)
			}
			for _, hp := range p.DestructHash {
				sr.declareShadowNone(hp.Name)
			}
			if p.Where != nil {
				sr.resolveExpr(p.Where)
			}
			continue
		}
		b := sr.allocSlot(p.Name)
		if b.kind == SlotLocal {
			paramSlots[i] = b.slot
		} else {
			paramSlots[i] = -1
			allParamsFast = false
		}
	}

	if body != nil {
		sr.resolveBlockInCurrentScope(body)
	}
	sr.popScope()

	frameSlots = sr.localSlotCnt
	bodyEnvNeeded := false
	if body != nil {
		bodyEnvNeeded = blockTouchesEnv(body)
	}
	needsEnv = !allParamsFast || bodyEnvNeeded
	fastSimple = allParamsFast && !needsEnv
	noParamChecks = noChecks
	return
}

func (sr *slotResolver) resolveBlock(b *BlockStmt) {
	if b == nil {
		return
	}
	sr.pushScope()
	sr.resolveBlockInCurrentScope(b)
	sr.popScope()
}

func (sr *slotResolver) resolveBlockInCurrentScope(b *BlockStmt) {
	if b == nil {
		return
	}
	b.Resolved = true
	b.HasLabels = false
	for _, st := range b.Stmts {
		if _, ok := st.(*LabelStmt); ok {
			b.HasLabels = true
			break
		}
	}
	sr.resolveStmts(b.Stmts)
	b.NeedsBlockEnv = blockTouchesEnv(b)
}

func (sr *slotResolver) resolveStmts(stmts []Stmt) {
	for _, st := range stmts {
		sr.resolveStmt(st)
	}
}

func (sr *slotResolver) resolveStmt(stmt Stmt) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *BlockStmt:
		sr.resolveBlock(s)

	case *VarDeclStmt:
		if s.Value != nil {
			sr.resolveExpr(s.Value)
		}
		if s.Scope == "my" && s.Type == "" && s.Where == nil {
			b := sr.allocSlot(s.Name)
			s.SlotKind = b.kind
			s.Slot = b.slot
		} else {
			s.SlotKind = SlotNone
			s.Slot = -1
			sr.declareShadowNone(s.Name)
			if s.Where != nil {
				sr.pushScope()
				sr.declareShadowNone("$_")
				sr.declareShadowNone(s.Name)
				sr.resolveExpr(s.Where)
				sr.popScope()
			}
		}

	case *PackageDeclStmt:
		prev := sr.inPackageDecl
		sr.inPackageDecl = true
		sr.resolveBlock(s.Body)
		sr.inPackageDecl = prev

	case *AssignStmt:
		if s.IsMyDecl {
			if ve, ok := s.Target.(*VarExpr); ok {
				sr.resolveExpr(s.Value)
				b := sr.allocSlot(ve.Name)
				ve.SlotKind = b.kind
				ve.Slot = b.slot
				return
			}
		}
		sr.resolveExpr(s.Value)
		sr.resolveExpr(s.Target)

	case *IfStmt:
		sr.resolveExpr(s.Condition)
		sr.resolveBlock(s.ThenBranch)
		for i, c := range s.ElsifConds {
			sr.resolveExpr(c)
			if i < len(s.ElsifThen) {
				sr.resolveBlock(s.ElsifThen[i])
			}
		}
		sr.resolveBlock(s.ElseBranch)

	case *UnlessStmt:
		sr.resolveExpr(s.Condition)
		sr.resolveBlock(s.Body)

	case *WhileStmt:
		sr.resolveExpr(s.Condition)
		sr.resolveBlock(s.Body)
		s.NeedsLoopEnv = s.Body != nil && s.Body.NeedsBlockEnv

	case *ForStmt:
		sr.resolveExpr(s.Iterable)
		sr.pushScope()
		if s.VarName != "" {
			b := sr.allocSlot(s.VarName)
			s.VarSlotKind = b.kind
			s.VarSlot = b.slot
		} else {
			s.VarSlotKind = SlotNone
			s.VarSlot = -1
		}
		sr.resolveBlockInCurrentScope(s.Body)
		sr.popScope()
		s.UsesTopic = blockUsesTopic(s.Body)
		s.NeedsLoopEnv = s.UsesTopic || (s.VarName != "" && s.VarSlotKind == SlotNone) || (s.Body != nil && s.Body.NeedsBlockEnv)

	case *LoopStmt:
		sr.pushScope()
		sr.resolveExpr(s.Init)
		sr.resolveExpr(s.Cond)
		sr.resolveBlock(s.Body)
		sr.resolveExpr(s.Step)
		sr.popScope()
		initTouches := exprTouchesEnv(s.Init) || exprTouchesEnv(s.Cond) || exprTouchesEnv(s.Step)
		s.NeedsBodyEnv = s.Body != nil && s.Body.NeedsBlockEnv
		s.NeedsLoopEnv = initTouches || s.NeedsBodyEnv

	case *SubsetDeclStmt:
		sr.declareShadowNone(s.Name)
		if cExpr, ok := s.Where.(*ClosureExpr); ok {
			sr.resolveExpr(cExpr)
		} else if s.Where != nil {
			sr.pushScope()
			sr.declareShadowNone("$_")
			sr.resolveExpr(s.Where)
			sr.popScope()
		}

	case *SubDeclStmt:
		if len(sr.scopes) > 1 || sr.isFunc || sr.inPackageDecl {
			s.GlobalSlot = -1
			norm := normalizeOpName(s.Name)
			sr.declareShadowNone(norm)
			sr.declareShadowNone(s.Name)
		}
		s.FrameSlots, s.ParamSlots, s.NeedsEnv, s.FastSimple, s.NoParamChecks = sr.in.resolveFunction(s.Params, s.Body, sr.topSubSlots)

	case *NativeSubDeclStmt:
		norm := normalizeOpName(s.Name)
		sr.declareShadowNone(norm)

	case *CStructDeclStmt:
		sr.declareShadowNone(s.Name)

	case *GrammarDeclStmt:
		sr.declareShadowNone(s.Name)

	case *EnumDeclStmt:
		for _, v := range s.Values {
			sr.declareShadowNone(v.Name)
		}

	case *ReturnStmt:
		sr.resolveExpr(s.Value)

	case *ExprStmt:
		sr.resolveExpr(s.Expr)

	case *GivenStmt:
		sr.resolveExpr(s.Topic)
		sr.pushScope()
		sr.declareShadowNone("$_")
		for _, w := range s.Whens {
			sr.resolveExpr(w.Match)
			sr.resolveBlock(w.Body)
		}
		sr.resolveBlock(s.Default)
		sr.popScope()

	case *AssertStmt:
		sr.resolveExpr(s.Condition)
		sr.resolveExpr(s.Message)

	case *VerifyStmt:
		sr.resolveExpr(s.Name)
		sr.resolveExpr(s.Cond)
		sr.resolveExpr(s.Message)
		if s.Body != nil {
			sr.pushScope()
			for _, p := range s.Params {
				sr.declareShadowNone(p.Name)
			}
			sr.resolveBlockInCurrentScope(s.Body)
			sr.popScope()
		}

	case *AdviceHookStmt:
		sr.pushScope()
		for _, p := range s.Params {
			sr.declareShadowNone(p.Name)
		}
		sr.resolveBlockInCurrentScope(s.Body)
		sr.popScope()

	case *ModifierStmt:
		sr.resolveExpr(s.Condition)
		if s.Kind == ModFor || s.Kind == ModGiven {
			sr.pushScope()
			if s.VarName != "" {
				sr.declareShadowNone(s.VarName)
			}
			sr.declareShadowNone("$_")
			sr.resolveStmt(s.Target)
			sr.popScope()
		} else {
			sr.resolveStmt(s.Target)
		}

	case *LabelStmt:
		sr.resolveStmt(s.Stmt)

	case *TakeStmt:
		sr.resolveExpr(s.Value)
	}
}

func (sr *slotResolver) resolveExpr(expr Expr) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *VarExpr:
		if b, ok := sr.lookup(e.Name); ok {
			e.SlotKind = b.kind
			e.Slot = b.slot
		} else {
			e.SlotKind = SlotNone
			e.Slot = -1
		}

	case *BinaryExpr:
		sr.resolveExpr(e.Left)
		sr.resolveExpr(e.Right)

	case *UnaryExpr:
		sr.resolveExpr(e.Right)

	case *TernaryExpr:
		sr.resolveExpr(e.Cond)
		sr.resolveExpr(e.Then)
		sr.resolveExpr(e.Else)

	case *CallExpr:
		e.GlobalSubSlot = -1
		sr.resolveExpr(e.Callee)
		if ve, ok := e.Callee.(*VarExpr); ok && ve.SlotKind == SlotNone {
			if b, shadowed := sr.lookup(ve.Name); !shadowed || b.kind == SlotNone {
				if !shadowed {
					if gSlot, exists := sr.topSubSlots[ve.Name]; exists {
						e.GlobalSubSlot = gSlot
					}
				}
			}
		}
		for _, a := range e.Args {
			sr.resolveExpr(a)
		}

	case *MethodCallExpr:
		sr.resolveExpr(e.Target)
		for _, a := range e.Args {
			sr.resolveExpr(a)
		}

	case *ArrayLiteralExpr:
		for _, el := range e.Elements {
			sr.resolveExpr(el)
		}

	case *HashLiteralExpr:
		for _, p := range e.Pairs {
			sr.resolveExpr(p[0])
			sr.resolveExpr(p[1])
		}

	case *IndexExpr:
		sr.resolveExpr(e.Array)
		sr.resolveExpr(e.Index)

	case *HashAccessExpr:
		sr.resolveExpr(e.Hash)
		sr.resolveExpr(e.Key)

	case *ClosureExpr:
		e.FrameSlots, e.ParamSlots, e.NeedsEnv, e.FastSimple, e.NoParamChecks = sr.in.resolveFunction(e.Params, e.Body, sr.topSubSlots)

	case *SmartMatchExpr:
		sr.resolveExpr(e.Left)
		sr.resolveExpr(e.Right)

	case *ChainedCompExpr:
		for _, ex := range e.Exprs {
			sr.resolveExpr(ex)
		}

	case *InterpStringExpr:
		for _, pt := range e.Parts {
			sr.resolveExpr(pt)
		}

	case *GatherExpr:
		sr.resolveBlock(e.Body)

	case *AssignStmt:
		sr.resolveStmt(e)

	case *RefExpr:
		sr.resolveExpr(e.Expr)

	case *DerefExpr:
		sr.resolveExpr(e.Ref)
		sr.resolveExpr(e.Index)
		for _, a := range e.Args {
			sr.resolveExpr(a)
		}

	case *BacktickExpr:
		sr.resolveExpr(e.Command)
	}
}

// blockTouchesEnv returns true if executing b may bind or mutate entries in a lexical *Env map.
func blockTouchesEnv(b *BlockStmt) bool {
	if b == nil {
		return false
	}
	for _, st := range b.Stmts {
		if stmtTouchesEnv(st) {
			return true
		}
	}
	return false
}

func stmtTouchesEnv(stmt Stmt) bool {
	if stmt == nil {
		return false
	}
	switch s := stmt.(type) {
	case *BlockStmt:
		return blockTouchesEnv(s)
	case *VarDeclStmt:
		if s.SlotKind == SlotNone {
			return true
		}
		return exprTouchesEnv(s.Value)
	case *AssignStmt:
		if ve, ok := s.Target.(*VarExpr); ok {
			if ve.SlotKind == SlotNone {
				return true
			}
			return exprTouchesEnv(s.Value)
		}
		return exprTouchesEnv(s.Target) || exprTouchesEnv(s.Value)
	case *IfStmt:
		if exprTouchesEnv(s.Condition) || blockTouchesEnv(s.ThenBranch) || blockTouchesEnv(s.ElseBranch) {
			return true
		}
		for i, c := range s.ElsifConds {
			if exprTouchesEnv(c) {
				return true
			}
			if i < len(s.ElsifThen) && blockTouchesEnv(s.ElsifThen[i]) {
				return true
			}
		}
		return false
	case *UnlessStmt:
		return exprTouchesEnv(s.Condition) || blockTouchesEnv(s.Body)
	case *WhileStmt:
		return exprTouchesEnv(s.Condition) || blockTouchesEnv(s.Body)
	case *ForStmt:
		if s.NeedsLoopEnv || exprTouchesEnv(s.Iterable) {
			return true
		}
		return false
	case *LoopStmt:
		return s.NeedsLoopEnv
	case *ReturnStmt:
		return exprTouchesEnv(s.Value)
	case *BreakStmt, *ContinueStmt, *RedoStmt:
		return false
	case *ExprStmt:
		return exprTouchesEnv(s.Expr)
	case *TakeStmt:
		return exprTouchesEnv(s.Value)
	case *ModifierStmt:
		if s.Kind == ModFor || s.Kind == ModGiven {
			return true
		}
		return exprTouchesEnv(s.Condition) || stmtTouchesEnv(s.Target)
	case *LabelStmt:
		return stmtTouchesEnv(s.Stmt)
	case *AssertStmt:
		return exprTouchesEnv(s.Condition) || exprTouchesEnv(s.Message)
	default:
		// SubDeclStmt, PackageDeclStmt, UseStmt, GivenStmt, EnumDeclStmt, GrammarDeclStmt, etc.
		return true
	}
}

func exprTouchesEnv(expr Expr) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *LiteralExpr, *VarExpr, *StubExpr:
		return false
	case *ClosureExpr:
		// Creating a closure captures env! If a block creates a closure, it needs its own Env if any outer vars were SlotNone.
		return true
	case *BinaryExpr:
		if e.Op == "~~" || e.Op == "!~" {
			return true // regex match writes $/
		}
		return exprTouchesEnv(e.Left) || exprTouchesEnv(e.Right)
	case *UnaryExpr:
		return exprTouchesEnv(e.Right)
	case *TernaryExpr:
		return exprTouchesEnv(e.Cond) || exprTouchesEnv(e.Then) || exprTouchesEnv(e.Else)
	case *CallExpr:
		// If callee is unresolved and not a known GlobalSubSlot or Builtin, AUTOLOAD could write $AUTOLOAD.
		if e.GlobalSubSlot < 0 {
			if ve, ok := e.Callee.(*VarExpr); ok && ve.SlotKind == SlotNone {
				if _, isB := eBuiltinCheck(ve.Name, expr); !isB {
					// Still safe unless AUTOLOAD exists, check at runtime or mark conservatively only if needed:
					// Note: AUTOLOAD is checked via GlobalEnv/Packages; normal calls don't mutate caller's env.
				}
			}
		}
		if exprTouchesEnv(e.Callee) {
			return true
		}
		for _, a := range e.Args {
			if exprTouchesEnv(a) {
				return true
			}
		}
		return false
	case *MethodCallExpr:
		if e.Method == "parse" || e.Method == "subparse" {
			return true // grammar parse writes $/
		}
		if exprTouchesEnv(e.Target) {
			return true
		}
		for _, a := range e.Args {
			if exprTouchesEnv(a) {
				return true
			}
		}
		return false
	case *ArrayLiteralExpr:
		for _, el := range e.Elements {
			if exprTouchesEnv(el) {
				return true
			}
		}
		return false
	case *HashLiteralExpr:
		for _, p := range e.Pairs {
			if exprTouchesEnv(p[0]) || exprTouchesEnv(p[1]) {
				return true
			}
		}
		return false
	case *IndexExpr:
		return exprTouchesEnv(e.Array) || exprTouchesEnv(e.Index)
	case *HashAccessExpr:
		return exprTouchesEnv(e.Hash) || exprTouchesEnv(e.Key)
	case *SmartMatchExpr:
		return true // may invoke regex setting $/
	case *ChainedCompExpr:
		for _, ex := range e.Exprs {
			if exprTouchesEnv(ex) {
				return true
			}
		}
		return false
	case *InterpStringExpr:
		for _, pt := range e.Parts {
			if exprTouchesEnv(pt) {
				return true
			}
		}
		return false
	case *GatherExpr:
		return blockTouchesEnv(e.Body)
	case *AssignStmt:
		return stmtTouchesEnv(e)
	case *RefExpr:
		return true
	case *DerefExpr:
		if exprTouchesEnv(e.Ref) || exprTouchesEnv(e.Index) {
			return true
		}
		for _, a := range e.Args {
			if exprTouchesEnv(a) {
				return true
			}
		}
		return false
	case *BacktickExpr:
		return true // writes $?, $!
	default:
		return true
	}
}

func eBuiltinCheck(_ string, _ Expr) (struct{}, bool) {
	return struct{}{}, true
}

// blockUsesTopic checks whether b (outside nested topic-rebinding constructs) references $_.
func blockUsesTopic(b *BlockStmt) bool {
	if b == nil {
		return false
	}
	for _, st := range b.Stmts {
		if stmtUsesTopic(st) {
			return true
		}
	}
	return false
}

func stmtUsesTopic(stmt Stmt) bool {
	if stmt == nil {
		return false
	}
	switch s := stmt.(type) {
	case *BlockStmt:
		return blockUsesTopic(s)
	case *VarDeclStmt:
		return exprUsesTopic(s.Value) || exprUsesTopic(s.Where)
	case *AssignStmt:
		return exprUsesTopic(s.Target) || exprUsesTopic(s.Value)
	case *IfStmt:
		if exprUsesTopic(s.Condition) || blockUsesTopic(s.ThenBranch) || blockUsesTopic(s.ElseBranch) {
			return true
		}
		for i, c := range s.ElsifConds {
			if exprUsesTopic(c) {
				return true
			}
			if i < len(s.ElsifThen) && blockUsesTopic(s.ElsifThen[i]) {
				return true
			}
		}
		return false
	case *UnlessStmt:
		return exprUsesTopic(s.Condition) || blockUsesTopic(s.Body)
	case *WhileStmt:
		return exprUsesTopic(s.Condition) || blockUsesTopic(s.Body)
	case *ForStmt:
		return exprUsesTopic(s.Iterable) // inner ForStmt rebinds its own $_
	case *LoopStmt:
		return exprUsesTopic(s.Init) || exprUsesTopic(s.Cond) || exprUsesTopic(s.Step) || blockUsesTopic(s.Body)
	case *ReturnStmt:
		return exprUsesTopic(s.Value)
	case *ExprStmt:
		return exprUsesTopic(s.Expr)
	case *GivenStmt:
		return exprUsesTopic(s.Topic)
	case *AssertStmt:
		return exprUsesTopic(s.Condition) || exprUsesTopic(s.Message)
	case *VerifyStmt:
		return exprUsesTopic(s.Name) || exprUsesTopic(s.Cond) || exprUsesTopic(s.Message) || blockUsesTopic(s.Body)
	case *ModifierStmt:
		if s.Kind == ModFor || s.Kind == ModGiven {
			return exprUsesTopic(s.Condition)
		}
		return exprUsesTopic(s.Condition) || stmtUsesTopic(s.Target)
	case *LabelStmt:
		return stmtUsesTopic(s.Stmt)
	case *TakeStmt:
		return exprUsesTopic(s.Value)
	default:
		return false
	}
}

func exprUsesTopic(expr Expr) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *VarExpr:
		return e.Name == "$_"
	case *LiteralExpr:
		return e.Type == TokIdent && e.Value == "$_"
	case *BinaryExpr:
		return exprUsesTopic(e.Left) || exprUsesTopic(e.Right)
	case *UnaryExpr:
		return exprUsesTopic(e.Right)
	case *TernaryExpr:
		return exprUsesTopic(e.Cond) || exprUsesTopic(e.Then) || exprUsesTopic(e.Else)
	case *CallExpr:
		if exprUsesTopic(e.Callee) {
			return true
		}
		for _, a := range e.Args {
			if exprUsesTopic(a) {
				return true
			}
		}
		return false
	case *MethodCallExpr:
		if exprUsesTopic(e.Target) {
			return true
		}
		for _, a := range e.Args {
			if exprUsesTopic(a) {
				return true
			}
		}
		return false
	case *ArrayLiteralExpr:
		for _, el := range e.Elements {
			if exprUsesTopic(el) {
				return true
			}
		}
		return false
	case *HashLiteralExpr:
		for _, p := range e.Pairs {
			if exprUsesTopic(p[0]) || exprUsesTopic(p[1]) {
				return true
			}
		}
		return false
	case *IndexExpr:
		return exprUsesTopic(e.Array) || exprUsesTopic(e.Index)
	case *HashAccessExpr:
		return exprUsesTopic(e.Hash) || exprUsesTopic(e.Key)
	case *ClosureExpr:
		// If closure uses $_ without declaring $_ as param, outer $_ might be captured!
		for _, p := range e.Params {
			if p.Name == "$_" {
				return false
			}
		}
		return blockUsesTopic(e.Body)
	case *SmartMatchExpr:
		return exprUsesTopic(e.Left) || exprUsesTopic(e.Right)
	case *ChainedCompExpr:
		for _, ex := range e.Exprs {
			if exprUsesTopic(ex) {
				return true
			}
		}
		return false
	case *InterpStringExpr:
		for _, pt := range e.Parts {
			if exprUsesTopic(pt) {
				return true
			}
		}
		return false
	case *GatherExpr:
		return blockUsesTopic(e.Body)
	case *AssignStmt:
		return stmtUsesTopic(e)
	case *RefExpr:
		return exprUsesTopic(e.Expr)
	case *DerefExpr:
		if exprUsesTopic(e.Ref) || exprUsesTopic(e.Index) {
			return true
		}
		for _, a := range e.Args {
			if exprUsesTopic(a) {
				return true
			}
		}
		return false
	case *BacktickExpr:
		return exprUsesTopic(e.Command)
	default:
		return false
	}
}
