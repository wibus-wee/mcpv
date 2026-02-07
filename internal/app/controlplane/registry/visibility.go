package registry

func isVisibleToTags(clientTags []string, serverTags []string) bool {
	if len(serverTags) == 0 || len(clientTags) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(clientTags))
	for _, tag := range clientTags {
		set[tag] = struct{}{}
	}
	for _, tag := range serverTags {
		if _, ok := set[tag]; ok {
			return true
		}
	}
	return false
}
