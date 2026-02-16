package runner_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/itchio/headway/state"
	"github.com/itchio/smaug/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testHelperPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "smaug-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	binaryName := "testhelper"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	testHelperPath = filepath.Join(tmpDir, binaryName)

	cmd := exec.Command("go", "build", "-o", testHelperPath, "./testdata/testhelper")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		panic("failed to build test helper: " + err.Error())
	}

	os.Exit(m.Run())
}

func newTestConsumer(t *testing.T) *state.Consumer {
	return &state.Consumer{
		OnMessage: func(lvl string, msg string) {
			t.Logf("[%s] %s", lvl, msg)
		},
	}
}

func newTestParams(t *testing.T, args ...string) runner.RunnerParams {
	return runner.RunnerParams{
		Consumer:       newTestConsumer(t),
		Ctx:            context.Background(),
		FullTargetPath: testHelperPath,
		Args:           args,
		InstallFolder:  filepath.Dir(testHelperPath),
	}
}

func TestBasicExecution(t *testing.T) {
	var stdout bytes.Buffer
	params := newTestParams(t, "echo", "hello")
	params.Stdout = &stdout

	r, err := runner.GetRunner(params)
	require.NoError(t, err)

	require.NoError(t, r.Prepare())

	require.NoError(t, r.Run())

	assert.Equal(t, "hello\n", stdout.String())
}

func TestArgumentPassing(t *testing.T) {
	var stdout bytes.Buffer
	params := newTestParams(t, "echo", "hello world", "foo\tbar", "baz\"qux")
	params.Stdout = &stdout

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "hello world", lines[0])
	assert.Equal(t, "foo\tbar", lines[1])
	assert.Equal(t, "baz\"qux", lines[2])
}

func TestEnvironmentVariables(t *testing.T) {
	var stdout bytes.Buffer
	params := newTestParams(t, "env", "TEST_VAR_A", "TEST_VAR_B")
	params.Stdout = &stdout
	params.Env = []string{
		"TEST_VAR_A=alpha",
		"TEST_VAR_B=beta",
	}

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "alpha", lines[0])
	assert.Equal(t, "beta", lines[1])
}

func TestStdoutStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	params := newTestParams(t, "output", "stdout", "out-msg", "stderr", "err-msg")
	params.Stdout = &stdout
	params.Stderr = &stderr

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	assert.Equal(t, "out-msg\n", stdout.String())
	assert.Equal(t, "err-msg\n", stderr.String())
}

func TestWorkingDirectory(t *testing.T) {
	var stdout bytes.Buffer
	dir := t.TempDir()
	params := newTestParams(t, "cwd")
	params.Stdout = &stdout
	params.Dir = dir

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	// On macOS, /tmp may resolve to /private/tmp via symlink
	got, err := filepath.EvalSymlinks(strings.TrimSpace(stdout.String()))
	require.NoError(t, err)
	expected, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestExitCodeZero(t *testing.T) {
	params := newTestParams(t, "exit", "0")

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())

	assert.NoError(t, r.Run())
}

func TestExitCodeNonZero(t *testing.T) {
	params := newTestParams(t, "exit", "42")

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())

	err = r.Run()

	if runtime.GOOS == "windows" {
		// On Windows, the job object completion port reports success
		// when all processes exit, regardless of individual exit codes.
		// See processgroup_windows.go:130
		assert.NoError(t, err)
	} else {
		require.Error(t, err)
		var exitErr *exec.ExitError
		require.True(t, errors.As(err, &exitErr), "expected *exec.ExitError, got %T: %v", err, err)
		assert.Equal(t, 42, exitErr.ExitCode())
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	params := newTestParams(t, "sleep", "30000")
	params.Ctx = ctx

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())

	done := make(chan error, 1)
	go func() {
		done <- r.Run()
	}()

	// Give the process time to start
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Run returned promptly after cancellation
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5 seconds after context cancellation")
	}
}

func skipIfNoBubblewrap(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("bubblewrap tests only run on Linux")
	}
	bwrapPath, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("bwrap not found in PATH")
	}

	// Bubblewrap can be present but unusable in restricted environments.
	probeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	probeCmd := exec.CommandContext(probeCtx, bwrapPath, "--unshare-user", "--ro-bind", "/", "/", "--", "true")
	out, err := probeCmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		t.Skipf("bwrap found but unusable in this environment: %s", msg)
	}

	return bwrapPath
}

func newBubblewrapParams(t *testing.T, bwrapPath string, args ...string) runner.RunnerParams {
	t.Helper()
	params := newTestParams(t, args...)
	params.Sandbox = true
	params.BubblewrapParams = runner.BubblewrapParams{
		BinaryPath: bwrapPath,
	}
	return params
}

func TestBubblewrapBasicExecution(t *testing.T) {
	bwrapPath := skipIfNoBubblewrap(t)

	var stdout bytes.Buffer
	params := newBubblewrapParams(t, bwrapPath, "echo", "hello")
	params.Stdout = &stdout

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	assert.Equal(t, "hello\n", stdout.String())
}

func TestBubblewrapArgumentPassing(t *testing.T) {
	bwrapPath := skipIfNoBubblewrap(t)

	var stdout bytes.Buffer
	params := newBubblewrapParams(t, bwrapPath, "echo", "hello world", "foo\tbar", "baz\"qux")
	params.Stdout = &stdout

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "hello world", lines[0])
	assert.Equal(t, "foo\tbar", lines[1])
	assert.Equal(t, "baz\"qux", lines[2])
}

func TestBubblewrapStdoutStderr(t *testing.T) {
	bwrapPath := skipIfNoBubblewrap(t)

	var stdout, stderr bytes.Buffer
	params := newBubblewrapParams(t, bwrapPath, "output", "stdout", "out-msg", "stderr", "err-msg")
	params.Stdout = &stdout
	params.Stderr = &stderr

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	assert.Equal(t, "out-msg\n", stdout.String())
	assert.Equal(t, "err-msg\n", stderr.String())
}

func TestBubblewrapContextCancellation(t *testing.T) {
	bwrapPath := skipIfNoBubblewrap(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	params := newBubblewrapParams(t, bwrapPath, "sleep", "30000")
	params.Ctx = ctx

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())

	done := make(chan error, 1)
	go func() {
		done <- r.Run()
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		// Run returned promptly after cancellation and did not fail before cancellation.
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5 seconds after context cancellation")
	}
}

func TestBubblewrapSelectionPriority(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("runner selection test only relevant on Linux")
	}

	params := newTestParams(t)
	params.Sandbox = true
	params.BubblewrapParams = runner.BubblewrapParams{
		BinaryPath: "/usr/bin/bwrap",
	}
	params.FirejailParams = runner.FirejailParams{
		BinaryPath: "/usr/bin/firejail",
	}

	r, err := runner.GetRunner(params)
	require.NoError(t, err)

	// When both are configured, bubblewrap should be chosen
	typeName := fmt.Sprintf("%T", r)
	assert.Contains(t, typeName, "bubblewrap", "expected bubblewrap runner when both are configured")
}

func TestInvalidExecutable(t *testing.T) {
	consumer := newTestConsumer(t)
	params := runner.RunnerParams{
		Consumer:       consumer,
		Ctx:            context.Background(),
		FullTargetPath: filepath.Join(t.TempDir(), "nonexistent-binary"),
		InstallFolder:  t.TempDir(),
	}

	r, err := runner.GetRunner(params)
	if err == nil {
		if err = r.Prepare(); err == nil {
			err = r.Run()
		}
	}
	require.Error(t, err, "expected an error for nonexistent executable")
}

func skipIfNotFlatpak(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("flatpak-spawn tests only run on Linux")
	}
	if _, err := os.Stat("/.flatpak-info"); err != nil {
		t.Skip("not running inside a Flatpak environment")
	}
	if _, err := exec.LookPath("flatpak-spawn"); err != nil {
		t.Skip("flatpak-spawn not found in PATH")
	}
}

func newFlatpakSpawnParams(t *testing.T, args ...string) runner.RunnerParams {
	t.Helper()
	params := newTestParams(t, args...)
	params.Sandbox = true
	return params
}

func TestFlatpakSpawnBasicExecution(t *testing.T) {
	skipIfNotFlatpak(t)

	var stdout bytes.Buffer
	params := newFlatpakSpawnParams(t, "echo", "hello")
	params.Stdout = &stdout

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	assert.Equal(t, "hello\n", stdout.String())
}

func TestFlatpakSpawnEnvironmentVariables(t *testing.T) {
	skipIfNotFlatpak(t)

	var stdout bytes.Buffer
	params := newFlatpakSpawnParams(t, "env", "TEST_VAR_A", "TEST_VAR_B")
	params.Stdout = &stdout
	params.Env = []string{
		"TEST_VAR_A=alpha",
		"TEST_VAR_B=beta",
	}

	r, err := runner.GetRunner(params)
	require.NoError(t, err)
	require.NoError(t, r.Prepare())
	require.NoError(t, r.Run())

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "alpha", lines[0])
	assert.Equal(t, "beta", lines[1])
}

func TestFlatpakSpawnSelectionInFlatpak(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("runner selection test only relevant on Linux")
	}
	if _, err := os.Stat("/.flatpak-info"); err != nil {
		t.Skip("not running inside a Flatpak environment")
	}

	params := newTestParams(t)
	params.Sandbox = true
	// Even with bubblewrap configured, flatpak-spawn should be chosen inside Flatpak
	params.BubblewrapParams = runner.BubblewrapParams{
		BinaryPath: "/usr/bin/bwrap",
	}

	r, err := runner.GetRunner(params)
	require.NoError(t, err)

	typeName := fmt.Sprintf("%T", r)
	assert.Contains(t, typeName, "flatpakSpawn", "expected flatpak-spawn runner when inside Flatpak")
}
