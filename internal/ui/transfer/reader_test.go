package transfer

import (
	"os"
	"path/filepath"
	"testing"

	"mcpv/internal/domain"
)

func TestReadSourceClaudeStdio(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".claude.json")
	writeFile(t, path, `{
  "mcpServers": {
    "alpha": {
      "command": "node",
      "args": ["server.js"],
      "env": {"API_KEY": "test"},
      "cwd": "/tmp"
    }
  }
}`)

	result, err := ReadSource(SourceClaude)
	if err != nil {
		t.Fatalf("ReadSource error: %v", err)
	}
	if len(result.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(result.Servers))
	}
	spec := result.Servers[0]
	if spec.Name != "alpha" {
		t.Fatalf("expected name alpha, got %q", spec.Name)
	}
	if spec.Transport != domain.TransportStdio {
		t.Fatalf("expected stdio transport, got %q", spec.Transport)
	}
	if len(spec.Cmd) != 2 || spec.Cmd[0] != "node" || spec.Cmd[1] != "server.js" {
		t.Fatalf("unexpected cmd: %#v", spec.Cmd)
	}
	if spec.Env["API_KEY"] != "test" {
		t.Fatalf("expected env API_KEY")
	}
	if spec.Cwd != "/tmp" {
		t.Fatalf("expected cwd /tmp, got %q", spec.Cwd)
	}
}

func TestReadSourceClaudeHTTP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".claude.json")
	writeFile(t, path, `{
  "mcpServers": {
    "web": {
      "transport": "streamable_http",
      "endpoint": "https://example.com/mcp",
      "headers": {"Authorization": "Bearer token"},
      "maxRetries": 2
    }
  }
}`)

	result, err := ReadSource(SourceClaude)
	if err != nil {
		t.Fatalf("ReadSource error: %v", err)
	}
	if len(result.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(result.Servers))
	}
	spec := result.Servers[0]
	if spec.Transport != domain.TransportStreamableHTTP {
		t.Fatalf("expected streamable_http transport, got %q", spec.Transport)
	}
	if spec.HTTP == nil || spec.HTTP.Endpoint != "https://example.com/mcp" {
		t.Fatalf("missing http endpoint")
	}
	if spec.HTTP.MaxRetries != 2 {
		t.Fatalf("expected maxRetries 2, got %d", spec.HTTP.MaxRetries)
	}
}

func TestReadSourceGemini(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".gemini", "settings.json")
	writeFile(t, path, `{
  "mcpServers": {
    "gem": {
      "type": "http",
      "url": "https://example.com/gemini"
    }
  }
}`)

	result, err := ReadSource(SourceGemini)
	if err != nil {
		t.Fatalf("ReadSource error: %v", err)
	}
	if len(result.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(result.Servers))
	}
	if result.Servers[0].Transport != domain.TransportStreamableHTTP {
		t.Fatalf("expected streamable_http transport")
	}
}

func TestReadSourceCodexToml(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".codex", "config.toml")
	writeFile(t, path, `
[mcp_servers.alpha]
command = "node"
args = ["index.js"]

[mcp_servers.beta]
type = "http"
url = "https://example.com/mcp"

[mcp_servers.beta.http_headers]
Authorization = "Bearer token"
`)

	result, err := ReadSource(SourceCodex)
	if err != nil {
		t.Fatalf("ReadSource error: %v", err)
	}
	if len(result.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(result.Servers))
	}
}

func TestReadSourceCodexLegacyDuplicate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".codex", "config.toml")
	writeFile(t, path, `
[mcp_servers.alpha]
command = "node"

[mcp.servers.alpha]
command = "python"
`)

	result, err := ReadSource(SourceCodex)
	if err != nil {
		t.Fatalf("ReadSource error: %v", err)
	}
	if len(result.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(result.Servers))
	}
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Kind != IssueDuplicate {
		t.Fatalf("expected duplicate issue, got %q", result.Issues[0].Kind)
	}
}

func TestReadSourceInvalidEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".claude.json")
	writeFile(t, path, `{
  "mcpServers": {
    "broken": {
      "transport": "streamable_http"
    }
  }
}`)

	result, err := ReadSource(SourceClaude)
	if err != nil {
		t.Fatalf("ReadSource error: %v", err)
	}
	if len(result.Servers) != 0 {
		t.Fatalf("expected 0 servers, got %d", len(result.Servers))
	}
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Kind != IssueInvalid {
		t.Fatalf("expected invalid issue, got %q", result.Issues[0].Kind)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
