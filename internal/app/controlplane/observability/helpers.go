package observability

func toSpecKeySet(keys []string) map[string]struct{} {
	if len(keys) == 0 {
		return map[string]struct{}{}
	}
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		set[key] = struct{}{}
	}
	return set
}
