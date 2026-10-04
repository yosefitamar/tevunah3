package main

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// Endpoints e JSON da tradução para o SIPOM (a lógica fica em
// internal/opsreport/sipom.go e internal/sipom).

// POST /api/ops-occurrences/sipom/recompute
func (a *app) handleSipomRecompute(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.import") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	ready, pending, err := a.opsReports.RecomputeSipom(r.Context(), a.sipom, "")
	if err != nil {
		log.Printf("sipom recompute: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao recalcular")
		return
	}
	out := map[string]any{"ready": ready, "pending": pending}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action: "opsreport.sipom.recompute", After: out,
	})
	httpx.OK(w, out)
}

// ─── JSON da ficha ───

type sipomRefJSON struct {
	ID   int    `json:"id"`
	Nome string `json:"nome"`
}

type sipomPendingJSON struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Blocking bool   `json:"blocking"`
}

// sipomPersonJSON é o envolvido como vai ao SIPOM (modal "Adicionar Pessoa").
type sipomPersonJSON struct {
	Vinculo      string  `json:"vinculo"`
	VinculoLabel string  `json:"vinculo_label"`
	Nome         string  `json:"nome"`
	CPF          string  `json:"cpf"`
	Sexo         int     `json:"sexo"`
	Nascimento   *string `json:"nascimento"`
	Mae          string  `json:"mae"`
	Morte        bool    `json:"morte"`
	Foto         bool    `json:"foto"`
	Fonte        string  `json:"fonte"`
}

type sipomOccurrenceJSON struct {
	Natureza    *sipomRefJSON      `json:"natureza"`
	Logradouro  string             `json:"logradouro"`
	Numeral     string             `json:"numeral"`
	Cidade      *sipomRefJSON      `json:"cidade"`
	Bairro      *sipomRefJSON      `json:"bairro"`
	Area        *sipomRefJSON      `json:"area"`
	AreaOptions []sipomRefJSON     `json:"area_options"`
	OPM         *sipomRefJSON      `json:"opm"`
	Pendencias  []sipomPendingJSON `json:"pendencias"`
	Ready       bool               `json:"ready"`
	Manual      []string           `json:"manual"`
	Pessoas     []sipomPersonJSON  `json:"pessoas"`
	// Listas de escolha para as correções do analista.
	AreaCandidates []sipomRefJSON `json:"area_candidates"`
	OPMCandidates  []sipomRefJSON `json:"opm_candidates"`
	Equipes        []string       `json:"equipes"`
}

// sipomJSON monta o bloco SIPOM da ficha. As áreas candidatas (local que cai
// em mais de uma companhia) são recalculadas aqui, do catálogo em memória.
func (a *app) sipomJSON(o *opsreport.Occurrence, people []sipom.Person) *sipomOccurrenceJSON {
	if a.sipom == nil {
		return nil
	}
	f := o.Sipom
	out := &sipomOccurrenceJSON{
		Logradouro: f.Logradouro, Numeral: f.Numeral,
		AreaOptions: []sipomRefJSON{}, Pendencias: []sipomPendingJSON{},
		Manual: nonNil(f.Manual), Ready: !opsreport.SipomBlocked(f.Pending),
		Pessoas:        make([]sipomPersonJSON, 0, len(people)),
		AreaCandidates: []sipomRefJSON{}, OPMCandidates: []sipomRefJSON{},
		Equipes: splitTeams(o.Teams),
	}
	if f.CidadeID != nil {
		for _, id := range a.sipom.CityAreas(*f.CidadeID) {
			co, _ := a.sipom.Companhia(id)
			out.AreaCandidates = append(out.AreaCandidates, sipomRefJSON{ID: id, Nome: co.Abreviado})
		}
	}
	for _, co := range a.sipom.BattalionCompanies(o.Unit()) {
		out.OPMCandidates = append(out.OPMCandidates, sipomRefJSON{ID: co.ID, Nome: co.Abreviado})
	}
	for _, p := range people {
		pj := sipomPersonJSON{
			Vinculo: p.Vinculo, VinculoLabel: sipom.VinculoLabel[p.Vinculo],
			Nome: p.Nome, CPF: p.CPF, Sexo: p.Sexo, Mae: p.Mae,
			Morte: p.Morte, Foto: p.Foto, Fonte: p.Fonte,
		}
		if p.Nascimento != nil {
			d := p.Nascimento.Format("2006-01-02")
			pj.Nascimento = &d
		}
		out.Pessoas = append(out.Pessoas, pj)
	}
	if f.NaturezaID != nil {
		if n, ok := a.sipom.Natureza(*f.NaturezaID); ok {
			out.Natureza = &sipomRefJSON{ID: n.ID, Nome: n.Rotulo}
		}
	}
	if f.CidadeID != nil {
		out.Cidade = &sipomRefJSON{ID: *f.CidadeID, Nome: a.sipom.CidadeNome(*f.CidadeID)}
	}
	if f.BairroID != nil {
		out.Bairro = &sipomRefJSON{ID: *f.BairroID, Nome: a.sipom.BairroNome(*f.BairroID)}
	}
	company := func(id *int) *sipomRefJSON {
		if id == nil {
			return nil
		}
		co, _ := a.sipom.Companhia(*id)
		return &sipomRefJSON{ID: *id, Nome: co.Abreviado}
	}
	out.Area, out.OPM = company(f.AreaID), company(f.OPMID)
	for _, p := range f.Pending {
		out.Pendencias = append(out.Pendencias, sipomPendingJSON{
			Code: p, Label: sipom.PendingLabel[p], Blocking: sipom.Blocking(p),
		})
		if p == sipom.PendAreaAmbig {
			for _, id := range a.sipom.Resolve(opsreport.SipomInput(o)).AreaOptions {
				out.AreaOptions = append(out.AreaOptions, *company(&id))
			}
		}
	}
	return out
}

// sipomOfficerLabels traduz tipo e função da composição para exibição.
func (a *app) sipomOfficerLabels(f opsreport.Officer) (policiamento, funcao string) {
	if a.sipom == nil {
		return "", ""
	}
	if f.SipomPoliciamentoTipoID != nil {
		policiamento = a.sipom.Policiamento(*f.SipomPoliciamentoTipoID)
	}
	if f.SipomFuncaoID != nil {
		funcao = a.sipom.Funcao(*f.SipomFuncaoID)
	}
	return policiamento, funcao
}

// ─── Natureza ───

// GET /api/sipom/naturezas — catálogo de naturezas do SIPOM, para escolher
// na ficha e no de-para.
func (a *app) handleSipomNaturezas(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	out := []map[string]any{}
	for _, n := range a.sipom.Naturezas() {
		out = append(out, map[string]any{"id": n.ID, "nome": n.Nome, "rotulo": n.Rotulo})
	}
	httpx.OK(w, map[string]any{"items": out})
}

// PUT /api/ops-occurrences/{id}/sipom/natureza {natureza_id}
//
// O analista fixa a natureza (vale sobre o de-para) ou, com null, devolve a
// decisão ao de-para.
func (a *app) handleSipomSetNatureza(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	id := r.PathValue("id")
	var req struct {
		NaturezaID *int `json:"natureza_id"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if req.NaturezaID != nil {
		if _, ok := a.sipom.Natureza(*req.NaturezaID); !ok {
			httpx.Error(w, http.StatusBadRequest, "natureza inexistente no SIPOM")
			return
		}
	}
	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	if err := a.opsReports.SetSipomNatureza(r.Context(), id, req.NaturezaID); err != nil {
		log.Printf("sipom natureza: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		return
	}
	if _, _, err := a.opsReports.RecomputeSipom(r.Context(), a.sipom, id); err != nil {
		log.Printf("sipom recompute one: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao recalcular")
		return
	}
	after, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler a ocorrência")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.sipom.natureza",
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
		Before:       map[string]any{"natureza_id": before.Sipom.NaturezaID, "manual": nonNil(before.Sipom.Manual)},
		After:        map[string]any{"natureza_id": after.Sipom.NaturezaID, "manual": nonNil(after.Sipom.Manual)},
	})
	httpx.OK(w, map[string]any{"sipom": a.sipomJSON(&after.Occurrence, opsreport.SipomPeople(after))})
}

// ─── De-para (Admin) ───

type sipomRuleJSON struct {
	ID          string        `json:"id"`
	Source      string        `json:"source"`
	Natureza    *sipomRefJSON `json:"natureza"`
	Confianca   string        `json:"confianca"`
	Prioridade  int           `json:"prioridade"`
	Ocorrencias int           `json:"ocorrencias"`
}

// GET /api/admin/sipom/natureza-map — regras do de-para, com quantas
// ocorrências do acervo cada uma alcança, e as naturezas do acervo que ainda
// não têm regra.
func (a *app) handleSipomMapList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "sipom.mapping.manage") {
		return
	}
	rules, err := a.sipomMap.List(r.Context())
	if err != nil {
		log.Printf("sipom map list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	seen, err := a.opsReports.SeenNatures(r.Context())
	if err != nil {
		log.Printf("sipom seen natures: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	counts := map[string]int{}
	for _, s := range seen {
		counts[sipom.NatureKey(s.Nature)] += s.Count
	}
	out := make([]sipomRuleJSON, 0, len(rules))
	known := map[string]bool{}
	for _, x := range rules {
		k := sipom.NatureKey(x.Source)
		known[k] = true
		out = append(out, a.sipomRuleJSON(x, counts[k]))
	}
	unmapped := []map[string]any{}
	added := map[string]bool{}
	for _, s := range seen {
		k := sipom.NatureKey(s.Nature)
		if known[k] || added[k] {
			continue
		}
		added[k] = true
		unmapped = append(unmapped, map[string]any{"nature": s.Nature, "ocorrencias": counts[k]})
	}
	httpx.OK(w, map[string]any{"rules": out, "unmapped": unmapped})
}

func (a *app) sipomRuleJSON(x sipom.NatureRule, count int) sipomRuleJSON {
	j := sipomRuleJSON{
		ID: x.ID, Source: x.Source, Confianca: x.Confianca,
		Prioridade: x.Prioridade, Ocorrencias: count,
	}
	if x.NaturezaID != nil && a.sipom != nil {
		if n, ok := a.sipom.Natureza(*x.NaturezaID); ok {
			j.Natureza = &sipomRefJSON{ID: n.ID, Nome: n.Rotulo}
		}
	}
	return j
}

// PUT /api/admin/sipom/natureza-map {source, natureza_id, confianca, prioridade}
//
// Grava a regra e recalcula o acervo — a mudança vale na hora para as
// ocorrências que o analista não fixou à mão.
func (a *app) handleSipomMapUpsert(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "sipom.mapping.manage") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	var req struct {
		Source     string `json:"source"`
		NaturezaID *int   `json:"natureza_id"`
		Confianca  string `json:"confianca"`
		Prioridade int    `json:"prioridade"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if req.NaturezaID != nil {
		if _, ok := a.sipom.Natureza(*req.NaturezaID); !ok {
			httpx.Error(w, http.StatusBadRequest, "natureza inexistente no SIPOM")
			return
		}
	}
	if req.Prioridade < 1 || req.Prioridade > 999 {
		httpx.Error(w, http.StatusBadRequest, "prioridade entre 1 e 999")
		return
	}
	me := middleware.UserFrom(r.Context())
	rule, err := a.sipomMap.Upsert(r.Context(), req.Source, req.NaturezaID, req.Confianca, req.Prioridade, me.ID)
	if err != nil {
		if errors.Is(err, sipom.ErrEmptySource) {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("sipom map upsert: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		return
	}
	ready, pending, err := a.opsReports.RecomputeSipom(r.Context(), a.sipom, "")
	if err != nil {
		log.Printf("sipom recompute after map: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "regra gravada, mas houve erro ao recalcular")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "sipom.mapping.upsert",
		ResourceType: audit.Ptr("sipom_natureza_map"),
		ResourceID:   &rule.ID,
		After: map[string]any{
			"source": rule.Source, "natureza_id": rule.NaturezaID,
			"confianca": rule.Confianca, "prioridade": rule.Prioridade,
		},
	})
	httpx.OK(w, map[string]any{
		"rule": a.sipomRuleJSON(*rule, 0), "ready": ready, "pending": pending,
	})
}

// sipomRecomputeOne refaz a tradução de uma ocorrência depois de mudança
// que a afeta (vínculo de pessoa). Falha só registra: a ficha já foi gravada
// e o próximo recálculo corrige.
func (a *app) sipomRecomputeOne(ctx context.Context, id string) {
	if a.sipom == nil {
		return
	}
	if _, _, err := a.opsReports.RecomputeSipom(ctx, a.sipom, id); err != nil {
		log.Printf("sipom recompute %s: %v", id, err)
	}
}

func splitTeams(teams string) []string {
	out := []string{}
	for _, t := range strings.FieldsFunc(teams, func(r rune) bool { return r == ';' || r == ',' }) {
		if t = strings.ToUpper(strings.Join(strings.Fields(t), " ")); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ─── Correções do analista ───

// PUT /api/ops-occurrences/{id}/sipom/{field}
//
//	area        {"area_id": 123 | null}
//	opm         {"opm_id": 92 | null}
//	endereco    {"logradouro": "...", "numeral": "..."} | {"reset": true}
//	composicao  {"equipes": ["RAIO 01", "RAIO 02", ...]} | {"reset": true}
//
// O valor fixado prevalece sobre o automático; null/reset devolve ao
// automático. A ocorrência é recalculada na hora.
func (a *app) handleSipomSetField(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	id, field := r.PathValue("id"), r.PathValue("field")
	var req struct {
		AreaID     *int     `json:"area_id"`
		OPMID      *int     `json:"opm_id"`
		Logradouro string   `json:"logradouro"`
		Numeral    string   `json:"numeral"`
		Equipes    []string `json:"equipes"`
		Reset      bool     `json:"reset"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	switch field {
	case opsreport.SipomFieldArea:
		if req.AreaID != nil && !a.sipom.IsActiveCompany(*req.AreaID) {
			httpx.Error(w, http.StatusBadRequest, "área inexistente ou desativada no SIPOM")
			return
		}
		err = a.opsReports.SetSipomArea(r.Context(), id, req.AreaID)
	case opsreport.SipomFieldOPM:
		if req.OPMID != nil && !a.sipom.IsActiveCompany(*req.OPMID) {
			httpx.Error(w, http.StatusBadRequest, "companhia inexistente ou desativada no SIPOM")
			return
		}
		err = a.opsReports.SetSipomOPM(r.Context(), id, req.OPMID)
	case opsreport.SipomFieldEndereco:
		logr := strings.Join(strings.Fields(req.Logradouro), " ")
		if !req.Reset && logr == "" {
			httpx.Error(w, http.StatusBadRequest, "informe o logradouro")
			return
		}
		err = a.opsReports.SetSipomEndereco(r.Context(), id, logr, strings.TrimSpace(req.Numeral), req.Reset)
	case opsreport.SipomFieldComposicao:
		var officers []opsreport.Officer
		if !req.Reset {
			if len(req.Equipes) != len(before.Officers) {
				httpx.Error(w, http.StatusBadRequest, "informe a equipe de cada policial da composição")
				return
			}
			comp, pend := sipom.CompositionByTeam(req.Equipes)
			if pend != "" {
				httpx.Error(w, http.StatusBadRequest, sipom.PendingLabel[pend])
				return
			}
			officers = make([]opsreport.Officer, len(comp))
			for i, c := range comp {
				officers[i] = opsreport.Officer{SipomEquipe: c.Equipe,
					SipomPoliciamentoTipoID: c.PoliciamentoTipoID, SipomFuncaoID: c.FuncaoID}
			}
		}
		err = a.opsReports.SetSipomComposicao(r.Context(), id, officers)
	default:
		httpx.Error(w, http.StatusNotFound, "campo desconhecido")
		return
	}
	if err != nil {
		log.Printf("sipom set %s: %v", field, err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		return
	}
	a.sipomRecomputeOne(r.Context(), id)
	after, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler a ocorrência")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.sipom." + field,
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
		Before:       sipomAuditSnapshot(before),
		After:        sipomAuditSnapshot(after),
	})
	occ := a.toOpsOccurrenceJSON(&after.Occurrence)
	occ.Sipom = a.sipomJSON(&after.Occurrence, opsreport.SipomPeople(after))
	httpx.OK(w, map[string]any{"sipom": occ.Sipom, "officers": occ.Officers})
}

func sipomAuditSnapshot(so *opsreport.StoredOccurrence) map[string]any {
	equipes := make([]string, 0, len(so.Officers))
	for _, f := range so.Officers {
		equipes = append(equipes, f.SipomEquipe)
	}
	return map[string]any{
		"area_id": so.Sipom.AreaID, "opm_id": so.Sipom.OPMID,
		"logradouro": so.Sipom.Logradouro, "numeral": so.Sipom.Numeral,
		"equipes": equipes, "manual": nonNil(so.Sipom.Manual), "pendencias": nonNil(so.Sipom.Pending),
	}
}

// ─── Fila de envio ───

type sipomQueueRowJSON struct {
	ID          string             `json:"id"`
	CIOPS       string             `json:"ciops_record"`
	OccurredOn  string             `json:"occurred_on"`
	StartTime   string             `json:"start_time"`
	Natures     []string           `json:"natures"`
	Natureza    *sipomRefJSON      `json:"natureza"`
	City        string             `json:"place_city"`
	Neigh       string             `json:"place_neighborhood"`
	Area        *sipomRefJSON      `json:"area"`
	OPM         *sipomRefJSON      `json:"opm"`
	Pendencias  []sipomPendingJSON `json:"pendencias"`
	Ready       bool               `json:"ready"`
	People      int                `json:"people_count"`
	Unlinked    int                `json:"unlinked_count"`
	Officers    int                `json:"officer_count"`
	IntelPartic bool               `json:"intel_participation"`
}

// GET /api/ops-occurrences/sipom/queue?date_from&date_to
//
// Fila de envio: todas as ocorrências do período com a situação no SIPOM,
// mais o resumo (prontas, pendentes e quantas por motivo) para os filtros.
func (a *app) handleSipomQueue(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	q := r.URL.Query()
	rows, err := a.opsReports.SipomQueue(r.Context(), q.Get("date_from"), q.Get("date_to"))
	if err != nil {
		log.Printf("sipom queue: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	company := func(id *int) *sipomRefJSON {
		if id == nil {
			return nil
		}
		co, _ := a.sipom.Companhia(*id)
		return &sipomRefJSON{ID: *id, Nome: co.Abreviado}
	}
	items := make([]sipomQueueRowJSON, 0, len(rows))
	ready := 0
	byCode := map[string]int{}
	for _, x := range rows {
		j := sipomQueueRowJSON{
			ID: x.ID, CIOPS: x.CIOPS, OccurredOn: x.OccurredOn.Format("2006-01-02"), StartTime: x.StartTime,
			Natures: nonNil(x.Natures), City: x.City, Neigh: x.Neigh,
			Area: company(x.AreaID), OPM: company(x.OPMID),
			Pendencias: []sipomPendingJSON{}, Ready: !opsreport.SipomBlocked(x.Pending),
			People: x.People, Unlinked: x.Unlinked, Officers: x.Officers, IntelPartic: x.IntelPartic,
		}
		if x.NaturezaID != nil {
			if n, ok := a.sipom.Natureza(*x.NaturezaID); ok {
				j.Natureza = &sipomRefJSON{ID: n.ID, Nome: n.Rotulo}
			}
		}
		for _, p := range x.Pending {
			j.Pendencias = append(j.Pendencias, sipomPendingJSON{
				Code: p, Label: sipom.PendingLabel[p], Blocking: sipom.Blocking(p),
			})
			byCode[p]++
		}
		if j.Ready {
			ready++
		}
		items = append(items, j)
	}
	httpx.OK(w, map[string]any{
		"items":   items,
		"summary": map[string]any{"total": len(items), "ready": ready, "pending": len(items) - ready, "by_code": byCode},
		"labels":  sipom.PendingLabel,
	})
}

// POST /api/ops-occurrences/sipom/natureza/confirm {ids}
//
// Confirma em lote a natureza sugerida pelo de-para.
func (a *app) handleSipomConfirmNaturezas(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := httpx.Decode(r, &req); err != nil || len(req.IDs) == 0 || len(req.IDs) > 500 {
		httpx.Error(w, http.StatusBadRequest, "informe de 1 a 500 ocorrências")
		return
	}
	done, err := a.opsReports.ConfirmSipomNaturezas(r.Context(), req.IDs)
	if err != nil {
		log.Printf("sipom confirm: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao confirmar")
		return
	}
	for _, id := range done {
		a.sipomRecomputeOne(r.Context(), id)
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action: "opsreport.sipom.natureza.confirm",
		After:  map[string]any{"occurrence_ids": done},
	})
	httpx.OK(w, map[string]any{"confirmed": len(done)})
}

// ─── Payload (contrato do POST ao SIPOM) ───

// Foto maior que isto não vai no envio (o dossiê aceita fotos grandes; o
// SIPOM recorta e guarda miniatura).
const sipomPhotoMaxBytes = 5 << 20

// sipomPhotoLoader lê a foto principal do dossiê para o payload, com as
// mesmas regras de quem a vê na tela: dossiê dentro do clearance, caminho
// sob PHOTO_DIR e auditoria. Sem withData, devolve só o tipo (prévia).
func (a *app) sipomPhotoLoader(r *http.Request, clearance int, withData bool) opsreport.PhotoLoader {
	return func(entityID string) *sipom.PayloadFoto {
		e, err := a.entities.FindByID(r.Context(), entityID)
		if err != nil || e.DeletedAt != nil || e.Classification > clearance {
			return nil
		}
		pp := primaryPhotoPath(e)
		if pp == "" {
			return nil
		}
		path := filepath.Join(photoDir(), pp)
		abs, err := filepath.Abs(path)
		if err != nil || !strings.HasPrefix(abs, photoDir()) {
			return nil
		}
		mime := "image/jpeg"
		if strings.HasSuffix(strings.ToLower(pp), ".png") {
			mime = "image/png"
		}
		if !withData {
			return &sipom.PayloadFoto{Mime: mime}
		}
		data, err := os.ReadFile(abs)
		if err != nil || len(data) > sipomPhotoMaxBytes {
			return nil
		}
		aid, sid, ip, ua := a.actorInfo(r)
		classPtr := e.Classification
		_ = a.audit.Log(r.Context(), audit.Entry{
			ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
			Action:                 "entity.photo.view",
			ResourceType:           audit.Ptr("entity"),
			ResourceID:             audit.Ptr(entityID),
			ResourceClassification: &classPtr,
			After:                  map[string]any{"contexto": "sipom.payload"},
		})
		return &sipom.PayloadFoto{Mime: mime, Base64: base64.StdEncoding.EncodeToString(data)}
	}
}

// GET /api/ops-occurrences/{id}/sipom/payload[?fotos=1]
//
// Prévia do corpo do POST ao SIPOM para a ocorrência: exatamente o que seria
// enviado. Ocorrência com pendência volta com payload null e as pendências.
// Sem fotos=1, as fotos vêm sem o conteúdo (só o tipo).
func (a *app) handleSipomPayload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	me := middleware.UserFrom(r.Context())
	so, err := a.opsReports.FindByID(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	unidade := "SAI/" + a.opsUnit
	payload, err := opsreport.BuildSipomPayload(a.sipom, so, unidade,
		a.sipomPhotoLoader(r, me.ClearanceLevel, r.URL.Query().Get("fotos") == "1"))
	var nr *opsreport.NotReadyError
	switch {
	case errors.As(err, &nr):
		pend := make([]sipomPendingJSON, 0, len(nr.Pending))
		for _, p := range nr.Pending {
			pend = append(pend, sipomPendingJSON{Code: p, Label: sipom.PendingLabel[p], Blocking: true})
		}
		httpx.OK(w, map[string]any{"ready": false, "pendencias": pend, "payload": nil})
		return
	case err != nil:
		log.Printf("sipom payload: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao montar o envio")
		return
	}
	httpx.OK(w, map[string]any{"ready": true, "pendencias": []sipomPendingJSON{}, "payload": payload})
}
