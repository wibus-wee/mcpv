package registry

import "sort"

func applySpecDelta(counts map[string]int, add []string, remove []string) {
	for _, key := range add {
		counts[key]++
	}
	for _, key := range remove {
		count := counts[key]
		switch {
		case count <= 1:
			delete(counts, key)
		default:
			counts[key] = count - 1
		}
	}
}

func sameKeySet(a []string, b []string) bool {
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

func diffKeys(next []string, prev []string) ([]string, []string) {
	nextSet := make(map[string]struct{}, len(next))
	for _, key := range next {
		nextSet[key] = struct{}{}
	}
	prevSet := make(map[string]struct{}, len(prev))
	for _, key := range prev {
		prevSet[key] = struct{}{}
	}
	var toActivate []string
	for key := range nextSet {
		if _, ok := prevSet[key]; !ok {
			toActivate = append(toActivate, key)
		}
	}
	var toDeactivate []string
	for key := range prevSet {
		if _, ok := nextSet[key]; !ok {
			toDeactivate = append(toDeactivate, key)
		}
	}
	return toActivate, toDeactivate
}

func filterOverlap(activate []string, deactivate []string) ([]string, []string) {
	if len(activate) == 0 || len(deactivate) == 0 {
		return activate, deactivate
	}
	deactivateSet := make(map[string]struct{}, len(deactivate))
	for _, key := range deactivate {
		deactivateSet[key] = struct{}{}
	}
	filteredActivate := make([]string, 0, len(activate))
	for _, key := range activate {
		if _, ok := deactivateSet[key]; ok {
			delete(deactivateSet, key)
			continue
		}
		filteredActivate = append(filteredActivate, key)
	}
	filteredDeactivate := make([]string, 0, len(deactivateSet))
	for _, key := range deactivate {
		if _, ok := deactivateSet[key]; ok {
			filteredDeactivate = append(filteredDeactivate, key)
		}
	}
	return filteredActivate, filteredDeactivate
}

func diffCounts(oldCounts map[string]int, newCounts map[string]int) ([]string, []string) {
	var starts []string
	var stops []string

	for key, count := range newCounts {
		if count > 0 && oldCounts[key] == 0 {
			starts = append(starts, key)
		}
	}
	for key, count := range oldCounts {
		if count > 0 && newCounts[key] == 0 {
			stops = append(stops, key)
		}
	}
	return starts, stops
}

func copyCounts(src map[string]int) map[string]int {
	dst := make(map[string]int, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func keysFromSet(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
