//go:build windows

package fuji

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Credentials(t *testing.T) {
	iif, err := NewInstance(&Settings{
		CredentialsRegistryKey: `SOFTWARE\smaug-test\Sandbox`,
	})
	assert.NoError(t, err)

	i, ok := iif.(*instance)
	assert.True(t, ok)

	err = i.saveCredentials(&Credentials{
		Username: "gecko",
		Password: "jesus",
	})
	assert.NoError(t, err)

	creds, err := i.GetCredentials()
	assert.NoError(t, err)
	assert.EqualValues(t, "gecko", creds.Username)
	assert.EqualValues(t, "jesus", creds.Password)
}
