//go:build linux

package plugin

import (
	"os"
	"os/exec"
	"syscall"
)

func setupProcessHandling(cmd *exec.Cmd) processCleanup {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
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
