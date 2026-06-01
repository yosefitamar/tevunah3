// Package informes gerencia os "informes" — captura rápida (quando/onde/como/
// descrição + foto opcional) que subsidia relatórios. Visibilidade é pool
// compartilhado gateado por clearance; posse (editar/excluir o próprio) é
// decidida na camada de handler.
package informes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotFound — informe inexistente (ou soft-deletado).
var ErrNotFound = errors.New("informe not found")

// Informe espelha uma linha de app.informes (com autor resolvido via join).
type Informe struct {
	ID                string
	OccurredOn        time.Time // só a data importa (coluna date)
	Location          string
	How               string
	Description       string
	PhotoPath         *string
	RequiredClearance int
	Version           int
	CreatedAt         time.Time
	CreatedBy         string
	CreatedByCode     string
	CreatedByName     string
	UpdatedAt         time.Time
	UpdatedBy         string
	DeletedAt         *time.Time
	DeletedBy         *string
}

// NewInforme é o input do Create.
type NewInforme struct {
	OccurredOn        time.Time
	Location          string
	How               string
	Description       string
	RequiredClearance int // 0 → default 1
	CreatedBy         string
}

// Patch descreve a edição (campos nil mantêm o valor atual).
type Patch struct {
	OccurredOn        *time.Time
	Location          *string
	How               *string
	Description       *string
	RequiredClearance *int
}

// ListOpts filtra/pagina a listagem. Clearance/IsAdmin vêm do middleware de auth.
type ListOpts struct {
	Limit     int
	Offset    int
	Search    string // ILIKE em location/how/description
	SortBy    string
	SortDir   string
	UserID    string
	Clearance int
	IsAdmin   bool
}

type ListResult struct {
	Items []Informe
	Total int
}

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

const selectFields = `
	i.id, i.occurred_on, i.location, i.how, i.description, i.photo_path,
	i.required_clearance, i.version,
	i.created_at, i.created_by, u.code, u.display_name,
	i.updated_at, i.updated_by, i.deleted_at, i.deleted_by`

func scan(row interface{ Scan(...any) error }) (*Informe, error) {
	var inf Informe
	var photo, deletedBy sql.NullString
	var deletedAt sql.NullTime
	if err := row.Scan(
		&inf.ID, &inf.OccurredOn, &inf.Location, &inf.How, &inf.Description, &photo,
		&inf.RequiredClearance, &inf.Version,
		&inf.CreatedAt, &inf.CreatedBy, &inf.CreatedByCode, &inf.CreatedByName,
		&inf.UpdatedAt, &inf.UpdatedBy, &deletedAt, &deletedBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if photo.Valid && photo.String != "" {
		s := photo.String
		inf.PhotoPath = &s
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		inf.DeletedAt = &t
	}
	if deletedBy.Valid {
		s := deletedBy.String
		inf.DeletedBy = &s
	}
	return &inf, nil
}

// FindByID devolve um informe (não soft-deletado). ErrNotFound se não existir.
func (r *Repo) FindByID(ctx context.Context, id string) (*Informe, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectFields+`
		  FROM app.informes i
		  JOIN app.users u ON u.id = i.created_by
		 WHERE i.id = $1 AND i.deleted_at IS NULL`, id)
	return scan(row)
}

func (r *Repo) Create(ctx context.Context, in NewInforme) (*Informe, error) {
	if in.OccurredOn.IsZero() {
		in.OccurredOn = time.Now()
	}
	if in.RequiredClearance == 0 {
		in.RequiredClearance = 1
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.informes
		  (occurred_on, location, how, description, required_clearance, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id`,
		in.OccurredOn, strings.TrimSpace(in.Location), strings.TrimSpace(in.How),
		strings.TrimSpace(in.Description), in.RequiredClearance, in.CreatedBy,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// informesOrderBy traduz (SortBy, SortDir) num ORDER BY seguro (whitelist).
func informesOrderBy(sortBy, sortDir string) string {
	dir := "DESC"
	if strings.EqualFold(sortDir, "asc") {
		dir = "ASC"
	}
	var ob string
	switch sortBy {
	case "location", "occurred_on", "required_clearance", "created_at":
		ob = "i." + sortBy + " " + dir
	case "author":
		ob = "u.display_name " + dir
	default:
		ob = "i.occurred_on DESC"
	}
	return ob + ", i.created_at DESC"
}

func (r *Repo) List(ctx context.Context, opts ListOpts) (*ListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 25
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	search := "%" + strings.ToLower(strings.TrimSpace(opts.Search)) + "%"

	// Predicado de acesso: admin vê tudo; senão autor OU clearance suficiente.
	countArgs := []any{opts.Search, search}
	listArgs := []any{opts.Search, search, opts.Limit, opts.Offset}
	countVis := "TRUE"
	listVis := "TRUE"
	if !opts.IsAdmin {
		countArgs = append(countArgs, opts.UserID, opts.Clearance)
		listArgs = append(listArgs, opts.UserID, opts.Clearance)
		countVis = `(i.created_by = $3 OR i.required_clearance <= $4)`
		listVis = `(i.created_by = $5 OR i.required_clearance <= $6)`
	}
	searchClause := `($1 = '' OR i.location ILIKE $2 OR i.how ILIKE $2 OR i.description ILIKE $2)`

	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM app.informes i
		 WHERE i.deleted_at IS NULL AND `+searchClause+` AND `+countVis,
		countArgs...,
	).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectFields+`
		  FROM app.informes i
		  JOIN app.users u ON u.id = i.created_by
		 WHERE i.deleted_at IS NULL AND `+searchClause+` AND `+listVis+`
		 ORDER BY `+informesOrderBy(opts.SortBy, opts.SortDir)+`
		 LIMIT $3 OFFSET $4`, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Informe, 0)
	for rows.Next() {
		inf, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *inf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total}, nil
}

// Update aplica o patch e bumpa a version. Devolve o estado depois.
func (r *Repo) Update(ctx context.Context, id string, p Patch, updatedBy string) (*Informe, error) {
	sets := []string{"updated_at = now()", "updated_by = $1", "version = version + 1"}
	args := []any{updatedBy}
	n := 1
	add := func(col string, val any) {
		n++
		sets = append(sets, fmt.Sprintf("%s = $%d", col, n))
		args = append(args, val)
	}
	if p.OccurredOn != nil {
		add("occurred_on", *p.OccurredOn)
	}
	if p.Location != nil {
		add("location", strings.TrimSpace(*p.Location))
	}
	if p.How != nil {
		add("how", strings.TrimSpace(*p.How))
	}
	if p.Description != nil {
		add("description", strings.TrimSpace(*p.Description))
	}
	if p.RequiredClearance != nil {
		add("required_clearance", *p.RequiredClearance)
	}
	n++
	args = append(args, id)
	res, err := r.db.ExecContext(ctx,
		`UPDATE app.informes SET `+strings.Join(sets, ", ")+
			fmt.Sprintf(" WHERE id = $%d AND deleted_at IS NULL", n), args...)
	if err != nil {
		return nil, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

// SoftDelete marca deleted_at/by. ErrNotFound se já não existir/estiver deletado.
func (r *Repo) SoftDelete(ctx context.Context, id, actor string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.informes
		   SET deleted_at = now(), deleted_by = $1, updated_at = now(), updated_by = $1
		 WHERE id = $2 AND deleted_at IS NULL`, actor, id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}

// SetPhotoPath grava (ou limpa, com "") o filename da foto e devolve o anterior
// para o caller remover o arquivo do disco.
func (r *Repo) SetPhotoPath(ctx context.Context, id, filename, updatedBy string) (oldPath string, err error) {
	var old sql.NullString
	var newVal any
	if filename == "" {
		newVal = nil
	} else {
		newVal = filename
	}
	err = r.db.QueryRowContext(ctx, `
		WITH prev AS (SELECT photo_path FROM app.informes WHERE id = $1 AND deleted_at IS NULL)
		UPDATE app.informes
		   SET photo_path = $2, updated_at = now(), updated_by = $3, version = version + 1
		 WHERE id = $1 AND deleted_at IS NULL
		RETURNING (SELECT photo_path FROM prev)`, id, newVal, updatedBy).Scan(&old)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if old.Valid {
		return old.String, nil
	}
	return "", nil
}

// CanAccess responde se o usuário pode ver o informe (mesma regra da List).
// ErrNotFound se o informe não existe.
func (r *Repo) CanAccess(ctx context.Context, id, userID string, clearance int, isAdmin bool) (bool, error) {
	existsSQL := `SELECT EXISTS(SELECT 1 FROM app.informes WHERE id = $1 AND deleted_at IS NULL)`
	if isAdmin {
		var exists bool
		if err := r.db.QueryRowContext(ctx, existsSQL, id).Scan(&exists); err != nil {
			return false, err
		}
		if !exists {
			return false, ErrNotFound
		}
		return true, nil
	}
	var ok bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM app.informes
			 WHERE id = $1 AND deleted_at IS NULL
			   AND (created_by = $2 OR required_clearance <= $3)
		)`, id, userID, clearance).Scan(&ok); err != nil {
		return false, err
	}
	if !ok {
		var exists bool
		if err := r.db.QueryRowContext(ctx, existsSQL, id).Scan(&exists); err != nil {
			return false, err
		}
		if !exists {
			return false, ErrNotFound
		}
	}
	return ok, nil
}
