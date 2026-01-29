package runtime

import "sort"

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
