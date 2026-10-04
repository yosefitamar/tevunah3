package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/incidents"
	"github.com/belia/tevunah/backend/internal/occurrences"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// Ocorrências: a listagem reúne o cadastro manual (/api/incidents) e o que
// veio do relatório operacional (/api/ops-occurrences). Cada linha diz de
// onde vem e o detalhe continua nos endpoints de cada fonte.

// occurrenceRowJSON é uma linha da listagem unificada. incident_id e ops_id
// dizem qual ficha abrir; os dois presentes = cadastro manual e relatório
// com a mesma ficha CIOPS.
type occurrenceRowJSON struct {
	IncidentID   *string   `json:"incident_id"`
	OpsID        *string   `json:"ops_id"`
	Type         string    `json:"type"`
	Natures      []string  `json:"natures"`
	OccurredOn   string    `json:"occurred_on"`
	Time         string    `json:"time"`
	CIOPSRecord  string    `json:"ciops_record"`
	City         string    `json:"city"`
	Neighborhood string    `json:"neighborhood"`
	Description  string    `json:"description"`
	Means        string    `json:"means"`
	Intel        bool      `json:"intel_participation"`
	HasGeo       bool      `json:"has_geo"`
	PeopleCount  int       `json:"people_count"`
	UpdatedAt    time.Time `json:"updated_at"`
	// Pendencias: o que falta na ocorrência do relatório. Aqui blocking
	// separa pendência de aviso (na ficha do SIPOM, separa o que impede o
	// envio). Vazio se a linha é só do cadastro.
	Pendencias []sipomPendingJSON `json:"pendencias"`
	// Coordenada (só no mapa interessa): geo_precision "bairro" = aproximada.
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	GeoPrecision string   `json:"geo_precision,omitempty"`
}

func toOccurrenceRowJSON(x *occurrences.Row) occurrenceRowJSON {
	pend := make([]sipomPendingJSON, 0, len(x.Pending))
	for _, p := range x.Pending {
		pend = append(pend, sipomPendingJSON{Code: p, Label: sipom.PendingLabel[p], Blocking: !sipom.Notice(p)})
	}
	return occurrenceRowJSON{
		IncidentID: x.IncidentID, OpsID: x.OpsID, Type: x.Type, Natures: nonNil(x.Natures),
		OccurredOn: x.OccurredOn.Format("2006-01-02"), Time: x.Time,
		CIOPSRecord: x.CIOPS, City: x.City, Neighborhood: x.Neighborhood,
		Description: x.Description, Means: x.Means, Intel: x.Intel, HasGeo: x.HasGeo,
		PeopleCount: x.PeopleCount, UpdatedAt: x.UpdatedAt, Pendencias: pend,
		Latitude: x.Latitude, Longitude: x.Longitude, GeoPrecision: x.GeoPrecision,
	}
}

func occurrenceListOpts(r *http.Request) occurrences.ListOpts {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	return occurrences.ListOpts{
		Limit:        limit,
		Offset:       offset,
		Category:     strings.TrimSpace(q.Get("category")),
		Source:       strings.TrimSpace(q.Get("source")),
		Type:         strings.TrimSpace(q.Get("type")),
		Means:        strings.TrimSpace(q.Get("means")),
		City:         strings.TrimSpace(q.Get("city")),
		Neighborhood: strings.TrimSpace(q.Get("neighborhood")),
		Search:       strings.TrimSpace(q.Get("search")),
		DateFrom:     strings.TrimSpace(q.Get("date_from")),
		DateTo:       strings.TrimSpace(q.Get("date_to")),
		SortBy:       strings.TrimSpace(q.Get("sort_by")),
		SortDir:      strings.TrimSpace(q.Get("sort_dir")),
	}
}

// duplicateCandidateJSON é uma ocorrência já gravada apontada como a mesma
// (ficha igual) ou como possível repetição (reasons).
type duplicateCandidateJSON struct {
	Source       string   `json:"source"` // "manual" | "operacional"
	ID           string   `json:"id"`
	Type         string   `json:"type,omitempty"`
	Natures      []string `json:"natures"`
	OccurredOn   string   `json:"occurred_on"`
	Time         string   `json:"time"`
	CIOPSRecord  string   `json:"ciops_record"`
	City         string   `json:"city"`
	Neighborhood string   `json:"neighborhood"`
	Reasons      []string `json:"reasons"`
}

func toDuplicateCandidatesJSON(in []occurrences.Candidate) []duplicateCandidateJSON {
	out := make([]duplicateCandidateJSON, 0, len(in))
	for _, c := range in {
		out = append(out, duplicateCandidateJSON{
			Source: c.Source, ID: c.ID, Type: c.Type, Natures: nonNil(c.Natures),
			OccurredOn: c.OccurredOn.Format("2006-01-02"), Time: c.Time,
			CIOPSRecord: c.CIOPS, City: c.City, Neighborhood: c.Neighborhood,
			Reasons: nonNil(c.Reasons),
		})
	}
	return out
}

// Códigos de conflito de duplicidade (campo errors.code da resposta 409).
const (
	// A ficha CIOPS já identifica outra ocorrência do cadastro: não grava.
	conflictCIOPSDuplicate = "ciops_duplicate"
	// Ficha diferente, mas data, hora e lugar (ou ficha quase igual) apontam
	// para uma ocorrência já gravada: só grava com confirm_duplicates.
	conflictPossibleDuplicates = "possible_duplicates"
)

// duplicateConflict responde 409 com as ocorrências em conflito, para a tela
// mostrar o que já existe em vez de só recusar.
func duplicateConflict(w http.ResponseWriter, code, message string, candidates []occurrences.Candidate) {
	httpx.WriteJSON(w, http.StatusConflict, httpx.Envelope{
		Success: false,
		Message: message,
		Errors: map[string]any{
			"code":       code,
			"candidates": toDuplicateCandidatesJSON(candidates),
		},
	})
}

// occScope: quais fontes o solicitante lê.
func (a *app) occScope(r *http.Request) occurrences.Scope {
	return occurrences.Scope{
		Manual: a.hasPerm(r, "incident.read"),
		Ops:    a.hasPerm(r, "opsreport.read"),
	}
}

// requireOccScope devolve o escopo ou responde 403 quando o solicitante não
// lê nenhuma das fontes.
func (a *app) requireOccScope(w http.ResponseWriter, r *http.Request) (occurrences.Scope, bool) {
	sc := a.occScope(r)
	if !sc.Manual && !sc.Ops {
		httpx.Error(w, http.StatusForbidden, "ação não autorizada: incident.read")
		return sc, false
	}
	return sc, true
}

// incidentJSON é toPublicIncident com o vínculo ao relatório operacional: a
// ocorrência importada de mesma ficha CIOPS, se o solicitante lê o módulo.
func (a *app) incidentJSON(r *http.Request, inc *incidents.Incident) publicIncident {
	out := toPublicIncident(inc)
	if inc.CIOPSRecord == "" || !a.hasPerm(r, "opsreport.read") {
		return out
	}
	ops, err := a.occurrences.OpsByCIOPS(r.Context(), inc.CIOPSRecord)
	if err != nil {
		log.Printf("incident ops link: %v", err)
		return out
	}
	if ops != nil {
		out.OpsOccurrenceID = &ops.ID
	}
	return out
}

// ─── GET /api/occurrences ──────────────────────────────────────────────

func (a *app) handleOccurrencesList(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireOccScope(w, r)
	if !ok {
		return
	}
	opts := occurrenceListOpts(r)
	limit, offset := opts.Limit, opts.Offset
	res, err := a.occurrences.List(r.Context(), sc, opts)
	if err != nil {
		log.Printf("occurrences list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	items := make([]occurrenceRowJSON, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, toOccurrenceRowJSON(&res.Items[i]))
	}
	httpx.OK(w, map[string]any{
		"items":  items,
		"total":  res.Total,
		"limit":  cmpDefault(limit, 25),
		"offset": offset,
		// Total de cada aba no recorte comum (busca, período e território).
		"counts": map[string]int{
			occurrences.CategoryProductivity: res.Counts.Productivity,
			occurrences.CategoryHomicides:    res.Counts.Homicides,
		},
	})
}

// ─── GET /api/occurrences/geo ──────────────────────────────────────────
//
// Pontos georreferenciados do recorte, para o mapa — a camada de
// produtividade (category=produtividade) lê daqui. Sem paginação: o período
// é o limitador, com teto de segurança sinalizado por "truncated". "total" é
// o recorte inteiro, com ou sem coordenada.

func (a *app) handleOccurrencesGeo(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireOccScope(w, r)
	if !ok {
		return
	}
	rows, total, truncated, err := a.occurrences.ListGeo(r.Context(), sc, occurrenceListOpts(r))
	if err != nil {
		log.Printf("occurrences geo: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao carregar o mapa")
		return
	}
	items := make([]occurrenceRowJSON, 0, len(rows))
	for i := range rows {
		items = append(items, toOccurrenceRowJSON(&rows[i]))
	}
	httpx.OK(w, map[string]any{"items": items, "total": total, "truncated": truncated})
}

// ─── GET /api/occurrences/locations ────────────────────────────────────
//
// Municípios e bairros da listagem unificada — o recorte territorial da tela
// de Ocorrências. (/api/incidents/locations segue servindo o mapa e o
// cadastro, que só enxergam o cadastro manual.)

func (a *app) handleOccurrencesLocations(w http.ResponseWriter, r *http.Request) {
	sc, ok := a.requireOccScope(w, r)
	if !ok {
		return
	}
	cities, neighborhoods, err := a.occurrences.Locations(r.Context(), sc)
	if err != nil {
		log.Printf("occurrences locations: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao carregar localidades")
		return
	}
	toPublic := func(in []occurrences.PlaceFacet) []publicPlaceFacet {
		out := make([]publicPlaceFacet, 0, len(in))
		for _, f := range in {
			out = append(out, publicPlaceFacet{
				City: f.City, Neighborhood: f.Neighborhood, Count: f.Count,
			})
		}
		return out
	}
	httpx.OK(w, map[string]any{
		"cities":        toPublic(cities),
		"neighborhoods": toPublic(neighborhoods),
	})
}
