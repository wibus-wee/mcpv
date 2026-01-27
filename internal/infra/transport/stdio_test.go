package transport

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"mcpd/internal/domain"
	"mcpd/internal/infra/telemetry"
)

func TestCommandLauncher_StartAndRoundTrip(t *testing.T) {
	launcher := NewCommandLauncher(CommandLauncherOptions{})
	transport := NewMCPTransport(MCPTransportOptions{})
	spec := domain.ServerSpec{
		Name:            "echo",
		Cmd:             []string{"python3", "-u", "-c", pythonEchoServerScript},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	streams, stop, err := launcher.Start(ctx, "spec-echo", spec)
	require.NoError(t, err)
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		require.NoError(t, stop(stopCtx))
	}()

	conn, err := transport.Connect(ctx, "spec-echo", spec, streams)
	require.NoError(t, err)
	defer conn.Close()

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`)
	got, err := conn.Call(ctx, msg)
	require.NoError(t, err)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`, string(got))
}

func TestCommandLauncher_InvalidCmd(t *testing.T) {
	launcher := NewCommandLauncher(CommandLauncherOptions{})
	spec := domain.ServerSpec{
		Name:            "bad",
		Cmd:             []string{},
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	_, _, err := launcher.Start(context.Background(), "spec-bad", spec)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidCommand)
}

func TestCommandLauncher_StopKillsProcess(t *testing.T) {
	launcher := NewCommandLauncher(CommandLauncherOptions{})
	spec := domain.ServerSpec{
		Name:            "sleep",
		Cmd:             []string{"/bin/sh", "-c", "sleep 10"},
		ProtocolVersion: domain.DefaultProtocolVersion,
		MaxConcurrent:   1,
	}

	streams, stop, err := launcher.Start(context.Background(), "spec-sleep", spec)
	require.NoError(t, err)
	_ = streams.Reader.Close()
	_ = streams.Writer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = stop(ctx)
	require.NoError(t, err)
}

func TestCommandLauncher_MissingExecutable(t *testing.T) {
	launcher := NewCommandLauncher(CommandLauncherOptions{})
	spec := domain.ServerSpec{
		Name:            "missing",
		Cmd:             []string{"/no/such/binary"},
		ProtocolVersion: domain.DefaultProtocolVersion,
		MaxConcurrent:   1,
	}

	_, _, err := launcher.Start(context.Background(), "spec-missing", spec)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrExecutableNotFound)
}

func TestCommandLauncher_MirrorsStderr(t *testing.T) {
	logs := telemetry.NewLogBroadcaster(zapcore.InfoLevel)
	logger := zap.New(zapcore.NewTee(zapcore.NewNopCore(), logs.Core()))
	logger = logger.With(zap.String(telemetry.FieldLogSource, telemetry.LogSourceCore))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logCh := logs.Subscribe(ctx)

	launcher := NewCommandLauncher(CommandLauncherOptions{Logger: logger})
	spec := domain.ServerSpec{
		Name:            "stderr",
		Cmd:             []string{"/bin/sh", "-c", "echo \"stderr line\" 1>&2; cat"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	streams, stop, err := launcher.Start(ctx, "spec-stderr", spec)
	require.NoError(t, err)
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		require.NoError(t, stop(stopCtx))
	}()
	_ = streams.Reader.Close()
	_ = streams.Writer.Close()

	entry := waitForDownstreamLog(t, logCh)
	require.NotEmpty(t, entry.Data)

	fields, ok := entry.Data["fields"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, telemetry.LogSourceDownstream, fields[telemetry.FieldLogSource])
	require.Equal(t, "stderr", fields[telemetry.FieldLogStream])
	require.Equal(t, spec.Name, fields[telemetry.FieldServerType])
	require.Equal(t, "stderr line", entry.Data["message"])
}

func waitForDownstreamLog(t *testing.T, logCh <-chan domain.LogEntry) domain.LogEntry {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case entry, ok := <-logCh:
			if !ok {
				t.Fatal("log channel closed before downstream log")
			}
			if len(entry.Data) == 0 {
				continue
			}
			fields, ok := entry.Data["fields"].(map[string]any)
			if !ok {
				continue
			}
			if fields[telemetry.FieldLogSource] == telemetry.LogSourceDownstream {
				return entry
			}
		case <-deadline:
			t.Fatal("timed out waiting for downstream stderr log")
		}
	}
}

const pythonEchoServerScript = `import sys, json
for line in sys.stdin:
    msg = json.loads(line)
    if "id" in msg:
        resp = {"jsonrpc": "2.0", "id": msg["id"], "result": {"ok": True}}
        sys.stdout.write(json.dumps(resp) + "\n")
        sys.stdout.flush()
`
