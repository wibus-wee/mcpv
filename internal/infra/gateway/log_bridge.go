package gateway

import (
	"context"
	"encoding/json"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/infra/retry"
	controlv1 "mcpv/pkg/api/control/v1"
)

type logBridge struct {
	server     *mcp.Server
	clients    *clientManager
	caller     string
	tags       []string
	serverName string
	callerPID  int64
	logger     *zap.Logger
}

func newLogBridge(server *mcp.Server, clients *clientManager, caller string, tags []string, serverName string, callerPID int64, logger *zap.Logger) *logBridge {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &logBridge{
		server:     server,
		clients:    clients,
		caller:     caller,
		tags:       tags,
		serverName: serverName,
		callerPID:  callerPID,
		logger:     logger.Named("log_bridge"),
	}
}

func (b *logBridge) Run(ctx context.Context) {
	backoff := retry.NewBackoff(retry.Policy{
		BaseDelay: time.Second,
		MaxDelay:  30 * time.Second,
	})

	for {
		if ctx.Err() != nil {
			return
		}

		client, err := b.clients.get(ctx)
		if err != nil {
			b.logger.Warn("rpc connect failed", zap.Error(err))
			backoff.Sleep(ctx)
			continue
		}

		stream, err := client.Control().StreamLogs(ctx, &controlv1.StreamLogsRequest{
			Caller:   b.caller,
			MinLevel: controlv1.LogLevel_LOG_LEVEL_DEBUG,
		})
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				if regErr := b.registerCaller(ctx); regErr == nil {
					continue
				}
			}
			b.logger.Warn("rpc stream logs failed", zap.Error(err))
			b.clients.reset()
			backoff.Sleep(ctx)
			continue
		}

		backoff.Reset()

		for {
			entry, err := stream.Recv()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if status.Code(err) == codes.Canceled {
					return
				}
				b.logger.Warn("rpc log stream interrupted", zap.Error(err))
				b.clients.reset()
				backoff.Sleep(ctx)
				break
			}
			b.publish(ctx, entry)
		}
	}
}

func (b *logBridge) registerCaller(ctx context.Context) error {
	client, err := b.clients.get(ctx)
	if err != nil {
		return err
	}
	_, err = client.Control().RegisterCaller(ctx, &controlv1.RegisterCallerRequest{
		Caller: b.caller,
		Pid:    b.callerPID,
		Tags:   append([]string(nil), b.tags...),
		Server: b.serverName,
	})
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			b.clients.reset()
		}
		return err
	}
	return nil
}

func (b *logBridge) publish(ctx context.Context, entry *controlv1.LogEntry) {
	if entry == nil {
		return
	}

	params := &mcp.LoggingMessageParams{
		Logger: entry.GetLogger(),
		Level:  mapProtoLogLevel(entry.GetLevel()),
		Data:   json.RawMessage(entry.GetDataJson()),
	}

	for session := range b.server.Sessions() {
		_ = session.Log(ctx, params)
	}
}

func mapProtoLogLevel(level controlv1.LogLevel) mcp.LoggingLevel {
	switch level {
	case controlv1.LogLevel_LOG_LEVEL_UNSPECIFIED:
		fallthrough
	case controlv1.LogLevel_LOG_LEVEL_INFO:
		return "info"
	case controlv1.LogLevel_LOG_LEVEL_NOTICE:
		return "notice"
	case controlv1.LogLevel_LOG_LEVEL_WARNING:
		return "warning"
	case controlv1.LogLevel_LOG_LEVEL_ERROR:
		return "error"
	case controlv1.LogLevel_LOG_LEVEL_CRITICAL:
		return "critical"
	case controlv1.LogLevel_LOG_LEVEL_ALERT:
		return "alert"
	case controlv1.LogLevel_LOG_LEVEL_EMERGENCY:
		return "emergency"
	case controlv1.LogLevel_LOG_LEVEL_DEBUG:
		fallthrough
	default:
		return "debug"
	}
}
