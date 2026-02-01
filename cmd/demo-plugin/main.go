// Package main provides a demo governance plugin that showcases all 7 plugin categories.
// It implements the PluginService gRPC interface and demonstrates how plugins interact
// with the governance pipeline.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pluginv1 "mcpv/pkg/api/plugin/v1"
)

// Demo plugin categories.
const (
	CategoryObservability  = "observability"
	CategoryAuthentication = "authentication"
	CategoryAuthorization  = "authorization"
	CategoryRateLimiting   = "rate_limiting"
	CategoryValidation     = "validation"
	CategoryContent        = "content"
	CategoryAudit          = "audit"
)

// DemoPlugin implements a simple demo plugin for testing the governance pipeline.
type DemoPlugin struct {
	pluginv1.UnimplementedPluginServiceServer
	category string
	name     string
}

// NewDemoPlugin creates a new demo plugin with the given category.
func NewDemoPlugin(category, name string) *DemoPlugin {
	return &DemoPlugin{
		category: category,
		name:     name,
	}
}

// ProcessRequest handles incoming requests based on the plugin category.
func (p *DemoPlugin) HandleRequest(ctx context.Context, req *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	start := time.Now()
	defer func() {
		log.Printf("[%s] HandleRequest took %v", p.name, time.Since(start))
	}()

	switch p.category {
	case CategoryObservability:
		return p.handleObservability(ctx, req)
	case CategoryAuthentication:
		return p.handleAuthentication(ctx, req)
	case CategoryAuthorization:
		return p.handleAuthorization(ctx, req)
	case CategoryRateLimiting:
		return p.handleRateLimiting(ctx, req)
	case CategoryValidation:
		return p.handleValidation(ctx, req)
	case CategoryContent:
		return p.handleContent(ctx, req)
	case CategoryAudit:
		return p.handleAudit(ctx, req)
	default:
		return &pluginv1.PluginHandleResponse{Continue: true}, nil
	}
}

// HandleResponse handles outgoing responses.
func (p *DemoPlugin) HandleResponse(_ context.Context, _ *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	log.Printf("[%s] HandleResponse called", p.name)
	return &pluginv1.PluginHandleResponse{Continue: true}, nil
}

func (p *DemoPlugin) GetMetadata(_ context.Context, _ *emptypb.Empty) (*pluginv1.PluginMetadata, error) {
	flows := []string{"request", "response"}
	return &pluginv1.PluginMetadata{
		Name:     p.name,
		Category: p.category,
		Flows:    flows,
	}, nil
}

func (p *DemoPlugin) Configure(_ context.Context, req *pluginv1.PluginConfigureRequest) (*pluginv1.PluginConfigureResponse, error) {
	log.Printf("[%s] Configure called with config: %s", p.name, string(req.GetConfigJson()))
	return &pluginv1.PluginConfigureResponse{}, nil
}

func (p *DemoPlugin) CheckReady(_ context.Context, _ *emptypb.Empty) (*pluginv1.PluginReadyResponse, error) {
	return &pluginv1.PluginReadyResponse{Ready: true}, nil
}

func (p *DemoPlugin) Shutdown(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	log.Printf("[%s] Shutdown called", p.name)
	return &emptypb.Empty{}, nil
}

func (p *DemoPlugin) handleObservability(_ context.Context, req *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	// Log request metadata
	log.Printf("[observability] Request: method=%s, tool=%s", req.GetMethod(), req.GetToolName())
	return &pluginv1.PluginHandleResponse{
		Continue: true,
	}, nil
}

func (p *DemoPlugin) handleAuthentication(_ context.Context, req *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	// Check for demo token
	token, ok := req.GetMetadata()["authorization"]
	if !ok {
		// Allow unauthenticated for demo
		log.Printf("[authentication] No token provided, allowing for demo")
		return &pluginv1.PluginHandleResponse{Continue: true}, nil
	}

	// Demo: reject if token is "invalid"
	if strings.Contains(strings.ToLower(token), "invalid") {
		log.Printf("[authentication] Rejecting invalid token")
		return &pluginv1.PluginHandleResponse{
			Continue:      false,
			RejectCode:    "AUTHENTICATION_FAILED",
			RejectMessage: "Invalid authentication token",
		}, nil
	}

	log.Printf("[authentication] Token validated: %s...", token[:min(8, len(token))])
	return &pluginv1.PluginHandleResponse{
		Continue: true,
	}, nil
}

func (p *DemoPlugin) handleAuthorization(_ context.Context, req *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	// Check for demo role
	role, ok := req.GetMetadata()["x-role"]
	if !ok {
		role = "user" // Default role
	}

	// Demo: block "guest" role from admin tools
	if role == "guest" && strings.HasPrefix(req.GetToolName(), "admin_") {
		log.Printf("[authorization] Blocking guest from admin tool")
		return &pluginv1.PluginHandleResponse{
			Continue:      false,
			RejectCode:    "AUTHORIZATION_FAILED",
			RejectMessage: fmt.Sprintf("Insufficient permissions: role '%s' cannot access admin tools", role),
		}, nil
	}

	log.Printf("[authorization] Role '%s' authorized for %s", role, req.GetToolName())
	return &pluginv1.PluginHandleResponse{
		Continue: true,
	}, nil
}

func (p *DemoPlugin) handleRateLimiting(_ context.Context, req *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	// Demo: simple in-memory rate limiting (resets on restart)
	// In production, use distributed rate limiting (Redis, etc.)

	clientID := req.GetMetadata()["x-client-id"]
	if clientID == "" {
		clientID = "anonymous"
	}

	// Demo: always allow, just log
	log.Printf("[rate_limiting] Client '%s' request allowed", clientID)
	return &pluginv1.PluginHandleResponse{
		Continue: true,
	}, nil
}

func (p *DemoPlugin) handleValidation(_ context.Context, req *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	// Demo: validate request payload
	if len(req.GetRequestJson()) == 0 {
		log.Printf("[validation] No payload to validate")
		return &pluginv1.PluginHandleResponse{Continue: true}, nil
	}

	// Check if payload is valid JSON
	var payload interface{}
	if err := json.Unmarshal(req.GetRequestJson(), &payload); err != nil {
		log.Printf("[validation] Invalid JSON: %v", err)
		return &pluginv1.PluginHandleResponse{
			Continue:      false,
			RejectCode:    "VALIDATION_FAILED",
			RejectMessage: fmt.Sprintf("Invalid JSON payload: %v", err),
		}, nil
	}

	log.Printf("[validation] Payload validated successfully")
	return &pluginv1.PluginHandleResponse{
		Continue: true,
	}, nil
}

func (p *DemoPlugin) handleContent(_ context.Context, req *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	// Demo: content transformation
	// Could redact sensitive data, add prefixes, etc.

	log.Printf("[content] Processing content (length: %d)", len(req.GetRequestJson()))

	// Demo: just pass through, but we could modify the payload
	return &pluginv1.PluginHandleResponse{
		Continue: true,
	}, nil
}

func (p *DemoPlugin) handleAudit(_ context.Context, req *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	// Demo: audit logging
	auditEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"method":    req.GetMethod(),
		"tool":      req.GetToolName(),
		"client_id": req.GetMetadata()["x-client-id"],
		"plugin":    p.name,
	}

	auditJSON, _ := json.Marshal(auditEntry)
	log.Printf("[audit] %s", string(auditJSON))

	return &pluginv1.PluginHandleResponse{
		Continue: true,
	}, nil
}

func main() {
	var (
		category = flag.String("category", "observability", "Plugin category")
		name     = flag.String("name", "demo-plugin", "Plugin name")
		socket   = flag.String("socket", "", "Unix socket path (default: from env or /tmp/<name>.sock)")
	)
	flag.Parse()

	// Check environment variables first (used by plugin manager)
	// Environment variables override CLI flags
	if envCategory := os.Getenv("MCPV_PLUGIN_CATEGORY"); envCategory != "" {
		*category = envCategory
	}
	if envName := os.Getenv("MCPV_PLUGIN_NAME"); envName != "" {
		*name = envName
	}
	if *socket == "" {
		*socket = os.Getenv("MCPV_PLUGIN_SOCKET")
		if *socket == "" {
			*socket = os.Getenv("MCPD_PLUGIN_SOCKET")
		}
	}
	if *socket == "" {
		*socket = fmt.Sprintf("/tmp/%s.sock", *name)
	}

	// Remove existing socket file
	_ = os.Remove(*socket)

	// Create Unix socket listener with context
	lc := &net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "unix", *socket)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *socket, err)
	}

	plugin := NewDemoPlugin(*category, *name)

	// Create gRPC server and register the plugin service
	server := grpc.NewServer()
	pluginv1.RegisterPluginServiceServer(server, plugin)

	log.Printf("Demo plugin '%s' (category: %s) listening on %s", *name, *category, *socket)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Printf("Shutting down plugin '%s'", *name)
		server.GracefulStop()
		listener.Close()
	}()

	// Serve
	if err := server.Serve(listener); err != nil {
		listener.Close()
	}
}
