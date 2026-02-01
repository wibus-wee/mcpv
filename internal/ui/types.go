package ui

import "encoding/json"

// Frontend-friendly types for Wails bindings

// ToolEntry represents a single tool for the frontend.
type ToolEntry struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ToolJSON    json.RawMessage `json:"toolJson"`
	SpecKey     string          `json:"specKey"`
	ServerName  string          `json:"serverName"`
	Source      string          `json:"source"`
	CachedAt    string          `json:"cachedAt,omitempty"`
}

// ResourceEntry represents a single resource for the frontend.
type ResourceEntry struct {
	URI          string          `json:"uri"`
	ResourceJSON json.RawMessage `json:"resourceJson"`
}

// PromptEntry represents a single prompt for the frontend.
type PromptEntry struct {
	Name       string          `json:"name"`
	PromptJSON json.RawMessage `json:"promptJson"`
}

// ResourcePage represents a paginated list of resources.
type ResourcePage struct {
	Resources  []ResourceEntry `json:"resources"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

// PromptPage represents a paginated list of prompts.
type PromptPage struct {
	Prompts    []PromptEntry `json:"prompts"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

// =============================================================================
// Configuration Management Types
// =============================================================================

// ConfigModeResponse indicates the configuration mode and path.
type ConfigModeResponse struct {
	Mode       string `json:"mode"`       // "file" or "unknown"
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

// RuntimeConfigDetail contains runtime configuration for frontend.
type RuntimeConfigDetail struct {
	RouteTimeoutSeconds        int                       `json:"routeTimeoutSeconds"`
	PingIntervalSeconds        int                       `json:"pingIntervalSeconds"`
	ToolRefreshSeconds         int                       `json:"toolRefreshSeconds"`
	ToolRefreshConcurrency     int                       `json:"toolRefreshConcurrency"`
	ClientCheckSeconds         int                       `json:"clientCheckSeconds"`
	ClientInactiveSeconds      int                       `json:"clientInactiveSeconds"`
	ServerInitRetryBaseSeconds int                       `json:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds  int                       `json:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries       int                       `json:"serverInitMaxRetries"`
	ReloadMode                 string                    `json:"reloadMode"`
	BootstrapMode              string                    `json:"bootstrapMode"`
	BootstrapConcurrency       int                       `json:"bootstrapConcurrency"`
	BootstrapTimeoutSeconds    int                       `json:"bootstrapTimeoutSeconds"`
	DefaultActivationMode      string                    `json:"defaultActivationMode"`
	ExposeTools                bool                      `json:"exposeTools"`
	ToolNamespaceStrategy      string                    `json:"toolNamespaceStrategy"`
	Observability              ObservabilityConfigDetail `json:"observability"`
	RPC                        RPCConfigDetail           `json:"rpc"`
}

// ObservabilityConfigDetail for frontend.
type ObservabilityConfigDetail struct {
	ListenAddress string `json:"listenAddress"`
}

// RPCConfigDetail for frontend.
type RPCConfigDetail struct {
	ListenAddress           string             `json:"listenAddress"`
	MaxRecvMsgSize          int                `json:"maxRecvMsgSize"`
	MaxSendMsgSize          int                `json:"maxSendMsgSize"`
	KeepaliveTimeSeconds    int                `json:"keepaliveTimeSeconds"`
	KeepaliveTimeoutSeconds int                `json:"keepaliveTimeoutSeconds"`
	SocketMode              string             `json:"socketMode"`
	TLS                     RPCTLSConfigDetail `json:"tls"`
}

// RPCTLSConfigDetail for frontend.
type RPCTLSConfigDetail struct {
	Enabled    bool   `json:"enabled"`
	CertFile   string `json:"certFile"`
	KeyFile    string `json:"keyFile"`
	CAFile     string `json:"caFile"`
	ClientAuth bool   `json:"clientAuth"`
}

// ServerSpecDetail contains server specification for frontend.
type ServerSummary struct {
	Name      string   `json:"name"`
	SpecKey   string   `json:"specKey"`
	Transport string   `json:"transport"`
	Tags      []string `json:"tags,omitempty"`
	Disabled  bool     `json:"disabled"`
}

// ServerDetail contains full server specification for frontend.
type ServerDetail = ServerSpecDetail

// ServerGroup aggregates server configuration and tool metadata for the frontend.
type ServerGroup struct {
	ID          string        `json:"id"`
	SpecKey     string        `json:"specKey"`
	ServerName  string        `json:"serverName"`
	Tools       []ToolEntry   `json:"tools"`
	Tags        []string      `json:"tags"`
	HasToolData bool          `json:"hasToolData"`
	SpecDetail  *ServerDetail `json:"specDetail,omitempty"`
}

// ServerSpecDetail contains server specification for frontend.
type ServerSpecDetail struct {
	Name                string                      `json:"name"`
	SpecKey             string                      `json:"specKey"`
	Transport           string                      `json:"transport"`
	Cmd                 []string                    `json:"cmd"`
	Env                 map[string]string           `json:"env"`
	Cwd                 string                      `json:"cwd"`
	Tags                []string                    `json:"tags,omitempty"`
	IdleSeconds         int                         `json:"idleSeconds"`
	MaxConcurrent       int                         `json:"maxConcurrent"`
	Strategy            string                      `json:"strategy"`
	SessionTTLSeconds   int                         `json:"sessionTTLSeconds"`
	Disabled            bool                        `json:"disabled"`
	MinReady            int                         `json:"minReady"`
	ActivationMode      string                      `json:"activationMode"`
	DrainTimeoutSeconds int                         `json:"drainTimeoutSeconds"`
	ProtocolVersion     string                      `json:"protocolVersion"`
	ExposeTools         []string                    `json:"exposeTools"`
	HTTP                *StreamableHTTPConfigDetail `json:"http,omitempty"`
}

// StreamableHTTPConfigDetail contains streamable HTTP configuration for frontend.
type StreamableHTTPConfigDetail struct {
	Endpoint   string            `json:"endpoint"`
	Headers    map[string]string `json:"headers,omitempty"`
	MaxRetries int               `json:"maxRetries"`
}

// ImportServerSpec represents a server spec from MCP JSON import.
type ImportServerSpec struct {
	Name            string                      `json:"name"`
	Transport       string                      `json:"transport,omitempty"`
	Cmd             []string                    `json:"cmd,omitempty"`
	Env             map[string]string           `json:"env,omitempty"`
	Cwd             string                      `json:"cwd,omitempty"`
	Tags            []string                    `json:"tags,omitempty"`
	ProtocolVersion string                      `json:"protocolVersion,omitempty"`
	HTTP            *StreamableHTTPConfigDetail `json:"http,omitempty"`
}

// ImportMcpServersRequest is the payload for importing MCP servers into the config file.
type ImportMcpServersRequest struct {
	Servers []ImportServerSpec `json:"servers"`
}

// UpdateServerStateRequest updates the disabled state for a server.
type UpdateServerStateRequest struct {
	Server   string `json:"server"`
	Disabled bool   `json:"disabled"`
}

// DeleteServerRequest removes a server.
type DeleteServerRequest struct {
	Server string `json:"server"`
}

// CreateServerRequest creates a server configuration entry.
type CreateServerRequest struct {
	Spec ServerSpecDetail `json:"spec"`
}

// UpdateServerRequest updates an existing server configuration entry.
type UpdateServerRequest struct {
	Spec ServerSpecDetail `json:"spec"`
}

// UpdateRuntimeConfigRequest updates runtime.yaml configuration.
type UpdateRuntimeConfigRequest struct {
	RouteTimeoutSeconds        int    `json:"routeTimeoutSeconds"`
	PingIntervalSeconds        int    `json:"pingIntervalSeconds"`
	ToolRefreshSeconds         int    `json:"toolRefreshSeconds"`
	ToolRefreshConcurrency     int    `json:"toolRefreshConcurrency"`
	ClientCheckSeconds         int    `json:"clientCheckSeconds"`
	ClientInactiveSeconds      int    `json:"clientInactiveSeconds"`
	ServerInitRetryBaseSeconds int    `json:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds  int    `json:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries       int    `json:"serverInitMaxRetries"`
	ReloadMode                 string `json:"reloadMode"`
	BootstrapMode              string `json:"bootstrapMode"`
	BootstrapConcurrency       int    `json:"bootstrapConcurrency"`
	BootstrapTimeoutSeconds    int    `json:"bootstrapTimeoutSeconds"`
	DefaultActivationMode      string `json:"defaultActivationMode"`
	ExposeTools                bool   `json:"exposeTools"`
	ToolNamespaceStrategy      string `json:"toolNamespaceStrategy"`
}

// =============================================================================
// Client Status Types
// =============================================================================

type ActiveClient struct {
	Client        string   `json:"client"`
	PID           int      `json:"pid"`
	Tags          []string `json:"tags"`
	Server        string   `json:"server,omitempty"`
	LastHeartbeat string   `json:"lastHeartbeat"`
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
	Client    string            `json:"client,omitempty"`
	ToolName  string            `json:"toolName,omitempty"`
	Policy    *StartCausePolicy `json:"policy,omitempty"`
	Timestamp string            `json:"timestamp"`
}

// ServerRuntimeStatus contains the runtime status of a server and its instances.
type ServerRuntimeStatus struct {
	SpecKey    string           `json:"specKey"`
	ServerName string           `json:"serverName"`
	Instances  []InstanceStatus `json:"instances"`
	Stats      PoolStats        `json:"stats"`
	Metrics    PoolMetrics      `json:"metrics"`
}

// InstanceStatus represents the status of a single server instance.
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

// PoolStats contains aggregated statistics for a server pool.
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

// SubAgentConfigDetail contains the runtime-level SubAgent LLM provider config.
type SubAgentConfigDetail struct {
	EnabledTags        []string `json:"enabledTags,omitempty"`
	Model              string   `json:"model"`
	Provider           string   `json:"provider"`
	APIKeyEnvVar       string   `json:"apiKeyEnvVar"`
	BaseURL            string   `json:"baseURL"`
	MaxToolsPerRequest int      `json:"maxToolsPerRequest"`
	FilterPrompt       string   `json:"filterPrompt"`
}

// UpdateSubAgentConfigRequest updates the runtime-level SubAgent config.
type UpdateSubAgentConfigRequest struct {
	EnabledTags        []string `json:"enabledTags,omitempty"`
	Model              string   `json:"model"`
	Provider           string   `json:"provider"`
	APIKey             *string  `json:"apiKey,omitempty"`
	APIKeyEnvVar       string   `json:"apiKeyEnvVar"`
	BaseURL            string   `json:"baseURL"`
	MaxToolsPerRequest int      `json:"maxToolsPerRequest"`
	FilterPrompt       string   `json:"filterPrompt"`
}

type ProxyFetchRequest struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
	TimeoutMs int               `json:"timeoutMs"`
}

type ProxyFetchResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// =============================================================================
// Plugin Management Types
// =============================================================================

// PluginListEntry represents a single plugin for the frontend.
type PluginListEntry struct {
	Name               string            `json:"name"`
	Category           string            `json:"category"`
	Flows              []string          `json:"flows"`
	Required           bool              `json:"required"`
	Enabled            bool              `json:"enabled"`
	Status             string            `json:"status"`                // "running", "stopped", "error"
	StatusError        string            `json:"statusError,omitempty"` // Error message if status is "error"
	CommitHash         string            `json:"commitHash,omitempty"`
	TimeoutMs          int               `json:"timeoutMs"`
	HandshakeTimeoutMs int               `json:"handshakeTimeoutMs"`
	Cmd                []string          `json:"cmd"`
	Env                map[string]string `json:"env,omitempty"`
	Cwd                string            `json:"cwd,omitempty"`
	ConfigJSON         string            `json:"configJson,omitempty"` // JSON string for frontend editing
	LatestMetrics      PluginMetrics     `json:"latestMetrics"`
}

// PluginMetrics represents aggregated metrics for a plugin.
type PluginMetrics struct {
	CallCount      int64   `json:"callCount"`
	RejectionCount int64   `json:"rejectionCount"`
	AvgLatencyMs   float64 `json:"avgLatencyMs"`
}

// TogglePluginRequest is the request to enable/disable a plugin.
type TogglePluginRequest struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}
