// Package middleware tem os interceptadores HTTP do projeto.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/session"
	"github.com/belia/tevunah/backend/internal/users"
)

type ctxKey int

const (
	userCtxKey ctxKey = iota
	sessionCtxKey
)

// WithUser injeta o usuário autenticado no contexto.
func WithUser(ctx context.Context, u *users.User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

// UserFrom devolve o usuário injetado pelo middleware (ou nil).
func UserFrom(ctx context.Context) *users.User {
	u, _ := ctx.Value(userCtxKey).(*users.User)
	return u
}

// WithSession injeta a sessão no contexto.
func WithSession(ctx context.Context, s *session.Session) context.Context {
	return context.WithValue(ctx, sessionCtxKey, s)
}

// SessionFrom devolve a sessão (ou nil).
func SessionFrom(ctx context.Context) *session.Session {
	s, _ := ctx.Value(sessionCtxKey).(*session.Session)
	return s
}

// SessionCookieName é o nome do cookie HttpOnly de sessão.
const SessionCookieName = "tevunah_session"

// RequireAuth lê o token (cookie OU Bearer), valida sessão, refresha TTL e injeta o usuário.
func RequireAuth(store *session.Store, repo *users.Repo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := tokenFrom(r)
			if tok == "" {
				httpx.Error(w, http.StatusUnauthorized, "token ausente")
				return
			}
			sess, err := store.Get(r.Context(), tok)
			if err != nil {
				if errors.Is(err, session.ErrNotFound) {
					httpx.Error(w, http.StatusUnauthorized, "sessão inválida ou expirada")
					return
				}
				httpx.Error(w, http.StatusInternalServerError, "erro ao validar sessão")
				return
			}
			u, err := repo.FindByID(r.Context(), sess.UserID)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "usuário não localizável")
				return
			}
			if !u.IsActive() {
				httpx.Error(w, http.StatusForbidden, "usuário inativo")
				return
			}
			// Refresh TTL idle.
			if err := store.Touch(r.Context(), sess); err != nil {
				httpx.Error(w, http.StatusInternalServerError, "erro ao renovar sessão")
				return
			}
			ctx := WithUser(WithSession(r.Context(), sess), u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// tokenFrom extrai o token de sessão, preferindo Authorization: Bearer (para CLIs/testes)
// e caindo para o cookie HttpOnly emitido em login bem-sucedido.
func tokenFrom(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}
