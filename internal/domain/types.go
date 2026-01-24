package domain

import (
	"errors"
	"time"
)

// InstanceStrategy defines how instances are managed for a server spec.
type InstanceStrategy string

const (
	// StrategyStateless: requests are distributed across instances via round-robin.
	// Instances are reclaimed after idle timeout.
	// Use for: HTTP APIs, stateless compute tasks.
	StrategyStateless InstanceStrategy = "stateless"

	// StrategyStateful: requests with the same routingKey are routed to the same instance.
	// Bindings expire after sessionTTLSeconds, then instances can be reclaimed after idle timeout.
	// Use for: file system sessions, temporary caches.
	StrategyStateful InstanceStrategy = "stateful"

	// StrategyPersistent: instances are never reclaimed regardless of idle state.
	// No session binding. Requests are distributed via round-robin.
	// Use for: database connection pools, long-lived services.
	StrategyPersistent InstanceStrategy = "persistent"

	// StrategySingleton: only one instance exists globally.
	// Never reclaimed. All requests go to the single instance.
	// Use for: config servers, task queues.
	StrategySingleton InstanceStrategy = "singleton"
)

// TransportKind identifies the transport used by a server.
type TransportKind string

const (
	// TransportStdio uses stdio streams for transport.
	TransportStdio TransportKind = "stdio"
	// TransportStreamableHTTP uses streamable HTTP for transport.
	TransportStreamableHTTP TransportKind = "streamable_http"
)

// StreamableHTTPConfig configures the streamable HTTP transport.
type StreamableHTTPConfig struct {
	Endpoint   string            `json:"endpoint"`
	Headers    map[string]string `json:"headers,omitempty"`
	MaxRetries int               `json:"maxRetries"`
}

// ServerSpec declares how to run and connect to a server.
type ServerSpec struct {
	Name                string                `json:"name"`
	Transport           TransportKind         `json:"transport"`
	Cmd                 []string              `json:"cmd"`
	Env                 map[string]string     `json:"env,omitempty"`
	Cwd                 string                `json:"cwd,omitempty"`
	Tags                []string              `json:"tags,omitempty"`
	IdleSeconds         int                   `json:"idleSeconds"`
	MaxConcurrent       int                   `json:"maxConcurrent"`
	Strategy            InstanceStrategy      `json:"strategy"`
	SessionTTLSeconds   int                   `json:"sessionTTLSeconds,omitempty"`
	Disabled            bool                  `json:"disabled,omitempty"`
	MinReady            int                   `json:"minReady"`
	ActivationMode      ActivationMode        `json:"activationMode"`
	DrainTimeoutSeconds int                   `json:"drainTimeoutSeconds"`
	ProtocolVersion     string                `json:"protocolVersion"`
	ExposeTools         []string              `json:"exposeTools,omitempty"`
	HTTP                *StreamableHTTPConfig `json:"http,omitempty"`
}

// RuntimeConfig defines runtime-level settings for orchestration.
type RuntimeConfig struct {
	RouteTimeoutSeconds        int                 `json:"routeTimeoutSeconds"`
	PingIntervalSeconds        int                 `json:"pingIntervalSeconds"`
	ToolRefreshSeconds         int                 `json:"toolRefreshSeconds"`
	ToolRefreshConcurrency     int                 `json:"toolRefreshConcurrency"`
	ClientCheckSeconds         int                 `json:"clientCheckSeconds"`
	ClientInactiveSeconds      int                 `json:"clientInactiveSeconds"`
	ServerInitRetryBaseSeconds int                 `json:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds  int                 `json:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries       int                 `json:"serverInitMaxRetries"`
	ExposeTools                bool                `json:"exposeTools"`
	ToolNamespaceStrategy      string              `json:"toolNamespaceStrategy"`
	Observability              ObservabilityConfig `json:"observability"`
	RPC                        RPCConfig           `json:"rpc"`
	SubAgent                   SubAgentConfig      `json:"subAgent"`

	// Bootstrap configuration
	BootstrapMode           BootstrapMode  `json:"bootstrapMode"`           // "metadata" or "disabled", default "metadata"
	BootstrapConcurrency    int            `json:"bootstrapConcurrency"`    // concurrent servers during bootstrap, default 3
	BootstrapTimeoutSeconds int            `json:"bootstrapTimeoutSeconds"` // per-server timeout, default 30
	DefaultActivationMode   ActivationMode `json:"defaultActivationMode"`   // "on-demand" or "always-on", default "on-demand"
}

// ObservabilityConfig controls runtime observability endpoints.
type ObservabilityConfig struct {
	ListenAddress string `json:"listenAddress"`
}

// RPCConfig configures the RPC server.
type RPCConfig struct {
	ListenAddress           string `json:"listenAddress"`
	MaxRecvMsgSize          int    `json:"maxRecvMsgSize"`
	MaxSendMsgSize          int    `json:"maxSendMsgSize"`
	KeepaliveTimeSeconds    int    `json:"keepaliveTimeSeconds"`
	KeepaliveTimeoutSeconds int    `json:"keepaliveTimeoutSeconds"`
	SocketMode              string `json:"socketMode"`
	TLS                     RPCTLSConfig
}

// RPCTLSConfig configures TLS for the RPC server.
type RPCTLSConfig struct {
	Enabled    bool   `json:"enabled"`
	CertFile   string `json:"certFile"`
	KeyFile    string `json:"keyFile"`
	CAFile     string `json:"caFile"`
	ClientAuth bool   `json:"clientAuth"`
}

// Catalog groups runtime and server spec configuration.
type Catalog struct {
	Specs   map[string]ServerSpec
	Runtime RuntimeConfig
}

// ServerCapabilities describes the capabilities reported by a server.
type ServerCapabilities struct {
	Tools        *ToolsCapability
	Resources    *ResourcesCapability
	Prompts      *PromptsCapability
	Logging      *LoggingCapability
	Completions  *CompletionsCapability
	Experimental map[string]any
}

// ToolsCapability captures tool-related capabilities.
type ToolsCapability struct {
	ListChanged bool
}

// ResourcesCapability captures resource-related capabilities.
type ResourcesCapability struct {
	Subscribe   bool
	ListChanged bool
}

// PromptsCapability captures prompt-related capabilities.
type PromptsCapability struct {
	ListChanged bool
}

// LoggingCapability captures logging capabilities.
type LoggingCapability struct{}

// CompletionsCapability captures completions capabilities.
type CompletionsCapability struct{}

// InstanceState describes the lifecycle state of an instance.
type InstanceState string

const (
	// InstanceStateStarting indicates the instance is starting.
	InstanceStateStarting InstanceState = "starting"
	// InstanceStateInitializing indicates the instance is initializing.
	InstanceStateInitializing InstanceState = "initializing"
	// InstanceStateHandshaking indicates the instance is handshaking.
	InstanceStateHandshaking InstanceState = "handshaking"
	// InstanceStateReady indicates the instance is ready.
	InstanceStateReady InstanceState = "ready"
	// InstanceStateBusy indicates the instance is busy.
	InstanceStateBusy InstanceState = "busy"
	// InstanceStateDraining indicates the instance is draining.
	InstanceStateDraining InstanceState = "draining"
	// InstanceStateStopped indicates the instance is stopped.
	InstanceStateStopped InstanceState = "stopped"
	// InstanceStateFailed indicates the instance failed.
	InstanceStateFailed InstanceState = "failed"
)

// Instance represents a running server instance.
type Instance struct {
	ID               string
	Spec             ServerSpec
	SpecKey          string
	State            InstanceState
	BusyCount        int
	LastActive       time.Time
	SpawnedAt        time.Time
	HandshakedAt     time.Time
	LastHeartbeatAt  time.Time
	StickyKey        string
	Conn             Conn
	Capabilities     ServerCapabilities
	LastStartCause   *StartCause
	callCount        int64
	errorCount       int64
	totalDurationNs  int64
	lastCallUnixNano int64
}

// InstanceInfo provides a read-only snapshot of instance state for status queries
type InstanceInfo struct {
	ID              string
	State           InstanceState
	BusyCount       int
	LastActive      time.Time
	SpawnedAt       time.Time
	HandshakedAt    time.Time
	LastHeartbeatAt time.Time
	LastStartCause  *StartCause
}

// PoolInfo provides a read-only snapshot of a pool's state for status queries
type PoolInfo struct {
	SpecKey    string
	ServerName string
	MinReady   int
	Instances  []InstanceInfo
	Metrics    PoolMetrics
}

// PoolMetrics aggregates pool-level metrics.
type PoolMetrics struct {
	StartCount    int
	StopCount     int
	TotalCalls    int64
	TotalErrors   int64
	TotalDuration time.Duration
	LastCallAt    time.Time
}

// ServerInitState describes the initialization state of a server.
type ServerInitState string

const (
	// ServerInitPending indicates initialization has not started.
	ServerInitPending ServerInitState = "pending"
	// ServerInitStarting indicates initialization is in progress.
	ServerInitStarting ServerInitState = "starting"
	// ServerInitReady indicates initialization is complete.
	ServerInitReady ServerInitState = "ready"
	// ServerInitDegraded indicates initialization completed with issues.
	ServerInitDegraded ServerInitState = "degraded"
	// ServerInitFailed indicates initialization failed.
	ServerInitFailed ServerInitState = "failed"
	// ServerInitSuspended indicates initialization retries are suspended.
	ServerInitSuspended ServerInitState = "suspended"
)

// ServerInitStatus reports initialization progress for a server.
type ServerInitStatus struct {
	SpecKey     string
	ServerName  string
	MinReady    int
	Ready       int
	Failed      int
	State       ServerInitState
	LastError   string
	RetryCount  int
	NextRetryAt time.Time
	UpdatedAt   time.Time
}

// ErrMethodNotAllowed indicates a method is not permitted by capabilities.
var ErrMethodNotAllowed = errors.New("method not allowed")

// ErrInvalidRequest indicates a request is malformed.
var ErrInvalidRequest = errors.New("invalid request")

// ErrToolNotFound indicates the requested tool does not exist.
var ErrToolNotFound = errors.New("tool not found")

// ErrResourceNotFound indicates the requested resource does not exist.
var ErrResourceNotFound = errors.New("resource not found")

// ErrPromptNotFound indicates the requested prompt does not exist.
var ErrPromptNotFound = errors.New("prompt not found")

// ErrInvalidCursor indicates a pagination cursor is invalid.
var ErrInvalidCursor = errors.New("invalid cursor")

// ErrClientNotRegistered indicates the client is unknown.
var ErrClientNotRegistered = errors.New("client not registered")

// ErrNoReadyInstance indicates no instance is ready to serve the request.
var ErrNoReadyInstance = errors.New("no ready instance")

// ErrUnknownSpecKey indicates the server spec key is unknown.
var ErrUnknownSpecKey = errors.New("unknown spec key")

// ErrInvalidCommand indicates a server command is invalid.
var ErrInvalidCommand = errors.New("invalid command")

// ErrExecutableNotFound indicates the executable could not be found.
var ErrExecutableNotFound = errors.New("executable not found")

// ErrPermissionDenied indicates permission was denied.
var ErrPermissionDenied = errors.New("permission denied")

// ErrUnsupportedProtocol indicates an unsupported protocol version.
var ErrUnsupportedProtocol = errors.New("unsupported protocol version")

// ErrConnectionClosed indicates the connection was closed.
var ErrConnectionClosed = errors.New("connection closed")

// BootstrapMode defines whether mcpd prefetches metadata during startup.
type BootstrapMode string

const (
	// BootstrapModeMetadata starts servers temporarily during bootstrap to fetch metadata.
	BootstrapModeMetadata BootstrapMode = "metadata"

	// BootstrapModeDisabled skips bootstrap metadata collection.
	BootstrapModeDisabled BootstrapMode = "disabled"
)

// ActivationMode defines whether a server stays running without active callers.
type ActivationMode string

const (
	// ActivationOnDemand only runs servers when there are active callers.
	ActivationOnDemand ActivationMode = "on-demand"

	// ActivationAlwaysOn keeps servers running even without active callers.
	ActivationAlwaysOn ActivationMode = "always-on"
)

// BootstrapState represents the current state of the bootstrap process
// BootstrapState represents the current bootstrap state.
type BootstrapState string

const (
	// BootstrapPending indicates bootstrap is pending.
	BootstrapPending BootstrapState = "pending"
	// BootstrapRunning indicates bootstrap is running.
	BootstrapRunning BootstrapState = "running"
	// BootstrapCompleted indicates bootstrap completed successfully.
	BootstrapCompleted BootstrapState = "completed"
	// BootstrapFailed indicates bootstrap failed.
	BootstrapFailed BootstrapState = "failed"
)

// BootstrapProgress provides real-time status of the bootstrap process
type BootstrapProgress struct {
	State     BootstrapState
	Total     int               // Total number of servers to bootstrap
	Completed int               // Successfully bootstrapped servers
	Failed    int               // Failed servers
	Current   string            // Currently bootstrapping server name
	Errors    map[string]string // specKey -> error message for failed servers
}

// Bootstrap configuration defaults
const (
	// DefaultBootstrapMode is the default bootstrap mode.
	DefaultBootstrapMode = BootstrapModeMetadata
	// DefaultBootstrapConcurrency is the default bootstrap concurrency.
	DefaultBootstrapConcurrency = 3
	// DefaultBootstrapTimeoutSeconds is the default bootstrap timeout in seconds.
	DefaultBootstrapTimeoutSeconds = 30
	// DefaultActivationMode is the default activation mode.
	DefaultActivationMode = ActivationOnDemand
)
