package aggregator

import (
	"context"

	"mcpv/internal/domain"
)

type listChangeSubscriber interface {
	Subscribe(ctx context.Context, kind domain.ListChangeKind) <-chan domain.ListChangeEvent
}

func listChangeApplies(specs map[string]domain.ServerSpec, specKeySet map[string]struct{}, event domain.ListChangeEvent) bool {
	if event.ServerType != "" {
		if _, ok := specs[event.ServerType]; ok {
			return true
		}
	}
	if event.SpecKey != "" {
		if _, ok := specKeySet[event.SpecKey]; ok {
			return true
		}
	}
	return false
}

func specKeySet(specKeys map[string]string) map[string]struct{} {
	if len(specKeys) == 0 {
		return map[string]struct{}{}
	}
	set := make(map[string]struct{}, len(specKeys))
	for _, key := range specKeys {
		if key == "" {
			continue
		}
		set[key] = struct{}{}
	}
	return set
}
