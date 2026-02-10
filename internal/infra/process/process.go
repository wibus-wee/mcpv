package process

import (
	"context"
	"errors"
	"os/exec"
)

type Cleanup func()

func Wait(ctx context.Context, cmd *exec.Cmd) error {
	if cmd == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	if ctx == nil {
		return <-done
	}
	select {
	case err := <-done:
		return normalizeExitError(err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func normalizeExitError(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == -1 {
		return nil
	}
	return err
}
