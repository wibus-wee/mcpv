package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcpd/internal/domain"
)

type StdioTransport struct{}

func NewStdioTransport() *StdioTransport {
	return &StdioTransport{}
}

type processCleanup func()

func (t *StdioTransport) Start(ctx context.Context, spec domain.ServerSpec) (domain.Conn, domain.StopFn, error) {
	if len(spec.Cmd) == 0 {
		return nil, nil, errors.New("cmd is required for stdio transport")
	}

	cmd := exec.CommandContext(ctx, spec.Cmd[0], spec.Cmd[1:]...)
	if spec.Cwd != "" {
		cmd.Dir = spec.Cwd
	}
	cmd.Env = append(os.Environ(), formatEnv(spec.Env)...)
	groupCleanup := setupProcessHandling(cmd)

	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	mcpConn, err := transport.Connect(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("connect stdio: %w", err)
	}

	conn := &mcpConnAdapter{conn: mcpConn}
	stop := func(stopCtx context.Context) error {
		_ = mcpConn.Close()
		if groupCleanup != nil {
			groupCleanup()
		}
		return nil
	}

	return conn, stop, nil
}

type mcpConnAdapter struct {
	conn mcp.Connection
}

func (a *mcpConnAdapter) Send(ctx context.Context, msg json.RawMessage) error {
	if len(msg) == 0 {
		return errors.New("message is empty")
	}
	decoded, err := jsonrpc.DecodeMessage(msg)
	if err != nil {
		return fmt.Errorf("decode message: %w", err)
	}
	return a.conn.Write(ctx, decoded)
}

func (a *mcpConnAdapter) Recv(ctx context.Context) (json.RawMessage, error) {
	msg, err := a.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := jsonrpc.EncodeMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("encode message: %w", err)
	}
	return json.RawMessage(raw), nil
}

func (a *mcpConnAdapter) Close() error {
	return a.conn.Close()
}

func formatEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
