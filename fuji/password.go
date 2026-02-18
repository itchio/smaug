//go:build windows

package fuji

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

/** Letters used when generating a random password */
const kLetters = "abcdefghijklmnopqrstuvwxyz"

/** Numbers used when generating a random password */
const kNumbers = "0123456789"

/** Special characters used when generating a random password */
const kSpecial = "!_?-.;+/()=&"

func randomCharFromSet(set string) (string, error) {
	index, err := randomInt(len(set))
	if err != nil {
		return "", err
	}
	return set[index : index+1], nil
}

func randomInt(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be positive")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, fmt.Errorf("could not generate secure random integer: %w", err)
	}
	return int(n.Int64()), nil
}

func generatePassword() (string, error) {
	pwd := ""

	for i := 0; i < 16; i++ {
		var token string
		var err error
		switch i % 4 {
		case 0:
			token, err = randomCharFromSet(kLetters)
		case 1:
			token, err = randomCharFromSet(kNumbers)
		case 2:
			token, err = randomCharFromSet(kSpecial)
		case 3:
			var letter string
			letter, err = randomCharFromSet(kLetters)
			token = strings.ToUpper(letter)
		}
		if err != nil {
			return "", err
		}
		pwd += token
	}
	return pwd, nil
}
