// Smoke test do repositório contra um Postgres real (pula sem
// APP_DATABASE_URL). A verificação na API da Anthropic precisa de chave de
// verdade e não roda aqui — só o formato.
package ai

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"testing"

	idb "github.com/belia/tevunah/backend/internal/db"
	"github.com/belia/tevunah/backend/internal/secretbox"
)

func TestCheckKeyFormat(t *testing.T) {
	cases := map[string]error{
		"sk-ant-api03-" + "abcdefghijklmnopqrstuvwxyz0123456789": nil,
		"sk-ant-admin01-abcdefghijklmnopqrstuvwxyz0123":          ErrKeyAdmin,
		"sk-proj-abcdefghijklmnopqrstuvwxyz0123456789":           ErrKeyFormat,
		"sk-ant-curta": ErrKeyFormat,
		"sk-ant-api03-com espaco-abcdefghijklmnopqrstuvwxyz": ErrKeyFormat,
	}
	for k, want := range cases {
		if got := CheckKeyFormat(k); !errors.Is(got, want) {
			t.Errorf("CheckKeyFormat(%q) = %v, want %v", k, got, want)
		}
	}
	if h := KeyHint("sk-ant-api03-abcdefghijklmnopqrstuvwxyzA1B2"); h != "sk-ant-…A1B2" {
		t.Errorf("hint = %q", h)
	}
}

func TestSmoke_Repo(t *testing.T) {
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL não definido")
	}
	db, err := idb.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	k := make([]byte, 32)
	_, _ = rand.Read(k)
	box, err := secretbox.New(base64.StdEncoding.EncodeToString(k))
	if err != nil {
		t.Fatal(err)
	}
	r := New(db, box, "America/Fortaleza")

	var user string
	if err := db.QueryRowContext(ctx, `
		SELECT u.id FROM app.users u
		 WHERE NOT EXISTS (SELECT 1 FROM app.user_ai_settings s
		                    WHERE s.user_id = u.id AND (s.api_key_enc IS NOT NULL OR s.model IS NOT NULL))
		 ORDER BY u.created_at
		 LIMIT 1`).Scan(&user); err != nil {
		t.Skipf("sem usuário livre para o teste: %v", err)
	}
	// Sem DELETE para o role da aplicação: o teste devolve a linha ao estado
	// "nunca configurado" (sem chave, modelo e perfil nulos = padrões), para
	// não deixar escolha fictícia na conta de um usuário real.
	defer func() {
		if _, err := db.ExecContext(ctx, `
			UPDATE app.user_ai_settings
			   SET api_key_enc = NULL, api_key_hint = NULL, key_verified_at = NULL,
			       available_models = '{}', model = NULL
			 WHERE user_id = $1`, user); err != nil {
			t.Errorf("limpeza: %v", err)
		}
		if s, err := r.Get(ctx, user); err != nil || s.HasKey || s.Model != DefaultModel {
			t.Errorf("limpeza deixou resíduo: %+v %v", s, err)
		}
	}()

	s, err := r.Get(ctx, user)
	if err != nil || s.HasKey || s.Model != DefaultModel {
		t.Fatalf("padrões = %+v, %v", s, err)
	}
	if _, _, err := r.Credentials(ctx, user); !errors.Is(err, ErrNoKey) {
		t.Fatalf("credentials sem chave = %v", err)
	}

	key := "sk-ant-api03-teste-ficticio-0123456789abcdefWXYZ"
	if err := r.SaveKey(ctx, user, key, []string{"claude-opus-5-5", "claude-haiku-5-5"}); err != nil {
		t.Fatal(err)
	}
	var stored []byte
	_ = db.QueryRowContext(ctx, `SELECT api_key_enc FROM app.user_ai_settings WHERE user_id = $1`, user).Scan(&stored)
	if len(stored) == 0 || string(stored) == key {
		t.Fatal("chave gravada em claro ou vazia")
	}
	if err := r.SaveModel(ctx, user, "claude-sonnet-5-5"); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveModel(ctx, user, "gpt-qualquer"); !errors.Is(err, ErrInvalidModel) {
		t.Errorf("modelo inválido aceito: %v", err)
	}
	gotKey, gotModel, err := r.Credentials(ctx, user)
	if err != nil || gotKey != key || gotModel != "claude-sonnet-5-5" {
		t.Fatalf("credentials = %q %q %v", gotKey, gotModel, err)
	}
	s, _ = r.Get(ctx, user)
	if !s.HasKey || s.KeyHint != "sk-ant-…WXYZ" || len(s.AvailableModels) != 2 {
		t.Errorf("get = %+v", s)
	}

	if err := r.ClearKey(ctx, user); err != nil {
		t.Fatal(err)
	}
	s, _ = r.Get(ctx, user)
	if s.HasKey || s.Model != "claude-sonnet-5-5" {
		t.Errorf("após remover = %+v (modelo deveria ficar)", s)
	}
}
