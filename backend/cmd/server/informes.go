package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/informes"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/users"
)

// publicInforme é a forma JSON de um informe (com autor resolvido).
type publicInforme struct {
	ID                string    `json:"id"`
	OccurredOn        string    `json:"occurred_on"` // YYYY-MM-DD
	Location          string    `json:"location"`
	How               string    `json:"how"`
	Description       string    `json:"description"`
	HasPhoto          bool      `json:"has_photo"`
	RequiredClearance int       `json:"required_clearance"`
	Version           int       `json:"version"`
	CreatedAt         time.Time `json:"created_at"`
	CreatedBy         string    `json:"created_by"`
	CreatedByCode     string    `json:"created_by_code"`
	CreatedByName     string    `json:"created_by_name"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func toPublicInforme(i *informes.Informe) publicInforme {
	return publicInforme{
		ID:                i.ID,
		OccurredOn:        i.OccurredOn.Format("2006-01-02"),
		Location:          i.Location,
		How:               i.How,
		Description:       i.Description,
		HasPhoto:          i.PhotoPath != nil && *i.PhotoPath != "",
		RequiredClearance: i.RequiredClearance,
		Version:           i.Version,
		CreatedAt:         i.CreatedAt,
		CreatedBy:         i.CreatedBy,
		CreatedByCode:     i.CreatedByCode,
		CreatedByName:     i.CreatedByName,
		UpdatedAt:         i.UpdatedAt,
	}
}

// canManageInforme: pode editar/excluir se for o autor OU gestor/administrador.
func canManageInforme(me *users.User, inf *informes.Informe) bool {
	if me == nil {
		return false
	}
	return me.ID == inf.CreatedBy ||
		hasRole(me.Roles, "gestor") || hasRole(me.Roles, "administrador")
}

// ─────────────────────────── GET /api/informes ─────────────────────────────

func (a *app) handleInformesList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "informe.read") {
		return
	}
	me := middleware.UserFrom(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	res, err := a.informes.List(r.Context(), informes.ListOpts{
		Limit:     limit,
		Offset:    offset,
		Search:    strings.TrimSpace(q.Get("search")),
		SortBy:    strings.TrimSpace(q.Get("sort_by")),
		SortDir:   strings.TrimSpace(q.Get("sort_dir")),
		UserID:    me.ID,
		Clearance: me.ClearanceLevel,
		IsAdmin:   hasRole(me.Roles, "administrador"),
	})
	if err != nil {
		log.Printf("informes list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	items := make([]publicInforme, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, toPublicInforme(&res.Items[i]))
	}
	httpx.OK(w, map[string]any{
		"items": items, "total": res.Total, "limit": limit, "offset": offset,
	})
}

// ─────────────────────────── POST /api/informes ────────────────────────────

type createInformeRequest struct {
	OccurredOn        string `json:"occurred_on"`
	Location          string `json:"location"`
	How               string `json:"how"`
	Description       string `json:"description"`
	RequiredClearance int    `json:"required_clearance"`
}

func (a *app) handleInformeCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "informe.create") {
		return
	}
	me := middleware.UserFrom(r.Context())
	var req createInformeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	occurred := time.Now()
	if s := strings.TrimSpace(req.OccurredOn); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "occurred_on inválido (esperado YYYY-MM-DD)")
			return
		}
		occurred = t
	}
	if req.RequiredClearance != 0 && (req.RequiredClearance < 1 || req.RequiredClearance > 5) {
		httpx.Error(w, http.StatusBadRequest, "required_clearance inválido (1..5)")
		return
	}
	inf, err := a.informes.Create(r.Context(), informes.NewInforme{
		OccurredOn:        occurred,
		Location:          req.Location,
		How:               req.How,
		Description:       req.Description,
		RequiredClearance: req.RequiredClearance,
		CreatedBy:         me.ID,
	})
	if err != nil {
		log.Printf("informe create: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao criar")
		return
	}
	a.auditInforme(r, "informe.create", inf.ID, inf.RequiredClearance, nil, map[string]any{
		"occurred_on": inf.OccurredOn.Format("2006-01-02"), "location": inf.Location,
	})
	httpx.OK(w, map[string]any{"informe": toPublicInforme(inf)})
}

// ─────────────────────────── GET /api/informes/{id} ────────────────────────

func (a *app) handleInformeDetail(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "informe.read") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	ok, err := a.informes.CanAccess(r.Context(), id, me.ID, me.ClearanceLevel, hasRole(me.Roles, "administrador"))
	if err != nil {
		if errors.Is(err, informes.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "informe não encontrado")
			return
		}
		log.Printf("informe access: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if !ok {
		httpx.Error(w, http.StatusNotFound, "informe não encontrado")
		return
	}
	inf, err := a.informes.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "informe não encontrado")
		return
	}
	httpx.OK(w, map[string]any{"informe": toPublicInforme(inf)})
}

// ─────────────────────────── PATCH /api/informes/{id} ──────────────────────

type updateInformeRequest struct {
	OccurredOn        *string `json:"occurred_on"`
	Location          *string `json:"location"`
	How               *string `json:"how"`
	Description       *string `json:"description"`
	RequiredClearance *int    `json:"required_clearance"`
}

func (a *app) handleInformeUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "informe.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	cur, err := a.informes.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "informe não encontrado")
		return
	}
	if !canManageInforme(me, cur) {
		httpx.Error(w, http.StatusForbidden, "só o autor (ou gestor/admin) pode editar este informe")
		return
	}
	var req updateInformeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	var p informes.Patch
	if req.OccurredOn != nil {
		t, err := time.Parse("2006-01-02", strings.TrimSpace(*req.OccurredOn))
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "occurred_on inválido (esperado YYYY-MM-DD)")
			return
		}
		p.OccurredOn = &t
	}
	p.Location = req.Location
	p.How = req.How
	p.Description = req.Description
	if req.RequiredClearance != nil {
		if *req.RequiredClearance < 1 || *req.RequiredClearance > 5 {
			httpx.Error(w, http.StatusBadRequest, "required_clearance inválido (1..5)")
			return
		}
		p.RequiredClearance = req.RequiredClearance
	}
	inf, err := a.informes.Update(r.Context(), id, p, me.ID)
	if err != nil {
		if errors.Is(err, informes.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "informe não encontrado")
			return
		}
		log.Printf("informe update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}
	a.auditInforme(r, "informe.update", id, inf.RequiredClearance, nil, map[string]any{
		"occurred_on": inf.OccurredOn.Format("2006-01-02"), "location": inf.Location,
		"required_clearance": inf.RequiredClearance,
	})
	httpx.OK(w, map[string]any{"informe": toPublicInforme(inf)})
}

// ─────────────────────────── DELETE /api/informes/{id} ─────────────────────

func (a *app) handleInformeDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "informe.delete") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	cur, err := a.informes.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "informe não encontrado")
		return
	}
	if !canManageInforme(me, cur) {
		httpx.Error(w, http.StatusForbidden, "só o autor (ou gestor/admin) pode excluir este informe")
		return
	}
	if err := a.informes.SoftDelete(r.Context(), id, me.ID); err != nil {
		if errors.Is(err, informes.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "informe não encontrado")
			return
		}
		log.Printf("informe delete: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao excluir")
		return
	}
	a.auditInforme(r, "informe.delete", id, cur.RequiredClearance,
		map[string]any{"location": cur.Location}, nil)
	httpx.NoContent(w)
}

// auditInforme centraliza o log de auditoria das ações de informe.
func (a *app) auditInforme(r *http.Request, action, id string, classification int, before, after map[string]any) {
	aid, sid, ip, ua := a.actorInfo(r)
	class := classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:                 action,
		ResourceType:           audit.Ptr("informe"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &class,
		Before:                 before,
		After:                  after,
	})
}
