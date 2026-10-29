package raptor

import (
	"strings"
)

const (
	frameSlabCap  = 4096
	tempSlabCap   = 2048
	globalSlabShift = 9
	globalSlabCap   = 1 << globalSlabShift // 512
	globalSlabMask  = globalSlabCap - 1
	initFrameDepth  = 256
	initArgStackCap = 512
)

// Zero-allocation singleton control-flow sentinels.
var (
	sigReturn   error = &ReturnSignal{}
	sigBreak    error = &BreakSignal{}
	sigContinue error = &ContinueSignal{}
	sigRedo     error = &RedoSignal{}
)

// stackFrame represents an active function's Goose-style bump-allocated local slot frame.
type stackFrame struct {
	prev         *stackFrame
	ptrs         []*Value
	boxes        []Value
	savedSlabIdx int
	savedSlabPos int
	dirtyPtrs    bool
}

func (f *stackFrame) setSlot(slot int, val *Value) {
	if val == nil {
		b := &f.boxes[slot]
		b.Type = ValNil
		b.flags = flagStackLocal
		f.ptrs[slot] = b
		return
	}
	switch val.Type {
	case ValNil:
		b := &f.boxes[slot]
		b.Type = ValNil
		b.flags = flagStackLocal
		f.ptrs[slot] = b
	case ValBool:
		b := &f.boxes[slot]
		b.Type = ValBool
		b.BoolVal = val.BoolVal
		b.flags = flagStackLocal
		f.ptrs[slot] = b
	case ValInt:
		b := &f.boxes[slot]
		b.Type = ValInt
		b.IntVal = val.IntVal
		b.flags = flagStackLocal
		f.ptrs[slot] = b
	case ValFloat:
		b := &f.boxes[slot]
		b.Type = ValFloat
		b.FloatVal = val.FloatVal
		b.flags = flagStackLocal
		f.ptrs[slot] = b
	case ValString:
		b := &f.boxes[slot]
		b.Type = ValString
		b.StrVal = val.StrVal
		b.ext = val.ext
		b.extKind = val.extKind
		b.flags = flagStackLocal
		f.ptrs[slot] = b
		f.dirtyPtrs = true
	default:
		f.ptrs[slot] = escapeValue(val)
		f.dirtyPtrs = true
	}
}

func (f *stackFrame) setInt(slot int, n int64) {
	b := &f.boxes[slot]
	b.Type = ValInt
	b.IntVal = n
	b.flags = flagStackLocal
	f.ptrs[slot] = b
}

// globalSlotTable holds non-moving indexed slots for top-level variables and subs.
type globalSlotTable struct {
	slotMap  map[string]int
	names    []string
	isNamed  []bool
	boxSynced []bool
	boxSlabs []*[globalSlabCap]Value
	ptrSlabs []*[globalSlabCap]*Value
	count    int
}

func newGlobalSlotTable() *globalSlotTable {
	return &globalSlotTable{
		slotMap: make(map[string]int, 64),
	}
}

func (g *globalSlotTable) allocScopedSlot() int {
	idx := g.count
	g.count++
	g.names = append(g.names, "")
	g.isNamed = append(g.isNamed, false)
	g.boxSynced = append(g.boxSynced, true)

	slabIdx := idx >> globalSlabShift
	off := idx & globalSlabMask
	for len(g.boxSlabs) <= slabIdx {
		g.boxSlabs = append(g.boxSlabs, new([globalSlabCap]Value))
		g.ptrSlabs = append(g.ptrSlabs, new([globalSlabCap]*Value))
	}
	b := &g.boxSlabs[slabIdx][off]
	b.Type = ValNil
	b.flags = flagSlotBox
	g.ptrSlabs[slabIdx][off] = b
	return idx
}

func (g *globalSlotTable) ensureSlot(name string, globalEnv *Env) int {
	if idx, ok := g.slotMap[name]; ok {
		return idx
	}
	idx := g.count
	g.count++
	g.slotMap[name] = idx
	g.names = append(g.names, name)
	g.isNamed = append(g.isNamed, true)
	g.boxSynced = append(g.boxSynced, false)

	slabIdx := idx >> globalSlabShift
	off := idx & globalSlabMask
	for len(g.boxSlabs) <= slabIdx {
		g.boxSlabs = append(g.boxSlabs, new([globalSlabCap]Value))
		g.ptrSlabs = append(g.ptrSlabs, new([globalSlabCap]*Value))
	}
	b := &g.boxSlabs[slabIdx][off]
	b.Type = ValNil
	b.flags = flagSlotBox

	if existing, ok := globalEnv.bindings[name]; ok && existing != nil {
		g.setSlot(idx, existing, globalEnv)
	} else {
		g.ptrSlabs[slabIdx][off] = b
	}
	return idx
}

func (g *globalSlotTable) get(slot int) *Value {
	return g.ptrSlabs[slot>>globalSlabShift][slot&globalSlabMask]
}

func (g *globalSlotTable) box(slot int) *Value {
	return &g.boxSlabs[slot>>globalSlabShift][slot&globalSlabMask]
}

func (g *globalSlotTable) setInt(slot int, n int64, globalEnv *Env) *Value {
	slabIdx := slot >> globalSlabShift
	off := slot & globalSlabMask
	b := &g.boxSlabs[slabIdx][off]
	b.Type = ValInt
	b.IntVal = n
	b.flags = flagSlotBox
	if g.ptrSlabs[slabIdx][off] != b || !g.boxSynced[slot] {
		g.ptrSlabs[slabIdx][off] = b
		if globalEnv != nil && g.isNamed[slot] {
			globalEnv.bindings[g.names[slot]] = b
		}
		g.boxSynced[slot] = true
	}
	return b
}

func (g *globalSlotTable) setSlot(slot int, val *Value, globalEnv *Env) *Value {
	slabIdx := slot >> globalSlabShift
	off := slot & globalSlabMask
	b := &g.boxSlabs[slabIdx][off]
	syncEnv := globalEnv != nil && g.isNamed[slot]
	if val == nil {
		b.Type = ValNil
		b.StrVal = ""
		b.ArrayVal = nil
		b.HashVal = nil
		b.ext = nil
		b.extKind = extNone
		b.flags = flagSlotBox
		g.ptrSlabs[slabIdx][off] = b
		if syncEnv && !g.boxSynced[slot] {
			globalEnv.bindings[g.names[slot]] = b
			g.boxSynced[slot] = true
		}
		return b
	}
	switch val.Type {
	case ValNil:
		b.Type = ValNil
		b.StrVal = ""
		b.ext = nil
		b.extKind = extNone
		b.flags = flagSlotBox
		g.ptrSlabs[slabIdx][off] = b
		if syncEnv && !g.boxSynced[slot] {
			globalEnv.bindings[g.names[slot]] = b
			g.boxSynced[slot] = true
		}
		return b
	case ValBool:
		b.Type = ValBool
		b.BoolVal = val.BoolVal
		b.flags = flagSlotBox
		g.ptrSlabs[slabIdx][off] = b
		if syncEnv && !g.boxSynced[slot] {
			globalEnv.bindings[g.names[slot]] = b
			g.boxSynced[slot] = true
		}
		return b
	case ValInt:
		b.Type = ValInt
		b.IntVal = val.IntVal
		b.flags = flagSlotBox
		g.ptrSlabs[slabIdx][off] = b
		if syncEnv && !g.boxSynced[slot] {
			globalEnv.bindings[g.names[slot]] = b
			g.boxSynced[slot] = true
		}
		return b
	case ValFloat:
		b.Type = ValFloat
		b.FloatVal = val.FloatVal
		b.flags = flagSlotBox
		g.ptrSlabs[slabIdx][off] = b
		if syncEnv && !g.boxSynced[slot] {
			globalEnv.bindings[g.names[slot]] = b
			g.boxSynced[slot] = true
		}
		return b
	case ValString:
		b.Type = ValString
		b.StrVal = val.StrVal
		b.ext = val.ext
		b.extKind = val.extKind
		b.flags = flagSlotBox
		g.ptrSlabs[slabIdx][off] = b
		if syncEnv && !g.boxSynced[slot] {
			globalEnv.bindings[g.names[slot]] = b
			g.boxSynced[slot] = true
		}
		return b
	default:
		esc := escapeValue(val)
		g.ptrSlabs[slabIdx][off] = esc
		g.boxSynced[slot] = false
		if syncEnv {
			globalEnv.bindings[g.names[slot]] = esc
		}
		return esc
	}
}

// dataStack holds per-worker Goose-style bump-allocated execution stacks.
type dataStack struct {
	// Function lexical slot arena (segmented non-moving slabs)
	frameSlabs    []*[frameSlabCap]Value
	framePtrSlabs []*[frameSlabCap]*Value
	frameSlabIdx  int
	frameSlabPos  int
	frameRecords  []stackFrame
	frameDepth    int
	curFrame      *stackFrame

	// Ephemeral expression temporary value arena (LIFO scope watermark reset)
	tempSlabs   []*[tempSlabCap]Value
	tempSlabIdx int
	tempSlabPos int

	// Contiguous call argument pointer stack
	argStack []*Value
	argSP    int

	// Zero-allocation return value register paired with sigReturn
	returnVal *Value
}

func (ds *dataStack) initLazyFrames() {
	if len(ds.frameSlabs) == 0 {
		ds.frameSlabs = []*[frameSlabCap]Value{new([frameSlabCap]Value)}
		ds.framePtrSlabs = []*[frameSlabCap]*Value{new([frameSlabCap]*Value)}
		ds.frameRecords = make([]stackFrame, initFrameDepth)
	}
}

func (ds *dataStack) pushFrame(numSlots int, initFromSlot int) *stackFrame {
	if len(ds.frameSlabs) == 0 {
		ds.initLazyFrames()
	}
	if ds.frameDepth >= len(ds.frameRecords) {
		grown := make([]stackFrame, len(ds.frameRecords)*2)
		copy(grown, ds.frameRecords)
		ds.frameRecords = grown
		if ds.frameDepth > 0 {
			for i := 0; i < ds.frameDepth; i++ {
				if i == 0 {
					ds.frameRecords[i].prev = nil
				} else {
					ds.frameRecords[i].prev = &ds.frameRecords[i-1]
				}
			}
			ds.curFrame = &ds.frameRecords[ds.frameDepth-1]
		}
	}

	f := &ds.frameRecords[ds.frameDepth]
	ds.frameDepth++
	f.prev = ds.curFrame
	f.savedSlabIdx = ds.frameSlabIdx
	f.savedSlabPos = ds.frameSlabPos
	f.dirtyPtrs = false

	if numSlots > 0 {
		if ds.frameSlabPos+numSlots > frameSlabCap {
			ds.frameSlabIdx++
			ds.frameSlabPos = 0
			for len(ds.frameSlabs) <= ds.frameSlabIdx {
				ds.frameSlabs = append(ds.frameSlabs, new([frameSlabCap]Value))
				ds.framePtrSlabs = append(ds.framePtrSlabs, new([frameSlabCap]*Value))
			}
		}
		pos := ds.frameSlabPos
		ds.frameSlabPos = pos + numSlots
		f.boxes = ds.frameSlabs[ds.frameSlabIdx][pos : pos+numSlots]
		f.ptrs = ds.framePtrSlabs[ds.frameSlabIdx][pos : pos+numSlots]
		for i := initFromSlot; i < numSlots; i++ {
			b := &f.boxes[i]
			b.Type = ValNil
			b.flags = flagStackLocal
			f.ptrs[i] = b
		}
	} else {
		f.boxes = nil
		f.ptrs = nil
	}

	ds.curFrame = f
	return f
}

func (ds *dataStack) popFrame(f *stackFrame) {
	if ds.frameDepth == 0 {
		return
	}
	ds.frameDepth--
	top := &ds.frameRecords[ds.frameDepth]
	if top.dirtyPtrs {
		for i := range top.boxes {
			b := &top.boxes[i]
			b.StrVal = ""
			b.ArrayVal = nil
			b.HashVal = nil
			b.ext = nil
			b.extKind = extNone
			top.ptrs[i] = nil
		}
	}
	ds.frameSlabIdx = top.savedSlabIdx
	ds.frameSlabPos = top.savedSlabPos
	if ds.frameDepth == 0 {
		ds.curFrame = nil
	} else {
		ds.curFrame = &ds.frameRecords[ds.frameDepth-1]
	}
}

func (ds *dataStack) markTemp() int {
	return (ds.tempSlabIdx << 16) | ds.tempSlabPos
}

func (ds *dataStack) rewindTemp(mark int) {
	ds.tempSlabIdx = mark >> 16
	ds.tempSlabPos = mark & 0xFFFF
}

func (ds *dataStack) allocTemp() *Value {
	if len(ds.tempSlabs) == 0 {
		ds.tempSlabs = []*[tempSlabCap]Value{new([tempSlabCap]Value)}
	}
	if ds.tempSlabPos >= tempSlabCap {
		ds.tempSlabIdx++
		ds.tempSlabPos = 0
		for len(ds.tempSlabs) <= ds.tempSlabIdx {
			ds.tempSlabs = append(ds.tempSlabs, new([tempSlabCap]Value))
		}
	}
	v := &ds.tempSlabs[ds.tempSlabIdx][ds.tempSlabPos]
	ds.tempSlabPos++
	if v.StrVal != "" || v.ArrayVal != nil || v.HashVal != nil || v.ext != nil {
		v.StrVal = ""
		v.ArrayVal = nil
		v.HashVal = nil
		v.ext = nil
		v.extKind = extNone
	}
	v.flags = flagTempArena
	return v
}

func (in *Interp) tempInt(n int64) *Value {
	if n >= intInternMin && n <= intInternMax {
		return intIntern[n-intInternMin]
	}
	v := in.ds.allocTemp()
	v.Type = ValInt
	v.IntVal = n
	return v
}

func (in *Interp) tempFloat(f float64) *Value {
	v := in.ds.allocTemp()
	v.Type = ValFloat
	v.FloatVal = f
	return v
}

// promoteReturn rewinds inner temporary values back to callerMark and promotes
// any stack-local or callee-temporary return value into a single caller temporary slot.
func (in *Interp) promoteReturn(ret *Value, callerMark int) *Value {
	if ret == nil {
		in.ds.rewindTemp(callerMark)
		return valNil
	}
	if (ret.flags & (flagStackLocal | flagTempArena)) == 0 {
		in.ds.rewindTemp(callerMark)
		return ret
	}
	switch ret.Type {
	case ValNil:
		in.ds.rewindTemp(callerMark)
		return valNil
	case ValBool:
		b := ret.BoolVal
		in.ds.rewindTemp(callerMark)
		return BoolValue(b)
	case ValInt:
		n := ret.IntVal
		in.ds.rewindTemp(callerMark)
		return in.tempInt(n)
	case ValFloat:
		f := ret.FloatVal
		in.ds.rewindTemp(callerMark)
		return in.tempFloat(f)
	case ValString:
		s := ret.StrVal
		if ret.extKind == extStrBuf {
			s = strings.Clone(s)
		}
		in.ds.rewindTemp(callerMark)
		if s == "" {
			return valEmptyStr
		}
		v := in.ds.allocTemp()
		v.Type = ValString
		v.StrVal = s
		return v
	default:
		esc := escapeValue(ret)
		in.ds.rewindTemp(callerMark)
		return esc
	}
}

func (ds *dataStack) pushArgs(n int) (int, []*Value) {
	if n == 0 {
		return ds.argSP, nil
	}
	if len(ds.argStack) == 0 {
		capN := initArgStackCap
		if n > capN {
			capN = n * 2
		}
		ds.argStack = make([]*Value, capN)
	} else if ds.argSP+n > len(ds.argStack) {
		capN := len(ds.argStack) * 2
		if ds.argSP+n > capN {
			capN = (ds.argSP + n) * 2
		}
		grown := make([]*Value, capN)
		copy(grown, ds.argStack[:ds.argSP])
		ds.argStack = grown
	}
	base := ds.argSP
	ds.argSP = base + n
	return base, ds.argStack[base : base+n : base+n]
}

func (ds *dataStack) popArgs(base int) {
	for i := base; i < ds.argSP; i++ {
		ds.argStack[i] = nil
	}
	ds.argSP = base
}

// forkWorker creates a lightweight concurrent execution context with its own private
// Goose DataStack while sharing read/module state with the parent Interp.
func (in *Interp) forkWorker() *Interp {
	w := *in
	w.ds = dataStack{}
	w.GatherStack = nil
	w.CurrentEnv = in.GlobalEnv
	return &w
}
