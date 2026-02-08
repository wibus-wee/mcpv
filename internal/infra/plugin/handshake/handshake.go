package handshake

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"mcpv/internal/domain"
	"mcpv/internal/infra/retry"
	pluginv1 "mcpv/pkg/api/plugin/v1"
)

func Connect(ctx context.Context, spec domain.PluginSpec, socketPath string) (conn *grpc.ClientConn, client pluginv1.PluginServiceClient, metadata *pluginv1.PluginMetadata, err error) {
	deadline := time.Duration(domain.DefaultPluginHandshakeTimeoutSeconds) * time.Second
	if spec.HandshakeTimeoutMs > 0 {
		deadline = time.Duration(spec.HandshakeTimeoutMs) * time.Millisecond
	}
	if deadline <= 0 {
		deadline = time.Duration(domain.DefaultPluginHandshakeTimeoutSeconds) * time.Second
	}

	handshakeCtx, handshakeCancel := context.WithTimeout(ctx, deadline)
	defer handshakeCancel()

	var lastErr error
	retryPolicy := retry.Policy{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Factor:     1,
		MaxRetries: -1,
	}
	if retryErr := retry.Retry(handshakeCtx, retryPolicy, func(ctx context.Context) error {
		conn, client, metadata, lastErr = tryConnect(ctx, socketPath)
		return lastErr
	}); retryErr != nil {
		if errors.Is(retryErr, context.DeadlineExceeded) || errors.Is(retryErr, context.Canceled) {
			if lastErr != nil {
				err = fmt.Errorf("plugin handshake timeout: %w", lastErr)
			} else {
				err = retryErr
			}
			return nil, nil, nil, err
		}
		return nil, nil, nil, retryErr
	}

	if err = validateMetadata(spec, metadata); err != nil {
		_ = conn.Close()
		return nil, nil, nil, err
	}

	cfgCtx, cfgCancel := context.WithTimeout(handshakeCtx, deadline)
	defer cfgCancel()
	_, cfgErr := client.Configure(cfgCtx, &pluginv1.PluginConfigureRequest{ConfigJson: spec.ConfigJSON})
	if cfgErr != nil {
		_ = conn.Close()
		err = fmt.Errorf("plugin configure: %w", cfgErr)
		return nil, nil, nil, err
	}

	readyCtx, readyCancel := context.WithTimeout(handshakeCtx, deadline)
	defer readyCancel()
	ready, readyErr := client.CheckReady(readyCtx, &emptypb.Empty{})
	if readyErr != nil {
		_ = conn.Close()
		err = fmt.Errorf("plugin readiness: %w", readyErr)
		return nil, nil, nil, err
	}
	if ready != nil && !ready.GetReady() {
		_ = conn.Close()
		msg := strings.TrimSpace(ready.GetMessage())
		if msg == "" {
			msg = "plugin not ready"
		}
		return nil, nil, nil, errors.New(msg)
	}

	return conn, client, metadata, nil
}

func tryConnect(ctx context.Context, socketPath string) (*grpc.ClientConn, pluginv1.PluginServiceClient, *pluginv1.PluginMetadata, error) {
	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(dialCtx context.Context, _ string) (net.Conn, error) {
			dialer := &net.Dialer{}
			return dialer.DialContext(dialCtx, "unix", socketPath)
		}),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("plugin dial: %w", err)
	}

	client := pluginv1.NewPluginServiceClient(conn)

	metaCtx, metaCancel := context.WithTimeout(ctx, 2*time.Second)
	defer metaCancel()
	metadata, err := client.GetMetadata(metaCtx, &emptypb.Empty{})
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, err
	}

	return conn, client, metadata, nil
}

func validateMetadata(spec domain.PluginSpec, metadata *pluginv1.PluginMetadata) error {
	if metadata == nil {
		return errors.New("plugin metadata missing")
	}
	if name := strings.TrimSpace(metadata.GetName()); name != "" && name != spec.Name {
		return fmt.Errorf("plugin name mismatch: expected %q got %q", spec.Name, name)
	}
	if category := strings.TrimSpace(metadata.GetCategory()); category != "" && category != string(spec.Category) {
		return fmt.Errorf("plugin category mismatch: expected %q got %q", spec.Category, category)
	}
	if spec.CommitHash != "" {
		if metadata.GetCommitHash() != spec.CommitHash {
			return fmt.Errorf("plugin commit hash mismatch: expected %q got %q", spec.CommitHash, metadata.GetCommitHash())
		}
	}
	if len(metadata.GetFlows()) > 0 && len(spec.Flows) > 0 {
		allowed := map[string]struct{}{}
		for _, flow := range metadata.GetFlows() {
			allowed[strings.ToLower(flow)] = struct{}{}
		}
		for _, flow := range spec.Flows {
			if _, ok := allowed[string(flow)]; !ok {
				return fmt.Errorf("plugin flow %q not supported", flow)
			}
		}
	}
	return nil
}
