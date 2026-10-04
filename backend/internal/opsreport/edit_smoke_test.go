// Smoke tests contra um Postgres real: a correção da ocorrência importada e
// a referência de área aprendida (cidade + bairro → área da unidade militar).
// Pulam se APP_DATABASE_URL não estiver definido.
package opsreport

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	idb "github.com/belia/tevunah/backend/internal/db"
	"github.com/belia/tevunah/backend/internal/sipom"
)

func smokeDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL não definido")
	}
	db, err := idb.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	var actor string
	if err := db.QueryRowContext(context.Background(), `SELECT id FROM app.users LIMIT 1`).Scan(&actor); err != nil {
		t.Fatalf("buscar actor: %v", err)
	}
	return db, actor
}

// A correção usa uma ocorrência que já esteja no banco e a devolve ao estado
// original no fim — o teste não cria relatório, que não teria como descartar.
func TestSmoke_UpdateOccurrence(t *testing.T) {
	db, actor := smokeDB(t)
	ctx := context.Background()
	r := New(db)

	var ids []string
	rows, err := db.QueryContext(ctx, `
		SELECT id FROM app.ops_occurrences
		 WHERE deleted_at IS NULL AND ciops_record <> '' ORDER BY created_at, id LIMIT 2`)
	if err != nil {
		t.Fatalf("buscar ocorrências: %v", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) == 0 {
		t.Skip("sem ocorrência do relatório operacional no banco")
	}
	id := ids[0]
	orig, err := r.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	// Restaura os campos e a marca de edição, aconteça o que acontecer.
	defer func() {
		o := &orig.Occurrence
		if err := r.Update(ctx, id, actor, Edit{
			Natures: &o.Natures, OccurredOn: &o.OccurredOn, StartTime: &o.StartTime, EndTime: &o.EndTime,
			CIOPS: &o.CIOPS, PlaceNeighborhood: &o.PlaceNeighborhood, Narrative: &o.Narrative,
		}); err != nil {
			t.Errorf("restaurar ocorrência: %v", err)
		}
		if _, err := db.ExecContext(ctx, `
			UPDATE app.ops_occurrences SET updated_at = $2, updated_by = NULL WHERE id = $1`,
			id, orig.UpdatedAt); err != nil {
			t.Errorf("restaurar marca de edição: %v", err)
		}
	}()

	day := orig.OccurredOn.AddDate(0, 0, -1)
	start, end, bairro := "03:21", "", " smoke bairro "
	natures := []string{" furto  qualificado ", ""}
	if err := r.Update(ctx, id, actor, Edit{
		Natures: &natures, OccurredOn: &day, StartTime: &start, EndTime: &end, PlaceNeighborhood: &bairro,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := r.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID (depois): %v", err)
	}
	if !got.OccurredOn.Equal(day) || got.StartTime != "03:21" || got.EndTime != "" {
		t.Errorf("data/hora: %s %q–%q", got.OccurredOn.Format("2006-01-02"), got.StartTime, got.EndTime)
	}
	if got.PlaceNeighborhood != "SMOKE BAIRRO" {
		t.Errorf("bairro não normalizado: %q", got.PlaceNeighborhood)
	}
	if len(got.Natures) != 1 || got.Natures[0] != "FURTO QUALIFICADO" {
		t.Errorf("naturezas: %v", got.Natures)
	}
	// Campo não enviado fica como estava.
	if got.Narrative != orig.Narrative || got.CIOPS != orig.CIOPS || got.PlaceCity != orig.PlaceCity {
		t.Errorf("campo não enviado foi alterado")
	}
	if got.UpdatedAt == nil {
		t.Errorf("edição não marcou updated_at")
	}

	// A ficha não pode passar a ser a de outra ocorrência do relatório.
	if len(ids) > 1 {
		other, err := r.FindByID(ctx, ids[1])
		if err != nil {
			t.Fatalf("FindByID (outra): %v", err)
		}
		if err := r.Update(ctx, id, actor, Edit{CIOPS: &other.CIOPS}); !errors.Is(err, ErrDuplicateCIOPS) {
			t.Errorf("ficha de outra ocorrência: esperado ErrDuplicateCIOPS, obtido %v", err)
		}
	}
	if err := r.Update(ctx, "00000000-0000-0000-0000-000000000000", actor, Edit{Narrative: &bairro}); !errors.Is(err, ErrNotFound) {
		t.Errorf("id inexistente: esperado ErrNotFound, obtido %v", err)
	}
}

// A área escolhida pelo analista vira referência do lugar e passa a resolver
// a pendência das próximas ocorrências dali.
func TestSmoke_AreaRule(t *testing.T) {
	db, actor := smokeDB(t)
	ctx := context.Background()
	cat, err := sipom.Load(ctx, db)
	if err != nil {
		t.Skipf("catálogo do SIPOM não carregado: %v", err)
	}
	// Duas companhias ativas quaisquer.
	var areas []int
	rows, err := db.QueryContext(ctx, `SELECT id FROM sipom.companhias WHERE deletado_em IS NULL ORDER BY id`)
	if err != nil {
		t.Fatalf("companhias: %v", err)
	}
	for rows.Next() && len(areas) < 2 {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		if cat.IsActiveCompany(id) {
			areas = append(areas, id)
		}
	}
	rows.Close()
	if len(areas) < 2 {
		t.Skip("catálogo sem duas companhias ativas")
	}

	// Lugar que não existe no catálogo: sem referência, não há área.
	const city, bairro = "Smoke City do Teste", "Bairro São Smoke"
	repo := sipom.NewAreaRuleRepo(db)
	// O papel da aplicação não apaga; quando o ambiente tem a conexão das
	// migrations (superusuário), a referência de teste é removida no fim.
	defer func() {
		dsn := os.Getenv("MIGRATIONS_DATABASE_URL")
		if dsn == "" {
			return
		}
		if su, err := idb.Open(dsn); err == nil {
			defer su.Close()
			_, _ = su.ExecContext(ctx, `DELETE FROM app.sipom_area_rules WHERE city_key LIKE 'smoke city%'`)
		}
	}()

	translate := func() *Occurrence {
		rules, err := repo.Load(ctx)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		o := &Occurrence{
			Natures: []string{"FURTO"}, StartTime: "10:00", CIOPS: "SMK1",
			PlaceAddress: "Rua Um, 10", PlaceNeighborhood: "BAIRRO SAO SMOKE", PlaceCity: "SMOKE CITY DO TESTE",
		}
		TranslateSipom(cat, SipomRefs{Areas: rules}, o)
		return o
	}

	if _, err := repo.Learn(ctx, "", bairro, areas[0], actor); !errors.Is(err, sipom.ErrAreaRuleNoCity) {
		t.Errorf("sem cidade: esperado ErrAreaRuleNoCity, obtido %v", err)
	}

	changed, err := repo.Learn(ctx, city, bairro, areas[0], actor)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	if !changed {
		// Sobra de um run anterior sem limpeza: força a troca para seguir.
		if _, err := repo.Learn(ctx, city, bairro, areas[1], actor); err != nil {
			t.Fatalf("Learn (reset): %v", err)
		}
		if changed, err = repo.Learn(ctx, city, bairro, areas[0], actor); err != nil || !changed {
			t.Fatalf("Learn (após reset): changed=%v err=%v", changed, err)
		}
	}
	// A grafia do relatório (caixa, acento) não importa: o lugar é o mesmo.
	o := translate()
	if o.Sipom.AreaID == nil || *o.Sipom.AreaID != areas[0] {
		t.Fatalf("área não veio da referência: %v (pendências %v)", o.Sipom.AreaID, o.Sipom.Pending)
	}
	if hasString(o.Sipom.Pending, sipom.PendArea) || hasString(o.Sipom.Pending, sipom.PendAreaAmbig) {
		t.Errorf("referência não tirou a pendência de área: %v", o.Sipom.Pending)
	}

	// Mesma escolha de novo não muda nada; outra área substitui.
	if changed, err := repo.Learn(ctx, city, bairro, areas[0], actor); err != nil || changed {
		t.Errorf("mesma referência: changed=%v err=%v", changed, err)
	}
	if changed, err := repo.Learn(ctx, city, bairro, areas[1], actor); err != nil || !changed {
		t.Errorf("troca de referência: changed=%v err=%v", changed, err)
	}
	if o := translate(); o.Sipom.AreaID == nil || *o.Sipom.AreaID != areas[1] {
		t.Errorf("referência trocada não valeu: %v", o.Sipom.AreaID)
	}

	// A referência é do par cidade + bairro: outro bairro continua sem área.
	rules, _ := repo.Load(ctx)
	if _, ok := rules.Lookup(city, "OUTRO BAIRRO"); ok {
		t.Errorf("referência de um bairro valeu para outro")
	}
	if _, ok := rules.Lookup(city, ""); ok {
		t.Errorf("referência de um bairro valeu para a cidade inteira")
	}

	// Área fixada pelo analista na própria ficha vale sobre a referência.
	fixed := &Occurrence{
		Natures: []string{"FURTO"}, StartTime: "10:00", CIOPS: "SMK2",
		PlaceNeighborhood: "BAIRRO SAO SMOKE", PlaceCity: "SMOKE CITY DO TESTE",
		Sipom: SipomFields{AreaID: &areas[0], Manual: []string{SipomFieldArea}},
	}
	TranslateSipom(cat, SipomRefs{Areas: rules}, fixed)
	if fixed.Sipom.AreaID == nil || *fixed.Sipom.AreaID != areas[0] {
		t.Errorf("área fixada na ficha perdeu para a referência: %v", fixed.Sipom.AreaID)
	}
}

// Com o geocodificador ligado, ocorrência sem coordenada fica pendente — e a
// pendência é da ocorrência, não do envio ao SIPOM.
func TestSmoke_GeoPending(t *testing.T) {
	db, _ := smokeDB(t)
	ctx := context.Background()
	cat, err := sipom.Load(ctx, db)
	if err != nil {
		t.Skipf("catálogo do SIPOM não carregado: %v", err)
	}
	translate := func(required bool, geo Geo) []string {
		o := &Occurrence{
			Natures: []string{"FURTO"}, StartTime: "10:00", CIOPS: "SMK3",
			PlaceAddress: "Rua Um, 10", PlaceNeighborhood: "CENTRO", PlaceCity: "CAUCAIA", Geo: geo,
		}
		TranslateSipom(cat, SipomRefs{GeoRequired: required}, o)
		return o.Sipom.Pending
	}
	lat, lng := -3.73, -38.65
	if p := translate(true, Geo{}); !hasString(p, sipom.PendCoordenada) {
		t.Errorf("geocodificador ligado e sem coordenada: faltou a pendência (%v)", p)
	}
	if p := translate(true, Geo{Lat: &lat, Lng: &lng, Precision: "bairro", Source: GeoAuto}); hasString(p, sipom.PendCoordenada) {
		t.Errorf("com coordenada (mesmo aproximada) não há pendência: %v", p)
	}
	if p := translate(false, Geo{}); hasString(p, sipom.PendCoordenada) {
		t.Errorf("geocodificador desligado não gera pendência de coordenada: %v", p)
	}
	if sipom.Blocking(sipom.PendCoordenada) || sipom.Notice(sipom.PendCoordenada) {
		t.Errorf("coordenada: não impede o envio ao SIPOM, mas é pendência (não aviso)")
	}
	if SipomBlocked([]string{sipom.PendCoordenada}) {
		t.Errorf("falta de coordenada não pode travar o envio ao SIPOM")
	}
}

// SetGeo grava e limpa a coordenada; Unlocated lista o que ficou sem.
func TestSmoke_SetGeo(t *testing.T) {
	db, _ := smokeDB(t)
	ctx := context.Background()
	r := New(db)
	var id string
	err := db.QueryRowContext(ctx, `
		SELECT id FROM app.ops_occurrences WHERE deleted_at IS NULL ORDER BY created_at, id LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		t.Skip("sem ocorrência do relatório operacional no banco")
	}
	if err != nil {
		t.Fatal(err)
	}
	orig, err := r.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	defer func() {
		if err := r.SetGeo(ctx, id, orig.Geo); err != nil {
			t.Errorf("restaurar coordenada: %v", err)
		}
	}()
	unlocated := func() bool {
		list, err := r.Unlocated(ctx)
		if err != nil {
			t.Fatalf("Unlocated: %v", err)
		}
		for _, so := range list {
			if so.ID == id {
				return so.PlaceCity == orig.PlaceCity
			}
		}
		return false
	}

	lat, lng := -3.7361, -38.6531
	if err := r.SetGeo(ctx, id, Geo{Lat: &lat, Lng: &lng, Precision: "rua", Source: GeoAuto}); err != nil {
		t.Fatalf("SetGeo: %v", err)
	}
	got, err := r.FindByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Geo.Located() || *got.Geo.Lat != lat || *got.Geo.Lng != lng || got.Geo.Precision != "rua" || got.Geo.Source != GeoAuto {
		t.Errorf("coordenada gravada: %+v", got.Geo)
	}
	if unlocated() {
		t.Errorf("ocorrência com coordenada apareceu em Unlocated")
	}

	// Sem latitude/longitude limpa tudo, inclusive precisão e origem.
	if err := r.SetGeo(ctx, id, Geo{Precision: "rua", Source: GeoAuto}); err != nil {
		t.Fatalf("SetGeo (limpar): %v", err)
	}
	if got, _ := r.FindByID(ctx, id); got.Geo.Located() || got.Geo.Precision != "" || got.Geo.Source != "" {
		t.Errorf("coordenada não foi limpa: %+v", got.Geo)
	}
	if !unlocated() {
		t.Errorf("ocorrência sem coordenada não apareceu em Unlocated")
	}
}
