package domain

import (
	"context"
	"encoding/json"
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
	MinReady            int               `json:"minReady"`
	DrainTimeoutSeconds int               `json:"drainTimeoutSeconds"`
	ProtocolVersion     string            `json:"protocolVersion"`
	ExposeTools         []string          `json:"exposeTools,omitempty"`
}

type RuntimeConfig struct {
	RouteTimeoutSeconds    int                 `json:"routeTimeoutSeconds"`
	PingIntervalSeconds    int                 `json:"pingIntervalSeconds"`
	ToolRefreshSeconds     int                 `json:"toolRefreshSeconds"`
	ToolRefreshConcurrency int                 `json:"toolRefreshConcurrency"`
	CallerCheckSeconds     int                 `json:"callerCheckSeconds"`
	ExposeTools            bool                `json:"exposeTools"`
	ToolNamespaceStrategy  string              `json:"toolNamespaceStrategy"`
	Observability          ObservabilityConfig `json:"observability"`
	RPC                    RPCConfig           `json:"rpc"`
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
	Specs   map[string]ServerSpec
	Runtime RuntimeConfig
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
	ServerInitPending  ServerInitState = "pending"
	ServerInitStarting ServerInitState = "starting"
	ServerInitReady    ServerInitState = "ready"
	ServerInitDegraded ServerInitState = "degraded"
	ServerInitFailed   ServerInitState = "failed"
)

type ServerInitStatus struct {
	SpecKey    string
	ServerName string
	MinReady   int
	Ready      int
	Failed     int
	State      ServerInitState
	LastError  string
	UpdatedAt  time.Time
}

type Conn interface {
	Send(ctx context.Context, msg json.RawMessage) error
	Recv(ctx context.Context) (json.RawMessage, error)
	Close() error
}

type StopFn func(ctx context.Context) error

type Transport interface {
	Start(ctx context.Context, spec ServerSpec) (Conn, StopFn, error)
}

type Lifecycle interface {
	StartInstance(ctx context.Context, spec ServerSpec) (*Instance, error)
	StopInstance(ctx context.Context, instance *Instance, reason string) error
}

type Scheduler interface {
	Acquire(ctx context.Context, specKey, routingKey string) (*Instance, error)
	AcquireReady(ctx context.Context, specKey, routingKey string) (*Instance, error)
	Release(ctx context.Context, instance *Instance) error
	SetDesiredMinReady(ctx context.Context, specKey string, minReady int) error
	StopSpec(ctx context.Context, specKey, reason string) error
	StartIdleManager(interval time.Duration)
	StopIdleManager()
	StartPingManager(interval time.Duration)
	StopPingManager()
	StopAll(ctx context.Context)
	GetPoolStatus(ctx context.Context) ([]PoolInfo, error)
}

type Router interface {
	Route(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage) (json.RawMessage, error)
	RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, opts RouteOptions) (json.RawMessage, error)
}

type RouteOptions struct {
	AllowStart bool
}

type CatalogLoader interface {
	Load(ctx context.Context, path string) (Catalog, error)
}

type HealthProbe interface {
	Ping(ctx context.Context, conn Conn) error
}

var ErrMethodNotAllowed = errors.New("method not allowed")
var ErrInvalidRequest = errors.New("invalid request")
var ErrToolNotFound = errors.New("tool not found")
var ErrResourceNotFound = errors.New("resource not found")
var ErrPromptNotFound = errors.New("prompt not found")
var ErrInvalidCursor = errors.New("invalid cursor")
var ErrCallerNotRegistered = errors.New("caller not registered")
var ErrNoReadyInstance = errors.New("no ready instance")
