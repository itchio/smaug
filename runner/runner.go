package runner

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/itchio/butler/manager"
	"github.com/itchio/smaug/firejail"
	"github.com/itchio/smaug/fuji"
	"github.com/itchio/wharf/state"
)

type RunnerParams struct {
	Consumer *state.Consumer
	Ctx      context.Context

	Sandbox bool

	FullTargetPath string

	Name   string
	Dir    string
	Args   []string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer

	InstallFolder string
	Runtime       *manager.Runtime

	FujiSettings     *fuji.Settings
	FirejailSettings *firejail.Settings
}

type Runner interface {
	Prepare() error
	Run() error
}

func GetRunner(params *RunnerParams) (Runner, error) {
	consumer := params.Consumer

	attachRunner, err := getAttachRunner(params)
	if attachRunner != nil {
		return attachRunner, nil
	}
	if err != nil {
		consumer.Warnf("Could not determine if app is aslready running: %s", err.Error())
	}

	switch runtime.GOOS {
	case "windows":
		if params.Sandbox {
			return newFujiRunner(params)
		}
		return newSimpleRunner(params)
	case "linux":
		if params.Sandbox {
			return newFirejailRunner(params)
		}
		return newSimpleRunner(params)
	case "darwin":
		if params.Sandbox {
			return newSandboxExecRunner(params)
		}
		return newAppRunner(params)
	}

	return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}
