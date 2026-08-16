//go:build windows

package moargo


import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

// WindowsMoarVM implements the Engine interface using dynamic Windows DLL FFI bindings.
type WindowsMoarVM struct {
	mu           sync.Mutex
	cfg          Config
	state        VMState
	dll          *syscall.DLL
	instance     uintptr
	logger       *slog.Logger
	procCreate   *syscall.Proc
	procDestroy  *syscall.Proc
	procRunFile  *syscall.Proc
	procRunBC    *syscall.Proc
	procSetArgs  *syscall.Proc
	procSetProg  *syscall.Proc
	procSetLib   *syscall.Proc
}

// New creates a new Windows MoarVM engine instance.
func New(cfg Config) (Engine, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	vm := &WindowsMoarVM{
		cfg:    cfg,
		state:  StateUninitialized,
		logger: cfg.Logger,
	}
	return vm, nil
}

// Init loads the dynamic DLL and creates a VM instance.
func (v *WindowsMoarVM) Init(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.state == StateReady || v.state == StateRunning {
		return ErrAlreadyInitialized
	}

	dllPath := v.cfg.DLLPath
	if dllPath == "" {
		dllPath = filepath.Join("build", "moarvm", "bin", "moar.dll")
	}

	// Add DLL directory to DLL search path so dependent runtime DLLs are found
	absDllPath, err := filepath.Abs(dllPath)
	if err == nil {
		dllDir := filepath.Dir(absDllPath)
		v.logger.Debug("resolving dll directory for dependency resolution", slog.String("dir", dllDir))
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		setDllDir := kernel32.NewProc("SetDllDirectoryW")
		if ptr, err := syscall.UTF16PtrFromString(dllDir); err == nil {
			_, _, _ = setDllDir.Call(uintptr(unsafe.Pointer(ptr)))
		}
		dllPath = absDllPath
	}

	v.logger.Info("loading moarvm dynamic library", slog.String("dll", dllPath))
	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		v.logger.Error("failed to load moar.dll", slog.String("path", dllPath), slog.Any("error", err))
		return fmt.Errorf("%w: %v", ErrDLLNotFound, err)
	}
	v.dll = dll

	procCreate, err := dll.FindProc("MVM_vm_create_instance")
	if err != nil {
		_ = dll.Release()
		return fmt.Errorf("%w: MVM_vm_create_instance: %v", ErrSymbolNotFound, err)
	}
	procDestroy, err := dll.FindProc("MVM_vm_destroy_instance")
	if err != nil {
		_ = dll.Release()
		return fmt.Errorf("%w: MVM_vm_destroy_instance: %v", ErrSymbolNotFound, err)
	}
	procRunFile, err := dll.FindProc("MVM_vm_run_file")
	if err != nil {
		_ = dll.Release()
		return fmt.Errorf("%w: MVM_vm_run_file: %v", ErrSymbolNotFound, err)
	}

	v.procCreate = procCreate
	v.procDestroy = procDestroy
	v.procRunFile = procRunFile
	v.procRunBC, _ = dll.FindProc("MVM_vm_run_bytecode")
	v.procSetArgs, _ = dll.FindProc("MVM_vm_set_clargs")
	v.procSetProg, _ = dll.FindProc("MVM_vm_set_prog_name")
	v.procSetLib, _ = dll.FindProc("MVM_vm_set_lib_path")

	// Call MVM_vm_create_instance
	ret, _, _ := v.procCreate.Call()
	if ret == 0 {
		v.state = StateError
		return fmt.Errorf("moarvm: MVM_vm_create_instance returned NULL pointer")
	}

	v.instance = ret
	v.state = StateReady
	v.logger.Info("moarvm instance initialized successfully", slog.Uint64("instance_ptr", uint64(v.instance)))

	if v.cfg.ProgName != "" {
		_ = v.setProgNameLocked(v.cfg.ProgName)
	}
	if len(v.cfg.Args) > 0 {
		_ = v.setArgsLocked(v.cfg.Args)
	}
	if len(v.cfg.LibPaths) > 0 {
		_ = v.setLibPathsLocked(v.cfg.LibPaths)
	}

	return nil
}

// Destroy cleans up the VM instance.
func (v *WindowsMoarVM) Destroy() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.instance == 0 || v.state == StateUninitialized || v.state == StateTerminated {
		return nil
	}

	v.logger.Info("destroying moarvm instance", slog.Uint64("instance_ptr", uint64(v.instance)))
	if v.procDestroy != nil {
		_, _, _ = v.procDestroy.Call(v.instance)
	}
	v.instance = 0
	v.state = StateTerminated

	if v.dll != nil {
		_ = v.dll.Release()
		v.dll = nil
	}
	v.logger.Info("moarvm instance destroyed")
	return nil
}

func (v *WindowsMoarVM) State() VMState {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state
}

func (v *WindowsMoarVM) RunFile(ctx context.Context, filePath string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.state != StateReady {
		return fmt.Errorf("%w: current state %s", ErrNotInitialized, v.state)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("moarvm: file not found: %s", filePath)
	}

	cFilePath, err := syscall.BytePtrFromString(filePath)
	if err != nil {
		return fmt.Errorf("moarvm: invalid filepath string: %w", err)
	}

	v.state = StateRunning
	v.logger.Info("executing bytecode file", slog.String("path", filePath))

	_, _, _ = v.procRunFile.Call(v.instance, uintptr(unsafe.Pointer(cFilePath)))

	v.state = StateReady
	return nil
}

func (v *WindowsMoarVM) RunBytecode(ctx context.Context, bytecode []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.state != StateReady {
		return fmt.Errorf("%w: current state %s", ErrNotInitialized, v.state)
	}
	if len(bytecode) == 0 {
		return fmt.Errorf("moarvm: empty bytecode")
	}

	v.state = StateRunning
	v.logger.Info("executing bytecode buffer", slog.Int("bytes", len(bytecode)))

	if v.procRunBC != nil {
		_, _, _ = v.procRunBC.Call(
			v.instance,
			uintptr(unsafe.Pointer(&bytecode[0])),
			uintptr(uint32(len(bytecode))),
		)
		v.state = StateReady
		return nil
	}

	tmpFile, err := os.CreateTemp("", "moar_*.mvm")
	if err != nil {
		v.state = StateReady
		return fmt.Errorf("moarvm: failed creating temporary bytecode file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(bytecode); err != nil {
		_ = tmpFile.Close()
		v.state = StateReady
		return fmt.Errorf("moarvm: failed writing bytecode to temp file: %w", err)
	}
	_ = tmpFile.Close()
	cFilePath, err := syscall.BytePtrFromString(tmpFile.Name())
	if err != nil {
		v.state = StateReady
		return err
	}
	_, _, _ = v.procRunFile.Call(v.instance, uintptr(unsafe.Pointer(cFilePath)))
	v.state = StateReady
	return nil
}

func (v *WindowsMoarVM) SetProgName(name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.setProgNameLocked(name)
}

func (v *WindowsMoarVM) setProgNameLocked(name string) error {
	if v.instance == 0 || v.procSetProg == nil {
		return ErrNotInitialized
	}
	cName, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	_, _, _ = v.procSetProg.Call(v.instance, uintptr(unsafe.Pointer(cName)))
	return nil
}

func (v *WindowsMoarVM) SetExecName(name string) error {
	return v.SetProgName(name)
}

func (v *WindowsMoarVM) SetArgs(args []string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.setArgsLocked(args)
}

func (v *WindowsMoarVM) setArgsLocked(args []string) error {
	if v.instance == 0 || v.procSetArgs == nil {
		return ErrNotInitialized
	}
	if len(args) == 0 {
		return nil
	}

	ptrs := make([]*byte, len(args))
	for i, arg := range args {
		p, err := syscall.BytePtrFromString(arg)
		if err != nil {
			return err
		}
		ptrs[i] = p
	}

	_, _, _ = v.procSetArgs.Call(
		v.instance,
		uintptr(len(args)),
		uintptr(unsafe.Pointer(&ptrs[0])),
	)
	return nil
}

func (v *WindowsMoarVM) SetLibPaths(paths []string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.setLibPathsLocked(paths)
}

func (v *WindowsMoarVM) setLibPathsLocked(paths []string) error {
	if v.instance == 0 || v.procSetLib == nil {
		return ErrNotInitialized
	}
	if len(paths) == 0 {
		return nil
	}

	ptrs := make([]*byte, len(paths)+1)
	for i, p := range paths {
		ptr, err := syscall.BytePtrFromString(p)
		if err != nil {
			return err
		}
		ptrs[i] = ptr
	}
	ptrs[len(paths)] = nil

	_, _, _ = v.procSetLib.Call(
		v.instance,
		uintptr(len(paths)),
		uintptr(unsafe.Pointer(&ptrs[0])),
	)
	return nil
}
