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
		Sticky:          true,
		Persistent:      false,
		MinReady:        1,
		ProtocolVersion: DefaultProtocolVersion,
		ExposeTools:     []string{"tool-b", "tool-a"},
	}
	other := base
	other.Name = "svc-b"

	baseKey, err := SpecFingerprint(base)
	require.NoError(t, err)
	otherKey, err := SpecFingerprint(other)
	require.NoError(t, err)
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

	baseKey, err := SpecFingerprint(base)
	require.NoError(t, err)
	changedKey, err := SpecFingerprint(changed)
	require.NoError(t, err)
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

	keyA, err := SpecFingerprint(specA)
	require.NoError(t, err)
	keyB, err := SpecFingerprint(specB)
	require.NoError(t, err)
	require.Equal(t, keyA, keyB)
}

func TestSpecFingerprint_ExposeToolsOrderIndependent(t *testing.T) {
	specA := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		ExposeTools:     []string{"tool-b", "tool-a"},
		ProtocolVersion: DefaultProtocolVersion,
	}
	specB := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		ExposeTools:     []string{"tool-a", "tool-b"},
		ProtocolVersion: DefaultProtocolVersion,
	}

	keyA, err := SpecFingerprint(specA)
	require.NoError(t, err)
	keyB, err := SpecFingerprint(specB)
	require.NoError(t, err)
	require.Equal(t, keyA, keyB)
}

func TestSpecFingerprint_EmptySlicesStable(t *testing.T) {
	specA := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		Env:             nil,
		ExposeTools:     nil,
		ProtocolVersion: DefaultProtocolVersion,
	}
	specB := ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		Env:             map[string]string{},
		ExposeTools:     []string{},
		ProtocolVersion: DefaultProtocolVersion,
	}

	keyA, err := SpecFingerprint(specA)
	require.NoError(t, err)
	keyB, err := SpecFingerprint(specB)
	require.NoError(t, err)
	require.Equal(t, keyA, keyB)
}
