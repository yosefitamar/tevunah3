// Smoke test da normalização de nomes de pessoa contra um Postgres real.
// Pula se APP_DATABASE_URL não estiver definido (ambiente local sem DB).
package entities

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	idb "github.com/belia/tevunah/backend/internal/db"
)

func TestStripAccents(t *testing.T) {
	cases := map[string]string{
		"JOSÉ DA CONCEIÇÃO": "JOSE DA CONCEICAO",
		"Ângela Muñoz":      "Angela Munoz",
		"SEM ACENTO":        "SEM ACENTO",
		"":                  "",
	}
	for in, want := range cases {
		if got := stripAccents(in); got != want {
			t.Errorf("stripAccents(%q) = %q, esperado %q", in, got, want)
		}
	}
}

// TestSmoke_PersonNameNormalization cobre as duas pontas da regra: a
// gravação canoniza o nome da pessoa (sem acento, MAIÚSCULAS) e a leitura
// encontra o registro com ou sem acento no termo — inclusive na detecção de
// homônimo, onde deixar "JOSÉ" e "JOSE" passarem como pessoas diferentes é o
// erro que a regra existe para evitar.
func TestSmoke_PersonNameNormalization(t *testing.T) {
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
	r := New(db)

	// Sufixo único: o teste busca por nome exato e não pode casar com
	// resíduo de execuções anteriores (não há hard delete).
	tag := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	name := "JOSÉ DA CONCEIÇÃO " + tag
	wantName := "JOSE DA CONCEICAO " + tag
	mother := "Maria Antônia Gonçalves"
	created, err := r.Create(ctx, NewEntity{
		Kind:           KindPerson,
		Name:           name,
		Classification: 1,
		Person: &PersonAttrs{
			Aliases:    []string{"Zé Cabeção"},
			MotherName: &mother,
		},
	}, actor)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx,
			`UPDATE app.entities SET deleted_at = now(), deleted_by = $2
			  WHERE id = $1 AND deleted_at IS NULL`, created.ID, actor); err != nil {
			t.Errorf("limpeza: %v", err)
		}
	}()

	if created.Name != wantName {
		t.Errorf("nome gravado = %q, esperado %q", created.Name, wantName)
	}
	if created.Person == nil {
		t.Fatal("person attrs não carregadas")
	}
	if got := created.Person.Aliases; len(got) != 1 || got[0] != "ZE CABECAO" {
		t.Errorf("alcunhas gravadas = %v, esperado [ZE CABECAO]", got)
	}
	if got := created.Person.MotherName; got == nil || *got != "MARIA ANTONIA GONCALVES" {
		t.Errorf("nome da mãe gravado = %v, esperado MARIA ANTONIA GONCALVES", got)
	}

	// Busca: o termo chega dos dois jeitos e tem de achar o mesmo registro.
	for _, term := range []string{"josé da conceição " + tag, "JOSE DA CONCEICAO " + tag, "zé cabeção"} {
		res, err := r.List(ctx, ListOpts{MaxClearance: 5, Search: term})
		if err != nil {
			t.Fatalf("list(%q): %v", term, err)
		}
		found := false
		for _, e := range res.Items {
			if e.ID == created.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("busca por %q não achou a entidade criada", term)
		}
	}

	// Homônimo: o nome acentuado tem de bater com o registro canonizado, e o
	// nome da mãe acentuado tem de somar ponto ao score.
	dupes, err := r.FindPersonDuplicates(ctx, DuplicatesQuery{
		Name:         name,
		MotherName:   mother,
		MaxClearance: 5,
	})
	if err != nil {
		t.Fatalf("duplicates: %v", err)
	}
	var match *PersonDuplicate
	for i := range dupes.Matches {
		if dupes.Matches[i].ID == created.ID {
			match = &dupes.Matches[i]
		}
	}
	if match == nil {
		t.Fatalf("homônimo com acento não foi detectado (matches: %d)", len(dupes.Matches))
	}
	if match.Score != 2 {
		t.Errorf("score = %d, esperado 2 (nome + nome da mãe)", match.Score)
	}
	if len(match.MatchedFields) != 2 {
		t.Errorf("matched_fields = %v, esperado nome e nome da mãe", match.MatchedFields)
	}
}
