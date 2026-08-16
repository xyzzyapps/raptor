//go:build unix

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

	saved, err := syscall.Dup(1)
	if err != nil {
		tmp.Close()
		return "", err
	}
	if err := syscall.Dup2(int(tmp.Fd()), 1); err != nil {
		tmp.Close()
		_ = syscall.Close(saved)
		return "", err
	}

	runErr := fn()

	_ = syscall.Dup2(saved, 1)
	_ = syscall.Close(saved)
	_ = tmp.Close()

	data, rerr := os.ReadFile(name)
	if runErr != nil {
		return string(data), runErr
	}
	return string(data), rerr
}
