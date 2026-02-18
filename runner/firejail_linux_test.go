//go:build linux

package runner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itchio/headway/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFirejailTestConsumer(t *testing.T) *state.Consumer {
	t.Helper()
	return &state.Consumer{
		OnMessage: func(lvl string, msg string) {
			t.Logf("[%s] %s", lvl, msg)
		},
	}
}

func newFirejailTestRunner(t *testing.T, noNetwork bool) *firejailRunner {
	t.Helper()
	return &firejailRunner{
		params: RunnerParams{
			Consumer:       newFirejailTestConsumer(t),
			Ctx:            context.Background(),
			Name:           "test-game",
			FullTargetPath: "/bin/true",
			InstallFolder:  t.TempDir(),
			TempDir:        t.TempDir(),
			FirejailParams: FirejailParams{
				BinaryPath: "/fake/firejail",
			},
			SandboxConfig: SandboxConfig{NoNetwork: noNetwork},
		},
	}
}

func parseEnvironmentOutput(output string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}

func TestFirejailNoNetworkAddsNetNone(t *testing.T) {
	origCommand := firejailCommand
	t.Cleanup(func() {
		firejailCommand = origCommand
	})

	var gotName string
	var gotArgs []string
	firejailCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	fr := newFirejailTestRunner(t, true)
	require.NoError(t, fr.Run())
	assert.Equal(t, "/fake/firejail", gotName)
	assert.Contains(t, gotArgs, "--net=none")
}

func TestFirejailNoNetworkDisabledOmitsNetNone(t *testing.T) {
	origCommand := firejailCommand
	t.Cleanup(func() {
		firejailCommand = origCommand
	})

	var gotArgs []string
	firejailCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	fr := newFirejailTestRunner(t, false)
	require.NoError(t, fr.Run())
	assert.NotContains(t, gotArgs, "--net=none")
}

func TestFirejailUsesAllowlistedEnvironment(t *testing.T) {
	origCommand := firejailCommand
	t.Cleanup(func() {
		firejailCommand = origCommand
	})

	firejailCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "env")
	}

	var stdout bytes.Buffer
	fr := newFirejailTestRunner(t, false)
	fr.params.Stdout = &stdout
	fr.params.Env = []string{
		"OPENAI_API_KEY=super-secret",
		"USER=sandbox-user",
		"ITCHIO_SANDBOX=1",
		"TMP=/game/.itch/temp",
	}

	require.NoError(t, fr.Run())

	gotEnv := parseEnvironmentOutput(stdout.String())
	assert.Equal(t, "sandbox-user", gotEnv["USER"])
	assert.Equal(t, "1", gotEnv["ITCHIO_SANDBOX"])
	assert.Equal(t, "/game/.itch/temp", gotEnv["TMP"])
	_, hasSecret := gotEnv["OPENAI_API_KEY"]
	assert.False(t, hasSecret, "unlisted variables must not be forwarded")
}

func TestFirejailEnvironmentPrefersParamsAndFallsBackToHost(t *testing.T) {
	origCommand := firejailCommand
	t.Cleanup(func() {
		firejailCommand = origCommand
	})

	firejailCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "env")
	}

	t.Setenv("USER", "host-user")
	t.Setenv("LANG", "host-lang")

	var stdout bytes.Buffer
	fr := newFirejailTestRunner(t, false)
	fr.params.Stdout = &stdout
	fr.params.Env = []string{
		"USER=params-user",
	}

	require.NoError(t, fr.Run())

	gotEnv := parseEnvironmentOutput(stdout.String())
	assert.Equal(t, "params-user", gotEnv["USER"])
	assert.Equal(t, "host-lang", gotEnv["LANG"])
}

func TestFirejailProfileBlacklistsSensitivePaths(t *testing.T) {
	origCommand := firejailCommand
	t.Cleanup(func() {
		firejailCommand = origCommand
	})

	firejailCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "true")
	}

	fr := newFirejailTestRunner(t, false)
	require.NoError(t, fr.Run())

	profilePath := filepath.Join(fr.params.InstallFolder, ".itch", "isolate-app.profile")
	profileBytes, err := os.ReadFile(profilePath)
	require.NoError(t, err)
	profileText := string(profileBytes)

	assert.Contains(t, profileText, "blacklist ${HOME}/.ssh")
	assert.Contains(t, profileText, "blacklist ${HOME}/.gnupg")
	assert.Contains(t, profileText, "blacklist ${HOME}/.aws")
	assert.Contains(t, profileText, "blacklist ${HOME}/.config/google-chrome")
}
