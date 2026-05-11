// Package permissions gerencia a matriz RBAC (tabela app.permissions).
//
// Esta camada é distinta de internal/authz: authz lê a matriz para decidir
// uma ação específica em request time; permissions edita a matriz como dado.
package permissions

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrNotFound — combinação (role_code, action) não existe na matriz.
var ErrNotFound = errors.New("permission row not found")

// Permission espelha uma linha de app.permissions.
type Permission struct {
	RoleCode             string     `json:"role_code"`
	Action               string     `json:"action"`
	Allowed              bool       `json:"allowed"`
	RequiresDualApproval bool       `json:"requires_dual_approval"`
	ApproverRole         *string    `json:"approver_role,omitempty"`
	UpdatedAt            time.Time  `json:"updated_at"`
	UpdatedBy            *string    `json:"updated_by,omitempty"`
}

// UpdateInput descreve a atualização de uma linha. Todos os campos são
// aplicados; o caller deve enviar o valor desejado (não usar PATCH parcial).
type UpdateInput struct {
	Allowed              bool
	RequiresDualApproval bool
	ApproverRole         *string // NULL se não houver
	UpdatedBy            *string // ator que está aplicando a mudança
}

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// List devolve todas as linhas da matriz, ordenadas por (action, role_code)
// para a UI exibir agrupado por ação.
func (r *Repo) List(ctx context.Context) ([]Permission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT role_code, action, allowed, requires_dual_approval, approver_role,
		       updated_at, updated_by::text
		  FROM app.permissions
		 ORDER BY action, role_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(
			&p.RoleCode, &p.Action, &p.Allowed, &p.RequiresDualApproval,
			&p.ApproverRole, &p.UpdatedAt, &p.UpdatedBy,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get devolve uma linha específica. ErrNotFound se não existir.
func (r *Repo) Get(ctx context.Context, roleCode, action string) (*Permission, error) {
	var p Permission
	err := r.db.QueryRowContext(ctx, `
		SELECT role_code, action, allowed, requires_dual_approval, approver_role,
		       updated_at, updated_by::text
		  FROM app.permissions
		 WHERE role_code = $1 AND action = $2`, roleCode, action).Scan(
		&p.RoleCode, &p.Action, &p.Allowed, &p.RequiresDualApproval,
		&p.ApproverRole, &p.UpdatedAt, &p.UpdatedBy,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Update aplica a mudança e devolve o estado antes (para audit) e depois.
func (r *Repo) Update(ctx context.Context, roleCode, action string, in UpdateInput) (before, after *Permission, err error) {
	before, err = r.Get(ctx, roleCode, action)
	if err != nil {
		return nil, nil, err
	}

	var approverArg any
	if in.ApproverRole != nil && *in.ApproverRole != "" {
		approverArg = *in.ApproverRole
	} else {
		approverArg = nil
	}
	var byArg any
	if in.UpdatedBy != nil && *in.UpdatedBy != "" {
		byArg = *in.UpdatedBy
	} else {
		byArg = nil
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE app.permissions
		   SET allowed                = $3,
		       requires_dual_approval = $4,
		       approver_role          = $5,
		       updated_at             = now(),
		       updated_by             = $6
		 WHERE role_code = $1 AND action = $2`,
		roleCode, action, in.Allowed, in.RequiresDualApproval, approverArg, byArg)
	if err != nil {
		return nil, nil, err
	}

	after, err = r.Get(ctx, roleCode, action)
	if err != nil {
		return nil, nil, err
	}
	return before, after, nil
}
