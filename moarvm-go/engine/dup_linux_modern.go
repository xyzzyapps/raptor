//go:build linux && (arm64 || riscv64 || loong64)

// dup_linux_modern.go duplicates file descriptors via dup3 on Linux architectures without legacy SYS_dup2.
package moargo

import "syscall"

func dupFD(oldfd, newfd int) error {
	if oldfd == newfd {
		return nil
	}
	return syscall.Dup3(oldfd, newfd, 0)
}
