package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry/diagnostics"
)

// StreamableHTTPTransport connects to MCP servers over streamable HTTP.
type StreamableHTTPTransport struct {
	logger             *zap.Logger
	listChangeEmitter  domain.ListChangeEmitter
	samplingHandler    domain.SamplingHandler
	elicitationHandler domain.ElicitationHandler
	probe              diagnostics.Probe
}

// StreamableHTTPTransportOptions configures the streamable HTTP transport.
type StreamableHTTPTransportOptions struct {
	Logger             *zap.Logger
	ListChangeEmitter  domain.ListChangeEmitter
	SamplingHandler    domain.SamplingHandler
	ElicitationHandler domain.ElicitationHandler
	Probe              diagnostics.Probe
}

// NewStreamableHTTPTransport creates a streamable HTTP transport for MCP.
func NewStreamableHTTPTransport(opts StreamableHTTPTransportOptions) *StreamableHTTPTransport {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	probe := opts.Probe
	if probe == nil {
		probe = diagnostics.NoopProbe{}
	}
	return &StreamableHTTPTransport{
		logger:             logger,
		listChangeEmitter:  opts.ListChangeEmitter,
		samplingHandler:    opts.SamplingHandler,
		elicitationHandler: opts.ElicitationHandler,
		probe:              probe,
	}
}

// Connect establishes a streamable HTTP connection for the given server spec.
func (t *StreamableHTTPTransport) Connect(ctx context.Context, specKey string, spec domain.ServerSpec, streams domain.IOStreams) (domain.Conn, error) {
	started := time.Now()
	attemptID, _ := diagnostics.AttemptIDFromContext(ctx)
	baseAttrs := map[string]string{
		"transport": string(domain.TransportStreamableHTTP),
	}
	if spec.HTTP == nil {
		t.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepTransportConnect,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      fmt.Errorf("server %s: streamable http config is required", spec.Name).Error(),
			Attributes: baseAttrs,
		})
		return nil, fmt.Errorf("server %s: streamable http config is required", spec.Name)
	}
	endpoint := strings.TrimSpace(spec.HTTP.Endpoint)
	if endpoint == "" {
		t.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepTransportConnect,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      fmt.Errorf("server %s: streamable http endpoint is required", spec.Name).Error(),
			Attributes: baseAttrs,
		})
		return nil, fmt.Errorf("server %s: streamable http endpoint is required", spec.Name)
	}
	attrs := map[string]string{
		"transport":    string(domain.TransportStreamableHTTP),
		"endpointSafe": safeEndpoint(endpoint),
		"maxRetries":   strconv.Itoa(effectiveMaxRetries(spec.HTTP.MaxRetries)),
	}
	if len(spec.HTTP.Headers) > 0 {
		attrs["headerKeys"] = strings.Join(sortedHeaderKeys(spec.HTTP.Headers), ",")
		attrs["headerCount"] = strconv.Itoa(len(spec.HTTP.Headers))
	}
	sensitive := map[string]string{}
	if t.captureSensitive() {
		sensitive["endpoint"] = endpoint
		if len(spec.HTTP.Headers) > 0 {
			sensitive["headers"] = diagnostics.EncodeStringMap(spec.HTTP.Headers)
		}
	}
	t.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepTransportConnect,
		Phase:      diagnostics.PhaseEnter,
		Timestamp:  started,
		Attributes: attrs,
		Sensitive:  sensitive,
	})

	headerTransport, err := buildStreamableHTTPTransport(spec)
	if err != nil {
		t.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepTransportConnect,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      err.Error(),
			Attributes: attrs,
			Sensitive:  sensitive,
		})
		return nil, err
	}

	client := &http.Client{
		Transport: headerTransport,
	}

	// Loader defaults MaxRetries, but keep a defensive fallback for unnormalized specs.
	maxRetries := effectiveMaxRetries(spec.HTTP.MaxRetries)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: client,
		MaxRetries: maxRetries,
	}
	mcpConn, err := transport.Connect(ctx)
	if err != nil {
		t.recordEvent(diagnostics.Event{
			SpecKey:    specKey,
			ServerName: spec.Name,
			AttemptID:  attemptID,
			Step:       diagnostics.StepTransportConnect,
			Phase:      diagnostics.PhaseError,
			Timestamp:  time.Now(),
			Duration:   time.Since(started),
			Error:      fmt.Errorf("connect streamable http: %w", err).Error(),
			Attributes: attrs,
			Sensitive:  sensitive,
		})
		return nil, fmt.Errorf("connect streamable http: %w", err)
	}
	t.recordEvent(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: spec.Name,
		AttemptID:  attemptID,
		Step:       diagnostics.StepTransportConnect,
		Phase:      diagnostics.PhaseExit,
		Timestamp:  time.Now(),
		Duration:   time.Since(started),
		Attributes: attrs,
		Sensitive:  sensitive,
	})

	if streams.Reader != nil || streams.Writer != nil {
		t.logger.Warn("streamable http transport ignores IO streams",
			zap.String("server", spec.Name),
		)
	}
	if streams.Reader != nil {
		if err := streams.Reader.Close(); err != nil {
			t.logger.Warn("close stream reader failed", zap.Error(err))
		}
	}
	if streams.Writer != nil {
		if err := streams.Writer.Close(); err != nil {
			t.logger.Warn("close stream writer failed", zap.Error(err))
		}
	}

	return newClientConn(mcpConn, clientConnOptions{
		Logger:             t.logger.Named("mcp_http_conn"),
		ListChangeEmitter:  t.listChangeEmitter,
		SamplingHandler:    t.samplingHandler,
		ElicitationHandler: t.elicitationHandler,
		ServerType:         spec.Name,
		SpecKey:            specKey,
	}), nil
}

func buildStreamableHTTPTransport(spec domain.ServerSpec) (http.RoundTripper, error) {
	headers := http.Header{}
	if spec.ProtocolVersion != "" {
		headers.Set("Mcp-Protocol-Version", spec.ProtocolVersion)
	}
	for key, value := range spec.HTTP.Headers {
		name := http.CanonicalHeaderKey(strings.TrimSpace(key))
		if name == "" {
			return nil, fmt.Errorf("server %s: http headers contain empty key", spec.Name)
		}
		headers.Set(name, value)
	}

	base := http.DefaultTransport
	if base == nil {
		return nil, errors.New("default http transport is nil")
	}

	return &headerRoundTripper{
		base:    base,
		headers: headers,
	}, nil
}

type headerRoundTripper struct {
	base    http.RoundTripper
	headers http.Header
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if cloned.Header == nil {
		cloned.Header = make(http.Header)
	}
	// Explicitly overwrite any existing values with configured headers.
	for key, values := range h.headers {
		cloned.Header.Del(key)
		for _, value := range values {
			cloned.Header.Add(key, value)
		}
	}
	return h.base.RoundTrip(cloned)
}

func effectiveMaxRetries(value int) int {
	if value == 0 {
		return domain.DefaultStreamableHTTPMaxRetries
	}
	return value
}

func sortedHeaderKeys(headers map[string]string) []string {
	if len(headers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func safeEndpoint(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func (t *StreamableHTTPTransport) recordEvent(event diagnostics.Event) {
	if t == nil || t.probe == nil {
		return
	}
	if len(event.Sensitive) == 0 {
		event.Sensitive = nil
	}
	t.probe.Record(event)
}

func (t *StreamableHTTPTransport) captureSensitive() bool {
	if t == nil || t.probe == nil {
		return false
	}
	if probe, ok := t.probe.(diagnostics.SensitiveProbe); ok {
		return probe.CaptureSensitive()
	}
	return false
}
