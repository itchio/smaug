//go:build linux

package runner

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	"github.com/itchio/headway/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFlatpakSpawnRunnerRequiresBinary(t *testing.T) {
	origLookPath := flatpakSpawnLookPath
	t.Cleanup(func() {
		flatpakSpawnLookPath = origLookPath
	})

	flatpakSpawnLookPath = func(file string) (string, error) {
		assert.Equal(t, "flatpak-spawn", file)
		return "", exec.ErrNotFound
	}

	_, err := newFlatpakSpawnRunner(RunnerParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inside Flatpak")
	assert.Contains(t, err.Error(), "flatpak-spawn")
}

func TestFlatpakSpawnRunnerUsesHostEnvForWrapper(t *testing.T) {
	origCommand := flatpakSpawnCommand
	t.Cleanup(func() {
		flatpakSpawnCommand = origCommand
	})

	var gotName string
	var gotArgs []string
	flatpakSpawnCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "printf %s \"$SMAUG_FLATPAK_WRAPPER_ENV\"")
	}

	t.Setenv("SMAUG_FLATPAK_WRAPPER_ENV", "host")

	var stdout bytes.Buffer
	fr := &flatpakSpawnRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			Env:      []string{"SMAUG_FLATPAK_WRAPPER_ENV=child"},
			Stdout:   &stdout,
		},
		binaryPath: "/fake/flatpak-spawn",
	}

	require.NoError(t, fr.Run())
	assert.Equal(t, "host", stdout.String())
	assert.Equal(t, "/fake/flatpak-spawn", gotName)
	assert.Contains(t, gotArgs, "--env=SMAUG_FLATPAK_WRAPPER_ENV=child")
}

func TestFlatpakSpawnAllowEnvPrefersParamsEnv(t *testing.T) {
	origCommand := flatpakSpawnCommand
	t.Cleanup(func() {
		flatpakSpawnCommand = origCommand
	})

	var gotArgs []string
	flatpakSpawnCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	t.Setenv("SMAUG_ALLOW_ENV_SHARED", "host")

	fr := &flatpakSpawnRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			Env:      []string{"SMAUG_ALLOW_ENV_SHARED=params"},
			SandboxConfig: SandboxConfig{
				AllowEnv: []string{"SMAUG_ALLOW_ENV_SHARED"},
			},
		},
		binaryPath: "/fake/flatpak-spawn",
	}

	require.NoError(t, fr.Run())
	assert.Contains(t, gotArgs, "--env=SMAUG_ALLOW_ENV_SHARED=params")
	assert.NotContains(t, gotArgs, "--env=SMAUG_ALLOW_ENV_SHARED=host")
}

func TestFlatpakSpawnAllowEnvFallsBackToHost(t *testing.T) {
	origCommand := flatpakSpawnCommand
	t.Cleanup(func() {
		flatpakSpawnCommand = origCommand
	})

	var gotArgs []string
	flatpakSpawnCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	t.Setenv("SMAUG_ALLOW_ENV_HOST_ONLY", "host")

	fr := &flatpakSpawnRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			SandboxConfig: SandboxConfig{
				AllowEnv: []string{"SMAUG_ALLOW_ENV_HOST_ONLY"},
			},
		},
		binaryPath: "/fake/flatpak-spawn",
	}

	require.NoError(t, fr.Run())
	assert.Contains(t, gotArgs, "--env=SMAUG_ALLOW_ENV_HOST_ONLY=host")
}

func TestFlatpakSpawnAllowEnvEmptyValueSuppressesHost(t *testing.T) {
	origCommand := flatpakSpawnCommand
	t.Cleanup(func() {
		flatpakSpawnCommand = origCommand
	})

	var gotArgs []string
	flatpakSpawnCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	t.Setenv("SMAUG_ALLOW_ENV_EMPTY", "host")

	fr := &flatpakSpawnRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			Env:      []string{"SMAUG_ALLOW_ENV_EMPTY="},
			SandboxConfig: SandboxConfig{
				AllowEnv: []string{"SMAUG_ALLOW_ENV_EMPTY"},
			},
		},
		binaryPath: "/fake/flatpak-spawn",
	}

	require.NoError(t, fr.Run())
	assert.Contains(t, gotArgs, "--env=SMAUG_ALLOW_ENV_EMPTY=")
	assert.NotContains(t, gotArgs, "--env=SMAUG_ALLOW_ENV_EMPTY=host")
}
