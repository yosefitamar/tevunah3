// Smoke test que exercita CRUD ponta-a-ponta contra um Postgres real.
// Pula se APP_DATABASE_URL não estiver definido (ambiente local sem DB).
package entities

import (
	"context"
	"os"
	"testing"

	idb "github.com/belia/tevunah/backend/internal/db"
)

func TestSmoke_CreateUpdateSoftDelete(t *testing.T) {
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL não definido")
	}
	db, err := idb.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Pega um usuário existente para servir de actor (created_by FK).
	ctx := context.Background()
	var actor string
	if err := db.QueryRowContext(ctx, `SELECT id FROM app.users LIMIT 1`).Scan(&actor); err != nil {
		t.Fatalf("buscar actor: %v", err)
	}

	r := New(db)

	gender := "M"
	motherName := "Maria de Souza"
	cpf := "12345678901"
	created, err := r.Create(ctx, NewEntity{
		Kind:           KindPerson,
		Name:           "Smoke Test Person",
		Description:    "criado por teste automático",
		Classification: 2,
		Tags:           []string{"smoke", "Test", "smoke"}, // dedupe + lower-case
		Person: &PersonAttrs{
			Aliases:    []string{"Sandman", "Pessoa Teste"},
			Gender:     &gender,
			MotherName: &motherName,
			CPF:        &cpf,
		},
	}, actor)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM app.entity_tags WHERE entity_id = $1`, created.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM app.entity_persons WHERE entity_id = $1`, created.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM app.entities WHERE id = $1`, created.ID)
	}()

	if created.Version != 1 {
		t.Errorf("version inicial esperado 1, obtido %d", created.Version)
	}
	if got := len(created.Tags); got != 2 {
		t.Errorf("tags esperadas 2 (dedupe+lower), obtidas %d (%v)", got, created.Tags)
	}
	if created.Person == nil || len(created.Person.Aliases) != 2 {
		t.Fatalf("person attrs não carregadas: %+v", created.Person)
	}

	// Update parcial
	newName := "Smoke Test Person (renomeado)"
	newClass := 3
	newTags := []string{"renomeado"}
	_, after, err := r.Update(ctx, created.ID, created.Version, Patch{
		Name:           &newName,
		Classification: &newClass,
		Tags:           &newTags,
	}, actor)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if after.Version != 2 {
		t.Errorf("version após update esperado 2, obtido %d", after.Version)
	}
	if after.Classification != 3 {
		t.Errorf("classification após update esperado 3, obtido %d", after.Classification)
	}
	if len(after.Tags) != 1 || after.Tags[0] != "renomeado" {
		t.Errorf("tags após update: %v", after.Tags)
	}

	// Optimistic lock: tentar update com versão velha deve falhar.
	_, _, err = r.Update(ctx, created.ID, created.Version, Patch{Name: &newName}, actor)
	if err != ErrVersionConflict {
		t.Errorf("esperado ErrVersionConflict, obtido %v", err)
	}

	// List por kind
	listRes, err := r.List(ctx, ListOpts{
		Kind: KindPerson, MaxClearance: 5, Search: "Smoke Test",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, e := range listRes.Items {
		if e.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("entidade criada não apareceu na listagem")
	}

	// Soft delete
	before, err := r.SoftDelete(ctx, created.ID, actor)
	if err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if before.DeletedAt != nil {
		t.Errorf("retorno do soft delete deveria ser o estado anterior (sem deleted_at)")
	}
	// Tentar deletar de novo
	if _, err := r.SoftDelete(ctx, created.ID, actor); err != ErrAlreadyDeleted {
		t.Errorf("esperado ErrAlreadyDeleted, obtido %v", err)
	}
	// FindByID retorna mesmo soft-deletado
	again, err := r.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find after delete: %v", err)
	}
	if again.DeletedAt == nil {
		t.Errorf("deleted_at deveria estar populado")
	}
}
