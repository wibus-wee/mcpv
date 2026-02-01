package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pluginv1 "mcpv/pkg/api/plugin/v1"
)

func main() {
	socket := os.Getenv("MCPV_PLUGIN_SOCKET")
	if socket == "" {
		socket = os.Getenv("MCPD_PLUGIN_SOCKET")
	}
	if socket == "" {
		log.Fatal("plugin socket not provided")
	}
	dir := filepath.Dir(socket)
	_ = os.MkdirAll(dir, 0o700)
	_ = os.Remove(socket)
	lis, err := net.Listen("unix", socket)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	server := grpc.NewServer()
	registry := newFakePluginServer()
	pluginv1.RegisterPluginServiceServer(server, registry)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := server.Serve(lis); err != nil {
			log.Printf("serve error: %v", err)
		}
	}()

	<-ctx.Done()
	server.GracefulStop()
}

func newFakePluginServer() *fakePluginServer {
	name := os.Getenv("MCPV_PLUGIN_NAME")
	if name == "" {
		name = "fake-plugin"
	}
	category := os.Getenv("MCPV_PLUGIN_CATEGORY")
	if category == "" {
		category = "authentication"
	}
	commitHash := os.Getenv("MCPV_PLUGIN_COMMIT_HASH")
	flows := parseFlows(os.Getenv("MCPV_PLUGIN_FLOWS"))
	return &fakePluginServer{
		metadata: &pluginv1.PluginMetadata{
			Name:       name,
			Category:   category,
			CommitHash: commitHash,
			Flows:      flows,
		},
	}
}

func parseFlows(raw string) []string {
	if raw == "" {
		return []string{"request", "response"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, strings.ToLower(part))
		}
	}
	if len(out) == 0 {
		return []string{"request", "response"}
	}
	return out
}

type fakePluginServer struct {
	pluginv1.UnimplementedPluginServiceServer
	metadata *pluginv1.PluginMetadata
}

func (f *fakePluginServer) GetMetadata(context.Context, *emptypb.Empty) (*pluginv1.PluginMetadata, error) {
	return f.metadata, nil
}

func (f *fakePluginServer) Configure(context.Context, *pluginv1.PluginConfigureRequest) (*pluginv1.PluginConfigureResponse, error) {
	return &pluginv1.PluginConfigureResponse{}, nil
}

func (f *fakePluginServer) CheckReady(context.Context, *emptypb.Empty) (*pluginv1.PluginReadyResponse, error) {
	return &pluginv1.PluginReadyResponse{Ready: true}, nil
}

func (f *fakePluginServer) HandleRequest(context.Context, *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	return &pluginv1.PluginHandleResponse{Continue: true}, nil
}

func (f *fakePluginServer) HandleResponse(context.Context, *pluginv1.PluginHandleRequest) (*pluginv1.PluginHandleResponse, error) {
	return &pluginv1.PluginHandleResponse{Continue: true}, nil
}

func (f *fakePluginServer) Shutdown(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
