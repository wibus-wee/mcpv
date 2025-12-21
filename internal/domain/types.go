package domain

import (
	"context"
	"encoding/json"
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
	ID         string
	Spec       ServerSpec
	State      InstanceState
	BusyCount  int
	LastActive time.Time
	StickyKey  string
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
}

type Router interface {
	Route(ctx context.Context, serverType, routingKey string, payload json.RawMessage) (json.RawMessage, error)
}

type CatalogLoader interface {
	Load(ctx context.Context, path string) (map[string]ServerSpec, error)
}

type HealthProbe interface {
	Ping(ctx context.Context, conn Conn) error
}
