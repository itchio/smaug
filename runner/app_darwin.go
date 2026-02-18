//go:build darwin

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"

	"github.com/itchio/ox/macox"
)

var pgrepCommand = exec.Command
var killCommand = exec.Command
var openCommand = exec.Command

type appRunner struct {
	params       RunnerParams
	target       *MacLaunchTarget
	simpleRunner Runner
}

var _ Runner = (*appRunner)(nil)

func newAppRunner(params RunnerParams) (Runner, error) {
	target, err := PrepareMacLaunchTarget(params)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	params.FullTargetPath = target.Path

	ar := &appRunner{
		params: params,
		target: target,
	}

	if !target.IsAppBundle {
		ar.simpleRunner, err = newSimpleRunner(params)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}

	return ar, nil
}

func (ar *appRunner) Prepare() error {
	if ar.simpleRunner != nil {
		return ar.simpleRunner.Prepare()
	}

	// nothing to prepare
	return nil
}

func (ar *appRunner) Run() error {
	consumer := ar.params.Consumer
	if ar.simpleRunner != nil {
		consumer.Infof("Mac app runner here, delegating run to simple runner")
		return ar.simpleRunner.Run()
	}

	return RunAppBundle(
		ar.params,
		ar.target.Path,
	)
}

func RunAppBundle(params RunnerParams, bundlePath string) error {
	consumer := params.Consumer

	var args = []string{
		"-W",
		bundlePath,
		"--args",
	}
	args = append(args, params.Args...)

	consumer.Infof("App bundle is (%s)", bundlePath)

	binaryPath, err := macox.GetExecutablePath(bundlePath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	consumer.Infof("Actual binary is (%s)", binaryPath)

	cmd := openCommand("open", args...)
	// I doubt this matters
	cmd.Dir = params.Dir
	cmd.Env = params.Env
	// 'open' does not relay stdout or stderr, so we don't
	// even bother setting them

	preLaunchPIDs, err := matchingPIDs(binaryPath)
	havePreLaunchSnapshot := err == nil
	if err != nil {
		consumer.Warnf("Could not snapshot existing app PIDs before launch: %s", err.Error())
	}

	processDone := make(chan struct{})
	interruptSignals := make(chan os.Signal, 1)
	signal.Notify(interruptSignals, os.Interrupt)
	defer signal.Stop(interruptSignals)

	go func() {
		consumer.Infof("Signal handler installed...")

		// Block until a signal is received.
		select {
		case <-params.Ctx.Done():
			consumer.Warnf("Context done!")
		case s := <-interruptSignals:
			consumer.Warnf("Got signal: %v", s)
		case <-processDone:
			return
		}

		consumer.Warnf("Killing app...")
		postLaunchPIDs, err := matchingPIDs(binaryPath)
		if err != nil {
			consumer.Errorf("While discovering app PIDs: %s", err.Error())
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return
		}

		var killList []int
		if havePreLaunchSnapshot {
			killList = diffPIDs(postLaunchPIDs, preLaunchPIDs)
		} else {
			killList = mapKeys(postLaunchPIDs)
		}

		if len(killList) == 0 {
			consumer.Warnf("No launch-specific process IDs found to terminate")
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return
		}

		for _, pid := range killList {
			killCmd := killCommand("kill", "-TERM", strconv.Itoa(pid))
			err = killCmd.Run()
			if err != nil {
				consumer.Warnf("Could not terminate pid %d: %s", pid, err.Error())
			}
		}
	}()

	err = cmd.Run()
	close(processDone)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func matchingPIDs(binaryPath string) (map[int]struct{}, error) {
	cmd := pgrepCommand("pgrep", "-f", binaryPath)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return map[int]struct{}{}, nil
		}
		return nil, fmt.Errorf("pgrep -f %q: %w", binaryPath, err)
	}

	pids := make(map[int]struct{})
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, convErr := strconv.Atoi(line)
		if convErr != nil {
			return nil, fmt.Errorf("parsing pid %q: %w", line, convErr)
		}
		pids[pid] = struct{}{}
	}
	return pids, nil
}

func diffPIDs(after map[int]struct{}, before map[int]struct{}) []int {
	var out []int
	for pid := range after {
		if _, found := before[pid]; found {
			continue
		}
		out = append(out, pid)
	}
	sort.Ints(out)
	return out
}

func mapKeys(pidSet map[int]struct{}) []int {
	var out []int
	for pid := range pidSet {
		out = append(out, pid)
	}
	sort.Ints(out)
	return out
}
