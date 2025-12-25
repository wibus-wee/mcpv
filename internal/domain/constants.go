package domain

const (
	DefaultProtocolVersion            = "2025-11-25"
	DefaultRouteTimeoutSeconds        = 10
	DefaultPingIntervalSeconds        = 30
	DefaultToolRefreshSeconds         = 60
	DefaultExposeTools                = true
	DefaultToolNamespaceStrategy      = "prefix"
	DefaultRPCListenAddress           = "unix:///tmp/mcpd.sock"
	DefaultRPCMaxRecvMsgSize          = 16 * 1024 * 1024
	DefaultRPCMaxSendMsgSize          = 16 * 1024 * 1024
	DefaultRPCKeepaliveTimeSeconds    = 30
	DefaultRPCKeepaliveTimeoutSeconds = 10
)
