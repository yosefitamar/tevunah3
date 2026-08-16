package main

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// Marcação de óbito de pessoa.
//
// O caminho normal é automático: vincular a pessoa como VÍTIMA de um
// homicídio marca o óbito (ver handleIncidentEntityAdd). Este arquivo cobre
// o inverso — desfazer uma marcação feita sobre o homônimo errado — e a
// marcação manual, para o óbito conhecido sem ocorrência cadastrada.
//
// Ambas exigem entity.update: quem edita o dossiê corrige o próprio erro.

// ─── DELETE /api/entities/{id}/death ───────────────────────────────────

func (a *app) handleEntityDeathClear(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")

	before, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		a.writeEntityError(w, err)
		return
	}
	after, err := a.entities.ClearDeceased(r.Context(), id, me.ID)
	if err != nil {
		a.writeEntityError(w, err)
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := after.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:                 "entity.death.clear",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(after.ID),
		ResourceClassification: &classPtr,
		Before:                 toPublicEntity(before),
		After:                  toPublicEntity(after),
	})
	httpx.OK(w, map[string]any{"entity": toPublicEntity(after)})
}

// ─── POST /api/entities/{id}/death ─────────────────────────────────────

type markDeathRequest struct {
	// Data do óbito (YYYY-MM-DD). Vazio = desconhecida.
	DeceasedOn string `json:"deceased_on"`
}

func (a *app) handleEntityDeathMark(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")

	var req markDeathRequest
	// Corpo é opcional: sem data, marca o óbito sem datar.
	_ = httpx.Decode(r, &req)
	occurredOn, ok := parseDateOpt(req.DeceasedOn)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "deceased_on inválido (esperado YYYY-MM-DD)")
		return
	}

	before, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		a.writeEntityError(w, err)
		return
	}
	after, err := a.entities.MarkDeceased(r.Context(), id, nil, occurredOn, me.ID)
	if err != nil {
		a.writeEntityError(w, err)
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := after.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:                 "entity.death.mark",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(after.ID),
		ResourceClassification: &classPtr,
		Before:                 toPublicEntity(before),
		After:                  toPublicEntity(after),
	})
	httpx.OK(w, map[string]any{"entity": toPublicEntity(after)})
}

// parseDateOpt valida "YYYY-MM-DD". Vazio → (nil, true) = data ausente;
// inválida → (nil, false).
func parseDateOpt(s string) (*time.Time, bool) {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil, true
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, false
	}
	return &t, true
}

// writeEntityError traduz os erros do repo de entidades em resposta HTTP.
func (a *app) writeEntityError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, entities.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
	case errors.Is(err, entities.ErrNotPerson):
		httpx.Error(w, http.StatusBadRequest, "óbito só se aplica a pessoas")
	case errors.Is(err, entities.ErrAlreadyDeleted):
		httpx.Error(w, http.StatusConflict, "entidade excluída")
	default:
		log.Printf("entity death: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar óbito")
	}
}
