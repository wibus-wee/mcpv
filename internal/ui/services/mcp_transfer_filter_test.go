package services

import (
	"testing"

	"mcpv/internal/domain"
)

func TestFilterTransferSpecs(t *testing.T) {
	specs := []domain.ServerSpec{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "beta"},
		{Name: ""},
	}
	existing := map[string]struct{}{
		"alpha": {},
	}

	filtered, issues := filterTransferSpecs(specs, existing)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 server, got %d", len(filtered))
	}
	if filtered[0].Name != "beta" {
		t.Fatalf("expected beta, got %q", filtered[0].Name)
	}

	kindCounts := map[string]int{}
	for _, issue := range issues {
		kindCounts[issue.Kind]++
	}
	if kindCounts["conflict"] != 1 {
		t.Fatalf("expected 1 conflict issue, got %d", kindCounts["conflict"])
	}
	if kindCounts["duplicate"] != 1 {
		t.Fatalf("expected 1 duplicate issue, got %d", kindCounts["duplicate"])
	}
	if kindCounts["invalid"] != 1 {
		t.Fatalf("expected 1 invalid issue, got %d", kindCounts["invalid"])
	}
}
