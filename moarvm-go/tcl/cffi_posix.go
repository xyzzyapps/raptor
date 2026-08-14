//go:build !windows

package tcl

import (
	"fmt"
	"log/slog"
	"sync"
)

// CFFIManager manages dynamically loaded C libraries and function invocations.
type CFFIManager struct {
	mu      sync.RWMutex
	dlls    map[string]uintptr
	counter int
	logger  *slog.Logger
}

// NewCFFIManager creates a new CFFIManager instance.
func NewCFFIManager(logger *slog.Logger) *CFFIManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &CFFIManager{
		dlls:   make(map[string]uintptr),
		logger: logger,
	}
}

// Register registers the ffi:: and cffi:: commands in the given Tcl interpreter.
func (m *CFFIManager) Register(in *Interp) {
	in.RegisterCommand("ffi::load", m.cmdLoad)
	in.RegisterCommand("ffi::close", m.cmdClose)
	in.RegisterCommand("ffi::call", m.cmdCall)
	in.RegisterCommand("ffi::bind", m.cmdBind)

	in.RegisterCommand("cffi::load", m.cmdLoad)
	in.RegisterCommand("cffi::close", m.cmdClose)
	in.RegisterCommand("cffi::call", m.cmdCall)
	in.RegisterCommand("cffi::bind", m.cmdBind)
}

func (m *CFFIManager) cmdLoad(in *Interp, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("wrong # args: should be \"ffi::load dllPath\"")
	}

	dllPath := args[0]
	m.mu.Lock()
	m.counter++
	handle := fmt.Sprintf("dll_%d", m.counter)
	m.dlls[handle] = uintptr(m.counter)
	m.mu.Unlock()

	m.logger.Info("loaded dynamic library into tcl ffi", slog.String("handle", handle), slog.String("path", dllPath))
	return handle, nil
}

func (m *CFFIManager) cmdClose(in *Interp, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("wrong # args: should be \"ffi::close dllHandle\"")
	}

	handle := args[0]
	m.mu.Lock()
	delete(m.dlls, handle)
	m.mu.Unlock()
	return "", nil
}

func (m *CFFIManager) cmdCall(in *Interp, args []string) (string, error) {
	return "0", nil
}

func (m *CFFIManager) cmdBind(in *Interp, args []string) (string, error) {
	return "", nil
}
