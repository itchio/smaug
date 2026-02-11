//go:build windows

package fuji

import "fmt"

type instance struct {
	settings *Settings
}

var _ Instance = (*instance)(nil)

func NewInstance(settings *Settings) (Instance, error) {
	if settings.CredentialsRegistryKey == "" {
		return nil, fmt.Errorf("CredentialsRegistryKey cannot be empty")
	}

	i := &instance{
		settings: settings,
	}
	return i, nil
}

func (i *instance) Settings() *Settings {
	return i.settings
}
