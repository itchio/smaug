//go:build linux

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	if _, err := os.Stat("/sys"); err == nil {
		args = append(args, "--ro-bind", "/sys", "/sys")
	}

	// Basic filesystem
	args = append(args, "--proc", "/proc")
	args = append(args, "--dev", "/dev")
	args = append(args, "--tmpfs", "/tmp")

	// GPU access
	if _, err := os.Stat("/dev/dri"); err == nil {
		args = append(args, "--dev-bind", "/dev/dri", "/dev/dri")
	}
	if nvidiaPaths, err := filepath.Glob("/dev/nvidia*"); err == nil {
		for _, nvidiaPath := range nvidiaPaths {
			if _, err := os.Stat(nvidiaPath); err == nil {
				args = append(args, "--dev-bind", nvidiaPath, nvidiaPath)
			}
		}
	}

	// Controller input devices
	if _, err := os.Stat("/dev/input"); err == nil {
		args = append(args, "--dev-bind", "/dev/input", "/dev/input")
	}

	// ALSA devices
	if _, err := os.Stat("/dev/snd"); err == nil {
		args = append(args, "--dev-bind", "/dev/snd", "/dev/snd")
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
	createdSandboxDirs := make(map[string]struct{})

	// X11
	if _, err := os.Stat("/tmp/.X11-unix"); err == nil {
		args = append(args, "--ro-bind", "/tmp/.X11-unix", "/tmp/.X11-unix")
	}

	// X11 authentication
	xauthority := envLookup(params.Env, "XAUTHORITY")
	if xauthority == "" {
		xauthority = os.Getenv("XAUTHORITY")
	}
	if xauthority == "" {
		// Default location if XAUTHORITY is not set
		if home := os.Getenv("HOME"); home != "" {
			defaultPath := home + "/.Xauthority"
			if _, err := os.Stat(defaultPath); err == nil {
				xauthority = defaultPath
			}
		}
	}
	if xauthority != "" {
		if _, err := os.Stat(xauthority); err == nil {
			ensureSandboxParentDirs(&args, createdSandboxDirs, xauthority)
			args = append(args, "--ro-bind", xauthority, xauthority)
		}
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
				ensureSandboxParentDirs(&args, createdSandboxDirs, socketPath)
				args = append(args, "--ro-bind", socketPath, socketPath)
			}
		}

		// PulseAudio
		pulsePath := xdgRuntimeDir + "/pulse"
		if _, err := os.Stat(pulsePath); err == nil {
			ensureSandboxParentDirs(&args, createdSandboxDirs, pulsePath)
			args = append(args, "--ro-bind", pulsePath, pulsePath)
		}

		// PipeWire
		pipewirePath := xdgRuntimeDir + "/pipewire-0"
		if _, err := os.Stat(pipewirePath); err == nil {
			ensureSandboxParentDirs(&args, createdSandboxDirs, pipewirePath)
			args = append(args, "--ro-bind", pipewirePath, pipewirePath)
		}
	}

	// D-Bus session socket
	dbusAddress := envLookup(params.Env, "DBUS_SESSION_BUS_ADDRESS")
	if dbusAddress == "" {
		dbusAddress = os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	}
	dbusSocketPath := parseDbusSocketPath(dbusAddress)
	if dbusSocketPath == "" && dbusAddress != "" && strings.Contains(dbusAddress, "unix:abstract=") {
		// Abstract sockets do not map to filesystem paths and do not need mounts.
	} else {
		if dbusSocketPath == "" && xdgRuntimeDir != "" {
			dbusSocketPath = filepath.Join(xdgRuntimeDir, "bus")
		}
		if dbusSocketPath != "" {
			if _, err := os.Stat(dbusSocketPath); err == nil {
				ensureSandboxParentDirs(&args, createdSandboxDirs, dbusSocketPath)
				args = append(args, "--ro-bind", dbusSocketPath, dbusSocketPath)
			}
		}
	}

	// Namespace isolation (keep network and IPC shared for X11 MIT-SHM compatibility)
	args = append(args, "--unshare-user")
	args = append(args, "--unshare-pid")
	args = append(args, "--unshare-uts")

	// Lifecycle
	args = append(args, "--die-with-parent")
	args = append(args, "--new-session")

	// Start from an empty environment, then pass through only required vars.
	args = append(args, "--clearenv")

	// Environment passthrough
	envVarsToForward := []string{
		"DISPLAY", "XAUTHORITY", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR",
		"PULSE_SERVER", "DBUS_SESSION_BUS_ADDRESS",
		"HOME", "USER", "LANG", "PATH",
		"TMP", "TEMP", "TMPDIR",
	}
	envVarsToForward = append(envVarsToForward, ItchioLaunchEnvVars...)
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

func ensureSandboxParentDirs(args *[]string, seen map[string]struct{}, path string) {
	cleanPath := filepath.Clean(path)
	if cleanPath == "" || cleanPath == "." || !filepath.IsAbs(cleanPath) {
		return
	}

	parent := filepath.Dir(cleanPath)
	if parent == "" || parent == "." || parent == "/" {
		return
	}

	current := "/"
	for _, part := range strings.Split(strings.TrimPrefix(parent, "/"), "/") {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		if _, ok := seen[current]; ok {
			continue
		}
		*args = append(*args, "--dir", current)
		seen[current] = struct{}{}
	}
}

func parseDbusSocketPath(address string) string {
	for _, candidate := range strings.Split(address, ";") {
		candidate = strings.TrimSpace(candidate)
		if !strings.HasPrefix(candidate, "unix:") {
			continue
		}

		for _, param := range strings.Split(strings.TrimPrefix(candidate, "unix:"), ",") {
			key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
			if !ok {
				continue
			}
			if key == "path" && value != "" {
				return value
			}
		}
	}

	return ""
}
