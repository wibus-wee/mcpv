package normalizer

type RawCatalog struct {
	Servers          []RawServerSpec `mapstructure:"servers"`
	Plugins          []RawPluginSpec `mapstructure:"plugins"`
	RawRuntimeConfig `mapstructure:",squash"`
}

type RawServerSpec struct {
	Name                string                  `mapstructure:"name"`
	Transport           string                  `mapstructure:"transport"`
	Cmd                 []string                `mapstructure:"cmd"`
	Env                 map[string]string       `mapstructure:"env"`
	Cwd                 string                  `mapstructure:"cwd"`
	Tags                []string                `mapstructure:"tags"`
	IdleSeconds         int                     `mapstructure:"idleSeconds"`
	MaxConcurrent       int                     `mapstructure:"maxConcurrent"`
	Strategy            string                  `mapstructure:"strategy"`
	SessionTTLSeconds   *int                    `mapstructure:"sessionTTLSeconds"`
	Disabled            bool                    `mapstructure:"disabled"`
	MinReady            int                     `mapstructure:"minReady"`
	ActivationMode      string                  `mapstructure:"activationMode"`
	DrainTimeoutSeconds int                     `mapstructure:"drainTimeoutSeconds"`
	ProtocolVersion     string                  `mapstructure:"protocolVersion"`
	ExposeTools         []string                `mapstructure:"exposeTools"`
	HTTP                RawStreamableHTTPConfig `mapstructure:"http"`
}

type RawPluginSpec struct {
	Name               string            `mapstructure:"name"`
	Category           string            `mapstructure:"category"`
	Required           *bool             `mapstructure:"required"`
	Cmd                []string          `mapstructure:"cmd"`
	Env                map[string]string `mapstructure:"env"`
	Cwd                string            `mapstructure:"cwd"`
	CommitHash         string            `mapstructure:"commitHash"`
	TimeoutMs          *int              `mapstructure:"timeoutMs"`
	HandshakeTimeoutMs *int              `mapstructure:"handshakeTimeoutMs"`
	Config             map[string]any    `mapstructure:"config"`
	Flows              []string          `mapstructure:"flows"`
}

type RawStreamableHTTPConfig struct {
	Endpoint   string            `mapstructure:"endpoint"`
	Headers    map[string]string `mapstructure:"headers"`
	MaxRetries *int              `mapstructure:"maxRetries"`
}

type RawRuntimeConfig struct {
	RouteTimeoutSeconds        int                    `mapstructure:"routeTimeoutSeconds"`
	PingIntervalSeconds        int                    `mapstructure:"pingIntervalSeconds"`
	ToolRefreshSeconds         int                    `mapstructure:"toolRefreshSeconds"`
	ToolRefreshConcurrency     int                    `mapstructure:"toolRefreshConcurrency"`
	ClientCheckSeconds         int                    `mapstructure:"clientCheckSeconds"`
	ClientInactiveSeconds      int                    `mapstructure:"clientInactiveSeconds"`
	ServerInitRetryBaseSeconds int                    `mapstructure:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds  int                    `mapstructure:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries       int                    `mapstructure:"serverInitMaxRetries"`
	ReloadMode                 string                 `mapstructure:"reloadMode"`
	BootstrapMode              string                 `mapstructure:"bootstrapMode"`
	BootstrapConcurrency       int                    `mapstructure:"bootstrapConcurrency"`
	BootstrapTimeoutSeconds    int                    `mapstructure:"bootstrapTimeoutSeconds"`
	DefaultActivationMode      string                 `mapstructure:"defaultActivationMode"`
	ExposeTools                bool                   `mapstructure:"exposeTools"`
	ToolNamespaceStrategy      string                 `mapstructure:"toolNamespaceStrategy"`
	Observability              RawObservabilityConfig `mapstructure:"observability"`
	RPC                        RawRPCConfig           `mapstructure:"rpc"`
	SubAgent                   RawSubAgentConfig      `mapstructure:"subAgent"`
}

type RawSubAgentConfig struct {
	Enabled            *bool    `mapstructure:"enabled"`
	EnabledTags        []string `mapstructure:"enabledTags"`
	Model              string   `mapstructure:"model"`
	Provider           string   `mapstructure:"provider"`
	APIKey             string   `mapstructure:"apiKey"`
	APIKeyEnvVar       string   `mapstructure:"apiKeyEnvVar"`
	BaseURL            string   `mapstructure:"baseURL"`
	MaxToolsPerRequest int      `mapstructure:"maxToolsPerRequest"`
	FilterPrompt       string   `mapstructure:"filterPrompt"`
}

type RawObservabilityConfig struct {
	ListenAddress  string `mapstructure:"listenAddress"`
	MetricsEnabled *bool  `mapstructure:"metricsEnabled"`
	HealthzEnabled *bool  `mapstructure:"healthzEnabled"`
}

type RawRPCConfig struct {
	ListenAddress           string          `mapstructure:"listenAddress"`
	MaxRecvMsgSize          int             `mapstructure:"maxRecvMsgSize"`
	MaxSendMsgSize          int             `mapstructure:"maxSendMsgSize"`
	KeepaliveTimeSeconds    int             `mapstructure:"keepaliveTimeSeconds"`
	KeepaliveTimeoutSeconds int             `mapstructure:"keepaliveTimeoutSeconds"`
	SocketMode              string          `mapstructure:"socketMode"`
	TLS                     RawRPCTLSConfig `mapstructure:"tls"`
}

type RawRPCTLSConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	CertFile   string `mapstructure:"certFile"`
	KeyFile    string `mapstructure:"keyFile"`
	CAFile     string `mapstructure:"caFile"`
	ClientAuth bool   `mapstructure:"clientAuth"`
}
