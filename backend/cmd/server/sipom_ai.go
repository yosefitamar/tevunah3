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

// POST /api/ops-occurrences/{id}/sipom/ia
//
// Pede à IA (chave e modelo do próprio usuário) sugestões para as
// pendências da ocorrência que travam o envio ao SIPOM. Nada é gravado: o
// analista confirma cada sugestão, que então passa pelo endpoint de
// correção de sempre.
func (a *app) handleSipomAISuggest(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	id := r.PathValue("id")
	so, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	terms, err := a.sipomTerms.Load(r.Context())
	if err != nil {
		log.Printf("sipom ia terms: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler os termos aprendidos")
		return
	}
	me := middleware.UserFrom(r.Context())
	active, err := a.ai.ActiveGuidance(r.Context(), sipomai.Feature, me.ID)
	if err != nil {
		log.Printf("sipom ia orientações: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler as orientações")
		return
	}
	guidance := make([]sipomai.Guidance, 0, len(active))
	for _, g := range active {
		guidance = append(guidance, sipomai.Guidance{
			ID: g.ID, Topic: g.Topic, CidadeID: g.CidadeID, Body: g.Body, Author: g.AuthorName,
			Date: g.CreatedAt.In(a.agencyLocation()).Format("02/01/2006"), Pending: g.Status == ai.GuidancePending,
		})
	}
	req, ok := sipomai.Build(a.sipom, terms, so, guidance)
	if !ok {
		httpx.OK(w, map[string]any{"suggestions": []sipomai.Suggestion{}, "sections": []string{}})
		return
	}

	res, err := a.ai.Structured(r.Context(), me.ID, ai.StructuredRequest{
		Feature:   sipomai.Feature,
		System:    sipomai.System,
		Prompt:    req.Prompt,
		Schema:    sipomai.Schema,
		MaxTokens: 6000,
		Effort:    "low",
	})
	var suggestions []sipomai.Suggestion
	if err == nil {
		if suggestions, err = req.Parse(res.JSON); err != nil {
			log.Printf("sipom ia parse: %v", err)
			err = ai.ErrBadOutput
		}
	}

	aid, sid, ip, ua := a.actorInfo(r)
	entry := audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.sipom.ia",
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
	}
	if res != nil {
		after := map[string]any{
			"model": res.Model, "sections": req.Sections,
			"input_tokens": res.Usage.InputTokens, "output_tokens": res.Usage.OutputTokens,
		}
		if err != nil {
			after["error"] = err.Error()
		} else {
			after["suggestions"] = len(suggestions)
		}
		entry.After = after
		_ = a.audit.Log(r.Context(), entry)
	}

	if err != nil {
		switch {
		case errors.Is(err, ai.ErrNoKey):
			httpx.Error(w, http.StatusPreconditionFailed, "cadastre sua chave da API Anthropic em SISTEMA → ASSISTENTE IA")
		case errors.Is(err, ai.ErrKeyRejected), errors.Is(err, ai.ErrKeyForbidden):
			httpx.Error(w, http.StatusPreconditionFailed, err.Error()+" (SISTEMA → ASSISTENTE IA)")
		case errors.Is(err, ai.ErrRateLimited):
			httpx.Error(w, http.StatusTooManyRequests, err.Error())
		case errors.Is(err, ai.ErrRefused), errors.Is(err, ai.ErrTruncated), errors.Is(err, ai.ErrBadOutput),
			errors.Is(err, ai.ErrUpstream):
			httpx.Error(w, http.StatusBadGateway, err.Error())
		default:
			log.Printf("sipom ia: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao consultar a IA")
		}
		return
	}

	if used := sipomai.UsedGuidance(suggestions); len(used) > 0 {
		if err := a.ai.MarkGuidanceUsed(r.Context(), used); err != nil {
			log.Printf("sipom ia uso das orientações: %v", err)
		}
	}
	var cidade *sipomRefJSON
	if req.CidadeID != 0 {
		cidade = &sipomRefJSON{ID: req.CidadeID, Nome: a.sipom.CidadeNome(req.CidadeID)}
	}
	cost := 0.0
	if m := ai.FindModel(res.Model); m != nil {
		u := res.Usage
		cost = m.Cost(u.InputTokens, u.OutputTokens, u.CacheWriteTokens, u.CacheReadTokens)
	}
	httpx.OK(w, map[string]any{
		"suggestions": suggestions,
		"sections":    req.Sections,
		"model":       res.Model,
		"cost_usd":    cost,
		"cidade":      cidade,
	})
}
