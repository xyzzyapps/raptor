package raptor

import (
	"fmt"
	goruntime "runtime"
	"slices"
	"sync"
)

// SortInt64Fast sorts an int64 slice in-place using O(n) counting sort for bounded ranges
// or 4-pass 16-bit LSD radix sort for large arrays, falling back to pdqsort for small slices.
func SortInt64Fast(xs []int64) {
	n := len(xs)
	if n < 2 {
		return
	}
	if n < 512 {
		slices.Sort(xs)
		return
	}

	minV := xs[0]
	maxV := xs[0]
	sorted := true
	for i := 1; i < n; i++ {
		v := xs[i]
		if v < xs[i-1] {
			sorted = false
		}
		if v < minV {
			minV = v
		} else if v > maxV {
			maxV = v
		}
	}
	if sorted {
		return
	}

	// O(n) dense counting sort when value span fits comfortably in L2 cache (<= 256K buckets = 1 MB)
	if maxV >= minV && uint64(maxV-minV) <= 262144 {
		span := int(maxV-minV) + 1
		counts := make([]uint32, span)
		for _, v := range xs {
			counts[int(v-minV)]++
		}
		outIdx := 0
		for b, c := range counts {
			if c == 0 {
				continue
			}
			val := minV + int64(b)
			for k := uint32(0); k < c; k++ {
				xs[outIdx] = val
				outIdx++
			}
		}
		return
	}

	// 4-pass 16-bit LSD Radix Sort with sign-bit flip for arbitrary 64-bit signed integers
	if n >= 2048 {
		buf := make([]uint64, n)
		src := make([]uint64, n)
		const signMask = uint64(1) << 63
		for i, v := range xs {
			src[i] = uint64(v) ^ signMask
		}
		var hist [65536]uint32
		for shift := uint(0); shift < 64; shift += 16 {
			clear(hist[:])
			for _, u := range src {
				hist[(u>>shift)&0xFFFF]++
			}
			// Check if entire pass falls in a single bucket (e.g. upper 32/48 bits all identical!)
			skipPass := false
			var sum uint32
			for b := 0; b < 65536; b++ {
				c := hist[b]
				if c == uint32(n) {
					skipPass = true
					break
				}
				hist[b] = sum
				sum += c
			}
			if skipPass {
				continue
			}
			for _, u := range src {
				idx := (u >> shift) & 0xFFFF
				pos := hist[idx]
				buf[pos] = u
				hist[idx] = pos + 1
			}
			src, buf = buf, src
		}
		for i, u := range src {
			xs[i] = int64(u ^ signMask)
		}
		return
	}

	slices.Sort(xs)
}

func (in *Interp) registerSIMDAndParallelBuiltins() {
	// simd_info() -> { backend => "x86_64-AVX2-256bit", avx2 => True, cpus => 16 }
	in.Builtins["simd_info"] = func(in *Interp, args []*Value) (*Value, error) {
		return HashValue(map[string]*Value{
			"backend": StringValue(SimdBackendName()),
			"avx2":    BoolValue(HasAVX2()),
			"cpus":    IntValue(int64(goruntime.NumCPU())),
		}), nil
	}

	// simd_sum(@arr)
	in.Builtins["simd_sum"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray {
			return IntValue(0), nil
		}
		if ints := args[0].intLane(); ints != nil {
			return IntValue(SimdSumInt64(ints)), nil
		}
		if floats := args[0].floatLane(); floats != nil {
			return FloatValue(SimdSumFloat64(floats)), nil
		}
		return in.Builtins["sum"](in, args)
	}

	// simd_dot(@a, @b) / dot(@a, @b)
	in.Builtins["simd_dot"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValArray || args[1].Type != ValArray {
			return FloatValue(0), nil
		}
		if f1, f2 := args[0].floatLane(), args[1].floatLane(); f1 != nil && f2 != nil {
			return FloatValue(SimdDotFloat64(f1, f2)), nil
		}
		if i1, i2 := args[0].intLane(), args[1].intLane(); i1 != nil && i2 != nil {
			n := len(i1)
			if len(i2) < n {
				n = len(i2)
			}
			var s0, s1, s2, s3 int64
			i := 0
			for ; i+3 < n; i += 4 {
				s0 += i1[i] * i2[i]
				s1 += i1[i+1] * i2[i+1]
				s2 += i1[i+2] * i2[i+2]
				s3 += i1[i+3] * i2[i+3]
			}
			acc := (s0 + s1) + (s2 + s3)
			for ; i < n; i++ {
				acc += i1[i] * i2[i]
			}
			return IntValue(acc), nil
		}
		n := args[0].arrayLen()
		if args[1].arrayLen() < n {
			n = args[1].arrayLen()
		}
		var sum float64
		for i := 0; i < n; i++ {
			sum += in.toFloat(args[0].arrayAt(i)) * in.toFloat(args[1].arrayAt(i))
		}
		return FloatValue(sum), nil
	}
	in.Builtins["dot"] = in.Builtins["simd_dot"]

	// simd_add(@a, @b)
	in.Builtins["simd_add"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValArray || args[1].Type != ValArray {
			return ArrayValue(nil), nil
		}
		if i1, i2 := args[0].intLane(), args[1].intLane(); i1 != nil && i2 != nil {
			return IntArrayValue(SimdAddInt64(i1, i2)), nil
		}
		if f1, f2 := args[0].floatLane(), args[1].floatLane(); f1 != nil && f2 != nil {
			n := len(f1)
			if len(f2) < n {
				n = len(f2)
			}
			out := make([]float64, n)
			for i := 0; i < n; i++ {
				out[i] = f1[i] + f2[i]
			}
			return FloatArrayValue(out), nil
		}
		n := args[0].arrayLen()
		if args[1].arrayLen() < n {
			n = args[1].arrayLen()
		}
		out := make([]*Value, n)
		for i := 0; i < n; i++ {
			out[i] = FloatValue(in.toFloat(args[0].arrayAt(i)) + in.toFloat(args[1].arrayAt(i)))
		}
		return ArrayValue(out), nil
	}

	// simd_scale(@a, $scalar)
	in.Builtins["simd_scale"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValArray {
			return ArrayValue(nil), nil
		}
		if ints := args[0].intLane(); ints != nil && args[1].Type == ValInt {
			k := args[1].IntVal
			out := make([]int64, len(ints))
			for i, v := range ints {
				out[i] = v * k
			}
			return IntArrayValue(out), nil
		}
		k := in.toFloat(args[1])
		if floats := args[0].floatLane(); floats != nil {
			out := make([]float64, len(floats))
			for i, v := range floats {
				out[i] = v * k
			}
			return FloatArrayValue(out), nil
		}
		n := args[0].arrayLen()
		out := make([]float64, n)
		for i := 0; i < n; i++ {
			out[i] = in.toFloat(args[0].arrayAt(i)) * k
		}
		return FloatArrayValue(out), nil
	}

	// psum(@arr) — multi-core chunked AVX2 SIMD sum across all CPU threads
	in.Builtins["psum"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray {
			return IntValue(0), nil
		}
		workers := goruntime.NumCPU()
		if ints := args[0].intLane(); ints != nil {
			n := len(ints)
			if n < 16384 || workers <= 1 {
				return IntValue(SimdSumInt64(ints)), nil
			}
			partials := make([]int64, workers)
			chunk := (n + workers - 1) / workers
			var wg sync.WaitGroup
			for w := 0; w < workers; w++ {
				lo := w * chunk
				if lo >= n {
					break
				}
				hi := lo + chunk
				if hi > n {
					hi = n
				}
				wg.Add(1)
				go func(slot int, sub []int64) {
					defer wg.Done()
					partials[slot] = SimdSumInt64(sub)
				}(w, ints[lo:hi])
			}
			wg.Wait()
			var total int64
			for _, p := range partials {
				total += p
			}
			return IntValue(total), nil
		}
		if floats := args[0].floatLane(); floats != nil {
			n := len(floats)
			if n < 16384 || workers <= 1 {
				return FloatValue(SimdSumFloat64(floats)), nil
			}
			partials := make([]float64, workers)
			chunk := (n + workers - 1) / workers
			var wg sync.WaitGroup
			for w := 0; w < workers; w++ {
				lo := w * chunk
				if lo >= n {
					break
				}
				hi := lo + chunk
				if hi > n {
					hi = n
				}
				wg.Add(1)
				go func(slot int, sub []float64) {
					defer wg.Done()
					partials[slot] = SimdSumFloat64(sub)
				}(w, floats[lo:hi])
			}
			wg.Wait()
			var total float64
			for _, p := range partials {
				total += p
			}
			return FloatValue(total), nil
		}
		return in.Builtins["sum"](in, args)
	}

	// pmap($callable, @list) or @list.pmap($callable) — lock-free multi-core parallel map
	in.Builtins["pmap"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return ArrayValue(nil), nil
		}
		var list, callable *Value
		if args[0].Type == ValArray {
			list, callable = args[0], args[1]
		} else {
			callable, list = args[0], args[1]
		}
		if list.Type != ValArray {
			return ArrayValue(nil), nil
		}
		n := list.arrayLen()
		if n == 0 {
			return ArrayValue(nil), nil
		}
		workers := goruntime.NumCPU()
		if n < 64 || workers <= 1 {
			return in.Builtins["map"](in, args)
		}
		if workers > n {
			workers = n
		}
		results := make([]*Value, n)
		errs := make([]error, workers)
		chunk := (n + workers - 1) / workers
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			lo := w * chunk
			if lo >= n {
				break
			}
			hi := lo + chunk
			if hi > n {
				hi = n
			}
			wk := in.forkWorker()
			wg.Add(1)
			go func(slot, startIdx, endIdx int, worker *Interp) {
				defer wg.Done()
				var oneArg [1]*Value
				for idx := startIdx; idx < endIdx; idx++ {
					tMark := worker.ds.markTemp()
					oneArg[0] = list.arrayAt(idx)
					val, err := worker.InvokeCallable(callable, oneArg[:])
					if err != nil {
						worker.ds.rewindTemp(tMark)
						errs[slot] = fmt.Errorf("pmap worker error: %w", err)
						return
					}
					results[idx] = escapeValue(val)
					worker.ds.rewindTemp(tMark)
				}
			}(w, lo, hi, wk)
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				return nil, err
			}
		}
		// Pack back into homogeneous intLane / floatLane if uniform
		allInts := true
		allFloats := true
		for _, r := range results {
			if r.Type != ValInt {
				allInts = false
			}
			if r.Type != ValFloat {
				allFloats = false
			}
			if !allInts && !allFloats {
				break
			}
		}
		if allInts {
			ints := make([]int64, n)
			for i, r := range results {
				ints[i] = r.IntVal
			}
			return IntArrayValue(ints), nil
		}
		if allFloats {
			floats := make([]float64, n)
			for i, r := range results {
				floats[i] = r.FloatVal
			}
			return FloatArrayValue(floats), nil
		}
		return ArrayValue(results), nil
	}

	// psort(@list) — multi-core parallel chunk sort + merge / radix sort
	in.Builtins["psort"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray {
			return ArrayValue(nil), nil
		}
		if ints := args[0].intLane(); ints != nil {
			cp := make([]int64, len(ints))
			copy(cp, ints)
			SortInt64Fast(cp)
			return IntArrayValue(cp), nil
		}
		if floats := args[0].floatLane(); floats != nil {
			n := len(floats)
			cp := make([]float64, n)
			copy(cp, floats)
			workers := goruntime.NumCPU()
			if n < 8192 || workers <= 1 {
				slices.Sort(cp)
				return FloatArrayValue(cp), nil
			}
			// Parallel chunk pdqsort + k-way pairwise merge
			if workers > 8 {
				workers = 8
			}
			bounds := make([]int, workers+1)
			for i := 0; i <= workers; i++ {
				bounds[i] = (i * n) / workers
			}
			var wg sync.WaitGroup
			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func(lo, hi int) {
					defer wg.Done()
					slices.Sort(cp[lo:hi])
				}(bounds[w], bounds[w+1])
			}
			wg.Wait()
			tmp := make([]float64, n)
			for step := 1; step < workers; step *= 2 {
				for i := 0; i+step < workers; i += 2 * step {
					lo := bounds[i]
					mid := bounds[i+step]
					hi := bounds[min(i+2*step, workers)]
					mergeFloat64(cp, tmp, lo, mid, hi)
				}
			}
			return FloatArrayValue(cp), nil
		}
		return in.Builtins["sort"](in, args)
	}
}

func mergeFloat64(arr, tmp []float64, lo, mid, hi int) {
	i, j, k := lo, mid, lo
	for i < mid && j < hi {
		if arr[i] <= arr[j] {
			tmp[k] = arr[i]
			i++
		} else {
			tmp[k] = arr[j]
			j++
		}
		k++
	}
	for i < mid {
		tmp[k] = arr[i]
		i++
		k++
	}
	for j < hi {
		tmp[k] = arr[j]
		j++
		k++
	}
	copy(arr[lo:hi], tmp[lo:hi])
}
