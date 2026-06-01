package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/permissions"
)

// codename: minúsculas, dígitos e underscore; começa com letra; 2–32 chars.
var roleCodeRE = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// roleExists encapsula a checagem dinâmica de papel (substitui o antigo
// whitelist estático validRoles). Loga e devolve false em erro de banco.
func (a *app) roleExists(ctx context.Context, code string) bool {
	ok, err := a.perms.RoleExists(ctx, code)
	if err != nil {
		log.Printf("role exists: %v", err)
		return false
	}
	return ok
}

// ─────────────────────────── GET /api/roles ────────────────────────────────
// Lista os papéis para dropdowns/labels. Exige apenas autenticação — nomes de
// papel não são sensíveis e são usados em várias telas (agentes, aprovações).

func (a *app) handleRolesList(w http.ResponseWriter, r *http.Request) {
	roles, err := a.perms.ListRoles(r.Context())
	if err != nil {
		log.Printf("roles list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar papéis")
		return
	}
	if roles == nil {
		roles = []permissions.Role{}
	}
	httpx.OK(w, map[string]any{"items": roles, "total": len(roles)})
}

// ─────────────────────────── POST /api/admin/roles ─────────────────────────

type roleCreateRequest struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

func (a *app) handleRoleCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "role.create") {
		return
	}
	var req roleCreateRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	code := strings.TrimSpace(req.Code)
	label := strings.TrimSpace(req.Label)
	if !roleCodeRE.MatchString(code) {
		httpx.Error(w, http.StatusBadRequest,
			"código inválido: use minúsculas, dígitos e _ (2–32, começando com letra)")
		return
	}
	if label == "" {
		httpx.Error(w, http.StatusBadRequest, "label obrigatório")
		return
	}

	role, err := a.perms.CreateRole(r.Context(), code, label)
	if err != nil {
		if errors.Is(err, permissions.ErrRoleExists) {
			httpx.Error(w, http.StatusConflict, "já existe papel com esse código")
			return
		}
		log.Printf("role create: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao criar papel")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "role.create",
		ResourceType: audit.Ptr("role"),
		ResourceID:   audit.Ptr(code),
		After:        map[string]any{"code": code, "label": label},
	})
	httpx.OK(w, map[string]any{"role": role})
}

// ─────────────────── PATCH /api/admin/roles/{code} ─────────────────────────

type roleUpdateRequest struct {
	Label string `json:"label"`
}

func (a *app) handleRoleUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "role.update") {
		return
	}
	code := r.PathValue("code")
	var req roleUpdateRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		httpx.Error(w, http.StatusBadRequest, "label obrigatório")
		return
	}

	role, err := a.perms.UpdateRoleLabel(r.Context(), code, label)
	if err != nil {
		if errors.Is(err, permissions.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "papel não encontrado")
			return
		}
		log.Printf("role update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar papel")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "role.update",
		ResourceType: audit.Ptr("role"),
		ResourceID:   audit.Ptr(code),
		After:        map[string]any{"code": code, "label": label},
	})
	httpx.OK(w, map[string]any{"role": role})
}

// ─────────────────── DELETE /api/admin/roles/{code} ────────────────────────

func (a *app) handleRoleDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "role.delete") {
		return
	}
	code := r.PathValue("code")

	err := a.perms.DeleteRole(r.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, permissions.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "papel não encontrado")
		case errors.Is(err, permissions.ErrRoleInUse):
			httpx.Error(w, http.StatusConflict,
				"papel não pode ser excluído: é de sistema, está atribuído a algum agente ou é aprovador de uma regra 4-eyes")
		default:
			log.Printf("role delete: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao excluir papel")
		}
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "role.delete",
		ResourceType: audit.Ptr("role"),
		ResourceID:   audit.Ptr(code),
		Before:       map[string]any{"code": code},
	})
	httpx.OK(w, map[string]any{"deleted": code})
}
