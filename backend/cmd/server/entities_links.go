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

// publicLink é a representação JSON de um Link na perspectiva da entidade
// consultada. Inclui a direção (out/in) e os dados das duas pontas (id, kind,
// name) pra UI renderizar sem precisar de outra chamada.
type publicLink struct {
	ID           string     `json:"id"`
	Direction    string     `json:"direction"` // "out" | "in"
	FromEntityID string     `json:"from_entity_id"`
	FromKind     string     `json:"from_kind"`
	FromName     string     `json:"from_name"`
	ToEntityID   string     `json:"to_entity_id"`
	ToKind       string     `json:"to_kind"`
	ToName       string     `json:"to_name"`
	RelationType string     `json:"relation_type"`
	ValidFrom    *string    `json:"valid_from,omitempty"`
	ValidTo      *string    `json:"valid_to,omitempty"`
	Note         string     `json:"note,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	CreatedBy    string     `json:"created_by"`
}

func toPublicLink(l *entities.LinkWithDirection) publicLink {
	pl := publicLink{
		ID:           l.ID,
		Direction:    string(l.Direction),
		FromEntityID: l.FromEntityID,
		FromKind:     string(l.FromKind),
		FromName:     l.FromName,
		ToEntityID:   l.ToEntityID,
		ToKind:       string(l.ToKind),
		ToName:       l.ToName,
		RelationType: string(l.RelationType),
		Note:         l.Note,
		CreatedAt:    l.CreatedAt,
		CreatedBy:    l.CreatedBy,
	}
	if l.ValidFrom != nil {
		s := l.ValidFrom.Format("2006-01-02")
		pl.ValidFrom = &s
	}
	if l.ValidTo != nil {
		s := l.ValidTo.Format("2006-01-02")
		pl.ValidTo = &s
	}
	return pl
}

// toPublicLinkFromPlain converte um Link sem direção (vindo de FindLink) em
// publicLink. Usado pelo audit pós-criação onde sabemos a direção pelo from.
func toPublicLinkFromPlain(l *entities.Link, direction entities.Direction) publicLink {
	wd := entities.LinkWithDirection{Link: *l, Direction: direction}
	return toPublicLink(&wd)
}

// ─────────────────────────── GET /api/entities/{id}/links ──────────────

func (a *app) handleEntityLinksList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.read") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	// Confirma que a entidade consultada existe e está visível pro chamador.
	e, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if e.Classification > me.ClearanceLevel || e.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

	links, err := a.entities.ListLinksForEntity(r.Context(), id)
	if err != nil {
		log.Printf("links list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar vínculos")
		return
	}

	// Filtra os links onde a entidade do outro lado tem classification acima
	// do clearance do chamador. O domain devolve tudo; o gating fica aqui
	// porque depende do usuário.
	out := make([]publicLink, 0, len(links))
	for i := range links {
		l := &links[i]
		// Para cada link, preciso checar a classification do "outro lado".
		// Como o repo já me deu kind/name mas não classification, faço uma
		// busca extra. Em volume isso vira N+1, mas vínculos por entidade
		// costuma ser <= 50 — aceitável; otimização posterior se virar gargalo.
		otherID := l.ToEntityID
		if l.Direction == entities.DirectionIn {
			otherID = l.FromEntityID
		}
		other, err := a.entities.FindByID(r.Context(), otherID)
		if err != nil {
			continue
		}
		if other.Classification > me.ClearanceLevel || other.DeletedAt != nil {
			continue
		}
		out = append(out, toPublicLink(l))
	}

	httpx.OK(w, map[string]any{"items": out})
}

// ─────────────────────────── POST /api/entities/{id}/links ─────────────

type createLinkRequest struct {
	ToEntityID   string  `json:"to_entity_id"`
	RelationType string  `json:"relation_type"`
	ValidFrom    *string `json:"valid_from,omitempty"`
	ValidTo      *string `json:"valid_to,omitempty"`
	Note         string  `json:"note,omitempty"`
}

func (a *app) handleEntityLinkCreate(w http.ResponseWriter, r *http.Request) {
	// Criar vínculo modifica a perspectiva da entidade de origem; reusamos
	// entity.update como permissão (não há link.create separado em Phase 1).
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	var req createLinkRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if strings.TrimSpace(req.ToEntityID) == "" {
		httpx.Error(w, http.StatusBadRequest, "to_entity_id obrigatório")
		return
	}
	relation := entities.RelationType(strings.TrimSpace(req.RelationType))
	if !relation.IsValid() {
		httpx.Error(w, http.StatusBadRequest, "relation_type inválido")
		return
	}

	// Visibilidade de ambas as pontas pro chamador. Esconde 404 quando a
	// origem ou o destino estão acima do clearance.
	from, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if from.Classification > me.ClearanceLevel || from.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}
	to, err := a.entities.FindByID(r.Context(), req.ToEntityID)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade destino não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if to.Classification > me.ClearanceLevel || to.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade destino não encontrada")
		return
	}

	in := entities.NewLink{
		FromEntityID: id,
		ToEntityID:   req.ToEntityID,
		RelationType: relation,
		Note:         strings.TrimSpace(req.Note),
	}
	if req.ValidFrom != nil {
		if t, err := time.Parse("2006-01-02", *req.ValidFrom); err == nil {
			in.ValidFrom = &t
		}
	}
	if req.ValidTo != nil {
		if t, err := time.Parse("2006-01-02", *req.ValidTo); err == nil {
			in.ValidTo = &t
		}
	}

	l, err := a.entities.CreateLink(r.Context(), in, me.ID)
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		case errors.Is(err, entities.ErrLinkAlreadyExists):
			httpx.Error(w, http.StatusConflict, "vínculo já existe entre estas entidades com este tipo")
		case errors.Is(err, entities.ErrLinkSelfReference):
			httpx.Error(w, http.StatusBadRequest, "entidade não pode ser ligada a si mesma")
		case errors.Is(err, entities.ErrLinkInvalidType):
			httpx.Error(w, http.StatusBadRequest, "relation_type inválido")
		default:
			log.Printf("link create: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao criar vínculo")
		}
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := from.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.link.add",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		After:                  toPublicLinkFromPlain(l, entities.DirectionOut),
	})

	httpx.Created(w, map[string]any{"link": toPublicLinkFromPlain(l, entities.DirectionOut)})
}

// ─────────────────────────── DELETE /api/entities/{id}/links/{lid} ─────

func (a *app) handleEntityLinkDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	lid := r.PathValue("lid")
	if id == "" || lid == "" {
		httpx.Error(w, http.StatusBadRequest, "id e lid obrigatórios")
		return
	}
	me := middleware.UserFrom(r.Context())

	// Confirma visibilidade da entidade de "ancoragem" no path. O link só
	// pode ser deletado a partir de uma das pontas que o chamador enxerga.
	e, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if e.Classification > me.ClearanceLevel || e.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

	link, err := a.entities.FindLink(r.Context(), lid)
	if err != nil {
		if errors.Is(err, entities.ErrLinkNotFound) {
			httpx.Error(w, http.StatusNotFound, "vínculo não encontrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar vínculo")
		return
	}
	if link.FromEntityID != id && link.ToEntityID != id {
		httpx.Error(w, http.StatusBadRequest, "vínculo não pertence à entidade informada")
		return
	}

	if _, err := a.entities.SoftDeleteLink(r.Context(), lid, me.ID); err != nil {
		if errors.Is(err, entities.ErrLinkNotFound) {
			httpx.Error(w, http.StatusNotFound, "vínculo não encontrado")
			return
		}
		log.Printf("link delete: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover vínculo")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := e.Classification
	dir := entities.DirectionOut
	if link.ToEntityID == id {
		dir = entities.DirectionIn
	}
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.link.remove",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		Before:                 toPublicLinkFromPlain(link, dir),
	})

	httpx.OK(w, map[string]any{"ok": true})
}
