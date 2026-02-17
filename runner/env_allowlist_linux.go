//go:build linux

package runner

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

func collectAllowedEnv(paramsEnv []string, hostEnv []string) []string {
	allowlist := SandboxEnvAllowlist()
	out := make([]string, 0, len(allowlist))
	for _, key := range allowlist {
		if val := envLookup(paramsEnv, key); val != "" {
			out = append(out, key+"="+val)
			continue
		}
		if val := envLookup(hostEnv, key); val != "" {
			out = append(out, key+"="+val)
		}
	}
	return out
}
