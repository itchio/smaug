package runner_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/itchio/headway/state"
	"github.com/itchio/smaug/runner"
	"github.com/stretchr/testify/assert"
)

func Test_PrepareMacLaunchTarget(t *testing.T) {
	assert := assert.New(t)

	installFolder, err := ioutil.TempDir("", "install-folder")
	tmust(t, err)
	defer os.RemoveAll(installFolder)

	t.Logf("Regular app bundle")
	bundlePath := filepath.Join(installFolder, "Foobar.app")
	tmust(t, os.MkdirAll(bundlePath, 0755))

	consumer := &state.Consumer{
		OnMessage: func(lvl string, msg string) {
			t.Logf("[%s] %s", lvl, msg)
		},
	}
	params := runner.RunnerParams{
		Consumer:       consumer,
		FullTargetPath: bundlePath,
		InstallFolder:  installFolder,
	}
	target, err := runner.PrepareMacLaunchTarget(params)
	assert.NoError(err)
	assert.EqualValues(bundlePath, target.Path)
	assert.True(target.IsAppBundle)

	machoHeader := []byte{0xfe, 0xed, 0xfa, 0xcf}

	t.Logf("Naked executable (not in bundle)")
	nakedExecPath := filepath.Join(installFolder, "utilities", "x86_64", "bin", "jtool")
	tmust(t, os.MkdirAll(filepath.Dir(nakedExecPath), 0755))
	tmust(t, ioutil.WriteFile(nakedExecPath, machoHeader, 0755))

	params.FullTargetPath = nakedExecPath
	target, err = runner.PrepareMacLaunchTarget(params)
	assert.NoError(err)
	assert.EqualValues(nakedExecPath, target.Path)
	assert.False(target.IsAppBundle)

	t.Logf("Nested executable (in bundle)")
	nestedExecPath := filepath.Join(bundlePath, "Contents", "MacOS", "crabapple-launcher")
	tmust(t, os.MkdirAll(filepath.Dir(nestedExecPath), 0755))
	tmust(t, ioutil.WriteFile(nestedExecPath, machoHeader, 0755))

	params.FullTargetPath = nestedExecPath
	target, err = runner.PrepareMacLaunchTarget(params)
	assert.NoError(err)
	assert.EqualValues(bundlePath, target.Path)
	assert.True(target.IsAppBundle)
}

// tmust shows a complete error stack and fails a test immediately
// if err is non-nil
func tmust(t *testing.T, err error) {
	if err != nil {
		t.Helper()
		t.Errorf("%+v", err)
		t.FailNow()
	}
}
