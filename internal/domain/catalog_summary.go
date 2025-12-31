package domain

import (
	"errors"
	"fmt"
)

type CatalogProfile struct {
	Profile  Profile
	SpecKeys map[string]string
}

type CatalogSummary struct {
	Profiles        map[string]CatalogProfile
	SpecRegistry    map[string]ServerSpec
	TotalServers    int
	MinPingInterval int
	DefaultRuntime  RuntimeConfig
}

func BuildCatalogSummary(store ProfileStore) (CatalogSummary, error) {
	if len(store.Profiles) == 0 {
		return CatalogSummary{}, errors.New("no profiles loaded")
	}

	defaultProfile, ok := store.Profiles[DefaultProfileName]
	if !ok {
		return CatalogSummary{}, fmt.Errorf("default profile %q not found", DefaultProfileName)
	}

	summary := CatalogSummary{
		Profiles:        make(map[string]CatalogProfile, len(store.Profiles)),
		SpecRegistry:    make(map[string]ServerSpec),
		TotalServers:    0,
		MinPingInterval: 0,
		DefaultRuntime:  defaultProfile.Catalog.Runtime,
	}

	for name, profile := range store.Profiles {
		if err := validateSharedRuntime(summary.DefaultRuntime, profile.Catalog.Runtime); err != nil {
			return CatalogSummary{}, fmt.Errorf("profile %q: %w", name, err)
		}

		enabledSpecs, enabledCount := filterEnabledSpecs(profile.Catalog.Specs)
		runtimeProfile := profile
		runtimeProfile.Catalog.Specs = enabledSpecs

		specKeys, err := buildSpecKeys(runtimeProfile.Catalog.Specs)
		if err != nil {
			return CatalogSummary{}, fmt.Errorf("profile %q: %w", name, err)
		}
		summary.Profiles[name] = CatalogProfile{
			Profile:  runtimeProfile,
			SpecKeys: specKeys,
		}
		summary.TotalServers += enabledCount
		if profile.Catalog.Runtime.PingIntervalSeconds > 0 {
			if summary.MinPingInterval == 0 || profile.Catalog.Runtime.PingIntervalSeconds < summary.MinPingInterval {
				summary.MinPingInterval = profile.Catalog.Runtime.PingIntervalSeconds
			}
		}

		for serverType, spec := range runtimeProfile.Catalog.Specs {
			specKey := specKeys[serverType]
			if specKey == "" {
				return CatalogSummary{}, fmt.Errorf("profile %q: missing spec key for %q", name, serverType)
			}
			if _, ok := summary.SpecRegistry[specKey]; ok {
				continue
			}
			summary.SpecRegistry[specKey] = spec
		}
	}

	return summary, nil
}

func filterEnabledSpecs(specs map[string]ServerSpec) (map[string]ServerSpec, int) {
	if len(specs) == 0 {
		return map[string]ServerSpec{}, 0
	}

	enabled := make(map[string]ServerSpec, len(specs))
	count := 0
	for name, spec := range specs {
		if spec.Disabled {
			continue
		}
		enabled[name] = spec
		count++
	}
	return enabled, count
}

func buildSpecKeys(specs map[string]ServerSpec) (map[string]string, error) {
	keys := make(map[string]string, len(specs))
	for serverType, spec := range specs {
		specKey, err := SpecFingerprint(spec)
		if err != nil {
			return nil, fmt.Errorf("spec fingerprint for %q: %w", serverType, err)
		}
		keys[serverType] = specKey
	}
	return keys, nil
}

func validateSharedRuntime(base RuntimeConfig, current RuntimeConfig) error {
	if base.RPC != current.RPC {
		return errors.New("rpc config must match across profiles")
	}
	if base.Observability != current.Observability {
		return errors.New("observability config must match across profiles")
	}
	return nil
}
