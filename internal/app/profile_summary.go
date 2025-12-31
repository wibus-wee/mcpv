package app

import (
	"errors"
	"fmt"
	"sort"

	"mcpd/internal/domain"
)

type profileConfig struct {
	profile  domain.Profile
	specKeys map[string]string
}

type profileSummary struct {
	configs         map[string]profileConfig
	specRegistry    map[string]domain.ServerSpec
	totalServers    int
	minPingInterval int
	defaultRuntime  domain.RuntimeConfig
}

func buildProfileSummary(store domain.ProfileStore) (profileSummary, error) {
	if len(store.Profiles) == 0 {
		return profileSummary{}, errors.New("no profiles loaded")
	}

	defaultProfile, ok := store.Profiles[domain.DefaultProfileName]
	if !ok {
		return profileSummary{}, fmt.Errorf("default profile %q not found", domain.DefaultProfileName)
	}

	summary := profileSummary{
		configs:         make(map[string]profileConfig, len(store.Profiles)),
		specRegistry:    make(map[string]domain.ServerSpec),
		totalServers:    0,
		minPingInterval: 0,
		defaultRuntime:  defaultProfile.Catalog.Runtime,
	}

	for name, profile := range store.Profiles {
		if err := validateSharedRuntime(summary.defaultRuntime, profile.Catalog.Runtime); err != nil {
			return profileSummary{}, fmt.Errorf("profile %q: %w", name, err)
		}

		enabledSpecs, enabledCount := filterEnabledSpecs(profile.Catalog.Specs)
		runtimeProfile := profile
		runtimeProfile.Catalog.Specs = enabledSpecs

		specKeys, err := buildSpecKeys(runtimeProfile.Catalog.Specs)
		if err != nil {
			return profileSummary{}, fmt.Errorf("profile %q: %w", name, err)
		}
		summary.configs[name] = profileConfig{
			profile:  runtimeProfile,
			specKeys: specKeys,
		}
		summary.totalServers += enabledCount
		if profile.Catalog.Runtime.PingIntervalSeconds > 0 {
			if summary.minPingInterval == 0 || profile.Catalog.Runtime.PingIntervalSeconds < summary.minPingInterval {
				summary.minPingInterval = profile.Catalog.Runtime.PingIntervalSeconds
			}
		}

		for serverType, spec := range runtimeProfile.Catalog.Specs {
			specKey := specKeys[serverType]
			if specKey == "" {
				return profileSummary{}, fmt.Errorf("profile %q: missing spec key for %q", name, serverType)
			}
			if _, ok := summary.specRegistry[specKey]; ok {
				continue
			}
			// Keep the original spec.Name as-is for display purposes
			summary.specRegistry[specKey] = spec
		}
	}

	return summary, nil
}

func filterEnabledSpecs(specs map[string]domain.ServerSpec) (map[string]domain.ServerSpec, int) {
	if len(specs) == 0 {
		return map[string]domain.ServerSpec{}, 0
	}

	enabled := make(map[string]domain.ServerSpec, len(specs))
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

func buildSpecKeys(specs map[string]domain.ServerSpec) (map[string]string, error) {
	keys := make(map[string]string, len(specs))
	for serverType, spec := range specs {
		specKey, err := domain.SpecFingerprint(spec)
		if err != nil {
			return nil, fmt.Errorf("spec fingerprint for %q: %w", serverType, err)
		}
		keys[serverType] = specKey
	}
	return keys, nil
}

func collectSpecKeys(specKeys map[string]string) []string {
	if len(specKeys) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(specKeys))
	for _, key := range specKeys {
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func validateSharedRuntime(base domain.RuntimeConfig, current domain.RuntimeConfig) error {
	if base.RPC != current.RPC {
		return errors.New("rpc config must match across profiles")
	}
	if base.Observability != current.Observability {
		return errors.New("observability config must match across profiles")
	}
	return nil
}
