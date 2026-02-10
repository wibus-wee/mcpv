package diagnostics

import "testing"

func TestRedactMap(t *testing.T) {
	input := map[string]string{
		"token": "secret-value",
		"name":  "alpha",
	}
	out := RedactMap(input)
	if out["token"] != "***" {
		t.Fatalf("expected redaction, got %q", out["token"])
	}
	if out["name"] != "alpha" {
		t.Fatalf("expected name preserved, got %q", out["name"])
	}
}
