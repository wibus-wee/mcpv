package rpc

type ConfigMode struct {
	Mode       string `json:"mode"`
	Path       string `json:"path"`
	IsWritable bool   `json:"isWritable"`
}

type RuntimeConfigPayload struct {
	RouteTimeoutSeconds        int                        `json:"routeTimeoutSeconds"`
	PingIntervalSeconds        int                        `json:"pingIntervalSeconds"`
	ToolRefreshSeconds         int                        `json:"toolRefreshSeconds"`
	ToolRefreshConcurrency     int                        `json:"toolRefreshConcurrency"`
	ClientCheckSeconds         int                        `json:"clientCheckSeconds"`
	ClientInactiveSeconds      int                        `json:"clientInactiveSeconds"`
	ServerInitRetryBaseSeconds int                        `json:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds  int                        `json:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries       int                        `json:"serverInitMaxRetries"`
	ReloadMode                 string                     `json:"reloadMode"`
	BootstrapMode              string                     `json:"bootstrapMode"`
	BootstrapConcurrency       int                        `json:"bootstrapConcurrency"`
	BootstrapTimeoutSeconds    int                        `json:"bootstrapTimeoutSeconds"`
	DefaultActivationMode      string                     `json:"defaultActivationMode"`
	ExposeTools                bool                       `json:"exposeTools"`
	ToolNamespaceStrategy      string                     `json:"toolNamespaceStrategy"`
	Proxy                      ProxyConfigPayload         `json:"proxy"`
	Observability              ObservabilityConfigPayload `json:"observability"`
	RPC                        rpcConfigPayload           `json:"rpc"`
}

type RuntimeConfigUpdatePayload struct {
	RouteTimeoutSeconds         int    `json:"routeTimeoutSeconds"`
	PingIntervalSeconds         int    `json:"pingIntervalSeconds"`
	ToolRefreshSeconds          int    `json:"toolRefreshSeconds"`
	ToolRefreshConcurrency      int    `json:"toolRefreshConcurrency"`
	ClientCheckSeconds          int    `json:"clientCheckSeconds"`
	ClientInactiveSeconds       int    `json:"clientInactiveSeconds"`
	ServerInitRetryBaseSeconds  int    `json:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds   int    `json:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries        int    `json:"serverInitMaxRetries"`
	ReloadMode                  string `json:"reloadMode"`
	BootstrapMode               string `json:"bootstrapMode"`
	BootstrapConcurrency        int    `json:"bootstrapConcurrency"`
	BootstrapTimeoutSeconds     int    `json:"bootstrapTimeoutSeconds"`
	DefaultActivationMode       string `json:"defaultActivationMode"`
	ExposeTools                 bool   `json:"exposeTools"`
	ToolNamespaceStrategy       string `json:"toolNamespaceStrategy"`
	ProxyMode                   string `json:"proxyMode"`
	ProxyURL                    string `json:"proxyUrl"`
	ProxyNoProxy                string `json:"proxyNoProxy"`
	ObservabilityListenAddress  string `json:"observabilityListenAddress"`
	ObservabilityMetricsEnabled bool   `json:"observabilityMetricsEnabled"`
	ObservabilityHealthzEnabled bool   `json:"observabilityHealthzEnabled"`
}

type ProxyConfigPayload struct {
	Mode    string `json:"mode"`
	URL     string `json:"url,omitempty"`
	NoProxy string `json:"noProxy,omitempty"`
}

type ObservabilityConfigPayload struct {
	ListenAddress  string `json:"listenAddress"`
	MetricsEnabled *bool  `json:"metricsEnabled,omitempty"`
	HealthzEnabled *bool  `json:"healthzEnabled,omitempty"`
}

type rpcConfigPayload struct {
	ListenAddress           string               `json:"listenAddress"`
	MaxRecvMsgSize          int                  `json:"maxRecvMsgSize"`
	MaxSendMsgSize          int                  `json:"maxSendMsgSize"`
	KeepaliveTimeSeconds    int                  `json:"keepaliveTimeSeconds"`
	KeepaliveTimeoutSeconds int                  `json:"keepaliveTimeoutSeconds"`
	SocketMode              string               `json:"socketMode"`
	TLS                     rpcTLSConfigPayload  `json:"tls"`
	Auth                    rpcAuthConfigPayload `json:"auth"`
}

type rpcTLSConfigPayload struct {
	Enabled    bool   `json:"enabled"`
	CertFile   string `json:"certFile"`
	KeyFile    string `json:"keyFile"`
	CAFile     string `json:"caFile"`
	ClientAuth bool   `json:"clientAuth"`
}

type rpcAuthConfigPayload struct {
	Enabled  bool   `json:"enabled"`
	Mode     string `json:"mode,omitempty"`
	Token    string `json:"token,omitempty"`
	TokenEnv string `json:"tokenEnv,omitempty"`
}

type SubAgentConfigPayload struct {
	Enabled            bool     `json:"enabled"`
	EnabledTags        []string `json:"enabledTags,omitempty"`
	Model              string   `json:"model"`
	Provider           string   `json:"provider"`
	APIKeyEnvVar       string   `json:"apiKeyEnvVar"`
	BaseURL            string   `json:"baseURL"`
	MaxToolsPerRequest int      `json:"maxToolsPerRequest"`
	FilterPrompt       string   `json:"filterPrompt"`
}

type SubAgentUpdatePayload struct {
	Enabled            bool     `json:"enabled"`
	EnabledTags        []string `json:"enabledTags,omitempty"`
	Model              string   `json:"model"`
	Provider           string   `json:"provider"`
	APIKey             *string  `json:"apiKey,omitempty"`
	APIKeyEnvVar       string   `json:"apiKeyEnvVar"`
	BaseURL            string   `json:"baseURL"`
	MaxToolsPerRequest int      `json:"maxToolsPerRequest"`
	FilterPrompt       string   `json:"filterPrompt"`
}

type PluginSpecPayload struct {
	Name               string            `json:"name"`
	Category           string            `json:"category"`
	Required           bool              `json:"required"`
	Disabled           bool              `json:"disabled"`
	Cmd                []string          `json:"cmd"`
	Env                map[string]string `json:"env,omitempty"`
	Cwd                string            `json:"cwd,omitempty"`
	CommitHash         string            `json:"commitHash,omitempty"`
	TimeoutMs          int               `json:"timeoutMs"`
	HandshakeTimeoutMs int               `json:"handshakeTimeoutMs"`
	ConfigJSON         string            `json:"configJson,omitempty"`
	Flows              []string          `json:"flows"`
}

type PluginStatusPayload struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}
