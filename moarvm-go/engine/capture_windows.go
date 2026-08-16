//go:build windows

package moargo

import (
	"os"
	"syscall"
)

func captureStdout(fn func() error) (string, error) {
	tmp, err := os.CreateTemp("", "moar_stdout_")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	defer os.Remove(name)

	// Convert the Win32 handle to a CRT fd, then dup2 onto C stdout (fd 1).
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

	saved, _, _ := dup.Call(1)
	osf, _, _ := openOSF.Call(tmp.Fd(), 0x0008) // _O_APPEND
	if osf == 0 || osf == ^uintptr(0) {
		_ = tmp.Close()
		err := fn()
		return "", err
	}
	_, _, _ = dup2.Call(osf, 1)

	runErr := fn()

	if saved != 0 && saved != ^uintptr(0) {
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
