package transport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry"
)

type CommandLauncher struct {
	logger *zap.Logger
}

type CommandLauncherOptions struct {
	Logger *zap.Logger
}

func NewCommandLauncher(opts CommandLauncherOptions) *CommandLauncher {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CommandLauncher{logger: logger}
}

func (l *CommandLauncher) Start(ctx context.Context, _ string, spec domain.ServerSpec) (domain.IOStreams, domain.StopFn, error) {
	if len(spec.Cmd) == 0 {
		return domain.IOStreams{}, nil, fmt.Errorf("%w: cmd is required for stdio launcher", domain.ErrInvalidCommand)
	}

	cmd := exec.CommandContext(ctx, spec.Cmd[0], spec.Cmd[1:]...)
	if spec.Cwd != "" {
		cmd.Dir = spec.Cwd
	}
	cmd.Env = append(os.Environ(), formatEnv(spec.Env)...)
	groupCleanup := setupProcessHandling(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return domain.IOStreams{}, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return domain.IOStreams{}, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return domain.IOStreams{}, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return domain.IOStreams{}, nil, fmt.Errorf("start command: %w", classifyStartError(err))
	}

	downstreamLogger := l.logger.With(
		zap.String(telemetry.FieldLogSource, telemetry.LogSourceDownstream),
		telemetry.ServerTypeField(spec.Name),
		zap.String(telemetry.FieldLogStream, "stderr"),
	)
	go mirrorStderr(stderr, downstreamLogger)

	stop := func(stopCtx context.Context) error {
		if err := stdin.Close(); err != nil {
			l.logger.Warn("close stdin failed", zap.Error(err))
		}
		if err := stdout.Close(); err != nil {
			l.logger.Warn("close stdout failed", zap.Error(err))
		}
		if err := stderr.Close(); err != nil {
			l.logger.Warn("close stderr failed", zap.Error(err))
		}
		if groupCleanup != nil {
			groupCleanup()
		}
		return waitForProcess(stopCtx, cmd)
	}

	return domain.IOStreams{Reader: stdout, Writer: stdin}, stop, nil
}

const maxStderrLineLength = 32 * 1024 // 32KB per line

func mirrorStderr(reader io.Reader, logger *zap.Logger) {
	buf := bufio.NewReaderSize(reader, 8192)
	for {
		line, isPrefix, err := buf.ReadLine()
		if len(line) > 0 {
			trimmed := strings.TrimRight(string(line), "\r\n")
			if trimmed != "" {
				if len(trimmed) > maxStderrLineLength {
					logger.Warn("stderr line truncated",
						zap.Int("originalLength", len(trimmed)),
						zap.Int("maxLength", maxStderrLineLength),
					)
					trimmed = trimmed[:maxStderrLineLength] + "... [truncated]"
				}
				logger.Info(trimmed)
			}
			// If isPrefix is true, the line was longer than buffer; skip remaining
			if isPrefix {
				// Discard rest of oversized line
				for isPrefix && err == nil {
					_, isPrefix, err = buf.ReadLine()
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func formatEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(env))
	for _, k := range keys {
		v := env[k]
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

func classifyStartError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s", domain.ErrExecutableNotFound, err.Error())
	}
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("%w: %s", domain.ErrPermissionDenied, err.Error())
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errors.Is(pathErr.Err, exec.ErrNotFound) || errors.Is(pathErr.Err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", domain.ErrExecutableNotFound, err.Error())
		}
		if errors.Is(pathErr.Err, os.ErrPermission) {
			return fmt.Errorf("%w: %s", domain.ErrPermissionDenied, err.Error())
		}
	}
	return err
}
