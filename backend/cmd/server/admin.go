package main

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/permissions"
)

// ─────────────────────────── GET /api/admin/permissions ────────────────────

func (a *app) handleAdminPermissionsList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "admin.permissions.read") {
		return
	}
	items, err := a.perms.List(r.Context())
	if err != nil {
		log.Printf("admin permissions list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar matriz")
		return
	}
	if items == nil {
		items = []permissions.Permission{}
	}
	httpx.OK(w, map[string]any{
		"items": items,
		"total": len(items),
	})
}

// ─────────────────── PATCH /api/admin/permissions/{role_code}/{action} ──────

type updatePermissionRequest struct {
	Allowed              *bool   `json:"allowed"`
	RequiresDualApproval *bool   `json:"requires_dual_approval"`
	ApproverRole         *string `json:"approver_role"`
}

func (a *app) handleAdminPermissionsUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "admin.permissions.update") {
		return
	}

	roleCode := r.PathValue("role_code")
	action := r.PathValue("action")
	if roleCode == "" || action == "" {
		httpx.Error(w, http.StatusBadRequest, "role_code e action obrigatórios")
		return
	}
	if !validRoles[roleCode] {
		httpx.Error(w, http.StatusBadRequest, "papel inválido: "+roleCode)
		return
	}

	// Lock: linhas do administrador são protegidas. O papel não pode perder
	// permissões nem ganhar 4-eyes — qualquer ajuste tem que ser feito por
	// migration explícita após decisão de design.
	if roleCode == "administrador" {
		httpx.Error(w, http.StatusForbidden,
			"linhas do papel administrador são protegidas — alterações requerem migration")
		return
	}

	current, err := a.perms.Get(r.Context(), roleCode, action)
	if err != nil {
		if errors.Is(err, permissions.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "linha não encontrada na matriz")
			return
		}
		log.Printf("admin permissions get: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler matriz")
		return
	}

	var req updatePermissionRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	// PATCH parcial: campos ausentes mantêm valor atual.
	in := permissions.UpdateInput{
		Allowed:              current.Allowed,
		RequiresDualApproval: current.RequiresDualApproval,
		ApproverRole:         current.ApproverRole,
	}
	if req.Allowed != nil {
		in.Allowed = *req.Allowed
	}
	if req.RequiresDualApproval != nil {
		in.RequiresDualApproval = *req.RequiresDualApproval
	}
	if req.ApproverRole != nil {
		s := strings.TrimSpace(*req.ApproverRole)
		if s == "" {
			in.ApproverRole = nil
		} else {
			if !validRoles[s] {
				httpx.Error(w, http.StatusBadRequest, "approver_role inválido: "+s)
				return
			}
			in.ApproverRole = &s
		}
	}

	// Consistência: 4-eyes ligado exige approver_role; 4-eyes desligado força approver=nil.
	if in.RequiresDualApproval && (in.ApproverRole == nil || *in.ApproverRole == "") {
		httpx.Error(w, http.StatusBadRequest,
			"4-eyes ligado exige approver_role")
		return
	}
	if !in.RequiresDualApproval {
		in.ApproverRole = nil
	}

	me := middleware.UserFrom(r.Context())
	in.UpdatedBy = &me.ID

	before, after, err := a.perms.Update(r.Context(), roleCode, action, in)
	if err != nil {
		log.Printf("admin permissions update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:    aid,
		ActorSessionID: sid,
		ActorIP:        ip,
		ActorUserAgent: ua,
		Action:         "admin.permissions.update",
		ResourceType:   audit.Ptr("permission"),
		ResourceID:     audit.Ptr(roleCode + ":" + action),
		Before: map[string]any{
			"allowed":                before.Allowed,
			"requires_dual_approval": before.RequiresDualApproval,
			"approver_role":          ptrDeref(before.ApproverRole),
		},
		After: map[string]any{
			"allowed":                after.Allowed,
			"requires_dual_approval": after.RequiresDualApproval,
			"approver_role":          ptrDeref(after.ApproverRole),
		},
	})

	httpx.OK(w, map[string]any{"permission": after})
}
