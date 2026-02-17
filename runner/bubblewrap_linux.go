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
var bubblewrapCommand = exec.Command

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
	msg := fmt.Sprintf("Running (%s) through bubblewrap", params.FullTargetPath)
	if params.SandboxConfig.NoNetwork {
		msg += " (networking disabled)"
	}
	consumer.Opf("%s", msg)

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

	createdSandboxDirs := make(map[string]struct{})

	// Give sandboxed apps a persistent per-game home directory.
	homeTarget, hasHome := envLookupWithPresence(params.Env, "HOME")
	if !hasHome {
		homeTarget = os.Getenv("HOME")
	}
	if params.InstallFolder != "" && filepath.IsAbs(homeTarget) {
		homeSource := filepath.Join(params.InstallFolder, ".itch", "home")
		if err := os.MkdirAll(homeSource, 0o755); err != nil {
			consumer.Warnf("Could not make sandbox home directory (%s): %s", homeSource, err.Error())
		} else {
			ensureSandboxParentDirs(&args, createdSandboxDirs, homeTarget)
			args = append(args, "--bind", homeSource, homeTarget)
		}
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

	// Namespace isolation:
	// - keep IPC shared for X11 MIT-SHM compatibility
	// - optionally isolate network when NoNetwork is requested
	args = append(args, "--unshare-user")
	if params.SandboxConfig.NoNetwork {
		args = append(args, "--unshare-net")
	}
	args = append(args, "--unshare-pid")
	args = append(args, "--unshare-uts")

	// Lifecycle
	args = append(args, "--die-with-parent")
	args = append(args, "--new-session")

	// Start from an empty environment, then pass through only required vars.
	args = append(args, "--clearenv")

	// Environment passthrough
	for _, key := range SandboxEnvAllowlist() {
		if val, found := envLookupWithPresence(params.Env, key); found {
			args = append(args, "--setenv", key, val)
		} else if val := os.Getenv(key); val != "" {
			args = append(args, "--setenv", key, val)
		}
	}
	for _, key := range params.SandboxConfig.AllowEnv {
		if val, found := envLookupWithPresence(params.Env, key); found {
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

	cmd := bubblewrapCommand(bwrapPath, args...)
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
