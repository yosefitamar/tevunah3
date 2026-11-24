// Package secretbox cifra segredos guardados no banco (ex.: a chave da API
// Anthropic de cada usuário) com AES-256-GCM e uma chave mestra que fica só
// no ambiente do servidor (APP_SECRETS_KEY, 32 bytes em base64).
//
// Formato gravado: nonce (12 bytes) || ciphertext+tag. O "dado associado"
// (aad) amarra a cifra ao registro dono dela — tipicamente o id do usuário —
// para que copiar a coluna para outra linha não produza um segredo válido.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// ErrNotConfigured: APP_SECRETS_KEY ausente — quem usa o cofre deve
// desligar a feature em vez de guardar segredo em claro.
var ErrNotConfigured = errors.New("cofre de segredos não configurado (APP_SECRETS_KEY)")

// ErrOpen: a cifra não abre (chave mestra trocada, dado corrompido ou aad
// de outro registro).
var ErrOpen = errors.New("não foi possível decifrar o segredo")

// Box cifra e decifra com a chave mestra. Box nil = não configurado.
type Box struct{ aead cipher.AEAD }

// New lê a chave mestra em base64 (32 bytes). Vazia → (nil, nil): o cofre
// fica desligado, e Seal/Open devolvem ErrNotConfigured.
func New(keyB64 string) (*Box, error) {
	keyB64 = strings.TrimSpace(keyB64)
	if keyB64 == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("APP_SECRETS_KEY: base64 inválido: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("APP_SECRETS_KEY: esperado 32 bytes, veio %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Ready diz se o cofre pode ser usado.
func (b *Box) Ready() bool { return b != nil }

// Seal cifra plain amarrado a aad.
func (b *Box) Seal(plain, aad []byte) ([]byte, error) {
	if b == nil {
		return nil, ErrNotConfigured
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, plain, aad), nil
}

// Open decifra o que Seal produziu com o mesmo aad.
func (b *Box) Open(sealed, aad []byte) ([]byte, error) {
	if b == nil {
		return nil, ErrNotConfigured
	}
	n := b.aead.NonceSize()
	if len(sealed) < n+b.aead.Overhead() {
		return nil, ErrOpen
	}
	plain, err := b.aead.Open(nil, sealed[:n], sealed[n:], aad)
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}
