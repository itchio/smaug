//go:build linux

package runner

import (
	"context"
	"os/exec"
	"testing"

	"github.com/itchio/headway/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				NoNetwork:  true,
			},
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
				NoNetwork:  false,
			},
			FullTargetPath: "/bin/true",
		},
	}

	require.NoError(t, br.Run())
	assert.NotContains(t, gotArgs, "--unshare-net")
}
