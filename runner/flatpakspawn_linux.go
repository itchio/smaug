//go:build linux

package runner

import (
	"fmt"
	"os"
	"os/exec"
)

type flatpakSpawnRunner struct {
	params     RunnerParams
	binaryPath string
}

var _ Runner = (*flatpakSpawnRunner)(nil)
var flatpakSpawnLookPath = exec.LookPath
var flatpakSpawnCommand = exec.Command

func isInsideFlatpak() bool {
	_, err := os.Stat("/.flatpak-info")
	return err == nil
}

func newFlatpakSpawnRunner(params RunnerParams) (Runner, error) {
	binaryPath, err := flatpakSpawnLookPath("flatpak-spawn")
	if err != nil {
		return nil, fmt.Errorf("inside Flatpak but flatpak-spawn is not available in PATH: %w", err)
	}

	return &flatpakSpawnRunner{
		params:     params,
		binaryPath: binaryPath,
	}, nil
}

func (fr *flatpakSpawnRunner) Prepare() error {
	return nil
}

func (fr *flatpakSpawnRunner) Run() error {
	params := fr.params
	consumer := params.Consumer

	msg := fmt.Sprintf("Running (%s) through flatpak-spawn --sandbox", params.FullTargetPath)
	if params.SandboxConfig.NoNetwork {
		msg += " (networking disabled)"
	}
	consumer.Opf("%s", msg)

	var args []string
	args = append(args, "--sandbox")

	if params.SandboxConfig.NoNetwork {
		args = append(args, "--no-network")
	}

	// Watch the session bus so the sandboxed process dies when butler exits
	args = append(args, "--watch-bus")

	// Environment variables
	for _, e := range params.Env {
		args = append(args, "--env="+e)
	}
	for _, key := range params.SandboxConfig.AllowEnv {
		if _, found := envLookupWithPresence(params.Env, key); found {
			continue
		}
		if val := os.Getenv(key); val != "" {
			args = append(args, "--env="+key+"="+val)
		}
	}

	// Working directory
	if params.Dir != "" {
		args = append(args, "--directory="+params.Dir)
	}

	// Command to run
	args = append(args, "--")
	args = append(args, params.FullTargetPath)
	args = append(args, params.Args...)

	cmd := flatpakSpawnCommand(fr.binaryPath, args...)
	cmd.Dir = params.Dir
	cmd.Env = os.Environ()
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
