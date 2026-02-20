//go:build darwin

package runner

import (
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

type capturingRunner struct {
	prepareCalled bool
	runCalled     bool
}

func (cr *capturingRunner) Prepare() error {
	cr.prepareCalled = true
	return nil
}

func (cr *capturingRunner) Run() error {
	cr.runCalled = true
	return nil
}

func parseEnvironmentOutput(output []string) map[string]string {
	out := make(map[string]string)
	for _, line := range output {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}

func newSandboxExecTestConsumer(t *testing.T) *state.Consumer {
	t.Helper()
	return &state.Consumer{
		OnMessage: func(lvl string, msg string) {
			t.Logf("[%s] %s", lvl, msg)
		},
	}
}

func TestSandboxExecRunNakedExecutableUsesSandboxExecBinaryAndFilteredEnv(t *testing.T) {
	origLookPath := sandboxExecLookPath
	origCommand := sandboxExecCommand
	origNewSimpleRunner := newSimpleRunnerForSandboxExec
	t.Cleanup(func() {
		sandboxExecLookPath = origLookPath
		sandboxExecCommand = origCommand
		newSimpleRunnerForSandboxExec = origNewSimpleRunner
	})

	t.Setenv("SMAUG_ALLOW_ENV_HOST_ONLY", "host-value")

	sandboxExecLookPath = func(file string) (string, error) {
		require.Equal(t, "sandbox-exec", file)
		return "/fake/sandbox-exec", nil
	}
	sandboxExecCommand = func(name string, args ...string) *exec.Cmd {
		require.Equal(t, "/fake/sandbox-exec", name)
		require.Equal(t, []string{"-n", "no-network", "true"}, args)
		return exec.Command("sh", "-c", "true")
	}

	var capturedParams RunnerParams
	fakeRunner := &capturingRunner{}
	newSimpleRunnerForSandboxExec = func(params RunnerParams) (Runner, error) {
		capturedParams = params
		return fakeRunner, nil
	}

	installFolder := t.TempDir()
	targetPath := filepath.Join(installFolder, "test-game")
	require.NoError(t, os.WriteFile(targetPath, []byte{0xfe, 0xed, 0xfa, 0xcf}, 0755))

	params := RunnerParams{
		Consumer:       newSandboxExecTestConsumer(t),
		Ctx:            context.Background(),
		InstallFolder:  installFolder,
		FullTargetPath: targetPath,
		Args:           []string{"--hello", "world"},
		Env: []string{
			"OPENAI_API_KEY=super-secret",
			"USER=sandbox-user",
			"ITCHIO_SANDBOX=1",
		},
		SandboxConfig: SandboxConfig{
			PolicyMode: "balanced",
			AllowEnv:   []string{"SMAUG_ALLOW_ENV_HOST_ONLY"},
		},
	}

	r, err := newSandboxExecRunner(params)
	require.NoError(t, err)

	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	require.True(t, fakeRunner.prepareCalled)
	require.True(t, fakeRunner.runCalled)

	assert.Equal(t, "/fake/sandbox-exec", capturedParams.FullTargetPath)
	require.GreaterOrEqual(t, len(capturedParams.Args), 3)
	assert.Equal(t, "-f", capturedParams.Args[0])
	assert.Equal(t, filepath.Join(installFolder, ".itch", "isolate-app.sb"), capturedParams.Args[1])
	assert.Equal(t, targetPath, capturedParams.Args[2])
	assert.Equal(t, []string{"--hello", "world"}, capturedParams.Args[3:])

	gotEnv := parseEnvironmentOutput(capturedParams.Env)
	assert.Equal(t, "sandbox-user", gotEnv["USER"])
	assert.Equal(t, "1", gotEnv["ITCHIO_SANDBOX"])
	assert.Equal(t, "host-value", gotEnv["SMAUG_ALLOW_ENV_HOST_ONLY"])
	_, hasSecret := gotEnv["OPENAI_API_KEY"]
	assert.False(t, hasSecret, "non-allowlisted variable should not be forwarded")
}

func TestWriteSandboxProfileNoNetworkOmitsNetworkRulesInBalancedMode(t *testing.T) {
	installFolder := t.TempDir()
	ser := &sandboxExecRunner{
		params: RunnerParams{
			Consumer:      newSandboxExecTestConsumer(t),
			InstallFolder: installFolder,
			SandboxConfig: SandboxConfig{
				PolicyMode: "balanced",
				NoNetwork:  true,
			},
		},
	}

	require.NoError(t, ser.WriteSandboxProfile())

	profilePath := filepath.Join(installFolder, ".itch", "isolate-app.sb")
	profileBytes, err := os.ReadFile(profilePath)
	require.NoError(t, err)
	profileText := string(profileBytes)

	assert.NotContains(t, profileText, "(allow network-bind)")
	assert.NotContains(t, profileText, "(allow network-outbound)")
	assert.NotContains(t, profileText, `(subpath "/private")`)
	assert.NotContains(t, profileText, `(subpath "/dev")`)
	assert.Contains(t, profileText, `(literal "/dev/null")`)
}

func TestWriteSandboxProfileLegacyModeKeepsLegacyCompatibilityRules(t *testing.T) {
	installFolder := t.TempDir()
	ser := &sandboxExecRunner{
		params: RunnerParams{
			Consumer:      newSandboxExecTestConsumer(t),
			InstallFolder: installFolder,
			SandboxConfig: SandboxConfig{
				PolicyMode: "legacy",
			},
		},
	}

	require.NoError(t, ser.WriteSandboxProfile())

	profilePath := filepath.Join(installFolder, ".itch", "isolate-app.sb")
	profileBytes, err := os.ReadFile(profilePath)
	require.NoError(t, err)
	profileText := string(profileBytes)

	assert.Contains(t, profileText, `(subpath "/private")`)
	assert.Contains(t, profileText, `(subpath "/dev")`)
	assert.Contains(t, profileText, "(allow network-bind)")
	assert.Contains(t, profileText, "(allow network-outbound)")
}

func TestWriteSandboxProfileEscapesInstallLocation(t *testing.T) {
	installFolder := filepath.Join(t.TempDir(), `folder"with\chars`)
	ser := &sandboxExecRunner{
		params: RunnerParams{
			Consumer:      newSandboxExecTestConsumer(t),
			InstallFolder: installFolder,
			SandboxConfig: SandboxConfig{
				PolicyMode: "balanced",
			},
		},
	}

	require.NoError(t, ser.WriteSandboxProfile())

	profilePath := filepath.Join(installFolder, ".itch", "isolate-app.sb")
	profileBytes, err := os.ReadFile(profilePath)
	require.NoError(t, err)
	profileText := string(profileBytes)

	assert.Contains(t, profileText, escapeSBPLString(installFolder))
	assert.NotContains(t, profileText, installFolder)
}

func TestWriteSandboxProfileIncludesRosettaPaths(t *testing.T) {
	installFolder := t.TempDir()
	ser := &sandboxExecRunner{
		params: RunnerParams{
			Consumer:      newSandboxExecTestConsumer(t),
			InstallFolder: installFolder,
			SandboxConfig: SandboxConfig{
				PolicyMode: "balanced",
			},
		},
	}

	require.NoError(t, ser.WriteSandboxProfile())

	profilePath := filepath.Join(installFolder, ".itch", "isolate-app.sb")
	profileBytes, err := os.ReadFile(profilePath)
	require.NoError(t, err)
	profileText := string(profileBytes)

	assert.Contains(t, profileText, `(subpath "/usr/libexec/rosetta")`)
	assert.Contains(t, profileText, `(subpath "/Library/Apple/usr/libexec/oah")`)
}

func TestCollectAllowedEnvDarwinParamsEmptyOverridesHostExtraKey(t *testing.T) {
	got := collectAllowedEnvDarwin(
		[]string{"SMAUG_EXTRA_ENV="},
		[]string{"SMAUG_EXTRA_ENV=host"},
		[]string{"SMAUG_EXTRA_ENV"},
	)

	assert.Contains(t, got, "SMAUG_EXTRA_ENV=")
	assert.NotContains(t, got, "SMAUG_EXTRA_ENV=host")
}
