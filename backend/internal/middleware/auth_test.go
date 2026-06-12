package middleware

import (
	"testing"

	"github.com/belia/tevunah/backend/internal/users"
)

// O caso crítico é a coexistência das duas flags (admin resetou senha E
// TOTP): ambos os endpoints de resolução precisam ficar acessíveis, senão
// o agente entra em deadlock — não consegue trocar a senha nem confirmar
// o TOTP.
func TestIsPendingCredentialAllowed(t *testing.T) {
	cases := []struct {
		name       string
		setupTOTP  bool
		changePass bool
		path       string
		want       bool
	}{
		{"ambas flags: troca de senha liberada", true, true, "/api/auth/password/change", true},
		{"ambas flags: setup de TOTP liberado", true, true, "/api/auth/totp/setup", true},
		{"ambas flags: me liberado", true, true, "/api/auth/me", true},
		{"ambas flags: logout liberado", true, true, "/api/auth/logout", true},
		{"ambas flags: resto bloqueado", true, true, "/api/users", false},

		{"só TOTP: setup liberado", true, false, "/api/auth/totp/setup", true},
		{"só TOTP: troca de senha bloqueada", true, false, "/api/auth/password/change", false},

		{"só senha: troca liberada", false, true, "/api/auth/password/change", true},
		{"só senha: setup de TOTP bloqueado", false, true, "/api/auth/totp/setup", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := &users.User{MustSetupTOTP: c.setupTOTP, MustChangePassword: c.changePass}
			if got := isPendingCredentialAllowed(u, c.path); got != c.want {
				t.Errorf("isPendingCredentialAllowed(%v, %v, %q) = %v, want %v",
					c.setupTOTP, c.changePass, c.path, got, c.want)
			}
		})
	}
}
