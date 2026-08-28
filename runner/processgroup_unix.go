//go:build !windows

package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/itchio/headway/state"
)

const terminationGracePeriod = 2 * time.Second

type processGroup struct {
	consumer *state.Consumer
	cmd      *exec.Cmd
	ctx      context.Context
}

func NewProcessGroup(consumer *state.Consumer, cmd *exec.Cmd, ctx context.Context) (*processGroup, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	pg := &processGroup{
		consumer: consumer,
		cmd:      cmd,
		ctx:      ctx,
	}
	return pg, nil
}

func (pg *processGroup) AfterStart() error {
	return nil
}

func (pg *processGroup) Wait() error {
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- pg.cmd.Wait()
	}()

	pid := pg.cmd.Process.Pid

	select {
	case <-pg.ctx.Done():
		pg.consumer.Infof("Force closing...")
		target := pid
		pgid, err := syscall.Getpgid(pid)
		if err == nil && pgid != 0 {
			target = -pgid
			pg.consumer.Infof("Terminating all processes in group %d", pgid)
		} else {
			if err != nil {
				pg.consumer.Infof("Could not get group of process %d: %s", pid, err.Error())
			} else {
				pg.consumer.Infof("Process %d had no group", pid)
			}
			pg.consumer.Infof("Terminating single process %d", pid)
		}

		err = syscall.Kill(target, syscall.SIGTERM)
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("sending SIGTERM: %w", err)
		}

		timer := time.NewTimer(terminationGracePeriod)
		defer timer.Stop()
		select {
		case <-waitDone:
			return nil
		case <-timer.C:
			pg.consumer.Infof("Process did not exit after %s, killing it", terminationGracePeriod)
		}

		err = syscall.Kill(target, syscall.SIGKILL)
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("sending SIGKILL: %w", err)
		}

		// Reap the direct child before reporting that the launch has ended.
		// SIGKILL also reaches any descendants that remain in its process group.
		<-waitDone
		return nil
	case err := <-waitDone:
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}
