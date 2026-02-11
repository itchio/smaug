//go:build !linux

package runner

import (
	"fmt"
	"runtime"
)

func newFirejailRunner(params RunnerParams) (Runner, error) {
	return nil, fmt.Errorf("firejail runner is not implemented on %s", runtime.GOOS)
}
