package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/belia/tevunah/backend/internal/ai"
	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/sipomai"
)

// Orientações dos analistas para a IA nas pendências do SIPOM. Quem corrige
// ocorrências (opsreport.update) escreve; vale só para o autor até um
// administrador (ai.guidance.approve) aprovar — aí vale para todos.
//
//	GET  /api/ai/orientacoes                 lista (admin: todas; demais: aprovadas + as próprias)
//	POST /api/ai/orientacoes                 {topic, cidade_id, body, occurrence_id}
//	PUT  /api/ai/orientacoes/{id}            {topic, cidade_id, body} — autor (pendente) ou admin (pendente/aprovada)
//	POST /api/ai/orientacoes/{id}/decisao    {decisao: aprovar|recusar|desativar|reativar, nota}
//
// A criada pelo administrador já nasce aprovada (vale para todos).

const guidanceApprovePerm = "ai.guidance.approve"

type guidanceJSON struct {
	ai.Guidance
	Cidade string `json:"cidade"`
	Mine   bool   `json:"mine"`
}

func (a *app) guidanceOut(g ai.Guidance, me string) guidanceJSON {
	j := guidanceJSON{Guidance: g, Mine: g.CreatedBy == me}
	if g.CidadeID != nil && a.sipom != nil {
		j.Cidade = a.sipom.CidadeNome(*g.CidadeID)
	}
	return j
}

func (a *app) handleGuidanceList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	me := middleware.UserFrom(r.Context())
	isAdmin := a.hasPerm(r, guidanceApprovePerm)
	list, err := a.ai.ListGuidance(r.Context(), sipomai.Feature, me.ID, isAdmin)
	if err != nil {
		log.Printf("ai guidance list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar as orientações")
		return
	}
	items := make([]guidanceJSON, 0, len(list))
	pending := 0
	for _, g := range list {
		items = append(items, a.guidanceOut(g, me.ID))
		if g.Status == ai.GuidancePending {
			pending++
		}
	}
	httpx.OK(w, map[string]any{"items": items, "can_approve": isAdmin, "pending": pending, "topics": ai.GuidanceTopics})
}

type guidanceReq struct {
	Topic        string  `json:"topic"`
	CidadeID     *int    `json:"cidade_id"`
	Body         string  `json:"body"`
	OccurrenceID *string `json:"occurrence_id"`
}

func (a *app) decodeGuidance(w http.ResponseWriter, r *http.Request) (*ai.GuidanceInput, bool) {
	var req guidanceReq
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return nil, false
	}
	if req.CidadeID != nil && (a.sipom == nil || a.sipom.CidadeNome(*req.CidadeID) == "") {
		httpx.Error(w, http.StatusBadRequest, "cidade inexistente no catálogo do SIPOM")
		return nil, false
	}
	if req.OccurrenceID != nil && *req.OccurrenceID == "" {
		req.OccurrenceID = nil
	}
	return &ai.GuidanceInput{Topic: req.Topic, CidadeID: req.CidadeID, Body: req.Body, SourceOccID: req.OccurrenceID}, true
}

func guidanceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ai.ErrGuidanceText), errors.Is(err, ai.ErrGuidanceTopic):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ai.ErrGuidanceNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ai.ErrGuidanceState):
		httpx.Error(w, http.StatusConflict, err.Error())
	default:
		log.Printf("ai guidance: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar a orientação")
	}
}

func (a *app) auditGuidance(r *http.Request, action string, id string, before, after *ai.Guidance) {
	aid, sid, ip, ua := a.actorInfo(r)
	e := audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action: action, ResourceType: audit.Ptr("ai_guidance"), ResourceID: &id,
	}
	snap := func(g *ai.Guidance) map[string]any {
		return map[string]any{"topic": g.Topic, "cidade_id": g.CidadeID, "body": g.Body, "status": g.Status, "decision_note": g.DecisionNote}
	}
	if before != nil {
		e.Before = snap(before)
	}
	if after != nil {
		e.After = snap(after)
	}
	_ = a.audit.Log(r.Context(), e)
}

func (a *app) handleGuidanceCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	in, ok := a.decodeGuidance(w, r)
	if !ok {
		return
	}
	me := middleware.UserFrom(r.Context())
	g, err := a.ai.CreateGuidance(r.Context(), sipomai.Feature, me.ID, *in)
	if err != nil {
		guidanceError(w, err)
		return
	}
	a.auditGuidance(r, "ai.guidance.create", g.ID, nil, g)
	// Do administrador, já vale para todos.
	if a.hasPerm(r, guidanceApprovePerm) {
		before := g
		if ap, err := a.ai.DecideGuidance(r.Context(), g.ID, ai.GuidanceApproved, me.ID, "criada pelo administrador"); err != nil {
			log.Printf("ai guidance auto-aprovar: %v", err)
		} else {
			a.auditGuidance(r, "ai.guidance.aprovar", g.ID, before, ap)
			g = ap
		}
	}
	httpx.OK(w, map[string]any{"item": a.guidanceOut(*g, me.ID)})
}

func (a *app) handleGuidanceUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	id := r.PathValue("id")
	me := middleware.UserFrom(r.Context())
	before, err := a.ai.GetGuidance(r.Context(), id)
	if err != nil {
		guidanceError(w, err)
		return
	}
	isAdmin := a.hasPerm(r, guidanceApprovePerm)
	if before.CreatedBy != me.ID && !isAdmin {
		httpx.Error(w, http.StatusForbidden, "só o autor ou o administrador edita a orientação")
		return
	}
	in, ok := a.decodeGuidance(w, r)
	if !ok {
		return
	}
	g, err := a.ai.UpdateGuidance(r.Context(), id, *in, isAdmin)
	if err != nil {
		guidanceError(w, err)
		return
	}
	a.auditGuidance(r, "ai.guidance.update", id, before, g)
	httpx.OK(w, map[string]any{"item": a.guidanceOut(*g, me.ID)})
}

func (a *app) handleGuidanceDecide(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	id := r.PathValue("id")
	var req struct {
		Decisao string `json:"decisao"`
		Nota    string `json:"nota"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	status := map[string]string{"aprovar": ai.GuidanceApproved, "recusar": ai.GuidanceRejected,
		"desativar": ai.GuidanceDisabled, "reativar": ai.GuidanceApproved}[req.Decisao]
	if status == "" {
		httpx.Error(w, http.StatusBadRequest, "decisao deve ser aprovar, recusar, desativar ou reativar")
		return
	}
	me := middleware.UserFrom(r.Context())
	before, err := a.ai.GetGuidance(r.Context(), id)
	if err != nil {
		guidanceError(w, err)
		return
	}
	isAdmin := a.hasPerm(r, guidanceApprovePerm)
	// Aprovar e recusar são do administrador; desativar, também do autor.
	if !isAdmin && (status != ai.GuidanceDisabled || before.CreatedBy != me.ID) {
		httpx.Error(w, http.StatusForbidden, "só o administrador aprova ou recusa orientações")
		return
	}
	if len([]rune(req.Nota)) > 500 {
		httpx.Error(w, http.StatusBadRequest, "nota com no máximo 500 caracteres")
		return
	}
	var from []string
	if req.Decisao == "reativar" {
		from = []string{ai.GuidanceDisabled, ai.GuidanceRejected}
	}
	g, err := a.ai.DecideGuidance(r.Context(), id, status, me.ID, req.Nota, from...)
	if err != nil {
		guidanceError(w, err)
		return
	}
	a.auditGuidance(r, "ai.guidance."+req.Decisao, id, before, g)
	httpx.OK(w, map[string]any{"item": a.guidanceOut(*g, me.ID)})
}
