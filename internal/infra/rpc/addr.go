package rpc

import (
	"errors"
	"strings"
)

func parseListenAddress(addr string) (string, string, error) {
	if strings.TrimSpace(addr) == "" {
		return "", "", errors.New("rpc.listenAddress is required")
	}
	if strings.HasPrefix(addr, "unix://") {
		path := strings.TrimPrefix(addr, "unix://")
		if path == "" {
			return "", "", errors.New("rpc.listenAddress unix path is empty")
		}
		return "unix", path, nil
	}
	if strings.HasPrefix(addr, "tcp://") {
		host := strings.TrimPrefix(addr, "tcp://")
		if host == "" {
			return "", "", errors.New("rpc.listenAddress tcp host is empty")
		}
		return "tcp", host, nil
	}
	return "tcp", addr, nil
}

func normalizeTargetAddress(addr string) (string, error) {
	if strings.TrimSpace(addr) == "" {
		return "", errors.New("rpc address is required")
	}
	if strings.HasPrefix(addr, "unix://") {
		return addr, nil
	}
	if strings.HasPrefix(addr, "tcp://") {
		host := strings.TrimPrefix(addr, "tcp://")
		if host == "" {
			return "", errors.New("rpc tcp address is empty")
		}
		return host, nil
	}
	return addr, nil
}
