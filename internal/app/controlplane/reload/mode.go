package reload

import "mcpv/internal/domain"

func ResolveMode(mode domain.ReloadMode) domain.ReloadMode {
	if mode == "" {
		return domain.DefaultReloadMode
	}
	return mode
}
