package runner

// ItchioLaunchEnvVars are launcher-provided variables that games may rely on.
// Keep this list centralized so sandbox backends can share passthrough policy.
var ItchioLaunchEnvVars = []string{
	"ITCHIO_API_KEY",
	"ITCHIO_API_KEY_EXPIRES_AT",
	"ITCHIO_OFFLINE_MODE",
	"ITCHIO_SANDBOX",
}
