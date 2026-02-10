package services

import (
	"testing"
	"time"
)

func TestComputeStuck(t *testing.T) {
	now := time.Now().UTC()
	events := map[string][]diagnosticsEvent{
		"svc": {
			{
				Step:      "transport_connect",
				Phase:     "enter",
				Timestamp: now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
			},
		},
		"fresh": {
			{
				Step:      "initialize_call",
				Phase:     "enter",
				Timestamp: now.Add(-5 * time.Second).Format(time.RFC3339Nano),
			},
		},
	}

	stuck := computeStuck(events, 30*time.Second)
	if _, ok := stuck["svc"]; !ok {
		t.Fatalf("expected svc to be stuck")
	}
	if _, ok := stuck["fresh"]; ok {
		t.Fatalf("expected fresh to be below threshold")
	}
}
