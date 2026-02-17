//go:build linux

package runner

import "strings"

// BaseSandboxEnvVars are platform/session vars required by sandboxed games.
var BaseSandboxEnvVars = []string{
	"DISPLAY",
	"XAUTHORITY",
	"WAYLAND_DISPLAY",
	"XDG_RUNTIME_DIR",
	"PULSE_SERVER",
	"DBUS_SESSION_BUS_ADDRESS",
	"HOME",
	"USER",
	"LANG",
	"PATH",
	"TMP",
	"TEMP",
	"TMPDIR",
}

func SandboxEnvAllowlist() []string {
	allowlist := make([]string, 0, len(BaseSandboxEnvVars)+len(ItchioLaunchEnvVars))
	allowlist = append(allowlist, BaseSandboxEnvVars...)
	allowlist = append(allowlist, ItchioLaunchEnvVars...)
	return allowlist
}

func collectAllowedEnv(paramsEnv []string, hostEnv []string, extraKeys []string) []string {
	allowlist := SandboxEnvAllowlist()
	allowlist = append(allowlist, extraKeys...)
	out := make([]string, 0, len(allowlist))
	for _, key := range allowlist {
		if val, found := envLookupWithPresence(paramsEnv, key); found {
			out = append(out, key+"="+val)
			continue
		}
		if val, found := envLookupWithPresence(hostEnv, key); found {
			out = append(out, key+"="+val)
		}
	}
	return out
}

// envLookup looks up a key in a []string{"KEY=VALUE", ...} slice.
func envLookup(env []string, key string) string {
	val, _ := envLookupWithPresence(env, key)
	return val
}

// envLookupWithPresence returns the value for KEY and whether KEY was present.
// Presence is true even when KEY is explicitly set to an empty value ("KEY=").
func envLookupWithPresence(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):], true
		}
	}
	return "", false
}
