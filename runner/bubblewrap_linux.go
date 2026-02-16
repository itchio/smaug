//go:build linux

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type bubblewrapRunner struct {
	params RunnerParams
}

var _ Runner = (*bubblewrapRunner)(nil)

func newBubblewrapRunner(params RunnerParams) (Runner, error) {
	if params.BubblewrapParams.BinaryPath == "" {
		return nil, fmt.Errorf("BubblewrapParams.BinaryPath must be set")
	}

	br := &bubblewrapRunner{
		params: params,
	}

	return br, nil
}

func (br *bubblewrapRunner) Prepare() error {
	return nil
}

func (br *bubblewrapRunner) Run() error {
	params := br.params
	consumer := params.Consumer

	bwrapPath := params.BubblewrapParams.BinaryPath
	consumer.Opf("Running (%s) through bubblewrap", params.FullTargetPath)

	var args []string

	// Read-only system mounts
	for _, dir := range []string{"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc"} {
		if _, err := os.Stat(dir); err == nil {
			args = append(args, "--ro-bind", dir, dir)
		}
	}

	// Basic filesystem
	args = append(args, "--proc", "/proc")
	args = append(args, "--dev", "/dev")
	args = append(args, "--tmpfs", "/tmp")

	// GPU access
	if _, err := os.Stat("/dev/dri"); err == nil {
		args = append(args, "--dev-bind", "/dev/dri", "/dev/dri")
	}

	// Game install folder (read-write)
	if params.InstallFolder != "" {
		args = append(args, "--bind", params.InstallFolder, params.InstallFolder)
	}

	// Temp directory (read-write)
	if params.TempDir != "" {
		args = append(args, "--bind", params.TempDir, params.TempDir)
	}

	// Working directory if different from install folder
	if params.Dir != "" && params.Dir != params.InstallFolder {
		args = append(args, "--bind", params.Dir, params.Dir)
	}

	// Display/audio socket mounts
	xdgRuntimeDir := envLookup(params.Env, "XDG_RUNTIME_DIR")
	if xdgRuntimeDir == "" {
		xdgRuntimeDir = os.Getenv("XDG_RUNTIME_DIR")
	}

	// X11
	if _, err := os.Stat("/tmp/.X11-unix"); err == nil {
		args = append(args, "--ro-bind", "/tmp/.X11-unix", "/tmp/.X11-unix")
	}

	if xdgRuntimeDir != "" {
		// Wayland
		waylandDisplay := envLookup(params.Env, "WAYLAND_DISPLAY")
		if waylandDisplay == "" {
			waylandDisplay = os.Getenv("WAYLAND_DISPLAY")
		}
		if waylandDisplay != "" {
			socketPath := xdgRuntimeDir + "/" + waylandDisplay
			if _, err := os.Stat(socketPath); err == nil {
				args = append(args, "--ro-bind", socketPath, socketPath)
			}
		}

		// PulseAudio
		pulsePath := xdgRuntimeDir + "/pulse"
		if _, err := os.Stat(pulsePath); err == nil {
			args = append(args, "--ro-bind", pulsePath, pulsePath)
		}

		// PipeWire
		pipewirePath := xdgRuntimeDir + "/pipewire-0"
		if _, err := os.Stat(pipewirePath); err == nil {
			args = append(args, "--ro-bind", pipewirePath, pipewirePath)
		}

		// Bind XDG_RUNTIME_DIR itself as tmpfs so paths under it work
		args = append(args, "--bind", xdgRuntimeDir, xdgRuntimeDir)
	}

	// Namespace isolation (keep network shared)
	args = append(args, "--unshare-user")
	args = append(args, "--unshare-ipc")
	args = append(args, "--unshare-pid")
	args = append(args, "--unshare-uts")

	// Lifecycle
	args = append(args, "--die-with-parent")
	args = append(args, "--new-session")

	// Environment passthrough
	envVarsToForward := []string{
		"DISPLAY", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR",
		"PULSE_SERVER", "DBUS_SESSION_BUS_ADDRESS",
		"HOME", "USER", "LANG", "PATH",
	}
	for _, key := range envVarsToForward {
		if val := envLookup(params.Env, key); val != "" {
			args = append(args, "--setenv", key, val)
		} else if val := os.Getenv(key); val != "" {
			args = append(args, "--setenv", key, val)
		}
	}

	// Working directory inside sandbox
	if params.Dir != "" {
		args = append(args, "--chdir", params.Dir)
	}

	// Command to run
	args = append(args, "--")
	args = append(args, params.FullTargetPath)
	args = append(args, params.Args...)

	cmd := exec.Command(bwrapPath, args...)
	cmd.Dir = params.Dir
	cmd.Env = params.Env
	cmd.Stdout = params.Stdout
	cmd.Stderr = params.Stderr

	pg, err := NewProcessGroup(consumer, cmd, params.Ctx)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = pg.AfterStart()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = pg.Wait()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// envLookup looks up a key in a []string{"KEY=VALUE", ...} slice.
func envLookup(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}
