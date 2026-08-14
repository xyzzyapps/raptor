//go:build windows && !wasm && !js

package raptor

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func loadDynamicLibrary(path string) (uintptr, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		k32, err := syscall.LoadLibrary("kernel32.dll")
		if err == nil {
			setDllDir, err := syscall.GetProcAddress(k32, "SetDllDirectoryW")
			if err == nil {
				dirPtr, err := syscall.UTF16PtrFromString(dir)
				if err == nil {
					_, _, _ = syscall.SyscallN(setDllDir, uintptr(unsafe.Pointer(dirPtr)))
				}
			}
		}
	}

	handle, err := syscall.LoadLibrary(path)
	if err != nil {
		baseName := filepath.Base(path)
		candidates := []string{
			filepath.Join("bin", baseName),
			filepath.Join(filepath.Dir(os.Args[0]), baseName),
			baseName,
		}
		for _, cand := range candidates {
			if h, e := syscall.LoadLibrary(cand); e == nil {
				return uintptr(h), nil
			}
		}
		return 0, fmt.Errorf("failed loading library %q: %w", path, err)
	}
	return uintptr(handle), nil
}

func getDynamicProcAddress(handle uintptr, name string) (uintptr, error) {
	addr, err := syscall.GetProcAddress(syscall.Handle(handle), name)
	if err != nil {
		return 0, err
	}
	return addr, nil
}

func callDynamicProc(proc uintptr, args ...uintptr) (uintptr, error) {
	r1, _, err := syscall.SyscallN(proc, args...)
	return r1, err
}

func freeDynamicLibrary(handle uintptr) error {
	return syscall.FreeLibrary(syscall.Handle(handle))
}

func createDynamicCallback(fn func(a1, a2, a3, a4 uintptr) uintptr) uintptr {
	return syscall.NewCallback(fn)
}
