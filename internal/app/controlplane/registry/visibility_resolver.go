package registry

import (
	"sort"
	"strings"

	"mcpv/internal/domain"
)

type VisibilityResolver struct {
	state State
}

func NewVisibilityResolver(state State) *VisibilityResolver {
	return &VisibilityResolver{state: state}
}

func (v *VisibilityResolver) VisibleSpecKeys(tags []string, server string) ([]string, int) {
	catalog := v.state.Catalog()
	serverSpecKeys := v.state.ServerSpecKeys()
	return v.VisibleSpecKeysForCatalog(catalog, serverSpecKeys, tags, server)
}

func (v *VisibilityResolver) VisibleSpecKeysForCatalog(catalog domain.Catalog, serverSpecKeys map[string]string, tags []string, server string) ([]string, int) {
	if len(serverSpecKeys) == 0 {
		return nil, 0
	}
	if server != "" {
		specKey, ok := serverSpecKeys[server]
		if !ok {
			return nil, 0
		}
		if _, ok := catalog.Specs[server]; !ok {
			return nil, 0
		}
		return []string{specKey}, 1
	}
	visible := make(map[string]struct{})
	serverCount := 0
	for name, specKey := range serverSpecKeys {
		spec, ok := catalog.Specs[name]
		if !ok {
			continue
		}
		if isVisibleToTags(tags, spec.Tags) {
			serverCount++
			visible[specKey] = struct{}{}
		}
	}
	return keysFromSet(visible), serverCount
}

func (v *VisibilityResolver) NormalizeServerName(server string) string {
	return strings.TrimSpace(server)
}

func (v *VisibilityResolver) NormalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	unique := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		unique[tag] = struct{}{}
	}
	if len(unique) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(unique))
	for tag := range unique {
		normalized = append(normalized, tag)
	}
	sort.Strings(normalized)
	return normalized
}

func (v *VisibilityResolver) TagsEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
