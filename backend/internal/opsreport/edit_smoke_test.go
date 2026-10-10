// Smoke tests contra um Postgres real: a correção da ocorrência importada e
// a referência de área aprendida (cidade + bairro → área da unidade militar).
// Pulam se APP_DATABASE_URL não estiver definido.
package opsreport

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
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

// Procedimento e materiais traduzidos a partir do que o relatório escreve.
// Contra o catálogo real do banco.
func TestSmoke_ProcedureAndMaterials(t *testing.T) {
	db, _ := smokeDB(t)
	ctx := context.Background()
	cat, err := sipom.Load(ctx, db)
	if err != nil {
		t.Skipf("catálogo do SIPOM não carregado: %v", err)
	}
	g := 582.0
	o := &Occurrence{
		Natures: []string{"TRÁFICO DE DROGAS"}, StartTime: "10:00", CIOPS: "SMK4",
		PlaceNeighborhood: "CENTRO", PlaceCity: "CAUCAIA",
		PoliceStation: "DMC", Delegate: "Ítalo Renno Alves", ProcedureType: "IP", ProcedureNumber: "939-7653/2026",
		Weapons: []Weapon{
			{Kind: "REVOLVER", Brand: "TAURUS", Caliber: "38", Serial: "GI63237"},
			{Kind: "SUBMETRALHADORA", Model: "ARTESANAL", Caliber: "9MM"},
		},
		Drugs:    []Drug{{Description: "MACONHA", Grams: &g}},
		Vehicles: []Vehicle{{Kind: "MOTOCICLETA", Brand: "HONDA", Model: "CG 160 FAN", Color: "PRETO", Plate: "ABC1D23"}},
	}
	TranslateSipom(cat, SipomRefs{}, o)
	f := o.Sipom
	if f.ProcedimentoID == nil || *f.ProcedimentoID != sipom.ProcIP {
		t.Errorf("tipo IP: %v", f.ProcedimentoID)
	}
	if f.DelegaciaID == nil {
		t.Errorf("delegacia pelo prefixo 939 não resolvida")
	} else if d, _ := cat.Delegacia(*f.DelegaciaID); d.Code != 939 {
		t.Errorf("delegacia %d (%s), quer o código 939", d.ID, d.Nome)
	}
	if f.DelegadoID == nil {
		t.Errorf("delegado abreviado não resolvido")
	} else if d, _ := cat.Delegado(*f.DelegadoID); d.Nome != "ITALO RENNO ALVES FEITOSA" {
		t.Errorf("delegado: %s", d.Nome)
	}
	w := o.Weapons[0]
	if w.SipomTipoID == nil || cat.ArmaTipoNome(*w.SipomTipoID) != "Revolver" ||
		w.SipomMarcaID == nil || cat.ArmaMarcaNome(*w.SipomMarcaID) != "Taurus" ||
		w.SipomCalibreID == nil || cat.ArmaCalibreNome(*w.SipomCalibreID) != ".38" {
		t.Errorf("revólver Taurus .38: %+v", w)
	}
	w = o.Weapons[1]
	if w.SipomTipoID == nil || cat.ArmaTipoNome(*w.SipomTipoID) != "Artesanal" || w.SipomMarcaID != nil ||
		w.SipomCalibreID == nil || cat.ArmaCalibreNome(*w.SipomCalibreID) != "9mm" {
		t.Errorf("submetralhadora artesanal 9mm: %+v", w)
	}
	if !hasString(f.Pending, sipom.PendArma) {
		t.Errorf("arma sem marca deveria deixar a pendência de arma: %v", f.Pending)
	}
	d := o.Drugs[0]
	if d.SipomDrogaID == nil || d.SipomQuantidade == nil || *d.SipomQuantidade != 582 {
		t.Errorf("maconha 582 g: %+v", d)
	} else if dr, _ := cat.Droga(*d.SipomDrogaID); dr.Nome != "Maconha" {
		t.Errorf("droga: %s", dr.Nome)
	}
	v := o.Vehicles[0]
	if v.SipomTipoCodigo == nil || *v.SipomTipoCodigo != 4 || v.SipomCorCodigo == nil || *v.SipomCorCodigo != 11 ||
		v.SipomSituacao != sipom.VeiculoApreendido {
		t.Errorf("moto preta apreendida: %+v", v)
	}
	// "HONDA/CG 160 FAN" existe tal qual na tabela DENATRAN.
	if v.SipomMarcaModeloCodigo == nil {
		t.Errorf("marca/modelo exato não resolvido: %+v", v)
	} else if mm, _ := cat.MarcaModelo(*v.SipomMarcaModeloCodigo); mm.Descricao != "HONDA/CG 160 FAN" {
		t.Errorf("marca/modelo: %s", mm.Descricao)
	}
	if hasString(f.Pending, sipom.PendDroga) || hasString(f.Pending, sipom.PendVeiculo) ||
		hasString(f.Pending, sipom.PendDelegacia) || hasString(f.Pending, sipom.PendDelegado) {
		t.Errorf("pendências inesperadas: %v", f.Pending)
	}

	// Sem procedimento no relatório; delegado desconhecido; cor com termo
	// aprendido; veículo recuperado.
	terms := &sipom.TermMap{}
	o2 := &Occurrence{
		Natures: []string{"RECUPERAÇÃO DE VEÍCULO"}, StartTime: "10:00", CIOPS: "SMK5",
		PlaceCity: "CAUCAIA", PoliceStation: "22° DP", Delegate: "Fulano Inexistente", ProcedureType: "BO",
		ProcedureNumber: "5291/2026",
		Vehicles:        []Vehicle{{Kind: "CARRO", Brand: "HONDA", Model: "CG", Color: "VERMELHO"}},
	}
	TranslateSipom(cat, SipomRefs{Terms: terms}, o2)
	f2 := o2.Sipom
	if f2.DelegaciaID == nil {
		t.Errorf("22° DP deveria virar o 22º Distrito")
	} else if d, _ := cat.Delegacia(*f2.DelegaciaID); d.Code != 122 {
		t.Errorf("delegacia do 22° DP: %d (%s)", d.Code, d.Nome)
	}
	if f2.DelegadoID != nil || !hasString(f2.Pending, sipom.PendDelegado) {
		t.Errorf("delegado inexistente: id=%v pend=%v", f2.DelegadoID, f2.Pending)
	}
	if SipomBlocked([]string{sipom.PendDelegado}) {
		t.Errorf("delegado não é obrigatório: não pode travar o envio")
	}
	v2 := o2.Vehicles[0]
	if v2.SipomSituacao != sipom.VeiculoRecuperado || v2.SipomTipoCodigo == nil || *v2.SipomTipoCodigo != 6 ||
		v2.SipomCorCodigo == nil || *v2.SipomCorCodigo != 15 {
		t.Errorf("carro vermelho recuperado: %+v", v2)
	}
	// "HONDA/CG" não existe tal qual, e "CG" casa dezenas de linhas: fica
	// pendente, com candidatos para o analista.
	if v2.SipomMarcaModeloCodigo != nil || !hasString(f2.Pending, sipom.PendVeiculo) {
		t.Errorf("HONDA CG ambíguo deveria ficar pendente: %+v %v", v2, f2.Pending)
	}
	if r := cat.ResolveVehicle(sipom.VehicleInput{Kind: "CARRO", Brand: "HONDA", Model: "CG"}, nil); len(r.MarcaModeloCandidates) < 2 {
		t.Errorf("candidatos de HONDA CG: %d", len(r.MarcaModeloCandidates))
	}

	o3 := &Occurrence{Natures: []string{"FURTO"}, StartTime: "10:00", CIOPS: "SMK6", PlaceCity: "CAUCAIA"}
	TranslateSipom(cat, SipomRefs{}, o3)
	if !hasString(o3.Sipom.Pending, sipom.PendProcedimento) || !sipom.Blocking(sipom.PendProcedimento) {
		t.Errorf("sem procedimento deveria ser pendência bloqueante: %v", o3.Sipom.Pending)
	}
}

// O payload completo (fase 2) leva o procedimento e um item por material.
func TestSmoke_PayloadPhase2(t *testing.T) {
	db, _ := smokeDB(t)
	ctx := context.Background()
	cat, err := sipom.Load(ctx, db)
	if err != nil {
		t.Skipf("catálogo do SIPOM não carregado: %v", err)
	}
	g := 582.0
	so := &StoredOccurrence{ID: "smoke-payload", Occurrence: Occurrence{
		Natures: []string{"TRÁFICO DE DROGAS"}, StartTime: "10:00", CIOPS: "SMK7",
		PlaceAddress: "Rua Um, 10", PlaceNeighborhood: "CENTRO", PlaceCity: "CAUCAIA", CIA: "1ª CIA", BPM: "2º BPRAIO",
		PoliceStation: "DMC", Delegate: "Ítalo Renno Alves", ProcedureType: "IP", ProcedureNumber: "939-7653/2026",
		Weapons:  []Weapon{{Kind: "REVOLVER", Brand: "TAURUS", Caliber: "38", Serial: "GI63237", Model: "RT 85"}},
		Drugs:    []Drug{{Description: "MACONHA", Grams: &g}},
		Vehicles: []Vehicle{{Kind: "MOTOCICLETA", Brand: "HONDA", Model: "CG 160 FAN", Color: "PRETO", Plate: "ABC1D23"}},
	}}
	TranslateSipom(cat, SipomRefs{}, &so.Occurrence)
	// Natureza e o que o catálogo não resolver para este endereço fictício:
	// o teste é do procedimento e dos materiais, não do cabeçalho.
	nat := cat.Naturezas()[0].ID
	so.Sipom.NaturezaID = &nat
	if so.Sipom.AreaID == nil || so.Sipom.OPMID == nil || so.Sipom.CidadeID == nil {
		var comp int
		for _, c := range cat.ActiveCompanies() {
			comp = c.ID
			break
		}
		if so.Sipom.AreaID == nil {
			so.Sipom.AreaID = &comp
		}
		if so.Sipom.OPMID == nil {
			so.Sipom.OPMID = &comp
		}
	}
	if so.Sipom.CidadeID == nil {
		t.Skip("CAUCAIA fora do catálogo de cidades")
	}
	so.Sipom.Pending = nil
	p, err := BuildSipomPayload(cat, so, "SAI/2º BPRAIO", nil, nil)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	if p.Origem.Versao != "3" {
		t.Errorf("versão do contrato: %s", p.Origem.Versao)
	}
	if p.Procedimento == nil || p.Procedimento.ProcedimentoID != sipom.ProcIP || p.Procedimento.Numero != "7653" ||
		p.Procedimento.Ano != "2026" || p.Procedimento.Reparticao != sipom.ReparticaoPoliciaCivil ||
		p.Procedimento.DelegadoID == nil || !strings.HasPrefix(p.Procedimento.Delegacia, "939-") {
		t.Errorf("procedimento: %+v", p.Procedimento)
	}
	if len(p.Materiais) != 3 {
		t.Fatalf("materiais: %d itens, quer 3 (%+v)", len(p.Materiais), p.Materiais)
	}
	arma, droga, veic := p.Materiais[0], p.Materiais[1], p.Materiais[2]
	if arma.TipoID != sipom.MaterialArma || arma.ArmaTipo != "Revolver" || arma.ArmaMarca != "Taurus" ||
		arma.ArmaCalibre != ".38" || arma.Numero != "GI63237" || arma.Descricao != "RT 85" || arma.Quantidade == nil || *arma.Quantidade != 1 {
		t.Errorf("arma: %+v", arma)
	}
	if droga.TipoID != sipom.MaterialDroga || droga.Droga != "Maconha" || droga.Unidade != "gramas (g)" ||
		droga.Quantidade == nil || *droga.Quantidade != 582 {
		t.Errorf("droga: %+v", droga)
	}
	if veic.TipoID != sipom.MaterialVeiculo || veic.Placa != "ABC1D23" || veic.MarcaModelo != "HONDA/CG 160 FAN" ||
		veic.Cor != "PRETA" || veic.Situacao == nil || *veic.Situacao != sipom.VeiculoApreendido {
		t.Errorf("veículo: %+v", veic)
	}
}
