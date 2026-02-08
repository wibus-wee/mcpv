package instance

import (
	"context"
	"os/exec"

	"google.golang.org/grpc"

	"mcpv/internal/domain"
	pluginv1 "mcpv/pkg/api/plugin/v1"
)

type StopFunc func(context.Context) error

type Instance struct {
	Spec       domain.PluginSpec
	SocketDir  string
	SocketPath string
	Cmd        *exec.Cmd
	Conn       *grpc.ClientConn
	Client     pluginv1.PluginServiceClient
	Metadata   *pluginv1.PluginMetadata
	Stop       StopFunc
}
