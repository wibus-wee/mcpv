//go:build wireinject
// +build wireinject

package app

import (
	"context"

	"github.com/google/wire"
)

func InitializeApplication(ctx context.Context, cfg ServeConfig, logging LoggingConfig) (*Application, error) {
	wire.Build(AppSet)
	return nil, nil
}
