package moargo

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// MockEngine provides an in-memory simulation of the MoarVM engine for testing.
type MockEngine struct {
	mu           sync.Mutex
	cfg          Config
	state        VMState
	progName     string
	execName     string
	args         []string
	libPaths     []string
	executedFiles []string
	logger       *slog.Logger
}

// NewMock creates a new mock engine instance.
func NewMock(cfg Config) *MockEngine {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &MockEngine{
		cfg:    cfg,
		state:  StateUninitialized,
		logger: cfg.Logger,
	}
}

// Init transitions the mock VM instance to StateReady.
func (m *MockEngine) Init(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateReady || m.state == StateRunning {
		return ErrAlreadyInitialized
	}
	m.state = StateReady
	m.logger.Info("mock moarvm initialized")
	return nil
}

// Destroy transitions the mock VM instance to StateTerminated.
func (m *MockEngine) Destroy() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = StateTerminated
	m.logger.Info("mock moarvm destroyed")
	return nil
}

// State returns the current lifecycle state of the mock VM.
func (m *MockEngine) State() VMState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// RunFile records filePath as executed on the mock VM.
func (m *MockEngine) RunFile(ctx context.Context, filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != StateReady {
		return ErrNotInitialized
	}
	m.executedFiles = append(m.executedFiles, filePath)
	m.logger.Info("mock moarvm executed file", slog.String("file", filePath))
	return nil
}

// RunBytecode validates VM readiness and logs bytecode buffer execution on the mock VM.
func (m *MockEngine) RunBytecode(ctx context.Context, bytecode []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != StateReady {
		return ErrNotInitialized
	}
	m.logger.Info("mock moarvm executed bytecode", slog.Int("len", len(bytecode)))
	return nil
}

// SetProgName records the program name on the mock VM.
func (m *MockEngine) SetProgName(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progName = name
	return nil
}

// SetExecName records the executable path on the mock VM.
func (m *MockEngine) SetExecName(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execName = name
	return nil
}

// SetArgs records command-line arguments on the mock VM.
func (m *MockEngine) SetArgs(args []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.args = append([]string(nil), args...)
	return nil
}

// SetLibPaths records module library search paths on the mock VM.
func (m *MockEngine) SetLibPaths(paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.libPaths = append([]string(nil), paths...)
	return nil
}

// GetExecutedFiles returns a copy of all bytecode file paths passed to RunFile.
func (m *MockEngine) GetExecutedFiles() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]string, len(m.executedFiles))
	copy(res, m.executedFiles)
	return res
}

// String formats a diagnostic summary of the mock VM state.
func (m *MockEngine) String() string {
	return fmt.Sprintf("MockEngine{state: %s, prog: %s}", m.State(), m.progName)
}
