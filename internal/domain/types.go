package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type ServerSpec struct {
	Name            string            `json:"name"`
	Cmd             []string          `json:"cmd"`
	Env             map[string]string `json:"env,omitempty"`
	Cwd             string            `json:"cwd,omitempty"`
	IdleSeconds     int               `json:"idleSeconds"`
	MaxConcurrent   int               `json:"maxConcurrent"`
	Sticky          bool              `json:"sticky"`
	Persistent      bool              `json:"persistent"`
	MinReady        int               `json:"minReady"`
	ProtocolVersion string            `json:"protocolVersion"`
	ExposeTools     []string          `json:"exposeTools,omitempty"`
}

type RuntimeConfig struct {
	RouteTimeoutSeconds   int                 `json:"routeTimeoutSeconds"`
	PingIntervalSeconds   int                 `json:"pingIntervalSeconds"`
	ToolRefreshSeconds    int                 `json:"toolRefreshSeconds"`
	ExposeTools           bool                `json:"exposeTools"`
	ToolNamespaceStrategy string              `json:"toolNamespaceStrategy"`
	Observability         ObservabilityConfig `json:"observability"`
	RPC                   RPCConfig           `json:"rpc"`
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
	Tools        bool
	Resources    bool
	Prompts      bool
	Logging      bool
	Completions  bool
	Experimental bool
}

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
	State        InstanceState
	BusyCount    int
	LastActive   time.Time
	StickyKey    string
	Conn         Conn
	Capabilities ServerCapabilities
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
	Acquire(ctx context.Context, serverType, routingKey string) (*Instance, error)
	Release(ctx context.Context, instance *Instance) error
	StartIdleManager(interval time.Duration)
	StopIdleManager()
	StartPingManager(interval time.Duration)
	StopPingManager()
	StopAll(ctx context.Context)
}

type Router interface {
	Route(ctx context.Context, serverType, routingKey string, payload json.RawMessage) (json.RawMessage, error)
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
