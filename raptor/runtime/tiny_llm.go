package raptor

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"unicode"
)

// Tiny character-level language model used by llm_tiny_* builtins.
// Forward is a log-linear n-gram mix plus a small linear layer
// (embed[last] ⊕ embed[prev]) @ W — that matmul can run on WebGPU.

const (
	tinyLMVocabSrc = "abcdefghijklmnopqrstuvwxyz 0123456789.,:-"
	tinyLMD        = 16
)

var tinyLMCorpus = strings.Join([]string{
	"raptor is a dynamic language. raptor is a dynamic language.",
	"raptor is a dynamic language that runs in the browser via webassembly.",
	"raptor is a tiny language. raptor is a tiny language.",
	"webgpu compute multiplies tensors for the tiny language model.",
	"ggml tensors add, multiply, and apply gelu then softmax.",
	"the model predicts the next character from a short context window.",
	"raptor calls the tiny llm through webgpu. raptor loves tensors.",
	"a tiny language model generates text one character at a time.",
	"compute shaders run matrix multiply on the gpu.",
	"the raptor tour plays a major seventh arpeggio over webaudio.",
	"tensor graphs compile to ggml and execute on the software backend.",
}, " ")

type tinyLM struct {
	v       int
	chars   []byte
	index   [128]int
	uni     []float64
	bi      [][]float64
	tri     [][][]float64
	embed   []float32 // [V, D]
	weight  []float32 // [2D, V] row-major
	trained bool
}

type tinyLMStore struct {
	mu      sync.Mutex
	next    int64
	models  map[int64]*tinyLM
	cached  *tinyLM
	backend string
}

var tinyLMs = &tinyLMStore{
	models:  make(map[int64]*tinyLM),
	backend: "cpu",
}

// tinyLMMatmulHook is set by the WASM WebGPU bridge when a GPU matmul is ready.
var tinyLMMatmulHook func(m, n, k int, a, b []float32) []float32

func newTinyLM() *tinyLM {
	chars := []byte(tinyLMVocabSrc)
	m := &tinyLM{
		v:     len(chars),
		chars: chars,
	}
	for i := range m.index {
		m.index[i] = -1
	}
	for i, ch := range chars {
		m.index[ch] = i
	}
	m.uni = make([]float64, m.v)
	m.bi = make([][]float64, m.v)
	m.tri = make([][][]float64, m.v)
	for i := 0; i < m.v; i++ {
		m.bi[i] = make([]float64, m.v)
		m.tri[i] = make([][]float64, m.v)
		for j := 0; j < m.v; j++ {
			m.tri[i][j] = make([]float64, m.v)
		}
	}
	m.embed = make([]float32, m.v*tinyLMD)
	m.weight = make([]float32, 2*tinyLMD*m.v)
	return m
}

func (m *tinyLM) encode(ch byte) int {
	if ch >= 'A' && ch <= 'Z' {
		ch = ch - 'A' + 'a'
	}
	if ch > 127 {
		return m.index[' ']
	}
	if id := m.index[ch]; id >= 0 {
		return id
	}
	if unicode.IsSpace(rune(ch)) {
		return m.index[' ']
	}
	return m.index[' ']
}

func (m *tinyLM) train(corpus string) {
	ids := make([]int, 0, len(corpus))
	for i := 0; i < len(corpus); i++ {
		ids = append(ids, m.encode(corpus[i]))
	}
	if len(ids) < 3 {
		return
	}
	for i := 0; i < len(ids); i++ {
		m.uni[ids[i]]++
		if i > 0 {
			m.bi[ids[i-1]][ids[i]]++
		}
		if i > 1 {
			m.tri[ids[i-2]][ids[i-1]][ids[i]]++
		}
	}
	logNorm := func(row []float64) {
		var s float64
		for _, v := range row {
			s += v
		}
		k := 0.02
		if s <= 0 {
			inv := 1.0 / float64(len(row))
			for i := range row {
				row[i] = math.Log(inv)
			}
			return
		}
		den := s + k*float64(len(row))
		for i, v := range row {
			row[i] = math.Log((v + k) / den)
		}
	}
	logNorm(m.uni)
	for i := 0; i < m.v; i++ {
		logNorm(m.bi[i])
		for j := 0; j < m.v; j++ {
			logNorm(m.tri[i][j])
		}
	}

	rng := rand.New(rand.NewSource(42))
	scale := float32(1.0 / math.Sqrt(float64(tinyLMD)))
	for i := range m.embed {
		m.embed[i] = (rng.Float32()*2 - 1) * scale
	}
	for i := range m.weight {
		m.weight[i] = (rng.Float32()*2 - 1) * scale
	}

	// A few SGD steps so the linear layer is not random noise.
	lr := float32(0.08)
	for step := 0; step < 80; step++ {
		i := 2 + rng.Intn(len(ids)-2)
		prev, last, target := ids[i-2], ids[i-1], ids[i]
		h := m.hidden(prev, last)
		logits := m.linear(h)
		probs := softmaxF32(logits)
		// dW = h^T * (p - y)
		for v := 0; v < m.v; v++ {
			grad := probs[v]
			if v == target {
				grad -= 1
			}
			for d := 0; d < 2*tinyLMD; d++ {
				m.weight[d*m.v+v] -= lr * h[d] * grad
			}
		}
	}
	m.trained = true
}

func (m *tinyLM) hidden(prev, last int) []float32 {
	h := make([]float32, 2*tinyLMD)
	copy(h[0:tinyLMD], m.embed[last*tinyLMD:(last+1)*tinyLMD])
	copy(h[tinyLMD:], m.embed[prev*tinyLMD:(prev+1)*tinyLMD])
	return h
}

func cpuMatmul(m, n, k int, a, b []float32) []float32 {
	out := make([]float32, m*n)
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			var acc float32
			for t := 0; t < k; t++ {
				acc += a[i*k+t] * b[t*n+j]
			}
			out[i*n+j] = acc
		}
	}
	return out
}

func tinyLMMatmul(m, n, k int, a, b []float32) []float32 {
	if tinyLMMatmulHook != nil {
		if out := tinyLMMatmulHook(m, n, k, a, b); len(out) == m*n {
			tinyLMs.mu.Lock()
			tinyLMs.backend = "webgpu"
			tinyLMs.mu.Unlock()
			return out
		}
	}
	tinyLMs.mu.Lock()
	if tinyLMs.backend != "webgpu" {
		tinyLMs.backend = "cpu"
	}
	tinyLMs.mu.Unlock()
	return cpuMatmul(m, n, k, a, b)
}

func (m *tinyLM) linear(h []float32) []float32 {
	// logits[1, V] = h[1, 2D] @ W[2D, V]
	return tinyLMMatmul(1, m.v, 2*tinyLMD, h, m.weight)
}

func logSoftmaxF32(x []float32) []float32 {
	p := softmaxF32(x)
	out := make([]float32, len(p))
	for i, v := range p {
		if v <= 0 {
			out[i] = -20
			continue
		}
		out[i] = float32(math.Log(float64(v)))
	}
	return out
}

func softmaxF32(x []float32) []float32 {
	maxv := float64(x[0])
	for _, v := range x[1:] {
		if float64(v) > maxv {
			maxv = float64(v)
		}
	}
	out := make([]float32, len(x))
	var sum float64
	for i, v := range x {
		e := math.Exp(float64(v) - maxv)
		out[i] = float32(e)
		sum += e
	}
	if sum == 0 {
		inv := float32(1.0 / float64(len(x)))
		for i := range out {
			out[i] = inv
		}
		return out
	}
	inv := float32(1.0 / sum)
	for i := range out {
		out[i] *= inv
	}
	return out
}

func (m *tinyLM) logits(text string) []float32 {
	prev, last := m.encode(' '), m.encode(' ')
	if n := len(text); n > 0 {
		last = m.encode(text[n-1])
		if n > 1 {
			prev = m.encode(text[n-2])
		}
	}
	nn := m.linear(m.hidden(prev, last))
	nnLog := logSoftmaxF32(nn)
	out := make([]float32, m.v)
	for i := 0; i < m.v; i++ {
		ng := 0.70*m.tri[prev][last][i] + 0.22*m.bi[last][i] + 0.08*m.uni[i]
		out[i] = 0.92*float32(ng) + 0.08*nnLog[i]
	}
	if len(text) >= 3 {
		tail := text[len(text)-3:]
		for i, ch := range m.chars {
			if strings.Count(text, tail+string(ch)) >= 1 {
				out[i] -= 2.0
			}
		}
	}
	return out
}

func (m *tinyLM) sample(logits []float32, temp float64, rng *rand.Rand) int {
	if temp <= 0 {
		best := 0
		for i := 1; i < len(logits); i++ {
			if logits[i] > logits[best] {
				best = i
			}
		}
		return best
	}
	scaled := make([]float32, len(logits))
	inv := 1.0 / temp
	for i, v := range logits {
		scaled[i] = float32(float64(v) * inv)
	}
	p := softmaxF32(scaled)
	r := rng.Float64()
	var acc float64
	for i, v := range p {
		acc += float64(v)
		if r <= acc {
			return i
		}
	}
	return len(p) - 1
}

func (m *tinyLM) generate(prompt string, n int, temp float64) string {
	if n < 1 {
		n = 1
	}
	if n > 256 {
		n = 256
	}
	var b strings.Builder
	b.WriteString(prompt)
	rng := rand.New(rand.NewSource(int64(len(prompt)*97 + n*13 + int(temp*1000))))
	text := prompt
	if text == "" {
		text = "r"
		b.Reset()
		b.WriteString(text)
	}
	for i := 0; i < n; i++ {
		ch := m.chars[m.sample(m.logits(text), temp, rng)]
		b.WriteByte(ch)
		text += string(ch)
		if len(text) > 64 {
			text = text[len(text)-64:]
		}
	}
	return b.String()
}

func tinyLMGetOrTrain() *tinyLM {
	tinyLMs.mu.Lock()
	if tinyLMs.cached != nil {
		m := tinyLMs.cached
		tinyLMs.mu.Unlock()
		return m
	}
	tinyLMs.mu.Unlock()
	m := newTinyLM()
	m.train(tinyLMCorpus)
	tinyLMs.mu.Lock()
	tinyLMs.cached = m
	tinyLMs.next++
	id := tinyLMs.next
	tinyLMs.models[id] = m
	tinyLMs.mu.Unlock()
	return m
}

func (in *Interp) registerTinyLLMBuiltins() {
	in.Builtins["llm_tiny_load"] = func(in *Interp, args []*Value) (*Value, error) {
		m := tinyLMGetOrTrain()
		tinyLMs.mu.Lock()
		defer tinyLMs.mu.Unlock()
		for id, existing := range tinyLMs.models {
			if existing == m {
				return IntValue(id), nil
			}
		}
		tinyLMs.next++
		id := tinyLMs.next
		tinyLMs.models[id] = m
		return IntValue(id), nil
	}

	in.Builtins["llm_tiny_backend"] = func(in *Interp, args []*Value) (*Value, error) {
		tinyLMs.mu.Lock()
		defer tinyLMs.mu.Unlock()
		return StringValue(tinyLMs.backend), nil
	}

	in.Builtins["llm_tiny_vocab"] = func(in *Interp, args []*Value) (*Value, error) {
		return StringValue(tinyLMVocabSrc), nil
	}

	lookup := func(args []*Value) *tinyLM {
		tinyLMs.mu.Lock()
		defer tinyLMs.mu.Unlock()
		if len(args) > 0 {
			if m := tinyLMs.models[ggmlAsInt(args[0])]; m != nil {
				return m
			}
		}
		return tinyLMs.cached
	}

	in.Builtins["llm_tiny_logits"] = func(in *Interp, args []*Value) (*Value, error) {
		m := lookup(args)
		if m == nil {
			m = tinyLMGetOrTrain()
		}
		text := ""
		if len(args) >= 2 {
			text = args[1].String()
		} else if len(args) == 1 && args[0].Type == ValString {
			text = args[0].String()
		}
		logits := m.logits(text)
		out := make([]*Value, len(logits))
		for i, v := range logits {
			out[i] = FloatValue(float64(v))
		}
		return ArrayValue(out), nil
	}

	in.Builtins["llm_tiny_sample"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray {
			return StringValue(""), fmt.Errorf("llm_tiny_sample(logits, temperature?)")
		}
		logits := make([]float32, len(args[0].ArrayVal))
		for i, v := range args[0].ArrayVal {
			logits[i] = float32(ggmlAsFloat(v))
		}
		temp := 0.8
		if len(args) > 1 {
			temp = ggmlAsFloat(args[1])
		}
		m := tinyLMGetOrTrain()
		rng := rand.New(rand.NewSource(int64(logits[0]*1000) + int64(len(logits))))
		idx := m.sample(logits, temp, rng)
		if idx < 0 || idx >= len(m.chars) {
			return StringValue(""), nil
		}
		return StringValue(string(m.chars[idx])), nil
	}

	in.Builtins["llm_tiny_generate"] = func(in *Interp, args []*Value) (*Value, error) {
		m := tinyLMGetOrTrain()
		prompt := "raptor is "
		n := 48
		temp := 0.35
		// llm_tiny_generate(model, prompt, n, temp) or (prompt, n, temp)
		off := 0
		if len(args) > 0 && (args[0].Type == ValInt || args[0].Type == ValFloat) {
			if got := lookup(args[:1]); got != nil {
				m = got
				off = 1
			}
		}
		if len(args) > off {
			prompt = args[off].String()
		}
		if len(args) > off+1 {
			n = int(ggmlAsInt(args[off+1]))
		}
		if len(args) > off+2 {
			temp = ggmlAsFloat(args[off+2])
		}
		return StringValue(m.generate(prompt, n, temp)), nil
	}
}
