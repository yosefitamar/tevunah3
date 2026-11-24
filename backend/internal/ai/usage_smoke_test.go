package ai

import (
	"context"
	"math"
	"os"
	"testing"

	idb "github.com/belia/tevunah/backend/internal/db"
)

func TestModelCost(t *testing.T) {
	m := *FindModel("claude-opus-5-5") // US$ 4 entrada, 20 saída, 0,20 leitura de cache
	got := m.Cost(1_000_000, 500_000, 100_000, 2_000_000)
	want := 4.0 + 10.0 + 0.5 + 0.4 // escrita de cache = 1,25 × 4 × 0,1M
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("custo = %v, want %v", got, want)
	}
}

// TestSmoke_Spending grava chamadas e confere as somas do mês e do ano. Roda
// numa transação desfeita no fim: app.ai_usage é append-only e o role da
// aplicação não apaga — o teste não pode deixar gasto fictício na conta de
// um usuário real.
func TestSmoke_Spending(t *testing.T) {
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

	var user string
	if err := db.QueryRowContext(ctx, `SELECT id FROM app.users ORDER BY created_at LIMIT 1`).Scan(&user); err != nil {
		t.Skipf("sem usuário: %v", err)
	}
	before, err := New(db, nil, "America/Fortaleza").Spending(ctx, user)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	cost := func(model string, in, out int) float64 { return FindModel(model).Cost(in, out, 0, 0) }
	ins := func(model, when string, in, out int) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO app.ai_usage (user_id, model, feature, input_tokens, output_tokens, cost_usd, created_at)
			VALUES ($1, $2, 'teste', $3, $4, $5, `+when+`)`,
			user, model, in, out, cost(model, in, out)); err != nil {
			t.Fatal(err)
		}
	}
	// Mês corrente (2×), início do ano corrente e ano passado (fica fora).
	ins("claude-haiku-5-5", "now()", 10_000, 2_000)
	ins("claude-opus-5-5", "now()", 1_000, 500)
	ins("claude-haiku-5-5", "date_trunc('year', now() AT TIME ZONE 'America/Fortaleza') AT TIME ZONE 'America/Fortaleza' + interval '1 hour'", 5_000, 1_000)
	ins("claude-haiku-5-5", "now() - interval '400 days'", 9_000_000, 9_000_000)

	// Spending lê pelo *sql.DB; para enxergar a transação, consulta via tx.
	r := &Repo{tz: "America/Fortaleza"}
	got, err := r.spendingQ(ctx, tx, user)
	if err != nil {
		t.Fatal(err)
	}
	monthCost := cost("claude-haiku-5-5", 10_000, 2_000) + cost("claude-opus-5-5", 1_000, 500)
	yearCost := monthCost + cost("claude-haiku-5-5", 5_000, 1_000)
	// O mês de janeiro coincide com o início do ano: aí a 3ª linha conta no mês.
	sameMonth := got.Month.Requests-before.Month.Requests == 3
	if sameMonth {
		monthCost = yearCost
	}
	if d := got.Month.CostUSD - before.Month.CostUSD; math.Abs(d-monthCost) > 1e-6 {
		t.Errorf("mês = %v, want %v", d, monthCost)
	}
	if d := got.Year.CostUSD - before.Year.CostUSD; math.Abs(d-yearCost) > 1e-6 {
		t.Errorf("ano = %v, want %v", d, yearCost)
	}
	if d := got.Year.Requests - before.Year.Requests; d != 3 {
		t.Errorf("requisições no ano = %d, want 3", d)
	}
	if d := got.Year.InputTokens - before.Year.InputTokens; d != 16_000 {
		t.Errorf("tokens de entrada no ano = %d, want 16000", d)
	}
}
