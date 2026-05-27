package crypt

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestVerifyBcrypt2yLaravel(t *testing.T) {
	// Hash bcrypt do dump legado (linha real do user admin@tevunah.com).
	const hash = "$2y$12$34NRaL5Tj6UkvIhmd8ogb.oolQl4VxRDwE7bXeSKcOY7153rpQF4."
	// Senha correta é desconhecida — só validamos o caminho de erro/mismatch
	// (não panic, retorna false sem erro pra senha errada).
	ok, err := Verify("senha-errada", hash)
	if err != nil {
		t.Fatalf("verify retornou erro: %v", err)
	}
	if ok {
		t.Fatal("verify aceitou senha errada")
	}
	if !NeedsRehash(hash) {
		t.Fatal("NeedsRehash deveria ser true para bcrypt")
	}
}

func TestVerifyBcryptRoundTrip(t *testing.T) {
	// Gera hash bcrypt e valida via Verify. Cobre tanto o prefixo $2a$
	// (nativo do bcrypt do Go) quanto $2y$ (Laravel) — substituímos o prefixo
	// pra simular o que vem do dump.
	raw, err := bcrypt.GenerateFromPassword([]byte("hunter2"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("gerar bcrypt: %v", err)
	}
	hashes := []string{
		string(raw),
		"$2y$" + strings.TrimPrefix(string(raw), "$2a$"),
	}
	for _, h := range hashes {
		ok, err := Verify("hunter2", h)
		if err != nil || !ok {
			t.Errorf("verify(%s): ok=%v err=%v", h[:4], ok, err)
		}
		bad, err := Verify("errada", h)
		if err != nil || bad {
			t.Errorf("verify(%s) senha errada aceita: ok=%v err=%v", h[:4], bad, err)
		}
		if !NeedsRehash(h) {
			t.Errorf("NeedsRehash(%s) deveria ser true", h[:4])
		}
	}
}

func TestArgon2idStillWorks(t *testing.T) {
	h, err := Hash("hunter2")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if NeedsRehash(h) {
		t.Fatal("hash recém-gerado não deveria precisar rehash")
	}
	ok, err := Verify("hunter2", h)
	if err != nil || !ok {
		t.Fatalf("verify falhou: ok=%v err=%v", ok, err)
	}
	bad, err := Verify("errada", h)
	if err != nil || bad {
		t.Fatalf("verify aceitou senha errada: ok=%v err=%v", bad, err)
	}
}
