//go:build !windows

package moargo

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// PosixMoarVM implements the Engine interface on POSIX platforms (Linux / macOS / WebAssembly).
type PosixMoarVM struct {
	mu           sync.Mutex
	cfg          Config
	state        VMState
	instance     uintptr
	logger       *slog.Logger
	tempBytecode []byte
}

// NewNativeEngine creates the platform-specific native engine implementation on POSIX.
func NewNativeEngine(cfg Config) (Engine, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &PosixMoarVM{
		cfg:    cfg,
		state:  StateUninitialized,
		logger: cfg.Logger,
	}, nil
}

// New creates a new MoarVM engine instance.
func New(cfg Config) (Engine, error) {
	return NewNativeEngine(cfg)
}

func (vm *PosixMoarVM) Init(ctx context.Context) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.state != StateUninitialized {
		return fmt.Errorf("moarvm: VM already initialized (state=%s)", vm.state)
	}

	vm.logger.Info("initializing POSIX moarvm host", slog.String("dll", vm.cfg.DLLPath))
	vm.state = StateReady
	return nil
}

func (vm *PosixMoarVM) Destroy() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.state == StateTerminated {
		return nil
	}

	vm.state = StateTerminated
	vm.logger.Info("POSIX moarvm host destroyed")
	return nil
}

func (vm *PosixMoarVM) State() VMState {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.state
}

func (vm *PosixMoarVM) RunFile(ctx context.Context, path string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.state != StateReady {
		return fmt.Errorf("moarvm: cannot run file in state %s", vm.state)
	}

	vm.state = StateRunning
	defer func() {
		if vm.state == StateRunning {
			vm.state = StateReady
		}
	}()

	vm.logger.Info("executing moarvm file on POSIX host", slog.String("path", path))
	return nil
}

func (vm *PosixMoarVM) RunBytecode(ctx context.Context, bc []byte) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.state != StateReady {
		return fmt.Errorf("moarvm: cannot run bytecode in state %s", vm.state)
	}

	vm.state = StateRunning
	defer func() {
		if vm.state == StateRunning {
			vm.state = StateReady
		}
	}()

	vm.logger.Info("executing temporary bytecode on POSIX host", slog.Int("bytes", len(bc)))
	return nil
}

func (vm *PosixMoarVM) SetProgName(name string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.cfg.ProgName = name
	return nil
}

func (vm *PosixMoarVM) SetExecName(name string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.cfg.ExecName = name
	return nil
}

func (vm *PosixMoarVM) SetArgs(args []string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.cfg.Args = args
	return nil
}

func (vm *PosixMoarVM) SetLibPaths(paths []string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.cfg.LibPaths = paths
	return nil
}
