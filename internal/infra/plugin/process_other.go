//go:build !linux && !darwin

package plugin

import "os/exec"

func setupProcessHandling(cmd *exec.Cmd) processCleanup {
	return nil
}
