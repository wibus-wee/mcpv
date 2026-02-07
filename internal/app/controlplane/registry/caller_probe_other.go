//go:build !darwin && !linux

package registry

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return true
}
