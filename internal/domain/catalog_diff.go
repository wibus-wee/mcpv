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
	RuntimeBehaviorSpecKeys []string
	RestartRequiredSpecKeys []string
	TagsChanged             bool
	RuntimeChanged          bool
	PluginsChanged          bool
	AddedPlugins            []string
	RemovedPlugins          []string
	UpdatedPlugins          []string
	RuntimeDiff             RuntimeDiff
}

// IsEmpty reports whether the diff contains any changes.
func (d CatalogDiff) IsEmpty() bool {
	return len(d.AddedSpecKeys) == 0 &&
		len(d.RemovedSpecKeys) == 0 &&
		len(d.ReplacedSpecKeys) == 0 &&
		len(d.UpdatedSpecKeys) == 0 &&
		len(d.ToolsOnlySpecKeys) == 0 &&
		len(d.RuntimeBehaviorSpecKeys) == 0 &&
		len(d.RestartRequiredSpecKeys) == 0 &&
		!d.TagsChanged &&
		!d.RuntimeChanged &&
		!d.PluginsChanged &&
		d.RuntimeDiff.IsEmpty()
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
	SpecDiffRuntimeBehavior SpecDiffClassification = "runtime_behavior"
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
	if runtimeBehaviorSpecEquals(prevSpec, nextSpec) {
		return SpecDiffRuntimeBehavior
	}
	return SpecDiffRestartRequired
}

// DiffCatalogStates computes a diff between two catalog states.
func DiffCatalogStates(prev CatalogState, next CatalogState) CatalogDiff {
	diff := CatalogDiff{}
	diff.RuntimeDiff = DiffRuntimeConfig(prev.Summary.Runtime, next.Summary.Runtime)
	diff.RuntimeChanged = !diff.RuntimeDiff.IsEmpty()

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
			case SpecDiffRuntimeBehavior:
				diff.RuntimeBehaviorSpecKeys = append(diff.RuntimeBehaviorSpecKeys, specKey)
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
	sort.Strings(diff.RuntimeBehaviorSpecKeys)
	sort.Strings(diff.RestartRequiredSpecKeys)

	prevPlugins := prev.Summary.PluginIndex
	if prevPlugins == nil {
		prevPlugins = map[string]PluginSpec{}
	}
	nextPlugins := next.Summary.PluginIndex
	if nextPlugins == nil {
		nextPlugins = map[string]PluginSpec{}
	}
	for name, prevPlugin := range prevPlugins {
		nextPlugin, ok := nextPlugins[name]
		if !ok {
			diff.RemovedPlugins = append(diff.RemovedPlugins, name)
			continue
		}
		if !reflect.DeepEqual(prevPlugin, nextPlugin) {
			diff.UpdatedPlugins = append(diff.UpdatedPlugins, name)
		}
	}
	for name := range nextPlugins {
		if _, ok := prevPlugins[name]; !ok {
			diff.AddedPlugins = append(diff.AddedPlugins, name)
		}
	}
	sort.Strings(diff.AddedPlugins)
	sort.Strings(diff.RemovedPlugins)
	sort.Strings(diff.UpdatedPlugins)
	diff.PluginsChanged = len(diff.AddedPlugins)+len(diff.RemovedPlugins)+len(diff.UpdatedPlugins) > 0

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

func runtimeBehaviorSpecEquals(a ServerSpec, b ServerSpec) bool {
	a = stripRuntimeBehaviorFields(stripToolSpecFields(a))
	b = stripRuntimeBehaviorFields(stripToolSpecFields(b))
	return reflect.DeepEqual(a, b)
}

func stripToolSpecFields(spec ServerSpec) ServerSpec {
	spec.Name = ""
	spec.Tags = nil
	spec.ExposeTools = nil
	return spec
}

func stripRuntimeBehaviorFields(spec ServerSpec) ServerSpec {
	spec.IdleSeconds = 0
	spec.MaxConcurrent = 0
	spec.MinReady = 0
	spec.DrainTimeoutSeconds = 0
	spec.ActivationMode = ""
	spec.SessionTTLSeconds = 0
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
