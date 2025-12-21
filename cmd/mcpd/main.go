package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"mcpd/internal/app"
)

type serveOptions struct {
	configPath string
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()

	root := newRootCmd(logger)
	if err := root.Execute(); err != nil {
		logger.Fatal("command failed", zap.Error(err))
	}
}

func newRootCmd(logger *zap.Logger) *cobra.Command {
	opts := serveOptions{
		configPath: "catalog.yaml",
	}

	root := &cobra.Command{
		Use:   "mcpd",
		Short: "Elastic MCP server orchestrator with scale-to-zero runtime",
	}

	root.PersistentFlags().StringVar(&opts.configPath, "config", opts.configPath, "path to catalog config file")

	root.AddCommand(
		newServeCmd(logger, &opts),
		newValidateCmd(logger, &opts),
	)

	return root
}

func newServeCmd(logger *zap.Logger, opts *serveOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP orchestrator",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalAwareContext(cmd.Context())
			defer cancel()

			application := app.New(logger)
			return application.Serve(ctx, app.ServeConfig{
				ConfigPath: opts.configPath,
			})
		},
	}

	return cmd
}

func newValidateCmd(logger *zap.Logger, opts *serveOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate catalog configuration without running servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			application := app.New(logger)
			return application.ValidateConfig(cmd.Context(), app.ValidateConfig{
				ConfigPath: opts.configPath,
			})
		},
	}

	return cmd
}

func signalAwareContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		defer signal.Stop(signals)
		select {
		case <-signals:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}
