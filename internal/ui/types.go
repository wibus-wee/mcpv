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
	Mode       string `json:"mode"`       // "directory" or "unknown"
	Path       string `json:"path"`       // Configuration path
	IsWritable bool   `json:"isWritable"` // Whether the config is writable
}

// CoreStateResponse is the core lifecycle status for the frontend.
type CoreStateResponse struct {
	State  string `json:"state"`
	Uptime int64  `json:"uptime"`
	Error  string `json:"error,omitempty"`
}

// InfoResponse exposes control plane metadata to the frontend.
type InfoResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Build   string `json:"build"`
}

// BootstrapProgressResponse represents bootstrap progress for the frontend.
type BootstrapProgressResponse struct {
	State     string            `json:"state"`
	Total     int               `json:"total"`
	Completed int               `json:"completed"`
	Failed    int               `json:"failed"`
	Current   string            `json:"current"`
	Errors    map[string]string `json:"errors,omitempty"`
}

// DebugSnapshotResponse is the metadata returned after exporting a debug snapshot.
type DebugSnapshotResponse struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	GeneratedAt string `json:"generatedAt"`
}

// StartCoreOptions controls how the core is started in Wails.
type StartCoreOptions struct {
	Mode           string `json:"mode,omitempty"`
	ConfigPath     string `json:"configPath,omitempty"`
	MetricsEnabled *bool  `json:"metricsEnabled,omitempty"`
	HealthzEnabled *bool  `json:"healthzEnabled,omitempty"`
}

// ProfileSummary provides a brief overview of a profile
type ProfileSummary struct {
	Name        string `json:"name"`
	ServerCount int    `json:"serverCount"`
	IsDefault   bool   `json:"isDefault"`
}

// ProfileDetail contains full profile configuration
type ProfileDetail struct {
	Name     string                      `json:"name"`
	Runtime  RuntimeConfigDetail         `json:"runtime"`
	Servers  []ServerSpecDetail          `json:"servers"`
	SubAgent ProfileSubAgentConfigDetail `json:"subAgent"`
}

// RuntimeConfigDetail contains runtime configuration for frontend
type RuntimeConfigDetail struct {
	RouteTimeoutSeconds        int                       `json:"routeTimeoutSeconds"`
	PingIntervalSeconds        int                       `json:"pingIntervalSeconds"`
	ToolRefreshSeconds         int                       `json:"toolRefreshSeconds"`
	ToolRefreshConcurrency     int                       `json:"toolRefreshConcurrency"`
	CallerCheckSeconds         int                       `json:"callerCheckSeconds"`
	CallerInactiveSeconds      int                       `json:"callerInactiveSeconds"`
	ServerInitRetryBaseSeconds int                       `json:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds  int                       `json:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries       int                       `json:"serverInitMaxRetries"`
	BootstrapMode              string                    `json:"bootstrapMode"`
	BootstrapConcurrency       int                       `json:"bootstrapConcurrency"`
	BootstrapTimeoutSeconds    int                       `json:"bootstrapTimeoutSeconds"`
	DefaultActivationMode      string                    `json:"defaultActivationMode"`
	ExposeTools                bool                      `json:"exposeTools"`
	ToolNamespaceStrategy      string                    `json:"toolNamespaceStrategy"`
	Observability              ObservabilityConfigDetail `json:"observability"`
	RPC                        RPCConfigDetail           `json:"rpc"`
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
	Strategy            string            `json:"strategy"`
	SessionTTLSeconds   int               `json:"sessionTTLSeconds"`
	Disabled            bool              `json:"disabled"`
	MinReady            int               `json:"minReady"`
	ActivationMode      string            `json:"activationMode"`
	DrainTimeoutSeconds int               `json:"drainTimeoutSeconds"`
	ProtocolVersion     string            `json:"protocolVersion"`
	ExposeTools         []string          `json:"exposeTools"`
}

// ImportServerSpec represents a server spec from MCP JSON import.
type ImportServerSpec struct {
	Name string            `json:"name"`
	Cmd  []string          `json:"cmd"`
	Env  map[string]string `json:"env,omitempty"`
	Cwd  string            `json:"cwd,omitempty"`
}

// ImportMcpServersRequest is the payload for importing MCP servers into profiles.
type ImportMcpServersRequest struct {
	Profiles []string           `json:"profiles"`
	Servers  []ImportServerSpec `json:"servers"`
}

// UpdateServerStateRequest updates the disabled state for a server in a profile.
type UpdateServerStateRequest struct {
	Profile  string `json:"profile"`
	Server   string `json:"server"`
	Disabled bool   `json:"disabled"`
}

// DeleteServerRequest removes a server from a profile.
type DeleteServerRequest struct {
	Profile string `json:"profile"`
	Server  string `json:"server"`
}

// CreateProfileRequest creates a new profile file.
type CreateProfileRequest struct {
	Name string `json:"name"`
}

// DeleteProfileRequest removes a profile file.
type DeleteProfileRequest struct {
	Name string `json:"name"`
}

// UpdateRuntimeConfigRequest updates runtime.yaml configuration.
type UpdateRuntimeConfigRequest struct {
	RouteTimeoutSeconds        int    `json:"routeTimeoutSeconds"`
	PingIntervalSeconds        int    `json:"pingIntervalSeconds"`
	ToolRefreshSeconds         int    `json:"toolRefreshSeconds"`
	ToolRefreshConcurrency     int    `json:"toolRefreshConcurrency"`
	CallerCheckSeconds         int    `json:"callerCheckSeconds"`
	CallerInactiveSeconds      int    `json:"callerInactiveSeconds"`
	ServerInitRetryBaseSeconds int    `json:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds  int    `json:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries       int    `json:"serverInitMaxRetries"`
	BootstrapMode              string `json:"bootstrapMode"`
	BootstrapConcurrency       int    `json:"bootstrapConcurrency"`
	BootstrapTimeoutSeconds    int    `json:"bootstrapTimeoutSeconds"`
	DefaultActivationMode      string `json:"defaultActivationMode"`
	ExposeTools                bool   `json:"exposeTools"`
	ToolNamespaceStrategy      string `json:"toolNamespaceStrategy"`
}

// UpdateCallerMappingRequest updates a caller to profile mapping.
type UpdateCallerMappingRequest struct {
	Caller  string `json:"caller"`
	Profile string `json:"profile"`
}

// =============================================================================
// Caller Status Types
// =============================================================================

type ActiveCaller struct {
	Caller        string `json:"caller"`
	PID           int    `json:"pid"`
	Profile       string `json:"profile"`
	LastHeartbeat string `json:"lastHeartbeat"`
}

// =============================================================================
// Initialization Status Types
// =============================================================================

type ServerInitStatus struct {
	SpecKey     string `json:"specKey"`
	ServerName  string `json:"serverName"`
	MinReady    int    `json:"minReady"`
	Ready       int    `json:"ready"`
	Failed      int    `json:"failed"`
	State       string `json:"state"`
	LastError   string `json:"lastError,omitempty"`
	RetryCount  int    `json:"retryCount"`
	NextRetryAt string `json:"nextRetryAt,omitempty"`
	UpdatedAt   string `json:"updatedAt"`
}

type RetryServerInitRequest struct {
	SpecKey string `json:"specKey"`
}

// =============================================================================
// Runtime Status Types
// =============================================================================

type StartCausePolicy struct {
	ActivationMode string `json:"activationMode"`
	MinReady       int    `json:"minReady"`
}

type StartCause struct {
	Reason    string            `json:"reason"`
	Caller    string            `json:"caller,omitempty"`
	ToolName  string            `json:"toolName,omitempty"`
	Profile   string            `json:"profile,omitempty"`
	Policy    *StartCausePolicy `json:"policy,omitempty"`
	Timestamp string            `json:"timestamp"`
}

// ServerRuntimeStatus contains the runtime status of a server and its instances
type ServerRuntimeStatus struct {
	SpecKey    string           `json:"specKey"`
	ServerName string           `json:"serverName"`
	Instances  []InstanceStatus `json:"instances"`
	Stats      PoolStats        `json:"stats"`
	Metrics    PoolMetrics      `json:"metrics"`
}

// InstanceStatus represents the status of a single server instance
type InstanceStatus struct {
	ID              string      `json:"id"`
	State           string      `json:"state"`
	BusyCount       int         `json:"busyCount"`
	LastActive      string      `json:"lastActive"`
	SpawnedAt       string      `json:"spawnedAt"`
	HandshakedAt    string      `json:"handshakedAt"`
	LastHeartbeatAt string      `json:"lastHeartbeatAt"`
	LastStartCause  *StartCause `json:"lastStartCause,omitempty"`
}

// PoolStats contains aggregated statistics for a server pool
type PoolStats struct {
	Total        int `json:"total"`
	Ready        int `json:"ready"`
	Busy         int `json:"busy"`
	Starting     int `json:"starting"`
	Initializing int `json:"initializing"`
	Handshaking  int `json:"handshaking"`
	Draining     int `json:"draining"`
	Failed       int `json:"failed"`
}

type PoolMetrics struct {
	StartCount      int    `json:"startCount"`
	StopCount       int    `json:"stopCount"`
	TotalCalls      int64  `json:"totalCalls"`
	TotalErrors     int64  `json:"totalErrors"`
	TotalDurationMs int64  `json:"totalDurationMs"`
	LastCallAt      string `json:"lastCallAt"`
}

// =============================================================================
// SubAgent Configuration Types
// =============================================================================

// SubAgentConfigDetail contains the runtime-level SubAgent LLM provider config
type SubAgentConfigDetail struct {
	Model              string `json:"model"`
	Provider           string `json:"provider"`
	APIKeyEnvVar       string `json:"apiKeyEnvVar"`
	BaseURL            string `json:"baseURL"`
	MaxToolsPerRequest int    `json:"maxToolsPerRequest"`
	FilterPrompt       string `json:"filterPrompt"`
}

// ProfileSubAgentConfigDetail contains the per-profile SubAgent settings
type ProfileSubAgentConfigDetail struct {
	Enabled bool `json:"enabled"`
}

// UpdateSubAgentConfigRequest updates the runtime-level SubAgent config
type UpdateSubAgentConfigRequest struct {
	Model              string `json:"model"`
	Provider           string `json:"provider"`
	APIKeyEnvVar       string `json:"apiKeyEnvVar"`
	MaxToolsPerRequest int    `json:"maxToolsPerRequest"`
	FilterPrompt       string `json:"filterPrompt"`
}

// UpdateProfileSubAgentRequest updates the per-profile SubAgent enabled state
type UpdateProfileSubAgentRequest struct {
	Profile string `json:"profile"`
	Enabled bool   `json:"enabled"`
}
