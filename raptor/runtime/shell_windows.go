//go:build windows

package raptor

import (
	"os/exec"
	"syscall"
)

func shellExecCommand(cmdStr string) *exec.Cmd {
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: `cmd /d /s /c "` + cmdStr + `"`,
	}
	return cmd
}
