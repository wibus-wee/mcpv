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
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry"
	"mcpv/internal/infra/telemetry/diagnostics"
)

type CommandLauncher struct {
	logger *zap.Logger
	probe  diagnostics.Probe
}

type CommandLauncherOptions struct {
	Logger *zap.Logger
	Probe  diagnostics.Probe
}

func NewCommandLauncher(opts CommandLauncherOptions) *CommandLauncher {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	probe := opts.Probe
	if probe == nil {
		probe = diagnostics.NoopProbe{}
	}
	return &CommandLauncher{
		logger: logger,
		probe:  probe,
	}
}

func (l *CommandLauncher) Start(ctx context.Context, specKey string, spec domain.ServerSpec) (domain.IOStreams, domain.StopFn, error) {
	attemptID, _ := diagnostics.AttemptIDFromContext(ctx)
	started := time.Now()
	if len(spec.Cmd) == 0 {
		l.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepLauncherStart,
			Phase:      diagnostics.PhaseError,
			Timestamp:  started,
			Error:      fmt.Errorf("%w: cmd is required for stdio launcher", domain.ErrInvalidCommand).Error(),
			Attributes: map[string]string{"argCount": "0"},
		})
		return domain.IOStreams{}, nil, fmt.Errorf("%w: cmd is required for stdio launcher", domain.ErrInvalidCommand)
	}
	attrs := map[string]string{
		"executable": spec.Cmd[0],
		"argCount":   strconv.Itoa(len(spec.Cmd) - 1),
	}
	if spec.Cwd != "" {
		attrs["cwd"] = spec.Cwd
	}
	if len(spec.Env) > 0 {
		attrs["envKeys"] = strings.Join(sortedKeys(spec.Env), ",")
		attrs["envCount"] = strconv.Itoa(len(spec.Env))
	}
	sensitive := map[string]string{}
	if l.captureSensitive() {
		sensitive["cmd"] = strings.Join(spec.Cmd, " ")
		if len(spec.Env) > 0 {
			sensitive["env"] = diagnostics.EncodeStringMap(spec.Env)
		}
	}
	l.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepLauncherStart,
		Phase:      diagnostics.PhaseEnter,
		Timestamp:  started,
		Attributes: attrs,
		Sensitive:  sensitive,
	})

	cmd := exec.CommandContext(ctx, spec.Cmd[0], spec.Cmd[1:]...)
	if spec.Cwd != "" {
		cmd.Dir = spec.Cwd
	}
	cmd.Env = append(os.Environ(), formatEnv(spec.Env)...)
	groupCleanup := setupProcessHandling(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		l.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepLauncherStart,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      fmt.Errorf("stdout pipe: %w", err).Error(),
			Attributes: attrs,
			Sensitive:  sensitive,
		})
		return domain.IOStreams{}, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		l.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepLauncherStart,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      fmt.Errorf("stdin pipe: %w", err).Error(),
			Attributes: attrs,
			Sensitive:  sensitive,
		})
		return domain.IOStreams{}, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		l.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepLauncherStart,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      fmt.Errorf("stderr pipe: %w", err).Error(),
			Attributes: attrs,
			Sensitive:  sensitive,
		})
		return domain.IOStreams{}, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		l.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepLauncherStart,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      fmt.Errorf("start command: %w", classifyStartError(err)).Error(),
			Attributes: attrs,
			Sensitive:  sensitive,
		})
		return domain.IOStreams{}, nil, fmt.Errorf("start command: %w", classifyStartError(err))
	}
	l.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepLauncherStart,
		Phase:      diagnostics.PhaseExit,
		Timestamp:  time.Now(),
		Duration:   time.Since(started),
		Attributes: attrs,
		Sensitive:  sensitive,
	})

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
	keys := sortedKeys(env)
	out := make([]string, 0, len(env))
	for _, k := range keys {
		v := env[k]
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

func sortedKeys(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

func (l *CommandLauncher) recordEvent(event diagnostics.Event) {
	if l == nil || l.probe == nil {
		return
	}
	if len(event.Sensitive) == 0 {
		event.Sensitive = nil
	}
	l.probe.Record(event)
}

func (l *CommandLauncher) captureSensitive() bool {
	if l == nil || l.probe == nil {
		return false
	}
	if probe, ok := l.probe.(diagnostics.SensitiveProbe); ok {
		return probe.CaptureSensitive()
	}
	return false
}
