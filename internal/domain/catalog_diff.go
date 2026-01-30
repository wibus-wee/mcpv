package domain

import (
	"reflect"
	"sort"
)

// CatalogDiff summarizes changes between catalog states.
type CatalogDiff struct {
	AddedSpecKeys           []string
	RemovedSpecKeys         []string
	ReplacedSpecKeys        []string
	UpdatedSpecKeys         []string
	ToolsOnlySpecKeys       []string
	RestartRequiredSpecKeys []string
	TagsChanged             bool
	RuntimeChanged          bool
}

// IsEmpty reports whether the diff contains any changes.
func (d CatalogDiff) IsEmpty() bool {
	return len(d.AddedSpecKeys) == 0 &&
		len(d.RemovedSpecKeys) == 0 &&
		len(d.ReplacedSpecKeys) == 0 &&
		len(d.UpdatedSpecKeys) == 0 &&
		len(d.ToolsOnlySpecKeys) == 0 &&
		len(d.RestartRequiredSpecKeys) == 0 &&
		!d.TagsChanged &&
		!d.RuntimeChanged
}

// HasSpecChanges reports whether any spec keys changed.
func (d CatalogDiff) HasSpecChanges() bool {
	return len(d.AddedSpecKeys) > 0 ||
		len(d.RemovedSpecKeys) > 0 ||
		len(d.ReplacedSpecKeys) > 0 ||
		len(d.UpdatedSpecKeys) > 0
}

// IsRuntimeOnly reports whether only runtime-level settings changed.
func (d CatalogDiff) IsRuntimeOnly() bool {
	return d.RuntimeChanged && !d.HasSpecChanges()
}

// SpecDiffClassification describes how spec changes should be applied.
type SpecDiffClassification string

const (
	SpecDiffNone            SpecDiffClassification = "none"
	SpecDiffToolsOnly       SpecDiffClassification = "tools_only"
	SpecDiffRestartRequired SpecDiffClassification = "restart_required"
)

// ClassifySpecDiff determines how a spec change should be applied.
func ClassifySpecDiff(prevSpec, nextSpec ServerSpec) SpecDiffClassification {
	if reflect.DeepEqual(prevSpec, nextSpec) {
		return SpecDiffNone
	}
	if nonToolSpecEquals(prevSpec, nextSpec) {
		return SpecDiffToolsOnly
	}
	return SpecDiffRestartRequired
}

// DiffCatalogStates computes a diff between two catalog states.
func DiffCatalogStates(prev CatalogState, next CatalogState) CatalogDiff {
	diff := CatalogDiff{}
	diff.RuntimeChanged = !reflect.DeepEqual(prev.Summary.Runtime, next.Summary.Runtime)

	prevSpecs := prev.Summary.SpecRegistry
	nextSpecs := next.Summary.SpecRegistry

	for specKey, prevSpec := range prevSpecs {
		nextSpec, ok := nextSpecs[specKey]
		if !ok {
			diff.RemovedSpecKeys = append(diff.RemovedSpecKeys, specKey)
			continue
		}
		if !reflect.DeepEqual(prevSpec, nextSpec) {
			diff.UpdatedSpecKeys = append(diff.UpdatedSpecKeys, specKey)
			switch ClassifySpecDiff(prevSpec, nextSpec) {
			case SpecDiffToolsOnly:
				diff.ToolsOnlySpecKeys = append(diff.ToolsOnlySpecKeys, specKey)
			case SpecDiffRestartRequired:
				diff.RestartRequiredSpecKeys = append(diff.RestartRequiredSpecKeys, specKey)
			case SpecDiffNone:
				// Should not occur since we checked !reflect.DeepEqual above
			}
		}
	}
	for specKey := range nextSpecs {
		if _, ok := prevSpecs[specKey]; !ok {
			diff.AddedSpecKeys = append(diff.AddedSpecKeys, specKey)
		}
	}

	replaced := make(map[string]struct{})
	for name, prevKey := range prev.Summary.ServerSpecKeys {
		nextKey, ok := next.Summary.ServerSpecKeys[name]
		if !ok || prevKey == "" || nextKey == "" {
			continue
		}
		if nextKey != prevKey {
			replaced[prevKey] = struct{}{}
		}
		prevSpec := prev.Catalog.Specs[name]
		nextSpec := next.Catalog.Specs[name]
		if !tagsEqual(prevSpec.Tags, nextSpec.Tags) {
			diff.TagsChanged = true
		}
	}
	diff.ReplacedSpecKeys = keysFromSet(replaced)

	sort.Strings(diff.AddedSpecKeys)
	sort.Strings(diff.RemovedSpecKeys)
	sort.Strings(diff.ReplacedSpecKeys)
	sort.Strings(diff.UpdatedSpecKeys)
	sort.Strings(diff.ToolsOnlySpecKeys)
	sort.Strings(diff.RestartRequiredSpecKeys)

	return diff
}

func tagsEqual(a []string, b []string) bool {
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

func nonToolSpecEquals(a ServerSpec, b ServerSpec) bool {
	a = stripToolSpecFields(a)
	b = stripToolSpecFields(b)
	return reflect.DeepEqual(a, b)
}

func stripToolSpecFields(spec ServerSpec) ServerSpec {
	spec.Name = ""
	spec.Tags = nil
	spec.ExposeTools = nil
	return spec
}

func keysFromSet(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
