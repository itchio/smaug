//go:build !linux

package runner

import (
	"fmt"
	"runtime"
)

func newBubblewrapRunner(params RunnerParams) (Runner, error) {
	return nil, fmt.Errorf("bubblewrap runner is not implemented on %s", runtime.GOOS)
}
