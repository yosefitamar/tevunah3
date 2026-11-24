package ai

import (
	"context"
	"database/sql"
	"os"
	"testing"

	idb "github.com/belia/tevunah/backend/internal/db"
)

func smokeRepo(t *testing.T) (*Repo, *sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL não definido")
	}
	db, err := idb.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var userID string
	if err := db.QueryRow(`SELECT id FROM app.users ORDER BY created_at, id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	return New(db, nil, "America/Fortaleza"), db, userID
}

// Ciclo da orientação num banco real, dentro de uma limpeza no fim (a linha
// de teste fica desativada; o schema nega DELETE à aplicação, então o teste
// usa o feature próprio "teste_smoke" para não aparecer em lugar nenhum).
func TestSmoke_GuidanceLifecycle(t *testing.T) {
	r, db, userID := smokeRepo(t)
	ctx := context.Background()
	const feature = "teste_smoke"
	t.Cleanup(func() {
		db.ExecContext(ctx, `UPDATE app.ai_guidance SET status = 'desativada' WHERE feature = $1`, feature)
	})

	if _, err := r.CreateGuidance(ctx, feature, userID, GuidanceInput{Topic: "bairro", Body: "oi"}); err != ErrGuidanceText {
		t.Fatalf("texto curto: %v", err)
	}
	g, err := r.CreateGuidance(ctx, feature, userID, GuidanceInput{Topic: "bairro", Body: "  Zona   Rural vai como Centro "})
	if err != nil {
		t.Fatal(err)
	}
	if g.Status != GuidancePending || g.Body != "Zona Rural vai como Centro" {
		t.Fatalf("criada: %+v", g)
	}
	// Pendente vale para o autor, não para os outros.
	mine, _ := r.ActiveGuidance(ctx, feature, userID)
	other, _ := r.ActiveGuidance(ctx, feature, "00000000-0000-0000-0000-000000000000")
	if !hasGuidance(mine, g.ID) || hasGuidance(other, g.ID) {
		t.Fatal("pendente deve valer só para o autor")
	}
	if _, err := r.DecideGuidance(ctx, g.ID, GuidanceApproved, userID, "ok"); err != nil {
		t.Fatal(err)
	}
	other, _ = r.ActiveGuidance(ctx, feature, "00000000-0000-0000-0000-000000000000")
	if !hasGuidance(other, g.ID) {
		t.Fatal("aprovada deve valer para todos")
	}
	if _, err := r.UpdateGuidance(ctx, g.ID, GuidanceInput{Topic: "bairro", Body: "outra coisa"}); err != ErrGuidanceState {
		t.Fatalf("aprovada não se edita: %v", err)
	}
	if err := r.MarkGuidanceUsed(ctx, []string{g.ID}); err != nil {
		t.Fatal(err)
	}
	if g, _ = r.GetGuidance(ctx, g.ID); g.Uses != 1 {
		t.Fatalf("uso: %d", g.Uses)
	}
	if _, err := r.DecideGuidance(ctx, g.ID, GuidanceDisabled, userID, ""); err != nil {
		t.Fatal(err)
	}
	if mine, _ = r.ActiveGuidance(ctx, feature, userID); hasGuidance(mine, g.ID) {
		t.Fatal("desativada não vale")
	}
}

func hasGuidance(list []Guidance, id string) bool {
	for _, g := range list {
		if g.ID == id {
			return true
		}
	}
	return false
}
