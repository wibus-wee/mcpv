package domain

import "strings"

func NormalizeTransport(transport TransportKind) TransportKind {
	trimmed := strings.ToLower(strings.TrimSpace(string(transport)))
	switch trimmed {
	case "":
		return TransportStdio
	case "stdio":
		return TransportStdio
	case "streamable_http", "streamable-http":
		return TransportStreamableHTTP
	default:
		return TransportKind(trimmed)
	}
}

func IsSupportedProtocolVersion(transport TransportKind, version string) bool {
	switch NormalizeTransport(transport) {
	case TransportStreamableHTTP:
		return isStreamableHTTPProtocolVersion(version)
	case TransportStdio:
		return version == DefaultProtocolVersion
	default:
		return false
	}
}

func isStreamableHTTPProtocolVersion(version string) bool {
	for _, candidate := range StreamableHTTPProtocolVersions {
		if version == candidate {
			return true
		}
	}
	return false
}
