package main

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/crypt"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/users"
)

var (
	validRoles = map[string]bool{
		"agente": true, "analista": true, "gestor": true, "administrador": true,
	}
	emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// actorInfo extrai actor_user_id, actor_session_id, ip e user_agent da
// requisição para preencher entradas de audit.
func (a *app) actorInfo(r *http.Request) (actorID, sessionID, ip, userAgent *string) {
	if u := middleware.UserFrom(r.Context()); u != nil {
		actorID = &u.ID
	}
	if s := middleware.SessionFrom(r.Context()); s != nil {
		sessionID = &s.Token
	}
	v := httpx.ClientIP(r)
	ip = &v
	if ua := r.UserAgent(); ua != "" {
		userAgent = &ua
	}
	return
}

// requirePerm avalia policy.Can; retorna true se autorizado. Em caso de negação,
// já escreve a resposta 403/500 apropriada.
func (a *app) requirePerm(w http.ResponseWriter, r *http.Request, action string) bool {
	me := middleware.UserFrom(r.Context())
	d, err := a.policy.Can(r.Context(), me.Roles, action)
	if err != nil {
		log.Printf("policy: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro de autorização")
		return false
	}
	if !d.Allowed {
		httpx.Error(w, http.StatusForbidden, "ação não autorizada: "+action)
		return false
	}
	// MVP: ações que exigem 4-eyes ainda não têm fluxo no servidor — só list/read/create/update.self
	// passam por aqui e nenhuma delas é dual no seed inicial. Caso fique, devolvemos 409 informando.
	if d.RequiresDualApproval {
		httpx.Error(w, http.StatusConflict,
			"ação requer aprovação dupla por "+d.ApproverRole+" (fluxo ainda não disponível)")
		return false
	}
	return true
}

// ─────────────────────────── GET /api/users ────────────────────────────

func (a *app) handleUsersList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "user.list") {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	clearance, _ := strconv.Atoi(q.Get("clearance"))
	res, err := a.users.List(r.Context(), users.ListOpts{
		Limit:     limit,
		Offset:    offset,
		Status:    q.Get("status"),
		Role:      q.Get("role"),
		Clearance: clearance,
		Search:    strings.TrimSpace(q.Get("search")),
		SortBy:    q.Get("sort_by"),
		SortDir:   q.Get("sort_dir"),
	})
	if err != nil {
		log.Printf("users list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar usuários")
		return
	}
	items := make([]publicUser, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, toPublic(&res.Items[i]))
	}
	httpx.OK(w, map[string]any{
		"items":  items,
		"total":  res.Total,
		"limit":  cmpDefault(limit, 25),
		"offset": offset,
	})
}

func cmpDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// ─────────────────────────── GET /api/users/{id} ────────────────────────

func (a *app) handleUserDetail(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}

	action := "user.list"
	if id == me.ID {
		action = "user.read.self"
	}
	if !a.requirePerm(w, r, action) {
		return
	}

	u, err := a.users.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "usuário não encontrado")
			return
		}
		log.Printf("users find: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	httpx.OK(w, map[string]any{"user": toPublic(u)})
}

// ─────────────────────────── POST /api/users ────────────────────────────

type createUserRequest struct {
	Email          string   `json:"email"`
	DisplayName    string   `json:"display_name"`
	Password       string   `json:"password"`
	Roles          []string `json:"roles"`
	ClearanceLevel int      `json:"clearance_level"`
}

func (a *app) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "user.create") {
		return
	}
	me := middleware.UserFrom(r.Context())

	var req createUserRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if !emailRE.MatchString(req.Email) {
		httpx.Error(w, http.StatusBadRequest, "e-mail inválido")
		return
	}
	if req.DisplayName == "" {
		httpx.Error(w, http.StatusBadRequest, "nome obrigatório")
		return
	}
	if len(req.Password) < 12 {
		httpx.Error(w, http.StatusBadRequest, "senha deve ter ao menos 12 caracteres")
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
	if req.ClearanceLevel == 0 {
		req.ClearanceLevel = 1
	}
	if req.ClearanceLevel < 1 || req.ClearanceLevel > 5 {
		httpx.Error(w, http.StatusBadRequest, "clearance_level deve estar entre 1 e 5")
		return
	}

	code, err := a.users.GenerateCode(r.Context())
	if err != nil {
		log.Printf("generate code: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gerar código")
		return
	}

	hash, err := crypt.Hash(req.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao processar senha")
		return
	}
	secret, err := crypt.NewTOTPSecret()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao gerar TOTP")
		return
	}

	u, err := a.users.Create(r.Context(), users.NewUser{
		Code:           code,
		Email:          req.Email,
		DisplayName:    req.DisplayName,
		PasswordHash:   hash,
		TOTPSecret:     secret,
		ClearanceLevel: req.ClearanceLevel,
		Roles:          req.Roles,
		CreatedBy:      &me.ID,
	})
	if err != nil {
		if errors.Is(err, users.ErrDuplicate) {
			httpx.Error(w, http.StatusConflict, "e-mail ou código já em uso")
			return
		}
		log.Printf("users create: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao criar usuário")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:    aid,
		ActorSessionID: sid,
		ActorIP:        ip,
		ActorUserAgent: ua,
		Action:         "user.create",
		ResourceType:   audit.Ptr("user"),
		ResourceID:     audit.Ptr(u.ID),
		After: map[string]any{
			"code": u.Code, "email": u.Email, "display_name": u.DisplayName,
			"clearance_level": u.ClearanceLevel, "roles": u.Roles,
		},
	})

	httpx.Created(w, map[string]any{
		"user":        toPublic(u),
		"totp_secret": secret,
		"note":        "guarde o TOTP secret agora — ele não será exibido novamente.",
	})
}

// ─────────────────────────── POST /api/users/{id}/deactivate ───────────

type deactivateRequest struct {
	Reason string `json:"reason"`
}

func (a *app) handleUserDeactivate(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	if id == me.ID {
		httpx.Error(w, http.StatusForbidden, "não é possível desativar o próprio usuário")
		return
	}
	if !a.requirePerm(w, r, "user.deactivate") {
		return
	}

	var req deactivateRequest
	_ = httpx.Decode(r, &req) // body opcional

	before, err := a.users.Deactivate(r.Context(), id)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "usuário não encontrado")
			return
		}
		if errors.Is(err, users.ErrAlreadyInactive) {
			httpx.Error(w, http.StatusConflict, "usuário já está inativo")
			return
		}
		log.Printf("deactivate: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao desativar")
		return
	}

	// Revoga todas as sessões do agente desativado.
	if n, err := a.sessions.DeleteAllForUser(r.Context(), id); err != nil {
		log.Printf("revogar sessões de %s: %v", id, err)
	} else if n > 0 {
		log.Printf("desativação: %d sessão(ões) revogada(s) de %s", n, id)
	}

	aid, sid, ip, ua := a.actorInfo(r)
	reason := strings.TrimSpace(req.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:    aid,
		ActorSessionID: sid,
		ActorIP:        ip,
		ActorUserAgent: ua,
		Action:         "user.deactivate",
		ResourceType:   audit.Ptr("user"),
		ResourceID:     audit.Ptr(id),
		Before: map[string]any{
			"status": before.Status,
			"code":   before.Code,
			"email":  before.Email,
		},
		After: map[string]any{
			"status": "deactivated",
		},
		Reason: reasonPtr,
	})

	httpx.NoContent(w)
}

// ─────────────────────────── PATCH /api/users/{id} ──────────────────────

type updateUserRequest struct {
	DisplayName *string `json:"display_name"`
}

func (a *app) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	if id != me.ID {
		// Edição de outros usuários é coberta por ações específicas
		// (assign role, set clearance, etc.) com 4-eyes — fora deste handler.
		httpx.Error(w, http.StatusForbidden,
			"somente edição do próprio perfil é permitida por este endpoint")
		return
	}
	if !a.requirePerm(w, r, "user.update.self") {
		return
	}

	var req updateUserRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if req.DisplayName == nil {
		httpx.Error(w, http.StatusBadRequest, "nenhum campo para atualizar")
		return
	}
	name := strings.TrimSpace(*req.DisplayName)
	if name == "" {
		httpx.Error(w, http.StatusBadRequest, "display_name não pode ser vazio")
		return
	}

	before := me.DisplayName
	if err := a.users.UpdateDisplayName(r.Context(), me.ID, name); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "usuário não encontrado")
			return
		}
		log.Printf("users update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:    aid,
		ActorSessionID: sid,
		ActorIP:        ip,
		ActorUserAgent: ua,
		Action:         "user.update.self",
		ResourceType:   audit.Ptr("user"),
		ResourceID:     audit.Ptr(me.ID),
		Before:         map[string]any{"display_name": before},
		After:          map[string]any{"display_name": name},
	})

	u, err := a.users.FindByID(r.Context(), me.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao recarregar usuário")
		return
	}
	httpx.OK(w, map[string]any{"user": toPublic(u)})
}

