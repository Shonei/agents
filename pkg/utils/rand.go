package utils

import "math/rand/v2"

const characterSet = "abcdefghijklmnopqrstuvwxyz0123456789"

func RandomString(n int) string {
	s := make([]byte, n)
	for i := range s {
		s[i] = characterSet[rand.N(len(characterSet))] //nolint:gosec
	}

	return string(s)
}
