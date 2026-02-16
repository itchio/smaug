//go:build !linux

package runner

import (
	"fmt"
	"runtime"
)

func isInsideFlatpak() bool {
	return false
}

func newFlatpakSpawnRunner(params RunnerParams) (Runner, error) {
	return nil, fmt.Errorf("flatpak-spawn runner is not implemented on %s", runtime.GOOS)
}
