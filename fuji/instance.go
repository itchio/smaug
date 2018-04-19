package fuji

type Settings struct {
	// CredentialsRegistryKey is the path of a key under HKEY_CURRENT_USER
	// itch uses `SOFTWARE\itch\Sandbox`.
	CredentialsRegistryKey string
}

type Instance interface {
	Settings() *Settings
}
