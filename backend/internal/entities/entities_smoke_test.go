// Smoke test que exercita CRUD ponta-a-ponta contra um Postgres real.
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
	// CPF único por execução: o índice entity_persons_cpf_uniq é global (não
	// filtra deleted_at) e o papel da aplicação não tem DELETE nesta tabela,
	// então um CPF fixo faria a segunda execução falhar com "CPF já
	// cadastrado" sobre o resíduo da primeira.
	cpf := fmt.Sprintf("%011d", time.Now().UnixNano()%100000000000)
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
	// Descarte pelo caminho do produto: o papel da aplicação não tem DELETE em
	// entities/entity_persons (o app é soft-delete por design), então marcar
	// deleted_at é o mais próximo de limpar que o teste consegue — e já tira o
	// registro das listagens.
	defer func() {
		if _, err := db.ExecContext(ctx,
			`DELETE FROM app.entity_tags WHERE entity_id = $1`, created.ID); err != nil {
			t.Errorf("limpeza (entity_tags): %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`UPDATE app.entities SET deleted_at = now(), deleted_by = $2
			  WHERE id = $1 AND deleted_at IS NULL`, created.ID, actor); err != nil {
			t.Errorf("limpeza (entities): %v", err)
		}
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
