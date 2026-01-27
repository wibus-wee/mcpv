package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpecFingerprint_StableAcrossName(t *testing.T) {
	base := ServerSpec{
		Name:            "svc-a",
		Cmd:             []string{"./svc"},
		Env:             map[string]string{"A": "1"},
		Cwd:             "/tmp",
		IdleSeconds:     10,
		MaxConcurrent:   2,
		Strategy:        StrategyStateful,
		MinReady:        1,
		ProtocolVersion: DefaultProtocolVersion,
		ExposeTools:     []string{"tool-b", "tool-a"},
	}
	other := base
	other.Name = "svc-b"

	baseKey := SpecFingerprint(base)
	otherKey := SpecFingerprint(other)
	require.Equal(t, baseKey, otherKey)
}

func TestSpecFingerprint_DifferentSpec(t *testing.T) {
	base := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		IdleSeconds:     10,
		MaxConcurrent:   1,
		ProtocolVersion: DefaultProtocolVersion,
	}
	changed := base
	changed.Cmd = []string{"./svc", "--flag"}

	baseKey := SpecFingerprint(base)
	changedKey := SpecFingerprint(changed)
	require.NotEqual(t, baseKey, changedKey)
}

func TestSpecFingerprint_CwdAffectsFingerprint(t *testing.T) {
	base := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		Cwd:             "/tmp/a",
		ProtocolVersion: DefaultProtocolVersion,
	}
	changed := base
	changed.Cwd = "/tmp/b"

	baseKey := SpecFingerprint(base)
	changedKey := SpecFingerprint(changed)
	require.NotEqual(t, baseKey, changedKey)
}

func TestSpecFingerprint_EnvOrderIndependent(t *testing.T) {
	specA := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		Env:             map[string]string{"B": "2", "A": "1"},
		ProtocolVersion: DefaultProtocolVersion,
	}
	specB := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		Env:             map[string]string{"A": "1", "B": "2"},
		ProtocolVersion: DefaultProtocolVersion,
	}

	keyA := SpecFingerprint(specA)
	keyB := SpecFingerprint(specB)
	require.Equal(t, keyA, keyB)
}

func TestSpecFingerprint_EnvChangeAffectsFingerprint(t *testing.T) {
	specA := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		Env:             map[string]string{"A": "1"},
		ProtocolVersion: DefaultProtocolVersion,
	}
	specB := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		Env:             map[string]string{"A": "2"},
		ProtocolVersion: DefaultProtocolVersion,
	}

	keyA := SpecFingerprint(specA)
	keyB := SpecFingerprint(specB)
	require.NotEqual(t, keyA, keyB)
}

func TestSpecFingerprint_IgnoresSchedulerFields(t *testing.T) {
	base := ServerSpec{
		Name:                "svc",
		Cmd:                 []string{"./svc"},
		Env:                 map[string]string{"A": "1"},
		ProtocolVersion:     DefaultProtocolVersion,
		IdleSeconds:         10,
		MaxConcurrent:       1,
		MinReady:            1,
		ExposeTools:         []string{"tool-a"},
		DrainTimeoutSeconds: 5,
	}
	changed := base
	changed.IdleSeconds = 20
	changed.MaxConcurrent = 2
	changed.MinReady = 3
	changed.ExposeTools = []string{"tool-b"}
	changed.DrainTimeoutSeconds = 15

	baseKey := SpecFingerprint(base)
	changedKey := SpecFingerprint(changed)
	require.Equal(t, baseKey, changedKey)
}

func TestSpecFingerprint_EmptyEnvStable(t *testing.T) {
	specA := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		Env:             nil,
		ProtocolVersion: DefaultProtocolVersion,
	}
	specB := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		Env:             map[string]string{},
		ProtocolVersion: DefaultProtocolVersion,
	}

	keyA := SpecFingerprint(specA)
	keyB := SpecFingerprint(specB)
	require.Equal(t, keyA, keyB)
}

func TestSpecFingerprint_StreamableHTTPDiffEndpoint(t *testing.T) {
	specA := ServerSpec{
		Name:            "svc",
		Transport:       TransportStreamableHTTP,
		ProtocolVersion: DefaultStreamableHTTPProtocolVersion,
		HTTP: &StreamableHTTPConfig{
			Endpoint:   "https://example.com/mcp",
			MaxRetries: 5,
		},
	}
	specB := specA
	specB.HTTP = &StreamableHTTPConfig{
		Endpoint:   "https://example.com/other",
		MaxRetries: 5,
	}

	keyA := SpecFingerprint(specA)
	keyB := SpecFingerprint(specB)
	require.NotEqual(t, keyA, keyB)
}

func TestSpecFingerprint_StreamableHTTPHeaderOrder(t *testing.T) {
	specA := ServerSpec{
		Name:            "svc",
		Transport:       TransportStreamableHTTP,
		ProtocolVersion: DefaultStreamableHTTPProtocolVersion,
		HTTP: &StreamableHTTPConfig{
			Endpoint: "https://example.com/mcp",
			Headers: map[string]string{
				"Authorization": "Bearer a",
				"X-Test":        "1",
			},
			MaxRetries: 5,
		},
	}
	specB := specA
	specB.HTTP = &StreamableHTTPConfig{
		Endpoint: "https://example.com/mcp",
		Headers: map[string]string{
			"X-Test":        "1",
			"Authorization": "Bearer a",
		},
		MaxRetries: 5,
	}

	keyA := SpecFingerprint(specA)
	keyB := SpecFingerprint(specB)
	require.Equal(t, keyA, keyB)
}
