# smaug

[![CI](https://github.com/itchio/smaug/actions/workflows/test.yml/badge.svg)](https://github.com/itchio/smaug/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/itchio/smaug)](https://goreportcard.com/report/github.com/itchio/smaug)
[![Go Reference](https://pkg.go.dev/badge/github.com/itchio/smaug.svg)](https://pkg.go.dev/github.com/itchio/smaug)
[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/itchio/smaug/blob/master/LICENSE)

Smaug is a Go library for running processes with context-aware lifecycle management, process group control, and optional platform-specific sandboxing. It provides a unified interface for launching executables across Windows, macOS, and Linux while handling the platform differences behind the scenes.

Built for the [itch.io app](https://itch.io/itch) to launch game binaries, smaug selects the right execution strategy automatically based on the target OS and whether sandboxing is enabled. The `runner` package exposes a `Runner` interface — call `GetRunner()` with your parameters, then `Prepare()` and `Run()`.

## Features

- **Context-aware process execution** — cancellation propagation via Go contexts
- **Process group management** — wait on or kill entire process trees (POSIX process groups on Unix, Job Objects on Windows)
- **Optional sandboxing** via platform-native mechanisms (firejail, sandbox-exec, isolated user accounts)
- **Process reattachment** — detect and attach to already-running processes on Windows
- **macOS app bundle support** — automatic `.app` detection and launching via `open -W`

## Packages

- **`runner`** — Core package containing `GetRunner()`, the `Runner` interface, platform-specific runner implementations (`simpleRunner`, `firejailRunner`, `bubblewrapRunner`, `flatpakSpawnRunner`, `sandboxExecRunner`, `fujiRunner`, `appRunner`), and process group management.
- **`fuji`** — Windows sandbox implementation using isolated user accounts. Creates a low-privilege `itch-player-XXXXX` user, manages credentials via the Windows registry, and handles folder sharing for each launch.

## Sandboxing

### Linux

Three sandbox backends are supported when `Sandbox` is enabled.
Shared sandbox settings are configured through `RunnerParams.SandboxConfig`:
- `Type`: explicit backend (`"bubblewrap"`, `"firejail"`, `"flatpak"`) or auto (`""`)
- `NoNetwork`: disable network access for the selected backend
- `AllowEnv`: additional environment variable names to pass through from the host
- `PolicyMode`: backend-specific policy mode (currently used by macOS `sandbox-exec`)

Sandbox backends:

1. **Flatpak-spawn** — uses `flatpak-spawn --sandbox` to create a sub-sandbox within the Flatpak container. Supports environment variable forwarding (`--env`), working directory (`--directory`), and optional network isolation via `SandboxConfig.NoNetwork` (`--no-network`). The `--watch-bus` flag ties the sandboxed process lifetime to the caller's session bus.

2. **Bubblewrap** — uses [bubblewrap](https://github.com/containers/bubblewrap) to create a lightweight user-namespace sandbox. Mounts system directories read-only, bind-mounts the game's install folder read-write, and forwards display/audio sockets (X11, Wayland, PulseAudio, PipeWire). The in-sandbox `HOME` path is backed by a per-game persistent directory at `{InstallFolder}/.itch/home`, so game saves written under home survive across launches. Namespace isolation covers user, PID, and UTS; IPC stays shared for X11 MIT-SHM compatibility. Network access is shared by default, with optional isolation via `SandboxConfig.NoNetwork`.

3. **Firejail** — uses [firejail](https://firejail.wordpress.com/) with a generated profile at `{InstallFolder}/.itch/isolate-app.profile` that blacklists sensitive directories and whitelists the game's install folder and temp directory. Environment forwarding follows the same allowlist baseline as bubblewrap (including itch launch vars and temp vars), supports additional passthrough via `SandboxConfig.AllowEnv`, and network access can be disabled with `SandboxConfig.NoNetwork`. Per-game local overrides can be placed in `/etc/firejail/` (e.g. `itch_game_{name}.local`), and a global override file `itch_games_globals.local` is also included if present.

Backend selection:
- Explicit selection: set `SandboxConfig.Type` to `"flatpak"`, `"bubblewrap"`, or `"firejail"`.
- Auto selection: leave `SandboxConfig.Type` empty (`""`).
- Linux auto priority: **Flatpak-spawn > Bubblewrap > Firejail**.
- Linux auto rule: choose Flatpak-spawn when running inside a [Flatpak](https://flatpak.org/) environment (`/.flatpak-info` present).
- Linux auto rule: choose Bubblewrap when `BubblewrapParams.BinaryPath` is configured.
- Linux auto rule: choose Firejail otherwise.

### macOS

Uses Apple's `sandbox-exec` with a generated [Seatbelt](https://reverse.put.as/wp-content/uploads/2011/09/Apple-Sandbox-Guide-v1.0.pdf) (SBPL) policy. The policy defaults to deny, then grants access to the game's install folder plus required runtime resources. `SandboxConfig.NoNetwork` is supported on macOS and removes network rules from the generated profile. Environment forwarding in sandbox mode follows a strict allowlist baseline (including itch launch vars) plus `SandboxConfig.AllowEnv`.

For app bundles, a temporary shim `.app` wrapper is created that invokes `sandbox-exec` inside the bundle structure so that macOS treats it as a proper application.

Policy rollout mode can be controlled with `RunnerParams.SandboxConfig.PolicyMode`:
- `"balanced"` (default)
- `"legacy"` (compatibility fallback)

### Windows

Uses [fuji](./fuji/), a custom sandbox that creates a low-privilege `itch-player-XXXXX` user account hidden from the login screen. Credentials are generated automatically and stored in the Windows registry. Before each launch the game folder is shared with the sandbox user; access is revoked after exit. Process trees are managed through Windows Job Objects with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, ensuring all child processes are cleaned up.

## License

Licensed under MIT License, see [LICENSE](LICENSE) for details.
