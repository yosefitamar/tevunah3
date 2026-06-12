// Package entities provê leitura/escrita do módulo de Entidades (pessoas,
// organizações, lugares). Modelagem polimórfica: tabela base app.entities +
// uma tabela filha por kind (entity_persons, entity_organizations, entity_places).
//
// Regras de negócio principais:
//   - Sem hard delete: SoftDelete marca deleted_at/deleted_by. List/FindByID
//     filtram registros soft-deletados por padrão.
//   - Optimistic locking via coluna version: Update exige expectedVersion.
//   - Visibilidade por classification (1..4): chamadas de leitura filtram
//     classification <= clearance do chamador.
//   - Tags: substituição total no Update (simples; o consumidor envia o
//     conjunto desejado).
package entities

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Kind enumera os tipos suportados de entidade.
type Kind string

const (
	KindPerson       Kind = "person"
	KindOrganization Kind = "organization"
	KindPlace        Kind = "place"
	KindVehicle      Kind = "vehicle"
)

// IsValid devolve true para um Kind suportado.
func (k Kind) IsValid() bool {
	switch k {
	case KindPerson, KindOrganization, KindPlace, KindVehicle:
		return true
	}
	return false
}

// Erros públicos.
var (
	ErrNotFound        = errors.New("entidade não encontrada")
	ErrVersionConflict = errors.New("versão desatualizada")
	ErrInvalidKind     = errors.New("kind inválido")
	ErrAlreadyDeleted  = errors.New("entidade já excluída")
	// ErrNotDeleted é retornado por Restore quando o registro não está na lixeira.
	ErrNotDeleted = errors.New("entidade não está excluída")
	// ErrOrgNameDuplicate vem do índice único parcial sobre lower(name)
	// quando kind=organization e a entidade está ativa.
	ErrOrgNameDuplicate = errors.New("organização com este nome já cadastrada")
	// ErrCPFDuplicate vem do índice único parcial sobre cpf (não-nulo) em
	// entity_persons. Captura tentativas de re-cadastrar o mesmo CPF.
	ErrCPFDuplicate = errors.New("CPF já cadastrado")
	// ErrPlateDuplicate vem do índice único parcial sobre plate (não-nulo)
	// em entity_vehicles. Captura tentativas de re-cadastrar a mesma placa.
	ErrPlateDuplicate = errors.New("placa já cadastrada")
)

// Entity representa o objeto consolidado (base + atributos polimórficos + tags).
// Para a tabela filha, exatamente um dos campos Person/Organization/Place é
// não-nil conforme o Kind.
type Entity struct {
	ID             string
	Kind           Kind
	Name           string
	Description    string
	Classification int
	Version        int
	Tags           []string
	CreatedAt      time.Time
	CreatedBy      string
	UpdatedAt      time.Time
	UpdatedBy      string
	DeletedAt      *time.Time
	DeletedBy      *string

	Person       *PersonAttrs
	Organization *OrganizationAttrs
	Place        *PlaceAttrs
	Vehicle      *VehicleAttrs

	// Photos é a galeria de fotos adicionais. Populada apenas por FindByID
	// (e portanto por Create/Update/Restore que recarregam). List não popula
	// para evitar N+1 — listagens não precisam da galeria.
	Photos []GalleryPhoto
}

// PersonAttrs são os atributos da tabela app.entity_persons.
// OrcrimName e OrcrimAlias são populados por join na leitura (não persistem).
// Addresses vem da tabela 1-N app.person_addresses; populada por FindByID.
type PersonAttrs struct {
	Aliases     []string
	Gender      *string
	DateOfBirth *time.Time
	MotherName  *string
	CPF         *string
	PhotoPath   *string
	OrcrimID    *string
	OrcrimName  *string
	OrcrimAlias *string
	Addresses   []PersonAddress
}

// OrganizationAttrs são os atributos da tabela app.entity_organizations.
// Aliases segue a mesma semântica de persons.Aliases — primeiro elemento é a
// sigla/alias primária (rótulo prioritário em selects e listagens).
type OrganizationAttrs struct {
	Aliases   []string
	LegalName *string
	TaxID     *string
	FoundedAt *time.Time
}

// PlaceAttrs são os atributos da tabela app.entity_places.
type PlaceAttrs struct {
	Address   *string
	Country   *string
	Region    *string
	Latitude  *float64
	Longitude *float64
	PhotoPath *string
}

// VehicleAttrs são os atributos da tabela app.entity_vehicles. Placa é o
// identificador prático no Brasil — normalizada em uppercase, sem hífen ou
// espaços, e única quando preenchida (unique index parcial).
type VehicleAttrs struct {
	// Category distingue carro de moto ('car' | 'motorcycle'). Vazio no
	// insert é tratado como 'car' (default da coluna).
	Category  *string
	Plate     *string
	Brand     *string
	Model     *string
	Color     *string
	Year      *int
	Chassis   *string
	Renavam   *string
	PhotoPath *string
}

// NewEntity é o input do Create.
type NewEntity struct {
	Kind           Kind
	Name           string
	Description    string
	Classification int
	Tags           []string
	Person         *PersonAttrs
	Organization   *OrganizationAttrs
	Place          *PlaceAttrs
	Vehicle        *VehicleAttrs
}

// Patch é o input do Update. Campos nil = não tocar. Tags nil = não tocar;
// Tags []string{} = limpar.
type Patch struct {
	Name           *string
	Description    *string
	Classification *int
	Tags           *[]string
	Person         *PersonAttrs
	Organization   *OrganizationAttrs
	Place          *PlaceAttrs
	Vehicle        *VehicleAttrs
}

// ListOpts controla a listagem.
type ListOpts struct {
	Limit          int    // <= 100; default 25
	Offset         int    // default 0
	Kind           Kind   // vazio = todos
	Classification int    // 0 = todos, 1..4 filtra exato
	Tag            string // vazio = todos
	Search         string // ILIKE em lower(name)
	SortBy         string // "name"|"kind"|"classification"|"created_at"|"updated_at"
	SortDir        string // "asc"|"desc"; default "asc"
	MaxClearance   int    // 1..5; oculta registros com classification > MaxClearance
	OnlyDeleted    bool   // true = exibe apenas soft-deletados (Lixeira)
}

var entitiesSortable = map[string]string{
	"name":           "e.name",
	"kind":           "e.kind",
	"classification": "e.classification",
	"created_at":     "e.created_at",
	"updated_at":     "e.updated_at",
}

// ListResult agrupa página + total.
type ListResult struct {
	Items []Entity
	Total int
}

// Repo encapsula queries sobre app.entities e tabelas filhas.
type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// ─────────────────────────── Create ────────────────────────────

// Create insere a entidade base, a tabela filha correspondente ao Kind e
// as tags, tudo em transação. Devolve a entidade carregada (com tags + attrs).
func (r *Repo) Create(ctx context.Context, in NewEntity, createdBy string) (*Entity, error) {
	if !in.Kind.IsValid() {
		return nil, ErrInvalidKind
	}
	// Veículo deriva nome automaticamente quando o client não informa um:
	// usa "MARCA MODELO" se ambos existem, senão placa, senão um placeholder.
	// Decisão de design: name fica NOT NULL na base; veículos têm rótulo
	// natural via attrs (placa/marca/modelo) e o name é só interno pra busca.
	if in.Kind == KindVehicle && strings.TrimSpace(in.Name) == "" && in.Vehicle != nil {
		in.Name = deriveVehicleName(in.Vehicle)
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("name é obrigatório")
	}
	if in.Classification < 1 || in.Classification > 4 {
		return nil, errors.New("classification deve estar entre 1 e 4")
	}
	if err := requireMatchingAttrs(in.Kind, in.Person, in.Organization, in.Place, in.Vehicle); err != nil {
		return nil, err
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
		INSERT INTO app.entities
		  (kind, name, description, classification, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $5)
		RETURNING id`,
		string(in.Kind), strings.TrimSpace(in.Name), nullableString(upperTrim(in.Description)),
		in.Classification, createdBy,
	).Scan(&id)
	if err != nil {
		return nil, classifyUniqueErr(err, in.Kind, in.Person)
	}

	if err := insertChild(ctx, tx, id, in.Kind, in.Person, in.Organization, in.Place, in.Vehicle); err != nil {
		return nil, classifyUniqueErr(err, in.Kind, in.Person)
	}

	if err := replaceTags(ctx, tx, id, in.Tags, createdBy); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rollback = false

	return r.FindByID(ctx, id)
}

// ─────────────────────────── Find / List ────────────────────────────

// FindByID busca por id, sem filtro de clearance. Caller é responsável por
// verificar visibilidade antes de devolver ao usuário final. Retorna soft-deletados.
func (r *Repo) FindByID(ctx context.Context, id string) (*Entity, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, kind, name, COALESCE(description,''), classification, version,
		       created_at, created_by, updated_at, updated_by,
		       deleted_at, deleted_by
		  FROM app.entities
		 WHERE id = $1`, id)
	e, err := scanEntity(row)
	if err != nil {
		return nil, err
	}
	if err := r.loadChild(ctx, e); err != nil {
		return nil, err
	}
	if err := r.loadTags(ctx, e); err != nil {
		return nil, err
	}
	photos, err := r.ListGalleryPhotos(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	e.Photos = photos
	return e, nil
}

// List devolve uma página de entidades não-deletadas, filtrada conforme opts
// e respeitando opts.MaxClearance (entidades com classification maior são
// suprimidas).
func (r *Repo) List(ctx context.Context, opts ListOpts) (*ListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 25
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.MaxClearance < 1 {
		opts.MaxClearance = 1
	}
	if opts.MaxClearance > 5 {
		opts.MaxClearance = 5
	}
	search := "%" + strings.ToLower(strings.TrimSpace(opts.Search)) + "%"

	// Filtro de soft-delete: por padrão oculta deletados; OnlyDeleted inverte
	// para mostrar exclusivamente registros da lixeira.
	deletedClause := "e.deleted_at IS NULL"
	if opts.OnlyDeleted {
		deletedClause = "e.deleted_at IS NOT NULL"
	}

	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM app.entities e
		 WHERE `+deletedClause+`
		   AND e.classification <= $1
		   AND ($2 = '' OR e.kind = $2)
		   AND ($3 = 0  OR e.classification = $3)
		   AND ($4 = '' OR EXISTS (SELECT 1 FROM app.entity_tags t
		                            WHERE t.entity_id = e.id AND t.tag = $4))
		   AND ($5 = '' OR lower(e.name) LIKE $6)`,
		opts.MaxClearance, string(opts.Kind), opts.Classification,
		strings.ToLower(opts.Tag), strings.TrimSpace(opts.Search), search,
	).Scan(&total)
	if err != nil {
		return nil, err
	}

	col, ok := entitiesSortable[opts.SortBy]
	if !ok {
		col = "e.name"
	}
	dir := "ASC"
	if strings.ToLower(opts.SortDir) == "desc" {
		dir = "DESC"
	}

	// LEFT JOINs trazem o mínimo de attrs específicos pra rotular linhas sem
	// N+1: aliases da organização (sigla) e placa do veículo.
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.kind, e.name, COALESCE(e.description,''), e.classification, e.version,
		       e.created_at, e.created_by, e.updated_at, e.updated_by,
		       e.deleted_at, e.deleted_by,
		       COALESCE(
		         (SELECT string_agg(t.tag, ',' ORDER BY t.tag)
		            FROM app.entity_tags t WHERE t.entity_id = e.id),
		         ''
		       ) AS tags_csv,
		       COALESCE(to_jsonb(o.aliases), 'null'::jsonb) AS org_aliases_json,
		       v.plate, v.category, v.brand, v.model, v.color, v.photo_path
		  FROM app.entities e
		  LEFT JOIN app.entity_organizations o ON o.entity_id = e.id
		  LEFT JOIN app.entity_vehicles v ON v.entity_id = e.id
		 WHERE `+deletedClause+`
		   AND e.classification <= $1
		   AND ($2 = '' OR e.kind = $2)
		   AND ($3 = 0  OR e.classification = $3)
		   AND ($4 = '' OR EXISTS (SELECT 1 FROM app.entity_tags t
		                            WHERE t.entity_id = e.id AND t.tag = $4))
		   AND ($5 = '' OR lower(e.name) LIKE $6)
		 ORDER BY `+col+` `+dir+`, e.name ASC
		 LIMIT $7 OFFSET $8`,
		opts.MaxClearance, string(opts.Kind), opts.Classification,
		strings.ToLower(opts.Tag), strings.TrimSpace(opts.Search), search,
		opts.Limit, opts.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Entity, 0)
	for rows.Next() {
		var e Entity
		var kind, createdBy, updatedBy, tagsCSV string
		var description string
		var deletedAt sql.NullTime
		var deletedBy sql.NullString
		var orgAliasesJSON []byte
		var vehiclePlate, vehicleCategory, vehicleBrand, vehicleModel, vehicleColor, vehiclePhotoPath sql.NullString
		if err := rows.Scan(
			&e.ID, &kind, &e.Name, &description, &e.Classification, &e.Version,
			&e.CreatedAt, &createdBy, &e.UpdatedAt, &updatedBy,
			&deletedAt, &deletedBy, &tagsCSV, &orgAliasesJSON,
			&vehiclePlate, &vehicleCategory, &vehicleBrand, &vehicleModel, &vehicleColor, &vehiclePhotoPath,
		); err != nil {
			return nil, err
		}
		e.Kind = Kind(kind)
		e.Description = description
		e.CreatedBy = createdBy
		e.UpdatedBy = updatedBy
		if deletedAt.Valid {
			t := deletedAt.Time
			e.DeletedAt = &t
		}
		if deletedBy.Valid {
			s := deletedBy.String
			e.DeletedBy = &s
		}
		if tagsCSV != "" {
			e.Tags = strings.Split(tagsCSV, ",")
		} else {
			e.Tags = []string{}
		}
		// Para organização, popula attrs.aliases (única info de attrs no list).
		if e.Kind == KindOrganization && len(orgAliasesJSON) > 0 &&
			string(orgAliasesJSON) != "null" {
			var aliases []string
			if err := json.Unmarshal(orgAliasesJSON, &aliases); err == nil {
				e.Organization = &OrganizationAttrs{Aliases: aliases}
			}
		}
		// Para veículo, popula os attrs usados no rótulo de listagem
		// (placa, categoria, marca, modelo, cor).
		if e.Kind == KindVehicle {
			e.Vehicle = &VehicleAttrs{
				Plate:     nullStr(vehiclePlate),
				Category:  nullStr(vehicleCategory),
				Brand:     nullStr(vehicleBrand),
				Model:     nullStr(vehicleModel),
				Color:     nullStr(vehicleColor),
				PhotoPath: nullStr(vehiclePhotoPath),
			}
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// List não popula attrs polimórficos (para não fazer N+1). Detalhe via FindByID.
	return &ListResult{Items: items, Total: total}, nil
}

// ─────────────────────────── Update ────────────────────────────

// Update aplica o patch sob lock otimista. Retorna a entidade antes da
// mutação (para audit) e a depois (recarregada). Falha com ErrVersionConflict
// se expectedVersion não bater com o atual.
func (r *Repo) Update(ctx context.Context, id string, expectedVersion int, p Patch, updatedBy string) (before, after *Entity, err error) {
	before, err = r.FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if before.DeletedAt != nil {
		return nil, nil, ErrAlreadyDeleted
	}
	if before.Version != expectedVersion {
		return nil, nil, ErrVersionConflict
	}

	// Valida attrs polimórficos: se vier patch para attrs, deve casar com kind.
	if (p.Person != nil && before.Kind != KindPerson) ||
		(p.Organization != nil && before.Kind != KindOrganization) ||
		(p.Place != nil && before.Kind != KindPlace) ||
		(p.Vehicle != nil && before.Kind != KindVehicle) {
		return nil, nil, fmt.Errorf("attrs incompatíveis com kind %s", before.Kind)
	}

	// Veículo: re-deriva o nome quando o client edita os attrs sem fornecer
	// um name explícito. Mantém o rótulo sincronizado com marca/modelo/placa.
	if before.Kind == KindVehicle && p.Name == nil && p.Vehicle != nil {
		merged := mergeVehicleAttrs(before.Vehicle, p.Vehicle)
		nm := deriveVehicleName(&merged)
		p.Name = &nm
	}

	if p.Classification != nil && (*p.Classification < 1 || *p.Classification > 4) {
		return nil, nil, errors.New("classification deve estar entre 1 e 4")
	}
	if p.Name != nil && strings.TrimSpace(*p.Name) == "" {
		return nil, nil, errors.New("name não pode ser vazio")
	}

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

	// Update da base com bump de version e lock otimista no WHERE.
	res, err := tx.ExecContext(ctx, `
		UPDATE app.entities
		   SET name           = COALESCE($1, name),
		       description    = COALESCE($2, description),
		       classification = COALESCE($3, classification),
		       version        = version + 1,
		       updated_at     = now(),
		       updated_by     = $4
		 WHERE id = $5 AND version = $6 AND deleted_at IS NULL`,
		nullableTrimmedString(p.Name), nullableStringPtr(upperTrimPtr(p.Description)),
		nullableInt(p.Classification), updatedBy, id, expectedVersion,
	)
	if err != nil {
		return nil, nil, classifyUniqueErr(err, before.Kind, p.Person)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil, ErrVersionConflict
	}

	// Tabela filha: se veio patch de attrs, faz update parcial (COALESCE).
	if err := updateChild(ctx, tx, id, before.Kind, p); err != nil {
		return nil, nil, classifyUniqueErr(err, before.Kind, p.Person)
	}

	// Tags: substituição total se Tags != nil.
	if p.Tags != nil {
		if err := replaceTags(ctx, tx, id, *p.Tags, updatedBy); err != nil {
			return nil, nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	rollback = false

	after, err = r.FindByID(ctx, id)
	return before, after, err
}

// ─────────────────────────── Photo path ────────────────────────────

// SetPhotoPath grava o filename da foto primária associada a uma pessoa ou
// lugar (dispatch pela coluna kind) e bumpa a versão da entidade. Retorna o
// photo_path anterior para o caller remover o arquivo antigo do disco, se
// houver, ou string vazia se não havia. Organizações não têm foto primária.
func (r *Repo) SetPhotoPath(ctx context.Context, entityID, filename, updatedBy string) (oldPath string, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	var kind string
	if err := tx.QueryRowContext(ctx,
		`SELECT kind FROM app.entities WHERE id = $1 AND deleted_at IS NULL`, entityID,
	).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}

	var childTable string
	switch Kind(kind) {
	case KindPerson:
		childTable = "app.entity_persons"
	case KindPlace:
		childTable = "app.entity_places"
	case KindVehicle:
		childTable = "app.entity_vehicles"
	default:
		return "", fmt.Errorf("foto primária não suportada para kind %q", kind)
	}

	var current sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT photo_path FROM `+childTable+` WHERE entity_id = $1`, entityID,
	).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE `+childTable+` SET photo_path = $1 WHERE entity_id = $2`,
		nullableString(filename), entityID,
	); err != nil {
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE app.entities
		   SET version = version + 1, updated_at = now(), updated_by = $1
		 WHERE id = $2 AND deleted_at IS NULL`,
		updatedBy, entityID,
	); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	rollback = false
	return current.String, nil
}

// ─────────────────────────── SoftDelete ────────────────────────────

// SoftDelete marca deleted_at/deleted_by. Hard delete não é suportado.
// Devolve a entidade antes da exclusão (para audit).
func (r *Repo) SoftDelete(ctx context.Context, id string, deletedBy string) (*Entity, error) {
	before, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if before.DeletedAt != nil {
		return nil, ErrAlreadyDeleted
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.entities
		   SET deleted_at = now(),
		       deleted_by = $1,
		       updated_at = now(),
		       updated_by = $1,
		       version    = version + 1
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

// ─────────────────────────── Restore ────────────────────────────

// Restore reverte o soft-delete: zera deleted_at/deleted_by, bumpa version e
// marca updated_by. Pode falhar com ErrOrgNameDuplicate se, entre o delete e
// o restore, outra organização ativa tiver assumido o mesmo nome (o índice
// único parcial de entities_organization_name_uniq exclui deletados).
func (r *Repo) Restore(ctx context.Context, id, restoredBy string) (before, after *Entity, err error) {
	before, err = r.FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if before.DeletedAt == nil {
		return nil, nil, ErrNotDeleted
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.entities
		   SET deleted_at = NULL,
		       deleted_by = NULL,
		       updated_at = now(),
		       updated_by = $1,
		       version    = version + 1
		 WHERE id = $2 AND deleted_at IS NOT NULL`,
		restoredBy, id,
	)
	if err != nil {
		return nil, nil, classifyUniqueErr(err, before.Kind, nil)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil, ErrNotDeleted
	}
	after, err = r.FindByID(ctx, id)
	return before, after, err
}

// classifyUniqueErr inspeciona um erro de DB e mapeia para o erro tipado
// correspondente quando se trata de violação de unique. Se não for unique,
// devolve o erro original embrulhado.
func classifyUniqueErr(err error, k Kind, _ *PersonAttrs) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// pgx envelopa SQLSTATE no texto; checamos por substring para resiliência.
	if strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key") {
		switch {
		case strings.Contains(msg, "entities_organization_name_uniq"):
			return ErrOrgNameDuplicate
		case strings.Contains(msg, "entity_persons_cpf_uniq"):
			return ErrCPFDuplicate
		case strings.Contains(msg, "entity_vehicles_plate_uniq"):
			return ErrPlateDuplicate
		}
		// Outras violações: mantém o erro original com sinal claro.
		if k == KindOrganization {
			return ErrOrgNameDuplicate
		}
	}
	return fmt.Errorf("db: %w", err)
}

// ─────────────────────────── Busca de duplicates (pessoa) ─────────────────

// PersonDuplicate é um match de homônimo com score = quantidade de critérios
// que bateram (1..3). MatchedFields lista os critérios que casaram.
type PersonDuplicate struct {
	ID            string
	Name          string
	MotherName    *string
	DateOfBirth   *time.Time
	Score         int
	MatchedFields []string
}

// DuplicatesQuery parametriza a busca de homônimos. Name é obrigatório
// (sem nome, não busca nada). Os demais campos são opcionais — quando
// preenchidos, sobem o score do match.
type DuplicatesQuery struct {
	Name         string
	MotherName   string
	DateOfBirth  string // YYYY-MM-DD; vazio = ignora
	CPF          string // se não vazio, retorna também CPFTakenBy
	ExcludeID    string // para edit mode
	MaxClearance int    // clamp para não vazar entidades acima do clearance
}

// DuplicatesResult agrupa CPF tomado (se houver) + homônimos por nome.
type DuplicatesResult struct {
	CPFTakenBy *PersonDuplicate // não-nil se outra pessoa já tem este CPF
	Matches    []PersonDuplicate
}

// FindPersonDuplicates devolve homônimos e (opcionalmente) o portador atual
// de um CPF. Respeita clearance.
func (r *Repo) FindPersonDuplicates(ctx context.Context, q DuplicatesQuery) (*DuplicatesResult, error) {
	if q.MaxClearance < 1 {
		q.MaxClearance = 1
	}
	if q.MaxClearance > 5 {
		q.MaxClearance = 5
	}
	out := &DuplicatesResult{Matches: []PersonDuplicate{}}

	// Helpers para tratar strings vazias como NULL — colunas com tipos
	// específicos (uuid, date) rejeitam '' mesmo dentro de CASE/OR.
	nullIfEmpty := func(s string) any {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return s
	}
	excludeParam := nullIfEmpty(q.ExcludeID)
	dobParam := nullIfEmpty(q.DateOfBirth)

	// 1) CPF tomado?
	if cpfDigits := strings.TrimSpace(q.CPF); cpfDigits != "" {
		row := r.db.QueryRowContext(ctx, `
			SELECT e.id, e.name, p.mother_name, p.date_of_birth
			  FROM app.entity_persons p
			  JOIN app.entities e ON e.id = p.entity_id
			 WHERE p.cpf = $1
			   AND e.deleted_at IS NULL
			   AND e.classification <= $2
			   AND ($3::uuid IS NULL OR e.id <> $3::uuid)
			 LIMIT 1`,
			cpfDigits, q.MaxClearance, excludeParam)
		var d PersonDuplicate
		var mn sql.NullString
		var dob sql.NullTime
		if err := row.Scan(&d.ID, &d.Name, &mn, &dob); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("query cpf: %w", err)
			}
		} else {
			d.MotherName = nullStr(mn)
			if dob.Valid {
				t := dob.Time
				d.DateOfBirth = &t
			}
			out.CPFTakenBy = &d
		}
	}

	// 2) Homônimos por nome (case-insensitive). Sem nome, não busca.
	name := strings.TrimSpace(q.Name)
	if name == "" {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.name, p.mother_name, p.date_of_birth,
		       (CASE WHEN lower(e.name) = lower($1) THEN 1 ELSE 0 END)
		     + (CASE WHEN $2 <> '' AND lower(coalesce(p.mother_name,'')) = lower($2) THEN 1 ELSE 0 END)
		     + (CASE WHEN $3::date IS NOT NULL AND p.date_of_birth = $3::date THEN 1 ELSE 0 END) AS score
		  FROM app.entity_persons p
		  JOIN app.entities e ON e.id = p.entity_id
		 WHERE e.kind = 'person'
		   AND e.deleted_at IS NULL
		   AND e.classification <= $4
		   AND lower(e.name) = lower($1)
		   AND ($5::uuid IS NULL OR e.id <> $5::uuid)
		 ORDER BY score DESC, e.name
		 LIMIT 20`,
		name, q.MotherName, dobParam, q.MaxClearance, excludeParam)
	if err != nil {
		return nil, fmt.Errorf("query duplicates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var d PersonDuplicate
		var mn sql.NullString
		var dob sql.NullTime
		if err := rows.Scan(&d.ID, &d.Name, &mn, &dob, &d.Score); err != nil {
			return nil, err
		}
		d.MotherName = nullStr(mn)
		if dob.Valid {
			t := dob.Time
			d.DateOfBirth = &t
		}
		// Reconstroi quais campos casaram (para a UI explicar o score).
		d.MatchedFields = []string{"name"}
		if q.MotherName != "" && d.MotherName != nil &&
			strings.EqualFold(strings.TrimSpace(*d.MotherName), strings.TrimSpace(q.MotherName)) {
			d.MatchedFields = append(d.MatchedFields, "mother_name")
		}
		if q.DateOfBirth != "" && d.DateOfBirth != nil &&
			d.DateOfBirth.Format("2006-01-02") == q.DateOfBirth {
			d.MatchedFields = append(d.MatchedFields, "date_of_birth")
		}
		out.Matches = append(out.Matches, d)
	}
	return out, rows.Err()
}

// ─────────────────────────── helpers internos ────────────────────────────

func scanEntity(row *sql.Row) (*Entity, error) {
	var e Entity
	var kind, createdBy, updatedBy string
	var description string
	var deletedAt sql.NullTime
	var deletedBy sql.NullString
	if err := row.Scan(
		&e.ID, &kind, &e.Name, &description, &e.Classification, &e.Version,
		&e.CreatedAt, &createdBy, &e.UpdatedAt, &updatedBy,
		&deletedAt, &deletedBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	e.Kind = Kind(kind)
	e.Description = description
	e.CreatedBy = createdBy
	e.UpdatedBy = updatedBy
	if deletedAt.Valid {
		t := deletedAt.Time
		e.DeletedAt = &t
	}
	if deletedBy.Valid {
		s := deletedBy.String
		e.DeletedBy = &s
	}
	return &e, nil
}

func (r *Repo) loadChild(ctx context.Context, e *Entity) error {
	switch e.Kind {
	case KindPerson:
		var a PersonAttrs
		var aliasesJSON []byte
		var dob sql.NullTime
		var gender, motherName, cpf, photoPath sql.NullString
		var orcrimID, orcrimName, orcrimAlias sql.NullString
		err := r.db.QueryRowContext(ctx, `
			SELECT to_jsonb(p.aliases), p.gender, p.date_of_birth,
			       p.mother_name, p.cpf, p.photo_path,
			       p.orcrim_id, oc.name, oco.aliases[1]
			  FROM app.entity_persons p
			  LEFT JOIN app.entities oc
			    ON oc.id = p.orcrim_id AND oc.deleted_at IS NULL
			  LEFT JOIN app.entity_organizations oco
			    ON oco.entity_id = p.orcrim_id
			 WHERE p.entity_id = $1`, e.ID,
		).Scan(&aliasesJSON, &gender, &dob, &motherName, &cpf, &photoPath,
			&orcrimID, &orcrimName, &orcrimAlias)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				e.Person = &PersonAttrs{Aliases: []string{}, Addresses: []PersonAddress{}}
				return nil
			}
			return err
		}
		a.Aliases = []string{}
		if len(aliasesJSON) > 0 {
			if err := json.Unmarshal(aliasesJSON, &a.Aliases); err != nil {
				return fmt.Errorf("decode aliases: %w", err)
			}
		}
		a.Gender = nullStr(gender)
		if dob.Valid {
			t := dob.Time
			a.DateOfBirth = &t
		}
		a.MotherName = nullStr(motherName)
		a.CPF = nullStr(cpf)
		a.PhotoPath = nullStr(photoPath)
		a.OrcrimID = nullStr(orcrimID)
		a.OrcrimName = nullStr(orcrimName)
		a.OrcrimAlias = nullStr(orcrimAlias)
		addrs, err := r.ListAddresses(ctx, e.ID)
		if err != nil {
			return err
		}
		a.Addresses = addrs
		e.Person = &a
	case KindOrganization:
		var a OrganizationAttrs
		var aliasesJSON []byte
		var legal, taxID sql.NullString
		var founded sql.NullTime
		err := r.db.QueryRowContext(ctx, `
			SELECT to_jsonb(aliases), legal_name, tax_id, founded_at
			  FROM app.entity_organizations WHERE entity_id = $1`, e.ID,
		).Scan(&aliasesJSON, &legal, &taxID, &founded)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				e.Organization = &OrganizationAttrs{Aliases: []string{}}
				return nil
			}
			return err
		}
		a.Aliases = []string{}
		if len(aliasesJSON) > 0 {
			if err := json.Unmarshal(aliasesJSON, &a.Aliases); err != nil {
				return fmt.Errorf("decode org aliases: %w", err)
			}
		}
		a.LegalName = nullStr(legal)
		a.TaxID = nullStr(taxID)
		if founded.Valid {
			t := founded.Time
			a.FoundedAt = &t
		}
		e.Organization = &a
	case KindPlace:
		var a PlaceAttrs
		var address, country, region, photoPath sql.NullString
		var lat, lng sql.NullFloat64
		err := r.db.QueryRowContext(ctx, `
			SELECT address, country, region, latitude, longitude, photo_path
			  FROM app.entity_places WHERE entity_id = $1`, e.ID,
		).Scan(&address, &country, &region, &lat, &lng, &photoPath)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				e.Place = &PlaceAttrs{}
				return nil
			}
			return err
		}
		a.Address = nullStr(address)
		a.Country = nullStr(country)
		a.Region = nullStr(region)
		if lat.Valid {
			v := lat.Float64
			a.Latitude = &v
		}
		if lng.Valid {
			v := lng.Float64
			a.Longitude = &v
		}
		a.PhotoPath = nullStr(photoPath)
		e.Place = &a
	case KindVehicle:
		var a VehicleAttrs
		var category, plate, brand, model, color, chassis, renavam, photoPath sql.NullString
		var year sql.NullInt64
		err := r.db.QueryRowContext(ctx, `
			SELECT category, plate, brand, model, color, year, chassis, renavam, photo_path
			  FROM app.entity_vehicles WHERE entity_id = $1`, e.ID,
		).Scan(&category, &plate, &brand, &model, &color, &year, &chassis, &renavam, &photoPath)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				e.Vehicle = &VehicleAttrs{}
				return nil
			}
			return err
		}
		a.Category = nullStr(category)
		a.Plate = nullStr(plate)
		a.Brand = nullStr(brand)
		a.Model = nullStr(model)
		a.Color = nullStr(color)
		if year.Valid {
			v := int(year.Int64)
			a.Year = &v
		}
		a.Chassis = nullStr(chassis)
		a.Renavam = nullStr(renavam)
		a.PhotoPath = nullStr(photoPath)
		e.Vehicle = &a
	}
	return nil
}

func (r *Repo) loadTags(ctx context.Context, e *Entity) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT tag FROM app.entity_tags WHERE entity_id = $1 ORDER BY tag`, e.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	e.Tags = []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return err
		}
		e.Tags = append(e.Tags, t)
	}
	return rows.Err()
}

func requireMatchingAttrs(k Kind, p *PersonAttrs, o *OrganizationAttrs, pl *PlaceAttrs, v *VehicleAttrs) error {
	count := 0
	if p != nil {
		count++
	}
	if o != nil {
		count++
	}
	if pl != nil {
		count++
	}
	if v != nil {
		count++
	}
	if count > 1 {
		return errors.New("envie apenas o bloco de attrs do kind escolhido")
	}
	switch k {
	case KindPerson:
		if o != nil || pl != nil || v != nil {
			return fmt.Errorf("attrs incompatíveis com kind %s", k)
		}
	case KindOrganization:
		if p != nil || pl != nil || v != nil {
			return fmt.Errorf("attrs incompatíveis com kind %s", k)
		}
	case KindPlace:
		if p != nil || o != nil || v != nil {
			return fmt.Errorf("attrs incompatíveis com kind %s", k)
		}
	case KindVehicle:
		if p != nil || o != nil || pl != nil {
			return fmt.Errorf("attrs incompatíveis com kind %s", k)
		}
	}
	return nil
}

func insertChild(ctx context.Context, tx *sql.Tx, id string, k Kind, p *PersonAttrs, o *OrganizationAttrs, pl *PlaceAttrs, v *VehicleAttrs) error {
	switch k {
	case KindPerson:
		if p == nil {
			p = &PersonAttrs{Aliases: []string{}}
		}
		if p.Aliases == nil {
			p.Aliases = []string{}
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO app.entity_persons
			  (entity_id, aliases, gender, date_of_birth,
			   mother_name, cpf, photo_path, orcrim_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			id, p.Aliases, nilStr(p.Gender), nilTime(p.DateOfBirth),
			nilStr(p.MotherName), nilStr(p.CPF), nilStr(p.PhotoPath), nilUUID(p.OrcrimID),
		)
		return err
	case KindOrganization:
		if o == nil {
			o = &OrganizationAttrs{Aliases: []string{}}
		}
		if o.Aliases == nil {
			o.Aliases = []string{}
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO app.entity_organizations
			  (entity_id, aliases, legal_name, tax_id, founded_at)
			VALUES ($1, $2, $3, $4, $5)`,
			id, o.Aliases, nilStr(o.LegalName), nilStr(o.TaxID), nilTime(o.FoundedAt),
		)
		return err
	case KindPlace:
		if pl == nil {
			pl = &PlaceAttrs{}
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO app.entity_places
			  (entity_id, address, country, region, latitude, longitude)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			id, nilStr(pl.Address), nilStr(pl.Country), nilStr(pl.Region),
			nilFloat(pl.Latitude), nilFloat(pl.Longitude),
		)
		return err
	case KindVehicle:
		if v == nil {
			v = &VehicleAttrs{}
		}
		// Placa normalizada em uppercase sem hífen/espaço — facilita lookup,
		// dedup e o unique index parcial.
		var plate any
		if v.Plate != nil {
			s := normalizePlate(*v.Plate)
			if s != "" {
				plate = s
			}
		}
		// Coluna category é NOT NULL; sem valor explícito assume 'car'.
		category := "car"
		if v.Category != nil && *v.Category != "" {
			category = *v.Category
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO app.entity_vehicles
			  (entity_id, category, plate, brand, model, color, year, chassis, renavam)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			id, category, plate, nilStr(v.Brand), nilStr(v.Model), nilStr(v.Color),
			nullableInt(v.Year), nilStr(v.Chassis), nilStr(v.Renavam),
		)
		return err
	}
	return ErrInvalidKind
}

// deriveVehicleName monta um rótulo legível a partir dos attrs do veículo,
// usado como name na base quando o client não envia explicitamente.
// Ordem de fallback: "MARCA MODELO" → "MARCA" → "MODELO" → placa → placeholder.
func deriveVehicleName(v *VehicleAttrs) string {
	brand := ""
	if v.Brand != nil {
		brand = strings.TrimSpace(*v.Brand)
	}
	model := ""
	if v.Model != nil {
		model = strings.TrimSpace(*v.Model)
	}
	plate := ""
	if v.Plate != nil {
		plate = normalizePlate(*v.Plate)
	}
	switch {
	case brand != "" && model != "":
		return brand + " " + model
	case brand != "":
		return brand
	case model != "":
		return model
	case plate != "":
		return plate
	default:
		return "VEÍCULO SEM IDENTIFICAÇÃO"
	}
}

// mergeVehicleAttrs aplica um patch sobre os attrs atuais, devolvendo um
// snapshot do estado resultante. Campos do patch que são nil mantêm o valor
// atual. Usado para derivar o name correto pós-update.
func mergeVehicleAttrs(cur, patch *VehicleAttrs) VehicleAttrs {
	out := VehicleAttrs{}
	if cur != nil {
		out = *cur
	}
	if patch == nil {
		return out
	}
	if patch.Category != nil {
		out.Category = patch.Category
	}
	if patch.Plate != nil {
		out.Plate = patch.Plate
	}
	if patch.Brand != nil {
		out.Brand = patch.Brand
	}
	if patch.Model != nil {
		out.Model = patch.Model
	}
	if patch.Color != nil {
		out.Color = patch.Color
	}
	if patch.Year != nil {
		out.Year = patch.Year
	}
	if patch.Chassis != nil {
		out.Chassis = patch.Chassis
	}
	if patch.Renavam != nil {
		out.Renavam = patch.Renavam
	}
	return out
}

// normalizePlate remove hífen, espaço e força uppercase. Aceita formato
// antigo (ABC-1234) e Mercosul (ABC1D23). Retorna string vazia se input
// vier vazio depois de limpar.
func normalizePlate(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' || c == ' ' || c == '.' {
			continue
		}
		if c >= 'a' && c <= 'z' {
			c = c - ('a' - 'A')
		}
		out = append(out, c)
	}
	return string(out)
}

func updateChild(ctx context.Context, tx *sql.Tx, id string, k Kind, p Patch) error {
	switch k {
	case KindPerson:
		if p.Person == nil {
			return nil
		}
		a := p.Person
		// Para attrs polimórficos, semântica é "tudo enviado vira valor".
		// Campos nil = mantém. Aliases nil = mantém; Aliases [] = limpa.
		// photo_path NÃO é atualizado por aqui — fluxo dedicado de upload.
		_, err := tx.ExecContext(ctx, `
			UPDATE app.entity_persons SET
			  aliases       = COALESCE($1, aliases),
			  gender        = COALESCE($2, gender),
			  date_of_birth = COALESCE($3, date_of_birth),
			  mother_name   = COALESCE($4, mother_name),
			  cpf           = COALESCE($5, cpf),
			  orcrim_id     = COALESCE($6, orcrim_id)
			WHERE entity_id = $7`,
			nilAliases(a.Aliases), nilStr(a.Gender), nilTime(a.DateOfBirth),
			nilStr(a.MotherName), nilStr(a.CPF), nilUUID(a.OrcrimID),
			id,
		)
		return err
	case KindOrganization:
		if p.Organization == nil {
			return nil
		}
		a := p.Organization
		_, err := tx.ExecContext(ctx, `
			UPDATE app.entity_organizations SET
			  aliases    = COALESCE($1, aliases),
			  legal_name = COALESCE($2, legal_name),
			  tax_id     = COALESCE($3, tax_id),
			  founded_at = COALESCE($4, founded_at)
			WHERE entity_id = $5`,
			nilAliases(a.Aliases),
			nilStr(a.LegalName), nilStr(a.TaxID), nilTime(a.FoundedAt), id,
		)
		return err
	case KindPlace:
		if p.Place == nil {
			return nil
		}
		a := p.Place
		_, err := tx.ExecContext(ctx, `
			UPDATE app.entity_places SET
			  address   = COALESCE($1, address),
			  country   = COALESCE($2, country),
			  region    = COALESCE($3, region),
			  latitude  = COALESCE($4, latitude),
			  longitude = COALESCE($5, longitude)
			WHERE entity_id = $6`,
			nilStr(a.Address), nilStr(a.Country), nilStr(a.Region),
			nilFloat(a.Latitude), nilFloat(a.Longitude), id,
		)
		return err
	case KindVehicle:
		if p.Vehicle == nil {
			return nil
		}
		a := p.Vehicle
		var plate any
		if a.Plate != nil {
			s := normalizePlate(*a.Plate)
			if s != "" {
				plate = s
			}
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE app.entity_vehicles SET
			  category = COALESCE($1, category),
			  plate    = COALESCE($2, plate),
			  brand    = COALESCE($3, brand),
			  model    = COALESCE($4, model),
			  color    = COALESCE($5, color),
			  year     = COALESCE($6, year),
			  chassis  = COALESCE($7, chassis),
			  renavam  = COALESCE($8, renavam)
			WHERE entity_id = $9`,
			nilStr(a.Category), plate, nilStr(a.Brand), nilStr(a.Model), nilStr(a.Color),
			nullableInt(a.Year), nilStr(a.Chassis), nilStr(a.Renavam), id,
		)
		return err
	}
	return ErrInvalidKind
}

// replaceTags faz delete-then-insert das tags. Normaliza para lowercase trim.
// DELETE em app.entity_tags é concedido a tevunah_app via migration 00011 —
// o histórico da mudança fica no audit_log do update da entidade.
func replaceTags(ctx context.Context, tx *sql.Tx, id string, tags []string, by string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM app.entity_tags WHERE entity_id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tags: %w", err)
	}
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.entity_tags (entity_id, tag, added_by) VALUES ($1, $2, $3)`,
			id, t, by,
		); err != nil {
			return fmt.Errorf("insert tag %s: %w", t, err)
		}
	}
	return nil
}

// ─── utilitários de conversão ───

func nullableString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullableStringPtr(p *string) any {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return ""
	}
	return v
}

func nullableTrimmedString(p *string) any {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return v
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nilStr(p *string) any {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
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

func nilTime(p *time.Time) any {
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

// nilUUID converte string opcional para parâmetro de coluna UUID, tratando
// string vazia como NULL.
func nilUUID(p *string) any {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return v
}

func nilAliases(a []string) any {
	if a == nil {
		return nil
	}
	return a
}
