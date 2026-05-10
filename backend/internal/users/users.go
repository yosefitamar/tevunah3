// Package users provê leitura/escrita do cadastro de agentes/usuários.
package users

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrNotFound indica que o usuário não foi localizado (ou está soft-deleted).
var ErrNotFound = errors.New("usuário não encontrado")

// User é a representação do agente no domínio app.
type User struct {
	ID             string
	Code           string
	Email          string
	DisplayName    string
	PasswordHash   string
	TOTPSecret     string
	ClearanceLevel int
	Status         string
	LastLoginAt    *time.Time
	Roles          []string // codenames (agente | analista | gestor | administrador)
}

// IsActive devolve true se o usuário está apto a logar.
func (u *User) IsActive() bool {
	return u.Status == "active"
}

// Repo encapsula queries sobre app.users e app.user_roles.
type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

const userSelect = `
  SELECT id, code, email, display_name, password_hash,
         COALESCE(totp_secret, ''), clearance_level, status, last_login_at
    FROM app.users
   WHERE %s
     AND deleted_at IS NULL`

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var totp sql.NullString
	var last sql.NullTime
	if err := row.Scan(&u.ID, &u.Code, &u.Email, &u.DisplayName, &u.PasswordHash,
		&totp, &u.ClearanceLevel, &u.Status, &last); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.TOTPSecret = totp.String
	if last.Valid {
		t := last.Time
		u.LastLoginAt = &t
	}
	return &u, nil
}

// FindByEmail busca um usuário ativo (não soft-deleted) por e-mail.
func (r *Repo) FindByEmail(ctx context.Context, email string) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, code, email, display_name, password_hash,
		        COALESCE(totp_secret, ''), clearance_level, status, last_login_at
		   FROM app.users
		  WHERE lower(email) = lower($1) AND deleted_at IS NULL`,
		strings.ToLower(email))
	u, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	if err := r.loadRoles(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// FindByID busca um usuário ativo pelo UUID.
func (r *Repo) FindByID(ctx context.Context, id string) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, code, email, display_name, password_hash,
		        COALESCE(totp_secret, ''), clearance_level, status, last_login_at
		   FROM app.users
		  WHERE id = $1 AND deleted_at IS NULL`,
		id)
	u, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	if err := r.loadRoles(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repo) loadRoles(ctx context.Context, u *User) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT role_code FROM app.user_roles WHERE user_id = $1`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return err
		}
		u.Roles = append(u.Roles, r)
	}
	return rows.Err()
}

// TouchLastLogin atualiza o carimbo de último login com now().
func (r *Repo) TouchLastLogin(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app.users SET last_login_at = now() WHERE id = $1`, id)
	return err
}
