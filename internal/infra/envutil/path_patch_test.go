package envutil

import (
	"os"
	"strings"
	"testing"
)

func TestMergePATHDeduplicates(t *testing.T) {
	sep := string(os.PathListSeparator)
	primary := strings.Join([]string{"/opt/bin", "/usr/bin"}, sep)
	fallback := strings.Join([]string{"/usr/bin", "/bin"}, sep)

	got := mergePATH(primary, fallback)
	want := strings.Join([]string{"/opt/bin", "/usr/bin", "/bin"}, sep)

	if got != want {
		t.Fatalf("mergePATH=%q, want %q", got, want)
	}
}

func TestEnvVarValueReturnsLast(t *testing.T) {
	env := []string{"PATH=/bin", "A=1", "PATH=/usr/bin"}
	got := envVarValue(env, "PATH")
	if got != "/usr/bin" {
		t.Fatalf("envVarValue=%q, want %q", got, "/usr/bin")
	}
}

func TestSetEnvValueReplacesAll(t *testing.T) {
	env := []string{"A=1", "PATH=/bin", "B=2", "PATH=/usr/bin"}
	out := setEnvValue(env, "PATH", "/opt/bin")

	var gotPaths []string
	for _, entry := range out {
		if strings.HasPrefix(entry, "PATH=") {
			gotPaths = append(gotPaths, entry)
		}
	}
	if len(gotPaths) != 1 {
		t.Fatalf("expected 1 PATH entry, got %d", len(gotPaths))
	}
	if gotPaths[0] != "PATH=/opt/bin" {
		t.Fatalf("PATH=%q, want %q", gotPaths[0], "PATH=/opt/bin")
	}
}
