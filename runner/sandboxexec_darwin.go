//go:build darwin

package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/itchio/ox/macox"
	"github.com/itchio/smaug/runner/policies"
)

var investigateSandbox = os.Getenv("INVESTIGATE_SANDBOX") == "1"

type sandboxExecRunner struct {
	params RunnerParams
	target *MacLaunchTarget
}

var _ Runner = (*sandboxExecRunner)(nil)

func newSandboxExecRunner(params RunnerParams) (Runner, error) {
	target, err := PrepareMacLaunchTarget(params)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	params.FullTargetPath = target.Path

	ser := &sandboxExecRunner{
		params: params,
		target: target,
	}

	return ser, nil
}

func (ser *sandboxExecRunner) Prepare() error {
	consumer := ser.params.Consumer

	// make sure we have sandbox-exec
	{
		cmd := exec.Command("sandbox-exec", "-n", "no-network", "true")
		err := cmd.Run()
		if err != nil {
			consumer.Warnf("While verifying sandbox-exec: %s", err.Error())
			return errors.New("Cannot set up itch.io sandbox, see logs for details")
		}
	}

	return nil
}

func (ser *sandboxExecRunner) SandboxProfilePath() string {
	params := ser.params
	return filepath.Join(params.InstallFolder, ".itch", "isolate-app.sb")
}

func (ser *sandboxExecRunner) WriteSandboxProfile() error {
	sandboxProfilePath := ser.SandboxProfilePath()

	params := ser.params
	consumer := params.Consumer
	consumer.Opf("Writing sandbox profile to (%s)", sandboxProfilePath)
	err := os.MkdirAll(filepath.Dir(sandboxProfilePath), 0755)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	userLibrary, err := macox.GetLibraryPath()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	sandboxSource := policies.SandboxExecTemplate
	sandboxSource = strings.ReplaceAll(
		sandboxSource,
		"{{USER_LIBRARY}}",
		userLibrary,
	)
	sandboxSource = strings.ReplaceAll(
		sandboxSource,
		"{{INSTALL_LOCATION}}",
		params.InstallFolder,
	)

	err = os.WriteFile(sandboxProfilePath, []byte(sandboxSource), 0644)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (ser *sandboxExecRunner) Run() error {
	params := ser.params
	consumer := params.Consumer

	err := ser.WriteSandboxProfile()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if !ser.target.IsAppBundle {
		consumer.Infof("Dealing with naked executable, launching via sandbox-exec directly")
		args := []string{
			"sandbox-exec",
			"-f",
			ser.SandboxProfilePath(),
			params.FullTargetPath,
		}
		args = append(args, params.Args...)

		simpleParams := params
		simpleParams.Args = args
		simpleRunner, err := newSimpleRunner(simpleParams)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		err = simpleRunner.Prepare()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		return simpleRunner.Run()
	}

	consumer.Infof("Creating shim app bundle to enable sandboxing")
	realBundlePath := params.FullTargetPath

	binaryPath, err := macox.GetExecutablePath(realBundlePath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	binaryName := filepath.Base(binaryPath)

	workDir, err := os.MkdirTemp("", "butler-shim-bundle")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer os.RemoveAll(workDir)

	shimBundlePath := filepath.Join(
		workDir,
		filepath.Base(realBundlePath),
	)
	consumer.Opf("Generating shim bundle as (%s)", shimBundlePath)

	shimBinaryPath := filepath.Join(
		shimBundlePath,
		"Contents",
		"MacOS",
		binaryName,
	)
	err = os.MkdirAll(filepath.Dir(shimBinaryPath), 0755)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	shimBinaryContents := fmt.Sprintf(`#!/bin/bash
		cd "%s"
		sandbox-exec -f "%s" "%s" "$@"
		`,
		params.Dir,
		ser.SandboxProfilePath(),
		binaryPath,
	)

	err = os.WriteFile(shimBinaryPath, []byte(shimBinaryContents), 0744)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = os.Symlink(
		filepath.Join(realBundlePath, "Contents", "Resources"),
		filepath.Join(shimBundlePath, "Contents", "Resources"),
	)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = os.Symlink(
		filepath.Join(realBundlePath, "Contents", "Info.plist"),
		filepath.Join(shimBundlePath, "Contents", "Info.plist"),
	)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if investigateSandbox {
		consumer.Warnf("Wrote shim app to (%s), waiting forever because INVESTIGATE_SANDBOX is set to 1", shimBundlePath)
		for {
			time.Sleep(1 * time.Second)
		}
	}

	consumer.Statf("All set, hope for the best")

	return RunAppBundle(
		params,
		shimBundlePath,
	)
}
