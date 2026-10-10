// Smoke test da visão unificada de ocorrências contra um Postgres real:
// ficha CIOPS única no cadastro manual (gatilho), detecção de duplicidade e
// a listagem que une cadastro e relatório operacional pela ficha.
// Pula se APP_DATABASE_URL não estiver definido (ambiente local sem DB).
package occurrences

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	idb "github.com/belia/tevunah/backend/internal/db"
	"github.com/belia/tevunah/backend/internal/incidents"
)

func TestSmoke_CIOPSAndDuplicates(t *testing.T) {
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

	inc := incidents.New(db)
	occ := New(db)
	all := Scope{Manual: true, Ops: true}

	// Data bem fora do acervo real, para a busca por duplicidade só enxergar
	// o que o teste criou; a ficha leva o relógio para não colidir entre runs.
	day := time.Date(1999, 1, 2, 0, 0, 0, 0, time.UTC)
	ficha := fmt.Sprintf("SMK%d", time.Now().UnixNano()%1_000_000_000)
	hour := "21:40"

	// O papel da aplicação não tem DELETE: o descarte é o soft delete.
	discard := func(id string) {
		if _, err := db.ExecContext(ctx,
			`UPDATE app.incidents SET deleted_at = now(), deleted_by = $2
			  WHERE id = $1 AND deleted_at IS NULL`, id, actor); err != nil {
			t.Errorf("limpeza (incidents): %v", err)
		}
	}

	first, err := inc.Create(ctx, incidents.NewIncident{
		Type: incidents.TypeHomicidio, OccurredOn: day, OccurredTime: &hour,
		// Espaços e minúsculas de propósito: a ficha é gravada limpa.
		CIOPSRecord: " " + ficha[:3] + " " + ficha[3:] + " ",
		City:        "smoke city", Neighborhood: "smoke bairro",
		Description: "smoke ocorrências", CreatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer discard(first.ID)
	if first.CIOPSRecord != ficha {
		t.Errorf("ficha gravada %q, quer %q", first.CIOPSRecord, ficha)
	}

	// ── Camada 1: ficha igual ──
	hit, err := occ.IncidentByCIOPS(ctx, "  "+ficha[:4]+"-"+ficha[4:], "")
	if err != nil {
		t.Fatalf("IncidentByCIOPS: %v", err)
	}
	if hit == nil || hit.ID != first.ID {
		t.Fatalf("IncidentByCIOPS não achou o cadastro pela ficha com outra grafia: %+v", hit)
	}
	if hit, _ := occ.IncidentByCIOPS(ctx, ficha, first.ID); hit != nil {
		t.Errorf("IncidentByCIOPS devolveu a própria ocorrência em edição")
	}

	// O gatilho recusa a segunda gravação mesmo sem a checagem do handler.
	if _, err := inc.Create(ctx, incidents.NewIncident{
		Type: incidents.TypePrisao, OccurredOn: day, CIOPSRecord: ficha[:4] + "." + ficha[4:],
		CreatedBy: actor,
	}); !errors.Is(err, incidents.ErrDuplicateCIOPS) {
		t.Fatalf("ficha repetida: esperado ErrDuplicateCIOPS, obtido %v", err)
	}

	// Outra ocorrência, e a tentativa de trocar a ficha dela pela primeira.
	other, err := inc.Create(ctx, incidents.NewIncident{
		Type: incidents.TypePrisao, OccurredOn: day.AddDate(0, 0, 5), CreatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create (outra): %v", err)
	}
	defer discard(other.ID)
	if _, err := inc.Update(ctx, other.ID, actor, incidents.UpdateOpts{CIOPSRecord: &ficha}); !errors.Is(err, incidents.ErrDuplicateCIOPS) {
		t.Errorf("troca para ficha repetida: esperado ErrDuplicateCIOPS, obtido %v", err)
	}
	// Editar a própria ocorrência sem trocar a ficha continua passando.
	desc := "smoke ocorrências — editada"
	if _, err := inc.Update(ctx, first.ID, actor, incidents.UpdateOpts{CIOPSRecord: &ficha, Description: &desc}); err != nil {
		t.Errorf("update com a mesma ficha: %v", err)
	}

	// ── Camada 2: ficha diferente, mesma data, hora e lugar ──
	dups, err := occ.PossibleDuplicates(ctx, DuplicateQuery{
		Subject: Subject{
			CIOPS: ficha + "9", OccurredOn: day, Time: "21:55",
			City: "SMOKE CITY", Neighborhood: "SMOKE BAIRRO",
		},
		Scope: all,
	})
	if err != nil {
		t.Fatalf("PossibleDuplicates: %v", err)
	}
	if len(dups) != 1 || dups[0].ID != first.ID || dups[0].Source != SourceManual {
		t.Fatalf("PossibleDuplicates: esperado só o primeiro cadastro, obtido %+v", dups)
	}
	if len(dups[0].Reasons) != 3 {
		t.Errorf("motivos: %v, quer ficha parecida + mesma hora + mesmo bairro", dups[0].Reasons)
	}
	// A própria ocorrência em edição e quem não lê o cadastro ficam de fora.
	if d, _ := occ.PossibleDuplicates(ctx, DuplicateQuery{
		Subject: Subject{OccurredOn: day, Time: hour, City: "SMOKE CITY"}, ExcludeIncidentID: first.ID, Scope: all,
	}); len(d) != 0 {
		t.Errorf("ExcludeIncidentID não excluiu: %+v", d)
	}
	if d, _ := occ.PossibleDuplicates(ctx, DuplicateQuery{
		Subject: Subject{OccurredOn: day, Time: hour, City: "SMOKE CITY"}, Scope: Scope{Ops: true},
	}); len(d) != 0 {
		t.Errorf("escopo sem cadastro manual devolveu cadastro: %+v", d)
	}

	// ── Listagem ──
	// list devolve itens e total, como a tela os consome.
	list := func(sc Scope, o ListOpts) ([]Row, int, error) {
		res, err := occ.List(ctx, sc, o)
		if err != nil {
			return nil, 0, err
		}
		return res.Items, res.Total, nil
	}
	rows, total, err := list(all, ListOpts{Search: ficha})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].IncidentID == nil || *rows[0].IncidentID != first.ID {
		t.Fatalf("List por ficha: total=%d rows=%+v", total, rows)
	}
	if rows[0].OpsID != nil || rows[0].Time != hour || rows[0].City != "SMOKE CITY" {
		t.Errorf("linha do cadastro manual inesperada: %+v", rows[0])
	}
	if _, total, _ := list(all, ListOpts{Search: ficha, Source: SourceOps}); total != 0 {
		t.Errorf("filtro de origem (operacional) devolveu cadastro manual")
	}
	if _, total, _ := list(Scope{Ops: true}, ListOpts{Search: ficha}); total != 0 {
		t.Errorf("escopo sem cadastro manual listou cadastro")
	}

	// ── Abas: homicídios de um lado, produtividade do outro ──
	// `first` é homicídio; `other` é prisão, cinco dias depois.
	tabs, err := occ.List(ctx, all, ListOpts{DateFrom: "1999-01-01", DateTo: "1999-01-31", Category: CategoryHomicides})
	if err != nil {
		t.Fatalf("List (abas): %v", err)
	}
	if tabs.Total != 1 || *tabs.Items[0].IncidentID != first.ID {
		t.Errorf("aba homicídios: total=%d items=%+v", tabs.Total, tabs.Items)
	}
	if tabs.Counts.Homicides != 1 || tabs.Counts.Productivity != 1 {
		t.Errorf("contagem das abas: %+v, quer 1 homicídio e 1 de produtividade", tabs.Counts)
	}
	prod, err := occ.List(ctx, all, ListOpts{DateFrom: "1999-01-01", DateTo: "1999-01-31", Category: CategoryProductivity})
	if err != nil || prod.Total != 1 || *prod.Items[0].IncidentID != other.ID {
		t.Errorf("aba produtividade: err=%v res=%+v", err, prod)
	}

	// Ordenações e recortes da tela: todos precisam ser SQL válido.
	for _, o := range []ListOpts{
		{SortBy: "type", SortDir: "asc"},
		{SortBy: "updated_at"},
		{SortBy: "occurred_on", SortDir: "asc", Limit: 5, Offset: 1},
		{Type: incidents.TypeHomicidio, Means: "paf", City: "smoke city", Neighborhood: "smoke bairro",
			DateFrom: "1999-01-01", DateTo: "1999-01-31", Source: SourceManual},
	} {
		if _, _, err := list(all, o); err != nil {
			t.Errorf("List(%+v): %v", o, err)
		}
	}
	if rows, total, err := list(all, ListOpts{City: "smoke city", DateFrom: "1999-01-01", DateTo: "1999-01-03"}); err != nil || total != 1 || len(rows) != 1 {
		t.Errorf("recorte por município e período: total=%d err=%v", total, err)
	}
	cities, neighborhoods, err := occ.Locations(ctx, all)
	if err != nil {
		t.Fatalf("Locations: %v", err)
	}
	foundCity, foundNeigh := false, false
	for _, c := range cities {
		foundCity = foundCity || c.City == "SMOKE CITY"
	}
	for _, n := range neighborhoods {
		foundNeigh = foundNeigh || (n.City == "SMOKE CITY" && n.Neighborhood == "SMOKE BAIRRO")
	}
	if !foundCity || !foundNeigh {
		t.Errorf("Locations sem o município/bairro do cadastro: city=%v bairro=%v", foundCity, foundNeigh)
	}

	// ── Mapa: só o que tem coordenada, cada ocorrência numa camada ──
	lat, lng := -3.7319, -38.5267
	if _, err := inc.Update(ctx, other.ID, actor, incidents.UpdateOpts{
		Latitude: &lat, LatitudeSet: true, Longitude: &lng, LongitudeSet: true,
	}); err != nil {
		t.Fatalf("georreferenciar a prisão: %v", err)
	}
	smokeWindow := ListOpts{DateFrom: "1999-01-01", DateTo: "1999-01-31", Category: CategoryProductivity}
	points, geoTotal, truncated, err := occ.ListGeo(ctx, all, smokeWindow)
	if err != nil {
		t.Fatalf("ListGeo: %v", err)
	}
	if truncated || geoTotal != 1 || len(points) != 1 || *points[0].IncidentID != other.ID {
		t.Fatalf("camada de produtividade: total=%d truncated=%v pontos=%+v", geoTotal, truncated, points)
	}
	if points[0].Latitude == nil || *points[0].Latitude != lat || *points[0].Longitude != lng || points[0].GeoPrecision != "" {
		t.Errorf("coordenada do cadastro manual: %+v", points[0])
	}
	// O homicídio do mesmo período não tem coordenada nem é produtividade.
	smokeWindow.Category = CategoryHomicides
	if points, geoTotal, _, err := occ.ListGeo(ctx, all, smokeWindow); err != nil || len(points) != 0 || geoTotal != 1 {
		t.Errorf("camada de homicídios: err=%v total=%d pontos=%+v", err, geoTotal, points)
	}

	// ── União pela ficha com o relatório operacional ──
	// Usa uma ocorrência importada que já esteja no banco (o teste não cria
	// relatório: não haveria como descartá-lo depois).
	var opsID, opsFicha string
	err = db.QueryRowContext(ctx, `
		SELECT o.id, o.ciops_record FROM app.ops_occurrences o
		 WHERE o.deleted_at IS NULL AND app.norm_ciops(o.ciops_record) <> ''
		   AND NOT EXISTS (SELECT 1 FROM app.incidents i
		                    WHERE i.deleted_at IS NULL AND i.ciops_record <> ''
		                      AND app.norm_ciops(i.ciops_record) = app.norm_ciops(o.ciops_record))
		 LIMIT 1`).Scan(&opsID, &opsFicha)
	if errors.Is(err, sql.ErrNoRows) {
		t.Log("sem ocorrência do relatório operacional no banco — união pela ficha não exercitada")
		return
	}
	if err != nil {
		t.Fatalf("buscar ocorrência operacional: %v", err)
	}
	before, total, err := list(all, ListOpts{Search: opsFicha})
	if err != nil || total != 1 || before[0].IncidentID != nil || before[0].OpsID == nil {
		t.Fatalf("antes da união: err=%v total=%d rows=%+v", err, total, before)
	}
	linked, err := inc.Create(ctx, incidents.NewIncident{
		Type: incidents.TypeHomicidio, OccurredOn: day.AddDate(0, 0, 9),
		CIOPSRecord: opsFicha, Description: "smoke — cadastro da ficha importada", CreatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create (mesma ficha do relatório): %v", err)
	}
	defer discard(linked.ID)
	after, total, err := list(all, ListOpts{Search: opsFicha})
	if err != nil {
		t.Fatalf("List (união): %v", err)
	}
	if total != 1 || after[0].IncidentID == nil || *after[0].IncidentID != linked.ID ||
		after[0].OpsID == nil || *after[0].OpsID != opsID {
		t.Fatalf("mesma ficha deveria virar UMA linha com as duas fontes: total=%d rows=%+v", total, after)
	}
	if after[0].Type != incidents.TypeHomicidio || len(after[0].Natures) == 0 {
		t.Errorf("linha unida sem tipo do cadastro ou sem naturezas do relatório: %+v", after[0])
	}
	// As pendências do SIPOM acompanham a linha, com ou sem cadastro manual.
	var wantPending int
	if err := db.QueryRowContext(ctx,
		`SELECT cardinality(sipom_pendencias) FROM app.ops_occurrences WHERE id = $1`, opsID).Scan(&wantPending); err != nil {
		t.Fatalf("pendências gravadas: %v", err)
	}
	if len(before[0].Pending) != wantPending || len(after[0].Pending) != wantPending {
		t.Errorf("pendências na listagem: antes=%v depois=%v, quer %d", before[0].Pending, after[0].Pending, wantPending)
	}
	if rows[0].Pending == nil || len(rows[0].Pending) != 0 {
		t.Errorf("linha só do cadastro deveria vir sem pendências: %v", rows[0].Pending)
	}
	if ops, err := occ.OpsByCIOPS(ctx, opsFicha); err != nil || ops == nil || ops.ID != opsID {
		t.Errorf("OpsByCIOPS: %+v, %v", ops, err)
	}
}
