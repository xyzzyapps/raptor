// Package tcl implements a lightweight, embeddable Tcl interpreter in pure Go.
package tcl

import (
	"errors"
)

// CommandFunc defines the signature of a built-in or custom Tcl command.
type CommandFunc func(interp *Interp, args []string) (string, error)

// Proc represents a user-defined Tcl procedure created via 'proc'.
type Proc struct {
	Name string
	Args []string
	Body string
}

// Scope represents a variable scope frame in the call stack.
type Scope struct {
	vars    map[string]string
	globals map[string]bool
}

// NewScope creates a new variable scope frame.
func NewScope() *Scope {
	return &Scope{
		vars:    make(map[string]string),
		globals: make(map[string]bool),
	}
}

var (
	// ErrVarNotFound is returned when referencing an undefined variable.
	ErrVarNotFound = errors.New("can't read variable: no such variable")
	// ErrCommandNotFound is returned when calling an unregistered command.
	ErrCommandNotFound = errors.New("invalid command name")
	// ErrBreak is returned internally to break out of loops.
	ErrBreak = errors.New("invoked 'break' outside of a loop")
	// ErrContinue is returned internally to continue loop iterations.
	ErrContinue = errors.New("invoked 'continue' outside of a loop")
)

// ReturnSignal is used internally to unwind stack frames on 'return'.
type ReturnSignal struct {
	Value string
}

func (r *ReturnSignal) Error() string {
	return "return signal"
}
