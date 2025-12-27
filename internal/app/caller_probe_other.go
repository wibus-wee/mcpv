//go:build !darwin && !linux

package app

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return true
}
