// Package users provê leitura/escrita do cadastro de agentes/usuários.
package users

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotFound indica que o usuário não foi localizado (ou está soft-deleted).
var ErrNotFound = errors.New("usuário não encontrado")

// ErrDuplicate indica que e-mail ou código já estão em uso.
var ErrDuplicate = errors.New("e-mail ou código já cadastrado")

// ErrAlreadyInactive indica que o usuário já não está ativo (não pode ser desativado de novo).
var ErrAlreadyInactive = errors.New("usuário já inativo")

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
	CreatedAt      time.Time
	Roles          []string // codenames (agente | analista | gestor | administrador)
}

// IsActive devolve true se o usuário está apto a logar.
func (u *User) IsActive() bool {
	return u.Status == "active"
}

// NewUser são os dados de entrada para Repo.Create.
// Espera password já hasheado (Argon2id PHC) e totp_secret já gerado.
type NewUser struct {
	Code           string
	Email          string
	DisplayName    string
	PasswordHash   string
	TOTPSecret     string
	ClearanceLevel int
	Roles          []string
	CreatedBy      *string // UUID do criador; nil em bootstrap
}

// ListOpts controla a listagem.
type ListOpts struct {
	Limit     int    // <= 100; default 25
	Offset    int    // default 0
	Status    string // "", "active", "suspended", "deactivated"
	Role      string // codename, vazio = todos
	Clearance int    // 0 = todos, 1..5 filtra exato
	Search    string // substring em código/email/nome (ILIKE)
}

// ListResult agrupa página + total.
type ListResult struct {
	Items []User
	Total int
}

// Repo encapsula queries sobre app.users e app.user_roles.
type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var totp sql.NullString
	var last sql.NullTime
	if err := row.Scan(&u.ID, &u.Code, &u.Email, &u.DisplayName, &u.PasswordHash,
		&totp, &u.ClearanceLevel, &u.Status, &last, &u.CreatedAt); err != nil {
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

const userSelectFields = `
	id, code, email, display_name, password_hash,
	COALESCE(totp_secret, ''), clearance_level, status, last_login_at, created_at`

// FindByEmail busca um usuário ativo (não soft-deleted) por e-mail.
func (r *Repo) FindByEmail(ctx context.Context, email string) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userSelectFields+`
		   FROM app.users
		  WHERE lower(email) = lower($1)`,
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
		`SELECT `+userSelectFields+`
		   FROM app.users
		  WHERE id = $1`,
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
		`SELECT role_code FROM app.user_roles WHERE user_id = $1 ORDER BY role_code`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rc string
		if err := rows.Scan(&rc); err != nil {
			return err
		}
		u.Roles = append(u.Roles, rc)
	}
	return rows.Err()
}

// TouchLastLogin atualiza o carimbo de último login com now().
func (r *Repo) TouchLastLogin(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app.users SET last_login_at = now() WHERE id = $1`, id)
	return err
}

// SetRoles substitui o conjunto de papéis de um usuário em uma transação.
// Recebe os codenames já validados; remove os atuais e insere os novos.
func (r *Repo) SetRoles(ctx context.Context, userID string, roles []string, assignedBy *string) error {
	if len(roles) == 0 {
		return errors.New("ao menos um papel é obrigatório")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx,
		`DELETE FROM app.user_roles WHERE user_id = $1`, userID,
	); err != nil {
		return err
	}
	for _, role := range roles {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO app.user_roles (user_id, role_code, assigned_by)
			VALUES ($1, $2, $3)`,
			userID, role, nullableUUID(assignedBy),
		); err != nil {
			return fmt.Errorf("link role %s: %w", role, err)
		}
	}
	if _, err = tx.ExecContext(ctx,
		`UPDATE app.users SET updated_at = now() WHERE id = $1`, userID,
	); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	rollback = false
	return nil
}

// SetClearance atualiza o nível de clearance (1..5).
func (r *Repo) SetClearance(ctx context.Context, userID string, level int) error {
	if level < 1 || level > 5 {
		return errors.New("clearance_level deve estar entre 1 e 5")
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE app.users SET clearance_level = $1, updated_at = now()
		  WHERE id = $2`,
		level, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Deactivate marca o usuário como desativado (soft delete via deleted_at + status).
// Hard delete não existe no sistema. Devolve o usuário ANTES da mudança para audit.
func (r *Repo) Deactivate(ctx context.Context, id string) (before *User, err error) {
	before, err = r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE app.users
		    SET status      = 'deactivated',
		        deleted_at  = COALESCE(deleted_at, now()),
		        updated_at  = now()
		  WHERE id = $1 AND status = 'active'`,
		id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrAlreadyInactive
	}
	return before, nil
}

// UpdateDisplayName atualiza apenas o nome de exibição (campo livre do próprio usuário).
func (r *Repo) UpdateDisplayName(ctx context.Context, id, displayName string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE app.users
		    SET display_name = $1, updated_at = now()
		  WHERE id = $2`,
		displayName, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GenerateCode devolve uma string "NNNN" (4 dígitos) única em app.users,
// gerada com crypto/rand. Tenta até 20 vezes; falha se não conseguir.
func (r *Repo) GenerateCode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		var b [2]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		n := binary.BigEndian.Uint16(b[:]) % 10000
		code := fmt.Sprintf("%04d", n)
		var exists bool
		if err := r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM app.users WHERE code = $1)`, code,
		).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("não foi possível gerar código único após 20 tentativas")
}

// Create insere usuário + papéis em transação. Espera in.PasswordHash já calculado
// e in.TOTPSecret já gerado. Devolve o usuário completo (com roles) recém-criado.
func (r *Repo) Create(ctx context.Context, in NewUser) (*User, error) {
	if in.Email == "" || in.DisplayName == "" || in.PasswordHash == "" || in.Code == "" {
		return nil, errors.New("campos obrigatórios: code, email, display_name, password_hash")
	}
	if len(in.Roles) == 0 {
		return nil, errors.New("ao menos um papel é obrigatório")
	}
	if in.ClearanceLevel < 1 || in.ClearanceLevel > 5 {
		return nil, errors.New("clearance_level deve estar entre 1 e 5")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO app.users
		  (code, email, display_name, password_hash, totp_secret,
		   clearance_level, status, created_by)
		VALUES ($1, lower($2), $3, $4, $5, $6, 'active', $7)
		RETURNING id`,
		in.Code, in.Email, in.DisplayName, in.PasswordHash, in.TOTPSecret,
		in.ClearanceLevel, nullableUUID(in.CreatedBy),
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	for _, role := range in.Roles {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO app.user_roles (user_id, role_code, assigned_by)
			VALUES ($1, $2, $3)`,
			id, role, nullableUUID(in.CreatedBy),
		); err != nil {
			return nil, fmt.Errorf("link role %s: %w", role, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	rollback = false

	return r.FindByID(ctx, id)
}

// List retorna uma página de usuários conforme opts.
func (r *Repo) List(ctx context.Context, opts ListOpts) (*ListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 25
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	search := "%" + opts.Search + "%"

	// Total
	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM app.users u
		 WHERE ($1 = '' OR u.status = $1)
		   AND ($2 = '' OR EXISTS (SELECT 1 FROM app.user_roles ur
		                            WHERE ur.user_id = u.id AND ur.role_code = $2))
		   AND ($3 = 0  OR u.clearance_level = $3)
		   AND ($4 = '' OR u.email ILIKE $5 OR u.display_name ILIKE $5 OR u.code ILIKE $5)`,
		opts.Status, opts.Role, opts.Clearance, opts.Search, search,
	).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.code, u.email, u.display_name, u.clearance_level, u.status,
		       u.last_login_at, u.created_at,
		       COALESCE(string_agg(ur.role_code, ',' ORDER BY ur.role_code), '') AS roles_csv
		  FROM app.users u
		  LEFT JOIN app.user_roles ur ON ur.user_id = u.id
		 WHERE ($1 = '' OR u.status = $1)
		   AND ($2 = '' OR EXISTS (SELECT 1 FROM app.user_roles ur2
		                            WHERE ur2.user_id = u.id AND ur2.role_code = $2))
		   AND ($3 = 0  OR u.clearance_level = $3)
		   AND ($4 = '' OR u.email ILIKE $5 OR u.display_name ILIKE $5 OR u.code ILIKE $5)
		 GROUP BY u.id
		 ORDER BY u.code
		 LIMIT $6 OFFSET $7`,
		opts.Status, opts.Role, opts.Clearance, opts.Search, search, opts.Limit, opts.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]User, 0)
	for rows.Next() {
		var u User
		var last sql.NullTime
		var rolesCSV string
		if err := rows.Scan(&u.ID, &u.Code, &u.Email, &u.DisplayName,
			&u.ClearanceLevel, &u.Status, &last, &u.CreatedAt, &rolesCSV); err != nil {
			return nil, err
		}
		if last.Valid {
			t := last.Time
			u.LastLoginAt = &t
		}
		if rolesCSV != "" {
			u.Roles = strings.Split(rolesCSV, ",")
		} else {
			u.Roles = []string{}
		}
		items = append(items, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total}, nil
}

func nullableUUID(p *string) any {
	if p == nil || *p == "" {
		return nil
	}
	return *p
}

// isUniqueViolation detecta erros 23505 do Postgres.
// (pgx encapsula em string; checagem por substring é resiliente entre versões.)
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key")
}

