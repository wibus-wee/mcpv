package domain

import (
	"errors"
	"time"
)

type ServerSpec struct {
	Name                string            `json:"name"`
	Cmd                 []string          `json:"cmd"`
	Env                 map[string]string `json:"env,omitempty"`
	Cwd                 string            `json:"cwd,omitempty"`
	IdleSeconds         int               `json:"idleSeconds"`
	MaxConcurrent       int               `json:"maxConcurrent"`
	Sticky              bool              `json:"sticky"`
	Persistent          bool              `json:"persistent"`
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
	InstanceStateStarting InstanceState = "starting"
	InstanceStateReady    InstanceState = "ready"
	InstanceStateBusy     InstanceState = "busy"
	InstanceStateDraining InstanceState = "draining"
	InstanceStateStopped  InstanceState = "stopped"
	InstanceStateFailed   InstanceState = "failed"
)

type Instance struct {
	ID           string
	Spec         ServerSpec
	SpecKey      string
	State        InstanceState
	BusyCount    int
	LastActive   time.Time
	StickyKey    string
	Conn         Conn
	Capabilities ServerCapabilities
}

// InstanceInfo provides a read-only snapshot of instance state for status queries
type InstanceInfo struct {
	ID         string
	State      InstanceState
	BusyCount  int
	LastActive time.Time
}

// PoolInfo provides a read-only snapshot of a pool's state for status queries
type PoolInfo struct {
	SpecKey    string
	ServerName string
	MinReady   int
	Instances  []InstanceInfo
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
