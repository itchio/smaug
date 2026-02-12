//go:build darwin

package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/itchio/smaug/runner"
	"github.com/stretchr/testify/require"
)

func TestAppBundle(t *testing.T) {
	bundleDir := filepath.Join(t.TempDir(), "TestHelper.app")
	macosDir := filepath.Join(bundleDir, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(macosDir, 0755))

	// Symlink the test helper binary into the bundle
	require.NoError(t, os.Symlink(testHelperPath, filepath.Join(macosDir, "testhelper")))

	// Write minimal Info.plist
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>testhelper</string>
</dict>
</plist>`
	require.NoError(t, os.WriteFile(
		filepath.Join(bundleDir, "Contents", "Info.plist"),
		[]byte(plist),
		0644,
	))

	params := runner.RunnerParams{
		Consumer:       newTestConsumer(t),
		Ctx:            context.Background(),
		FullTargetPath: bundleDir,
		Args:           []string{"echo", "hello"},
		InstallFolder:  filepath.Dir(bundleDir),
	}

	r, err := runner.GetRunner(params)
	require.NoError(t, err)

	require.NoError(t, r.Prepare())

	// open -W does not relay stdout/stderr, so we can only verify
	// that the bundle launches and exits without error.
	require.NoError(t, r.Run())
}
