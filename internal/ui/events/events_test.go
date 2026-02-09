package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMustMarshalLogData(t *testing.T) {
	empty := mustMarshalLogData(map[string]any{})
	if empty != nil {
		t.Fatalf("expected nil for empty map, got %s", string(empty))
	}

	good := mustMarshalLogData(map[string]any{"ok": "yes"})
	if len(good) == 0 {
		t.Fatal("expected JSON output for non-empty map")
	}
	var decoded map[string]string
	if err := json.Unmarshal(good, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if decoded["ok"] != "yes" {
		t.Fatalf("expected ok=yes, got %s", decoded["ok"])
	}

	invalid := mustMarshalLogData(map[string]any{"bad": make(chan int)})
	if invalid != nil {
		t.Fatalf("expected nil for unmarshalable data, got %s", string(invalid))
	}
}

func TestFormatTimestampUsesUTC(t *testing.T) {
	loc := time.FixedZone("offset", -7*60*60)
	input := time.Date(2024, time.March, 10, 15, 4, 5, 123456789, loc)

	formatted := formatTimestamp(input)
	expected := input.UTC().Format(time.RFC3339Nano)
	if formatted != expected {
		t.Fatalf("expected %s, got %s", expected, formatted)
	}
}
