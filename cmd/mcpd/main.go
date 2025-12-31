package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"mcpd/internal/app"
)

type serveOptions struct {
	configPath string
	// logStderr  bool
	logger *zap.Logger
}

func main() {
	opts := serveOptions{
		configPath: ".",
		// logStderr:  false,
		logger: zap.NewNop(),
	}

	root := &cobra.Command{
		Use:   "mcpd",
		Short: "Elastic MCP control plane with scale-to-zero runtime",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// if !opts.logStderr {
			// 	opts.logger = zap.NewNop()
			// 	return nil
			// }
			cfg := zap.NewProductionConfig()
			// cfg.OutputPaths = []string{""}
			// cfg.ErrorOutputPaths = []string{"stderr"}
			log, err := cfg.Build()
			if err != nil {
				return err
			}
			// replace logger in options
			opts.logger = log
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			_ = opts.logger.Sync()
		},
	}

	root.PersistentFlags().StringVar(&opts.configPath, "config", opts.configPath, "path to profile store directory")
	// root.PersistentFlags().BoolVar(&opts.logStderr, "log-stderr", opts.logStderr, "enable structured logs to stderr (off by default to avoid stdio noise)")

	root.AddCommand(
		newServeCmd(&opts),
		newValidateCmd(&opts),
	)

	if err := root.Execute(); err != nil {
		opts.logger.Fatal("command failed", zap.Error(err))
	}
}

func newServeCmd(opts *serveOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP control plane",
		RunE: func(cmd *cobra.Command, args []string) error {
			applyFlagBindings(cmd.Flags(), opts)
			ctx, cancel := signalAwareContext(cmd.Context())
			defer cancel()

			application, err := app.InitializeApplication(ctx, app.ServeConfig{
				ConfigPath: opts.configPath,
			}, app.LoggingConfig{Logger: opts.logger})
			if err != nil {
				return err
			}
			return application.Run()
		},
	}

	return cmd
}

func newValidateCmd(opts *serveOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate catalog configuration without running servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			applyFlagBindings(cmd.Flags(), opts)
			application := app.New(opts.logger)
			return application.ValidateConfig(cmd.Context(), app.ValidateConfig{
				ConfigPath: opts.configPath,
			})
		},
	}

	return cmd
}

func applyFlagBindings(flags *pflag.FlagSet, opts *serveOptions) {
	flags.Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "config":
			opts.configPath, _ = flags.GetString("config")
			// case "log-stderr":
			// 	opts.logStderr, _ = flags.GetBool("log-stderr")
		}
	})
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
