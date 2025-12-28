package ui

import "encoding/json"

// Frontend-friendly types for Wails bindings

// ToolEntry represents a single tool for the frontend
type ToolEntry struct {
	Name       string          `json:"name"`
	ToolJSON   json.RawMessage `json:"toolJson"`
	SpecKey    string          `json:"specKey"`
	ServerName string          `json:"serverName"`
}

// ResourceEntry represents a single resource for the frontend
type ResourceEntry struct {
	URI          string          `json:"uri"`
	ResourceJSON json.RawMessage `json:"resourceJson"`
}

// PromptEntry represents a single prompt for the frontend
type PromptEntry struct {
	Name       string          `json:"name"`
	PromptJSON json.RawMessage `json:"promptJson"`
}

// ResourcePage represents a paginated list of resources
type ResourcePage struct {
	Resources  []ResourceEntry `json:"resources"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

// PromptPage represents a paginated list of prompts
type PromptPage struct {
	Prompts    []PromptEntry `json:"prompts"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

// =============================================================================
// Configuration Management Types
// =============================================================================

// ConfigModeResponse indicates the configuration mode and path
type ConfigModeResponse struct {
	Mode       string `json:"mode"`       // "single" (file) or "directory"
	Path       string `json:"path"`       // Configuration path
	IsWritable bool   `json:"isWritable"` // Whether the config is writable
}

// ProfileSummary provides a brief overview of a profile
type ProfileSummary struct {
	Name        string `json:"name"`
	ServerCount int    `json:"serverCount"`
	IsDefault   bool   `json:"isDefault"`
}

// ProfileDetail contains full profile configuration
type ProfileDetail struct {
	Name    string              `json:"name"`
	Runtime RuntimeConfigDetail `json:"runtime"`
	Servers []ServerSpecDetail  `json:"servers"`
}

// RuntimeConfigDetail contains runtime configuration for frontend
type RuntimeConfigDetail struct {
	RouteTimeoutSeconds   int                       `json:"routeTimeoutSeconds"`
	PingIntervalSeconds   int                       `json:"pingIntervalSeconds"`
	ToolRefreshSeconds    int                       `json:"toolRefreshSeconds"`
	CallerCheckSeconds    int                       `json:"callerCheckSeconds"`
	ExposeTools           bool                      `json:"exposeTools"`
	ToolNamespaceStrategy string                    `json:"toolNamespaceStrategy"`
	Observability         ObservabilityConfigDetail `json:"observability"`
	RPC                   RPCConfigDetail           `json:"rpc"`
}

// ObservabilityConfigDetail for frontend
type ObservabilityConfigDetail struct {
	ListenAddress string `json:"listenAddress"`
}

// RPCConfigDetail for frontend
type RPCConfigDetail struct {
	ListenAddress           string             `json:"listenAddress"`
	MaxRecvMsgSize          int                `json:"maxRecvMsgSize"`
	MaxSendMsgSize          int                `json:"maxSendMsgSize"`
	KeepaliveTimeSeconds    int                `json:"keepaliveTimeSeconds"`
	KeepaliveTimeoutSeconds int                `json:"keepaliveTimeoutSeconds"`
	SocketMode              string             `json:"socketMode"`
	TLS                     RPCTLSConfigDetail `json:"tls"`
}

// RPCTLSConfigDetail for frontend
type RPCTLSConfigDetail struct {
	Enabled    bool   `json:"enabled"`
	CertFile   string `json:"certFile"`
	KeyFile    string `json:"keyFile"`
	CAFile     string `json:"caFile"`
	ClientAuth bool   `json:"clientAuth"`
}

// ServerSpecDetail contains server specification for frontend
type ServerSpecDetail struct {
	Name                string            `json:"name"`
	SpecKey             string            `json:"specKey"`
	Cmd                 []string          `json:"cmd"`
	Env                 map[string]string `json:"env"`
	Cwd                 string            `json:"cwd"`
	IdleSeconds         int               `json:"idleSeconds"`
	MaxConcurrent       int               `json:"maxConcurrent"`
	Sticky              bool              `json:"sticky"`
	Persistent          bool              `json:"persistent"`
	MinReady            int               `json:"minReady"`
	DrainTimeoutSeconds int               `json:"drainTimeoutSeconds"`
	ProtocolVersion     string            `json:"protocolVersion"`
	ExposeTools         []string          `json:"exposeTools"`
}

// =============================================================================
// Runtime Status Types
// =============================================================================

// ServerRuntimeStatus contains the runtime status of a server and its instances
type ServerRuntimeStatus struct {
	SpecKey    string           `json:"specKey"`
	ServerName string           `json:"serverName"`
	Instances  []InstanceStatus `json:"instances"`
	Stats      PoolStats        `json:"stats"`
}

// InstanceStatus represents the status of a single server instance
type InstanceStatus struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	BusyCount  int    `json:"busyCount"`
	LastActive string `json:"lastActive"`
}

// PoolStats contains aggregated statistics for a server pool
type PoolStats struct {
	Total    int `json:"total"`
	Ready    int `json:"ready"`
	Busy     int `json:"busy"`
	Starting int `json:"starting"`
	Draining int `json:"draining"`
	Failed   int `json:"failed"`
}
