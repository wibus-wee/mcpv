//go:build !linux && !darwin

package transport

import "os/exec"

func setupProcessHandling(cmd *exec.Cmd) processCleanup {
	return nil
}
