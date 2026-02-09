package gateway

import "os"

func resolveCallerPID() int64 {
	ppid := os.Getppid()
	if ppid > 0 {
		return int64(ppid)
	}
	pid := os.Getpid()
	if pid > 0 {
		return int64(pid)
	}
	return 0
}
