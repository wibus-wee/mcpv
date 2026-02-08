//go:build !linux && !darwin

package process

import "os/exec"

func Setup(cmd *exec.Cmd) Cleanup {
	return nil
}
