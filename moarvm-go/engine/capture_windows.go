//go:build windows

package moargo

import (
	"os"
	"sync"
	"syscall"
)

var (
	detachStdoutOnce sync.Once
	pinnedStdFiles   []*os.File
)

// detachGoStdoutFromCRT gives Go's os.Stdout its own duplicated Win32 HANDLE once
// before UCRT _dup2(osf, 1) closes CRT fd 1's initial HANDLE, and pins the original
// *os.File wrapper so Go's GC finalizer never double-closes that initial HANDLE.
func detachGoStdoutFromCRT(getOSF *syscall.LazyProc) {
	detachStdoutOnce.Do(func() {
		if os.Stdout == nil {
			return
		}
		crtH, _, _ := getOSF.Call(1)
		goH := os.Stdout.Fd()
		if crtH == 0 || crtH == ^uintptr(0) || crtH != goH {
			return
		}
		curProc, err := syscall.GetCurrentProcess()
		if err != nil {
			return
		}
		var dupGoH syscall.Handle
		if err := syscall.DuplicateHandle(curProc, syscall.Handle(goH), curProc, &dupGoH, 0, false, syscall.DUPLICATE_SAME_ACCESS); err != nil {
			return
		}
		origCopy := *os.Stdout
		pinnedStdFiles = append(pinnedStdFiles, &origCopy)

		if nf := os.NewFile(uintptr(dupGoH), os.Stdout.Name()); nf != nil {
			*os.Stdout = *nf
			syscall.Stdout = dupGoH
		}
	})
}

func captureStdout(fn func() error) (string, error) {
	tmp, err := os.CreateTemp("", "moar_stdout_")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	defer os.Remove(name)

	// Convert a duplicated Win32 handle to a CRT fd, then dup2 onto C stdout (fd 1).
	// Moar's say() writes through the C FILE* for fd 1, not SetStdHandle.
	var crt *syscall.LazyDLL
	for _, n := range []string{"ucrtbase.dll", "msvcrt.dll"} {
		d := syscall.NewLazyDLL(n)
		if d.NewProc("_dup2").Find() == nil {
			crt = d
			break
		}
	}
	if crt == nil {
		_ = tmp.Close()
		err := fn()
		return "", err
	}
	dup := crt.NewProc("_dup")
	dup2 := crt.NewProc("_dup2")
	closeFd := crt.NewProc("_close")
	openOSF := crt.NewProc("_open_osfhandle")
	getOSF := crt.NewProc("_get_osfhandle")

	detachGoStdoutFromCRT(getOSF)

	curProc, _ := syscall.GetCurrentProcess()
	var dupTmp syscall.Handle
	if derr := syscall.DuplicateHandle(curProc, syscall.Handle(tmp.Fd()), curProc, &dupTmp, 0, false, syscall.DUPLICATE_SAME_ACCESS); derr != nil {
		_ = tmp.Close()
		err := fn()
		return "", err
	}

	saved, _, _ := dup.Call(1)
	osf, _, _ := openOSF.Call(uintptr(dupTmp), 0x0008) // _O_APPEND
	if osf == ^uintptr(0) {
		_ = syscall.CloseHandle(dupTmp)
		_ = tmp.Close()
		err := fn()
		return "", err
	}
	_, _, _ = dup2.Call(osf, 1)

	runErr := fn()

	if saved != ^uintptr(0) {
		_, _, _ = dup2.Call(saved, 1)
		_, _, _ = closeFd.Call(saved)
	}
	_, _, _ = closeFd.Call(osf)
	_ = tmp.Close()

	data, rerr := os.ReadFile(name)
	if runErr != nil {
		return string(data), runErr
	}
	return string(data), rerr
}
