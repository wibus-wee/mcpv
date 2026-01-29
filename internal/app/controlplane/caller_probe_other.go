//go:build !darwin && !linux

package controlplane

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return true
}
