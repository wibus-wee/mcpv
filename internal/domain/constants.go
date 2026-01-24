package domain

const (
	// DefaultProtocolVersion is the default MCP protocol version.
	DefaultProtocolVersion = "2025-11-25"
	// DefaultStreamableHTTPProtocolVersion is the default streamable HTTP protocol version.
	DefaultStreamableHTTPProtocolVersion = "2025-06-18"
	// DefaultMaxConcurrent is the default max concurrent requests per instance.
	DefaultMaxConcurrent = 1
	// DefaultRouteTimeoutSeconds is the default route timeout in seconds.
	DefaultRouteTimeoutSeconds = 10
	// DefaultPingIntervalSeconds is the default ping interval in seconds.
	DefaultPingIntervalSeconds = 30
	// DefaultToolRefreshSeconds is the default tool refresh interval in seconds.
	DefaultToolRefreshSeconds = 60
	// DefaultToolRefreshConcurrency is the default tool refresh concurrency.
	DefaultToolRefreshConcurrency = 4
	// DefaultRefreshFailureThreshold is the default refresh failure threshold.
	DefaultRefreshFailureThreshold = 3
	// DefaultClientCheckSeconds is the default client check interval in seconds.
	DefaultClientCheckSeconds = 5
	// DefaultClientInactiveSeconds is the default client inactive threshold in seconds.
	DefaultClientInactiveSeconds = 300
	// DefaultServerInitRetryBaseSeconds is the default server init retry base in seconds.
	DefaultServerInitRetryBaseSeconds = 1
	// DefaultServerInitRetryMaxSeconds is the default server init retry max in seconds.
	DefaultServerInitRetryMaxSeconds = 30
	// DefaultServerInitMaxRetries is the default server init max retries.
	DefaultServerInitMaxRetries = 5
	// DefaultDrainTimeoutSeconds is the default drain timeout in seconds.
	DefaultDrainTimeoutSeconds = 30
	// DefaultExposeTools is the default flag for exposing tools.
	DefaultExposeTools = true
	// DefaultToolNamespaceStrategy is the default tool namespace strategy.
	DefaultToolNamespaceStrategy = "prefix"
	// DefaultObservabilityListenAddress is the default observability listen address.
	DefaultObservabilityListenAddress = "0.0.0.0:9090"
	// DefaultRPCListenAddress is the default RPC listen address.
	DefaultRPCListenAddress = "unix:///tmp/mcpd.sock"
	// DefaultRPCMaxRecvMsgSize is the default RPC max receive size in bytes.
	DefaultRPCMaxRecvMsgSize = 16 * 1024 * 1024
	// DefaultRPCMaxSendMsgSize is the default RPC max send size in bytes.
	DefaultRPCMaxSendMsgSize = 16 * 1024 * 1024
	// DefaultRPCKeepaliveTimeSeconds is the default RPC keepalive time in seconds.
	DefaultRPCKeepaliveTimeSeconds = 30
	// DefaultRPCKeepaliveTimeoutSeconds is the default RPC keepalive timeout in seconds.
	DefaultRPCKeepaliveTimeoutSeconds = 10
	// DefaultRPCSocketMode is the default RPC socket mode.
	DefaultRPCSocketMode = "0660"
	// DefaultStreamableHTTPMaxRetries is the default max retries for streamable HTTP.
	DefaultStreamableHTTPMaxRetries = 5

	// Instance strategy defaults
	// DefaultStrategy is the default instance strategy.
	DefaultStrategy = StrategyStateless
	// DefaultSessionTTLSeconds is the default session TTL in seconds.
	DefaultSessionTTLSeconds = 300 // 5 minutes
)

// StreamableHTTPProtocolVersions lists supported streamable HTTP protocol versions.
var StreamableHTTPProtocolVersions = []string{
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}
