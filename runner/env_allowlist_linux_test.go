//go:build linux

package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectAllowedEnvParamsEmptyOverridesHostBaseKey(t *testing.T) {
	got := collectAllowedEnv(
		[]string{"LANG="},
		[]string{"LANG=host-lang"},
		nil,
	)

	assert.Contains(t, got, "LANG=")
	assert.NotContains(t, got, "LANG=host-lang")
}

func TestCollectAllowedEnvParamsEmptyOverridesHostExtraKey(t *testing.T) {
	got := collectAllowedEnv(
		[]string{"SMAUG_EXTRA_ENV="},
		[]string{"SMAUG_EXTRA_ENV=host"},
		[]string{"SMAUG_EXTRA_ENV"},
	)

	assert.Contains(t, got, "SMAUG_EXTRA_ENV=")
	assert.NotContains(t, got, "SMAUG_EXTRA_ENV=host")
}

func TestCollectAllowedEnvExtraKeyFallsBackToHost(t *testing.T) {
	got := collectAllowedEnv(
		nil,
		[]string{"SMAUG_EXTRA_ENV=host"},
		[]string{"SMAUG_EXTRA_ENV"},
	)

	assert.Contains(t, got, "SMAUG_EXTRA_ENV=host")
}
