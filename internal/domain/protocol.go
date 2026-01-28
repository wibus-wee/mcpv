package domain

import "strings"

// IsSupportedProtocolVersion reports whether the version is supported for a transport.
func IsSupportedProtocolVersion(transport TransportKind, version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return true
	}
	switch NormalizeTransport(transport) {
	case TransportStreamableHTTP:
		for _, candidate := range StreamableHTTPProtocolVersions {
			if version == candidate {
				return true
			}
		}
		return false
	case TransportStdio:
		return version == DefaultProtocolVersion
	default:
		return version == DefaultProtocolVersion
	}
}
