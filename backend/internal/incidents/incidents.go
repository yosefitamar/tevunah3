// Package incidents provê leitura/escrita do módulo de Ocorrências —
// registros operacionais (homicídio, apreensão, prisão) alimentados
// diariamente pelos analistas.
//
// Modelagem MVP: tabela única app.incidents com campos comuns a todos os
// tipos + tabela de vínculo app.incident_entities (envolvidos: pessoas/
// entidades já cadastradas). Sem máquina de status e sem níveis de sigilo
// nesta versão.
//
// Convenções (espelham entities/reports):
//   - Sem hard delete: SoftDelete marca deleted_at/deleted_by; List/FindByID
//     filtram soft-deletados por padrão.
//   - photo_path é atualizado por fluxo dedicado (SetPhotoPath), não pelo Update.
//   - Vínculos de entidade são substituídos/adicionados via Add/RemoveEntity.
package incidents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Tipos suportados de ocorrência.
const (
	TypeHomicidio = "homicidio"
	TypeApreensao = "apreensao"
	TypePrisao    = "prisao"
)

// IsValidType devolve true para um tipo suportado.
func IsValidType(t string) bool {
	switch t {
	case TypeHomicidio, TypeApreensao, TypePrisao:
		return true
	}
	return false
}

// Erros públicos.
var (
	ErrNotFound       = errors.New("ocorrência não encontrada")
	ErrAlreadyDeleted = errors.New("ocorrência já excluída")
	ErrInvalidType    = errors.New("tipo inválido")
)

// Incident é o registro consolidado (campos base + envolvidos).
type Incident struct {
	ID                 string
	Type               string
	OccurredOn         time.Time
	OccurredTime       *string // "HH:MM" (NULL = hora desconhecida)
	CIOPSRecord        string
	IntelParticipation bool
	PhotoPath          *string
	Latitude           *float64
	Longitude          *float64
	Description        string

	CreatedAt time.Time
	CreatedBy string
	UpdatedAt time.Time
	UpdatedBy *string
	DeletedAt *time.Time
	DeletedBy *string

	// Involved é populado apenas por FindByID (List evita N+1).
	Involved []InvolvedEntity
}

// InvolvedEntity é um vínculo resolvido entre a ocorrência e uma entidade.
type InvolvedEntity struct {
	EntityID string
	Name     string
	Kind     string
	Role     string
	HasPhoto bool
	Version  int
	AddedAt  time.Time
}

// NewIncident é o input do Create.
type NewIncident struct {
	Type               string
	OccurredOn         time.Time
	OccurredTime       *string
	CIOPSRecord        string
	IntelParticipation bool
	Latitude           *float64
	Longitude          *float64
	Description        string
	CreatedBy          string
}

// UpdateOpts é o input do Update. Campos nil = não tocar.
type UpdateOpts struct {
	Type               *string
	OccurredOn         *time.Time
	OccurredTime       *string // ponteiro p/ "HH:MM"; "" limpa a hora
	OccurredTimeSet    bool    // distingue "não enviado" de "limpar"
	CIOPSRecord        *string
	IntelParticipation *bool
	Latitude           *float64
	LatitudeSet        bool
	Longitude          *float64
	LongitudeSet       bool
	Description        *string
}

// ListOpts controla a listagem.
type ListOpts struct {
	Limit       int    // <= 100; default 25
	Offset      int    // default 0
	Type        string // vazio = todos
	IntelOnly   bool   // true = só com participação INTEL
	Search      string // ILIKE em description/ciops_record
	DateFrom    string // YYYY-MM-DD; vazio = ignora
	DateTo      string // YYYY-MM-DD; vazio = ignora
	SortBy      string // "occurred_on"|"type"|"created_at"|"updated_at"
	SortDir     string // "asc"|"desc"; default "desc"
	OnlyDeleted bool
}

var incidentsSortable = map[string]string{
	"occurred_on": "i.occurred_on",
	"type":        "i.type",
	"created_at":  "i.created_at",
	"updated_at":  "i.updated_at",
}

// ListResult agrupa página + total.
type ListResult struct {
	Items []Incident
	Total int
}

// Repo encapsula queries sobre app.incidents e app.incident_entities.
type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

const incidentSelectFields = `
	i.id, i.type, i.occurred_on, to_char(i.occurred_time, 'HH24:MI'),
	i.ciops_record, i.intel_participation, i.photo_path,
	i.latitude, i.longitude, i.description,
	i.created_at, i.created_by, i.updated_at, i.updated_by,
	i.deleted_at, i.deleted_by`

// ─────────────────────────── Create ────────────────────────────

func (r *Repo) Create(ctx context.Context, in NewIncident) (*Incident, error) {
	if !IsValidType(in.Type) {
		return nil, ErrInvalidType
	}
	if in.OccurredOn.IsZero() {
		return nil, errors.New("occurred_on é obrigatório")
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.incidents
		  (type, occurred_on, occurred_time, ciops_record, intel_participation,
		   latitude, longitude, description, created_by, updated_by)
		VALUES ($1, $2, $3::time, $4, $5, $6, $7, $8, $9, $9)
		RETURNING id`,
		in.Type, in.OccurredOn, nilTimeStr(in.OccurredTime),
		strings.TrimSpace(in.CIOPSRecord), in.IntelParticipation,
		nilFloat(in.Latitude), nilFloat(in.Longitude),
		strings.TrimSpace(in.Description), in.CreatedBy,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	return r.FindByID(ctx, id)
}

// ─────────────────────────── Find / List ────────────────────────────

// FindByID busca por id (inclui soft-deletados) e carrega os envolvidos.
func (r *Repo) FindByID(ctx context.Context, id string) (*Incident, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+incidentSelectFields+` FROM app.incidents i WHERE i.id = $1`, id)
	inc, err := scanIncident(row)
	if err != nil {
		return nil, err
	}
	involved, err := r.ListEntities(ctx, inc.ID)
	if err != nil {
		return nil, err
	}
	inc.Involved = involved
	return inc, nil
}

// List devolve uma página de ocorrências conforme opts.
func (r *Repo) List(ctx context.Context, opts ListOpts) (*ListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 25
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	search := "%" + strings.ToLower(strings.TrimSpace(opts.Search)) + "%"

	deletedClause := "i.deleted_at IS NULL"
	if opts.OnlyDeleted {
		deletedClause = "i.deleted_at IS NOT NULL"
	}

	// Args compartilhados entre count e select (mesma cláusula WHERE).
	args := []any{
		opts.Type,                          // $1
		strings.TrimSpace(opts.Search),     // $2
		search,                             // $3
		opts.IntelOnly,                     // $4
		nilDateStr(opts.DateFrom),          // $5
		nilDateStr(opts.DateTo),            // $6
	}
	where := `
		WHERE ` + deletedClause + `
		  AND ($1 = '' OR i.type = $1)
		  AND ($2 = '' OR lower(i.description) LIKE $3 OR lower(i.ciops_record) LIKE $3)
		  AND ($4 = false OR i.intel_participation = true)
		  AND ($5::date IS NULL OR i.occurred_on >= $5::date)
		  AND ($6::date IS NULL OR i.occurred_on <= $6::date)`

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM app.incidents i`+where, args...,
	).Scan(&total); err != nil {
		return nil, err
	}

	col, ok := incidentsSortable[opts.SortBy]
	if !ok {
		col = "i.occurred_on"
	}
	dir := "DESC"
	if strings.ToLower(opts.SortDir) == "asc" {
		dir = "ASC"
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+incidentSelectFields+` FROM app.incidents i`+where+
			` ORDER BY `+col+` `+dir+`, i.created_at DESC
			  LIMIT $7 OFFSET $8`,
		append(args, opts.Limit, opts.Offset)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Incident, 0)
	for rows.Next() {
		inc, err := scanIncidentRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *inc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total}, nil
}

// ─────────────────────────── Update ────────────────────────────

// Update aplica o patch e devolve a ocorrência recarregada.
func (r *Repo) Update(ctx context.Context, id, updatedBy string, p UpdateOpts) (*Incident, error) {
	before, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if before.DeletedAt != nil {
		return nil, ErrAlreadyDeleted
	}
	if p.Type != nil && !IsValidType(*p.Type) {
		return nil, ErrInvalidType
	}

	// occurred_time/latitude/longitude usam flags *Set pra distinguir
	// "não enviado" (mantém) de "enviado vazio/null" (limpa).
	var timeArg any = nil
	useTime := false
	if p.OccurredTimeSet {
		useTime = true
		timeArg = nilTimeStr(p.OccurredTime)
	}
	var latArg any = nil
	useLat := false
	if p.LatitudeSet {
		useLat = true
		latArg = nilFloat(p.Latitude)
	}
	var lngArg any = nil
	useLng := false
	if p.LongitudeSet {
		useLng = true
		lngArg = nilFloat(p.Longitude)
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE app.incidents SET
		  type                = COALESCE($1, type),
		  occurred_on         = COALESCE($2, occurred_on),
		  occurred_time       = CASE WHEN $3 THEN $4::time ELSE occurred_time END,
		  ciops_record        = COALESCE($5, ciops_record),
		  intel_participation = COALESCE($6, intel_participation),
		  latitude            = CASE WHEN $7 THEN $8::double precision ELSE latitude END,
		  longitude           = CASE WHEN $9 THEN $10::double precision ELSE longitude END,
		  description         = COALESCE($11, description),
		  updated_at          = now(),
		  updated_by          = $12
		WHERE id = $13 AND deleted_at IS NULL`,
		nilStrP(p.Type), nilTimePtr(p.OccurredOn),
		useTime, timeArg,
		nilTrimP(p.CIOPSRecord), nilBool(p.IntelParticipation),
		useLat, latArg, useLng, lngArg,
		nilTrimP(p.Description), updatedBy, id,
	)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

// ─────────────────────────── Photo ────────────────────────────

// SetPhotoPath grava (ou limpa, com filename="") o photo_path e devolve o
// path anterior pra o caller remover o arquivo antigo do disco.
func (r *Repo) SetPhotoPath(ctx context.Context, id, filename, updatedBy string) (oldPath string, err error) {
	var current sql.NullString
	if err := r.db.QueryRowContext(ctx,
		`SELECT photo_path FROM app.incidents WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE app.incidents
		   SET photo_path = $1, updated_at = now(), updated_by = $2
		 WHERE id = $3 AND deleted_at IS NULL`,
		nilStr(filename), updatedBy, id,
	); err != nil {
		return "", err
	}
	return current.String, nil
}

// ─────────────────────────── SoftDelete ────────────────────────────

func (r *Repo) SoftDelete(ctx context.Context, id, deletedBy string) (*Incident, error) {
	before, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if before.DeletedAt != nil {
		return nil, ErrAlreadyDeleted
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.incidents
		   SET deleted_at = now(), deleted_by = $1,
		       updated_at = now(), updated_by = $1
		 WHERE id = $2 AND deleted_at IS NULL`,
		deletedBy, id,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrAlreadyDeleted
	}
	return before, nil
}

// ─────────────────────────── Envolvidos ────────────────────────────

// ListEntities devolve as entidades vinculadas à ocorrência, resolvendo
// nome/kind e se há foto primária (pra thumbnail no front).
func (r *Repo) ListEntities(ctx context.Context, incidentID string) ([]InvolvedEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ie.entity_id, e.name, e.kind, ie.role, e.version, ie.added_at,
		       COALESCE(p.photo_path, pl.photo_path, v.photo_path) IS NOT NULL AS has_photo
		  FROM app.incident_entities ie
		  JOIN app.entities e ON e.id = ie.entity_id
		  LEFT JOIN app.entity_persons   p  ON p.entity_id  = e.id
		  LEFT JOIN app.entity_places    pl ON pl.entity_id = e.id
		  LEFT JOIN app.entity_vehicles  v  ON v.entity_id  = e.id
		 WHERE ie.incident_id = $1
		 ORDER BY ie.added_at`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InvolvedEntity, 0)
	for rows.Next() {
		var ie InvolvedEntity
		if err := rows.Scan(&ie.EntityID, &ie.Name, &ie.Kind, &ie.Role,
			&ie.Version, &ie.AddedAt, &ie.HasPhoto); err != nil {
			return nil, err
		}
		out = append(out, ie)
	}
	return out, rows.Err()
}

// AddEntity vincula uma entidade à ocorrência (upsert do papel se já existe).
func (r *Repo) AddEntity(ctx context.Context, incidentID, entityID, role, addedBy string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO app.incident_entities (incident_id, entity_id, role, added_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (incident_id, entity_id)
		DO UPDATE SET role = EXCLUDED.role`,
		incidentID, entityID, strings.TrimSpace(role), addedBy)
	return err
}

// RemoveEntity desfaz o vínculo.
func (r *Repo) RemoveEntity(ctx context.Context, incidentID, entityID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM app.incident_entities WHERE incident_id = $1 AND entity_id = $2`,
		incidentID, entityID)
	return err
}

// ─────────────────────────── helpers internos ────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanIncident(row *sql.Row) (*Incident, error) { return scanCommon(row) }
func scanIncidentRows(rows *sql.Rows) (*Incident, error) { return scanCommon(rows) }

func scanCommon(s scanner) (*Incident, error) {
	var inc Incident
	var occurredTime sql.NullString
	var photoPath sql.NullString
	var lat, lng sql.NullFloat64
	var updatedBy sql.NullString
	var deletedAt sql.NullTime
	var deletedBy sql.NullString
	if err := s.Scan(
		&inc.ID, &inc.Type, &inc.OccurredOn, &occurredTime,
		&inc.CIOPSRecord, &inc.IntelParticipation, &photoPath,
		&lat, &lng, &inc.Description,
		&inc.CreatedAt, &inc.CreatedBy, &inc.UpdatedAt, &updatedBy,
		&deletedAt, &deletedBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	inc.OccurredTime = nullStr(occurredTime)
	inc.PhotoPath = nullStr(photoPath)
	if lat.Valid {
		v := lat.Float64
		inc.Latitude = &v
	}
	if lng.Valid {
		v := lng.Float64
		inc.Longitude = &v
	}
	inc.UpdatedBy = nullStr(updatedBy)
	if deletedAt.Valid {
		t := deletedAt.Time
		inc.DeletedAt = &t
	}
	inc.DeletedBy = nullStr(deletedBy)
	return &inc, nil
}

// ─── conversão de parâmetros ───

func nilStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nilStrP(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// nilTrimP trata "" como NULL pra COALESCE manter o valor atual quando o
// caller envia string vazia sem intenção de limpar (campos texto NOT NULL
// usam DEFAULT '', então "" seria gravado; aqui preferimos manter).
func nilTrimP(p *string) any {
	if p == nil {
		return nil
	}
	return strings.TrimSpace(*p)
}

func nilBool(p *bool) any {
	if p == nil {
		return nil
	}
	return *p
}

func nilFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nilTimePtr(p *time.Time) any {
	if p == nil {
		return nil
	}
	return *p
}

// nilTimeStr trata ponteiro de string "HH:MM" como parâmetro ::time (NULL se
// nil ou vazio).
func nilTimeStr(p *string) any {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return v
}

// nilDateStr trata "YYYY-MM-DD" vazio como NULL.
func nilDateStr(s string) any {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return v
}

func nullStr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}
