package domain

const (
	DefaultProtocolVersion            = "2025-11-25"
	DefaultMaxConcurrent              = 1
	DefaultRouteTimeoutSeconds        = 10
	DefaultPingIntervalSeconds        = 30
	DefaultToolRefreshSeconds         = 60
	DefaultCallerCheckSeconds         = 5
	DefaultExposeTools                = true
	DefaultToolNamespaceStrategy      = "prefix"
	DefaultObservabilityListenAddress = "0.0.0.0:9090"
	DefaultRPCListenAddress           = "unix:///tmp/mcpd.sock"
	DefaultRPCMaxRecvMsgSize          = 16 * 1024 * 1024
	DefaultRPCMaxSendMsgSize          = 16 * 1024 * 1024
	DefaultRPCKeepaliveTimeSeconds    = 30
	DefaultRPCKeepaliveTimeoutSeconds = 10
	DefaultRPCSocketMode              = "0660"
)
