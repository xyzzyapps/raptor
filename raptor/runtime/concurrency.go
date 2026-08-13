package raptor

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)


func unsafePointer(u uintptr) unsafe.Pointer {
	return unsafe.Pointer(u)
}


// NewPromise creates a pending Promise.
func NewPromise() *Promise {
	return &Promise{
		Done:   make(chan struct{}),
		Status: "Planned",
	}
}

// Keep resolves the promise with a successful value.
func (p *Promise) Keep(val *Value) {
	p.Mu.Lock()
	defer p.Mu.Unlock()
	if p.Status != "Planned" {
		return
	}
	p.Result = val
	p.Status = "Kept"
	close(p.Done)
}

// Break marks the promise as failed.
func (p *Promise) Break(err error) {
	p.Mu.Lock()
	defer p.Mu.Unlock()
	if p.Status != "Planned" {
		return
	}
	p.Err = err
	p.Status = "Broken"
	close(p.Done)
}

// Await blocks until the promise resolves and returns the result.
func (p *Promise) Await() (*Value, error) {
	<-p.Done
	if p.Err != nil {
		return nil, p.Err
	}
	if p.Result == nil {
		return NilValue(), nil
	}
	return p.Result, nil
}

// NewChannel creates a new buffered concurrent Channel.
func NewChannel(capacity int) *Channel {
	if capacity <= 0 {
		capacity = 64
	}
	return &Channel{
		Ch: make(chan *Value, capacity),
	}
}

// Send enqueues a value into the channel.
func (c *Channel) Send(val *Value) error {
	c.Mu.Lock()
	if c.Closed {
		c.Mu.Unlock()
		return fmt.Errorf("cannot send to a closed channel")
	}
	c.Mu.Unlock()
	c.Ch <- val
	return nil
}

// Receive dequeues a value from the channel, blocking if empty.
func (c *Channel) Receive() (*Value, error) {
	val, ok := <-c.Ch
	if !ok {
		return NilValue(), fmt.Errorf("channel is closed and exhausted")
	}
	return val, nil
}

// Poll non-blockingly checks for an available value.
func (c *Channel) Poll() *Value {
	select {
	case val, ok := <-c.Ch:
		if !ok {
			return NilValue()
		}
		return val
	default:
		return NilValue()
	}
}

// Close closes the channel for sending.
func (c *Channel) Close() {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if !c.Closed {
		c.Closed = true
		close(c.Ch)
	}
}

func (in *Interp) registerConcurrencyAndAtomics() {
	// 1. Atomics primitives
	in.Builtins["atomic_add"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("atomic_add requires target and delta")
		}
		delta := in.toInt(args[1])
		if args[0].Type == ValCStruct && args[0].CStructVal != nil && args[0].CStructVal.Ptr != 0 {
			ptr := (*int64)(unsafePointer(args[0].CStructVal.Ptr))
			newVal := atomic.AddInt64(ptr, delta)
			return IntValue(newVal), nil
		}
		old := args[0].IntVal
		newVal := atomic.AddInt64(&args[0].IntVal, delta)
		_ = old
		return IntValue(newVal), nil
	}

	in.Builtins["atomic_sub"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("atomic_sub requires target and delta")
		}
		delta := in.toInt(args[1])
		if args[0].Type == ValCStruct && args[0].CStructVal != nil && args[0].CStructVal.Ptr != 0 {
			ptr := (*int64)(unsafePointer(args[0].CStructVal.Ptr))
			newVal := atomic.AddInt64(ptr, -delta)
			return IntValue(newVal), nil
		}
		newVal := atomic.AddInt64(&args[0].IntVal, -delta)
		return IntValue(newVal), nil
	}

	in.Builtins["atomic_load"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("atomic_load requires target")
		}
		if args[0].Type == ValCStruct && args[0].CStructVal != nil && args[0].CStructVal.Ptr != 0 {
			ptr := (*int64)(unsafePointer(args[0].CStructVal.Ptr))
			val := atomic.LoadInt64(ptr)
			return IntValue(val), nil
		}
		val := atomic.LoadInt64(&args[0].IntVal)
		return IntValue(val), nil
	}

	in.Builtins["atomic_store"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("atomic_store requires target and value")
		}
		val := in.toInt(args[1])
		if args[0].Type == ValCStruct && args[0].CStructVal != nil && args[0].CStructVal.Ptr != 0 {
			ptr := (*int64)(unsafePointer(args[0].CStructVal.Ptr))
			atomic.StoreInt64(ptr, val)
			return IntValue(val), nil
		}
		atomic.StoreInt64(&args[0].IntVal, val)
		return IntValue(val), nil
	}

	in.Builtins["atomic_cas"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("atomic_cas requires target, oldVal, and newVal")
		}
		oldVal := in.toInt(args[1])
		newVal := in.toInt(args[2])
		var swapped bool
		if args[0].Type == ValCStruct && args[0].CStructVal != nil && args[0].CStructVal.Ptr != 0 {
			ptr := (*int64)(unsafePointer(args[0].CStructVal.Ptr))
			swapped = atomic.CompareAndSwapInt64(ptr, oldVal, newVal)
		} else {
			swapped = atomic.CompareAndSwapInt64(&args[0].IntVal, oldVal, newVal)
		}
		return BoolValue(swapped), nil
	}

	// 2. Concurrency: Promise, start, await, Channel
	in.Builtins["start"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("start requires a closure")
		}
		callee := args[0]
		p := NewPromise()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					p.Break(fmt.Errorf("panic in async task: %v", r))
				}
			}()
			res, err := in.InvokeCallable(callee, nil)
			if err != nil {
				p.Break(err)
			} else {
				p.Keep(res)
			}
		}()
		return PromiseValue(p), nil
	}

	in.Builtins["await"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return NilValue(), nil
		}
		if len(args) == 1 {
			if args[0].Type == ValPromise && args[0].PromiseVal != nil {
				return args[0].PromiseVal.Await()
			}
			if args[0].Type == ValArray {
				var results []*Value
				for _, item := range args[0].ArrayVal {
					if item.Type == ValPromise && item.PromiseVal != nil {
						res, err := item.PromiseVal.Await()
						if err != nil {
							return nil, err
						}
						results = append(results, res)
					} else {
						results = append(results, item)
					}
				}
				return ArrayValue(results), nil
			}
			return args[0], nil
		}
		var results []*Value
		for _, a := range args {
			if a.Type == ValPromise && a.PromiseVal != nil {
				res, err := a.PromiseVal.Await()
				if err != nil {
					return nil, err
				}
				results = append(results, res)
			} else {
				results = append(results, a)
			}
		}
		return ArrayValue(results), nil
	}

	in.Builtins["sleep"] = func(in *Interp, args []*Value) (*Value, error) {
		sec := 1.0
		if len(args) > 0 {
			sec = in.toFloat(args[0])
		}
		time.Sleep(time.Duration(sec * float64(time.Second)))
		return NilValue(), nil
	}

	// 3. Channel constructor & methods
	in.Builtins["channel_new"] = func(in *Interp, args []*Value) (*Value, error) {
		cap := 64
		if len(args) > 0 {
			cap = int(in.toInt(args[0]))
		}
		return ChannelValue(NewChannel(cap)), nil
	}

	// 4. Mutex Primitives
	mutexes := make(map[string]*sync.Mutex)
	var mutexMu sync.Mutex
	mID := 1

	in.Builtins["mutex_create"] = func(in *Interp, args []*Value) (*Value, error) {
		mutexMu.Lock()
		key := fmt.Sprintf("mtx_%d", mID)
		mID++
		mutexes[key] = &sync.Mutex{}
		mutexMu.Unlock()
		return StringValue(key), nil
	}

	in.Builtins["mutex_lock"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("mutex_lock requires mutex handle")
		}
		key := args[0].String()
		mutexMu.Lock()
		m, ok := mutexes[key]
		mutexMu.Unlock()
		if !ok || m == nil {
			return nil, fmt.Errorf("invalid mutex handle %q", key)
		}
		m.Lock()
		return BoolValue(true), nil
	}

	in.Builtins["mutex_unlock"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("mutex_unlock requires mutex handle")
		}
		key := args[0].String()
		mutexMu.Lock()
		m, ok := mutexes[key]
		mutexMu.Unlock()
		if !ok || m == nil {
			return nil, fmt.Errorf("invalid mutex handle %q", key)
		}
		m.Unlock()
		return BoolValue(true), nil
	}

	// 5. Semaphore Primitives
	semaphores := make(map[string]chan struct{})
	var semMu sync.Mutex
	semID := 1

	in.Builtins["semaphore_create"] = func(in *Interp, args []*Value) (*Value, error) {
		cap := 1
		if len(args) > 0 {
			cap = int(in.toInt(args[0]))
		}
		if cap <= 0 {
			cap = 1
		}
		semMu.Lock()
		key := fmt.Sprintf("sem_%d", semID)
		semID++
		semaphores[key] = make(chan struct{}, cap)
		semMu.Unlock()
		return StringValue(key), nil
	}

	in.Builtins["semaphore_acquire"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("semaphore_acquire requires semaphore handle")
		}
		key := args[0].String()
		semMu.Lock()
		ch, ok := semaphores[key]
		semMu.Unlock()
		if !ok || ch == nil {
			return nil, fmt.Errorf("invalid semaphore handle %q", key)
		}
		ch <- struct{}{}
		return BoolValue(true), nil
	}

	in.Builtins["semaphore_release"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("semaphore_release requires semaphore handle")
		}
		key := args[0].String()
		semMu.Lock()
		ch, ok := semaphores[key]
		semMu.Unlock()
		if !ok || ch == nil {
			return nil, fmt.Errorf("invalid semaphore handle %q", key)
		}
		select {
		case <-ch:
			return BoolValue(true), nil
		default:
			return BoolValue(false), nil
		}
	}

	// 6. WaitGroup Primitives
	waitgroups := make(map[string]*sync.WaitGroup)
	var wgMu sync.Mutex
	wgID := 1

	in.Builtins["waitgroup_create"] = func(in *Interp, args []*Value) (*Value, error) {
		wgMu.Lock()
		key := fmt.Sprintf("wg_%d", wgID)
		wgID++
		waitgroups[key] = &sync.WaitGroup{}
		wgMu.Unlock()
		return StringValue(key), nil
	}

	in.Builtins["waitgroup_add"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("waitgroup_add requires waitgroup handle and delta count")
		}
		key := args[0].String()
		delta := int(in.toInt(args[1]))
		wgMu.Lock()
		wg, ok := waitgroups[key]
		wgMu.Unlock()
		if !ok || wg == nil {
			return nil, fmt.Errorf("invalid waitgroup handle %q", key)
		}
		wg.Add(delta)
		return BoolValue(true), nil
	}

	in.Builtins["waitgroup_done"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("waitgroup_done requires waitgroup handle")
		}
		key := args[0].String()
		wgMu.Lock()
		wg, ok := waitgroups[key]
		wgMu.Unlock()
		if !ok || wg == nil {
			return nil, fmt.Errorf("invalid waitgroup handle %q", key)
		}
		wg.Done()
		return BoolValue(true), nil
	}

	in.Builtins["waitgroup_wait"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("waitgroup_wait requires waitgroup handle")
		}
		key := args[0].String()
		wgMu.Lock()
		wg, ok := waitgroups[key]
		wgMu.Unlock()
		if !ok || wg == nil {
			return nil, fmt.Errorf("invalid waitgroup handle %q", key)
		}
		wg.Wait()
		return BoolValue(true), nil
	}

	// 7. Parallel Map / Worker Pool
	in.Builtins["parallel_map"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("parallel_map requires items array and worker sub")
		}
		items := args[0]
		worker := args[1]

		var list []*Value
		if items.Type == ValArray {
			list = items.ArrayVal
		} else if items.Type == ValLazySeq && items.LazySeqVal != nil {
			list = items.LazySeqVal.Items
		} else {
			list = []*Value{items}
		}

		limit := 8
		if len(args) >= 3 && args[2].Type != ValNil {
			l := int(in.toInt(args[2]))
			if l > 0 {
				limit = l
			}
		}

		results := make([]*Value, len(list))
		sem := make(chan struct{}, limit)
		var wg sync.WaitGroup
		var firstErr error
		var errMu sync.Mutex

		for i, item := range list {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, it *Value) {
				defer wg.Done()
				defer func() { <-sem }()

				res, err := in.InvokeCallable(worker, []*Value{it, IntValue(int64(idx))})
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
				} else {
					results[idx] = res
				}
			}(i, item)
		}

		wg.Wait()
		if firstErr != nil {
			return nil, firstErr
		}
		return ArrayValue(results), nil
	}

	// 8. Reactive Event Streams (Supply / Supplier)
	type supplyStream struct {
		handlers []*Value
		mu       sync.Mutex
		done     bool
	}
	supplies := make(map[string]*supplyStream)
	var supMu sync.Mutex
	supID := 1

	in.Builtins["supply_create"] = func(in *Interp, args []*Value) (*Value, error) {
		supMu.Lock()
		key := fmt.Sprintf("sup_%d", supID)
		supID++
		supplies[key] = &supplyStream{}
		supMu.Unlock()
		return StringValue(key), nil
	}

	in.Builtins["supply_tap"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("supply_tap requires supply handle and handler callable")
		}
		key := args[0].String()
		handler := args[1]
		supMu.Lock()
		s, ok := supplies[key]
		supMu.Unlock()
		if !ok || s == nil {
			return nil, fmt.Errorf("invalid supply handle %q", key)
		}

		s.mu.Lock()
		s.handlers = append(s.handlers, handler)
		s.mu.Unlock()
		return BoolValue(true), nil
	}

	in.Builtins["supply_emit"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("supply_emit requires supply handle and value")
		}
		key := args[0].String()
		val := args[1]
		supMu.Lock()
		s, ok := supplies[key]
		supMu.Unlock()
		if !ok || s == nil {
			return nil, fmt.Errorf("invalid supply handle %q", key)
		}

		s.mu.Lock()
		handlers := append([]*Value(nil), s.handlers...)
		s.mu.Unlock()

		for _, h := range handlers {
			_, _ = in.InvokeCallable(h, []*Value{val})
		}
		return BoolValue(true), nil
	}

	in.Builtins["supply_done"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("supply_done requires supply handle")
		}
		key := args[0].String()
		supMu.Lock()
		delete(supplies, key)
		supMu.Unlock()
		return BoolValue(true), nil
	}
}
