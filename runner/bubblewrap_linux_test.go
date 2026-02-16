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
