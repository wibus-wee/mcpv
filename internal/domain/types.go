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

type ServerSpec struct {
	Name                string            `json:"name"`
	Cmd                 []string          `json:"cmd"`
	Env                 map[string]string `json:"env,omitempty"`
	Cwd                 string            `json:"cwd,omitempty"`
	IdleSeconds         int               `json:"idleSeconds"`
	MaxConcurrent       int               `json:"maxConcurrent"`
	Strategy            InstanceStrategy  `json:"strategy"`
	SessionTTLSeconds   int               `json:"sessionTTLSeconds,omitempty"`
	Disabled            bool              `json:"disabled,omitempty"`
	MinReady            int               `json:"minReady"`
	DrainTimeoutSeconds int               `json:"drainTimeoutSeconds"`
	ProtocolVersion     string            `json:"protocolVersion"`
	ExposeTools         []string          `json:"exposeTools,omitempty"`
}

type RuntimeConfig struct {
	RouteTimeoutSeconds        int                 `json:"routeTimeoutSeconds"`
	PingIntervalSeconds        int                 `json:"pingIntervalSeconds"`
	ToolRefreshSeconds         int                 `json:"toolRefreshSeconds"`
	ToolRefreshConcurrency     int                 `json:"toolRefreshConcurrency"`
	CallerCheckSeconds         int                 `json:"callerCheckSeconds"`
	CallerInactiveSeconds      int                 `json:"callerInactiveSeconds"`
	ServerInitRetryBaseSeconds int                 `json:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds  int                 `json:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries       int                 `json:"serverInitMaxRetries"`
	ExposeTools                bool                `json:"exposeTools"`
	ToolNamespaceStrategy      string              `json:"toolNamespaceStrategy"`
	Observability              ObservabilityConfig `json:"observability"`
	RPC                        RPCConfig           `json:"rpc"`
	SubAgent                   SubAgentConfig      `json:"subAgent"`

	// Bootstrap configuration
	StartupStrategy         string `json:"startupStrategy"`         // "lazy" or "eager", default "lazy"
	BootstrapConcurrency    int    `json:"bootstrapConcurrency"`    // concurrent servers during bootstrap, default 3
	BootstrapTimeoutSeconds int    `json:"bootstrapTimeoutSeconds"` // per-server timeout, default 30
}

type ObservabilityConfig struct {
	ListenAddress string `json:"listenAddress"`
}

type RPCConfig struct {
	ListenAddress           string `json:"listenAddress"`
	MaxRecvMsgSize          int    `json:"maxRecvMsgSize"`
	MaxSendMsgSize          int    `json:"maxSendMsgSize"`
	KeepaliveTimeSeconds    int    `json:"keepaliveTimeSeconds"`
	KeepaliveTimeoutSeconds int    `json:"keepaliveTimeoutSeconds"`
	SocketMode              string `json:"socketMode"`
	TLS                     RPCTLSConfig
}

type RPCTLSConfig struct {
	Enabled    bool   `json:"enabled"`
	CertFile   string `json:"certFile"`
	KeyFile    string `json:"keyFile"`
	CAFile     string `json:"caFile"`
	ClientAuth bool   `json:"clientAuth"`
}

type Catalog struct {
	Specs    map[string]ServerSpec
	Runtime  RuntimeConfig
	SubAgent ProfileSubAgentConfig // Per-profile SubAgent settings (enabled/disabled)
}

type ServerCapabilities struct {
	Tools        *ToolsCapability
	Resources    *ResourcesCapability
	Prompts      *PromptsCapability
	Logging      *LoggingCapability
	Completions  *CompletionsCapability
	Experimental map[string]any
}

type ToolsCapability struct {
	ListChanged bool
}

type ResourcesCapability struct {
	Subscribe   bool
	ListChanged bool
}

type PromptsCapability struct {
	ListChanged bool
}

type LoggingCapability struct{}

type CompletionsCapability struct{}

type InstanceState string

const (
	InstanceStateStarting     InstanceState = "starting"
	InstanceStateInitializing InstanceState = "initializing"
	InstanceStateHandshaking  InstanceState = "handshaking"
	InstanceStateReady        InstanceState = "ready"
	InstanceStateBusy         InstanceState = "busy"
	InstanceStateDraining     InstanceState = "draining"
	InstanceStateStopped      InstanceState = "stopped"
	InstanceStateFailed       InstanceState = "failed"
)

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
}

// PoolInfo provides a read-only snapshot of a pool's state for status queries
type PoolInfo struct {
	SpecKey    string
	ServerName string
	MinReady   int
	Instances  []InstanceInfo
	Metrics    PoolMetrics
}

type PoolMetrics struct {
	StartCount    int
	StopCount     int
	TotalCalls    int64
	TotalErrors   int64
	TotalDuration time.Duration
	LastCallAt    time.Time
}

type ServerInitState string

const (
	ServerInitPending   ServerInitState = "pending"
	ServerInitStarting  ServerInitState = "starting"
	ServerInitReady     ServerInitState = "ready"
	ServerInitDegraded  ServerInitState = "degraded"
	ServerInitFailed    ServerInitState = "failed"
	ServerInitSuspended ServerInitState = "suspended"
)

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

var ErrMethodNotAllowed = errors.New("method not allowed")
var ErrInvalidRequest = errors.New("invalid request")
var ErrToolNotFound = errors.New("tool not found")
var ErrResourceNotFound = errors.New("resource not found")
var ErrPromptNotFound = errors.New("prompt not found")
var ErrInvalidCursor = errors.New("invalid cursor")
var ErrCallerNotRegistered = errors.New("caller not registered")
var ErrNoReadyInstance = errors.New("no ready instance")
var ErrUnknownSpecKey = errors.New("unknown spec key")
var ErrInvalidCommand = errors.New("invalid command")
var ErrExecutableNotFound = errors.New("executable not found")
var ErrPermissionDenied = errors.New("permission denied")
var ErrUnsupportedProtocol = errors.New("unsupported protocol version")
var ErrConnectionClosed = errors.New("connection closed")

// StartupStrategy defines how mcpd initializes MCP servers at startup
type StartupStrategy string

const (
	// StartupStrategyLazy starts servers temporarily during bootstrap to fetch metadata,
	// then shuts them down. Servers are started on-demand when callers need them.
	StartupStrategyLazy StartupStrategy = "lazy"

	// StartupStrategyEager starts servers during bootstrap and keeps them running
	// for immediate caller access with zero latency.
	StartupStrategyEager StartupStrategy = "eager"
)

// BootstrapState represents the current state of the bootstrap process
type BootstrapState string

const (
	BootstrapPending   BootstrapState = "pending"
	BootstrapRunning   BootstrapState = "running"
	BootstrapCompleted BootstrapState = "completed"
	BootstrapFailed    BootstrapState = "failed"
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
	DefaultStartupStrategy         = "lazy"
	DefaultBootstrapConcurrency    = 3
	DefaultBootstrapTimeoutSeconds = 30
)
