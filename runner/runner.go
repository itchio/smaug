package runner

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/itchio/headway/state"
	"github.com/itchio/ox"
	"github.com/itchio/smaug/fuji"
)

type RunnerParams struct {
	Consumer *state.Consumer
	Ctx      context.Context

	Sandbox bool
	Console bool

	FullTargetPath string

	Name   string
	Dir    string
	Args   []string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer

	InstallFolder string
	TempDir       string
	Runtime       ox.Runtime

	SandboxConfig SandboxConfig

	// runner-specific params

	FirejailParams     FirejailParams
	BubblewrapParams   BubblewrapParams
	FlatpakSpawnParams FlatpakSpawnParams
	FujiParams         FujiParams
	AttachParams       AttachParams
}

type SandboxType string

const (
	SandboxTypeAuto       SandboxType = ""
	SandboxTypeBubblewrap SandboxType = "bubblewrap"
	SandboxTypeFirejail   SandboxType = "firejail"
	SandboxTypeFlatpak    SandboxType = "flatpak"
	SandboxTypeFuji       SandboxType = "fuji"
)

type SandboxPolicyMode string

const (
	SandboxPolicyModeAuto     SandboxPolicyMode = ""
	SandboxPolicyModeBalanced SandboxPolicyMode = "balanced"
	SandboxPolicyModeLegacy   SandboxPolicyMode = "legacy"
)

type SandboxConfig struct {
	// Which sandbox runner to use. Empty means auto-detect (default).
	Type SandboxType

	// If true, disable network access within the sandbox.
	NoNetwork bool

	// Environment variable names to allow through from the host into the sandbox.
	AllowEnv []string

	// Sandbox policy mode for backends that support multiple policy variants.
	// On macOS sandbox-exec:
	// - "balanced" (default): hardened profile with compatibility safeguards
	// - "legacy": broader compatibility-focused profile
	PolicyMode SandboxPolicyMode
}

type FirejailParams struct {
	BinaryPath string
}

type BubblewrapParams struct {
	BinaryPath string
}

type FlatpakSpawnParams struct {
}

type FujiParams struct {
	Settings             *fuji.Settings
	PerformElevatedSetup func() error
}

type AttachParams struct {
	BringWindowToForeground func(hwnd int64)
}

type Runner interface {
	Prepare() error
	Run() error
}

func GetRunner(params RunnerParams) (Runner, error) {
	consumer := params.Consumer

	attachRunner, err := getAttachRunner(params)
	if attachRunner != nil {
		return attachRunner, nil
	}
	if err != nil {
		consumer.Warnf("Could not determine if app is already running: %s", err.Error())
	}

	switch runtime.GOOS {
	case "windows":
		if params.Sandbox {
			switch params.SandboxConfig.Type {
			case SandboxTypeAuto, SandboxTypeFuji:
				return newFujiRunner(params)
			default:
				return nil, fmt.Errorf("sandbox type %q is not supported on windows", params.SandboxConfig.Type)
			}
		}
		return newSimpleRunner(params)
	case "linux":
		if params.Sandbox {
			switch params.SandboxConfig.Type {
			case SandboxTypeAuto:
				if isInsideFlatpak() {
					return newFlatpakSpawnRunner(params)
				}
				if params.BubblewrapParams.BinaryPath != "" {
					return newBubblewrapRunner(params)
				}
				return newFirejailRunner(params)
			case SandboxTypeBubblewrap:
				return newBubblewrapRunner(params)
			case SandboxTypeFirejail:
				return newFirejailRunner(params)
			case SandboxTypeFlatpak:
				return newFlatpakSpawnRunner(params)
			default:
				return nil, fmt.Errorf("sandbox type %q is not supported on linux", params.SandboxConfig.Type)
			}
		}
		return newSimpleRunner(params)
	case "darwin":
		if params.Sandbox {
			switch params.SandboxConfig.Type {
			case SandboxTypeAuto:
				return newSandboxExecRunner(params)
			default:
				return nil, fmt.Errorf("sandbox type %q is not supported on macOS", params.SandboxConfig.Type)
			}
		}
		return newAppRunner(params)
	}

	return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}
