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

	ConfidentialitySigiloso      = "sigiloso"
	ConfidentialitySecreto       = "secreto"
	ConfidentialityUltrassecreto = "ultrassecreto"

	VisibilityAberto   = "aberto"
	VisibilityRestrito = "restrito"
)

// IsValidConfidentiality valida o valor recebido no payload.
func IsValidConfidentiality(s string) bool {
	switch s {
	case ConfidentialitySigiloso, ConfidentialitySecreto, ConfidentialityUltrassecreto:
		return true
	}
	return false
}

// IsValidVisibility valida o valor recebido no payload.
func IsValidVisibility(s string) bool {
	return s == VisibilityAberto || s == VisibilityRestrito
}

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

	Confidentiality string // sigiloso | secreto | ultrassecreto (sigilo legal/LAI)
	Visibility      string // aberto | restrito
	// Nível de ACESSO: clearance mínimo do usuário para ver o RI (1..5).
	// Ortogonal à confidentiality. Ver migration 00035.
	RequiredClearance int

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
	Kind              string
	DocDate           time.Time
	Subject           string
	Origin            string
	Diffusion         string
	Confidentiality   string // se vazio usa default da coluna ('secreto')
	RequiredClearance int    // 0 → default 1 (visível a qualquer clearance)
	CreatedBy         string
}

func (r *Repo) Create(ctx context.Context, in NewReport) (*Report, error) {
	if in.Kind == "" {
		in.Kind = KindInterno
	}
	if in.DocDate.IsZero() {
		in.DocDate = time.Now()
	}
	if in.Confidentiality == "" {
		in.Confidentiality = ConfidentialitySecreto
	}
	if in.RequiredClearance == 0 {
		in.RequiredClearance = 1
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.reports
		  (kind, status, doc_date, subject, origin, diffusion, confidentiality,
		   required_clearance, created_by, updated_by)
		VALUES ($1, 'criado', $2, $3, $4, $5, $6, $7, $8, $8)
		RETURNING id`,
		in.Kind, in.DocDate, in.Subject, in.Origin, in.Diffusion,
		in.Confidentiality, in.RequiredClearance, in.CreatedBy,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

const reportSelectFields = `
	id, kind, status, seq, year, doc_date, subject, origin, diffusion,
	prior_diffusion, reference, attachments, confidentiality, visibility,
	required_clearance, body_html,
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
		&r.Confidentiality, &r.Visibility, &r.RequiredClearance,
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
		`SELECT `+reportSelectFields+` FROM app.reports
		  WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanReport(row)
}

// UpdateOpts permite update parcial — só campos não-nulos são gravados.
// Visibility e viewers NÃO entram aqui; têm endpoint próprio (SetVisibility,
// SetViewers) porque podem ser alterados a qualquer status pelo autor/admin,
// enquanto Update segue a regra "só em status=criado".
type UpdateOpts struct {
	DocDate         *time.Time
	Subject         *string
	Origin          *string
	Diffusion       *string
	PriorDiffusion  *string
	Reference       *string
	Attachments     *string
	Confidentiality *string
	BodyHTML        *string
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
	if in.Confidentiality != nil {
		add("confidentiality", *in.Confidentiality)
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
	var curSeq, curYear sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT status, seq, year FROM app.reports WHERE id = $1 FOR UPDATE`, id,
	).Scan(&status, &curSeq, &curYear); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status != StatusCriado {
		return nil, ErrInvalidStatus
	}

	// Re-difusão pós-undiffuse: preserva o número original (seq/year já gravados)
	// pra não desperdiçar/realocar. Difusão inédita: aloca próximo seq do ano.
	if curSeq.Valid && curYear.Valid {
		if _, err := tx.ExecContext(ctx, `
			UPDATE app.reports
			   SET status = 'difundido',
			       diffused_at = now(),
			       diffused_by = $1,
			       updated_at  = now(),
			       updated_by  = $1
			 WHERE id = $2`,
			actor, id,
		); err != nil {
			return nil, err
		}
	} else {
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
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rollback = false
	return r.FindByID(ctx, id)
}

// Undiffuse reverte a difusão: status volta a 'criado' e diffused_at/by ficam
// nulos. seq/year/number são preservados pra não criar gaps na numeração — se
// o relatório for difundido novamente, Diffuse reaproveita o seq existente.
// Só é permitido a partir de 'difundido'.
func (r *Repo) Undiffuse(ctx context.Context, id, actor string) (*Report, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.reports
		   SET status      = 'criado',
		       diffused_at = NULL,
		       diffused_by = NULL,
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
		cur, ferr := r.FindByID(ctx, id)
		if ferr != nil {
			return nil, ferr
		}
		if cur.Status != StatusDifundido {
			return nil, ErrInvalidStatus
		}
		return nil, ErrInvalidStatus
	}
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

// ListYears devolve os anos distintos (year IS NOT NULL) que têm relatórios
// visíveis ao usuário, em ordem desc. Admin vê todos; senão filtra pelo
// mesmo predicado de visibilidade do List.
func (r *Repo) ListYears(ctx context.Context, userID string, isAdmin bool) ([]int, error) {
	vis := "TRUE"
	args := []any{}
	if !isAdmin {
		vis = `(
			r.visibility = 'aberto'
			OR r.created_by = $1
			OR EXISTS (SELECT 1 FROM app.report_viewers v WHERE v.report_id = r.id AND v.user_id = $1)
		)`
		args = append(args, userID)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT r.year
		  FROM app.reports r
		 WHERE r.deleted_at IS NULL
		   AND r.year IS NOT NULL
		   AND `+vis+`
		 ORDER BY r.year DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int, 0)
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			return nil, err
		}
		out = append(out, y)
	}
	return out, rows.Err()
}

// ─── Listagem ──────────────────────────────────────────────────────────

type ListOpts struct {
	Limit  int
	Offset int
	Status string // "", "criado", "difundido", "arquivado"
	Search string // substring em subject (ILIKE)
	Year   int    // 0 = todos os anos; >0 = filtra year exato (inclui rascunhos com year)

	// Filtro de visibilidade (obrigatório nos endpoints HTTP — vem do middleware
	// de auth). Se IsAdmin=true, ignora o filtro e devolve todos os relatórios.
	// Senão, devolve apenas (autor sempre vê o próprio; demais precisam de
	// visibilidade E clearance suficiente):
	//   created_by=UserID
	//   OR ((visibility='aberto' OR é viewer) AND required_clearance <= Clearance)
	UserID    string
	Clearance int
	IsAdmin   bool

	// Ordenação. SortBy aceita: number|status|doc_date|subject|confidentiality|
	// diffusion|updated_at (default doc_date). SortDir: asc|desc (default desc).
	SortBy  string
	SortDir string
}

// reportsOrderBy traduz (SortBy, SortDir) num ORDER BY seguro (whitelist). O
// switch é a própria whitelist — entradas desconhecidas caem no default.
func reportsOrderBy(sortBy, sortDir string) string {
	dir := "DESC"
	if strings.EqualFold(sortDir, "asc") {
		dir = "ASC"
	}
	var ob string
	switch sortBy {
	case "number":
		ob = "r.year " + dir + ", r.seq " + dir
	case "status", "subject", "confidentiality", "diffusion", "doc_date", "updated_at":
		ob = "r." + sortBy + " " + dir
	default:
		ob = "r.doc_date DESC"
	}
	return ob + ", r.created_at DESC"
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

	// Predicado de visibilidade — true pra admin (vê tudo), senão filtra
	// pelo autor + report_viewers. Placeholders diferentes pra count ($5) e
	// list ($7, depois do limit/offset), por isso duas strings.
	countArgs := []any{opts.Status, opts.Search, search, opts.Year}
	listArgs := []any{opts.Status, opts.Search, search, opts.Year, opts.Limit, opts.Offset}
	countVis := "TRUE"
	listVis := "TRUE"
	if !opts.IsAdmin {
		// Autor sempre vê o próprio; demais precisam de visibilidade E clearance.
		countArgs = append(countArgs, opts.UserID, opts.Clearance)
		listArgs = append(listArgs, opts.UserID, opts.Clearance)
		countVis = `(
			r.created_by = $5
			OR (
			     (r.visibility = 'aberto'
			      OR EXISTS (SELECT 1 FROM app.report_viewers v WHERE v.report_id = r.id AND v.user_id = $5))
			     AND r.required_clearance <= $6
			)
		)`
		listVis = `(
			r.created_by = $7
			OR (
			     (r.visibility = 'aberto'
			      OR EXISTS (SELECT 1 FROM app.report_viewers v WHERE v.report_id = r.id AND v.user_id = $7))
			     AND r.required_clearance <= $8
			)
		)`
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM app.reports r
		 WHERE r.deleted_at IS NULL
		   AND ($1 = '' OR r.status = $1)
		   AND ($2 = '' OR r.subject ILIKE $3)
		   AND ($4 = 0  OR r.year = $4)
		   AND `+countVis,
		countArgs...,
	).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT `+reportSelectFields+`
		  FROM app.reports r
		 WHERE r.deleted_at IS NULL
		   AND ($1 = '' OR r.status = $1)
		   AND ($2 = '' OR r.subject ILIKE $3)
		   AND ($4 = 0  OR r.year = $4)
		   AND `+listVis+`
		 ORDER BY `+reportsOrderBy(opts.SortBy, opts.SortDir)+`
		 LIMIT $5 OFFSET $6`, listArgs...)
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
			&rep.Reference, &rep.Attachments,
			&rep.Confidentiality, &rep.Visibility, &rep.RequiredClearance, &rep.BodyHTML,
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

// SetQualificationPhotoPath grava (ou limpa, se newPath == "") o campo
// data.photo_path da qualificação. Devolve o filename anterior (pra remoção
// no filesystem quando trocamos por outra extensão).
func (r *Repo) SetQualificationPhotoPath(ctx context.Context, id, newPath string) (string, error) {
	var oldPath sql.NullString
	if err := r.db.QueryRowContext(ctx,
		`SELECT data->>'photo_path' FROM app.report_qualifications WHERE id = $1`,
		id,
	).Scan(&oldPath); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if newPath == "" {
		_, err := r.db.ExecContext(ctx,
			`UPDATE app.report_qualifications
			    SET data = data - 'photo_path'
			  WHERE id = $1`, id)
		if err != nil {
			return "", err
		}
	} else {
		_, err := r.db.ExecContext(ctx,
			`UPDATE app.report_qualifications
			    SET data = jsonb_set(COALESCE(data, '{}'::jsonb), '{photo_path}', to_jsonb($2::text), true)
			  WHERE id = $1`, id, newPath)
		if err != nil {
			return "", err
		}
	}
	if oldPath.Valid {
		return oldPath.String, nil
	}
	return "", nil
}

// FindQualificationReport devolve o report ID e status da qualificação — útil
// para handlers de foto precisarem checar o status do relatório dono.
func (r *Repo) FindQualificationReport(ctx context.Context, qualifID string) (reportID, status string, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT r.id, r.status
		  FROM app.report_qualifications q
		  JOIN app.reports r ON r.id = q.report_id
		 WHERE q.id = $1`, qualifID,
	).Scan(&reportID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return
}

// ─── Visibilidade & viewers ────────────────────────────────────────────

// Viewer combina user_id e display_name pra UI.
type Viewer struct {
	UserID      string
	UserCode    string
	DisplayName string
	GrantedBy   string
	GrantedAt   time.Time
}

// CanAccess devolve true se userID tem acesso ao relatório. Admin sempre
// tem; senão precisa ser autor, ou estar na tabela report_viewers, ou o
// relatório estar 'aberto'. Devolve ErrNotFound se o relatório não existe.
//
// Importante: handlers chamam este método ANTES de exibir conteúdo (detalhe,
// download, preview) pra que listagem e endpoints individuais sigam a mesma
// regra.
func (r *Repo) CanAccess(ctx context.Context, reportID, userID string, clearance int, isAdmin bool) (bool, error) {
	existsSQL := `SELECT EXISTS(SELECT 1 FROM app.reports WHERE id = $1 AND deleted_at IS NULL)`
	if isAdmin {
		var exists bool
		if err := r.db.QueryRowContext(ctx, existsSQL, reportID).Scan(&exists); err != nil {
			return false, err
		}
		if !exists {
			return false, ErrNotFound
		}
		return true, nil
	}
	// Autor sempre vê o próprio; demais precisam de visibilidade E clearance.
	var ok bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM app.reports r
			 WHERE r.id = $1
			   AND r.deleted_at IS NULL
			   AND (
			        r.created_by = $2
			        OR (
			             (r.visibility = 'aberto'
			              OR EXISTS (SELECT 1 FROM app.report_viewers v
			                          WHERE v.report_id = r.id AND v.user_id = $2))
			             AND r.required_clearance <= $3
			        )
			      )
		)`, reportID, userID, clearance).Scan(&ok); err != nil {
		return false, err
	}
	if !ok {
		var exists bool
		if err := r.db.QueryRowContext(ctx, existsSQL, reportID).Scan(&exists); err != nil {
			return false, err
		}
		if !exists {
			return false, ErrNotFound
		}
	}
	return ok, nil
}

// Destroy faz soft delete do relatório (preenche deleted_at/deleted_by).
// Permitido SOMENTE em status='criado' — depois de difundido o RI vira
// registro oficial e segue o fluxo arquivar. Mantém audit/forense intactos.
func (r *Repo) Destroy(ctx context.Context, id, actor string) error {
	cur, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if cur.Status != StatusCriado {
		return ErrInvalidStatus
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.reports
		   SET deleted_at = now(),
		       deleted_by = $1,
		       updated_at = now(),
		       updated_by = $1
		 WHERE id = $2
		   AND status = 'criado'
		   AND deleted_at IS NULL`, actor, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrInvalidStatus
	}
	return nil
}

// SetVisibility altera 'aberto'<->'restrito'. Permitido SOMENTE em
// status='criado' — uma vez difundido o RI é registro oficial e nada nele
// pode mudar, inclusive quem enxerga (a lista de destinatários é parte do
// ato de difusão).
func (r *Repo) SetVisibility(ctx context.Context, reportID, visibility, actor string) (*Report, error) {
	if !IsValidVisibility(visibility) {
		return nil, fmt.Errorf("visibility inválido: %q", visibility)
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.reports
		   SET visibility = $1,
		       updated_at = now(),
		       updated_by = $2
		 WHERE id = $3
		   AND status = 'criado'`, visibility, actor, reportID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Distingue inexistente de status inválido pra o handler responder
		// o código HTTP correto (404 vs 409).
		var exists bool
		if err := r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM app.reports WHERE id = $1)`, reportID,
		).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrNotFound
		}
		return nil, ErrInvalidStatus
	}
	return r.FindByID(ctx, reportID)
}

// SetRequiredClearance altera o nível de ACESSO (clearance mínimo, 1..5) do RI.
// Espelha SetVisibility: permitido SOMENTE em status='criado' — uma vez
// difundido o RI é registro oficial e nada nele muda, inclusive quem enxerga.
func (r *Repo) SetRequiredClearance(ctx context.Context, reportID string, level int, actor string) (*Report, error) {
	if level < 1 || level > 5 {
		return nil, fmt.Errorf("required_clearance inválido: %d (1..5)", level)
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.reports
		   SET required_clearance = $1,
		       updated_at = now(),
		       updated_by = $2
		 WHERE id = $3
		   AND status = 'criado'`, level, actor, reportID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var exists bool
		if err := r.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM app.reports WHERE id = $1)`, reportID,
		).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrNotFound
		}
		return nil, ErrInvalidStatus
	}
	return r.FindByID(ctx, reportID)
}

// ListViewers devolve o conjunto explícito de viewers de um relatório, já
// com display_name e code resolvidos pra UI. Ordem por display_name.
func (r *Repo) ListViewers(ctx context.Context, reportID string) ([]Viewer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT v.user_id, u.code, u.display_name, v.granted_by, v.granted_at
		  FROM app.report_viewers v
		  JOIN app.users u ON u.id = v.user_id
		 WHERE v.report_id = $1
		 ORDER BY u.display_name ASC`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Viewer, 0)
	for rows.Next() {
		var v Viewer
		if err := rows.Scan(&v.UserID, &v.UserCode, &v.DisplayName, &v.GrantedBy, &v.GrantedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SetViewers substitui ATOMICAMENTE o conjunto de viewers do relatório. O
// autor original é descartado do input se aparecer (autor tem acesso por
// outra via — coluna created_by). Devolve antes/depois pra audit.
func (r *Repo) SetViewers(ctx context.Context, reportID string, userIDs []string, grantedBy string) (before, after []string, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	// Confirma que o relatório existe + busca autor pra excluir do conjunto.
	// Bloqueia edição da lista após difusão: o RI difundido é imutável e isso
	// inclui o conjunto de destinatários explícitos.
	var authorID, status string
	if err = tx.QueryRowContext(ctx,
		`SELECT created_by, status FROM app.reports WHERE id = $1`, reportID,
	).Scan(&authorID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if status != StatusCriado {
		return nil, nil, ErrInvalidStatus
	}

	// Captura antes pra audit.
	beforeRows, err := tx.QueryContext(ctx,
		`SELECT user_id FROM app.report_viewers WHERE report_id = $1 ORDER BY user_id`, reportID)
	if err != nil {
		return nil, nil, err
	}
	for beforeRows.Next() {
		var uid string
		if err = beforeRows.Scan(&uid); err != nil {
			beforeRows.Close()
			return nil, nil, err
		}
		before = append(before, uid)
	}
	beforeRows.Close()

	// Deduplica + remove o autor (não faz sentido viewer == autor).
	seen := make(map[string]bool, len(userIDs))
	clean := make([]string, 0, len(userIDs))
	for _, u := range userIDs {
		u = strings.TrimSpace(u)
		if u == "" || u == authorID || seen[u] {
			continue
		}
		seen[u] = true
		clean = append(clean, u)
	}

	if _, err = tx.ExecContext(ctx,
		`DELETE FROM app.report_viewers WHERE report_id = $1`, reportID); err != nil {
		return nil, nil, err
	}
	for _, u := range clean {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO app.report_viewers (report_id, user_id, granted_by)
			VALUES ($1, $2, $3)`, reportID, u, grantedBy); err != nil {
			return nil, nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	rollback = false
	return before, clean, nil
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
