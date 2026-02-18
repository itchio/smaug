//go:build darwin

package runner

import "strings"

var darwinBaseSandboxEnvVars = []string{
	"HOME",
	"USER",
	"LOGNAME",
	"PATH",
	"LANG",
	"LC_ALL",
	"LC_CTYPE",
	"TMP",
	"TEMP",
	"TMPDIR",
}

func darwinSandboxEnvAllowlist() []string {
	allowlist := make([]string, 0, len(darwinBaseSandboxEnvVars)+len(ItchioLaunchEnvVars))
	allowlist = append(allowlist, darwinBaseSandboxEnvVars...)
	allowlist = append(allowlist, ItchioLaunchEnvVars...)
	return allowlist
}

func collectAllowedEnvDarwin(paramsEnv []string, hostEnv []string, extraKeys []string) []string {
	allowlist := darwinSandboxEnvAllowlist()
	allowlist = append(allowlist, extraKeys...)
	out := make([]string, 0, len(allowlist))
	for _, key := range allowlist {
		if val, found := envLookupWithPresenceDarwin(paramsEnv, key); found {
			out = append(out, key+"="+val)
			continue
		}
		if val, found := envLookupWithPresenceDarwin(hostEnv, key); found {
			out = append(out, key+"="+val)
		}
	}
	return out
}

func envLookupWithPresenceDarwin(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):], true
		}
	}
	return "", false
}
