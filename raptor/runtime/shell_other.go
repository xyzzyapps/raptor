//go:build !windows

package raptor

import "os/exec"

func shellExecCommand(cmdStr string) *exec.Cmd {
	return exec.Command("sh", "-c", cmdStr)
}
