package ui

import (
	"fmt"
	"net/url"
	"strings"
)

// DeepLinkScheme is the custom URL scheme for mcpv.
const (
	DeepLinkScheme    = "mcpv"
	DeepLinkSchemeDev = "mcpvev"
)

// DeepLink represents a parsed mcpv:// or mcpvev:// URL.
type DeepLink struct {
	raw    string
	path   string
	params map[string]string
}

// ParseDeepLink parses a mcpv:// or mcpvev:// URL into a DeepLink.
// Returns error if the URL is invalid or uses wrong scheme.
func ParseDeepLink(rawURL string) (*DeepLink, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("empty URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != DeepLinkScheme && parsed.Scheme != DeepLinkSchemeDev {
		return nil, fmt.Errorf("invalid scheme: expected %s or %s, got %s", DeepLinkScheme, DeepLinkSchemeDev, parsed.Scheme)
	}

	// Build path from host and path segments
	// mcpv://servers → path = "servers"
	// mcpv://servers/detail → path = "servers/detail"
	pathParts := []string{}
	if parsed.Host != "" {
		pathParts = append(pathParts, parsed.Host)
	}
	if parsed.Path != "" {
		pathParts = append(pathParts, strings.Trim(parsed.Path, "/"))
	}

	finalPath := strings.Join(pathParts, "/")
	if finalPath == "" {
		finalPath = "/"
	} else if !strings.HasPrefix(finalPath, "/") {
		finalPath = "/" + finalPath
	}

	// Extract query parameters
	params := make(map[string]string)
	for key, values := range parsed.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}

	return &DeepLink{
		raw:    rawURL,
		path:   finalPath,
		params: params,
	}, nil
}

// Path returns the normalized path from the deep link.
func (d *DeepLink) Path() string {
	return d.path
}

// Params returns the query parameters as a map.
func (d *DeepLink) Params() map[string]string {
	return d.params
}

// Raw returns the original raw URL string.
func (d *DeepLink) Raw() string {
	return d.raw
}
