//go:build windows

package fuji

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

func (i *instance) GetCredentials() (*Credentials, error) {
	username, err := getRegistryString(i.settings, "username")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	password, err := getRegistryString(i.settings, "password")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	creds := &Credentials{
		Username: username,
		Password: password,
	}
	return creds, nil
}

func (i *instance) saveCredentials(creds *Credentials) error {
	err := setRegistryString(i.settings, "username", creds.Username)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = setRegistryString(i.settings, "password", creds.Password)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// registry utilities

func getRegistryString(s *Settings, name string) (string, error) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, s.CredentialsRegistryKey, registry.READ)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	defer key.Close()

	ret, _, err := key.GetStringValue(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("%w", err)
	}

	return ret, nil
}

func setRegistryString(s *Settings, name string, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, s.CredentialsRegistryKey, registry.WRITE)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	defer key.Close()

	err = key.SetStringValue(name, value)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
