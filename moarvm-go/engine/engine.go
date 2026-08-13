// Package moargo provides native and mock bindings for the MoarVM runtime.
package moargo

import (
	"context"
	"errors"
	"log/slog"
)

// VMState represents the lifecycle state of a MoarVM instance.
type VMState int

const (
	StateUninitialized VMState = iota
	StateReady
	StateRunning
	StateTerminated
	StateError
)

func (s VMState) String() string {
	switch s {
	case StateUninitialized:
		return "UNINITIALIZED"
	case StateReady:
		return "READY"
	case StateRunning:
		return "RUNNING"
	case StateTerminated:
		return "TERMINATED"
	case StateError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

var (
	ErrNotInitialized    = errors.New("moarvm: VM instance not initialized")
	ErrAlreadyInitialized = errors.New("moarvm: VM instance already initialized")
	ErrDLLNotFound       = errors.New("moarvm: moar.dll could not be loaded")
	ErrSymbolNotFound    = errors.New("moarvm: required symbol not found in dynamic library")
	ErrExecutionFailed   = errors.New("moarvm: bytecode execution failed")
)

// Config defines the configuration for a MoarVM runtime instance.
type Config struct {
	DLLPath      string
	ProgName     string
	ExecName     string
	Args         []string
	LibPaths     []string
	FullCleanup  bool
	DebugPort    int
	Logger       *slog.Logger
}

// Engine represents an abstract MoarVM runtime execution engine.
type Engine interface {
	Init(ctx context.Context) error
	Destroy() error
	State() VMState
	RunFile(ctx context.Context, filePath string) error
	RunBytecode(ctx context.Context, bytecode []byte) error
	SetProgName(name string) error
	SetExecName(name string) error
	SetArgs(args []string) error
	SetLibPaths(paths []string) error
}
