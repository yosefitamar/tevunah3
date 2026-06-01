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

// Role espelha uma linha de app.roles (papel).
type Role struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	IsSystem bool   `json:"is_system"`
}

// ErrRoleInUse — papel não pode ser excluído porque está em uso (atribuído a
// algum usuário ou referenciado como aprovador na matriz) ou é de sistema.
var ErrRoleInUse = errors.New("role in use or system")

// ErrRoleExists — codename já cadastrado.
var ErrRoleExists = errors.New("role already exists")

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// ListRoles devolve os papéis cadastrados, em ordem estável de criação.
func (r *Repo) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT codename, label, is_system FROM app.roles
		 ORDER BY created_at, codename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Role
	for rows.Next() {
		var rl Role
		if err := rows.Scan(&rl.Code, &rl.Label, &rl.IsSystem); err != nil {
			return nil, err
		}
		out = append(out, rl)
	}
	return out, rows.Err()
}

// RoleExists informa se um papel com esse codename existe.
func (r *Repo) RoleExists(ctx context.Context, code string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM app.roles WHERE codename = $1)`, code).Scan(&ok)
	return ok, err
}

// CreateRole cria um papel customizado (is_system=false). ErrRoleExists se o
// codename já estiver em uso.
func (r *Repo) CreateRole(ctx context.Context, code, label string) (*Role, error) {
	exists, err := r.RoleExists(ctx, code)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrRoleExists
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO app.roles (codename, label, is_system) VALUES ($1, $2, false)`,
		code, label)
	if err != nil {
		return nil, err
	}
	return &Role{Code: code, Label: label, IsSystem: false}, nil
}

// UpdateRoleLabel renomeia o label de um papel. ErrNotFound se não existir.
func (r *Repo) UpdateRoleLabel(ctx context.Context, code, label string) (*Role, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE app.roles SET label = $2 WHERE codename = $1`, code, label)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	var rl Role
	err = r.db.QueryRowContext(ctx,
		`SELECT codename, label, is_system FROM app.roles WHERE codename = $1`, code).
		Scan(&rl.Code, &rl.Label, &rl.IsSystem)
	if err != nil {
		return nil, err
	}
	return &rl, nil
}

// DeleteRole exclui um papel customizado. Recusa (ErrRoleInUse) se for de
// sistema, estiver atribuído a algum usuário ou for aprovador de alguma regra
// 4-eyes. As linhas da matriz desse papel são removidas junto, em transação.
func (r *Repo) DeleteRole(ctx context.Context, code string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var isSystem bool
	err = tx.QueryRowContext(ctx,
		`SELECT is_system FROM app.roles WHERE codename = $1`, code).Scan(&isSystem)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if isSystem {
		return ErrRoleInUse
	}

	var inUse bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM app.user_roles WHERE role_code = $1)
		    OR EXISTS (SELECT 1 FROM app.permissions WHERE approver_role = $1)`,
		code).Scan(&inUse)
	if err != nil {
		return err
	}
	if inUse {
		return ErrRoleInUse
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM app.permissions WHERE role_code = $1`, code); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM app.roles WHERE codename = $1`, code); err != nil {
		return err
	}
	return tx.Commit()
}

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

// GovernanceReachableAfter simula a mudança da célula (role, action) para
// (allowed, dual) e responde se, após ela, AINDA existe ao menos um usuário
// ativo capaz de executar `action` SEM aprovação dupla. É a base da guarda
// anti-lockout: impede que a última via de administração do RBAC seja removida
// pela própria matriz.
func (r *Repo) GovernanceReachableAfter(ctx context.Context, action, role string, allowed, dual bool) (bool, error) {
	// Papéis que hoje concedem a ação de forma irrestrita (allowed && !dual).
	rows, err := r.db.QueryContext(ctx, `
		SELECT role_code FROM app.permissions
		 WHERE action = $1 AND allowed AND NOT requires_dual_approval`, action)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	unguarded := map[string]bool{}
	for rows.Next() {
		var rc string
		if err := rows.Scan(&rc); err != nil {
			return false, err
		}
		unguarded[rc] = true
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	// Aplica a simulação sobre a célula editada.
	if allowed && !dual {
		unguarded[role] = true
	} else {
		delete(unguarded, role)
	}
	if len(unguarded) == 0 {
		return false, nil
	}

	codes := make([]string, 0, len(unguarded))
	for rc := range unguarded {
		codes = append(codes, rc)
	}

	// Existe usuário ativo (não soft-deletado) com algum desses papéis?
	var reachable bool
	err = r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
		  SELECT 1
		    FROM app.user_roles ur
		    JOIN app.users u ON u.id = ur.user_id
		   WHERE ur.role_code = ANY($1)
		     AND u.status = 'active'
		     AND u.deleted_at IS NULL
		)`, codes).Scan(&reachable)
	if err != nil {
		return false, err
	}
	return reachable, nil
}

// Upsert aplica a mudança criando a linha se ainda não existir e devolve o
// estado antes (nil se a linha não existia) e depois.
//
// A matriz é renderizada como grade cheia (papéis × catálogo de ações); muitas
// células não têm linha semeada. Editar uma dessas células precisa INSERIR, não
// só atualizar — por isso o upsert. O caller deve enviar o estado desejado
// completo (allowed/requires_dual_approval/approver_role).
func (r *Repo) Upsert(ctx context.Context, roleCode, action string, in UpdateInput) (before, after *Permission, err error) {
	before, err = r.Get(ctx, roleCode, action)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, nil, err
	}
	if errors.Is(err, ErrNotFound) {
		before = nil
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
		INSERT INTO app.permissions
		       (role_code, action, allowed, requires_dual_approval, approver_role, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, now(), $6)
		ON CONFLICT (role_code, action) DO UPDATE
		   SET allowed                = EXCLUDED.allowed,
		       requires_dual_approval = EXCLUDED.requires_dual_approval,
		       approver_role          = EXCLUDED.approver_role,
		       updated_at             = now(),
		       updated_by             = EXCLUDED.updated_by`,
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
