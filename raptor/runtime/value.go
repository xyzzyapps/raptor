package raptor

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

// ValueType identifies the runtime kind of a Value.
type ValueType uint8

const (
	ValNil ValueType = iota
	ValBool
	ValInt
	ValFloat
	ValString
	ValArray
	ValHash
	ValClosure
	ValMultiSub
	ValNativePtr
	ValCStruct
	ValPromise
	ValChannel
	ValJunction
	ValLazySeq
	ValRef
)

// extKind discriminates the single rare-variant pointer stored in Value.ext.
const (
	extNone uint8 = iota
	extClosure
	extMultiSub
	extCStruct
	extPromise
	extChannel
	extJunction
	extLazySeq
	extRef
	extStrBuf
	extArrayLane
)

// Arena / stack ownership flags on Value.flags.
const (
	flagTempArena  uint8 = 1 << 0 // Allocated in per-Interp temporary bump arena
	flagStackLocal uint8 = 1 << 1 // Inline box in an active function stackFrame
	flagSlotBox    uint8 = 1 << 2 // Inline box in a persistent global slot
)

type JunctionKind int

const (
	JunctionAll JunctionKind = iota
	JunctionAny
	JunctionOne
	JunctionNone
)

// Junction represents a quantum autothreading value in Raku.
type Junction struct {
	Kind   JunctionKind
	Values []*Value
}

// LazySeq represents a sequence generated via gather/take or lazy list.
type LazySeq struct {
	Items []*Value
}

// Promise represents an asynchronous computation.
type Promise struct {
	Done   chan struct{}
	Result *Value
	Err    error
	Status string // "Planned", "Kept", "Broken"
	Mu     sync.Mutex
}

// Channel represents a concurrent thread-safe queue.
type Channel struct {
	Ch     chan *Value
	Closed bool
	Mu     sync.Mutex
}

// CStructInstance represents a live instance of a CStruct or CUnion in memory.
type CStructInstance struct {
	Class    *CStructDeclStmt
	Ptr      uintptr
	Buffer   []byte
	Closures map[string]*Value
}

// Closure represents a first-class callable with captured lexical environment.
type Closure struct {
	Name          string
	Params        []Param
	Body          *BlockStmt
	Env           *Env
	IsMulti       bool
	IsRaw         bool
	FrameSlots    int
	ParamSlots    []int
	NeedsEnv      bool
	FastSimple    bool
	NoParamChecks bool
}

type multiSubBox struct {
	candidates []*Closure
}

type strBufBox struct {
	buf []byte
}

// StrTape is a Goose/Arrow-style pointerless contiguous string buffer.
// Neither Offsets nor Data contains pointers, so Go's GC skips scanning the entire tape.
type StrTape struct {
	Offsets []uint32 // length N+1; element i is Data[Offsets[i]:Offsets[i+1]]
	Data    []byte
}

func NewStrTape(capElems, capBytes int) *StrTape {
	offs := make([]uint32, 1, capElems+1)
	offs[0] = 0
	var data []byte
	if capBytes > 0 {
		data = make([]byte, 0, capBytes)
	}
	return &StrTape{Offsets: offs, Data: data}
}

func (st *StrTape) Len() int {
	if st == nil || len(st.Offsets) == 0 {
		return 0
	}
	return len(st.Offsets) - 1
}

func (st *StrTape) At(i int) string {
	start := st.Offsets[i]
	end := st.Offsets[i+1]
	if start == end {
		return ""
	}
	return unsafe.String(&st.Data[start], int(end-start))
}

func (st *StrTape) Append(s string) {
	st.Data = append(st.Data, s...)
	st.Offsets = append(st.Offsets, uint32(len(st.Data)))
}

func (st *StrTape) Clone() *StrTape {
	if st == nil {
		return nil
	}
	offs := make([]uint32, len(st.Offsets))
	copy(offs, st.Offsets)
	data := make([]byte, len(st.Data))
	copy(data, st.Data)
	return &StrTape{Offsets: offs, Data: data}
}

// ArrayLane holds packed homogeneous columnar storage for ValArray.
type ArrayLane struct {
	Ints   []int64
	Floats []float64
	Strs   *StrTape
}

// Value represents any runtime value in Raptor.
// Pointer fields are grouped at offsets 0..56 so Go's GC PtrBytes stops at 56 bytes
// (4 pointer words instead of 12), and total struct size is 88 bytes (down from 216 bytes).
type Value struct {
	StrVal   string
	ArrayVal []*Value
	HashVal  map[string]*Value
	ext      unsafe.Pointer

	IntVal   int64
	FloatVal float64
	PtrVal   uintptr
	Type     ValueType
	BoolVal  bool
	extKind  uint8
	flags    uint8
}

const (
	intInternMin int64 = -256
	intInternMax int64 = 256
)

var (
	intIntern    [intInternMax - intInternMin + 1]*Value
	smallIntStrs [1024]string
)

func init() {
	for i := intInternMin; i <= intInternMax; i++ {
		intIntern[i-intInternMin] = &Value{Type: ValInt, IntVal: i}
	}
	for i := 0; i < len(smallIntStrs); i++ {
		smallIntStrs[i] = strconv.Itoa(i)
	}
}

func formatIntFast(n int64) string {
	if n >= 0 && n < int64(len(smallIntStrs)) {
		return smallIntStrs[n]
	}
	return strconv.FormatInt(n, 10)
}

func isInternedInt(v *Value) bool {
	if v == nil || v.Type != ValInt {
		return false
	}
	i := v.IntVal
	if i < intInternMin || i > intInternMax {
		return false
	}
	return intIntern[i-intInternMin] == v
}

func isFlyweight(v *Value) bool {
	if v == nil {
		return true
	}
	if v == valNil || v == valTrue || v == valFalse || v == valEmptyStr {
		return true
	}
	return isInternedInt(v)
}

// escapeValue promotes stack-local or temporary arena values to independent heap values
// before they are stored into heap containers, globals, closures, or channels.
func escapeValue(v *Value) *Value {
	if v == nil {
		return valNil
	}
	if v.flags == 0 {
		return v
	}
	switch v.Type {
	case ValNil:
		return valNil
	case ValBool:
		return BoolValue(v.BoolVal)
	case ValInt:
		return IntValue(v.IntVal)
	case ValFloat:
		return &Value{Type: ValFloat, FloatVal: v.FloatVal}
	case ValString:
		if v.StrVal == "" && v.extKind != extStrBuf {
			return valEmptyStr
		}
		s := v.StrVal
		if v.extKind == extStrBuf {
			s = strings.Clone(s)
		}
		return &Value{Type: ValString, StrVal: s}
	default:
		cp := *v
		cp.flags = 0
		return &cp
	}
}

func escapeValues(vs []*Value) {
	for i, v := range vs {
		if v != nil && v.flags != 0 {
			vs[i] = escapeValue(v)
		}
	}
}

// Rare-variant typed accessors backed by Value.ext.

func (v *Value) Closure() *Closure {
	if v == nil || v.extKind != extClosure || v.ext == nil {
		return nil
	}
	return (*Closure)(v.ext)
}

func (v *Value) Candidates() []*Closure {
	if v == nil || v.extKind != extMultiSub || v.ext == nil {
		return nil
	}
	return (*multiSubBox)(v.ext).candidates
}

func (v *Value) AppendCandidate(cand *Closure) {
	if v == nil {
		return
	}
	if v.extKind != extMultiSub || v.ext == nil {
		box := &multiSubBox{candidates: []*Closure{cand}}
		v.ext = unsafe.Pointer(box)
		v.extKind = extMultiSub
		return
	}
	box := (*multiSubBox)(v.ext)
	box.candidates = append(box.candidates, cand)
}

func (v *Value) CStruct() *CStructInstance {
	if v == nil || v.extKind != extCStruct || v.ext == nil {
		return nil
	}
	return (*CStructInstance)(v.ext)
}

func (v *Value) Promise() *Promise {
	if v == nil || v.extKind != extPromise || v.ext == nil {
		return nil
	}
	return (*Promise)(v.ext)
}

func (v *Value) Channel() *Channel {
	if v == nil || v.extKind != extChannel || v.ext == nil {
		return nil
	}
	return (*Channel)(v.ext)
}

func (v *Value) Junction() *Junction {
	if v == nil || v.extKind != extJunction || v.ext == nil {
		return nil
	}
	return (*Junction)(v.ext)
}

func (v *Value) LazySeq() *LazySeq {
	if v == nil || v.extKind != extLazySeq || v.ext == nil {
		return nil
	}
	return (*LazySeq)(v.ext)
}

func (v *Value) Ref() *Value {
	if v == nil || v.extKind != extRef || v.ext == nil {
		return nil
	}
	return (*Value)(v.ext)
}

func (v *Value) lane() *ArrayLane {
	if v == nil || v.extKind != extArrayLane || v.ext == nil {
		return nil
	}
	return (*ArrayLane)(v.ext)
}

func (v *Value) ensureLane() *ArrayLane {
	if v.extKind == extArrayLane && v.ext != nil {
		return (*ArrayLane)(v.ext)
	}
	l := &ArrayLane{}
	v.ext = unsafe.Pointer(l)
	v.extKind = extArrayLane
	return l
}

func (v *Value) intLane() []int64 {
	if v == nil || v.ArrayVal != nil || v.extKind != extArrayLane || v.ext == nil {
		return nil
	}
	return (*ArrayLane)(v.ext).Ints
}

func (v *Value) floatLane() []float64 {
	if v == nil || v.ArrayVal != nil || v.extKind != extArrayLane || v.ext == nil {
		return nil
	}
	return (*ArrayLane)(v.ext).Floats
}

func (v *Value) strLane() *StrTape {
	if v == nil || v.ArrayVal != nil || v.extKind != extArrayLane || v.ext == nil {
		return nil
	}
	return (*ArrayLane)(v.ext).Strs
}

func JunctionValue(kind JunctionKind, vals []*Value) *Value {
	escapeValues(vals)
	j := &Junction{Kind: kind, Values: vals}
	return &Value{
		Type:    ValJunction,
		ext:     unsafe.Pointer(j),
		extKind: extJunction,
	}
}

func LazySeqValue(items []*Value) *Value {
	escapeValues(items)
	ls := &LazySeq{Items: items}
	return &Value{
		Type:    ValLazySeq,
		ext:     unsafe.Pointer(ls),
		extKind: extLazySeq,
	}
}

var (
	valNil      = &Value{Type: ValNil}
	valTrue     = &Value{Type: ValBool, BoolVal: true}
	valFalse    = &Value{Type: ValBool, BoolVal: false}
	valEmptyStr = &Value{Type: ValString, StrVal: ""}
)

func NilValue() *Value {
	return valNil
}

func BoolValue(b bool) *Value {
	if b {
		return valTrue
	}
	return valFalse
}

func IntValue(i int64) *Value {
	if i >= intInternMin && i <= intInternMax {
		return intIntern[i-intInternMin]
	}
	return &Value{Type: ValInt, IntVal: i}
}

func IntArrayValue(ints []int64) *Value {
	if ints == nil {
		ints = []int64{}
	}
	l := &ArrayLane{Ints: ints}
	return &Value{Type: ValArray, ext: unsafe.Pointer(l), extKind: extArrayLane}
}

func FloatArrayValue(floats []float64) *Value {
	if floats == nil {
		floats = []float64{}
	}
	l := &ArrayLane{Floats: floats}
	return &Value{Type: ValArray, ext: unsafe.Pointer(l), extKind: extArrayLane}
}

func StrTapeArrayValue(tape *StrTape) *Value {
	if tape == nil {
		tape = NewStrTape(0, 0)
	}
	l := &ArrayLane{Strs: tape}
	return &Value{Type: ValArray, ext: unsafe.Pointer(l), extKind: extArrayLane}
}

func (v *Value) arrayLen() int {
	if v == nil || v.Type != ValArray {
		return 0
	}
	if v.ArrayVal == nil && v.extKind == extArrayLane && v.ext != nil {
		l := (*ArrayLane)(v.ext)
		if l.Ints != nil {
			return len(l.Ints)
		}
		if l.Floats != nil {
			return len(l.Floats)
		}
		if l.Strs != nil {
			return l.Strs.Len()
		}
	}
	return len(v.ArrayVal)
}

func (v *Value) arrayAt(i int) *Value {
	if v == nil || v.Type != ValArray {
		return NilValue()
	}
	if v.ArrayVal == nil && v.extKind == extArrayLane && v.ext != nil {
		l := (*ArrayLane)(v.ext)
		if l.Ints != nil {
			if i < 0 || i >= len(l.Ints) {
				return NilValue()
			}
			return IntValue(l.Ints[i])
		}
		if l.Floats != nil {
			if i < 0 || i >= len(l.Floats) {
				return NilValue()
			}
			return FloatValue(l.Floats[i])
		}
		if l.Strs != nil {
			if i < 0 || i >= l.Strs.Len() {
				return NilValue()
			}
			return StringValue(l.Strs.At(i))
		}
	}
	if i < 0 || i >= len(v.ArrayVal) {
		return NilValue()
	}
	return v.ArrayVal[i]
}

func (v *Value) materializeArray() []*Value {
	if v == nil || v.Type != ValArray {
		return nil
	}
	if v.ArrayVal != nil {
		return v.ArrayVal
	}
	l := v.lane()
	if l == nil {
		return nil
	}
	if l.Ints != nil {
		out := make([]*Value, len(l.Ints))
		for i, n := range l.Ints {
			out[i] = IntValue(n)
		}
		v.ArrayVal = out
		v.ext = nil
		v.extKind = extNone
		return out
	}
	if l.Floats != nil {
		out := make([]*Value, len(l.Floats))
		for i, f := range l.Floats {
			out[i] = FloatValue(f)
		}
		v.ArrayVal = out
		v.ext = nil
		v.extKind = extNone
		return out
	}
	if l.Strs != nil {
		n := l.Strs.Len()
		out := make([]*Value, n)
		for i := 0; i < n; i++ {
			out[i] = StringValue(strings.Clone(l.Strs.At(i)))
		}
		v.ArrayVal = out
		v.ext = nil
		v.extKind = extNone
		return out
	}
	return nil
}

func (v *Value) pushValue(x *Value) {
	if v == nil {
		return
	}
	v.Type = ValArray
	if v.ArrayVal == nil {
		l := v.lane()
		if x != nil {
			switch x.Type {
			case ValInt:
				if l == nil {
					l = v.ensureLane()
					l.Ints = make([]int64, 0, 8)
				}
				if l.Ints != nil && l.Floats == nil && l.Strs == nil {
					l.Ints = append(l.Ints, x.IntVal)
					return
				}
			case ValFloat:
				if l == nil {
					l = v.ensureLane()
					l.Floats = make([]float64, 0, 8)
				}
				if l.Floats != nil && l.Ints == nil && l.Strs == nil {
					l.Floats = append(l.Floats, x.FloatVal)
					return
				}
			case ValString:
				if l == nil {
					l = v.ensureLane()
					l.Strs = NewStrTape(8, len(x.StrVal)*8)
				}
				if l.Strs != nil && l.Ints == nil && l.Floats == nil {
					l.Strs.Append(x.StrVal)
					return
				}
			}
		}
		if l != nil {
			v.materializeArray()
		}
	}
	v.ArrayVal = append(v.ArrayVal, escapeValue(x))
}

func (v *Value) appendString(suffix string) *Value {
	if v == nil || isFlyweight(v) || v.Type != ValString {
		return StringValue(v.String() + suffix)
	}
	var sb *strBufBox
	if v.extKind == extStrBuf && v.ext != nil {
		sb = (*strBufBox)(v.ext)
	} else {
		need := len(v.StrVal) + len(suffix)
		capn := need + 32
		if capn < 64 {
			capn = 64
		}
		buf := make([]byte, len(v.StrVal), capn)
		copy(buf, v.StrVal)
		sb = &strBufBox{buf: buf}
		v.ext = unsafe.Pointer(sb)
		v.extKind = extStrBuf
	}
	sb.buf = append(sb.buf, suffix...)
	// Share backing store — string() would copy and restore quadratic churn.
	v.StrVal = unsafe.String(unsafe.SliceData(sb.buf), len(sb.buf))
	return v
}

func FloatValue(f float64) *Value {
	return &Value{Type: ValFloat, FloatVal: f}
}

func StringValue(s string) *Value {
	if s == "" {
		return valEmptyStr
	}
	return &Value{Type: ValString, StrVal: s}
}

func ArrayValue(elems []*Value) *Value {
	escapeValues(elems)
	return &Value{Type: ValArray, ArrayVal: elems}
}

func HashValue(m map[string]*Value) *Value {
	for k, v := range m {
		if v != nil && v.flags != 0 {
			m[k] = escapeValue(v)
		}
	}
	return &Value{Type: ValHash, HashVal: m}
}

func ClosureValue(c *Closure) *Value {
	return &Value{Type: ValClosure, ext: unsafe.Pointer(c), extKind: extClosure}
}

func MultiSubValue(candidates []*Closure) *Value {
	box := &multiSubBox{candidates: candidates}
	return &Value{Type: ValMultiSub, ext: unsafe.Pointer(box), extKind: extMultiSub}
}

func NativePtrValue(ptr uintptr) *Value {
	return &Value{Type: ValNativePtr, PtrVal: ptr}
}

func CStructValue(inst *CStructInstance) *Value {
	var ptr uintptr
	if inst != nil {
		ptr = inst.Ptr
	}
	return &Value{Type: ValCStruct, PtrVal: ptr, ext: unsafe.Pointer(inst), extKind: extCStruct}
}

func PromiseValue(p *Promise) *Value {
	return &Value{Type: ValPromise, ext: unsafe.Pointer(p), extKind: extPromise}
}

func ChannelValue(c *Channel) *Value {
	return &Value{Type: ValChannel, ext: unsafe.Pointer(c), extKind: extChannel}
}

func RefValue(target *Value) *Value {
	target = escapeValue(target)
	return &Value{Type: ValRef, ext: unsafe.Pointer(target), extKind: extRef}
}

// IsTrue returns Raku truthiness.
func (v *Value) IsTrue() bool {
	if v == nil {
		return false
	}
	switch v.Type {
	case ValNil:
		return false
	case ValBool:
		return v.BoolVal
	case ValInt:
		return v.IntVal != 0
	case ValFloat:
		return v.FloatVal != 0.0
	case ValString:
		return v.StrVal != "" && v.StrVal != "0"
	case ValArray:
		return v.arrayLen() > 0
	case ValHash:
		return len(v.HashVal) > 0
	case ValClosure, ValMultiSub, ValNativePtr, ValCStruct, ValPromise, ValChannel, ValRef:
		return true
	case ValJunction:
		j := v.Junction()
		if j == nil {
			return false
		}
		switch j.Kind {
		case JunctionAll:
			for _, elem := range j.Values {
				if !elem.IsTrue() {
					return false
				}
			}
			return len(j.Values) > 0
		case JunctionAny:
			for _, elem := range j.Values {
				if elem.IsTrue() {
					return true
				}
			}
			return false
		case JunctionOne:
			count := 0
			for _, elem := range j.Values {
				if elem.IsTrue() {
					count++
				}
			}
			return count == 1
		case JunctionNone:
			for _, elem := range j.Values {
				if elem.IsTrue() {
					return false
				}
			}
			return true
		}
		return false
	case ValLazySeq:
		if ls := v.LazySeq(); ls != nil {
			return len(ls.Items) > 0
		}
		return false
	default:
		return false
	}
}

// TypeName returns the type name of the value.
func (v *Value) TypeName() string {
	if v == nil {
		return "Nil"
	}
	switch v.Type {
	case ValNil:
		return "Nil"
	case ValBool:
		return "Bool"
	case ValInt:
		return "Int"
	case ValFloat:
		return "Num"
	case ValString:
		return "Str"
	case ValArray:
		return "Array"
	case ValHash:
		return "Hash"
	case ValClosure, ValMultiSub:
		return "Callable"
	case ValNativePtr:
		return "Pointer"
	case ValPromise:
		return "Promise"
	case ValChannel:
		return "Channel"
	case ValJunction:
		return "Junction"
	case ValLazySeq:
		return "Seq"
	case ValRef:
		return "Ref"
	case ValCStruct:
		if cs := v.CStruct(); cs != nil && cs.Class != nil {
			return cs.Class.Name
		}
		return "CStruct"
	default:
		return "Any"
	}
}

// MatchesType checks if the value satisfies a type constraint.
func (v *Value) MatchesType(constraint string) bool {
	if constraint == "" || constraint == "Any" {
		return true
	}
	if v == nil {
		return constraint == "Nil"
	}
	switch constraint {
	case "Int":
		return v.Type == ValInt
	case "Str":
		return v.Type == ValString
	case "Num":
		return v.Type == ValFloat || v.Type == ValInt
	case "Bool":
		return v.Type == ValBool
	case "Array":
		return v.Type == ValArray || v.Type == ValLazySeq
	case "Hash":
		return v.Type == ValHash
	case "Callable":
		return v.Type == ValClosure || v.Type == ValMultiSub
	case "Pointer", "OpaquePointer":
		return v.Type == ValNativePtr || v.Type == ValCStruct
	case "Promise":
		return v.Type == ValPromise
	case "Channel":
		return v.Type == ValChannel
	case "Junction":
		return v.Type == ValJunction
	case "Seq":
		return v.Type == ValLazySeq || v.Type == ValArray
	case "Ref", "Reference":
		return v.Type == ValRef
	default:
		if v.Type == ValCStruct {
			if cs := v.CStruct(); cs != nil && cs.Class != nil {
				return cs.Class.Name == constraint
			}
		}
		return false
	}
}

func (v *Value) String() string {
	if v == nil {
		return "(nil)"
	}
	switch v.Type {
	case ValNil:
		return "Nil"
	case ValBool:
		if v.BoolVal {
			return "True"
		}
		return "False"
	case ValInt:
		return formatIntFast(v.IntVal)
	case ValFloat:
		return strconv.FormatFloat(v.FloatVal, 'g', -1, 64)
	case ValString:
		return v.StrVal
	case ValArray:
		if v.ArrayVal == nil {
			if ints := v.intLane(); ints != nil {
				items := make([]string, len(ints))
				for i, n := range ints {
					items[i] = formatIntFast(n)
				}
				return "[" + strings.Join(items, " ") + "]"
			}
			if floats := v.floatLane(); floats != nil {
				items := make([]string, len(floats))
				for i, f := range floats {
					items[i] = strconv.FormatFloat(f, 'g', -1, 64)
				}
				return "[" + strings.Join(items, " ") + "]"
			}
			if strs := v.strLane(); strs != nil {
				n := strs.Len()
				items := make([]string, n)
				for i := 0; i < n; i++ {
					items[i] = strs.At(i)
				}
				return "[" + strings.Join(items, " ") + "]"
			}
		}
		items := make([]string, len(v.ArrayVal))
		for i, e := range v.ArrayVal {
			items[i] = e.String()
		}
		return "[" + strings.Join(items, " ") + "]"
	case ValHash:
		var pairs []string
		for k, val := range v.HashVal {
			pairs = append(pairs, k+" => "+val.String())
		}
		return "{" + strings.Join(pairs, ", ") + "}"
	case ValClosure:
		if c := v.Closure(); c != nil && c.Name != "" {
			return "sub " + c.Name + " { ... }"
		}
		return "sub { ... }"
	case ValMultiSub:
		return fmt.Sprintf("multi sub (%d candidates)", len(v.Candidates()))
	case ValNativePtr:
		return "0x" + strconv.FormatUint(uint64(v.PtrVal), 16)
	case ValPromise:
		if p := v.Promise(); p != nil {
			return "Promise(" + p.Status + ")"
		}
		return "Promise"
	case ValChannel:
		return "Channel"
	case ValJunction:
		if j := v.Junction(); j != nil {
			var parts []string
			for _, elem := range j.Values {
				parts = append(parts, elem.String())
			}
			switch j.Kind {
			case JunctionAll:
				return "all(" + strings.Join(parts, ", ") + ")"
			case JunctionAny:
				return "any(" + strings.Join(parts, ", ") + ")"
			case JunctionOne:
				return "one(" + strings.Join(parts, ", ") + ")"
			case JunctionNone:
				return "none(" + strings.Join(parts, ", ") + ")"
			}
		}
		return "junction()"
	case ValLazySeq:
		if ls := v.LazySeq(); ls != nil {
			var parts []string
			for _, it := range ls.Items {
				parts = append(parts, it.String())
			}
			return "(" + strings.Join(parts, " ") + "...)"
		}
		return "Seq(...)"
	case ValRef:
		if r := v.Ref(); r != nil {
			return "\\" + r.String()
		}
		return "\\(nil)"
	case ValCStruct:
		if cs := v.CStruct(); cs != nil && cs.Class != nil {
			return fmt.Sprintf("struct %s(0x%x)", cs.Class.Name, cs.Ptr)
		}
		return "struct (nil)"
	default:
		return "<unknown>"
	}
}

// RefType returns the Perl 5 ref($val) type string: "SCALAR", "ARRAY", "HASH", "CODE", "REF", or "".
func (v *Value) RefType() string {
	if v == nil {
		return ""
	}
	switch v.Type {
	case ValRef:
		r := v.Ref()
		if r == nil {
			return "SCALAR"
		}
		switch r.Type {
		case ValArray:
			return "ARRAY"
		case ValHash:
			return "HASH"
		case ValClosure, ValMultiSub:
			return "CODE"
		case ValRef:
			return "REF"
		default:
			return "SCALAR"
		}
	case ValArray:
		return "ARRAY"
	case ValHash:
		return "HASH"
	case ValClosure, ValMultiSub:
		return "CODE"
	default:
		return ""
	}
}

// ToJSON marshals Value into standard JSON representation.
func (v *Value) ToJSON() (string, error) {
	raw := v.ToInterface()
	b, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ToInterface converts a Value into Go primitive types.
func (v *Value) ToInterface() any {
	if v == nil {
		return nil
	}
	switch v.Type {
	case ValNil:
		return nil
	case ValBool:
		return v.BoolVal
	case ValInt:
		return v.IntVal
	case ValFloat:
		return v.FloatVal
	case ValString:
		return v.StrVal
	case ValArray:
		if v.ArrayVal == nil {
			if ints := v.intLane(); ints != nil {
				list := make([]any, len(ints))
				for i, n := range ints {
					list[i] = n
				}
				return list
			}
			if floats := v.floatLane(); floats != nil {
				list := make([]any, len(floats))
				for i, f := range floats {
					list[i] = f
				}
				return list
			}
			if strs := v.strLane(); strs != nil {
				n := strs.Len()
				list := make([]any, n)
				for i := 0; i < n; i++ {
					list[i] = strings.Clone(strs.At(i))
				}
				return list
			}
		}
		list := make([]any, 0, len(v.ArrayVal))
		for _, item := range v.ArrayVal {
			list = append(list, item.ToInterface())
		}
		return list
	case ValHash:
		m := make(map[string]any)
		for k, val := range v.HashVal {
			m[k] = val.ToInterface()
		}
		return m
	default:
		return v.String()
	}
}
