package catalog

import "mcpv/internal/domain"

func parseSocketMode(value string) (uint32, error) {
	return domain.ParseSocketMode(value)
}
