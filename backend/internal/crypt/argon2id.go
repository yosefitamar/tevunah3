// Package crypt provê hashing de senha com Argon2id no formato PHC.
//
// Compatibilidade legada: Verify também aceita hashes bcrypt do PHP/Laravel
// ($2y$/$2a$/$2b$) pra suportar usuários importados do sistema legado. Use
// NeedsRehash() pós-Verify pra detectar hashes legados e re-codificar em
// argon2id no primeiro login bem-sucedido.
package crypt

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// Parâmetros do Argon2id — alvo: ~250ms em hardware moderno.
const (
	timeCost    = 3
	memoryCost  = 64 * 1024 // 64 MiB
	parallelism = 4
	keyLen      = 32
	saltLen     = 16
)

// Hash retorna a senha codificada no formato PHC do argon2id.
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("rand salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, timeCost, memoryCost, parallelism, keyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryCost, timeCost, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify compara uma senha em texto plano com o hash. Aceita argon2id (PHC)
// e bcrypt do PHP/Laravel ($2y$/$2a$/$2b$).
func Verify(password, encoded string) (bool, error) {
	if isBcrypt(encoded) {
		return verifyBcrypt(password, encoded)
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("formato de hash inválido")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("versão: %w", err)
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, fmt.Errorf("parâmetros: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("hash: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// NeedsRehash devolve true quando o hash não está no formato preferencial
// (argon2id PHC) — caso típico: hash bcrypt herdado da importação Laravel.
// O caller deve re-hashear com Hash() e persistir após Verify bem-sucedido.
func NeedsRehash(encoded string) bool {
	return !strings.HasPrefix(encoded, "$argon2id$")
}

func isBcrypt(encoded string) bool {
	return strings.HasPrefix(encoded, "$2y$") ||
		strings.HasPrefix(encoded, "$2a$") ||
		strings.HasPrefix(encoded, "$2b$")
}

// verifyBcrypt verifica um hash bcrypt. PHP/Laravel emite com prefixo $2y$
// (cosmetic), enquanto golang.org/x/crypto/bcrypt aceita só $2a$/$2b$ —
// normalizamos antes de comparar (são funcionalmente idênticos).
func verifyBcrypt(password, encoded string) (bool, error) {
	if strings.HasPrefix(encoded, "$2y$") {
		encoded = "$2a$" + encoded[4:]
	}
	err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	return false, err
}
