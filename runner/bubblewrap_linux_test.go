//go:build linux

package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
