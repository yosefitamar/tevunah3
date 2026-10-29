package main

import (
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// ─── Referências de área da unidade militar ───
//
// O analista vê e mantém as referências (cidade [+ bairro] → companhia) que
// a tradução usa antes do catálogo. Onde o catálogo do SIPOM distingue a
// área por bairro (Fortaleza, Caucaia), a referência é do bairro; onde põe
// as companhias na cidade inteira, é da cidade.

type areaRefJSON struct {
	City          string        `json:"city"`
	Neighborhood  string        `json:"neighborhood"` // "" = cidade inteira (ou sem bairro)
	CityWide      bool          `json:"city_wide"`
	Area          *sipomRefJSON `json:"area"`
	UpdatedAt     time.Time     `json:"updated_at"`
	UpdatedByName string        `json:"updated_by_name"`
}

// Lugar da fila que ainda pede área: uma escolha resolve todas as fichas.
type areaPendingJSON struct {
	City         string         `json:"city"`
	Neighborhood string         `json:"neighborhood"`
	CityWide     bool           `json:"city_wide"`
	CidadeID     *int           `json:"cidade_id"`
	Fichas       []string       `json:"fichas"`
	Areas        []sipomRefJSON `json:"areas"`
}

func (a *app) areaRefs(ids []int) []sipomRefJSON {
	out := make([]sipomRefJSON, 0, len(ids))
	for _, id := range ids {
		if co, ok := a.sipom.Companhia(id); ok && a.sipom.IsActiveCompany(id) {
			out = append(out, sipomRefJSON{ID: id, Nome: co.Abreviado})
		}
	}
	return out
}

// GET /api/sipom/area-refs
func (a *app) handleSipomAreaRefs(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	ctx := r.Context()
	rules, err := a.sipomAreas.List(ctx)
	if err != nil {
		log.Printf("sipom area refs: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar as referências")
		return
	}
	items := make([]areaRefJSON, 0, len(rules))
	for _, x := range rules {
		j := areaRefJSON{City: x.City, Neighborhood: x.Neighborhood, UpdatedAt: x.UpdatedAt, UpdatedByName: x.UpdatedByName}
		if cid, ok := a.sipom.CidadeID(x.City); ok {
			j.CityWide = x.Neighborhood == "" && a.sipom.CityWideAreas(&cid, nil)
		}
		if co, ok := a.sipom.Companhia(x.AreaID); ok {
			j.Area = &sipomRefJSON{ID: x.AreaID, Nome: co.Abreviado}
		}
		items = append(items, j)
	}

	occs, err := a.opsReports.AreaPendingList(ctx)
	if err != nil {
		log.Printf("sipom area pending: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar as pendências de área")
		return
	}
	groups := map[string]*areaPendingJSON{}
	var order []string
	for _, o := range occs {
		cityWide := a.sipom.CityWideAreas(o.CidadeID, o.BairroID)
		nb := strings.ToUpper(strings.TrimSpace(o.Neighborhood))
		if cityWide {
			nb = ""
		}
		k := strings.ToUpper(strings.TrimSpace(o.City)) + "\x00" + nb
		g, ok := groups[k]
		if !ok {
			g = &areaPendingJSON{City: strings.ToUpper(strings.TrimSpace(o.City)), Neighborhood: nb,
				CityWide: cityWide, CidadeID: o.CidadeID, Fichas: []string{}}
			opts := a.sipom.Areas(o.CidadeID, o.BairroID)
			if len(opts) == 0 && o.CidadeID != nil {
				opts = a.sipom.CityAreas(*o.CidadeID)
			}
			g.Areas = a.areaRefs(opts)
			groups[k] = g
			order = append(order, k)
		}
		g.Fichas = append(g.Fichas, o.CIOPS)
	}
	pending := make([]areaPendingJSON, 0, len(order))
	for _, k := range order {
		pending = append(pending, *groups[k])
	}
	sort.SliceStable(pending, func(i, j int) bool { return len(pending[i].Fichas) > len(pending[j].Fichas) })
	httpx.OK(w, map[string]any{"items": items, "pending": pending})
}

// GET /api/sipom/area-refs/cidades — cidades do catálogo, para a referência
// nova. GET /api/sipom/area-refs/cidades/{id}: se a área é da cidade
// inteira, os bairros e as áreas possíveis.
func (a *app) handleSipomAreaRefCities(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	items := []sipomRefJSON{}
	for _, c := range a.sipom.Cidades() {
		items = append(items, sipomRefJSON{ID: c.ID, Nome: c.Nome})
	}
	httpx.OK(w, map[string]any{"items": items})
}

func (a *app) handleSipomAreaRefCity(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	cid, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || a.sipom.CidadeNome(cid) == "" {
		httpx.Error(w, http.StatusNotFound, "cidade fora do catálogo do SIPOM")
		return
	}
	bairros := []sipomRefJSON{}
	for _, b := range a.sipom.Bairros(cid) {
		bairros = append(bairros, sipomRefJSON{ID: b.ID, Nome: b.Nome})
	}
	httpx.OK(w, map[string]any{
		"city_wide": a.sipom.CityWideAreas(&cid, nil),
		"bairros":   bairros,
		"areas":     a.areaRefs(a.sipom.CityAreas(cid)),
	})
}

// PUT /api/sipom/area-refs {"city": "...", "neighborhood": "...", "area_id": 33}
//
// Grava a referência. city vem do catálogo ou da fila (a grafia do
// relatório casa com o catálogo pelo nome). Onde a área é da cidade
// inteira, o bairro é ignorado. Recalcula a fila.
func (a *app) handleSipomAreaRefSet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	var req struct {
		City         string `json:"city"`
		Neighborhood string `json:"neighborhood"`
		AreaID       int    `json:"area_id"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	cid, ok := a.sipom.CidadeID(req.City)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "cidade fora do catálogo do SIPOM")
		return
	}
	if !a.sipom.IsActiveCompany(req.AreaID) {
		httpx.Error(w, http.StatusBadRequest, "área inexistente ou desativada no SIPOM")
		return
	}
	nb := strings.ToUpper(strings.Join(strings.Fields(req.Neighborhood), " "))
	var bairroID *int
	if b, ok := a.sipom.MatchBairro(cid, nb); ok {
		bairroID = &b
	}
	cityWide := a.sipom.CityWideAreas(&cid, bairroID)
	if cityWide {
		nb = ""
	}
	if opts := a.sipom.CityAreas(cid); len(opts) > 0 && !containsInt(opts, req.AreaID) {
		httpx.Error(w, http.StatusBadRequest, "essa companhia não atua na cidade, segundo o catálogo do SIPOM")
		return
	}
	me := middleware.UserFrom(r.Context())
	city := strings.ToUpper(strings.TrimSpace(req.City))
	changed, resolved, err := a.saveAreaRule(r.Context(), city, nb, req.AreaID, me.ID)
	if err != nil {
		log.Printf("sipom area ref set: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar a referência")
		return
	}
	if changed {
		aid, sid, ip, ua := a.actorInfo(r)
		_ = a.audit.Log(r.Context(), audit.Entry{
			ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
			Action:       "sipom.area_ref.set",
			ResourceType: audit.Ptr("sipom_area_rule"),
			After: map[string]any{"city": city, "neighborhood": nb, "city_wide": cityWide,
				"area_id": req.AreaID, "resolved": resolved},
		})
	}
	httpx.OK(w, map[string]any{"city_wide": cityWide, "resolved": resolved})
}

// DELETE /api/sipom/area-refs?city=...&neighborhood=...
func (a *app) handleSipomAreaRefDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	q := r.URL.Query()
	city, nb := q.Get("city"), q.Get("neighborhood")
	ok, err := a.sipomAreas.Delete(r.Context(), city, nb)
	if err != nil {
		log.Printf("sipom area ref delete: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao apagar a referência")
		return
	}
	if !ok {
		httpx.Error(w, http.StatusNotFound, "referência não encontrada")
		return
	}
	a.recomputeAreas(r.Context())
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "sipom.area_ref.delete",
		ResourceType: audit.Ptr("sipom_area_rule"),
		Before:       map[string]any{"city": city, "neighborhood": nb},
	})
	httpx.NoContent(w)
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
