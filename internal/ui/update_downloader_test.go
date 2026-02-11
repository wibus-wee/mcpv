package ui

import "testing"

func TestParseGitHubReleaseURL(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantOK     bool
		wantTag    string
		wantOwner  string
		wantRepo   string
		wantLatest bool
	}{
		{
			name:       "tag url",
			raw:        "https://github.com/acme/app/releases/tag/v1.2.3",
			wantOK:     true,
			wantTag:    "v1.2.3",
			wantOwner:  "acme",
			wantRepo:   "app",
			wantLatest: false,
		},
		{
			name:       "latest url",
			raw:        "https://github.com/acme/app/releases/latest",
			wantOK:     true,
			wantOwner:  "acme",
			wantRepo:   "app",
			wantLatest: true,
		},
		{
			name:   "invalid host",
			raw:    "https://example.com/acme/app/releases/tag/v1.2.3",
			wantOK: false,
		},
		{
			name:   "invalid path",
			raw:    "https://github.com/acme/app/issues/1",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, tag, latest, ok := parseGitHubReleaseURL(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok mismatch: got %v want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Fatalf("owner/repo mismatch: got %s/%s want %s/%s", owner, repo, tt.wantOwner, tt.wantRepo)
			}
			if tag != tt.wantTag {
				t.Fatalf("tag mismatch: got %s want %s", tag, tt.wantTag)
			}
			if latest != tt.wantLatest {
				t.Fatalf("latest mismatch: got %v want %v", latest, tt.wantLatest)
			}
		})
	}
}

func TestSelectReleaseAsset(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "mcpv-1.0.0-macos-universal.dmg", BrowserDownloadURL: "https://example.com/universal"},
		{Name: "mcpv-1.0.0-macos-amd64.dmg", BrowserDownloadURL: "https://example.com/amd64"},
		{Name: "mcpv-1.0.0-macos-arm64.dmg", BrowserDownloadURL: "https://example.com/arm64"},
	}

	t.Run("preferred name", func(t *testing.T) {
		asset, ok := selectReleaseAsset(assets, "mcpv-1.0.0-macos-arm64.dmg", "arm64")
		if !ok {
			t.Fatalf("expected asset")
		}
		if asset.Name != "mcpv-1.0.0-macos-arm64.dmg" {
			t.Fatalf("unexpected asset: %s", asset.Name)
		}
	})

	t.Run("prefers arch-specific", func(t *testing.T) {
		asset, ok := selectReleaseAsset(assets, "", "arm64")
		if !ok {
			t.Fatalf("expected asset")
		}
		if asset.Name != "mcpv-1.0.0-macos-arm64.dmg" {
			t.Fatalf("unexpected asset: %s", asset.Name)
		}
	})

	t.Run("skips universal", func(t *testing.T) {
		asset, ok := selectReleaseAsset(assets, "", "amd64")
		if !ok {
			t.Fatalf("expected asset")
		}
		if asset.Name != "mcpv-1.0.0-macos-amd64.dmg" {
			t.Fatalf("unexpected asset: %s", asset.Name)
		}
	})
}
