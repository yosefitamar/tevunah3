// Package authz implementa o policy decision point central (PDP).
//
// `Can(user, action)` combina as linhas da matriz `app.permissions` pertinentes
// aos papéis do usuário e devolve uma `Decision`. Regras de combinação para
// usuários multi-role:
//
//   - Se nenhuma linha permite a ação -> negado.
//   - Se ao menos uma permite, é permitido.
//   - Se ao menos uma permitir SEM dual approval, a ação não exige dual approval
//     (a regra mais permissiva vence — multi-role é união de capacidades).
//   - Caso só permita via linhas com dual approval, a decisão exige dual
//     approval; o `ApproverRole` resultante é o de uma linha qualquer que
//     permitiu (todas as linhas que exigem dual normalmente apontam para o
//     mesmo aprovador, mas se divergirem, qualquer um serve).
package authz

import (
	"context"
	"database/sql"
)

// Decision descreve o resultado de uma checagem de permissão.
type Decision struct {
	Allowed              bool
	RequiresDualApproval bool
	ApproverRole         string
}

// Reason devolve um motivo legível.
func (d Decision) Reason() string {
	if !d.Allowed {
		return "negado pela matriz de permissões"
	}
	if d.RequiresDualApproval {
		return "permitido, mas exige aprovação dupla por " + d.ApproverRole
	}
	return "permitido"
}

// Permission é uma linha da matriz, exposta a chamadores de teste.
type Permission struct {
	RoleCode             string
	Action               string
	Allowed              bool
	RequiresDualApproval bool
	ApproverRole         string
}

// Policy resolve permissões consultando o banco.
type Policy struct {
	db *sql.DB
}

func New(db *sql.DB) *Policy { return &Policy{db: db} }

// Can resolve a decisão para um usuário (lista de papéis) e uma ação.
func (p *Policy) Can(ctx context.Context, roles []string, action string) (Decision, error) {
	if len(roles) == 0 {
		return Decision{}, nil
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT role_code, allowed, requires_dual_approval, COALESCE(approver_role, '')
		  FROM app.permissions
		 WHERE action = $1 AND role_code = ANY($2)`,
		action, roles)
	if err != nil {
		return Decision{}, err
	}
	defer rows.Close()

	var perms []Permission
	for rows.Next() {
		var pm Permission
		pm.Action = action
		if err := rows.Scan(&pm.RoleCode, &pm.Allowed, &pm.RequiresDualApproval, &pm.ApproverRole); err != nil {
			return Decision{}, err
		}
		perms = append(perms, pm)
	}
	if err := rows.Err(); err != nil {
		return Decision{}, err
	}
	return Combine(perms), nil
}

// AllowedActions devolve o conjunto de ações que o usuário (lista de papéis)
// pode executar — i.e., toda ação cuja matriz tem ao menos uma linha permitida
// para algum papel do usuário. Usado para o gating da UI por permissões
// efetivas (em vez de por nome de papel). A regra de dual approval NÃO entra
// aqui: uma ação que exige 4-eyes ainda é "permitida" (o servidor cuida do
// fluxo de aprovação no momento da execução).
func (p *Policy) AllowedActions(ctx context.Context, roles []string) ([]string, error) {
	if len(roles) == 0 {
		return []string{}, nil
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT DISTINCT action
		  FROM app.permissions
		 WHERE allowed AND role_code = ANY($1)
		 ORDER BY action`, roles)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Combine aplica a regra de combinação multi-role descrita no doc do pacote.
// Exposto para uso em testes e para PDPs que carreguem permissões em memória.
func Combine(perms []Permission) Decision {
	var (
		anyAllowed     bool
		anyNoDual      bool
		dualApprover   string
	)
	for _, p := range perms {
		if !p.Allowed {
			continue
		}
		anyAllowed = true
		if !p.RequiresDualApproval {
			anyNoDual = true
		} else if dualApprover == "" {
			dualApprover = p.ApproverRole
		}
	}
	if !anyAllowed {
		return Decision{}
	}
	if anyNoDual {
		return Decision{Allowed: true}
	}
	return Decision{Allowed: true, RequiresDualApproval: true, ApproverRole: dualApprover}
}
