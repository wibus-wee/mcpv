package catalog

import "mcpd/internal/domain"

func parseSocketMode(value string) (uint32, error) {
	return domain.ParseSocketMode(value)
}
