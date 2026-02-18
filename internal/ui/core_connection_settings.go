package ui

import (
	"encoding/json"
	"strings"

	"mcpv/internal/domain"
)

const CoreConnectionSectionKey = "core-connection"

type CoreConnectionMode string

const (
	CoreConnectionModeLocal  CoreConnectionMode = "local"
	CoreConnectionModeRemote CoreConnectionMode = "remote"
)

type CoreConnectionSettings struct {
	Mode                    CoreConnectionMode
	Address                 string
	Caller                  string
	MaxRecvMsgSize          int
	MaxSendMsgSize          int
	KeepaliveTimeSeconds    int
	KeepaliveTimeoutSeconds int
	TLS                     domain.RPCTLSConfig
	Auth                    domain.RPCAuthConfig
}

type coreConnectionPayload struct {
	Mode                    string                `json:"mode,omitempty"`
	RPCAddress              string                `json:"rpcAddress,omitempty"`
	Caller                  string                `json:"caller,omitempty"`
	MaxRecvMsgSize          *int                  `json:"maxRecvMsgSize,omitempty"`
	MaxSendMsgSize          *int                  `json:"maxSendMsgSize,omitempty"`
	KeepaliveTimeSeconds    *int                  `json:"keepaliveTimeSeconds,omitempty"`
	KeepaliveTimeoutSeconds *int                  `json:"keepaliveTimeoutSeconds,omitempty"`
	TLS                     *rpcTLSConfigPayload  `json:"tls,omitempty"`
	Auth                    *rpcAuthConfigPayload `json:"auth,omitempty"`
}

type rpcTLSConfigPayload struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	CertFile string `json:"certFile,omitempty"`
	KeyFile  string `json:"keyFile,omitempty"`
	CAFile   string `json:"caFile,omitempty"`
}

type rpcAuthConfigPayload struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Token    string `json:"token,omitempty"`
	TokenEnv string `json:"tokenEnv,omitempty"`
}

func DefaultCoreConnectionSettings() CoreConnectionSettings {
	return CoreConnectionSettings{
		Mode:                    CoreConnectionModeLocal,
		Address:                 domain.DefaultRPCListenAddress,
		Caller:                  domain.InternalUIClientName,
		MaxRecvMsgSize:          domain.DefaultRPCMaxRecvMsgSize,
		MaxSendMsgSize:          domain.DefaultRPCMaxSendMsgSize,
		KeepaliveTimeSeconds:    domain.DefaultRPCKeepaliveTimeSeconds,
		KeepaliveTimeoutSeconds: domain.DefaultRPCKeepaliveTimeoutSeconds,
		TLS: domain.RPCTLSConfig{
			Enabled: false,
		},
		Auth: domain.RPCAuthConfig{
			Enabled: false,
			Mode:    domain.RPCAuthModeToken,
		},
	}
}

func ParseCoreConnectionSettings(raw json.RawMessage) (CoreConnectionSettings, error) {
	settings := DefaultCoreConnectionSettings()
	if len(raw) == 0 {
		return settings, nil
	}

	var payload coreConnectionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return settings, err
	}

	if mode := normalizeCoreConnectionMode(payload.Mode); mode != "" {
		settings.Mode = mode
	}
	if strings.TrimSpace(payload.RPCAddress) != "" {
		settings.Address = strings.TrimSpace(payload.RPCAddress)
	}
	if strings.TrimSpace(payload.Caller) != "" {
		settings.Caller = strings.TrimSpace(payload.Caller)
	}
	if payload.MaxRecvMsgSize != nil {
		settings.MaxRecvMsgSize = *payload.MaxRecvMsgSize
	}
	if payload.MaxSendMsgSize != nil {
		settings.MaxSendMsgSize = *payload.MaxSendMsgSize
	}
	if payload.KeepaliveTimeSeconds != nil {
		settings.KeepaliveTimeSeconds = *payload.KeepaliveTimeSeconds
	}
	if payload.KeepaliveTimeoutSeconds != nil {
		settings.KeepaliveTimeoutSeconds = *payload.KeepaliveTimeoutSeconds
	}
	if payload.TLS != nil {
		if payload.TLS.Enabled != nil {
			settings.TLS.Enabled = *payload.TLS.Enabled
		}
		if strings.TrimSpace(payload.TLS.CertFile) != "" {
			settings.TLS.CertFile = strings.TrimSpace(payload.TLS.CertFile)
		}
		if strings.TrimSpace(payload.TLS.KeyFile) != "" {
			settings.TLS.KeyFile = strings.TrimSpace(payload.TLS.KeyFile)
		}
		if strings.TrimSpace(payload.TLS.CAFile) != "" {
			settings.TLS.CAFile = strings.TrimSpace(payload.TLS.CAFile)
		}
	}
	if payload.Auth != nil {
		if payload.Auth.Enabled != nil {
			settings.Auth.Enabled = *payload.Auth.Enabled
		}
		if strings.TrimSpace(payload.Auth.Token) != "" {
			settings.Auth.Token = strings.TrimSpace(payload.Auth.Token)
		}
		if strings.TrimSpace(payload.Auth.TokenEnv) != "" {
			settings.Auth.TokenEnv = strings.TrimSpace(payload.Auth.TokenEnv)
		}
		if mode := normalizeAuthMode(payload.Auth.Mode); mode != "" {
			settings.Auth.Mode = mode
		}
	}

	return normalizeCoreConnectionSettings(settings), nil
}

func normalizeCoreConnectionSettings(settings CoreConnectionSettings) CoreConnectionSettings {
	if settings.Mode != CoreConnectionModeRemote && settings.Mode != CoreConnectionModeLocal {
		settings.Mode = CoreConnectionModeLocal
	}
	if strings.TrimSpace(settings.Address) == "" {
		settings.Address = domain.DefaultRPCListenAddress
	}
	if strings.TrimSpace(settings.Caller) == "" {
		settings.Caller = domain.InternalUIClientName
	}
	if settings.MaxRecvMsgSize <= 0 {
		settings.MaxRecvMsgSize = domain.DefaultRPCMaxRecvMsgSize
	}
	if settings.MaxSendMsgSize <= 0 {
		settings.MaxSendMsgSize = domain.DefaultRPCMaxSendMsgSize
	}
	if settings.KeepaliveTimeSeconds <= 0 {
		settings.KeepaliveTimeSeconds = domain.DefaultRPCKeepaliveTimeSeconds
	}
	if settings.KeepaliveTimeoutSeconds <= 0 {
		settings.KeepaliveTimeoutSeconds = domain.DefaultRPCKeepaliveTimeoutSeconds
	}

	if strings.TrimSpace(settings.TLS.CertFile) != "" || strings.TrimSpace(settings.TLS.KeyFile) != "" || strings.TrimSpace(settings.TLS.CAFile) != "" {
		settings.TLS.Enabled = true
	}

	if settings.Auth.Mode == "" {
		settings.Auth.Mode = domain.RPCAuthModeToken
	}
	if strings.TrimSpace(settings.Auth.Token) != "" || strings.TrimSpace(settings.Auth.TokenEnv) != "" {
		settings.Auth.Enabled = true
	}

	return settings
}

func normalizeCoreConnectionMode(mode string) CoreConnectionMode {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case string(CoreConnectionModeRemote):
		return CoreConnectionModeRemote
	case string(CoreConnectionModeLocal):
		return CoreConnectionModeLocal
	default:
		return ""
	}
}

func normalizeAuthMode(mode string) domain.RPCAuthMode {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case string(domain.RPCAuthModeMTLS):
		return domain.RPCAuthModeMTLS
	case string(domain.RPCAuthModeToken):
		return domain.RPCAuthModeToken
	default:
		return ""
	}
}
