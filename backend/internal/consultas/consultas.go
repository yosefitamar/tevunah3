// Package consultas guarda o registro das consultas que agentes e analistas
// fazem em sistemas externos a pedido de alguém: quando, em qual sistema, o
// que foi pesquisado, quem pediu, por quê e o que foi encontrado. É a
// justificativa da pesquisa quando o sistema consultado é auditado.
//
// Visibilidade: cada um vê os próprios registros; SeeAll (ação
// consulta.read_all) abre os de todos. A posse para editar/excluir é decidida
// na camada de handler.
package consultas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotFound — registro inexistente (ou soft-deletado).
var ErrNotFound = errors.New("consulta not found")

// Consulta espelha uma linha de app.consultas (com autor resolvido via join).
type Consulta struct {
	ID            string
	PerformedAt   time.Time
	System        string
	Target        string
	Requester     string
	RequesterOrg  string
	ReferenceDoc  string
	Reason        string
	Result        string
	Version       int
	CreatedAt     time.Time
	CreatedBy     string
	CreatedByCode string
	CreatedByName string
	UpdatedAt     time.Time
	UpdatedBy     string
}

// Fields são os campos que o agente preenche; valem para criar e editar.
type Fields struct {
	PerformedAt  time.Time
	System       string
	Target       string
	Requester    string
	RequesterOrg string
	ReferenceDoc string
	Reason       string
	Result       string
}

// normalize apara os textos e põe o sistema em caixa alta, para que o mesmo
// sistema não apareça com grafias diferentes nas sugestões e nos filtros.
func (f Fields) normalize() Fields {
	f.System = strings.ToUpper(strings.TrimSpace(f.System))
	f.Target = strings.TrimSpace(f.Target)
	f.Requester = strings.TrimSpace(f.Requester)
	f.RequesterOrg = strings.TrimSpace(f.RequesterOrg)
	f.ReferenceDoc = strings.TrimSpace(f.ReferenceDoc)
	f.Reason = strings.TrimSpace(f.Reason)
	f.Result = strings.TrimSpace(f.Result)
	return f
}

// ListOpts filtra/pagina a listagem.
type ListOpts struct {
	Limit    int
	Offset   int
	Search   string // ILIKE nos campos de texto (e no agente, com SeeAll)
	DateFrom string // YYYY-MM-DD no fuso TZ; vazio = sem limite
	DateTo   string
	TZ       string // fuso da agência, para o recorte por dia civil
	SortBy   string
	SortDir  string
	UserID   string
	SeeAll   bool
}

type ListResult struct {
	Items []Consulta
	Total int
}

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

const selectFields = `
	c.id, c.performed_at, c.system, c.target, c.requester, c.requester_org,
	c.reference_doc, c.reason, c.result, c.version,
	c.created_at, c.created_by, u.code, u.display_name,
	c.updated_at, c.updated_by`

func scan(row interface{ Scan(...any) error }) (*Consulta, error) {
	var c Consulta
	if err := row.Scan(
		&c.ID, &c.PerformedAt, &c.System, &c.Target, &c.Requester, &c.RequesterOrg,
		&c.ReferenceDoc, &c.Reason, &c.Result, &c.Version,
		&c.CreatedAt, &c.CreatedBy, &c.CreatedByCode, &c.CreatedByName,
		&c.UpdatedAt, &c.UpdatedBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// FindByID devolve um registro (não soft-deletado). ErrNotFound se não existir.
func (r *Repo) FindByID(ctx context.Context, id string) (*Consulta, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectFields+`
		  FROM app.consultas c
		  JOIN app.users u ON u.id = c.created_by
		 WHERE c.id = $1 AND c.deleted_at IS NULL`, id)
	return scan(row)
}

func (r *Repo) Create(ctx context.Context, in Fields, createdBy string) (*Consulta, error) {
	in = in.normalize()
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.consultas
		  (performed_at, system, target, requester, requester_org, reference_doc,
		   reason, result, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING id`,
		in.PerformedAt, in.System, in.Target, in.Requester, in.RequesterOrg,
		in.ReferenceDoc, in.Reason, in.Result, createdBy,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// Update regrava os campos e bumpa a version. Devolve o estado depois.
func (r *Repo) Update(ctx context.Context, id string, in Fields, updatedBy string) (*Consulta, error) {
	in = in.normalize()
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.consultas
		   SET performed_at = $1, system = $2, target = $3, requester = $4,
		       requester_org = $5, reference_doc = $6, reason = $7, result = $8,
		       updated_at = now(), updated_by = $9, version = version + 1
		 WHERE id = $10 AND deleted_at IS NULL`,
		in.PerformedAt, in.System, in.Target, in.Requester, in.RequesterOrg,
		in.ReferenceDoc, in.Reason, in.Result, updatedBy, id)
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
		UPDATE app.consultas
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

// orderBy traduz (SortBy, SortDir) num ORDER BY seguro (whitelist).
func orderBy(sortBy, sortDir string) string {
	dir := "DESC"
	if strings.EqualFold(sortDir, "asc") {
		dir = "ASC"
	}
	var ob string
	switch sortBy {
	case "performed_at", "system", "target", "requester", "created_at":
		ob = "c." + sortBy + " " + dir
	case "author":
		ob = "u.display_name " + dir
	default:
		ob = "c.performed_at DESC"
	}
	return ob + ", c.created_at DESC"
}

func (r *Repo) List(ctx context.Context, opts ListOpts) (*ListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 25
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.TZ == "" {
		opts.TZ = "UTC"
	}

	where := []string{"c.deleted_at IS NULL"}
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if !opts.SeeAll {
		where = append(where, "c.created_by = "+arg(opts.UserID))
	}
	if s := strings.TrimSpace(opts.Search); s != "" {
		p := arg("%" + s + "%")
		cols := []string{
			"c.system", "c.target", "c.requester", "c.requester_org",
			"c.reference_doc", "c.reason", "c.result",
		}
		if opts.SeeAll {
			cols = append(cols, "u.code", "u.display_name")
		}
		parts := make([]string, len(cols))
		for i, col := range cols {
			parts[i] = col + " ILIKE " + p
		}
		where = append(where, "("+strings.Join(parts, " OR ")+")")
	}
	if opts.DateFrom != "" || opts.DateTo != "" {
		day := "(c.performed_at AT TIME ZONE " + arg(opts.TZ) + ")::date"
		if opts.DateFrom != "" {
			where = append(where, day+" >= "+arg(opts.DateFrom)+"::date")
		}
		if opts.DateTo != "" {
			where = append(where, day+" <= "+arg(opts.DateTo)+"::date")
		}
	}

	from := `
		  FROM app.consultas c
		  JOIN app.users u ON u.id = c.created_by
		 WHERE ` + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, err
	}

	limit := arg(opts.Limit)
	offset := arg(opts.Offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectFields+from+`
		 ORDER BY `+orderBy(opts.SortBy, opts.SortDir)+`
		 LIMIT `+limit+` OFFSET `+offset, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Consulta, 0)
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total}, nil
}

// Systems devolve os sistemas já usados nos registros, dos mais frequentes
// para os menos — são as sugestões do campo SISTEMA. Considera os registros de
// todos os agentes: o nome do sistema não é dado sensível e a lista comum é o
// que mantém a grafia uniforme.
func (r *Repo) Systems(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT system
		  FROM app.consultas
		 WHERE deleted_at IS NULL AND system <> ''
		 GROUP BY system
		 ORDER BY COUNT(*) DESC, system
		 LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
