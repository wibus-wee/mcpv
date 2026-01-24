package domain

import "fmt"

// CatalogSummary aggregates catalog metadata.
type CatalogSummary struct {
	SpecRegistry   map[string]ServerSpec
	ServerSpecKeys map[string]string
	TotalServers   int
	Runtime        RuntimeConfig
}

// BuildCatalogSummary computes a summary view of the catalog.
func BuildCatalogSummary(catalog Catalog) (CatalogSummary, error) {
	summary := CatalogSummary{
		SpecRegistry:   make(map[string]ServerSpec),
		ServerSpecKeys: make(map[string]string),
		TotalServers:   0,
		Runtime:        catalog.Runtime,
	}

	for name, spec := range catalog.Specs {
		if spec.Disabled {
			continue
		}
		specKey, err := SpecFingerprint(spec)
		if err != nil {
			return CatalogSummary{}, fmt.Errorf("spec fingerprint for %q: %w", name, err)
		}
		summary.ServerSpecKeys[name] = specKey
		if _, ok := summary.SpecRegistry[specKey]; !ok {
			summary.SpecRegistry[specKey] = spec
		}
		summary.TotalServers++
	}

	return summary, nil
}
