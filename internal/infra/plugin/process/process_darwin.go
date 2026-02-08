//go:build darwin

package process

import (
	"os"
	"os/exec"
	"syscall"
)

func Setup(cmd *exec.Cmd) Cleanup {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Cancel = func() error {
		return killProcessGroup(cmd.Process)
	}
	return func() {
		_ = killProcessGroup(cmd.Process)
	}
}

func killProcessGroup(proc *os.Process) error {
	if proc == nil {
		return nil
	}
	if err := syscall.Kill(-proc.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}
