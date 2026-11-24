package secretbox

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
)

func key(t *testing.T) string {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(k)
}

func TestRoundTripAndBinding(t *testing.T) {
	b, err := New(key(t))
	if err != nil || !b.Ready() {
		t.Fatalf("new: %v", err)
	}
	sealed, err := b.Seal([]byte("sk-ant-segredo"), []byte("user-1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("segredo")) {
		t.Fatal("texto em claro na cifra")
	}
	got, err := b.Open(sealed, []byte("user-1"))
	if err != nil || string(got) != "sk-ant-segredo" {
		t.Fatalf("open = %q, %v", got, err)
	}
	if _, err := b.Open(sealed, []byte("user-2")); !errors.Is(err, ErrOpen) {
		t.Fatalf("aad de outro usuário abriu a cifra: %v", err)
	}
	other, _ := New(key(t))
	if _, err := other.Open(sealed, []byte("user-1")); !errors.Is(err, ErrOpen) {
		t.Fatalf("outra chave mestra abriu a cifra: %v", err)
	}
}

func TestNotConfigured(t *testing.T) {
	b, err := New("")
	if err != nil || b.Ready() {
		t.Fatalf("vazio deveria desligar: %v", err)
	}
	if _, err := b.Seal([]byte("x"), nil); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("seal sem chave: %v", err)
	}
	if _, err := New("curta"); err == nil {
		t.Fatal("chave inválida aceita")
	}
}
