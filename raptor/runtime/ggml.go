package raptor

import (
	"fmt"
	"math"
	"strconv"
	"sync"
)

// GGML C API type tags — values match ggml.h (ggml_type).
const (
	GGMLTypeF32 = 0
	GGMLTypeF16 = 1
	GGMLTypeI8  = 16
	GGMLTypeI16 = 17
	GGMLTypeI32 = 18
	GGMLTypeI64 = 19
	GGMLTypeF64 = 20
)

// Graph op tags. Leaves are opNone.
const (
	ggmlOpNone = iota
	ggmlOpDup
	ggmlOpAdd
	ggmlOpSub
	ggmlOpMul
	ggmlOpDiv
	ggmlOpSqr
	ggmlOpSqrt
	ggmlOpLog
	ggmlOpAbs
	ggmlOpSgn
	ggmlOpNeg
	ggmlOpScale
	ggmlOpSum
	ggmlOpMean
	ggmlOpNorm
	ggmlOpRMSNorm
	ggmlOpMulMat
	ggmlOpTranspose
	ggmlOpReshape
	ggmlOpCont
	ggmlOpRelu
	ggmlOpGelu
	ggmlOpSilu
	ggmlOpTanh
	ggmlOpSigmoid
	ggmlOpSoftMax
	ggmlOpCpy
)

type ggmlTensor struct {
	id      int64
	typ     int
	ne      [4]int64
	nDim    int
	name    string
	data    []float32
	op      int
	src     []*ggmlTensor
	scale   float64
	reshape [4]int64
	eps     float64
}

type ggmlGraph struct {
	id    int64
	nodes []*ggmlTensor
}

type ggmlContext struct {
	id      int64
	memSize int64
	next    int64
	tensors map[int64]*ggmlTensor
	graphs  map[int64]*ggmlGraph
}

type ggmlEngine struct {
	mu      sync.Mutex
	next    int64
	ctxs    map[int64]*ggmlContext
	backend string
	native  string
}

var ggmlState = &ggmlEngine{
	ctxs:    make(map[int64]*ggmlContext),
	backend: "software",
}

func ggmlAsFloat(v *Value) float64 {
	if v == nil {
		return 0
	}
	switch v.Type {
	case ValFloat:
		return v.FloatVal
	case ValInt:
		return float64(v.IntVal)
	case ValString:
		f, _ := strconv.ParseFloat(v.StrVal, 64)
		return f
	default:
		return 0
	}
}

func ggmlAsInt(v *Value) int64 {
	if v == nil {
		return 0
	}
	switch v.Type {
	case ValInt:
		return v.IntVal
	case ValFloat:
		return int64(v.FloatVal)
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

func (t *ggmlTensor) nelements() int64 {
	n := int64(1)
	for i := 0; i < t.nDim; i++ {
		n *= t.ne[i]
	}
	if n < 1 {
		return 0
	}
	return n
}

func (t *ggmlTensor) nbytes() int64 {
	return t.nelements() * 4
}

func (t *ggmlTensor) ensureData() {
	n := int(t.nelements())
	if n < 1 {
		n = 1
	}
	if len(t.data) != n {
		t.data = make([]float32, n)
	}
}

func (t *ggmlTensor) idx(i0, i1, i2, i3 int64) int {
	return int(i0 + i1*t.ne[0] + i2*t.ne[0]*t.ne[1] + i3*t.ne[0]*t.ne[1]*t.ne[2])
}

func (c *ggmlContext) alloc(t *ggmlTensor) *ggmlTensor {
	// Global ids: ggml_get_* looks up tensors across contexts by handle.
	ggmlState.next++
	t.id = ggmlState.next
	t.ensureData()
	c.tensors[t.id] = t
	return t
}

func (c *ggmlContext) tensor(id int64) *ggmlTensor {
	return c.tensors[id]
}

func (e *ggmlEngine) ctx(id int64) *ggmlContext {
	return e.ctxs[id]
}

func ggmlNewTensor(c *ggmlContext, typ int, dims ...int64) *ggmlTensor {
	t := &ggmlTensor{typ: typ, nDim: len(dims), op: ggmlOpNone}
	for i, d := range dims {
		if i < 4 {
			if d < 1 {
				d = 1
			}
			t.ne[i] = d
		}
	}
	if t.nDim < 1 {
		t.nDim = 1
		t.ne[0] = 1
	}
	return c.alloc(t)
}

func ggmlUnary(c *ggmlContext, a *ggmlTensor, op int) *ggmlTensor {
	t := &ggmlTensor{typ: a.typ, nDim: a.nDim, ne: a.ne, op: op, src: []*ggmlTensor{a}}
	return c.alloc(t)
}

func ggmlBinary(c *ggmlContext, a, b *ggmlTensor, op int) *ggmlTensor {
	t := &ggmlTensor{typ: a.typ, nDim: a.nDim, ne: a.ne, op: op, src: []*ggmlTensor{a, b}}
	return c.alloc(t)
}

func ggmlComputeTensor(t *ggmlTensor) {
	if t == nil {
		return
	}
	for _, s := range t.src {
		ggmlComputeTensor(s)
	}
	t.ensureData()
	switch t.op {
	case ggmlOpNone, ggmlOpCont, ggmlOpCpy:
		if t.op == ggmlOpCpy && len(t.src) > 0 {
			copy(t.data, t.src[0].data)
		}
	case ggmlOpDup:
		if len(t.src) > 0 {
			copy(t.data, t.src[0].data)
		}
	case ggmlOpAdd, ggmlOpSub, ggmlOpMul, ggmlOpDiv:
		a, b := t.src[0], t.src[1]
		n := len(t.data)
		for i := 0; i < n; i++ {
			av := a.data[i%len(a.data)]
			bv := b.data[i%len(b.data)]
			switch t.op {
			case ggmlOpAdd:
				t.data[i] = av + bv
			case ggmlOpSub:
				t.data[i] = av - bv
			case ggmlOpMul:
				t.data[i] = av * bv
			case ggmlOpDiv:
				if bv == 0 {
					t.data[i] = 0
				} else {
					t.data[i] = av / bv
				}
			}
		}
	case ggmlOpSqr, ggmlOpSqrt, ggmlOpLog, ggmlOpAbs, ggmlOpSgn, ggmlOpNeg,
		ggmlOpRelu, ggmlOpGelu, ggmlOpSilu, ggmlOpTanh, ggmlOpSigmoid:
		a := t.src[0]
		for i := range t.data {
			x := float64(a.data[i])
			var y float64
			switch t.op {
			case ggmlOpSqr:
				y = x * x
			case ggmlOpSqrt:
				y = math.Sqrt(math.Max(x, 0))
			case ggmlOpLog:
				if x <= 0 {
					y = math.Inf(-1)
				} else {
					y = math.Log(x)
				}
			case ggmlOpAbs:
				y = math.Abs(x)
			case ggmlOpSgn:
				if x > 0 {
					y = 1
				} else if x < 0 {
					y = -1
				}
			case ggmlOpNeg:
				y = -x
			case ggmlOpRelu:
				y = math.Max(x, 0)
			case ggmlOpGelu:
				// tanh approximation used by ggml
				y = 0.5 * x * (1 + math.Tanh(math.Sqrt(2/math.Pi)*(x+0.044715*x*x*x)))
			case ggmlOpSilu:
				y = x / (1 + math.Exp(-x))
			case ggmlOpTanh:
				y = math.Tanh(x)
			case ggmlOpSigmoid:
				y = 1 / (1 + math.Exp(-x))
			}
			t.data[i] = float32(y)
		}
	case ggmlOpScale:
		a := t.src[0]
		s := float32(t.scale)
		if len(t.src) > 1 && len(t.src[1].data) > 0 {
			s = t.src[1].data[0]
		}
		for i := range t.data {
			t.data[i] = a.data[i] * s
		}
	case ggmlOpSum:
		var s float32
		for _, v := range t.src[0].data {
			s += v
		}
		t.data[0] = s
	case ggmlOpMean:
		var s float32
		for _, v := range t.src[0].data {
			s += v
		}
		n := float32(len(t.src[0].data))
		if n == 0 {
			t.data[0] = 0
		} else {
			t.data[0] = s / n
		}
	case ggmlOpNorm:
		a := t.src[0]
		var mean float64
		for _, v := range a.data {
			mean += float64(v)
		}
		n := float64(len(a.data))
		if n == 0 {
			return
		}
		mean /= n
		var varr float64
		for _, v := range a.data {
			d := float64(v) - mean
			varr += d * d
		}
		inv := 1.0 / math.Sqrt(varr/n+t.eps)
		for i, v := range a.data {
			t.data[i] = float32((float64(v) - mean) * inv)
		}
	case ggmlOpRMSNorm:
		a := t.src[0]
		var ms float64
		for _, v := range a.data {
			ms += float64(v) * float64(v)
		}
		n := float64(len(a.data))
		inv := 1.0 / math.Sqrt(ms/n+t.eps)
		for i, v := range a.data {
			t.data[i] = float32(float64(v) * inv)
		}
	case ggmlOpMulMat:
		// dst = A^T * B   (ggml.h)
		// A: [k, m], B: [k, n], dst: [m, n]
		A, B := t.src[0], t.src[1]
		k := int(A.ne[0])
		m := int(A.ne[1])
		n := int(B.ne[1])
		if B.nDim == 1 {
			n = 1
		}
		for j := 0; j < n; j++ {
			for i := 0; i < m; i++ {
				var acc float32
				for kk := 0; kk < k; kk++ {
					acc += A.data[A.idx(int64(kk), int64(i), 0, 0)] * B.data[B.idx(int64(kk), int64(j), 0, 0)]
				}
				t.data[t.idx(int64(i), int64(j), 0, 0)] = acc
			}
		}
	case ggmlOpTranspose:
		a := t.src[0]
		rows := int(a.ne[0])
		cols := int(a.ne[1])
		if a.nDim < 2 {
			cols = 1
		}
		for j := 0; j < cols; j++ {
			for i := 0; i < rows; i++ {
				t.data[t.idx(int64(j), int64(i), 0, 0)] = a.data[a.idx(int64(i), int64(j), 0, 0)]
			}
		}
	case ggmlOpReshape:
		copy(t.data, t.src[0].data)
	case ggmlOpSoftMax:
		a := t.src[0]
		rows := int(a.ne[0])
		cols := 1
		if a.nDim > 1 {
			cols = int(a.ne[1])
		}
		for j := 0; j < cols; j++ {
			maxv := float64(a.data[a.idx(0, int64(j), 0, 0)])
			for i := 1; i < rows; i++ {
				if v := float64(a.data[a.idx(int64(i), int64(j), 0, 0)]); v > maxv {
					maxv = v
				}
			}
			var sum float64
			for i := 0; i < rows; i++ {
				e := math.Exp(float64(a.data[a.idx(int64(i), int64(j), 0, 0)]) - maxv)
				t.data[t.idx(int64(i), int64(j), 0, 0)] = float32(e)
				sum += e
			}
			if sum == 0 {
				continue
			}
			inv := 1.0 / sum
			for i := 0; i < rows; i++ {
				t.data[t.idx(int64(i), int64(j), 0, 0)] *= float32(inv)
			}
		}
	}
}

func ggmlLookup(ctxID, tensorID int64) (*ggmlContext, *ggmlTensor, error) {
	ggmlState.mu.Lock()
	defer ggmlState.mu.Unlock()
	c := ggmlState.ctx(ctxID)
	if c == nil {
		return nil, nil, fmt.Errorf("ggml: unknown context %d", ctxID)
	}
	t := c.tensor(tensorID)
	if t == nil {
		return nil, nil, fmt.Errorf("ggml: unknown tensor %d", tensorID)
	}
	return c, t, nil
}

func ggmlRequireCtx(args []*Value, n int, name string) (*ggmlContext, error) {
	if len(args) < n {
		return nil, fmt.Errorf("%s: expected at least %d args", name, n)
	}
	ggmlState.mu.Lock()
	defer ggmlState.mu.Unlock()
	c := ggmlState.ctx(ggmlAsInt(args[0]))
	if c == nil {
		return nil, fmt.Errorf("%s: unknown context", name)
	}
	return c, nil
}

func (in *Interp) registerGGMLBuiltins() {
	ok, path := ggmlProbeNative()
	ggmlState.mu.Lock()
	if ok {
		ggmlState.native = path
	}
	ggmlState.mu.Unlock()

	in.GlobalEnv.Define("$GGML_TYPE_F32", IntValue(GGMLTypeF32))
	in.GlobalEnv.Define("$GGML_TYPE_F16", IntValue(GGMLTypeF16))
	in.GlobalEnv.Define("$GGML_TYPE_I8", IntValue(GGMLTypeI8))
	in.GlobalEnv.Define("$GGML_TYPE_I16", IntValue(GGMLTypeI16))
	in.GlobalEnv.Define("$GGML_TYPE_I32", IntValue(GGMLTypeI32))
	in.GlobalEnv.Define("$GGML_TYPE_I64", IntValue(GGMLTypeI64))
	in.GlobalEnv.Define("$GGML_TYPE_F64", IntValue(GGMLTypeF64))

	in.Builtins["ggml_init"] = func(in *Interp, args []*Value) (*Value, error) {
		mem := int64(16 << 20)
		if len(args) > 0 {
			mem = ggmlAsInt(args[0])
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		ggmlState.next++
		id := ggmlState.next
		ggmlState.ctxs[id] = &ggmlContext{
			id:      id,
			memSize: mem,
			tensors: make(map[int64]*ggmlTensor),
			graphs:  make(map[int64]*ggmlGraph),
		}
		return IntValue(id), nil
	}

	in.Builtins["ggml_free"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		id := ggmlAsInt(args[0])
		ggmlState.mu.Lock()
		delete(ggmlState.ctxs, id)
		ggmlState.mu.Unlock()
		return BoolValue(true), nil
	}

	in.Builtins["ggml_used_mem"] = func(in *Interp, args []*Value) (*Value, error) {
		c, err := ggmlRequireCtx(args, 1, "ggml_used_mem")
		if err != nil {
			return IntValue(0), err
		}
		var n int64
		ggmlState.mu.Lock()
		for _, t := range c.tensors {
			n += t.nbytes()
		}
		ggmlState.mu.Unlock()
		return IntValue(n), nil
	}

	in.Builtins["ggml_backend"] = func(in *Interp, args []*Value) (*Value, error) {
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		if ggmlState.native != "" {
			return StringValue("software+" + ggmlState.native), nil
		}
		return StringValue(ggmlState.backend), nil
	}

	in.Builtins["ggml_native_available"] = func(in *Interp, args []*Value) (*Value, error) {
		ok, path := ggmlProbeNative()
		if !ok {
			return BoolValue(false), nil
		}
		return StringValue(path), nil
	}

	in.Builtins["ggml_time_us"] = func(in *Interp, args []*Value) (*Value, error) {
		if us, ok := ggmlNativeTimeUs(); ok {
			return IntValue(us), nil
		}
		return IntValue(0), nil
	}

	new1d := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("ggml_new_tensor_1d(ctx, type, ne0)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_new_tensor_1d: unknown context")
		}
		t := ggmlNewTensor(c, int(ggmlAsInt(args[1])), ggmlAsInt(args[2]))
		return IntValue(t.id), nil
	}
	new2d := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 4 {
			return nil, fmt.Errorf("ggml_new_tensor_2d(ctx, type, ne0, ne1)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_new_tensor_2d: unknown context")
		}
		t := ggmlNewTensor(c, int(ggmlAsInt(args[1])), ggmlAsInt(args[2]), ggmlAsInt(args[3]))
		return IntValue(t.id), nil
	}
	new3d := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 5 {
			return nil, fmt.Errorf("ggml_new_tensor_3d(ctx, type, ne0, ne1, ne2)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_new_tensor_3d: unknown context")
		}
		t := ggmlNewTensor(c, int(ggmlAsInt(args[1])), ggmlAsInt(args[2]), ggmlAsInt(args[3]), ggmlAsInt(args[4]))
		return IntValue(t.id), nil
	}
	in.Builtins["ggml_new_tensor_1d"] = new1d
	in.Builtins["ggml_new_tensor_2d"] = new2d
	in.Builtins["ggml_new_tensor_3d"] = new3d

	in.Builtins["ggml_set_name"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				t.name = args[1].String()
				return BoolValue(true), nil
			}
		}
		return BoolValue(false), nil
	}
	in.Builtins["ggml_get_name"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				return StringValue(t.name), nil
			}
		}
		return StringValue(""), nil
	}

	in.Builtins["ggml_n_dims"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				return IntValue(int64(t.nDim)), nil
			}
		}
		return IntValue(0), nil
	}
	in.Builtins["ggml_nelements"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				return IntValue(t.nelements()), nil
			}
		}
		return IntValue(0), nil
	}
	in.Builtins["ggml_nbytes"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				return IntValue(t.nbytes()), nil
			}
		}
		return IntValue(0), nil
	}
	in.Builtins["ggml_type_size"] = func(in *Interp, args []*Value) (*Value, error) {
		return IntValue(4), nil
	}

	in.Builtins["ggml_set_zero"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				t.ensureData()
				for i := range t.data {
					t.data[i] = 0
				}
				return IntValue(t.id), nil
			}
		}
		return BoolValue(false), nil
	}

	in.Builtins["ggml_set_f32"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		v := float32(ggmlAsFloat(args[1]))
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				t.ensureData()
				for i := range t.data {
					t.data[i] = v
				}
				return IntValue(t.id), nil
			}
		}
		return BoolValue(false), nil
	}

	in.Builtins["ggml_set_f32_1d"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return BoolValue(false), nil
		}
		i := int(ggmlAsInt(args[1]))
		v := float32(ggmlAsFloat(args[2]))
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				t.ensureData()
				if i >= 0 && i < len(t.data) {
					t.data[i] = v
				}
				return BoolValue(true), nil
			}
		}
		return BoolValue(false), nil
	}

	in.Builtins["ggml_get_f32_1d"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return FloatValue(0), nil
		}
		i := int(ggmlAsInt(args[1]))
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				if i >= 0 && i < len(t.data) {
					return FloatValue(float64(t.data[i])), nil
				}
			}
		}
		return FloatValue(0), nil
	}

	in.Builtins["ggml_set_f32_nd"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 4 {
			return BoolValue(false), nil
		}
		i0 := ggmlAsInt(args[1])
		i1 := ggmlAsInt(args[2])
		v := float32(ggmlAsFloat(args[len(args)-1]))
		if len(args) >= 5 {
			// (tensor, i0, i1, v) or (tensor, i0, i1, i2, v)
			if len(args) == 4 {
				v = float32(ggmlAsFloat(args[3]))
			}
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				t.ensureData()
				idx := t.idx(i0, i1, 0, 0)
				if idx >= 0 && idx < len(t.data) {
					t.data[idx] = v
				}
				return BoolValue(true), nil
			}
		}
		return BoolValue(false), nil
	}

	in.Builtins["ggml_get_f32_nd"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return FloatValue(0), nil
		}
		i0 := ggmlAsInt(args[1])
		i1 := ggmlAsInt(args[2])
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				idx := t.idx(i0, i1, 0, 0)
				if idx >= 0 && idx < len(t.data) {
					return FloatValue(float64(t.data[idx])), nil
				}
			}
		}
		return FloatValue(0), nil
	}

	in.Builtins["ggml_set_data"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[1].Type != ValArray {
			return BoolValue(false), nil
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				t.ensureData()
				for i, item := range args[1].ArrayVal {
					if i >= len(t.data) {
						break
					}
					t.data[i] = float32(ggmlAsFloat(item))
				}
				return BoolValue(true), nil
			}
		}
		return BoolValue(false), nil
	}

	in.Builtins["ggml_get_data"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return ArrayValue(nil), nil
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			if t := c.tensor(ggmlAsInt(args[0])); t != nil {
				out := make([]*Value, len(t.data))
				for i, v := range t.data {
					out[i] = FloatValue(float64(v))
				}
				return ArrayValue(out), nil
			}
		}
		return ArrayValue(nil), nil
	}

	unary := func(name string, op int) BuiltinFunc {
		return func(in *Interp, args []*Value) (*Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("%s(ctx, tensor)", name)
			}
			ggmlState.mu.Lock()
			defer ggmlState.mu.Unlock()
			c := ggmlState.ctx(ggmlAsInt(args[0]))
			if c == nil {
				return nil, fmt.Errorf("%s: unknown context", name)
			}
			a := c.tensor(ggmlAsInt(args[1]))
			if a == nil {
				return nil, fmt.Errorf("%s: unknown tensor", name)
			}
			t := ggmlUnary(c, a, op)
			if op == ggmlOpNorm || op == ggmlOpRMSNorm {
				t.eps = 1e-5
				if len(args) > 2 {
					t.eps = ggmlAsFloat(args[2])
				}
			}
			return IntValue(t.id), nil
		}
	}
	binary := func(name string, op int) BuiltinFunc {
		return func(in *Interp, args []*Value) (*Value, error) {
			if len(args) < 3 {
				return nil, fmt.Errorf("%s(ctx, a, b)", name)
			}
			ggmlState.mu.Lock()
			defer ggmlState.mu.Unlock()
			c := ggmlState.ctx(ggmlAsInt(args[0]))
			if c == nil {
				return nil, fmt.Errorf("%s: unknown context", name)
			}
			a := c.tensor(ggmlAsInt(args[1]))
			b := c.tensor(ggmlAsInt(args[2]))
			if a == nil || b == nil {
				return nil, fmt.Errorf("%s: unknown tensor", name)
			}
			t := ggmlBinary(c, a, b, op)
			return IntValue(t.id), nil
		}
	}

	in.Builtins["ggml_dup"] = unary("ggml_dup", ggmlOpDup)
	in.Builtins["ggml_add"] = binary("ggml_add", ggmlOpAdd)
	in.Builtins["ggml_sub"] = binary("ggml_sub", ggmlOpSub)
	in.Builtins["ggml_mul"] = binary("ggml_mul", ggmlOpMul)
	in.Builtins["ggml_div"] = binary("ggml_div", ggmlOpDiv)
	in.Builtins["ggml_sqr"] = unary("ggml_sqr", ggmlOpSqr)
	in.Builtins["ggml_sqrt"] = unary("ggml_sqrt", ggmlOpSqrt)
	in.Builtins["ggml_log"] = unary("ggml_log", ggmlOpLog)
	in.Builtins["ggml_abs"] = unary("ggml_abs", ggmlOpAbs)
	in.Builtins["ggml_sgn"] = unary("ggml_sgn", ggmlOpSgn)
	in.Builtins["ggml_neg"] = unary("ggml_neg", ggmlOpNeg)
	in.Builtins["ggml_sum"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("ggml_sum(ctx, tensor)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_sum: unknown context")
		}
		a := c.tensor(ggmlAsInt(args[1]))
		if a == nil {
			return nil, fmt.Errorf("ggml_sum: unknown tensor")
		}
		t := ggmlNewTensor(c, a.typ, 1)
		t.op = ggmlOpSum
		t.src = []*ggmlTensor{a}
		return IntValue(t.id), nil
	}
	in.Builtins["ggml_mean"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("ggml_mean(ctx, tensor)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_mean: unknown context")
		}
		a := c.tensor(ggmlAsInt(args[1]))
		if a == nil {
			return nil, fmt.Errorf("ggml_mean: unknown tensor")
		}
		t := ggmlNewTensor(c, a.typ, 1)
		t.op = ggmlOpMean
		t.src = []*ggmlTensor{a}
		return IntValue(t.id), nil
	}
	in.Builtins["ggml_norm"] = unary("ggml_norm", ggmlOpNorm)
	in.Builtins["ggml_rms_norm"] = unary("ggml_rms_norm", ggmlOpRMSNorm)
	in.Builtins["ggml_relu"] = unary("ggml_relu", ggmlOpRelu)
	in.Builtins["ggml_gelu"] = unary("ggml_gelu", ggmlOpGelu)
	in.Builtins["ggml_silu"] = unary("ggml_silu", ggmlOpSilu)
	in.Builtins["ggml_tanh"] = unary("ggml_tanh", ggmlOpTanh)
	in.Builtins["ggml_sigmoid"] = unary("ggml_sigmoid", ggmlOpSigmoid)
	in.Builtins["ggml_soft_max"] = unary("ggml_soft_max", ggmlOpSoftMax)
	in.Builtins["ggml_cont"] = unary("ggml_cont", ggmlOpCont)

	in.Builtins["ggml_scale"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("ggml_scale(ctx, tensor, scale)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_scale: unknown context")
		}
		a := c.tensor(ggmlAsInt(args[1]))
		if a == nil {
			return nil, fmt.Errorf("ggml_scale: unknown tensor")
		}
		t := ggmlUnary(c, a, ggmlOpScale)
		if args[2].Type == ValInt {
			if s := c.tensor(ggmlAsInt(args[2])); s != nil && s != a {
				t.src = []*ggmlTensor{a, s}
			} else {
				t.scale = ggmlAsFloat(args[2])
			}
		} else {
			t.scale = ggmlAsFloat(args[2])
		}
		return IntValue(t.id), nil
	}

	in.Builtins["ggml_mul_mat"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("ggml_mul_mat(ctx, a, b)  # dest = A^T * B")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_mul_mat: unknown context")
		}
		a := c.tensor(ggmlAsInt(args[1]))
		b := c.tensor(ggmlAsInt(args[2]))
		if a == nil || b == nil {
			return nil, fmt.Errorf("ggml_mul_mat: unknown tensor")
		}
		m := a.ne[1]
		if a.nDim < 2 {
			m = 1
		}
		n := b.ne[1]
		if b.nDim < 2 {
			n = 1
		}
		t := ggmlNewTensor(c, a.typ, m, n)
		t.op = ggmlOpMulMat
		t.src = []*ggmlTensor{a, b}
		return IntValue(t.id), nil
	}

	in.Builtins["ggml_transpose"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("ggml_transpose(ctx, tensor)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_transpose: unknown context")
		}
		a := c.tensor(ggmlAsInt(args[1]))
		if a == nil {
			return nil, fmt.Errorf("ggml_transpose: unknown tensor")
		}
		n0, n1 := a.ne[1], a.ne[0]
		if a.nDim < 2 {
			n0 = 1
		}
		t := ggmlNewTensor(c, a.typ, n0, n1)
		t.op = ggmlOpTranspose
		t.src = []*ggmlTensor{a}
		return IntValue(t.id), nil
	}

	in.Builtins["ggml_reshape_1d"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("ggml_reshape_1d(ctx, tensor, ne0)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_reshape_1d: unknown context")
		}
		a := c.tensor(ggmlAsInt(args[1]))
		if a == nil {
			return nil, fmt.Errorf("ggml_reshape_1d: unknown tensor")
		}
		t := ggmlNewTensor(c, a.typ, ggmlAsInt(args[2]))
		t.op = ggmlOpReshape
		t.src = []*ggmlTensor{a}
		return IntValue(t.id), nil
	}
	in.Builtins["ggml_reshape_2d"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 4 {
			return nil, fmt.Errorf("ggml_reshape_2d(ctx, tensor, ne0, ne1)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_reshape_2d: unknown context")
		}
		a := c.tensor(ggmlAsInt(args[1]))
		if a == nil {
			return nil, fmt.Errorf("ggml_reshape_2d: unknown tensor")
		}
		t := ggmlNewTensor(c, a.typ, ggmlAsInt(args[2]), ggmlAsInt(args[3]))
		t.op = ggmlOpReshape
		t.src = []*ggmlTensor{a}
		return IntValue(t.id), nil
	}

	in.Builtins["ggml_new_graph"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("ggml_new_graph(ctx)")
		}
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		c := ggmlState.ctx(ggmlAsInt(args[0]))
		if c == nil {
			return nil, fmt.Errorf("ggml_new_graph: unknown context")
		}
		ggmlState.next++
		g := &ggmlGraph{id: ggmlState.next}
		c.graphs[g.id] = g
		return IntValue(g.id), nil
	}

	in.Builtins["ggml_build_forward_expand"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		gfID := ggmlAsInt(args[0])
		tID := ggmlAsInt(args[1])
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		for _, c := range ggmlState.ctxs {
			g := c.graphs[gfID]
			t := c.tensor(tID)
			if g != nil && t != nil {
				var walk func(n *ggmlTensor)
				walk = func(n *ggmlTensor) {
					if n == nil {
						return
					}
					for _, s := range n.src {
						walk(s)
					}
					g.nodes = append(g.nodes, n)
				}
				walk(t)
				return BoolValue(true), nil
			}
		}
		return BoolValue(false), nil
	}

	compute := func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		// C: ggml_graph_compute_with_ctx(ctx, gf, n_threads)
		// also accept ggml_graph_compute(gf) / (ctx, gf)
		var g *ggmlGraph
		ggmlState.mu.Lock()
		defer ggmlState.mu.Unlock()
		if len(args) >= 2 {
			c := ggmlState.ctx(ggmlAsInt(args[0]))
			if c != nil {
				g = c.graphs[ggmlAsInt(args[1])]
			}
			if g == nil {
				for _, cx := range ggmlState.ctxs {
					if gg := cx.graphs[ggmlAsInt(args[0])]; gg != nil {
						g = gg
						break
					}
				}
			}
		}
		if g == nil {
			return BoolValue(false), fmt.Errorf("ggml_graph_compute: unknown graph")
		}
		for _, n := range g.nodes {
			ggmlComputeTensor(n)
		}
		return BoolValue(true), nil
	}
	in.Builtins["ggml_graph_compute"] = compute
	in.Builtins["ggml_graph_compute_with_ctx"] = compute
}
