package domain

const (
	DefaultProtocolVersion               = "2025-11-25"
	DefaultStreamableHTTPProtocolVersion = "2025-06-18"
	DefaultMaxConcurrent                 = 1
	DefaultRouteTimeoutSeconds           = 10
	DefaultPingIntervalSeconds           = 30
	DefaultToolRefreshSeconds            = 60
	DefaultToolRefreshConcurrency        = 4
	DefaultRefreshFailureThreshold       = 3
	DefaultCallerCheckSeconds            = 5
	DefaultCallerInactiveSeconds         = 300
	DefaultServerInitRetryBaseSeconds    = 1
	DefaultServerInitRetryMaxSeconds     = 30
	DefaultServerInitMaxRetries          = 5
	DefaultDrainTimeoutSeconds           = 30
	DefaultExposeTools                   = true
	DefaultToolNamespaceStrategy         = "prefix"
	DefaultObservabilityListenAddress    = "0.0.0.0:9090"
	DefaultRPCListenAddress              = "unix:///tmp/mcpd.sock"
	DefaultRPCMaxRecvMsgSize             = 16 * 1024 * 1024
	DefaultRPCMaxSendMsgSize             = 16 * 1024 * 1024
	DefaultRPCKeepaliveTimeSeconds       = 30
	DefaultRPCKeepaliveTimeoutSeconds    = 10
	DefaultRPCSocketMode                 = "0660"
	DefaultStreamableHTTPMaxRetries      = 5

	// Instance strategy defaults
	DefaultStrategy          = StrategyStateless
	DefaultSessionTTLSeconds = 300 // 5 minutes
)

var StreamableHTTPProtocolVersions = []string{
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}
