//go:build unix && !(linux && (arm64 || riscv64 || loong64))

// dup_unix.go duplicates file descriptors via standard POSIX dup2.
package moargo

import "syscall"

func dupFD(oldfd, newfd int) error {
	return syscall.Dup2(oldfd, newfd)
}
