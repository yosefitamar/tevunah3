// tevunah-backend é o servidor HTTP do Tevunah.
//
// Endpoints (MVP):
//
//	GET    /api/health        -> liveness + ambiente
//	POST   /api/auth/login    -> {email, password, totp_code} -> {token, user}
//	GET    /api/auth/me       -> usuário autenticado (Bearer)
//	POST   /api/auth/logout   -> encerra a sessão atual
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/belia/tevunah/backend/internal/approvals"
	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/authz"
	"github.com/belia/tevunah/backend/internal/crypt"
	idb "github.com/belia/tevunah/backend/internal/db"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/pdf"
	"github.com/belia/tevunah/backend/internal/reports"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/permissions"
	"github.com/belia/tevunah/backend/internal/session"
	"github.com/belia/tevunah/backend/internal/settings"
	"github.com/belia/tevunah/backend/internal/users"
	"github.com/pquerna/otp/totp"
)

var start = time.Now()

type app struct {
	env         string
	users       *users.Repo
	audit       *audit.Logger
	auditReader *audit.Reader
	sessions    *session.Store
	policy      *authz.Policy
	approvals   *approvals.Repo
	perms       *permissions.Repo
	entities    *entities.Repo
	reports     *reports.Repo
	settings    *settings.Repo
	pdf         *pdf.Client
}

func main() {
	env := idb.Env("APP_ENV", "development")
	addr := idb.Env("ADDR", ":8080")

	appDB := mustOpen("APP_DATABASE_URL")
	defer appDB.Close()

	auditDB := mustOpen("AUDIT_DATABASE_URL")
	defer auditDB.Close()

	idleTTL := sessionIdleTTL()
	log.Printf("session idle timeout: %v", idleTTL)
	store, err := session.New(idb.Env("REDIS_URL", "redis://redis:6379/0"), idleTTL)
	if err != nil {
		log.Fatalf("session store: %v", err)
	}

	a := &app{
		env:         env,
		users:       users.New(appDB),
		audit:       audit.New(auditDB),
		auditReader: audit.NewReader(appDB),
		sessions:    store,
		policy:      authz.New(appDB),
		approvals:   approvals.New(appDB),
		perms:       permissions.New(appDB),
		entities:    entities.New(appDB),
		reports:     reports.New(appDB),
		settings:    settings.New(appDB),
		pdf:         pdf.New("", photoDir()),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)

	auth := middleware.RequireAuth(a.sessions, a.users)
	mux.Handle("GET /api/auth/me", auth(http.HandlerFunc(a.handleMe)))
	mux.Handle("POST /api/auth/logout", auth(http.HandlerFunc(a.handleLogout)))
	mux.Handle("POST /api/auth/password/change", auth(http.HandlerFunc(a.handlePasswordChange)))
	mux.Handle("POST /api/auth/totp/setup", auth(http.HandlerFunc(a.handleTOTPSetup)))

	mux.Handle("GET /api/users", auth(http.HandlerFunc(a.handleUsersList)))
	mux.Handle("GET /api/users/lookup", auth(http.HandlerFunc(a.handleUsersLookup)))
	mux.Handle("POST /api/users", auth(http.HandlerFunc(a.handleUserCreate)))
	mux.Handle("GET /api/users/{id}", auth(http.HandlerFunc(a.handleUserDetail)))
	mux.Handle("PATCH /api/users/{id}", auth(http.HandlerFunc(a.handleUserUpdate)))
	mux.Handle("POST /api/users/{id}/deactivate", auth(http.HandlerFunc(a.handleUserDeactivate)))
	mux.Handle("POST /api/users/{id}/password/reset", auth(http.HandlerFunc(a.handleUserPasswordReset)))
	mux.Handle("POST /api/users/{id}/totp/reset", auth(http.HandlerFunc(a.handleUserTOTPReset)))
	mux.Handle("POST /api/users/{id}/roles", auth(http.HandlerFunc(a.handleUserSetRoles)))
	mux.Handle("POST /api/users/{id}/clearance", auth(http.HandlerFunc(a.handleUserSetClearance)))

	mux.Handle("GET /api/audit", auth(http.HandlerFunc(a.handleAuditList)))
	mux.Handle("GET /api/audit/{id}", auth(http.HandlerFunc(a.handleAuditDetail)))

	mux.Handle("GET /api/admin/permissions", auth(http.HandlerFunc(a.handleAdminPermissionsList)))
	mux.Handle("PATCH /api/admin/permissions/{role_code}/{action}", auth(http.HandlerFunc(a.handleAdminPermissionsUpdate)))

	mux.Handle("GET /api/roles", auth(http.HandlerFunc(a.handleRolesList)))
	mux.Handle("POST /api/admin/roles", auth(http.HandlerFunc(a.handleRoleCreate)))
	mux.Handle("PATCH /api/admin/roles/{code}", auth(http.HandlerFunc(a.handleRoleUpdate)))
	mux.Handle("DELETE /api/admin/roles/{code}", auth(http.HandlerFunc(a.handleRoleDelete)))

	mux.HandleFunc("GET /api/system-settings", a.handleSystemSettingsGet)
	mux.Handle("PUT /api/admin/system-settings", auth(http.HandlerFunc(a.handleSystemSettingsUpdate)))
	mux.Handle("PUT /api/admin/system-settings/brasao", auth(http.HandlerFunc(a.handleSystemSettingsBrasaoUpload)))
	mux.Handle("GET /api/admin/system-settings/brasao", auth(http.HandlerFunc(a.handleSystemSettingsBrasaoGet)))

	mux.Handle("GET /api/entities/persons/duplicates", auth(http.HandlerFunc(a.handleEntityPersonDuplicates)))
	mux.Handle("GET /api/entities", auth(http.HandlerFunc(a.handleEntitiesList)))
	mux.Handle("POST /api/entities", auth(http.HandlerFunc(a.handleEntityCreate)))
	mux.Handle("GET /api/entities/{id}", auth(http.HandlerFunc(a.handleEntityDetail)))
	mux.Handle("PATCH /api/entities/{id}", auth(http.HandlerFunc(a.handleEntityUpdate)))
	mux.Handle("DELETE /api/entities/{id}", auth(http.HandlerFunc(a.handleEntityDelete)))
	mux.Handle("POST /api/entities/{id}/restore", auth(http.HandlerFunc(a.handleEntityRestore)))
	mux.Handle("GET /api/entities/{id}/photo", auth(http.HandlerFunc(a.handleEntityPhotoGet)))
	mux.Handle("POST /api/entities/{id}/photo", auth(http.HandlerFunc(a.handleEntityPhotoUpload)))
	mux.Handle("DELETE /api/entities/{id}/photo", auth(http.HandlerFunc(a.handleEntityPhotoDelete)))
	mux.Handle("POST /api/entities/{id}/photos", auth(http.HandlerFunc(a.handleEntityGalleryUpload)))
	mux.Handle("GET /api/entities/{id}/photos/{pid}", auth(http.HandlerFunc(a.handleEntityGalleryGet)))
	mux.Handle("PATCH /api/entities/{id}/photos/{pid}", auth(http.HandlerFunc(a.handleEntityGalleryPatch)))
	mux.Handle("DELETE /api/entities/{id}/photos/{pid}", auth(http.HandlerFunc(a.handleEntityGalleryDelete)))
	mux.Handle("GET /api/entities/{id}/links", auth(http.HandlerFunc(a.handleEntityLinksList)))
	mux.Handle("POST /api/entities/{id}/links", auth(http.HandlerFunc(a.handleEntityLinkCreate)))
	mux.Handle("DELETE /api/entities/{id}/links/{lid}", auth(http.HandlerFunc(a.handleEntityLinkDelete)))
	mux.Handle("GET /api/entities/{id}/graph", auth(http.HandlerFunc(a.handleEntityGraph)))
	mux.Handle("GET /api/entities/{id}/addresses", auth(http.HandlerFunc(a.handlePersonAddressList)))
	mux.Handle("POST /api/entities/{id}/addresses", auth(http.HandlerFunc(a.handlePersonAddressCreate)))
	mux.Handle("PATCH /api/entities/{id}/addresses/{aid}", auth(http.HandlerFunc(a.handlePersonAddressUpdate)))
	mux.Handle("DELETE /api/entities/{id}/addresses/{aid}", auth(http.HandlerFunc(a.handlePersonAddressDelete)))

	mux.Handle("GET /api/reports", auth(http.HandlerFunc(a.handleReportsList)))
	mux.Handle("GET /api/reports/years", auth(http.HandlerFunc(a.handleReportsYears)))
	mux.Handle("POST /api/reports", auth(http.HandlerFunc(a.handleReportCreate)))
	mux.Handle("GET /api/reports/{id}", auth(http.HandlerFunc(a.handleReportDetail)))
	mux.Handle("PATCH /api/reports/{id}", auth(http.HandlerFunc(a.handleReportUpdate)))
	mux.Handle("DELETE /api/reports/{id}", auth(http.HandlerFunc(a.handleReportDestroy)))
	mux.Handle("POST /api/reports/{id}/diffuse", auth(http.HandlerFunc(a.handleReportDiffuse)))
	mux.Handle("POST /api/reports/{id}/undiffuse", auth(http.HandlerFunc(a.handleReportUndiffuse)))
	mux.Handle("POST /api/reports/{id}/archive", auth(http.HandlerFunc(a.handleReportArchive)))
	mux.Handle("GET /api/reports/{id}/download", auth(http.HandlerFunc(a.handleReportDownload)))
	mux.Handle("PUT /api/reports/{id}/visibility", auth(http.HandlerFunc(a.handleReportSetVisibility)))
	mux.Handle("GET /api/reports/{id}/viewers", auth(http.HandlerFunc(a.handleReportViewersList)))
	mux.Handle("PUT /api/reports/{id}/viewers", auth(http.HandlerFunc(a.handleReportSetViewers)))
	mux.Handle("POST /api/reports/{id}/qualifications", auth(http.HandlerFunc(a.handleReportQualificationCreate)))
	mux.Handle("DELETE /api/reports/{id}/qualifications/{qid}", auth(http.HandlerFunc(a.handleReportQualificationDelete)))
	mux.Handle("POST /api/reports/{id}/qualifications/{qid}/photo", auth(http.HandlerFunc(a.handleQualificationPhotoUpload)))
	mux.Handle("GET /api/reports/{id}/qualifications/{qid}/photo", auth(http.HandlerFunc(a.handleQualificationPhotoGet)))
	mux.Handle("DELETE /api/reports/{id}/qualifications/{qid}/photo", auth(http.HandlerFunc(a.handleQualificationPhotoDelete)))

	mux.Handle("GET /api/approvals", auth(http.HandlerFunc(a.handleApprovalsList)))
	mux.Handle("GET /api/approvals/{id}", auth(http.HandlerFunc(a.handleApprovalDetail)))
	mux.Handle("POST /api/approvals/{id}/approve", auth(http.HandlerFunc(a.handleApprovalApprove)))
	mux.Handle("POST /api/approvals/{id}/reject", auth(http.HandlerFunc(a.handleApprovalReject)))
	mux.Handle("POST /api/approvals/{id}/cancel", auth(http.HandlerFunc(a.handleApprovalCancel)))

	srv := &http.Server{
		Addr:              addr,
		Handler:           withRequestLog(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("tevunah-backend (%s) escutando em %s", env, addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// sessionIdleTTL lê SESSION_IDLE_MINUTES do ambiente e devolve a duração
// correspondente. Default: 15 minutos (alinhado com NIST 800-63B AAL2, que
// permite até 30 min; OWASP recomenda 2-5 min para apps de alto valor e
// 15-30 min para baixo risco — 15 é o sweet-spot para um sistema de
// inteligência com MFA + audit chain, dado o workflow prolongado de análise).
// Clamp em [1, 60] para evitar configuração acidental insegura.
func sessionIdleTTL() time.Duration {
	const defaultMin = 15
	v := os.Getenv("SESSION_IDLE_MINUTES")
	if v == "" {
		return time.Duration(defaultMin) * time.Minute
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		log.Printf("SESSION_IDLE_MINUTES inválido (%q) — usando default %dm", v, defaultMin)
		return time.Duration(defaultMin) * time.Minute
	}
	if n > 60 {
		log.Printf("SESSION_IDLE_MINUTES %d > 60 acima do recomendado por NIST AAL2 — clampando para 60m", n)
		n = 60
	}
	return time.Duration(n) * time.Minute
}

func mustOpen(envVar string) *sql.DB {
	dsn := os.Getenv(envVar)
	if dsn == "" {
		log.Fatalf("%s não definido", envVar)
	}
	d, err := idb.Open(dsn)
	if err != nil {
		log.Fatalf("%s: %v", envVar, err)
	}
	return d
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(t0))
	})
}

// ─────────────────────────── handlers ────────────────────────────

func (a *app) handleHealth(w http.ResponseWriter, _ *http.Request) {
	httpx.OK(w, map[string]any{
		"service": "tevunah-backend",
		"env":     a.env,
		"uptime":  time.Since(start).String(),
		"now":     time.Now().UTC().Format(time.RFC3339),
	})
}

type loginRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	TOTPCode  string `json:"totp_code"`
}

type publicUser struct {
	ID                 string     `json:"id"`
	Code               string     `json:"code"`
	Email              string     `json:"email"`
	DisplayName        string     `json:"display_name"`
	ClearanceLevel     int        `json:"clearance_level"`
	Status             string     `json:"status"`
	Roles              []string   `json:"roles"`
	Permissions        []string   `json:"permissions,omitempty"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
	MustChangePassword bool       `json:"must_change_password,omitempty"`
	MustSetupTOTP      bool       `json:"must_setup_totp,omitempty"`
}

func toPublic(u *users.User) publicUser {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	return publicUser{
		ID: u.ID, Code: u.Code, Email: u.Email, DisplayName: u.DisplayName,
		ClearanceLevel: u.ClearanceLevel, Status: u.Status,
		Roles: roles, LastLoginAt: u.LastLoginAt,
		MustChangePassword: u.MustChangePassword,
		MustSetupTOTP:      u.MustSetupTOTP,
	}
}

func (a *app) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if req.Email == "" || req.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "email e senha são obrigatórios")
		return
	}

	ctx := r.Context()
	ip := httpx.ClientIP(r)
	var uaPtr *string
	if ua := r.UserAgent(); ua != "" {
		uaPtr = &ua
	}
	logDenied := func(actorID *string, reason string) {
		_ = a.audit.Log(ctx, audit.Entry{
			ActorUserID:    actorID,
			ActorIP:        audit.Ptr(ip),
			ActorUserAgent: uaPtr,
			Action:         "auth.login_denied",
			Reason:         audit.Ptr(reason),
			After:          map[string]any{"email": req.Email},
		})
	}

	u, err := a.users.FindByEmail(ctx, req.Email)
	if err != nil {
		logDenied(nil, "usuário não encontrado")
		httpx.Error(w, http.StatusUnauthorized, "credenciais inválidas")
		return
	}
	if !u.IsActive() {
		logDenied(&u.ID, "usuário inativo: "+u.Status)
		httpx.Error(w, http.StatusUnauthorized, "credenciais inválidas")
		return
	}

	ok, err := crypt.Verify(req.Password, u.PasswordHash)
	if err != nil || !ok {
		logDenied(&u.ID, "senha inválida")
		httpx.Error(w, http.StatusUnauthorized, "credenciais inválidas")
		return
	}

	// Auto-rehash: usuários importados do legado têm hash bcrypt. Na primeira
	// autenticação bem-sucedida, re-codificamos em argon2id e persistimos —
	// transparente pro usuário, sem reset de senha. Falha aqui não bloqueia
	// o login (a senha continua válida via bcrypt na próxima tentativa).
	if crypt.NeedsRehash(u.PasswordHash) {
		if newHash, herr := crypt.Hash(req.Password); herr == nil {
			if err := a.users.SetPassword(ctx, u.ID, newHash, false); err != nil {
				log.Printf("auto-rehash %s: %v", u.ID, err)
			}
		}
	}

	// Setup pendente do TOTP: pula a validação do código. Gera o secret se
	// ainda não foi gerado (primeira tentativa após reset). Persiste como
	// pending — só vira definitivo quando o agente confirma no setup.
	var pendingTOTPSecret string
	if u.MustSetupTOTP {
		if u.TOTPSecret == "" {
			secret, err := crypt.NewTOTPSecret()
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "erro ao gerar TOTP")
				return
			}
			if err := a.users.SetPendingTOTPSecret(ctx, u.ID, secret); err != nil {
				log.Printf("set pending totp: %v", err)
				httpx.Error(w, http.StatusInternalServerError, "erro ao iniciar setup TOTP")
				return
			}
			u.TOTPSecret = secret
		}
		pendingTOTPSecret = u.TOTPSecret
	} else {
		if u.TOTPSecret == "" {
			logDenied(&u.ID, "TOTP não configurado")
			httpx.Error(w, http.StatusUnauthorized, "credenciais inválidas")
			return
		}
		if req.TOTPCode == "" {
			httpx.Error(w, http.StatusBadRequest, "código TOTP obrigatório")
			return
		}
		if !totp.Validate(req.TOTPCode, u.TOTPSecret) {
			logDenied(&u.ID, "código TOTP inválido")
			httpx.Error(w, http.StatusUnauthorized, "credenciais inválidas")
			return
		}
	}

	sess, err := a.sessions.Create(ctx, u.ID, ip)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao iniciar sessão")
		return
	}
	a.setSessionCookie(w, sess.Token, int(a.sessions.TTL().Seconds()))
	if err := a.users.TouchLastLogin(ctx, u.ID); err != nil {
		log.Printf("touch last_login: %v", err)
	}
	_ = a.audit.Log(ctx, audit.Entry{
		ActorUserID:    &u.ID,
		ActorSessionID: &sess.Token,
		ActorIP:        audit.Ptr(ip),
		ActorUserAgent: uaPtr,
		Action:         "auth.login",
		After:          map[string]any{"email": u.Email, "roles": u.Roles},
	})

	resp := map[string]any{
		"token":      sess.Token,
		"expires_in": int(a.sessions.TTL().Seconds()),
		"user":       a.publicWithPerms(ctx, u),
	}
	if pendingTOTPSecret != "" {
		// Agente usa esse secret pra montar QR/inserir no authenticator.
		// Frontend confirma via POST /api/auth/totp/setup.
		resp["totp_setup"] = map[string]any{
			"secret": pendingTOTPSecret,
			"email":  u.Email,
		}
	}
	httpx.OK(w, resp)
}

// publicWithPerms monta o usuário público já com as permissões efetivas
// (ações que ele pode executar), usadas pelo front pra gating por permissão em
// vez de por nome de papel. Se a resolução falhar, devolve sem permissões — a
// UI degrada escondendo botões, e o servidor segue sendo a fonte de verdade.
func (a *app) publicWithPerms(ctx context.Context, u *users.User) publicUser {
	pu := toPublic(u)
	if perms, err := a.policy.AllowedActions(ctx, u.Roles); err == nil {
		pu.Permissions = perms
	} else {
		log.Printf("allowed actions: %v", err)
	}
	return pu
}

func (a *app) handleMe(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	httpx.OK(w, map[string]any{"user": a.publicWithPerms(r.Context(), u)})
}

// setSessionCookie emite o cookie HttpOnly da sessão.
// Secure é ligado em produção (HTTPS). SameSite=Strict bloqueia envio cross-site.
func (a *app) setSessionCookie(w http.ResponseWriter, token string, maxAgeSec int) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.env == "production",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAgeSec,
	})
}

func (a *app) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.env == "production",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (a *app) handleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := middleware.SessionFrom(ctx)
	u := middleware.UserFrom(ctx)

	if err := a.sessions.Delete(ctx, sess.Token); err != nil {
		log.Printf("delete session: %v", err)
	}
	a.clearSessionCookie(w)
	var uaPtr *string
	if ua := r.UserAgent(); ua != "" {
		uaPtr = &ua
	}
	_ = a.audit.Log(ctx, audit.Entry{
		ActorUserID:    &u.ID,
		ActorSessionID: &sess.Token,
		ActorIP:        audit.Ptr(httpx.ClientIP(r)),
		ActorUserAgent: uaPtr,
		Action:         "auth.logout",
	})
	httpx.NoContent(w)
}
