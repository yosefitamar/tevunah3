// Smoke test do módulo de ocorrências contra um Postgres real. Cobre o meio
// utilizado (CVLI) e a consulta geográfica que alimenta o mapa do crime.
// Pula se APP_DATABASE_URL não estiver definido (ambiente local sem DB).
package incidents

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	idb "github.com/belia/tevunah/backend/internal/db"
)

func TestSmoke_MeansAndGeo(t *testing.T) {
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

	// Entidade descartável pra exercitar o vínculo de suspeito.
	var entityID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO app.entities (kind, name, classification, created_by, updated_by)
		VALUES ('person', 'SMOKE INCIDENT SUSPECT', 1, $1, $1)
		RETURNING id`, actor).Scan(&entityID); err != nil {
		t.Fatalf("criar entidade: %v", err)
	}
	// Modelagem polimórfica: os atributos de pessoa (onde vive o óbito) ficam
	// na tabela filha, que o INSERT na base não cria sozinha.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO app.entity_persons (entity_id) VALUES ($1)`, entityID); err != nil {
		t.Fatalf("criar entity_persons: %v", err)
	}
	// Idem para a entidade: sem DELETE, o descarte é o soft delete do app.
	defer func() {
		if _, err := db.ExecContext(ctx,
			`UPDATE app.entities SET deleted_at = now(), deleted_by = $2
			  WHERE id = $1 AND deleted_at IS NULL`, entityID, actor); err != nil {
			t.Errorf("limpeza (entities): %v", err)
		}
	}()

	r := New(db)
	lat, lng := -3.7319, -38.5267
	occurred := time.Now().AddDate(0, 0, -1)

	created, err := r.Create(ctx, NewIncident{
		Type:       TypeHomicidio,
		OccurredOn: occurred,
		Latitude:   &lat,
		Longitude:  &lng,
		// Minúsculas de propósito: o repo normaliza pra MAIÚSCULAS, senão a
		// agregação por município se fragmentaria por grafia.
		City:         " fortaleza ",
		Neighborhood: "centro",
		Description:  "smoke test",
		Means:        MeansPAF,
		CreatedBy:    actor,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Limpeza. O papel da aplicação não tem DELETE em app.incidents (o app é
	// soft-delete por design), então o descarte segue o mesmo caminho do
	// produto: marca deleted_at, o que já tira o registro de List/ListGeo.
	defer func() {
		if _, err := db.ExecContext(ctx,
			`DELETE FROM app.incident_entities WHERE incident_id = $1`, created.ID); err != nil {
			t.Errorf("limpeza (incident_entities): %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`UPDATE app.incidents SET deleted_at = now(), deleted_by = $2
			  WHERE id = $1 AND deleted_at IS NULL`, created.ID, actor); err != nil {
			t.Errorf("limpeza (incidents): %v", err)
		}
	}()

	if created.Means != MeansPAF {
		t.Errorf("means esperado %q, obtido %q", MeansPAF, created.Means)
	}
	if created.City != "FORTALEZA" || created.Neighborhood != "CENTRO" {
		t.Errorf("local não normalizado: (%q, %q)", created.City, created.Neighborhood)
	}
	if created.Description != "SMOKE TEST" {
		t.Errorf("descrição não normalizada pra MAIÚSCULAS: %q", created.Description)
	}

	// Meio inválido é rejeitado antes de chegar ao CHECK do banco.
	if _, err := r.Create(ctx, NewIncident{
		Type: TypeHomicidio, OccurredOn: occurred, Means: "bazuca", CreatedBy: actor,
	}); err != ErrInvalidMeans {
		t.Errorf("esperado ErrInvalidMeans, obtido %v", err)
	}

	// Troca pra "outros" + detalhe, depois limpa: "" é valor legítimo e não
	// pode ser tratado como "campo não enviado".
	outros, detail := MeansOutros, "atropelamento"
	upd, err := r.Update(ctx, created.ID, actor, UpdateOpts{
		Means: &outros, MeansSet: true,
		MeansDetail: &detail, MeansDetailSet: true,
	})
	if err != nil {
		t.Fatalf("update means: %v", err)
	}
	if upd.Means != MeansOutros || upd.MeansDetail != detail {
		t.Errorf("update means: obtido (%q, %q)", upd.Means, upd.MeansDetail)
	}

	// Patch sem tocar em means não pode apagá-lo.
	desc := "smoke test — descrição nova"
	upd, err = r.Update(ctx, created.ID, actor, UpdateOpts{Description: &desc})
	if err != nil {
		t.Fatalf("update description: %v", err)
	}
	if upd.Means != MeansOutros || upd.MeansDetail != detail {
		t.Errorf("means perdido em patch alheio: (%q, %q)", upd.Means, upd.MeansDetail)
	}
	if upd.Description != "SMOKE TEST — DESCRIÇÃO NOVA" {
		t.Errorf("descrição do update não normalizada: %q", upd.Description)
	}

	// Busca insensível a acento: o termo sem acento acha a descrição com.
	semAcento, err := r.List(ctx, ListOpts{Search: "descricao nova"})
	if err != nil {
		t.Fatalf("list busca sem acento: %v", err)
	}
	achou := false
	for _, it := range semAcento.Items {
		if it.ID == created.ID {
			achou = true
		}
	}
	if !achou {
		t.Errorf("busca sem acento não achou a ocorrência de descrição acentuada")
	}

	empty := ""
	upd, err = r.Update(ctx, created.ID, actor, UpdateOpts{
		Means: &empty, MeansSet: true, MeansDetail: &empty, MeansDetailSet: true,
	})
	if err != nil {
		t.Fatalf("limpar means: %v", err)
	}
	if upd.Means != "" || upd.MeansDetail != "" {
		t.Errorf("means não foi limpo: (%q, %q)", upd.Means, upd.MeansDetail)
	}

	// Volta pra PAF e exercita o filtro da listagem.
	paf := MeansPAF
	if _, err := r.Update(ctx, created.ID, actor, UpdateOpts{Means: &paf, MeansSet: true}); err != nil {
		t.Fatalf("restaurar means: %v", err)
	}
	list, err := r.List(ctx, ListOpts{Means: MeansPAF, Type: TypeHomicidio})
	if err != nil {
		t.Fatalf("list por means: %v", err)
	}
	if !containsID(list.Items, created.ID) {
		t.Errorf("ocorrência não veio no filtro means=paf")
	}
	list, err = r.List(ctx, ListOpts{Means: MeansAsfixia})
	if err != nil {
		t.Fatalf("list por means (asfixia): %v", err)
	}
	if containsID(list.Items, created.ID) {
		t.Errorf("filtro means=asfixia não deveria trazer a ocorrência")
	}

	// Recorte territorial: filtro exato por município e por bairro, inclusive
	// quando o chamador manda em minúsculas.
	list, err = r.List(ctx, ListOpts{City: "fortaleza", Neighborhood: "Centro"})
	if err != nil {
		t.Fatalf("list por local: %v", err)
	}
	if !containsID(list.Items, created.ID) {
		t.Errorf("ocorrência não veio no filtro city+neighborhood")
	}
	list, err = r.List(ctx, ListOpts{City: "FORTALEZA", Neighborhood: "MONDUBIM"})
	if err != nil {
		t.Fatalf("list por bairro alheio: %v", err)
	}
	if containsID(list.Items, created.ID) {
		t.Errorf("filtro de bairro alheio não deveria trazer a ocorrência")
	}

	// Facetas: o município e o bairro precisam aparecer na lista que popula
	// os filtros do mapa.
	cities, neighborhoods, err := r.Locations(ctx)
	if err != nil {
		t.Fatalf("locations: %v", err)
	}
	if !hasFacet(cities, "FORTALEZA", "") {
		t.Errorf("FORTALEZA ausente nas facetas de município")
	}
	if !hasFacet(neighborhoods, "FORTALEZA", "CENTRO") {
		t.Errorf("CENTRO ausente nas facetas de bairro")
	}

	// Papel fora da lista fechada é recusado.
	if err := r.AddEntity(ctx, created.ID, entityID, "ENVOLVIDO", actor); err != ErrInvalidRole {
		t.Errorf("esperado ErrInvalidRole, obtido %v", err)
	}

	// Vínculo de acusado + consulta do mapa (envolvidos carregados em lote).
	// Papel em minúsculas e sem acento chega canônico no banco.
	if err := r.AddEntity(ctx, created.ID, entityID, "acusado", actor); err != nil {
		t.Fatalf("add entity: %v", err)
	}
	// Busca por envolvido: achar a ocorrência pelo nome de quem está
	// vinculado, não só pelo texto do relato.
	byName, _, err := r.ListGeo(ctx, GeoOpts{Search: "smoke incident suspect"})
	if err != nil {
		t.Fatalf("list geo por nome do envolvido: %v", err)
	}
	if !containsID(byName, created.ID) {
		t.Errorf("busca por nome do envolvido não achou a ocorrência")
	}
	byOther, _, err := r.ListGeo(ctx, GeoOpts{Search: "nome-que-nao-existe-xyz"})
	if err != nil {
		t.Fatalf("list geo por termo inexistente: %v", err)
	}
	if containsID(byOther, created.ID) {
		t.Errorf("termo inexistente não deveria casar")
	}

	geo, truncated, err := r.ListGeo(ctx, GeoOpts{
		Type:         TypeHomicidio,
		City:         "FORTALEZA",
		Neighborhood: "CENTRO",
		DateFrom:     occurred.Format("2006-01-02"),
		DateTo:       occurred.Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("list geo: %v", err)
	}
	if truncated {
		t.Errorf("resultado truncado inesperadamente")
	}
	var found *Incident
	for i := range geo {
		if geo[i].ID == created.ID {
			found = &geo[i]
		}
	}
	if found == nil {
		t.Fatalf("ocorrência não apareceu no mapa")
	}
	if found.Latitude == nil || *found.Latitude != lat {
		t.Errorf("latitude não voltou no ponto do mapa")
	}
	if len(found.Involved) != 1 || found.Involved[0].Role != RoleAcusado {
		t.Errorf("envolvidos não vieram em lote: %+v", found.Involved)
	}

	// Óbito: o vínculo de vítima é marcado pelo handler HTTP, mas o efeito
	// no banco (marcar / desvincular sem ressuscitar) é o que importa aqui.
	if _, err := db.ExecContext(ctx, `
		UPDATE app.entity_persons
		   SET deceased = true, deceased_on = $2, death_incident_id = $3
		 WHERE entity_id = $1`, entityID, occurred, created.ID); err != nil {
		t.Fatalf("marcar óbito: %v", err)
	}
	var deceased bool
	var deathIncident sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT deceased, death_incident_id FROM app.entity_persons WHERE entity_id = $1`,
		entityID).Scan(&deceased, &deathIncident); err != nil {
		t.Fatalf("ler óbito: %v", err)
	}
	if !deceased || deathIncident.String != created.ID {
		t.Errorf("óbito não vinculou a ocorrência: (%v, %q)", deceased, deathIncident.String)
	}

	// Desvincular a vítima tira a identificação da ocorrência, mas o óbito
	// permanece — a morte não depende do vínculo.
	if err := r.RemoveEntity(ctx, created.ID, entityID); err != nil {
		t.Fatalf("remove entity: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT deceased, death_incident_id FROM app.entity_persons WHERE entity_id = $1`,
		entityID).Scan(&deceased, &deathIncident); err != nil {
		t.Fatalf("ler óbito (2): %v", err)
	}
	if !deceased {
		t.Errorf("óbito foi desfeito ao desvincular a vítima")
	}
	if deathIncident.Valid {
		t.Errorf("referência à ocorrência deveria ter sido limpa: %q", deathIncident.String)
	}

	// Sem coordenadas o registro não é plotável e some do mapa.
	if _, err := r.Update(ctx, created.ID, actor, UpdateOpts{
		LatitudeSet: true, LongitudeSet: true,
	}); err != nil {
		t.Fatalf("limpar coordenadas: %v", err)
	}
	geo, _, err = r.ListGeo(ctx, GeoOpts{Type: TypeHomicidio})
	if err != nil {
		t.Fatalf("list geo (2): %v", err)
	}
	for i := range geo {
		if geo[i].ID == created.ID {
			t.Errorf("ocorrência sem coordenadas continuou no mapa")
		}
	}
}

func hasFacet(fs []PlaceFacet, city, neighborhood string) bool {
	for _, f := range fs {
		if f.City == city && f.Neighborhood == neighborhood && f.Count > 0 {
			return true
		}
	}
	return false
}

func containsID(items []Incident, id string) bool {
	for i := range items {
		if items[i].ID == id {
			return true
		}
	}
	return false
}
