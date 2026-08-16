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

// Meios utilizados (CVLI). Aplicam-se a homicídio; "" = não informado.
const (
	MeansUnknown     = ""
	MeansPAF         = "paf"         // projétil de arma de fogo
	MeansArmaBranca  = "arma_branca" // faca, canivete, etc.
	MeansAsfixia     = "asfixia"     // asfixia / estrangulamento
	MeansContundente = "contundente" // objeto contundente
	MeansOutros      = "outros"
)

// IsValidMeans devolve true para um meio suportado ("" = não informado).
func IsValidMeans(m string) bool {
	switch m {
	case MeansUnknown, MeansPAF, MeansArmaBranca, MeansAsfixia,
		MeansContundente, MeansOutros:
		return true
	}
	return false
}

// Papéis do envolvido na ocorrência. Lista fechada — o papel é chave de
// leitura do caso e VÍTIMA implica óbito, então não pode variar por grafia.
// Quantas pessoas quiser em cada papel: várias vítimas, vários acusados,
// várias testemunhas na mesma ocorrência.
const (
	RoleVitima     = "VÍTIMA"
	RoleAcusado    = "ACUSADO"
	RoleTestemunha = "TESTEMUNHA"
)

// NormalizeRole devolve o papel canônico, tolerando caixa e a grafia sem
// acento. Devolve "" quando o valor não é um papel suportado.
func NormalizeRole(role string) string {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case RoleVitima, "VITIMA":
		return RoleVitima
	case RoleAcusado:
		return RoleAcusado
	case RoleTestemunha:
		return RoleTestemunha
	}
	return ""
}

// Erros públicos.
var (
	ErrNotFound       = errors.New("ocorrência não encontrada")
	ErrAlreadyDeleted = errors.New("ocorrência já excluída")
	ErrInvalidType    = errors.New("tipo inválido")
	ErrInvalidMeans   = errors.New("meio utilizado inválido")
	ErrInvalidRole    = errors.New("papel inválido")
)

// Incident é o registro consolidado (campos base + envolvidos).
type Incident struct {
	ID           string
	Type         string
	OccurredOn   time.Time
	OccurredTime *string // "HH:MM" (NULL = hora desconhecida)
	CIOPSRecord  string
	PhotoPath    *string
	Latitude     *float64
	Longitude    *float64
	// City/Neighborhood dão o recorte territorial (o par lat/long plota o
	// ponto, mas não agrega). Gravados em MAIÚSCULAS.
	City         string
	Neighborhood string
	Description  string
	// Means é o meio utilizado (relevante em homicídio); "" = não informado.
	Means       string
	MeansDetail string

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
	// Deceased permite à UI aplicar a tarja de óbito já na lista de
	// envolvidos, sem buscar cada entidade.
	Deceased bool
}

// NewIncident é o input do Create.
type NewIncident struct {
	Type         string
	OccurredOn   time.Time
	OccurredTime *string
	CIOPSRecord  string
	Latitude     *float64
	Longitude    *float64
	City         string
	Neighborhood string
	Description  string
	Means        string
	MeansDetail  string
	CreatedBy    string
}

// UpdateOpts é o input do Update. Campos nil = não tocar.
type UpdateOpts struct {
	Type            *string
	OccurredOn      *time.Time
	OccurredTime    *string // ponteiro p/ "HH:MM"; "" limpa a hora
	OccurredTimeSet bool    // distingue "não enviado" de "limpar"
	CIOPSRecord     *string
	Latitude        *float64
	LatitudeSet     bool
	Longitude       *float64
	LongitudeSet    bool
	City            *string // "" limpa
	CitySet         bool
	Neighborhood    *string // "" limpa
	NeighborhoodSet bool
	Description     *string
	Means           *string // "" limpa (volta a "não informado")
	MeansSet        bool
	MeansDetail     *string
	MeansDetailSet  bool
}

// ListOpts controla a listagem.
type ListOpts struct {
	Limit        int    // <= 100; default 25
	Offset       int    // default 0
	Type         string // vazio = todos
	Means        string // vazio = todos; "paf", "arma_branca", …
	City         string // vazio = todos (comparação exata, valor já em UPPER)
	Neighborhood string // vazio = todos
	Search       string // ILIKE em description/ciops_record
	DateFrom    string // YYYY-MM-DD; vazio = ignora
	DateTo      string // YYYY-MM-DD; vazio = ignora
	SortBy      string // "occurred_on"|"type"|"created_at"|"updated_at"
	SortDir     string // "asc"|"desc"; default "desc"
	OnlyDeleted bool
}

// searchClause casa o termo contra a própria ocorrência (descrição, ficha
// CIOPS) e contra quem está vinculado a ela (nome, alcunha e CPF). É o que
// permite achar um homicídio pela vítima ou pelo acusado, não só pelo texto
// do relato. `%s` recebe o índice do parâmetro com o termo em minúsculas.
func searchClause(termParam, likeParam string) string {
	return `(` + termParam + ` = '' OR lower(i.description) LIKE ` + likeParam + `
	          OR lower(i.ciops_record) LIKE ` + likeParam + `
	          OR EXISTS (
	               SELECT 1 FROM app.incident_entities ie
	               JOIN app.entities e ON e.id = ie.entity_id
	               LEFT JOIN app.entity_persons p ON p.entity_id = e.id
	              WHERE ie.incident_id = i.id
	                AND (lower(e.name) LIKE ` + likeParam + `
	                     OR lower(COALESCE(p.cpf, '')) LIKE ` + likeParam + `
	                     OR EXISTS (SELECT 1 FROM unnest(COALESCE(p.aliases, '{}')) al
	                                 WHERE lower(al) LIKE ` + likeParam + `))
	             ))`
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
	i.ciops_record, i.photo_path,
	i.latitude, i.longitude, i.city, i.neighborhood,
	i.description, i.means, i.means_detail,
	i.created_at, i.created_by, i.updated_at, i.updated_by,
	i.deleted_at, i.deleted_by`

// ─────────────────────────── Create ────────────────────────────

func (r *Repo) Create(ctx context.Context, in NewIncident) (*Incident, error) {
	if !IsValidType(in.Type) {
		return nil, ErrInvalidType
	}
	if !IsValidMeans(in.Means) {
		return nil, ErrInvalidMeans
	}
	if in.OccurredOn.IsZero() {
		return nil, errors.New("occurred_on é obrigatório")
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.incidents
		  (type, occurred_on, occurred_time, ciops_record,
		   latitude, longitude, city, neighborhood,
		   description, means, means_detail,
		   created_by, updated_by)
		VALUES ($1, $2, $3::time, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)
		RETURNING id`,
		in.Type, in.OccurredOn, nilTimeStr(in.OccurredTime),
		strings.TrimSpace(in.CIOPSRecord),
		nilFloat(in.Latitude), nilFloat(in.Longitude),
		upperTrim(in.City), upperTrim(in.Neighborhood),
		strings.TrimSpace(in.Description),
		in.Means, strings.TrimSpace(in.MeansDetail), in.CreatedBy,
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
		opts.Type,                      // $1
		strings.TrimSpace(opts.Search), // $2
		search,                         // $3
		upperTrim(opts.City),           // $4
		nilDateStr(opts.DateFrom),      // $5
		nilDateStr(opts.DateTo),        // $6
		strings.TrimSpace(opts.Means),  // $7
		upperTrim(opts.Neighborhood),   // $8
	}
	where := `
		WHERE ` + deletedClause + `
		  AND ($1 = '' OR i.type = $1)
		  AND ` + searchClause("$2", "$3") + `
		  AND ($4 = '' OR i.city = $4)
		  AND ($5::date IS NULL OR i.occurred_on >= $5::date)
		  AND ($6::date IS NULL OR i.occurred_on <= $6::date)
		  AND ($7 = '' OR i.means = $7)
		  AND ($8 = '' OR i.neighborhood = $8)`

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
			  LIMIT $9 OFFSET $10`,
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

// ─────────────────────────── Mapa (geo) ────────────────────────────

// geoMaxPoints limita o retorno do mapa. Recortes largos (ano inteiro) em
// bases grandes não devem derrubar o navegador; o handler informa quando
// truncou pra UI avisar o analista.
const geoMaxPoints = 5000

// GeoOpts controla a consulta de pontos do mapa do crime.
type GeoOpts struct {
	Type         string // vazio = todos
	Means        string // vazio = todos
	City         string // vazio = todos
	Neighborhood string // vazio = todos
	// Search casa descrição, ficha CIOPS e os envolvidos (nome, alcunha, CPF).
	Search   string
	DateFrom string // YYYY-MM-DD; vazio = ignora
	DateTo   string // YYYY-MM-DD; vazio = ignora
}

// ListGeo devolve as ocorrências georreferenciadas do recorte, já com os
// envolvidos resolvidos (carregados em lote, sem N+1), para plotagem no mapa.
// O segundo retorno indica se o resultado foi truncado em geoMaxPoints.
func (r *Repo) ListGeo(ctx context.Context, opts GeoOpts) ([]Incident, bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+incidentSelectFields+`
		  FROM app.incidents i
		 WHERE i.deleted_at IS NULL
		   AND i.latitude IS NOT NULL AND i.longitude IS NOT NULL
		   AND ($1 = '' OR i.type = $1)
		   AND ($2 = '' OR i.means = $2)
		   AND ($3 = '' OR i.city = $3)
		   AND ($4::date IS NULL OR i.occurred_on >= $4::date)
		   AND ($5::date IS NULL OR i.occurred_on <= $5::date)
		   AND ($7 = '' OR i.neighborhood = $7)
		   AND `+searchClause("$8", "$9")+`
		 ORDER BY i.occurred_on DESC, i.created_at DESC
		 LIMIT $6`,
		strings.TrimSpace(opts.Type), strings.TrimSpace(opts.Means),
		upperTrim(opts.City), nilDateStr(opts.DateFrom), nilDateStr(opts.DateTo),
		geoMaxPoints+1, upperTrim(opts.Neighborhood),
		strings.TrimSpace(opts.Search),
		"%"+strings.ToLower(strings.TrimSpace(opts.Search))+"%",
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	items := make([]Incident, 0)
	for rows.Next() {
		inc, err := scanIncidentRows(rows)
		if err != nil {
			return nil, false, err
		}
		items = append(items, *inc)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	truncated := len(items) > geoMaxPoints
	if truncated {
		items = items[:geoMaxPoints]
	}
	if len(items) == 0 {
		return items, truncated, nil
	}

	ids := make([]string, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	byIncident, err := r.entitiesByIncident(ctx, ids)
	if err != nil {
		return nil, false, err
	}
	for i := range items {
		items[i].Involved = byIncident[items[i].ID]
	}
	return items, truncated, nil
}

// ─────────────────────────── Facetas territoriais ────────────────────

// PlaceFacet é um município (com Neighborhood vazio) ou um bairro dentro de
// um município, com a contagem de ocorrências ativas.
type PlaceFacet struct {
	City         string
	Neighborhood string
	Count        int
}

// Locations devolve os municípios e bairros efetivamente usados nos
// registros ativos — é o que popula os filtros de recorte territorial.
// Filtrar por algo que não existe no acervo não teria utilidade, então a
// lista sai dos próprios dados, e não da lista fechada de municípios da UI.
func (r *Repo) Locations(ctx context.Context) (cities []PlaceFacet, neighborhoods []PlaceFacet, err error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT city, '' AS neighborhood, count(*)::int
		  FROM app.incidents
		 WHERE deleted_at IS NULL AND city <> ''
		 GROUP BY city
		UNION ALL
		SELECT city, neighborhood, count(*)::int
		  FROM app.incidents
		 WHERE deleted_at IS NULL AND city <> '' AND neighborhood <> ''
		 GROUP BY city, neighborhood
		 ORDER BY 1, 2`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cities = make([]PlaceFacet, 0)
	neighborhoods = make([]PlaceFacet, 0)
	for rows.Next() {
		var f PlaceFacet
		if err := rows.Scan(&f.City, &f.Neighborhood, &f.Count); err != nil {
			return nil, nil, err
		}
		if f.Neighborhood == "" {
			cities = append(cities, f)
		} else {
			neighborhoods = append(neighborhoods, f)
		}
	}
	return cities, neighborhoods, rows.Err()
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
	if p.MeansSet && p.Means != nil && !IsValidMeans(*p.Means) {
		return nil, ErrInvalidMeans
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

	// means/means_detail também usam flags *Set: "" é valor legítimo (volta a
	// "não informado"), então COALESCE não serve.
	meansArg := ""
	if p.Means != nil {
		meansArg = strings.TrimSpace(*p.Means)
	}
	detailArg := ""
	if p.MeansDetail != nil {
		detailArg = strings.TrimSpace(*p.MeansDetail)
	}

	// city/neighborhood seguem a mesma regra: "" limpa o campo.
	cityArg := ""
	if p.City != nil {
		cityArg = upperTrim(*p.City)
	}
	neighborhoodArg := ""
	if p.Neighborhood != nil {
		neighborhoodArg = upperTrim(*p.Neighborhood)
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE app.incidents SET
		  type          = COALESCE($1, type),
		  occurred_on   = COALESCE($2, occurred_on),
		  occurred_time = CASE WHEN $3 THEN $4::time ELSE occurred_time END,
		  ciops_record  = COALESCE($5, ciops_record),
		  latitude      = CASE WHEN $6 THEN $7::double precision ELSE latitude END,
		  longitude     = CASE WHEN $8 THEN $9::double precision ELSE longitude END,
		  description   = COALESCE($10, description),
		  means         = CASE WHEN $13 THEN $14::text ELSE means END,
		  means_detail  = CASE WHEN $15 THEN $16::text ELSE means_detail END,
		  city          = CASE WHEN $17 THEN $18::text ELSE city END,
		  neighborhood  = CASE WHEN $19 THEN $20::text ELSE neighborhood END,
		  updated_at    = now(),
		  updated_by    = $11
		WHERE id = $12 AND deleted_at IS NULL`,
		nilStrP(p.Type), nilTimePtr(p.OccurredOn),
		useTime, timeArg,
		nilTrimP(p.CIOPSRecord),
		useLat, latArg, useLng, lngArg,
		nilTrimP(p.Description), updatedBy, id,
		p.MeansSet, meansArg, p.MeansDetailSet, detailArg,
		p.CitySet, cityArg, p.NeighborhoodSet, neighborhoodArg,
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
		       COALESCE(p.photo_path, pl.photo_path, v.photo_path) IS NOT NULL AS has_photo,
		       COALESCE(p.deceased, false)
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
			&ie.Version, &ie.AddedAt, &ie.HasPhoto, &ie.Deceased); err != nil {
			return nil, err
		}
		out = append(out, ie)
	}
	return out, rows.Err()
}

// entitiesByIncident carrega os envolvidos de várias ocorrências numa única
// query, agrupados por incident_id (usado pelo mapa, que plota N pontos).
func (r *Repo) entitiesByIncident(ctx context.Context, incidentIDs []string) (map[string][]InvolvedEntity, error) {
	out := make(map[string][]InvolvedEntity, len(incidentIDs))
	if len(incidentIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT ie.incident_id, ie.entity_id, e.name, e.kind, ie.role, e.version, ie.added_at,
		       COALESCE(p.photo_path, pl.photo_path, v.photo_path) IS NOT NULL AS has_photo,
		       COALESCE(p.deceased, false)
		  FROM app.incident_entities ie
		  JOIN app.entities e ON e.id = ie.entity_id
		  LEFT JOIN app.entity_persons   p  ON p.entity_id  = e.id
		  LEFT JOIN app.entity_places    pl ON pl.entity_id = e.id
		  LEFT JOIN app.entity_vehicles  v  ON v.entity_id  = e.id
		 WHERE ie.incident_id = ANY($1::uuid[])
		 ORDER BY ie.added_at`, incidentIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var incidentID string
		var ie InvolvedEntity
		if err := rows.Scan(&incidentID, &ie.EntityID, &ie.Name, &ie.Kind,
			&ie.Role, &ie.Version, &ie.AddedAt, &ie.HasPhoto, &ie.Deceased); err != nil {
			return nil, err
		}
		out[incidentID] = append(out[incidentID], ie)
	}
	return out, rows.Err()
}

// AddEntity vincula uma entidade à ocorrência (upsert do papel se já existe).
func (r *Repo) AddEntity(ctx context.Context, incidentID, entityID, role, addedBy string) error {
	canonical := NormalizeRole(role)
	if canonical == "" {
		return ErrInvalidRole
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO app.incident_entities (incident_id, entity_id, role, added_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (incident_id, entity_id)
		DO UPDATE SET role = EXCLUDED.role`,
		incidentID, entityID, canonical, addedBy)
	return err
}

// RemoveEntity desfaz o vínculo.
//
// Se a pessoa tinha o óbito atribuído a ESTA ocorrência, a referência é
// limpa: a ocorrência volta ao estado "vítima não identificada". O óbito em
// si permanece — desfazer o vínculo não ressuscita ninguém, e desmarcá-lo é
// ação explícita na ficha da entidade.
func (r *Repo) RemoveEntity(ctx context.Context, incidentID, entityID string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM app.incident_entities WHERE incident_id = $1 AND entity_id = $2`,
		incidentID, entityID); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE app.entity_persons
		   SET death_incident_id = NULL
		 WHERE entity_id = $1 AND death_incident_id = $2`,
		entityID, incidentID)
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
		&inc.CIOPSRecord, &photoPath,
		&lat, &lng, &inc.City, &inc.Neighborhood,
		&inc.Description, &inc.Means, &inc.MeansDetail,
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

// upperTrim normaliza campos textuais para MAIÚSCULAS sem espaços nas
// pontas — mesma convenção do módulo de entidades, o que mantém município
// e bairro agregáveis por comparação exata.
func upperTrim(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
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
