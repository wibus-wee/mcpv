package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/ui"
)

const (
	defaultProxyTimeout = 15 * time.Second
	maxProxyBodyBytes   = 4 << 20
)

// ProxyService forwards HTTP requests for the frontend.
type ProxyService struct {
	logger *zap.Logger
}

func NewProxyService(deps *ServiceDeps) *ProxyService {
	return &ProxyService{
		logger: deps.loggerNamed("proxy-service"),
	}
}

func (s *ProxyService) Fetch(ctx context.Context, req ProxyFetchRequest) (ProxyFetchResponse, error) {
	trimmedURL := strings.TrimSpace(req.URL)
	if trimmedURL == "" {
		return ProxyFetchResponse{}, ui.NewError(ui.ErrCodeInvalidRequest, "URL is required")
	}

	parsed, err := url.Parse(trimmedURL)
	if err != nil {
		return ProxyFetchResponse{}, ui.NewErrorWithDetails(ui.ErrCodeInvalidRequest, "Invalid URL", err.Error())
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ProxyFetchResponse{}, ui.NewError(ui.ErrCodeInvalidRequest, "Only http/https URLs are allowed")
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !isAllowedProxyMethod(method) {
		return ProxyFetchResponse{}, ui.NewError(ui.ErrCodeInvalidRequest, fmt.Sprintf("Method %s is not allowed", method))
	}

	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	request, err := http.NewRequestWithContext(ctx, method, trimmedURL, body)
	if err != nil {
		return ProxyFetchResponse{}, ui.NewErrorWithDetails(ui.ErrCodeInvalidRequest, "Failed to build request", err.Error())
	}

	for key, value := range req.Headers {
		if key == "" {
			continue
		}
		if strings.EqualFold(key, "Host") || strings.EqualFold(key, "Content-Length") {
			continue
		}
		request.Header.Set(key, value)
	}

	timeout := defaultProxyTimeout
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	client := &http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		s.logger.Warn("proxy request failed", zap.String("url", trimmedURL), zap.Error(err))
		return ProxyFetchResponse{}, ui.NewErrorWithDetails(ui.ErrCodeInternal, "Proxy request failed", err.Error())
	}
	defer response.Body.Close()

	bodyBytes, err := readLimitedBody(response.Body, maxProxyBodyBytes)
	if err != nil {
		return ProxyFetchResponse{}, ui.NewErrorWithDetails(ui.ErrCodeInternal, "Failed to read response body", err.Error())
	}

	headers := make(map[string]string, len(response.Header))
	for key, values := range response.Header {
		headers[key] = strings.Join(values, ", ")
	}

	return ProxyFetchResponse{
		Status:  response.StatusCode,
		Headers: headers,
		Body:    string(bodyBytes),
	}, nil
}

func isAllowedProxyMethod(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete:
		return true
	default:
		return false
	}
}

func readLimitedBody(reader io.Reader, limit int64) ([]byte, error) {
	limited := io.LimitReader(reader, limit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}
