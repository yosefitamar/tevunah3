package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/ai"
	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// Assistente de IA — configuração do PRÓPRIO usuário (/api/me/ai). Cada um
// cadastra a sua chave da API Anthropic; ninguém vê nem usa a chave de
// outro. A chave só entra (PUT) e nunca volta nas respostas: a tela recebe
// a dica "sk-ant-…a1B2".

type aiSettingsResponse struct {
	SecretsReady    bool            `json:"secrets_ready"`
	Configured      bool            `json:"configured"`
	KeyHint         string          `json:"key_hint,omitempty"`
	KeyVerifiedAt   *time.Time      `json:"key_verified_at,omitempty"`
	AvailableModels []string        `json:"available_models"`
	Model           string          `json:"model"`
	Usage           ai.UsageProfile `json:"usage"`
	Catalog         []ai.Model      `json:"catalog"`
	PricesAsOf      string          `json:"prices_as_of"`
}

func (a *app) aiSettingsPayload(r *http.Request, userID string) (*aiSettingsResponse, error) {
	s, err := a.ai.Get(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	return &aiSettingsResponse{
		SecretsReady:    a.ai.SecretsReady(),
		Configured:      s.HasKey,
		KeyHint:         s.KeyHint,
		KeyVerifiedAt:   s.KeyVerifiedAt,
		AvailableModels: s.AvailableModels,
		Model:           s.Model,
		Usage:           s.Usage,
		Catalog:         ai.Catalog,
		PricesAsOf:      ai.PricesAsOf,
	}, nil
}

func (a *app) writeAISettings(w http.ResponseWriter, r *http.Request, userID string) {
	out, err := a.aiSettingsPayload(r, userID)
	if err != nil {
		log.Printf("ai settings get: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao carregar a configuração de IA")
		return
	}
	httpx.OK(w, out)
}

// GET /api/me/ai
func (a *app) handleAISettingsGet(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	a.writeAISettings(w, r, me.ID)
}

// verifyStatus traduz o erro da verificação para HTTP.
func verifyStatus(err error) int {
	switch {
	case errors.Is(err, ai.ErrKeyFormat), errors.Is(err, ai.ErrKeyAdmin):
		return http.StatusBadRequest
	case errors.Is(err, ai.ErrKeyRejected), errors.Is(err, ai.ErrKeyForbidden):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusBadGateway
	}
}

// PUT /api/me/ai/key {api_key}
//
// Verifica a chave na API da Anthropic (lista de modelos, sem custo) antes
// de gravar: chave que não funciona não é guardada.
func (a *app) handleAIKeyPut(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	if !a.ai.SecretsReady() {
		httpx.Error(w, http.StatusServiceUnavailable,
			"o servidor ainda não tem o cofre de segredos configurado (APP_SECRETS_KEY) — fale com o administrador")
		return
	}
	var req struct {
		APIKey string `json:"api_key"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	key := strings.TrimSpace(req.APIKey)
	models, err := ai.VerifyKey(r.Context(), key)
	if err != nil {
		if errors.Is(err, ai.ErrUpstream) {
			log.Printf("ai key verify (user %s): falha na API", me.ID)
		}
		httpx.Error(w, verifyStatus(err), err.Error())
		return
	}
	prev, _ := a.ai.Get(r.Context(), me.ID)
	if err := a.ai.SaveKey(r.Context(), me.ID, key, models); err != nil {
		log.Printf("ai key save: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar a chave")
		return
	}
	var before map[string]any
	if prev != nil && prev.HasKey {
		before = map[string]any{"key_hint": prev.KeyHint}
	}
	a.auditAI(r, "ai.key_set", me.ID, before, map[string]any{
		"provider": "anthropic", "key_hint": ai.KeyHint(key), "available_models": len(models),
	})
	a.writeAISettings(w, r, me.ID)
}

// POST /api/me/ai/key/verify — testa de novo a chave gravada.
func (a *app) handleAIKeyVerify(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	key, _, err := a.ai.Credentials(r.Context(), me.ID)
	if err != nil {
		if errors.Is(err, ai.ErrNoKey) {
			httpx.Error(w, http.StatusNotFound, err.Error())
			return
		}
		log.Printf("ai credentials: %v", err)
		httpx.Error(w, http.StatusInternalServerError,
			"não foi possível abrir a chave gravada — cadastre-a de novo")
		return
	}
	models, err := ai.VerifyKey(r.Context(), key)
	if err != nil {
		httpx.Error(w, verifyStatus(err), err.Error())
		return
	}
	if err := a.ai.MarkVerified(r.Context(), me.ID, models); err != nil {
		log.Printf("ai mark verified: %v", err)
	}
	a.writeAISettings(w, r, me.ID)
}

// DELETE /api/me/ai/key
func (a *app) handleAIKeyDelete(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	prev, err := a.ai.Get(r.Context(), me.ID)
	if err != nil {
		log.Printf("ai get: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover a chave")
		return
	}
	if prev.HasKey {
		if err := a.ai.ClearKey(r.Context(), me.ID); err != nil {
			log.Printf("ai key clear: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao remover a chave")
			return
		}
		a.auditAI(r, "ai.key_removed", me.ID, map[string]any{"key_hint": prev.KeyHint}, nil)
	}
	a.writeAISettings(w, r, me.ID)
}

// PUT /api/me/ai/settings {model, usage}
func (a *app) handleAISettingsPut(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	var req struct {
		Model string          `json:"model"`
		Usage ai.UsageProfile `json:"usage"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	prev, _ := a.ai.Get(r.Context(), me.ID)
	if err := a.ai.SaveSettings(r.Context(), me.ID, req.Model, req.Usage); err != nil {
		switch {
		case errors.Is(err, ai.ErrInvalidModel):
			httpx.Error(w, http.StatusBadRequest, "modelo não oferecido")
		case errors.Is(err, ai.ErrInvalidUsage):
			httpx.Error(w, http.StatusBadRequest, "perfil de uso fora dos limites")
		default:
			log.Printf("ai settings save: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao gravar a configuração de IA")
		}
		return
	}
	if prev == nil || prev.Model != req.Model {
		var before map[string]any
		if prev != nil {
			before = map[string]any{"model": prev.Model}
		}
		a.auditAI(r, "ai.model_set", me.ID, before, map[string]any{"model": req.Model})
	}
	a.writeAISettings(w, r, me.ID)
}

// auditAI registra cadastro/remoção de chave e troca de modelo. Nunca a
// chave: só a dica.
func (a *app) auditAI(r *http.Request, action, userID string, before, after map[string]any) {
	aid, sid, ip, ua := a.actorInfo(r)
	entry := audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       action,
		ResourceType: audit.Ptr("user_ai_settings"),
		ResourceID:   audit.Ptr(userID),
	}
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
