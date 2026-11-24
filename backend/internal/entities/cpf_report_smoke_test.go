// Smoke test dos campos que o RELATÓRIO DE CPF preenche (pai, naturalidade,
// RG, telefones) e da busca por CPF, contra um Postgres real.
// Pula se APP_DATABASE_URL não estiver definido.
package entities

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	idb "github.com/belia/tevunah/backend/internal/db"
)

func TestSmoke_PersonCPFReportFields(t *testing.T) {
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

	// CPF fictício e único por execução (o índice único cobre também os
	// excluídos, então não pode repetir entre execuções).
	cpf := fmt.Sprintf("999%08d", time.Now().UnixNano()%100000000)
	father := "José Ficticio"
	birthplace := "sobral/ce"
	rg := "2007975651-9"
	issuer := "ssp"
	state := "ce"
	issued := time.Date(2011, 9, 2, 0, 0, 0, 0, time.UTC)
	created, err := r.Create(ctx, NewEntity{
		Kind:           KindPerson,
		Name:           "PESSOA FICTICIA " + cpf,
		Classification: 1,
		Person: &PersonAttrs{
			CPF:        &cpf,
			FatherName: &father,
			Birthplace: &birthplace,
			RGNumber:   &rg,
			RGIssuer:   &issuer,
			RGState:    &state,
			RGIssuedOn: &issued,
			Phones:     []string{"(85) 98957-7505", "85989577505", "(85) 3101-2330"},
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

	p := created.Person
	str := func(s *string) string {
		if s == nil {
			return "<nil>"
		}
		return *s
	}
	if got := str(p.FatherName); got != "JOSE FICTICIO" {
		t.Errorf("father_name = %q", got)
	}
	if got := str(p.Birthplace); got != "SOBRAL/CE" {
		t.Errorf("birthplace = %q", got)
	}
	if got := str(p.RGNumber) + " " + str(p.RGIssuer) + "/" + str(p.RGState); got != "20079756519 SSP/CE" {
		t.Errorf("rg = %q", got)
	}
	if p.RGIssuedOn == nil || p.RGIssuedOn.Format("2006-01-02") != "2011-09-02" {
		t.Errorf("rg_issued_on = %v", p.RGIssuedOn)
	}
	if want := []string{"85989577505", "8531012330"}; !reflect.DeepEqual(p.Phones, want) {
		t.Errorf("phones = %v, want %v", p.Phones, want)
	}

	// Update parcial: campo omitido mantém; telefones enviados substituem.
	nat := "brasileiro"
	_, updated, err := r.Update(ctx, created.ID, created.Version, Patch{
		Person: &PersonAttrs{Nationality: &nat, Phones: []string{"88999990000"}},
	}, actor)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := str(updated.Person.Nationality); got != "BRASILEIRO" {
		t.Errorf("nationality = %q", got)
	}
	if got := str(updated.Person.FatherName); got != "JOSE FICTICIO" {
		t.Errorf("father_name após update = %q (deveria manter)", got)
	}
	if want := []string{"88999990000"}; !reflect.DeepEqual(updated.Person.Phones, want) {
		t.Errorf("phones após update = %v", updated.Person.Phones)
	}

	// Histórico criminal: normaliza, descarta item vazio, update substitui
	// a lista e update sem a lista mantém.
	_, withRecs, err := r.Update(ctx, created.ID, updated.Version, Patch{
		Person: &PersonAttrs{CriminalRecords: []CriminalRecord{
			{Category: "Policial", Role: "infrator", Title: "roubo", Date: "2021-04-20", FiledOn: "03/05/2021"},
			{Source: "CONSULTA INTEGRADA"},
		}},
	}, actor)
	if err != nil {
		t.Fatalf("update records: %v", err)
	}
	wantRecs := []CriminalRecord{{Category: "policial", Role: "INFRATOR", Title: "ROUBO", Date: "2021-04-20"}}
	if !reflect.DeepEqual(withRecs.Person.CriminalRecords, wantRecs) {
		t.Errorf("criminal_records = %+v", withRecs.Person.CriminalRecords)
	}
	_, kept, err := r.Update(ctx, created.ID, withRecs.Version, Patch{
		Person: &PersonAttrs{Nationality: &nat},
	}, actor)
	if err != nil {
		t.Fatalf("update sem records: %v", err)
	}
	if !reflect.DeepEqual(kept.Person.CriminalRecords, wantRecs) {
		t.Errorf("criminal_records após update sem lista = %+v (deveria manter)", kept.Person.CriminalRecords)
	}

	// Busca por CPF: visível com clearance suficiente.
	found, err := r.FindPersonByCPF(ctx, cpf, 5)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if found.Entity == nil || found.Entity.ID != created.ID || found.Blocked {
		t.Errorf("lookup = %+v, esperado a pessoa criada", found)
	}
	if free, err := r.FindPersonByCPF(ctx, "99900000000", 5); err != nil || free.Entity != nil || free.Blocked {
		t.Errorf("lookup de CPF livre = %+v, %v", free, err)
	}

	// Excluída: o CPF continua ocupado, mas a pessoa não pode ser usada.
	if _, err := db.ExecContext(ctx,
		`UPDATE app.entities SET deleted_at = now(), deleted_by = $2 WHERE id = $1`,
		created.ID, actor); err != nil {
		t.Fatalf("excluir: %v", err)
	}
	blocked, err := r.FindPersonByCPF(ctx, cpf, 5)
	if err != nil {
		t.Fatalf("lookup após excluir: %v", err)
	}
	if blocked.Entity != nil || !blocked.Blocked {
		t.Errorf("lookup após excluir = %+v, esperado Blocked", blocked)
	}
}
