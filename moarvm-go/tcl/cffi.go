//go:build windows

package tcl

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// CFFIManager manages dynamically loaded C libraries and function invocations.
type CFFIManager struct {
	mu      sync.RWMutex
	dlls    map[string]*syscall.DLL
	counter int
	logger  *slog.Logger
}

// NewCFFIManager creates a new CFFIManager instance.
func NewCFFIManager(logger *slog.Logger) *CFFIManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &CFFIManager{
		dlls:   make(map[string]*syscall.DLL),
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
	if strings.Contains(dllPath, "/") || strings.Contains(dllPath, "\\") {
		absDllPath, err := filepath.Abs(dllPath)
		if err == nil {
			dllDir := filepath.Dir(absDllPath)
			kernel32 := syscall.NewLazyDLL("kernel32.dll")
			setDllDir := kernel32.NewProc("SetDllDirectoryW")
			if ptr, err := syscall.UTF16PtrFromString(dllDir); err == nil {
				_, _, _ = setDllDir.Call(uintptr(unsafe.Pointer(ptr)))
			}
			dllPath = absDllPath
		}
	}

	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		return "", fmt.Errorf("ffi::load failed to load library %q: %w", dllPath, err)
	}

	m.mu.Lock()
	m.counter++
	handle := fmt.Sprintf("dll_%d", m.counter)
	m.dlls[handle] = dll
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
	dll, ok := m.dlls[handle]
	if ok {
		delete(m.dlls, handle)
		_ = dll.Release()
	}
	m.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("ffi::close: invalid or closed handle %q", handle)
	}
	return "OK", nil
}

func (m *CFFIManager) cmdCall(in *Interp, args []string) (string, error) {
	if len(args) < 4 {
		return "", fmt.Errorf("wrong # args: should be \"ffi::call dllHandle funcName retType argTypes ?arg ...?\"")
	}

	handle := args[0]
	funcName := args[1]
	retType := strings.ToLower(args[2])
	argTypesList := strings.Fields(args[3])
	callArgs := args[4:]

	if len(argTypesList) != len(callArgs) {
		return "", fmt.Errorf("ffi::call argument count mismatch: expected %d types (%s), got %d values",
			len(argTypesList), strings.Join(argTypesList, " "), len(callArgs))
	}

	m.mu.RLock()
	dll, ok := m.dlls[handle]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("ffi::call: unknown library handle %q", handle)
	}

	proc, err := dll.FindProc(funcName)
	if err != nil {
		return "", fmt.Errorf("ffi::call: symbol %q not found in %s: %w", funcName, handle, err)
	}

	cArgs := make([]uintptr, len(callArgs))
	for i, argVal := range callArgs {
		typ := strings.ToLower(argTypesList[i])
		switch typ {
		case "int", "i32", "i64", "long":
			val, err := strconv.ParseInt(argVal, 0, 64)
			if err != nil {
				return "", fmt.Errorf("ffi::call: argument %d expected integer (%s), got %q: %w", i+1, typ, argVal, err)
			}
			cArgs[i] = uintptr(val)
		case "uint", "u32", "u64", "ulong", "size_t":
			val, err := strconv.ParseUint(argVal, 0, 64)
			if err != nil {
				return "", fmt.Errorf("ffi::call: argument %d expected unsigned integer (%s), got %q: %w", i+1, typ, argVal, err)
			}
			cArgs[i] = uintptr(val)
		case "ptr", "pointer":
			val, err := strconv.ParseUint(argVal, 0, 64)
			if err != nil {
				return "", fmt.Errorf("ffi::call: argument %d expected pointer address, got %q: %w", i+1, argVal, err)
			}
			cArgs[i] = uintptr(val)
		case "str", "string", "char*":
			cStr, err := syscall.BytePtrFromString(argVal)
			if err != nil {
				return "", fmt.Errorf("ffi::call: argument %d string error: %w", i+1, err)
			}
			cArgs[i] = uintptr(unsafe.Pointer(cStr))
		default:
			return "", fmt.Errorf("ffi::call: unsupported argument type %q (use int, uint, ptr, or str)", typ)
		}
	}

	// Invoke C function dynamically
	retVal, _, _ := syscall.SyscallN(proc.Addr(), cArgs...)

	switch retType {
	case "void":
		return "", nil
	case "int", "i32":
		return strconv.FormatInt(int64(int32(retVal)), 10), nil
	case "i64", "long":
		return strconv.FormatInt(int64(retVal), 10), nil
	case "uint", "u32":
		return strconv.FormatUint(uint64(uint32(retVal)), 10), nil
	case "u64", "ulong", "size_t":
		return strconv.FormatUint(uint64(retVal), 10), nil
	case "ptr", "pointer":
		return fmt.Sprintf("0x%x", retVal), nil
	case "str", "string", "char*":
		if retVal == 0 {
			return "", nil
		}
		str := stringFromCString(retVal)
		return str, nil
	default:
		return fmt.Sprintf("%d", retVal), nil
	}
}

func (m *CFFIManager) cmdBind(in *Interp, args []string) (string, error) {
	if len(args) != 5 {
		return "", fmt.Errorf("wrong # args: should be \"ffi::bind dllHandle funcName retType argTypes tclCmdName\"")
	}

	handle := args[0]
	funcName := args[1]
	retType := args[2]
	argTypes := args[3]
	cmdName := args[4]

	in.RegisterCommand(cmdName, func(interp *Interp, callArgs []string) (string, error) {
		fullArgs := append([]string{handle, funcName, retType, argTypes}, callArgs...)
		return m.cmdCall(interp, fullArgs)
	})

	m.logger.Debug("bound native C function to tcl command",
		slog.String("c_func", funcName),
		slog.String("tcl_cmd", cmdName),
	)
	return "OK", nil
}

func stringFromCString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var bytes []byte
	p := (*byte)(unsafe.Pointer(ptr))
	for *p != 0 {
		bytes = append(bytes, *p)
		ptr++
		p = (*byte)(unsafe.Pointer(ptr))
	}
	return string(bytes)
}
