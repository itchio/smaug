//go:build darwin

package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestAppBundle(t *testing.T) string {
	t.Helper()

	bundleDir := filepath.Join(t.TempDir(), "TestHelper.app")
	macosDir := filepath.Join(bundleDir, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(macosDir, 0755))

	execPath := filepath.Join(macosDir, "testhelper")
	require.NoError(t, os.WriteFile(execPath, []byte("#!/bin/sh\nexit 0\n"), 0755))

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>testhelper</string>
</dict>
</plist>`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "Contents", "Info.plist"), []byte(plist), 0644))

	return bundleDir
}

func TestRunAppBundleCancellationKillsOnlyNewPIDs(t *testing.T) {
	origPgrep := pgrepCommand
	origKill := killCommand
	origOpen := openCommand
	t.Cleanup(func() {
		pgrepCommand = origPgrep
		killCommand = origKill
		openCommand = origOpen
	})

	var pgrepCalls int
	var killMu sync.Mutex
	var killed []string

	pgrepCommand = func(name string, args ...string) *exec.Cmd {
		pgrepCalls++
		if pgrepCalls == 1 {
			return exec.Command("sh", "-c", "printf '10\n20\n'")
		}
		return exec.Command("sh", "-c", "printf '10\n20\n30\n40\n'")
	}
	killCommand = func(name string, args ...string) *exec.Cmd {
		killMu.Lock()
		killed = append(killed, args[len(args)-1])
		killMu.Unlock()
		return exec.Command("sh", "-c", "true")
	}
	openCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "sleep 0.2")
	}

	bundlePath := writeTestAppBundle(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := RunAppBundle(
		RunnerParams{
			Consumer: newSandboxExecTestConsumer(t),
			Ctx:      ctx,
		},
		bundlePath,
	)
	require.NoError(t, err)

	killMu.Lock()
	defer killMu.Unlock()
	assert.Equal(t, []string{"30", "40"}, killed)
}

func TestDiffPIDsIsSortedAndExcludesExisting(t *testing.T) {
	after := map[int]struct{}{
		30: {},
		40: {},
		20: {},
	}
	before := map[int]struct{}{
		20: {},
	}

	assert.Equal(t, []int{30, 40}, diffPIDs(after, before))
}
