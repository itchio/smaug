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

- **`runner`** — Core package containing `GetRunner()`, the `Runner` interface, platform-specific runner implementations (`firejailRunner`, `sandboxExecRunner`, `fujiRunner`, `simpleRunner`, `appRunner`), and process group management.
- **`fuji`** — Windows sandbox implementation using isolated user accounts. Creates a low-privilege `itch-player-XXXXX` user, manages credentials via the Windows registry, and handles folder sharing for each launch.

## Sandboxing

### Linux

Uses [firejail](https://firejail.wordpress.com/). A profile is generated at `{InstallFolder}/.itch/isolate-app.profile` that blacklists sensitive directories (browser caches, itch/kitch config) and whitelists the game's install folder and temp directory. Per-game local overrides can be placed in `/etc/firejail/` (e.g. `itch_game_{name}.local`), and a global override file `itch_games_globals.local` is also included if present.

### macOS

Uses Apple's `sandbox-exec` with a generated [Seatbelt](https://reverse.put.as/wp-content/uploads/2011/09/Apple-Sandbox-Guide-v1.0.pdf) (SBPL) policy. The policy defaults to deny, then grants access to the game's install folder, system libraries, fonts, audio, and networking. For app bundles, a temporary shim `.app` wrapper is created that invokes `sandbox-exec` inside the bundle structure so that macOS treats it as a proper application.

### Windows

Uses [fuji](./fuji/), a custom sandbox that creates a low-privilege `itch-player-XXXXX` user account hidden from the login screen. Credentials are generated automatically and stored in the Windows registry. Before each launch the game folder is shared with the sandbox user; access is revoked after exit. Process trees are managed through Windows Job Objects with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, ensuring all child processes are cleaned up.

## License

Licensed under MIT License, see [LICENSE](LICENSE) for details.
