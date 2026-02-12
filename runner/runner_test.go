package runner_test

import (
	"bytes"
	"context"
	"errors"
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
