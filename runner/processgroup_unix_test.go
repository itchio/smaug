//go:build !windows

package runner

import (
	"bufio"
	"context"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/itchio/headway/state"
	"github.com/stretchr/testify/require"
)

func TestProcessGroupCancellationWaitsForExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.Command("sh", "-c", "trap '' TERM; echo ready; while :; do sleep 1; done")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)

	consumer := &state.Consumer{
		OnMessage: func(level string, message string) {
			t.Logf("[%s] %s", level, message)
		},
	}
	pg, err := NewProcessGroup(consumer, cmd, ctx)
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	})
	require.NoError(t, pg.AfterStart())

	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		ready <- scanner.Scan() && scanner.Text() == "ready"
	}()
	select {
	case ok := <-ready:
		require.True(t, ok, "child did not report ready")
	case <-time.After(5 * time.Second):
		t.Fatal("child did not report ready")
	}

	cancelledAt := time.Now()
	cancel()
	require.NoError(t, pg.Wait())
	require.GreaterOrEqual(t, time.Since(cancelledAt), terminationGracePeriod)
	require.ErrorIs(t, syscall.Kill(cmd.Process.Pid, 0), syscall.ESRCH)
}
