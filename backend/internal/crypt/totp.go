package crypt

import (
	"crypto/rand"
	"encoding/base32"
)

// NewTOTPSecret gera 20 bytes aleatórios em base32 (RFC 4648 sem padding),
// formato esperado por apps autenticadores e por github.com/pquerna/otp/totp.
func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}
