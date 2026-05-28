// users_credentials.go reúne os handlers de reset/troca de senha e reset/
// setup de TOTP. Fluxos:
//
//   POST /api/users/{id}/password/reset  (admin)
//     → gera senha aleatória, hashea, força troca no próximo login,
//       revoga sessões ativas e DEVOLVE a temporária UMA VEZ.
//
//   POST /api/auth/password/change       (self, autenticado)
//     → {current_password, new_password}; valida atual, troca hash,
//       limpa must_change_password.
//
//   POST /api/users/{id}/totp/reset      (admin)
//     → apaga o secret, marca must_setup_totp=true, revoga sessões.
//       Admin NÃO vê o novo secret — quem vê é o agente no próximo login.
//
//   POST /api/auth/totp/setup            (self, autenticado em estado de
//                                         must_setup_totp)
//     → {totp_code} confirma o enrollment do novo secret (gerado e
//       persistido pelo handleLogin como "pending"). Limpa o flag.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/crypt"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/users"
	"github.com/pquerna/otp/totp"
)

// ─────────── helpers ───────────

// generateTempPassword devolve uma string de ~16 chars URL-safe pra ser
// entregue ao agente. Usa crypto/rand; alfabeto base64 sem padding.
func generateTempPassword() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// ─────────── POST /api/users/{id}/password/reset ───────────

func (a *app) handleUserPasswordReset(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "user.password.reset") {
		return
	}
	id := r.PathValue("id")
	me := middleware.UserFrom(r.Context())
	if id == me.ID {
		httpx.Error(w, http.StatusBadRequest,
			"para alterar a própria senha use /api/auth/password/change")
		return
	}
	target, err := a.users.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "usuário não encontrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if !target.IsActive() {
		httpx.Error(w, http.StatusBadRequest, "agente inativo")
		return
	}

	temp, err := generateTempPassword()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao gerar senha temporária")
		return
	}
	hash, err := crypt.Hash(temp)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao hashear senha")
		return
	}
	if err := a.users.SetPassword(r.Context(), id, hash, true); err != nil {
		log.Printf("password reset: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar senha")
		return
	}
	if _, err := a.sessions.DeleteAllForUser(r.Context(), id); err != nil {
		log.Printf("revoke sessions on password reset: %v", err)
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:    aid,
		ActorSessionID: sid,
		ActorIP:        ip,
		ActorUserAgent: ua,
		Action:         "user.password.reset",
		ResourceType:   audit.Ptr("user"),
		ResourceID:     audit.Ptr(id),
		After:          map[string]any{"must_change_password": true},
	})

	httpx.OK(w, map[string]any{
		"temp_password": temp,
		"note":          "Repasse esta senha de forma segura. O agente deverá trocá-la no próximo login.",
	})
}

// ─────────── POST /api/auth/password/change ───────────

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (a *app) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	var req changePasswordRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		httpx.Error(w, http.StatusBadRequest, "current_password e new_password são obrigatórios")
		return
	}
	if len(req.NewPassword) < 8 {
		httpx.Error(w, http.StatusBadRequest, "nova senha deve ter ao menos 8 caracteres")
		return
	}
	if req.CurrentPassword == req.NewPassword {
		httpx.Error(w, http.StatusBadRequest, "a nova senha deve ser diferente da atual")
		return
	}

	ok, err := crypt.Verify(req.CurrentPassword, me.PasswordHash)
	if err != nil || !ok {
		httpx.Error(w, http.StatusUnauthorized, "senha atual inválida")
		return
	}

	hash, err := crypt.Hash(req.NewPassword)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao hashear senha")
		return
	}
	if err := a.users.SetPassword(r.Context(), me.ID, hash, false); err != nil {
		log.Printf("password change: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar senha")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:    aid,
		ActorSessionID: sid,
		ActorIP:        ip,
		ActorUserAgent: ua,
		Action:         "user.password.change.self",
		ResourceType:   audit.Ptr("user"),
		ResourceID:     audit.Ptr(me.ID),
	})

	httpx.NoContent(w)
}

// ─────────── POST /api/users/{id}/totp/reset ───────────

func (a *app) handleUserTOTPReset(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "user.totp.reset") {
		return
	}
	id := r.PathValue("id")
	target, err := a.users.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "usuário não encontrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if !target.IsActive() {
		httpx.Error(w, http.StatusBadRequest, "agente inativo")
		return
	}
	if err := a.users.ResetTOTP(r.Context(), id); err != nil {
		log.Printf("totp reset: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao resetar TOTP")
		return
	}
	if _, err := a.sessions.DeleteAllForUser(r.Context(), id); err != nil {
		log.Printf("revoke sessions on totp reset: %v", err)
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:    aid,
		ActorSessionID: sid,
		ActorIP:        ip,
		ActorUserAgent: ua,
		Action:         "user.totp.reset",
		ResourceType:   audit.Ptr("user"),
		ResourceID:     audit.Ptr(id),
		After:          map[string]any{"must_setup_totp": true},
	})

	httpx.OK(w, map[string]any{
		"note": "TOTP resetado. O agente receberá o novo QR no próximo login.",
	})
}

// ─────────── POST /api/auth/totp/setup ───────────

type totpSetupRequest struct {
	TOTPCode string `json:"totp_code"`
}

func (a *app) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	me := middleware.UserFrom(r.Context())
	if !me.MustSetupTOTP {
		httpx.Error(w, http.StatusBadRequest, "TOTP já configurado")
		return
	}
	if me.TOTPSecret == "" {
		httpx.Error(w, http.StatusBadRequest,
			"secret pendente ausente — reabra a sessão de login")
		return
	}
	var req totpSetupRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	code := strings.TrimSpace(req.TOTPCode)
	if code == "" {
		httpx.Error(w, http.StatusBadRequest, "totp_code obrigatório")
		return
	}
	if !totp.Validate(code, me.TOTPSecret) {
		httpx.Error(w, http.StatusUnauthorized, "código TOTP inválido")
		return
	}
	if err := a.users.CompleteTOTPSetup(r.Context(), me.ID); err != nil {
		log.Printf("totp complete: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao confirmar TOTP")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:    aid,
		ActorSessionID: sid,
		ActorIP:        ip,
		ActorUserAgent: ua,
		Action:         "user.totp.setup.self",
		ResourceType:   audit.Ptr("user"),
		ResourceID:     audit.Ptr(me.ID),
	})

	httpx.NoContent(w)
}
