//go:build !linux && !darwin

package daemon

import "context"

func platformServiceName() string {
	return "mcpv"
}

func platformInstall(_ context.Context, _ *Manager, _ string, _ string) error {
	return ErrUnsupported
}

func platformUninstall(_ context.Context, _ *Manager) error {
	return ErrUnsupported
}

func platformStart(_ context.Context, _ *Manager, _ string, _ string) error {
	return ErrUnsupported
}

func platformStop(_ context.Context, _ *Manager) error {
	return ErrUnsupported
}

func platformStatus(_ context.Context, _ *Manager) (bool, bool, error) {
	return false, false, ErrUnsupported
}
