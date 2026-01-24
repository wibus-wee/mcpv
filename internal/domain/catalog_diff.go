package domain

import (
	"reflect"
	"sort"
)

// CatalogDiff summarizes changes between catalog states.
type CatalogDiff struct {
	AddedSpecKeys    []string
	RemovedSpecKeys  []string
	ReplacedSpecKeys []string
	UpdatedSpecKeys  []string
	TagsChanged      bool
	RuntimeChanged   bool
}

// IsEmpty reports whether the diff contains any changes.
func (d CatalogDiff) IsEmpty() bool {
	return len(d.AddedSpecKeys) == 0 &&
		len(d.RemovedSpecKeys) == 0 &&
		len(d.ReplacedSpecKeys) == 0 &&
		len(d.UpdatedSpecKeys) == 0 &&
		!d.TagsChanged &&
		!d.RuntimeChanged
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
