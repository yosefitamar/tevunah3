package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/belia/tevunah/backend/internal/approvals"
	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/authz"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/users"
)

// ─────────── Decision helper (dual-aware) ───────────

// decide consulta a política e devolve a decisão. Em caso de erro/negado já
// escreve a resposta. Diferente de requirePerm, NÃO trata RequiresDualApproval
// como erro — handlers dual-aware decidem se executam direto ou criam pending.
func (a *app) decide(w http.ResponseWriter, r *http.Request, action string) (authz.Decision, bool) {
	me := middleware.UserFrom(r.Context())
	d, err := a.policy.Can(r.Context(), me.Roles, action)
	if err != nil {
		log.Printf("policy: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro de autorização")
		return d, false
	}
	if !d.Allowed {
		httpx.Error(w, http.StatusForbidden, "ação não autorizada: "+action)
		return d, false
	}
	return d, true
}

// ─────────── Executor registry ───────────

type executor func(ctx context.Context, a *app, ap *approvals.Approval) error

var executors = map[string]executor{
	"user.role.assign":   execUserRoleAssign,
	"user.clearance.set": execUserClearanceSet,
}

func execUserRoleAssign(ctx context.Context, a *app, ap *approvals.Approval) error {
	if ap.ResourceID == nil {
		return errors.New("resource_id ausente")
	}
	var p struct {
		Roles []string `json:"roles"`
	}
	if err := json.Unmarshal(ap.Payload, &p); err != nil {
		return err
	}
	before, err := a.users.FindByID(ctx, *ap.ResourceID)
	if err != nil {
		return err
	}
	if err := a.users.SetRoles(ctx, *ap.ResourceID, p.Roles, ap.DecidedBy); err != nil {
		return err
	}
	after, err := a.users.FindByID(ctx, *ap.ResourceID)
	if err != nil {
		return err
	}
	_ = a.audit.Log(ctx, audit.Entry{
		ActorUserID:  ap.DecidedBy,
		Action:       "user.role.assign",
		ResourceType: audit.Ptr("user"),
		ResourceID:   ap.ResourceID,
		Before:       map[string]any{"roles": before.Roles},
		After: map[string]any{
			"roles":        after.Roles,
			"approval_id":  ap.ID,
			"requested_by": ap.RequestedBy,
		},
	})
	return nil
}

func execUserClearanceSet(ctx context.Context, a *app, ap *approvals.Approval) error {
	if ap.ResourceID == nil {
		return errors.New("resource_id ausente")
	}
	var p struct {
		ClearanceLevel int `json:"clearance_level"`
	}
	if err := json.Unmarshal(ap.Payload, &p); err != nil {
		return err
	}
	before, err := a.users.FindByID(ctx, *ap.ResourceID)
	if err != nil {
		return err
	}
	if err := a.users.SetClearance(ctx, *ap.ResourceID, p.ClearanceLevel); err != nil {
		return err
	}
	_ = a.audit.Log(ctx, audit.Entry{
		ActorUserID:  ap.DecidedBy,
		Action:       "user.clearance.set",
		ResourceType: audit.Ptr("user"),
		ResourceID:   ap.ResourceID,
		Before:       map[string]any{"clearance_level": before.ClearanceLevel},
		After: map[string]any{
			"clearance_level": p.ClearanceLevel,
			"approval_id":     ap.ID,
			"requested_by":    ap.RequestedBy,
		},
	})
	return nil
}

// ─────────── Request handlers (criam pending ou executam direto) ───────────

type setRolesRequest struct {
	Roles []string `json:"roles"`
}

func (a *app) handleUserSetRoles(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	targetID := r.PathValue("id")
	if targetID == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	if targetID == me.ID {
		httpx.Error(w, http.StatusForbidden,
			"não é possível solicitar alteração de papéis de si mesmo")
		return
	}

	d, ok := a.decide(w, r, "user.role.assign")
	if !ok {
		return
	}

	var req setRolesRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if len(req.Roles) == 0 {
		httpx.Error(w, http.StatusBadRequest, "ao menos um papel é obrigatório")
		return
	}
	for _, role := range req.Roles {
		if !validRoles[role] {
			httpx.Error(w, http.StatusBadRequest, "papel inválido: "+role)
			return
		}
	}

	// Valida alvo
	if _, err := a.users.FindByID(r.Context(), targetID); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "alvo não encontrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar alvo")
		return
	}

	payload, _ := json.Marshal(map[string]any{"roles": req.Roles})

	if d.RequiresDualApproval {
		ap, err := a.approvals.Create(r.Context(), approvals.CreateInput{
			Action:               "user.role.assign",
			RequestedBy:          me.ID,
			RequiredApproverRole: d.ApproverRole,
			ResourceType:         strPtr("user"),
			ResourceID:           &targetID,
			Payload:              payload,
		})
		if err != nil {
			log.Printf("create approval: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao criar solicitação")
			return
		}
		a.logRequested(r, me.ID, ap, map[string]any{"roles": req.Roles})
		httpx.WriteJSON(w, http.StatusAccepted, httpx.Envelope{
			Success: true,
			Data: map[string]any{
				"approval": toPublicApproval(ap),
				"note":     "aguardando aprovação de " + d.ApproverRole,
			},
		})
		return
	}

	// Execução direta (caso a matriz seja alterada para sem 4-eyes)
	if err := a.users.SetRoles(r.Context(), targetID, req.Roles, &me.ID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao atribuir papéis")
		return
	}
	after, _ := a.users.FindByID(r.Context(), targetID)
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "user.role.assign",
		ResourceType: audit.Ptr("user"), ResourceID: &targetID,
		After: map[string]any{"roles": req.Roles},
	})
	httpx.OK(w, map[string]any{"user": toPublic(after)})
}

type setClearanceRequest struct {
	ClearanceLevel int `json:"clearance_level"`
}

func (a *app) handleUserSetClearance(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	targetID := r.PathValue("id")
	if targetID == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	if targetID == me.ID {
		httpx.Error(w, http.StatusForbidden,
			"não é possível solicitar alteração do próprio clearance")
		return
	}

	d, ok := a.decide(w, r, "user.clearance.set")
	if !ok {
		return
	}

	var req setClearanceRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if req.ClearanceLevel < 1 || req.ClearanceLevel > 5 {
		httpx.Error(w, http.StatusBadRequest, "clearance_level deve estar entre 1 e 5")
		return
	}

	if _, err := a.users.FindByID(r.Context(), targetID); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "alvo não encontrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar alvo")
		return
	}

	payload, _ := json.Marshal(map[string]any{"clearance_level": req.ClearanceLevel})

	if d.RequiresDualApproval {
		ap, err := a.approvals.Create(r.Context(), approvals.CreateInput{
			Action:               "user.clearance.set",
			RequestedBy:          me.ID,
			RequiredApproverRole: d.ApproverRole,
			ResourceType:         strPtr("user"),
			ResourceID:           &targetID,
			Payload:              payload,
		})
		if err != nil {
			log.Printf("create approval: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao criar solicitação")
			return
		}
		a.logRequested(r, me.ID, ap, map[string]any{"clearance_level": req.ClearanceLevel})
		httpx.WriteJSON(w, http.StatusAccepted, httpx.Envelope{
			Success: true,
			Data: map[string]any{
				"approval": toPublicApproval(ap),
				"note":     "aguardando aprovação de " + d.ApproverRole,
			},
		})
		return
	}

	if err := a.users.SetClearance(r.Context(), targetID, req.ClearanceLevel); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao alterar clearance")
		return
	}
	after, _ := a.users.FindByID(r.Context(), targetID)
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "user.clearance.set",
		ResourceType: audit.Ptr("user"), ResourceID: &targetID,
		After: map[string]any{"clearance_level": req.ClearanceLevel},
	})
	httpx.OK(w, map[string]any{"user": toPublic(after)})
}

// ─────────── Approvals endpoints ───────────

func (a *app) handleApprovalsList(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	q := r.URL.Query()

	mode := q.Get("mode") // "" | "pending_for_me" | "mine"
	opts := approvals.ListOpts{
		Status:  approvals.Status(q.Get("status")),
		SortBy:  strings.TrimSpace(q.Get("sort_by")),
		SortDir: strings.TrimSpace(q.Get("sort_dir")),
	}

	canSeeAll := hasRole(me.Roles, "administrador") || hasRole(me.Roles, "gestor")

	switch mode {
	case "mine":
		opts.RequestedBy = me.ID
	case "pending_for_me":
		opts.ApproverRoles = me.Roles
		opts.ExcludeUserID = me.ID
		if opts.Status == "" {
			opts.Status = approvals.StatusPending
		}
	default:
		// Sem mode: admin/gestor vêem tudo; demais só as próprias.
		if !canSeeAll {
			opts.RequestedBy = me.ID
		}
	}

	res, err := a.approvals.List(r.Context(), opts)
	if err != nil {
		log.Printf("approvals list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar aprovações")
		return
	}
	items := make([]publicApproval, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, toPublicApproval(&res.Items[i]))
	}
	httpx.OK(w, map[string]any{"items": items, "total": res.Total})
}

func (a *app) handleApprovalDetail(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	ap, err := a.approvals.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, approvals.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "aprovação não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if !canSeeApproval(me, ap) {
		httpx.Error(w, http.StatusForbidden, "ação não autorizada")
		return
	}
	httpx.OK(w, map[string]any{"approval": toPublicApproval(ap)})
}

type approvalDecisionRequest struct {
	Reason string `json:"reason"`
}

func (a *app) handleApprovalApprove(w http.ResponseWriter, r *http.Request) {
	a.handleApprovalDecision(w, r, approvals.StatusApproved)
}

func (a *app) handleApprovalReject(w http.ResponseWriter, r *http.Request) {
	a.handleApprovalDecision(w, r, approvals.StatusRejected)
}

func (a *app) handleApprovalDecision(w http.ResponseWriter, r *http.Request, decision approvals.Status) {
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")

	var req approvalDecisionRequest
	_ = httpx.Decode(r, &req)

	ap, err := a.approvals.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, approvals.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "aprovação não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}

	if ap.Status != approvals.StatusPending {
		httpx.Error(w, http.StatusConflict, "aprovação não está pendente")
		return
	}
	if ap.RequestedBy == me.ID {
		httpx.Error(w, http.StatusForbidden, "solicitante não pode decidir a própria aprovação")
		return
	}
	if !hasRole(me.Roles, ap.RequiredApproverRole) {
		httpx.Error(w, http.StatusForbidden,
			"requer papel "+ap.RequiredApproverRole+" para decidir")
		return
	}

	decided, err := a.approvals.Decide(r.Context(), id, me.ID, decision, req.Reason)
	if err != nil {
		if errors.Is(err, approvals.ErrInvalidStatus) {
			httpx.Error(w, http.StatusConflict, "aprovação não está pendente")
			return
		}
		if errors.Is(err, approvals.ErrExpired) {
			httpx.Error(w, http.StatusConflict, "aprovação expirada")
			return
		}
		if errors.Is(err, approvals.ErrSelfApproval) {
			httpx.Error(w, http.StatusForbidden, "solicitante não pode decidir")
			return
		}
		log.Printf("approval decide: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao decidir")
		return
	}

	// Audit do ato de decisão
	aid, sid, ip, ua := a.actorInfo(r)
	auditAction := decided.Action + ".approved"
	if decision == approvals.StatusRejected {
		auditAction = decided.Action + ".rejected"
	}
	var reasonPtr *string
	if s := strings.TrimSpace(req.Reason); s != "" {
		reasonPtr = &s
	}
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:    aid,
		ActorSessionID: sid,
		ActorIP:        ip,
		ActorUserAgent: ua,
		Action:         auditAction,
		ResourceType:   audit.Ptr("approval"),
		ResourceID:     &decided.ID,
		After: map[string]any{
			"action":       decided.Action,
			"resource_id":  ptrDeref(decided.ResourceID),
			"requested_by": decided.RequestedBy,
			"decision":     string(decision),
		},
		Reason: reasonPtr,
	})

	// Execução se aprovado
	if decision == approvals.StatusApproved {
		exec, ok := executors[decided.Action]
		if !ok {
			log.Printf("⚠ aprovação %s aprovada mas nenhum executor cadastrado para %s",
				decided.ID, decided.Action)
		} else if err := exec(r.Context(), a, decided); err != nil {
			log.Printf("executor %s falhou (approval %s): %v",
				decided.Action, decided.ID, err)
			httpx.Error(w, http.StatusInternalServerError,
				"aprovação registrada mas execução falhou: "+err.Error())
			return
		}
	}

	httpx.OK(w, map[string]any{"approval": toPublicApproval(decided)})
}

func (a *app) handleApprovalCancel(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")

	var req approvalDecisionRequest
	_ = httpx.Decode(r, &req)

	cancelled, err := a.approvals.Cancel(r.Context(), id, me.ID, req.Reason)
	if err != nil {
		if errors.Is(err, approvals.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "aprovação não encontrada")
			return
		}
		if errors.Is(err, approvals.ErrInvalidStatus) {
			httpx.Error(w, http.StatusConflict, "aprovação não está pendente")
			return
		}
		if errors.Is(err, approvals.ErrNotRequester) {
			httpx.Error(w, http.StatusForbidden, "apenas o solicitante pode cancelar")
			return
		}
		log.Printf("approval cancel: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao cancelar")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	var reasonPtr *string
	if s := strings.TrimSpace(req.Reason); s != "" {
		reasonPtr = &s
	}
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       cancelled.Action + ".cancelled",
		ResourceType: audit.Ptr("approval"),
		ResourceID:   &cancelled.ID,
		Reason:       reasonPtr,
	})

	httpx.OK(w, map[string]any{"approval": toPublicApproval(cancelled)})
}

// ─────────── helpers ───────────

func (a *app) logRequested(r *http.Request, requesterID string, ap *approvals.Approval, payload map[string]any) {
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       ap.Action + ".requested",
		ResourceType: audit.Ptr("approval"),
		ResourceID:   &ap.ID,
		After: map[string]any{
			"target_user_id":         ptrDeref(ap.ResourceID),
			"payload":                payload,
			"required_approver_role": ap.RequiredApproverRole,
		},
	})
}

func canSeeApproval(me *users.User, ap *approvals.Approval) bool {
	if hasRole(me.Roles, "administrador") || hasRole(me.Roles, "gestor") {
		return true
	}
	if ap.RequestedBy == me.ID {
		return true
	}
	if hasRole(me.Roles, ap.RequiredApproverRole) {
		return true
	}
	return false
}

func hasRole(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}

type publicApproval struct {
	ID                   string          `json:"id"`
	Action               string          `json:"action"`
	ResourceType         *string         `json:"resource_type,omitempty"`
	ResourceID           *string         `json:"resource_id,omitempty"`
	Payload              json.RawMessage `json:"payload"`
	RequestedBy          string          `json:"requested_by"`
	RequestedAt          string          `json:"requested_at"`
	RequiredApproverRole string          `json:"required_approver_role"`
	Status               string          `json:"status"`
	DecidedBy            *string         `json:"decided_by,omitempty"`
	DecidedAt            *string         `json:"decided_at,omitempty"`
	DecisionReason       *string         `json:"decision_reason,omitempty"`
	ExpiresAt            string          `json:"expires_at"`
}

func toPublicApproval(a *approvals.Approval) publicApproval {
	pa := publicApproval{
		ID: a.ID, Action: a.Action,
		ResourceType: a.ResourceType, ResourceID: a.ResourceID,
		Payload:              a.Payload,
		RequestedBy:          a.RequestedBy,
		RequestedAt:          a.RequestedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		RequiredApproverRole: a.RequiredApproverRole,
		Status:               string(a.Status),
		DecidedBy:            a.DecidedBy,
		DecisionReason:       a.DecisionReason,
		ExpiresAt:            a.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if a.DecidedAt != nil {
		s := a.DecidedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		pa.DecidedAt = &s
	}
	return pa
}

func strPtr(s string) *string { return &s }
func ptrDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
