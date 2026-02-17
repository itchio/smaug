//go:build linux

package runner

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/itchio/headway/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bubblewrapSetenvValues(args []string, key string) []string {
	out := make([]string, 0, 1)
	for i := 0; i+2 < len(args); i++ {
		if args[i] == "--setenv" && args[i+1] == key {
			out = append(out, args[i+2])
		}
	}
	return out
}

func bubblewrapHasBind(args []string, source string, target string) bool {
	for i := 0; i+2 < len(args); i++ {
		if args[i] == "--bind" && args[i+1] == source && args[i+2] == target {
			return true
		}
	}
	return false
}

func bubblewrapBindIndex(args []string, source string, target string) int {
	for i := 0; i+2 < len(args); i++ {
		if args[i] == "--bind" && args[i+1] == source && args[i+2] == target {
			return i
		}
	}
	return -1
}

func TestEnsureSandboxParentDirs(t *testing.T) {
	var args []string
	seen := make(map[string]struct{})

	ensureSandboxParentDirs(&args, seen, "/run/user/1000/wayland-0")
	assert.Equal(t, []string{
		"--dir", "/run",
		"--dir", "/run/user",
		"--dir", "/run/user/1000",
	}, args)

	ensureSandboxParentDirs(&args, seen, "/run/user/1000/pipewire-0")
	assert.Equal(t, []string{
		"--dir", "/run",
		"--dir", "/run/user",
		"--dir", "/run/user/1000",
	}, args)

	ensureSandboxParentDirs(&args, seen, "relative/path.sock")
	assert.Equal(t, []string{
		"--dir", "/run",
		"--dir", "/run/user",
		"--dir", "/run/user/1000",
	}, args)
}

func TestParseDbusSocketPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		address string
		want    string
	}{
		{
			name:    "unix path",
			address: "unix:path=/run/user/1000/bus",
			want:    "/run/user/1000/bus",
		},
		{
			name:    "unix path with params",
			address: "unix:path=/run/user/1000/bus,guid=abc123",
			want:    "/run/user/1000/bus",
		},
		{
			name:    "unix abstract",
			address: "unix:abstract=/tmp/dbus-123",
			want:    "",
		},
		{
			name:    "empty",
			address: "",
			want:    "",
		},
		{
			name:    "multiple addresses with unix path",
			address: "unix:abstract=/tmp/dbus-123;unix:path=/run/user/1000/bus,guid=abc123",
			want:    "/run/user/1000/bus",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, parseDbusSocketPath(tc.address))
		})
	}
}

func TestBubblewrapNoNetworkAddsUnshareNet(t *testing.T) {
	origCommand := bubblewrapCommand
	t.Cleanup(func() {
		bubblewrapCommand = origCommand
	})

	var gotName string
	var gotArgs []string
	bubblewrapCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	br := &bubblewrapRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			BubblewrapParams: BubblewrapParams{
				BinaryPath: "/fake/bwrap",
			},
			SandboxConfig:  SandboxConfig{NoNetwork: true},
			FullTargetPath: "/bin/true",
		},
	}

	require.NoError(t, br.Run())
	assert.Equal(t, "/fake/bwrap", gotName)
	assert.Contains(t, gotArgs, "--unshare-net")
}

func TestBubblewrapNoNetworkDisabledOmitsUnshareNet(t *testing.T) {
	origCommand := bubblewrapCommand
	t.Cleanup(func() {
		bubblewrapCommand = origCommand
	})

	var gotArgs []string
	bubblewrapCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	br := &bubblewrapRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			BubblewrapParams: BubblewrapParams{
				BinaryPath: "/fake/bwrap",
			},
			FullTargetPath: "/bin/true",
		},
	}

	require.NoError(t, br.Run())
	assert.NotContains(t, gotArgs, "--unshare-net")
}

func TestBubblewrapAllowlistEnvEmptyValueOverridesHost(t *testing.T) {
	origCommand := bubblewrapCommand
	t.Cleanup(func() {
		bubblewrapCommand = origCommand
	})

	var gotArgs []string
	bubblewrapCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	t.Setenv("LANG", "host-lang")

	br := &bubblewrapRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			BubblewrapParams: BubblewrapParams{
				BinaryPath: "/fake/bwrap",
			},
			Env:            []string{"LANG="},
			FullTargetPath: "/bin/true",
		},
	}

	require.NoError(t, br.Run())
	assert.Equal(t, []string{""}, bubblewrapSetenvValues(gotArgs, "LANG"))
}

func TestBubblewrapAllowEnvEmptyValueOverridesHost(t *testing.T) {
	origCommand := bubblewrapCommand
	t.Cleanup(func() {
		bubblewrapCommand = origCommand
	})

	var gotArgs []string
	bubblewrapCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	t.Setenv("SMAUG_ALLOW_ENV_EMPTY", "host")

	br := &bubblewrapRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			BubblewrapParams: BubblewrapParams{
				BinaryPath: "/fake/bwrap",
			},
			Env: []string{"SMAUG_ALLOW_ENV_EMPTY="},
			SandboxConfig: SandboxConfig{
				AllowEnv: []string{"SMAUG_ALLOW_ENV_EMPTY"},
			},
			FullTargetPath: "/bin/true",
		},
	}

	require.NoError(t, br.Run())
	assert.Equal(t, []string{""}, bubblewrapSetenvValues(gotArgs, "SMAUG_ALLOW_ENV_EMPTY"))
}

func TestBubblewrapMountsPersistentSandboxHome(t *testing.T) {
	origCommand := bubblewrapCommand
	t.Cleanup(func() {
		bubblewrapCommand = origCommand
	})

	var gotArgs []string
	bubblewrapCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	installFolder := t.TempDir()
	homeTarget := "/home/sandbox-user"
	homeSource := filepath.Join(installFolder, ".itch", "home")

	br := &bubblewrapRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			BubblewrapParams: BubblewrapParams{
				BinaryPath: "/fake/bwrap",
			},
			InstallFolder:  installFolder,
			Env:            []string{"HOME=" + homeTarget},
			FullTargetPath: "/bin/true",
		},
	}

	require.NoError(t, br.Run())
	assert.DirExists(t, homeSource)
	assert.True(t, bubblewrapHasBind(gotArgs, homeSource, homeTarget))
}

func TestBubblewrapInstallBindComesAfterSandboxHomeBind(t *testing.T) {
	origCommand := bubblewrapCommand
	t.Cleanup(func() {
		bubblewrapCommand = origCommand
	})

	var gotArgs []string
	bubblewrapCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string{}, args...)
		return exec.Command("sh", "-c", "true")
	}

	installFolder := "/home/leafo/.config/kitch/apps/sample-evil-app 2"
	homeTarget := "/home/leafo"
	homeSource := filepath.Join(installFolder, ".itch", "home")

	br := &bubblewrapRunner{
		params: RunnerParams{
			Consumer: &state.Consumer{OnMessage: func(string, string) {}},
			Ctx:      context.Background(),
			BubblewrapParams: BubblewrapParams{
				BinaryPath: "/fake/bwrap",
			},
			InstallFolder:  installFolder,
			Env:            []string{"HOME=" + homeTarget},
			FullTargetPath: "/bin/true",
		},
	}

	require.NoError(t, br.Run())

	homeBind := bubblewrapBindIndex(gotArgs, homeSource, homeTarget)
	installBind := bubblewrapBindIndex(gotArgs, installFolder, installFolder)
	require.NotEqual(t, -1, homeBind)
	require.NotEqual(t, -1, installBind)
	assert.Less(t, homeBind, installBind)
}
