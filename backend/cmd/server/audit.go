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
)

// ─────────────────────────── GET /api/audit ────────────────────────────

func (a *app) handleAuditList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "audit.read") {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	opts := audit.ListOpts{
		Limit:        limit,
		Offset:       offset,
		Action:       strings.TrimSpace(q.Get("action")),
		ActorID:      strings.TrimSpace(q.Get("actor_id")),
		ResourceType: strings.TrimSpace(q.Get("resource_type")),
		ResourceID:   strings.TrimSpace(q.Get("resource_id")),
		Search:       strings.TrimSpace(q.Get("search")),
		SortBy:       strings.TrimSpace(q.Get("sort_by")),
		SortDir:      strings.TrimSpace(q.Get("sort_dir")),
	}
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		if t, err := parseFlexTime(v); err == nil {
			opts.From = &t
		}
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		if t, err := parseFlexTime(v); err == nil {
			opts.To = &t
		}
	}

	res, err := a.auditReader.List(r.Context(), opts)
	if err != nil {
		log.Printf("audit list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar trilho de auditoria")
		return
	}

	// Importante: leituras do audit não geram nova entrada no audit
	// (evitamos loop). A intenção/autoria fica registrada via logs HTTP
	// e pelas decisões do PDP que negaram a entrada quando não autorizada.

	httpx.OK(w, map[string]any{
		"items":  res.Items,
		"total":  res.Total,
		"limit":  cmpDefault(limit, 25),
		"offset": offset,
	})
}

// ─────────────────────────── GET /api/audit/{id} ────────────────────────

func (a *app) handleAuditDetail(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "audit.read") {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		httpx.Error(w, http.StatusBadRequest, "id inválido")
		return
	}
	e, err := a.auditReader.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, audit.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entrada não encontrada")
			return
		}
		log.Printf("audit find: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar entrada")
		return
	}
	httpx.OK(w, map[string]any{"entry": e})
}

// parseFlexTime aceita RFC3339 ("2026-05-11T00:00:00Z") ou date-only ("2026-05-11").
func parseFlexTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}
