// Smoke test do painel contra um Postgres real. O que se quer provar aqui
// não é o número — é que cada query casa com o schema e com a quantidade de
// parâmetros que recebe (o Postgres recusa bind sobrando, e as agregações
// montam o SQL condicionalmente conforme o solicitante seja admin ou não).
// Pula se APP_DATABASE_URL não estiver definido.
package dashboard

import (
	"context"
	"os"
	"testing"
	"time"

	idb "github.com/belia/tevunah/backend/internal/db"
)

func TestSmoke_Blocos(t *testing.T) {
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
	var actor string
	if err := db.QueryRowContext(ctx, `SELECT id FROM app.users LIMIT 1`).Scan(&actor); err != nil {
		t.Fatalf("buscar actor: %v", err)
	}

	repo := New(db, "America/Fortaleza")

	win, err := BuildWindow("2026-03-01", "2026-03-31", day("2026-03-19"))
	if err != nil {
		t.Fatalf("BuildWindow: %v", err)
	}
	// Recorte aberto: exercita o caminho em que as datas entram como NULL.
	open, err := BuildWindow("", "", time.Now())
	if err != nil {
		t.Fatalf("BuildWindow aberto: %v", err)
	}

	for _, ac := range []Access{
		{UserID: actor, Clearance: 3, IsAdmin: false},
		{UserID: actor, Clearance: 5, IsAdmin: true},
	} {
		for _, w := range []Window{win, open} {
			inc, err := repo.Incidents(ctx, w)
			if err != nil {
				t.Fatalf("Incidents (admin=%v): %v", ac.IsAdmin, err)
			}
			if len(inc.Series) != seriesMonths {
				t.Fatalf("série com %d meses, quero %d", len(inc.Series), seriesMonths)
			}
			if _, err := repo.Reports(ctx, w, ac); err != nil {
				t.Fatalf("Reports (admin=%v): %v", ac.IsAdmin, err)
			}
			if _, err := repo.Informes(ctx, w, ac); err != nil {
				t.Fatalf("Informes (admin=%v): %v", ac.IsAdmin, err)
			}
			if _, err := repo.Entities(ctx, w); err != nil {
				t.Fatalf("Entities (admin=%v): %v", ac.IsAdmin, err)
			}
		}
	}
}
