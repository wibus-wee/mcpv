package rpc

import (
	"strings"

	"mcpv/internal/domain"
)

func parseListenAddress(addr string) (string, string, error) {
	if strings.TrimSpace(addr) == "" {
		return "", "", domain.E(domain.CodeInvalidArgument, "rpc listen address", "rpc.listenAddress is required", nil)
	}
	if strings.HasPrefix(addr, "unix://") {
		path := strings.TrimPrefix(addr, "unix://")
		if path == "" {
			return "", "", domain.E(domain.CodeInvalidArgument, "rpc listen address", "rpc.listenAddress unix path is empty", nil)
		}
		return "unix", path, nil
	}
	if strings.HasPrefix(addr, "tcp://") {
		host := strings.TrimPrefix(addr, "tcp://")
		if host == "" {
			return "", "", domain.E(domain.CodeInvalidArgument, "rpc listen address", "rpc.listenAddress tcp host is empty", nil)
		}
		return "tcp", host, nil
	}
	return "tcp", addr, nil
}

func normalizeTargetAddress(addr string) (string, error) {
	if strings.TrimSpace(addr) == "" {
		return "", domain.E(domain.CodeInvalidArgument, "rpc target address", "rpc address is required", nil)
	}
	if strings.HasPrefix(addr, "unix://") {
		return addr, nil
	}
	if strings.HasPrefix(addr, "tcp://") {
		host := strings.TrimPrefix(addr, "tcp://")
		if host == "" {
			return "", domain.E(domain.CodeInvalidArgument, "rpc target address", "rpc tcp address is empty", nil)
		}
		return host, nil
	}
	return addr, nil
}
