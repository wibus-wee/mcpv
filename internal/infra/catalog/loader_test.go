package catalog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/fsutil"
)

func TestLoader_Success(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: git-helper
    cmd: ["./git-helper"]
    idleSeconds: 60
    maxConcurrent: 2
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
	`)

	loader := NewLoader(zap.NewNop())
	catalog, err := loader.Load(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, catalog.Specs, 1)

	got := catalog.Specs["git-helper"]
	expect := domain.ServerSpec{
		Name:                "git-helper",
		Transport:           domain.TransportStdio,
		Cmd:                 []string{"./git-helper"},
		IdleSeconds:         60,
		MaxConcurrent:       2,
		Strategy:            domain.StrategyStateless,
		MinReady:            0,
		DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
		ProtocolVersion:     domain.DefaultProtocolVersion,
	}
	if diff := cmp.Diff(expect, got); diff != "" {
		t.Fatalf("spec mismatch (-want +got):\n%s", diff)
	}

	require.Equal(t, domain.DefaultRouteTimeoutSeconds, catalog.Runtime.RouteTimeoutSeconds)
	require.Equal(t, domain.DefaultPingIntervalSeconds, catalog.Runtime.PingIntervalSeconds)
	require.Equal(t, domain.DefaultToolRefreshSeconds, catalog.Runtime.ToolRefreshSeconds)
	require.Equal(t, domain.DefaultToolRefreshConcurrency, catalog.Runtime.ToolRefreshConcurrency)
	require.Equal(t, domain.DefaultClientCheckSeconds, catalog.Runtime.ClientCheckSeconds)
	require.Equal(t, domain.DefaultClientInactiveSeconds, catalog.Runtime.ClientInactiveSeconds)
	require.Equal(t, domain.DefaultServerInitRetryBaseSeconds, catalog.Runtime.ServerInitRetryBaseSeconds)
	require.Equal(t, domain.DefaultServerInitRetryMaxSeconds, catalog.Runtime.ServerInitRetryMaxSeconds)
	require.Equal(t, domain.DefaultServerInitMaxRetries, catalog.Runtime.ServerInitMaxRetries)
	require.Equal(t, domain.DefaultBootstrapMode, catalog.Runtime.BootstrapMode)
	require.Equal(t, domain.DefaultBootstrapConcurrency, catalog.Runtime.BootstrapConcurrency)
	require.Equal(t, domain.DefaultBootstrapTimeoutSeconds, catalog.Runtime.BootstrapTimeoutSeconds)
	require.Equal(t, domain.DefaultActivationMode, catalog.Runtime.DefaultActivationMode)
	require.Equal(t, domain.DefaultExposeTools, catalog.Runtime.ExposeTools)
	require.Equal(t, domain.DefaultToolNamespaceStrategy, catalog.Runtime.ToolNamespaceStrategy)
	require.Equal(t, domain.DefaultObservabilityListenAddress, catalog.Runtime.Observability.ListenAddress)
	require.Equal(t, domain.DefaultRPCListenAddress, catalog.Runtime.RPC.ListenAddress)
	require.Equal(t, domain.DefaultRPCMaxRecvMsgSize, catalog.Runtime.RPC.MaxRecvMsgSize)
	require.Equal(t, domain.DefaultRPCMaxSendMsgSize, catalog.Runtime.RPC.MaxSendMsgSize)
	require.Equal(t, domain.DefaultRPCKeepaliveTimeSeconds, catalog.Runtime.RPC.KeepaliveTimeSeconds)
	require.Equal(t, domain.DefaultRPCKeepaliveTimeoutSeconds, catalog.Runtime.RPC.KeepaliveTimeoutSeconds)
	require.Equal(t, domain.DefaultRPCSocketMode, catalog.Runtime.RPC.SocketMode)
}

func TestLoader_EnvExpansion(t *testing.T) {
	t.Setenv("SERVER_CMD", "./from-\"env\"")
	file := writeTempConfig(t, `
servers:
  - name: env-server
    cmd: ["${SERVER_CMD}"]
    idleSeconds: 0
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`)

	loader := NewLoader(zap.NewNop())
	catalog, err := loader.Load(context.Background(), file)
	require.NoError(t, err)
	require.Equal(t, []string{"./from-\"env\""}, catalog.Specs["env-server"].Cmd)
}

func TestLoader_EnvExpansionNumeric(t *testing.T) {
	t.Setenv("ROUTE_TIMEOUT", "15")
	file := writeTempConfig(t, `
routeTimeoutSeconds: ${ROUTE_TIMEOUT}
servers:
  - name: env-server
    cmd: ["./from-env"]
    idleSeconds: 0
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`)

	loader := NewLoader(zap.NewNop())
	catalog, err := loader.Load(context.Background(), file)
	require.NoError(t, err)
	require.Equal(t, 15, catalog.Runtime.RouteTimeoutSeconds)
}

func TestLoader_DuplicateName(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: dup
    cmd: ["./a"]
    idleSeconds: 0
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
  - name: dup
    cmd: ["./b"]
    idleSeconds: 0
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`)

	loader := NewLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate name")
}

func TestLoader_InvalidProtocolVersion(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: bad-protocol
    cmd: ["./a"]
    idleSeconds: 0
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2024-01"
`)

	loader := NewLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "protocolVersion must match")
}

func TestLoader_MissingRequiredFields(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: ""
    cmd: []
    idleSeconds: -1
    maxConcurrent: 0
    strategy: stateless
    minReady: -2
    protocolVersion: ""
`)

	loader := NewLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")
	require.Contains(t, err.Error(), "cmd is required")
	require.Contains(t, err.Error(), "idleSeconds must be")
	require.Contains(t, err.Error(), "minReady must be")
}

func TestLoader_NoServers(t *testing.T) {
	file := writeTempConfig(t, `
servers: []
`)

	loader := NewLoader(zap.NewNop())
	catalog, err := loader.Load(context.Background(), file)
	require.NoError(t, err)
	require.Empty(t, catalog.Specs)
}

func TestLoader_ContextCanceled(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: ok
    cmd: ["./a"]
    idleSeconds: 0
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	loader := NewLoader(zap.NewNop())
	_, err := loader.Load(ctx, file)
	require.ErrorIs(t, err, context.Canceled)
}

func TestLoader_InvalidRuntimeConfig(t *testing.T) {
	file := writeTempConfig(t, `
routeTimeoutSeconds: 0
pingIntervalSeconds: -1
toolRefreshSeconds: -2
clientCheckSeconds: 0
clientInactiveSeconds: 0
toolNamespaceStrategy: "bad"
rpc:
  listenAddress: ""
  maxRecvMsgSize: 0
  maxSendMsgSize: 0
  keepaliveTimeSeconds: -1
  keepaliveTimeoutSeconds: -2
  socketMode: "bad"
  tls:
    enabled: true
    certFile: ""
    keyFile: ""
    caFile: ""
    clientAuth: true
servers:
  - name: ok
    cmd: ["./a"]
    idleSeconds: 0
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`)

	loader := NewLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "routeTimeoutSeconds")
	require.Contains(t, err.Error(), "pingIntervalSeconds")
	require.Contains(t, err.Error(), "toolRefreshSeconds")
	require.Contains(t, err.Error(), "clientCheckSeconds")
	require.Contains(t, err.Error(), "clientInactiveSeconds")
	require.Contains(t, err.Error(), "toolNamespaceStrategy")
	require.Contains(t, err.Error(), "rpc.listenAddress")
	require.Contains(t, err.Error(), "rpc.maxRecvMsgSize")
	require.Contains(t, err.Error(), "rpc.maxSendMsgSize")
	require.Contains(t, err.Error(), "rpc.keepaliveTimeSeconds")
	require.Contains(t, err.Error(), "rpc.keepaliveTimeoutSeconds")
	require.Contains(t, err.Error(), "rpc.socketMode")
	require.Contains(t, err.Error(), "rpc.tls.certFile")
	require.Contains(t, err.Error(), "rpc.tls.caFile")
}

func TestLoader_ServerSpecDefaults(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: defaults
    cmd: ["./svc"]
`)

	loader := NewLoader(zap.NewNop())
	catalog, err := loader.Load(context.Background(), file)
	require.NoError(t, err)

	got := catalog.Specs["defaults"]
	require.Equal(t, domain.DefaultProtocolVersion, got.ProtocolVersion)
	require.Equal(t, domain.DefaultMaxConcurrent, got.MaxConcurrent)
	require.Equal(t, domain.DefaultDrainTimeoutSeconds, got.DrainTimeoutSeconds)
	require.Equal(t, domain.TransportStdio, got.Transport)
}

func TestLoader_StreamableHTTPSuccess(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: remote
    transport: streamable_http
    idleSeconds: 0
    maxConcurrent: 2
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-06-18"
    http:
      endpoint: "https://example.com/mcp"
      headers:
        Authorization: "Bearer token"
      maxRetries: 2
`)

	loader := NewLoader(zap.NewNop())
	catalog, err := loader.Load(context.Background(), file)
	require.NoError(t, err)

	got := catalog.Specs["remote"]
	require.Equal(t, domain.TransportStreamableHTTP, got.Transport)
	require.Nil(t, got.Cmd)
	require.Equal(t, "https://example.com/mcp", got.HTTP.Endpoint)
	require.Equal(t, 2, got.HTTP.MaxRetries)
	require.Equal(t, "Bearer token", got.HTTP.Headers["Authorization"])
	require.Equal(t, "2025-06-18", got.ProtocolVersion)
}

func TestLoader_StreamableHTTPImplicitTransport(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: remote
    http:
      endpoint: "https://example.com/mcp"
`)

	loader := NewLoader(zap.NewNop())
	catalog, err := loader.Load(context.Background(), file)
	require.NoError(t, err)

	got := catalog.Specs["remote"]
	require.Equal(t, domain.TransportStreamableHTTP, got.Transport)
	require.Equal(t, "https://example.com/mcp", got.HTTP.Endpoint)
	require.Equal(t, domain.DefaultStreamableHTTPProtocolVersion, got.ProtocolVersion)
}

func TestLoader_StreamableHTTPInvalid(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: remote
    transport: streamable_http
    cmd: ["./svc"]
    http:
      endpoint: "file://nope"
`)

	loader := NewLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cmd must be empty")
	require.Contains(t, err.Error(), "http.endpoint must be a valid http(s) URL")
}

func TestLoader_StatefulSessionTTLOmittedUsesDefault(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: stateful-default
    cmd: ["./svc"]
    strategy: stateful
`)

	loader := NewLoader(zap.NewNop())
	catalog, err := loader.Load(context.Background(), file)
	require.NoError(t, err)

	got := catalog.Specs["stateful-default"]
	require.Equal(t, domain.StrategyStateful, got.Strategy)
	require.Equal(t, domain.DefaultSessionTTLSeconds, got.SessionTTLSeconds)
}

func TestLoader_StatefulSessionTTLZeroPreserved(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: stateful-no-ttl
    cmd: ["./svc"]
    strategy: stateful
    sessionTTLSeconds: 0
`)

	loader := NewLoader(zap.NewNop())
	catalog, err := loader.Load(context.Background(), file)
	require.NoError(t, err)

	got := catalog.Specs["stateful-no-ttl"]
	require.Equal(t, domain.StrategyStateful, got.Strategy)
	require.Equal(t, 0, got.SessionTTLSeconds)
}

func TestLoader_SchemaUnknownKey(t *testing.T) {
	file := writeTempConfig(t, `
unknownKey: true
servers:
  - name: ok
    cmd: ["./a"]
    idleSeconds: 0
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`)

	loader := NewLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema validation failed")
}

func TestLoader_SchemaWrongType(t *testing.T) {
	file := writeTempConfig(t, `
routeTimeoutSeconds: "fast"
servers:
  - name: ok
    cmd: ["./a"]
    idleSeconds: 0
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`)

	loader := NewLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema validation failed")
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	normalized := strings.ReplaceAll(content, "\t", "  ")
	if err := os.WriteFile(path, []byte(normalized), fsutil.DefaultFileMode); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
