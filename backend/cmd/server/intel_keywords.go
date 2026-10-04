package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/intel"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// Termos que marcam a participação da inteligência nas ocorrências do
// relatório operacional (e pré-marcam o cadastro de CVLI importado do
// grupo). Só o administrador mexe: um termo mal escolhido marca ou desmarca
// ocorrências em massa na reaplicação.

type intelKeywordJSON struct {
	ID            string    `json:"id"`
	Term          string    `json:"term"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	CreatedByName string    `json:"created_by_name"`
}

func toIntelKeywordJSON(k *intel.Keyword) intelKeywordJSON {
	return intelKeywordJSON{
		ID: k.ID, Term: k.Term, Active: k.Active,
		CreatedAt: k.CreatedAt, CreatedByName: k.CreatedByName,
	}
}

func (a *app) auditIntelKeyword(r *http.Request, action string, id *string, before, after map[string]any) {
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       action,
		ResourceType: audit.Ptr("intel_keyword"),
		ResourceID:   id,
		Before:       before,
		After:        after,
	})
}

// GET /api/admin/intel-keywords
func (a *app) handleIntelKeywordsList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "intel.keywords.manage") {
		return
	}
	items, err := a.intel.List(r.Context())
	if err != nil {
		log.Printf("intel keywords list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	out := make([]intelKeywordJSON, 0, len(items))
	for i := range items {
		out = append(out, toIntelKeywordJSON(&items[i]))
	}
	httpx.OK(w, map[string]any{"items": out, "total": len(out)})
}

// POST /api/admin/intel-keywords {term}
func (a *app) handleIntelKeywordCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "intel.keywords.manage") {
		return
	}
	me := middleware.UserFrom(r.Context())
	var req struct {
		Term string `json:"term"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if len([]rune(req.Term)) > 120 {
		httpx.Error(w, http.StatusBadRequest, "termo muito longo (máx. 120 caracteres)")
		return
	}
	k, err := a.intel.Create(r.Context(), req.Term, me.ID)
	switch {
	case errors.Is(err, intel.ErrEmpty):
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, intel.ErrDuplicate):
		httpx.Error(w, http.StatusConflict, "termo já cadastrado (ou equivalente, sem acento e pontuação)")
		return
	case err != nil:
		log.Printf("intel keyword create: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao cadastrar")
		return
	}
	a.auditIntelKeyword(r, "intel.keyword.create", &k.ID, nil, map[string]any{"term": k.Term})
	httpx.Created(w, toIntelKeywordJSON(k))
}

// PATCH /api/admin/intel-keywords/{id} {active}
func (a *app) handleIntelKeywordUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "intel.keywords.manage") {
		return
	}
	id := r.PathValue("id")
	var req struct {
		Active *bool `json:"active"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.Active == nil {
		httpx.Error(w, http.StatusBadRequest, "active obrigatório (true|false)")
		return
	}
	k, err := a.intel.SetActive(r.Context(), id, *req.Active)
	if err != nil {
		if !errors.Is(err, intel.ErrNotFound) {
			log.Printf("intel keyword update: %v", err)
		}
		httpx.Error(w, http.StatusNotFound, "termo não encontrado")
		return
	}
	a.auditIntelKeyword(r, "intel.keyword.update", &k.ID,
		map[string]any{"term": k.Term, "active": !k.Active},
		map[string]any{"term": k.Term, "active": k.Active})
	httpx.OK(w, toIntelKeywordJSON(k))
}

// DELETE /api/admin/intel-keywords/{id}
func (a *app) handleIntelKeywordDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "intel.keywords.manage") {
		return
	}
	k, err := a.intel.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		if !errors.Is(err, intel.ErrNotFound) {
			log.Printf("intel keyword delete: %v", err)
		}
		httpx.Error(w, http.StatusNotFound, "termo não encontrado")
		return
	}
	a.auditIntelKeyword(r, "intel.keyword.delete", &k.ID,
		map[string]any{"term": k.Term, "active": k.Active}, nil)
	httpx.NoContent(w)
}

// POST /api/admin/intel-keywords/reapply
//
// Reavalia as ocorrências já importadas com os termos ativos. As marcadas à
// mão pelo analista não mudam.
func (a *app) handleIntelKeywordsReapply(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "intel.keywords.manage") {
		return
	}
	m, err := a.intel.Matcher(r.Context())
	if err != nil {
		log.Printf("intel reapply matcher: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao carregar os termos")
		return
	}
	res, err := a.opsReports.ReapplyIntel(r.Context(), m.Match)
	if err != nil {
		log.Printf("intel reapply: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao reaplicar os termos")
		return
	}
	out := map[string]any{"checked": res.Checked, "marked": res.Marked, "cleared": res.Cleared}
	a.auditIntelKeyword(r, "intel.keywords.reapply", nil, nil, out)
	httpx.OK(w, out)
}
