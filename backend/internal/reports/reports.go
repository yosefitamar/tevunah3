// Package reports gerencia a persistência e a máquina de estados dos
// relatórios (documentos de inteligência) do Tevunah.
//
// Tipos suportados (kind):
//   - "interno": Relatório Interno (primeiro tipo entregue)
//
// Máquina de estados:
//
//	criado  ──(diffuse)──▶  difundido  ──(archive)──▶  arquivado
//
// Apenas em "criado" o relatório é editável. A numeração (NN/AAAA) é
// alocada na transição criado→difundido, via sequência por ano — assim
// rascunhos descartados não consomem número.
package reports

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Erros de domínio.
var (
	ErrNotFound          = errors.New("relatório não encontrado")
	ErrInvalidStatus     = errors.New("transição de status inválida")
	ErrNotEditable       = errors.New("relatório só pode ser editado em status 'criado'")
	ErrNumberAllocation  = errors.New("falha ao alocar número do relatório")
	ErrMissingEntity     = errors.New("qualificação CIVIL exige entity_id")
	ErrUnexpectedEntity  = errors.New("qualificação MILITAR não aceita entity_id")
	ErrQualificationKind = errors.New("kind de qualificação inválido")
)

// Status / Kind constantes.
const (
	KindInterno = "interno"

	StatusCriado    = "criado"
	StatusDifundido = "difundido"
	StatusArquivado = "arquivado"

	QualMilitar = "militar"
	QualCivil   = "civil"
)

// Report representa um documento de inteligência.
type Report struct {
	ID     string
	Kind   string
	Status string

	// Número e ano são preenchidos na difusão. NULL antes disso.
	Seq  *int
	Year *int

	DocDate        time.Time
	Subject        string
	Origin         string
	Diffusion      string
	PriorDiffusion string
	Reference      string
	Attachments    string

	BodyHTML string

	CreatedAt   time.Time
	CreatedBy   string
	UpdatedAt   time.Time
	UpdatedBy   *string
	DiffusedAt  *time.Time
	DiffusedBy  *string
	ArchivedAt  *time.Time
	ArchivedBy  *string
}

// Number devolve a numeração formatada "NN/AAAA" ou "" se ainda em rascunho.
func (r *Report) Number() string {
	if r.Seq == nil || r.Year == nil {
		return ""
	}
	return fmt.Sprintf("%02d/%d", *r.Seq, *r.Year)
}

// Qualification é um bloco de qualificação (militar ou civil) embutido
// no relatório. Snapshot dos campos vai em Data (jsonb).
type Qualification struct {
	ID           string
	ReportID     string
	Ord          int
	Kind         string
	EntityID     *string
	Data         map[string]any
	Source       string
	ConsultedAt  *time.Time
	Licencas     []Licenca // populado por LoadLicencas; vazio pra civil
	CreatedAt    time.Time
}

// Licenca é uma linha da tabela de licenças (somente para militares).
type Licenca struct {
	ID              string
	QualificationID string
	Ord             int
	Boletim         string
	Unidade         string
	Publicacao      *time.Time
	DataInicio      *time.Time
	DataFim         *time.Time
	Dias            *int
	CID             string
}

// Repo encapsula queries sobre app.reports e tabelas filhas.
type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// ─── CRUD básico ───────────────────────────────────────────────────────

// NewReport é o input do Create.
type NewReport struct {
	Kind      string
	DocDate   time.Time
	Subject   string
	Origin    string
	Diffusion string
	CreatedBy string
}

func (r *Repo) Create(ctx context.Context, in NewReport) (*Report, error) {
	if in.Kind == "" {
		in.Kind = KindInterno
	}
	if in.DocDate.IsZero() {
		in.DocDate = time.Now()
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.reports
		  (kind, status, doc_date, subject, origin, diffusion, created_by, updated_by)
		VALUES ($1, 'criado', $2, $3, $4, $5, $6, $6)
		RETURNING id`,
		in.Kind, in.DocDate, in.Subject, in.Origin, in.Diffusion, in.CreatedBy,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

const reportSelectFields = `
	id, kind, status, seq, year, doc_date, subject, origin, diffusion,
	prior_diffusion, reference, attachments, body_html,
	created_at, created_by, updated_at, updated_by,
	diffused_at, diffused_by, archived_at, archived_by`

func scanReport(row *sql.Row) (*Report, error) {
	var r Report
	var seq, year sql.NullInt64
	var updatedBy, diffusedBy, archivedBy sql.NullString
	var diffusedAt, archivedAt sql.NullTime
	err := row.Scan(
		&r.ID, &r.Kind, &r.Status, &seq, &year, &r.DocDate, &r.Subject,
		&r.Origin, &r.Diffusion, &r.PriorDiffusion, &r.Reference, &r.Attachments,
		&r.BodyHTML, &r.CreatedAt, &r.CreatedBy, &r.UpdatedAt, &updatedBy,
		&diffusedAt, &diffusedBy, &archivedAt, &archivedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if seq.Valid {
		v := int(seq.Int64)
		r.Seq = &v
	}
	if year.Valid {
		v := int(year.Int64)
		r.Year = &v
	}
	if updatedBy.Valid {
		s := updatedBy.String
		r.UpdatedBy = &s
	}
	if diffusedAt.Valid {
		t := diffusedAt.Time
		r.DiffusedAt = &t
	}
	if diffusedBy.Valid {
		s := diffusedBy.String
		r.DiffusedBy = &s
	}
	if archivedAt.Valid {
		t := archivedAt.Time
		r.ArchivedAt = &t
	}
	if archivedBy.Valid {
		s := archivedBy.String
		r.ArchivedBy = &s
	}
	return &r, nil
}

func (r *Repo) FindByID(ctx context.Context, id string) (*Report, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+reportSelectFields+` FROM app.reports WHERE id = $1`, id)
	return scanReport(row)
}

// UpdateOpts permite update parcial — só campos não-nulos são gravados.
type UpdateOpts struct {
	DocDate        *time.Time
	Subject        *string
	Origin         *string
	Diffusion      *string
	PriorDiffusion *string
	Reference      *string
	Attachments    *string
	BodyHTML       *string
}

// Update aplica patch nos campos editáveis. Apenas permitido em status='criado'.
func (r *Repo) Update(ctx context.Context, id, updatedBy string, in UpdateOpts) (*Report, error) {
	cur, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cur.Status != StatusCriado {
		return nil, ErrNotEditable
	}

	sets := []string{"updated_at = now()", "updated_by = $2"}
	args := []any{id, updatedBy}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if in.DocDate != nil {
		add("doc_date", *in.DocDate)
	}
	if in.Subject != nil {
		add("subject", *in.Subject)
	}
	if in.Origin != nil {
		add("origin", *in.Origin)
	}
	if in.Diffusion != nil {
		add("diffusion", *in.Diffusion)
	}
	if in.PriorDiffusion != nil {
		add("prior_diffusion", *in.PriorDiffusion)
	}
	if in.Reference != nil {
		add("reference", *in.Reference)
	}
	if in.Attachments != nil {
		add("attachments", *in.Attachments)
	}
	if in.BodyHTML != nil {
		add("body_html", *in.BodyHTML)
	}
	if len(sets) == 2 {
		// Só os defaults updated_at/updated_by — nada de fato pra atualizar.
		return cur, nil
	}
	query := "UPDATE app.reports SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// ─── Status machine ────────────────────────────────────────────────────

// Diffuse aloca o número da sequência do ano corrente e marca status como
// 'difundido'. Idempotente em caso de já difundido (devolve ErrInvalidStatus).
//
// A alocação de seq usa SELECT FOR UPDATE no max(seq) do ano em uma tx pra
// evitar race entre dois admins difundindo simultaneamente.
func (r *Repo) Diffuse(ctx context.Context, id, actor string) (*Report, error) {
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

	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM app.reports WHERE id = $1 FOR UPDATE`, id,
	).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status != StatusCriado {
		return nil, ErrInvalidStatus
	}

	year := time.Now().Year()
	var nextSeq int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM app.reports WHERE year = $1`, year,
	).Scan(&nextSeq); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNumberAllocation, err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE app.reports
		   SET status = 'difundido',
		       seq    = $1,
		       year   = $2,
		       diffused_at = now(),
		       diffused_by = $3,
		       updated_at  = now(),
		       updated_by  = $3
		 WHERE id = $4`,
		nextSeq, year, actor, id,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rollback = false
	return r.FindByID(ctx, id)
}

// Archive transita 'difundido' → 'arquivado'. Não realoca número.
func (r *Repo) Archive(ctx context.Context, id, actor string) (*Report, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.reports
		   SET status      = 'arquivado',
		       archived_at = now(),
		       archived_by = $1,
		       updated_at  = now(),
		       updated_by  = $1
		 WHERE id = $2 AND status = 'difundido'`,
		actor, id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		cur, err := r.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if cur.Status == StatusArquivado {
			return cur, nil // idempotente
		}
		return nil, ErrInvalidStatus
	}
	return r.FindByID(ctx, id)
}

// ─── Listagem ──────────────────────────────────────────────────────────

type ListOpts struct {
	Limit  int
	Offset int
	Status string // "", "criado", "difundido", "arquivado"
	Search string // substring em subject (ILIKE)
}

type ListResult struct {
	Items []Report
	Total int
}

func (r *Repo) List(ctx context.Context, opts ListOpts) (*ListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 25
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	search := "%" + opts.Search + "%"

	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM app.reports
		 WHERE ($1 = '' OR status = $1)
		   AND ($2 = '' OR subject ILIKE $3)`,
		opts.Status, opts.Search, search,
	).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT `+reportSelectFields+`
		  FROM app.reports
		 WHERE ($1 = '' OR status = $1)
		   AND ($2 = '' OR subject ILIKE $3)
		 ORDER BY doc_date DESC, created_at DESC
		 LIMIT $4 OFFSET $5`,
		opts.Status, opts.Search, search, opts.Limit, opts.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Report, 0)
	for rows.Next() {
		var rep Report
		var seq, year sql.NullInt64
		var updatedBy, diffusedBy, archivedBy sql.NullString
		var diffusedAt, archivedAt sql.NullTime
		if err := rows.Scan(
			&rep.ID, &rep.Kind, &rep.Status, &seq, &year, &rep.DocDate,
			&rep.Subject, &rep.Origin, &rep.Diffusion, &rep.PriorDiffusion,
			&rep.Reference, &rep.Attachments, &rep.BodyHTML,
			&rep.CreatedAt, &rep.CreatedBy, &rep.UpdatedAt, &updatedBy,
			&diffusedAt, &diffusedBy, &archivedAt, &archivedBy,
		); err != nil {
			return nil, err
		}
		if seq.Valid {
			v := int(seq.Int64)
			rep.Seq = &v
		}
		if year.Valid {
			v := int(year.Int64)
			rep.Year = &v
		}
		if updatedBy.Valid {
			s := updatedBy.String
			rep.UpdatedBy = &s
		}
		if diffusedAt.Valid {
			t := diffusedAt.Time
			rep.DiffusedAt = &t
		}
		if diffusedBy.Valid {
			s := diffusedBy.String
			rep.DiffusedBy = &s
		}
		if archivedAt.Valid {
			t := archivedAt.Time
			rep.ArchivedAt = &t
		}
		if archivedBy.Valid {
			s := archivedBy.String
			rep.ArchivedBy = &s
		}
		items = append(items, rep)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total}, nil
}

// ─── Qualifications ────────────────────────────────────────────────────

type NewQualification struct {
	ReportID    string
	Kind        string
	EntityID    *string
	Data        map[string]any
	Source      string
	ConsultedAt *time.Time
}

func (r *Repo) AddQualification(ctx context.Context, in NewQualification) (*Qualification, error) {
	if in.Kind != QualMilitar && in.Kind != QualCivil {
		return nil, ErrQualificationKind
	}
	if in.Kind == QualCivil && (in.EntityID == nil || *in.EntityID == "") {
		return nil, ErrMissingEntity
	}
	if in.Kind == QualMilitar && in.EntityID != nil && *in.EntityID != "" {
		return nil, ErrUnexpectedEntity
	}

	// ord = max + 1 entre as qualificações do relatório.
	var ord int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(ord), -1) + 1
		   FROM app.report_qualifications WHERE report_id = $1`,
		in.ReportID,
	).Scan(&ord); err != nil {
		return nil, err
	}

	dataJSON, err := json.Marshal(in.Data)
	if err != nil {
		return nil, err
	}
	if string(dataJSON) == "null" {
		dataJSON = []byte("{}")
	}

	var id string
	var entityArg any
	if in.EntityID != nil && *in.EntityID != "" {
		entityArg = *in.EntityID
	}
	if err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.report_qualifications
		  (report_id, ord, kind, entity_id, data, source, consulted_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
		RETURNING id`,
		in.ReportID, ord, in.Kind, entityArg, string(dataJSON),
		in.Source, in.ConsultedAt,
	).Scan(&id); err != nil {
		return nil, err
	}
	return r.FindQualification(ctx, id)
}

func (r *Repo) FindQualification(ctx context.Context, id string) (*Qualification, error) {
	var q Qualification
	var entityID sql.NullString
	var consultedAt sql.NullTime
	var dataJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, report_id, ord, kind, entity_id, data, source, consulted_at, created_at
		  FROM app.report_qualifications WHERE id = $1`, id,
	).Scan(&q.ID, &q.ReportID, &q.Ord, &q.Kind, &entityID, &dataJSON,
		&q.Source, &consultedAt, &q.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if entityID.Valid {
		s := entityID.String
		q.EntityID = &s
	}
	if consultedAt.Valid {
		t := consultedAt.Time
		q.ConsultedAt = &t
	}
	q.Data = map[string]any{}
	if len(dataJSON) > 0 {
		_ = json.Unmarshal(dataJSON, &q.Data)
	}
	return &q, nil
}

func (r *Repo) ListQualifications(ctx context.Context, reportID string) ([]Qualification, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, report_id, ord, kind, entity_id, data, source, consulted_at, created_at
		  FROM app.report_qualifications
		 WHERE report_id = $1
		 ORDER BY ord ASC`, reportID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Qualification, 0)
	for rows.Next() {
		var q Qualification
		var entityID sql.NullString
		var consultedAt sql.NullTime
		var dataJSON []byte
		if err := rows.Scan(&q.ID, &q.ReportID, &q.Ord, &q.Kind, &entityID,
			&dataJSON, &q.Source, &consultedAt, &q.CreatedAt); err != nil {
			return nil, err
		}
		if entityID.Valid {
			s := entityID.String
			q.EntityID = &s
		}
		if consultedAt.Valid {
			t := consultedAt.Time
			q.ConsultedAt = &t
		}
		q.Data = map[string]any{}
		if len(dataJSON) > 0 {
			_ = json.Unmarshal(dataJSON, &q.Data)
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *Repo) DeleteQualification(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM app.report_qualifications WHERE id = $1`, id)
	return err
}

// ─── Downloads (forense) ───────────────────────────────────────────────

type NewDownload struct {
	ReportID         string
	UserID           string
	SessionTokenHash string
	IP               string
	UserAgent        string
	PDFSha256        string
}

func (r *Repo) RecordDownload(ctx context.Context, in NewDownload) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.report_downloads
		  (report_id, user_id, session_token_hash, ip, user_agent, pdf_sha256)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		in.ReportID, in.UserID, in.SessionTokenHash, in.IP, in.UserAgent, in.PDFSha256,
	).Scan(&id)
	return id, err
}
