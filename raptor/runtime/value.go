package raptor

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// ValueType identifies the runtime kind of a Value.
type ValueType int

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
	Name    string
	Params  []Param
	Body    *BlockStmt
	Env     *Env
	IsMulti bool
	IsRaw   bool
}

// Value represents any runtime value in Raku5.
type Value struct {
	Type        ValueType
	BoolVal     bool
	IntVal      int64
	FloatVal    float64
	StrVal      string
	ArrayVal    []*Value
	HashVal     map[string]*Value
	ClosureVal  *Closure
	Candidates  []*Closure // For ValMultiSub: list of multi candidates
	PtrVal      uintptr
	CStructVal  *CStructInstance
	PromiseVal  *Promise
	ChannelVal  *Channel
	JunctionVal *Junction
	LazySeqVal  *LazySeq
	RefVal      *Value
}

func JunctionValue(kind JunctionKind, vals []*Value) *Value {
	return &Value{
		Type: ValJunction,
		JunctionVal: &Junction{
			Kind:   kind,
			Values: vals,
		},
	}
}

func LazySeqValue(items []*Value) *Value {
	return &Value{
		Type: ValLazySeq,
		LazySeqVal: &LazySeq{
			Items: items,
		},
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
	return &Value{Type: ValInt, IntVal: i}
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
	return &Value{Type: ValArray, ArrayVal: elems}
}

func HashValue(m map[string]*Value) *Value {
	return &Value{Type: ValHash, HashVal: m}
}

func ClosureValue(c *Closure) *Value {
	return &Value{Type: ValClosure, ClosureVal: c}
}

func MultiSubValue(candidates []*Closure) *Value {
	return &Value{Type: ValMultiSub, Candidates: candidates}
}

func NativePtrValue(ptr uintptr) *Value {
	return &Value{Type: ValNativePtr, PtrVal: ptr}
}

func CStructValue(inst *CStructInstance) *Value {
	return &Value{Type: ValCStruct, CStructVal: inst, PtrVal: inst.Ptr}
}

func PromiseValue(p *Promise) *Value {
	return &Value{Type: ValPromise, PromiseVal: p}
}

func ChannelValue(c *Channel) *Value {
	return &Value{Type: ValChannel, ChannelVal: c}
}

func RefValue(target *Value) *Value {
	return &Value{Type: ValRef, RefVal: target}
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
		return len(v.ArrayVal) > 0
	case ValHash:
		return len(v.HashVal) > 0
	case ValClosure, ValMultiSub, ValNativePtr, ValCStruct, ValPromise, ValChannel, ValRef:
		return true
	case ValJunction:
		if v.JunctionVal == nil {
			return false
		}
		switch v.JunctionVal.Kind {
		case JunctionAll:
			for _, elem := range v.JunctionVal.Values {
				if !elem.IsTrue() {
					return false
				}
			}
			return len(v.JunctionVal.Values) > 0
		case JunctionAny:
			for _, elem := range v.JunctionVal.Values {
				if elem.IsTrue() {
					return true
				}
			}
			return false
		case JunctionOne:
			count := 0
			for _, elem := range v.JunctionVal.Values {
				if elem.IsTrue() {
					count++
				}
			}
			return count == 1
		case JunctionNone:
			for _, elem := range v.JunctionVal.Values {
				if elem.IsTrue() {
					return false
				}
			}
			return true
		}
		return false
	case ValLazySeq:
		if v.LazySeqVal != nil {
			return len(v.LazySeqVal.Items) > 0
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
		if v.CStructVal != nil && v.CStructVal.Class != nil {
			return v.CStructVal.Class.Name
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
		if v.Type == ValCStruct && v.CStructVal != nil && v.CStructVal.Class != nil {
			return v.CStructVal.Class.Name == constraint
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
		return fmt.Sprintf("%d", v.IntVal)
	case ValFloat:
		return fmt.Sprintf("%g", v.FloatVal)
	case ValString:
		return v.StrVal
	case ValArray:
		var items []string
		for _, e := range v.ArrayVal {
			items = append(items, e.String())
		}
		return "[" + strings.Join(items, " ") + "]"
	case ValHash:
		var pairs []string
		for k, val := range v.HashVal {
			pairs = append(pairs, fmt.Sprintf("%s => %s", k, val.String()))
		}
		return "{" + strings.Join(pairs, ", ") + "}"
	case ValClosure:
		if v.ClosureVal != nil && v.ClosureVal.Name != "" {
			return fmt.Sprintf("sub %s { ... }", v.ClosureVal.Name)
		}
		return "sub { ... }"
	case ValMultiSub:
		return fmt.Sprintf("multi sub (%d candidates)", len(v.Candidates))
	case ValNativePtr:
		return fmt.Sprintf("0x%x", v.PtrVal)
	case ValPromise:
		if v.PromiseVal != nil {
			return fmt.Sprintf("Promise(%s)", v.PromiseVal.Status)
		}
		return "Promise"
	case ValChannel:
		return "Channel"
	case ValJunction:
		if v.JunctionVal != nil {
			var parts []string
			for _, elem := range v.JunctionVal.Values {
				parts = append(parts, elem.String())
			}
			switch v.JunctionVal.Kind {
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
		if v.LazySeqVal != nil {
			var parts []string
			for _, it := range v.LazySeqVal.Items {
				parts = append(parts, it.String())
			}
			return "(" + strings.Join(parts, " ") + "...)"
		}
		return "Seq(...)"
	case ValRef:
		if v.RefVal != nil {
			return "\\" + v.RefVal.String()
		}
		return "\\(nil)"
	case ValCStruct:
		if v.CStructVal != nil && v.CStructVal.Class != nil {
			return fmt.Sprintf("struct %s(0x%x)", v.CStructVal.Class.Name, v.CStructVal.Ptr)
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
		if v.RefVal == nil {
			return "SCALAR"
		}
		switch v.RefVal.Type {
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
		var list []any
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
