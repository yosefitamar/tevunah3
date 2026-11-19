package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/consultas"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/users"
)

// publicConsulta é a forma JSON de um registro de consulta. performed_on e
// performed_time são a data e a hora no fuso da agência — é o que o formulário
// edita; performed_at é o mesmo instante em ISO 8601.
type publicConsulta struct {
	ID            string    `json:"id"`
	PerformedAt   time.Time `json:"performed_at"`
	PerformedOn   string    `json:"performed_on"`   // YYYY-MM-DD
	PerformedTime string    `json:"performed_time"` // HH:MM
	System        string    `json:"system"`
	Target        string    `json:"target"`
	Requester     string    `json:"requester"`
	RequesterOrg  string    `json:"requester_org"`
	ReferenceDoc  string    `json:"reference_doc"`
	Reason        string    `json:"reason"`
	Result        string    `json:"result"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	CreatedBy     string    `json:"created_by"`
	CreatedByCode string    `json:"created_by_code"`
	CreatedByName string    `json:"created_by_name"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// agencyLocation devolve o fuso da agência (UTC se o nome for inválido).
func (a *app) agencyLocation() *time.Location {
	loc, err := time.LoadLocation(a.tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (a *app) toPublicConsulta(c *consultas.Consulta) publicConsulta {
	local := c.PerformedAt.In(a.agencyLocation())
	return publicConsulta{
		ID:            c.ID,
		PerformedAt:   c.PerformedAt,
		PerformedOn:   local.Format("2006-01-02"),
		PerformedTime: local.Format("15:04"),
		System:        c.System,
		Target:        c.Target,
		Requester:     c.Requester,
		RequesterOrg:  c.RequesterOrg,
		ReferenceDoc:  c.ReferenceDoc,
		Reason:        c.Reason,
		Result:        c.Result,
		Version:       c.Version,
		CreatedAt:     c.CreatedAt,
		CreatedBy:     c.CreatedBy,
		CreatedByCode: c.CreatedByCode,
		CreatedByName: c.CreatedByName,
		UpdatedAt:     c.UpdatedAt,
	}
}

// canManageConsulta: pode editar/excluir se for o autor OU gestor/administrador.
func canManageConsulta(me *users.User, c *consultas.Consulta) bool {
	if me == nil {
		return false
	}
	return me.ID == c.CreatedBy ||
		hasRole(me.Roles, "gestor") || hasRole(me.Roles, "administrador")
}

// consultaRequest é o corpo de criação e de edição: o registro é sempre
// gravado inteiro.
type consultaRequest struct {
	PerformedOn   string `json:"performed_on"`   // YYYY-MM-DD, no fuso da agência
	PerformedTime string `json:"performed_time"` // HH:MM
	System        string `json:"system"`
	Target        string `json:"target"`
	Requester     string `json:"requester"`
	RequesterOrg  string `json:"requester_org"`
	ReferenceDoc  string `json:"reference_doc"`
	Reason        string `json:"reason"`
	Result        string `json:"result"`
}

// fields valida o corpo e o converte para o repositório. A mensagem devolvida
// (quando não vazia) vai direto ao usuário num 400.
func (a *app) consultaFields(req consultaRequest) (consultas.Fields, string) {
	var f consultas.Fields
	when, err := time.ParseInLocation("2006-01-02 15:04",
		strings.TrimSpace(req.PerformedOn)+" "+strings.TrimSpace(req.PerformedTime), a.agencyLocation())
	if err != nil {
		return f, "informe a data (AAAA-MM-DD) e a hora (HH:MM) da consulta"
	}
	// Tolerância para relógio do terminal adiantado em relação ao servidor.
	if when.After(time.Now().Add(5 * time.Minute)) {
		return f, "a data e a hora da consulta não podem estar no futuro"
	}
	type check struct {
		label    string
		val      string
		required bool
		max      int
	}
	for _, c := range []check{
		{"o sistema consultado", req.System, true, 120},
		{"o alvo da pesquisa", req.Target, true, 500},
		{"o solicitante", req.Requester, true, 200},
		{"o órgão do solicitante", req.RequesterOrg, false, 200},
		{"o documento de referência", req.ReferenceDoc, false, 200},
		{"o motivo", req.Reason, true, 4000},
		{"o resultado", req.Result, true, 8000},
	} {
		v := strings.TrimSpace(c.val)
		if c.required && v == "" {
			return f, "informe " + c.label
		}
		if utf8.RuneCountInString(v) > c.max {
			return f, c.label + " passa de " + strconv.Itoa(c.max) + " caracteres"
		}
	}
	return consultas.Fields{
		PerformedAt:  when,
		System:       req.System,
		Target:       req.Target,
		Requester:    req.Requester,
		RequesterOrg: req.RequesterOrg,
		ReferenceDoc: req.ReferenceDoc,
		Reason:       req.Reason,
		Result:       req.Result,
	}, ""
}

// consultaAuditMap é o retrato do registro gravado na trilha de auditoria.
func (a *app) consultaAuditMap(c *consultas.Consulta) map[string]any {
	return map[string]any{
		"performed_at":  c.PerformedAt.In(a.agencyLocation()).Format("2006-01-02 15:04"),
		"system":        c.System,
		"target":        c.Target,
		"requester":     c.Requester,
		"requester_org": c.RequesterOrg,
		"reference_doc": c.ReferenceDoc,
		"reason":        c.Reason,
		"result":        c.Result,
		"created_by":    c.CreatedByCode,
	}
}

// ─────────────────────────── GET /api/consultas ────────────────────────────

func (a *app) handleConsultasList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "consulta.read") {
		return
	}
	me := middleware.UserFrom(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	from := strings.TrimSpace(q.Get("date_from"))
	to := strings.TrimSpace(q.Get("date_to"))
	for _, d := range []string{from, to} {
		if d == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d); err != nil {
			httpx.Error(w, http.StatusBadRequest, "período inválido (esperado YYYY-MM-DD)")
			return
		}
	}
	res, err := a.consultas.List(r.Context(), consultas.ListOpts{
		Limit:    limit,
		Offset:   offset,
		Search:   q.Get("search"),
		DateFrom: from,
		DateTo:   to,
		TZ:       a.tz,
		SortBy:   strings.TrimSpace(q.Get("sort_by")),
		SortDir:  strings.TrimSpace(q.Get("sort_dir")),
		UserID:   me.ID,
		SeeAll:   a.hasPerm(r, "consulta.read_all"),
	})
	if err != nil {
		log.Printf("consultas list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	items := make([]publicConsulta, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, a.toPublicConsulta(&res.Items[i]))
	}
	httpx.OK(w, map[string]any{
		"items": items, "total": res.Total, "limit": limit, "offset": offset,
	})
}

// ─────────────────────────── GET /api/consultas/sistemas ───────────────────

func (a *app) handleConsultasSystems(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "consulta.read") {
		return
	}
	systems, err := a.consultas.Systems(r.Context())
	if err != nil {
		log.Printf("consultas systems: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar sistemas")
		return
	}
	httpx.OK(w, map[string]any{"items": systems})
}

// ─────────────────────────── POST /api/consultas ───────────────────────────

func (a *app) handleConsultaCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "consulta.create") {
		return
	}
	me := middleware.UserFrom(r.Context())
	var req consultaRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	fields, msg := a.consultaFields(req)
	if msg != "" {
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	c, err := a.consultas.Create(r.Context(), fields, me.ID)
	if err != nil {
		log.Printf("consulta create: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao registrar")
		return
	}
	a.auditConsulta(r, "consulta.create", c.ID, nil, a.consultaAuditMap(c))
	httpx.OK(w, map[string]any{"consulta": a.toPublicConsulta(c)})
}

// findVisibleConsulta carrega o registro se o solicitante puder vê-lo (autor
// ou consulta.read_all). Quando não pode, responde 404 — sem revelar que o
// registro existe — e devolve nil.
func (a *app) findVisibleConsulta(w http.ResponseWriter, r *http.Request) *consultas.Consulta {
	me := middleware.UserFrom(r.Context())
	c, err := a.consultas.FindByID(r.Context(), r.PathValue("id"))
	if err != nil {
		if !errors.Is(err, consultas.ErrNotFound) {
			log.Printf("consulta find: %v", err)
		}
		httpx.Error(w, http.StatusNotFound, "registro de consulta não encontrado")
		return nil
	}
	if c.CreatedBy != me.ID && !a.hasPerm(r, "consulta.read_all") {
		httpx.Error(w, http.StatusNotFound, "registro de consulta não encontrado")
		return nil
	}
	return c
}

// ─────────────────────────── GET /api/consultas/{id} ───────────────────────

func (a *app) handleConsultaDetail(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "consulta.read") {
		return
	}
	c := a.findVisibleConsulta(w, r)
	if c == nil {
		return
	}
	httpx.OK(w, map[string]any{"consulta": a.toPublicConsulta(c)})
}

// ─────────────────────────── PUT /api/consultas/{id} ───────────────────────

func (a *app) handleConsultaUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "consulta.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	cur := a.findVisibleConsulta(w, r)
	if cur == nil {
		return
	}
	if !canManageConsulta(me, cur) {
		httpx.Error(w, http.StatusForbidden, "só o autor (ou gestor/admin) pode editar este registro")
		return
	}
	var req consultaRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	fields, msg := a.consultaFields(req)
	if msg != "" {
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	c, err := a.consultas.Update(r.Context(), cur.ID, fields, me.ID)
	if err != nil {
		if errors.Is(err, consultas.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "registro de consulta não encontrado")
			return
		}
		log.Printf("consulta update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}
	// Só o que mudou entra na trilha, com o valor de antes e o de depois.
	before, after := map[string]any{}, map[string]any{}
	was, now := a.consultaAuditMap(cur), a.consultaAuditMap(c)
	for k, v := range now {
		if was[k] != v {
			before[k], after[k] = was[k], v
		}
	}
	if len(after) > 0 {
		a.auditConsulta(r, "consulta.update", c.ID, before, after)
	}
	httpx.OK(w, map[string]any{"consulta": a.toPublicConsulta(c)})
}

// ─────────────────────────── DELETE /api/consultas/{id} ────────────────────

func (a *app) handleConsultaDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "consulta.delete") {
		return
	}
	me := middleware.UserFrom(r.Context())
	cur := a.findVisibleConsulta(w, r)
	if cur == nil {
		return
	}
	if !canManageConsulta(me, cur) {
		httpx.Error(w, http.StatusForbidden, "só o autor (ou gestor/admin) pode excluir este registro")
		return
	}
	if err := a.consultas.SoftDelete(r.Context(), cur.ID, me.ID); err != nil {
		if errors.Is(err, consultas.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "registro de consulta não encontrado")
			return
		}
		log.Printf("consulta delete: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao excluir")
		return
	}
	a.auditConsulta(r, "consulta.delete", cur.ID, a.consultaAuditMap(cur), nil)
	httpx.NoContent(w)
}

// auditConsulta centraliza o log de auditoria das ações do registro de consultas.
func (a *app) auditConsulta(r *http.Request, action, id string, before, after map[string]any) {
	aid, sid, ip, ua := a.actorInfo(r)
	entry := audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       action,
		ResourceType: audit.Ptr("consulta"),
		ResourceID:   audit.Ptr(id),
	}
	// Mapa nil vira interface não-nil e seria gravado como "null" em vez de NULL.
	if before != nil {
		entry.Before = before
	}
	if after != nil {
		entry.After = after
	}
	if err := a.audit.Log(r.Context(), entry); err != nil {
		log.Printf("audit %s: %v", action, err)
	}
}
