// Smoke test que exercita o registro de consultas contra um Postgres real.
// Pula se APP_DATABASE_URL não estiver definido (ambiente local sem DB).
package consultas

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	idb "github.com/belia/tevunah/backend/internal/db"
)

func TestSmoke_CreateListUpdateSoftDelete(t *testing.T) {
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL não definido")
	}
	db, err := idb.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	// Dois usuários: o autor e um colega, que não pode ver o registro do autor.
	var author, other string
	rows, err := db.QueryContext(ctx, `SELECT id FROM app.users ORDER BY created_at LIMIT 2`)
	if err != nil {
		t.Fatalf("buscar usuários: %v", err)
	}
	for _, dst := range []*string{&author, &other} {
		if !rows.Next() {
			rows.Close()
			t.Skip("o teste precisa de dois usuários em app.users")
		}
		if err := rows.Scan(dst); err != nil {
			t.Fatalf("scan: %v", err)
		}
	}
	rows.Close()

	r := New(db)
	// Marca única por execução: o papel da aplicação não apaga linhas, então
	// o resíduo (soft-deletado) de execuções anteriores continua na tabela.
	tag := fmt.Sprintf("SMOKE-%d", time.Now().UnixNano())
	when := time.Now().Add(-time.Hour).Truncate(time.Second)

	created, err := r.Create(ctx, Fields{
		PerformedAt:  when,
		System:       "  sistema " + tag + " ",
		Target:       "FULANO DE TAL",
		Requester:    "CAP BELTRANO",
		RequesterOrg: "2º BPRAIO",
		ReferenceDoc: "OF 123/2026",
		Reason:       "apoio a investigação",
		Result:       "nada consta",
	}, author)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	system := "SISTEMA " + tag
	if created.System != system {
		t.Errorf("sistema deveria ser aparado e em caixa alta: %q", created.System)
	}
	if !created.PerformedAt.Equal(when) {
		t.Errorf("performed_at = %v, esperado %v", created.PerformedAt, when)
	}
	if created.Version != 1 || created.CreatedBy != author {
		t.Errorf("version/autor inesperados: %d %s", created.Version, created.CreatedBy)
	}

	has := func(opts ListOpts) bool {
		t.Helper()
		opts.Search = tag
		res, err := r.List(ctx, opts)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		for _, c := range res.Items {
			if c.ID == created.ID {
				return true
			}
		}
		return false
	}
	day := when.UTC().Format("2006-01-02")
	if !has(ListOpts{UserID: author}) {
		t.Error("o autor deveria ver o próprio registro")
	}
	if has(ListOpts{UserID: other}) {
		t.Error("um colega não deveria ver o registro de outro agente")
	}
	if !has(ListOpts{UserID: other, SeeAll: true}) {
		t.Error("quem tem read_all deveria ver o registro de outro agente")
	}
	if !has(ListOpts{UserID: author, DateFrom: day, DateTo: day, TZ: "UTC"}) {
		t.Error("o registro deveria aparecer no recorte do próprio dia")
	}
	if has(ListOpts{UserID: author, DateTo: "2000-01-01", TZ: "UTC"}) {
		t.Error("o registro não deveria aparecer num recorte anterior")
	}

	systems, err := r.Systems(ctx)
	if err != nil {
		t.Fatalf("systems: %v", err)
	}
	found := false
	for _, s := range systems {
		found = found || s == system
	}
	if !found {
		t.Errorf("sistema %q deveria estar nas sugestões", system)
	}

	updated, err := r.Update(ctx, created.ID, Fields{
		PerformedAt: when, System: system, Target: "FULANO DE TAL",
		Requester: "CAP BELTRANO", Reason: "apoio a investigação",
		Result: "mandado de prisão em aberto",
	}, author)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != 2 || updated.Result != "mandado de prisão em aberto" || updated.RequesterOrg != "" {
		t.Errorf("update não aplicado: %+v", updated)
	}

	if err := r.SoftDelete(ctx, created.ID, author); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := r.FindByID(ctx, created.ID); err != ErrNotFound {
		t.Errorf("registro excluído deveria dar ErrNotFound, deu %v", err)
	}
	if has(ListOpts{UserID: author}) {
		t.Error("registro excluído não deveria aparecer na listagem")
	}
	if err := r.SoftDelete(ctx, created.ID, author); err != ErrNotFound {
		t.Errorf("excluir de novo deveria dar ErrNotFound, deu %v", err)
	}
}
