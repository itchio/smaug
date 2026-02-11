//go:build windows

package fuji

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GeneratePassword(t *testing.T) {
	previousPasswords := make(map[string]bool)

	for i := 0; i < 100; i++ {
		pass := generatePassword()
		assert.True(t, strings.ContainsAny(pass, kLetters), "password has letters")
		assert.True(t, strings.ContainsAny(pass, kNumbers), "password has numbers")
		assert.True(t, strings.ContainsAny(pass, kSpecial), "password has special characters")
		assert.False(t, previousPasswords[pass], "password is the not the same as the previous one")
		previousPasswords[pass] = true
	}
}
