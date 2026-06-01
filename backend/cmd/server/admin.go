package main

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/authz"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/permissions"
)

// matrixCell é uma célula da grade cheia (papel × ação). UpdatedAt é nil quando
// a célula nunca foi gravada (linha ausente em app.permissions) — a UI mostra
// "—" nesse caso.
type matrixCell struct {
	RoleCode             string     `json:"role_code"`
	Action               string     `json:"action"`
	Allowed              bool       `json:"allowed"`
	RequiresDualApproval bool       `json:"requires_dual_approval"`
	ApproverRole         *string    `json:"approver_role"`
	UpdatedAt            *time.Time `json:"updated_at"`
}

// ─────────────────────────── GET /api/admin/permissions ────────────────────

func (a *app) handleAdminPermissionsList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "admin.permissions.read") {
		return
	}
	ctx := r.Context()

	roles, err := a.perms.ListRoles(ctx)
	if err != nil {
		log.Printf("admin permissions list roles: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar papéis")
		return
	}
	existing, err := a.perms.List(ctx)
	if err != nil {
		log.Printf("admin permissions list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar matriz")
		return
	}

	// Indexa as linhas existentes por (papel|ação).
	byKey := make(map[string]permissions.Permission, len(existing))
	for _, p := range existing {
		byKey[p.RoleCode+"|"+p.Action] = p
	}

	// Grade cheia: produto cartesiano papéis × catálogo de ações. Células sem
	// linha viram default (negado), permitindo togglar qualquer combinação.
	items := make([]matrixCell, 0, len(roles)*len(authz.Catalog))
	for _, role := range roles {
		for _, def := range authz.Catalog {
			cell := matrixCell{RoleCode: role.Code, Action: def.Code}
			if p, ok := byKey[role.Code+"|"+def.Code]; ok {
				cell.Allowed = p.Allowed
				cell.RequiresDualApproval = p.RequiresDualApproval
				cell.ApproverRole = p.ApproverRole
				ts := p.UpdatedAt
				cell.UpdatedAt = &ts
			}
			items = append(items, cell)
		}
	}

	httpx.OK(w, map[string]any{
		"roles":   roles,
		"actions": authz.Catalog,
		"items":   items,
		"total":   len(items),
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
	if !authz.IsValidAction(action) {
		httpx.Error(w, http.StatusBadRequest, "ação inválida: "+action)
		return
	}

	// Estado atual da célula. A grade é renderizada cheia (papéis × catálogo),
	// então a célula editada pode não ter linha ainda — tratamos como default
	// (negado) para suportar PATCH parcial sobre células sintéticas.
	current := &permissions.Permission{RoleCode: roleCode, Action: action}
	if got, err := a.perms.Get(r.Context(), roleCode, action); err == nil {
		current = got
	} else if !errors.Is(err, permissions.ErrNotFound) {
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

	// Guarda anti-lockout: ações de governança (admin.permissions.*) não podem
	// perder sua última via de administração. Se a mudança proposta deixaria
	// zero usuários ativos capazes de executá-la sem 4-eyes, recusamos — assim
	// ninguém se tranca pra fora do RBAC pela própria matriz.
	if def, _ := authz.LookupAction(action); def.Governance {
		reachable, err := a.perms.GovernanceReachableAfter(
			r.Context(), action, roleCode, in.Allowed, in.RequiresDualApproval)
		if err != nil {
			log.Printf("admin permissions lockout check: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao validar mudança")
			return
		}
		if !reachable {
			httpx.Error(w, http.StatusConflict,
				"bloqueado: deixaria o sistema sem nenhum usuário ativo capaz de '"+action+"' sem aprovação dupla")
			return
		}
	}

	me := middleware.UserFrom(r.Context())
	in.UpdatedBy = &me.ID

	before, after, err := a.perms.Upsert(r.Context(), roleCode, action, in)
	if err != nil {
		log.Printf("admin permissions update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}

	// before é nil quando a célula não existia (foi criada agora).
	beforeEntry := map[string]any{"allowed": false, "requires_dual_approval": false, "approver_role": ""}
	if before != nil {
		beforeEntry = map[string]any{
			"allowed":                before.Allowed,
			"requires_dual_approval": before.RequiresDualApproval,
			"approver_role":          ptrDeref(before.ApproverRole),
		}
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
		Before:         beforeEntry,
		After: map[string]any{
			"allowed":                after.Allowed,
			"requires_dual_approval": after.RequiresDualApproval,
			"approver_role":          ptrDeref(after.ApproverRole),
		},
	})

	httpx.OK(w, map[string]any{"permission": after})
}
