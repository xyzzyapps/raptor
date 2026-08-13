package raptor

import "fmt"

// Env represents a lexical scope environment frame.
type Env struct {
	parent   *Env
	bindings map[string]*Value
}

// NewEnv creates a new top-level environment.
func NewEnv() *Env {
	return &Env{
		bindings: make(map[string]*Value),
	}
}

// NewChild creates a new child environment frame.
func (e *Env) NewChild() *Env {
	return &Env{
		parent:   e,
		bindings: make(map[string]*Value),
	}
}

// Define declares a new variable in the current frame.
func (e *Env) Define(name string, val *Value) {
	e.bindings[name] = val
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
		e.bindings[name] = val
		return nil
	}
	if e.parent != nil {
		return e.parent.Assign(name, val)
	}
	return fmt.Errorf("cannot assign to undeclared variable %q", name)
}

// Lookup finds a variable value looking up the lexical chain.
func (e *Env) Lookup(name string) (*Value, bool) {
	if v, ok := e.bindings[name]; ok {
		return v, true
	}
	if e.parent != nil {
		return e.parent.Lookup(name)
	}
	return nil, false
}
