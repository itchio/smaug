package runner_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/itchio/smaug/runner"
	"github.com/itchio/wharf/state"
	"github.com/itchio/wharf/wtest"
	"github.com/stretchr/testify/assert"
)

func Test_PrepareMacLaunchTarget(t *testing.T) {
	assert := assert.New(t)

	installFolder, err := ioutil.TempDir("", "install-folder")
	wtest.Must(t, err)
	defer os.RemoveAll(installFolder)

	t.Logf("Regular app bundle")
	bundlePath := filepath.Join(installFolder, "Foobar.app")
	wtest.Must(t, os.MkdirAll(bundlePath, 0755))

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
	wtest.Must(t, os.MkdirAll(filepath.Dir(nakedExecPath), 0755))
	wtest.Must(t, ioutil.WriteFile(nakedExecPath, machoHeader, 0755))

	params.FullTargetPath = nakedExecPath
	target, err = runner.PrepareMacLaunchTarget(params)
	assert.NoError(err)
	assert.EqualValues(nakedExecPath, target.Path)
	assert.False(target.IsAppBundle)

	t.Logf("Nested executable (in bundle)")
	nestedExecPath := filepath.Join(bundlePath, "Contents", "MacOS", "crabapple-launcher")
	wtest.Must(t, os.MkdirAll(filepath.Dir(nestedExecPath), 0755))
	wtest.Must(t, ioutil.WriteFile(nestedExecPath, machoHeader, 0755))

	params.FullTargetPath = nestedExecPath
	target, err = runner.PrepareMacLaunchTarget(params)
	assert.NoError(err)
	assert.EqualValues(bundlePath, target.Path)
	assert.True(target.IsAppBundle)
}
