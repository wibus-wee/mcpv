package loader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/catalog/normalizer"
	"mcpv/internal/infra/catalog/validator"
)

type Loader struct {
	logger *zap.Logger
}

func NewLoader(logger *zap.Logger) *Loader {
	if logger == nil {
		return &Loader{logger: zap.NewNop()}
	}
	return &Loader{logger: logger.Named("catalog")}
}

// LoadRuntimeConfig loads only the runtime section from a config file.
// This is intended for profile-store level runtime defaults.
func (l *Loader) LoadRuntimeConfig(_ context.Context, path string) (domain.RuntimeConfig, error) {
	if path == "" {
		return domain.RuntimeConfig{}, errors.New("config path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return domain.RuntimeConfig{}, fmt.Errorf("read config: %w", err)
	}

	expanded, missing, err := expandConfigEnv(data)
	if err != nil {
		return domain.RuntimeConfig{}, err
	}
	if len(missing) > 0 {
		l.logger.Warn("missing environment variables in runtime config", zap.String("path", path), zap.Strings("missing", missing))
	}

	rawCfg, err := decodeRuntimeConfig(expanded)
	if err != nil {
		return domain.RuntimeConfig{}, err
	}

	runtime, errs := normalizer.NormalizeRuntimeConfig(rawCfg)
	if len(errs) > 0 {
		return domain.RuntimeConfig{}, errors.New(strings.Join(errs, "; "))
	}
	return runtime, nil
}

func (l *Loader) Load(ctx context.Context, path string) (domain.Catalog, error) {
	if path == "" {
		return domain.Catalog{}, errors.New("config path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Catalog{}, fmt.Errorf("read config: %w", err)
	}

	expanded, missing, err := expandConfigEnv(data)
	if err != nil {
		return domain.Catalog{}, err
	}
	if len(missing) > 0 {
		l.logger.Warn("missing environment variables in config", zap.String("path", path), zap.Strings("missing", missing))
	}

	if err := validator.ValidateCatalogSchema(expanded); err != nil {
		return domain.Catalog{}, err
	}

	cfg, err := decodeCatalog(expanded)
	if err != nil {
		return domain.Catalog{}, err
	}

	if err := ctx.Err(); err != nil {
		return domain.Catalog{}, err
	}

	specs := make(map[string]domain.ServerSpec, len(cfg.Servers))
	var validationErrors []string
	nameSeen := make(map[string]struct{})
	runtime, runtimeErrs := normalizer.NormalizeRuntimeConfig(cfg.RawRuntimeConfig)
	validationErrors = append(validationErrors, runtimeErrs...)
	plugins, pluginErrs := normalizer.NormalizePluginSpecs(cfg.Plugins)
	validationErrors = append(validationErrors, pluginErrs...)

	for i, spec := range cfg.Servers {
		normalized, implicitHTTP := normalizer.NormalizeServerSpec(spec)
		if implicitHTTP {
			l.logger.Warn("server transport inferred from http config; consider setting transport explicitly",
				zap.String("server", normalized.Name),
				zap.Int("index", i),
			)
		}
		if _, exists := nameSeen[normalized.Name]; exists {
			validationErrors = append(validationErrors, fmt.Sprintf("servers[%d]: duplicate name %q", i, normalized.Name))
		} else if normalized.Name != "" {
			nameSeen[normalized.Name] = struct{}{}
		}

		if errs := validator.ValidateServerSpec(normalized, i); len(errs) > 0 {
			validationErrors = append(validationErrors, errs...)
			continue
		}

		specs[normalized.Name] = normalized
	}

	if len(validationErrors) > 0 {
		return domain.Catalog{}, errors.New(strings.Join(validationErrors, "; "))
	}

	normalizer.ApplyRuntimeProxyToSpecs(runtime, specs)

	return domain.Catalog{
		Specs:   specs,
		Plugins: plugins,
		Runtime: runtime,
	}, nil
}
