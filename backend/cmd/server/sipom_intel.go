package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// ─── Composição da inteligência ───

// GET /api/sipom/inteligencia/agentes — os agentes que podem entrar na
// composição da inteligência (marcados no cadastro, com matrícula).
func (a *app) handleSipomIntelAgentes(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	list, err := a.opsReports.IntelCandidates(r.Context())
	if err != nil {
		log.Printf("sipom intel agentes: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar os agentes")
		return
	}
	items := make([]sipomIntelOfficerJSON, 0, len(list))
	for _, x := range list {
		items = append(items, a.intelOfficerJSON(x))
	}
	funcoes := make([]sipomRefJSON, 0, 3)
	for _, id := range []int{sipom.FuncaoGestor, sipom.FuncaoAgente, sipom.FuncaoAnalista} {
		funcoes = append(funcoes, sipomRefJSON{ID: id, Nome: a.sipom.Funcao(id)})
	}
	httpx.OK(w, map[string]any{"items": items, "funcoes": funcoes})
}

// PUT /api/ops-occurrences/{id}/sipom/inteligencia
//
//	{"agentes": [{"user_id": "...", "funcao_id": 11}, ...]} | {"reset": true}
//
// Grava quem da agência entra na composição (e com que função) ou devolve a
// ocorrência ao cadastro dos agentes. A guarnição do relatório não muda.
func (a *app) handleSipomSetIntel(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	id := r.PathValue("id")
	var req struct {
		Agentes []struct {
			UserID   string `json:"user_id"`
			FuncaoID int    `json:"funcao_id"`
		} `json:"agentes"`
		Reset bool `json:"reset"`
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
	if !before.IntelParticipation {
		httpx.Error(w, http.StatusConflict, "a ocorrência não está marcada com participação da inteligência")
		return
	}
	var members []opsreport.IntelMember
	if !req.Reset {
		// Entra quem está marcado no cadastro — ou quem já estava na lista
		// desta ocorrência (ex.: agente que saiu da agência depois do envio).
		allowed := map[string]bool{}
		for _, x := range before.IntelOfficers {
			allowed[x.UserID] = true
		}
		cands, err := a.opsReports.IntelCandidates(r.Context())
		if err != nil {
			log.Printf("sipom intel agentes: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao listar os agentes")
			return
		}
		for _, x := range cands {
			allowed[x.UserID] = true
		}
		members = make([]opsreport.IntelMember, 0, len(req.Agentes))
		seen := map[string]bool{}
		for _, m := range req.Agentes {
			if !allowed[m.UserID] {
				httpx.Error(w, http.StatusBadRequest, "agente fora da composição do SIPOM (confira o cadastro)")
				return
			}
			if !sipom.IntelFuncao(m.FuncaoID) {
				httpx.Error(w, http.StatusBadRequest, "função inválida para o policiamento de Inteligência")
				return
			}
			if seen[m.UserID] {
				continue
			}
			seen[m.UserID] = true
			members = append(members, opsreport.IntelMember{UserID: m.UserID, FuncaoID: m.FuncaoID})
		}
		if len(before.Officers)+len(members) > sipom.MaxComposicao {
			httpx.Error(w, http.StatusBadRequest, "a composição passa do limite do SIPOM (30 policiais)")
			return
		}
	}
	if err := a.opsReports.SetIntelComposition(r.Context(), id, members); err != nil {
		if errors.Is(err, opsreport.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
			return
		}
		log.Printf("sipom intel set: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
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
		Action:       "opsreport.sipom.inteligencia",
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
		Before:       intelAudit(before),
		After:        intelAudit(after),
	})
	occ := a.toOpsOccurrenceJSON(&after.Occurrence)
	occ.Sipom = a.sipomJSON(&after.Occurrence, opsreport.SipomPeople(after))
	a.markAreaLearned(r.Context(), &after.Occurrence, occ.Sipom)
	httpx.OK(w, map[string]any{"sipom": occ.Sipom, "officers": occ.Officers})
}

// GET /api/sipom/postos — postos/graduações do SIPOM, para o cadastro dos
// agentes.
func (a *app) handleSipomPostos(w http.ResponseWriter, r *http.Request) {
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	items := make([]sipomRefJSON, 0)
	for _, p := range a.sipom.Postos() {
		items = append(items, sipomRefJSON{ID: p.ID, Nome: p.Nome})
	}
	httpx.OK(w, map[string]any{"items": items})
}
