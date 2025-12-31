package domain

import (
	"reflect"
	"sort"
)

type CatalogDiff struct {
	AddedSpecKeys    []string
	RemovedSpecKeys  []string
	ReplacedSpecKeys []string
	UpdatedSpecKeys  []string
	AddedProfiles    []string
	RemovedProfiles  []string
	UpdatedProfiles  []string
	CallersChanged   bool
}

func (d CatalogDiff) IsEmpty() bool {
	return len(d.AddedSpecKeys) == 0 &&
		len(d.RemovedSpecKeys) == 0 &&
		len(d.ReplacedSpecKeys) == 0 &&
		len(d.UpdatedSpecKeys) == 0 &&
		len(d.AddedProfiles) == 0 &&
		len(d.RemovedProfiles) == 0 &&
		len(d.UpdatedProfiles) == 0 &&
		!d.CallersChanged
}

func DiffCatalogStates(prev CatalogState, next CatalogState) CatalogDiff {
	diff := CatalogDiff{}
	diff.CallersChanged = !reflect.DeepEqual(prev.Store.Callers, next.Store.Callers)

	prevProfiles := prev.Summary.Profiles
	nextProfiles := next.Summary.Profiles

	for name, prevProfile := range prevProfiles {
		nextProfile, ok := nextProfiles[name]
		if !ok {
			diff.RemovedProfiles = append(diff.RemovedProfiles, name)
			continue
		}
		if !reflect.DeepEqual(prevProfile, nextProfile) {
			diff.UpdatedProfiles = append(diff.UpdatedProfiles, name)
		}
	}
	for name := range nextProfiles {
		if _, ok := prevProfiles[name]; !ok {
			diff.AddedProfiles = append(diff.AddedProfiles, name)
		}
	}

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
	for name, prevProfile := range prevProfiles {
		nextProfile, ok := nextProfiles[name]
		if !ok {
			continue
		}
		for serverType, prevKey := range prevProfile.SpecKeys {
			if prevKey == "" {
				continue
			}
			nextKey, ok := nextProfile.SpecKeys[serverType]
			if !ok || nextKey == "" {
				continue
			}
			if nextKey != prevKey {
				replaced[prevKey] = struct{}{}
			}
		}
	}
	diff.ReplacedSpecKeys = keysFromSet(replaced)

	sort.Strings(diff.AddedSpecKeys)
	sort.Strings(diff.RemovedSpecKeys)
	sort.Strings(diff.ReplacedSpecKeys)
	sort.Strings(diff.UpdatedSpecKeys)
	sort.Strings(diff.AddedProfiles)
	sort.Strings(diff.RemovedProfiles)
	sort.Strings(diff.UpdatedProfiles)

	return diff
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
