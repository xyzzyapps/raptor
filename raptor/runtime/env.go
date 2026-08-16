package raptor

import "fmt"

// StateCell holds a persistent, mutable reference cell for state variables.
type StateCell struct {
	Val *Value
}

// Env represents a lexical scope environment frame.
type Env struct {
	parent        *Env
	bindings      map[string]*Value
	types         map[string]string
	wheres        map[string]Expr
	stateBindings map[string]*StateCell
}

// NewEnv creates a new top-level environment.
func NewEnv() *Env {
	return &Env{
		bindings: make(map[string]*Value),
	}
}

// NewChild creates a new child environment frame.
// types / wheres / stateBindings are allocated only if used.
func (e *Env) NewChild() *Env {
	return &Env{
		parent:   e,
		bindings: make(map[string]*Value, 4),
	}
}

// Reset clears this frame's bindings so it can be reused for the next loop iteration.
func (e *Env) Reset() {
	if e == nil {
		return
	}
	clear(e.bindings)
	if e.types != nil {
		clear(e.types)
	}
	if e.wheres != nil {
		clear(e.wheres)
	}
	if e.stateBindings != nil {
		clear(e.stateBindings)
	}
}

// RecycleChild reuses child (or allocates one) as a fresh frame under e.
func (e *Env) RecycleChild(child *Env) *Env {
	if child == nil {
		return e.NewChild()
	}
	child.Reset()
	child.parent = e
	return child
}

// DefineState binds a state variable cell to the environment frame.
func (e *Env) DefineState(name string, cell *StateCell) {
	e.bindings[name] = cell.Val
	if e.stateBindings == nil {
		e.stateBindings = make(map[string]*StateCell)
	}
	e.stateBindings[name] = cell
}

// Define declares a new variable in the current frame.
func detachInt(v *Value) *Value {
	if isInternedInt(v) {
		return &Value{Type: ValInt, IntVal: v.IntVal}
	}
	return v
}

func (e *Env) Define(name string, val *Value) {
	val = detachInt(val)
	e.bindings[name] = val
	if e.stateBindings != nil {
		if cell, ok := e.stateBindings[name]; ok {
			cell.Val = val
		}
	}
}

// DefineTyped declares a variable with an invariant type and optional predicate constraint.
func (e *Env) DefineTyped(name string, val *Value, typeName string, where Expr) {
	val = detachInt(val)
	e.bindings[name] = val
	if typeName != "" {
		if e.types == nil {
			e.types = make(map[string]string)
		}
		e.types[name] = typeName
	}
	if where != nil {
		if e.wheres == nil {
			e.wheres = make(map[string]Expr)
		}
		e.wheres[name] = where
	}
}

// LookupType finds a variable's type and where constraint looking up the lexical chain.
func (e *Env) LookupType(name string) (string, Expr, bool) {
	if _, ok := e.bindings[name]; ok {
		var t string
		var w Expr
		if e.types != nil {
			t = e.types[name]
		}
		if e.wheres != nil {
			w = e.wheres[name]
		}
		return t, w, true
	}
	if e.parent != nil {
		return e.parent.LookupType(name)
	}
	return "", nil, false
}

// RegisterMulti adds a multi sub candidate to the current frame.
func (e *Env) RegisterMulti(name string, cand *Closure) {
	cand.IsMulti = true
	cand.Name = name

	existing, ok := e.bindings[name]
	if !ok || existing == nil {
		e.bindings[name] = MultiSubValue([]*Closure{cand})
		e.bindings["&"+name] = e.bindings[name]
		return
	}

	if existing.Type == ValMultiSub {
		for _, c := range existing.Candidates {
			if c == cand {
				return
			}
		}
		existing.Candidates = append(existing.Candidates, cand)
		return
	}

	if existing.Type == ValClosure {
		// Upgrade existing single closure to multi sub
		multi := MultiSubValue([]*Closure{existing.ClosureVal, cand})
		e.bindings[name] = multi
		e.bindings["&"+name] = multi
		return
	}

	e.bindings[name] = MultiSubValue([]*Closure{cand})
	e.bindings["&"+name] = e.bindings[name]
}

// Assign updates an existing variable looking up the lexical chain.
func (e *Env) Assign(name string, val *Value) error {
	if _, ok := e.bindings[name]; ok {
		val = detachInt(val)
		e.bindings[name] = val
		if e.stateBindings != nil {
			if cell, ok := e.stateBindings[name]; ok {
				cell.Val = val
			}
		}
		return nil
	}
	if e.parent != nil {
		return e.parent.Assign(name, val)
	}
	return fmt.Errorf("cannot assign to undeclared variable %q", name)
}

// Lookup finds a variable value looking up the lexical chain.
func (e *Env) Lookup(name string) (*Value, bool) {
	if e.stateBindings != nil {
		if cell, ok := e.stateBindings[name]; ok {
			return cell.Val, true
		}
	}
	if v, ok := e.bindings[name]; ok {
		return v, true
	}
	if e.parent != nil {
		return e.parent.Lookup(name)
	}
	return nil, false
}

// Delete removes a variable binding from the environment.
func (e *Env) Delete(name string) {
	delete(e.bindings, name)
	if e.types != nil {
		delete(e.types, name)
	}
	if e.wheres != nil {
		delete(e.wheres, name)
	}
	if e.stateBindings != nil {
		delete(e.stateBindings, name)
	}
}

// Bindings returns a copy of the current frame bindings map.
func (e *Env) Bindings() map[string]*Value {
	res := make(map[string]*Value, len(e.bindings))
	for k, v := range e.bindings {
		res[k] = v
	}
	return res
}
