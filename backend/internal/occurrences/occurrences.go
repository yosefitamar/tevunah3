// Package occurrences é a visão única do módulo de Ocorrências: reúne o
// cadastro manual (app.incidents) e o que foi importado do relatório
// operacional (app.ops_occurrences).
//
// As duas tabelas seguem separadas e cada uma tem o seu pacote (incidents,
// opsreport) — uma é curadoria do analista, a outra é cópia fiel do PDF e
// origem do envio ao SIPOM. O que as une é a ficha CIOPS: mesma ficha, mesma
// ocorrência. Aqui ficam só as leituras que atravessam as duas: a listagem
// unificada e a detecção de duplicidade.
package occurrences

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Origem da ocorrência.
const (
	SourceManual = "manual"      // cadastro em app.incidents
	SourceOps    = "operacional" // importada do relatório operacional
)

// Scope diz quais fontes o solicitante pode ler (incident.read e
// opsreport.read). Fonte fora do escopo não entra em nenhuma consulta.
type Scope struct {
	Manual bool
	Ops    bool
}

// Repo lê app.incidents e app.ops_occurrences em conjunto.
type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// ─────────────────────────── Listagem unificada ────────────────────────────

// Row é uma linha da listagem. Tem IncidentID, OpsID ou os dois — os dois
// quando a ficha do cadastro manual é a mesma de uma ocorrência do relatório,
// caso em que os campos curados do cadastro prevalecem.
type Row struct {
	IncidentID   *string
	OpsID        *string
	Type         string   // tipo do cadastro manual; "" se só do relatório
	Natures      []string // naturezas do relatório; vazio se só manual
	OccurredOn   time.Time
	Time         string // "HH:MM" ou ""
	CIOPS        string
	City         string
	Neighborhood string
	// Description é a descrição do cadastro; sem ela, o histórico do relatório.
	Description string
	Means       string
	Intel       bool
	HasGeo      bool
	PeopleCount int
	UpdatedAt   time.Time
	// Pending são as pendências da ocorrência do relatório para o envio ao
	// SIPOM (códigos de internal/sipom); vazio se a linha é só do cadastro.
	Pending []string
	// Coordenada: a do cadastro manual; sem ela, a da ocorrência do
	// relatório. GeoPrecision só vale para a do relatório ("bairro" = ponto
	// aproximado); vazio quando a coordenada é do cadastro ou do analista.
	Latitude     *float64
	Longitude    *float64
	GeoPrecision string
}

// Abas da tela: o cadastro de homicídios (CVLI) de um lado; do outro, a
// produtividade — prisões e apreensões do cadastro e tudo o que veio do
// relatório operacional.
const (
	CategoryHomicides    = "homicidios"
	CategoryProductivity = "produtividade"
)

// ListOpts controla a listagem.
type ListOpts struct {
	Limit, Offset int
	Category      string // "" = todas | CategoryHomicides | CategoryProductivity
	Source        string // "" = todas | SourceManual | SourceOps
	Type          string // tipo do cadastro manual
	Means         string
	City          string
	Neighborhood  string
	Search        string
	DateFrom      string // YYYY-MM-DD
	DateTo        string
	SortBy        string // occurred_on | type | updated_at
	SortDir       string
}

// Counts é o total de cada aba no recorte comum às duas (busca, período e
// território) — o que a tela mostra no rótulo das abas.
type Counts struct {
	Productivity int
	Homicides    int
}

// ListResult é uma página da listagem, o total do recorte e o de cada aba.
type ListResult struct {
	Items  []Row
	Total  int
	Counts Counts
}

// unified é a união das duas fontes, uma linha por ocorrência. $1 e $2 são o
// escopo (manual, operacional).
//
// Primeiro ramo: todo cadastro manual, com a ocorrência do relatório de mesma
// ficha ao lado (se houver). Segundo ramo: as do relatório que não têm
// cadastro manual correspondente. A ficha é comparada por app.norm_ciops().
const unified = `
	WITH u AS (
	  SELECT i.id AS incident_id, o.id AS ops_id, i.type AS type,
	         COALESCE(o.natures, '{}') AS natures,
	         i.occurred_on AS occurred_on,
	         COALESCE(i.occurred_time, o.start_time) AS occurred_time,
	         i.ciops_record AS ciops_record,
	         CASE WHEN i.city <> '' THEN i.city ELSE COALESCE(o.place_city, '') END AS city,
	         CASE WHEN i.neighborhood <> '' THEN i.neighborhood
	              ELSE COALESCE(o.place_neighborhood, '') END AS neighborhood,
	         i.description AS description,
	         COALESCE(o.narrative, '') AS narrative,
	         i.means AS means,
	         (i.intel_participation OR COALESCE(o.intel_participation, false)) AS intel,
	         i.updated_at AS updated_at,
	         COALESCE(o.sipom_pendencias, '{}') AS pending,
	         CASE WHEN i.latitude IS NOT NULL AND i.longitude IS NOT NULL
	              THEN i.latitude ELSE o.latitude END AS latitude,
	         CASE WHEN i.latitude IS NOT NULL AND i.longitude IS NOT NULL
	              THEN i.longitude ELSE o.longitude END AS longitude,
	         CASE WHEN i.latitude IS NOT NULL AND i.longitude IS NOT NULL
	              THEN '' ELSE COALESCE(o.geo_precision, '') END AS geo_precision
	    FROM app.incidents i
	    LEFT JOIN app.ops_occurrences o
	      ON $2 AND o.deleted_at IS NULL
	     AND o.ciops_record <> '' AND i.ciops_record <> ''
	     AND app.norm_ciops(i.ciops_record) <> ''
	     AND app.norm_ciops(o.ciops_record) = app.norm_ciops(i.ciops_record)
	   WHERE $1 AND i.deleted_at IS NULL
	  UNION ALL
	  SELECT NULL::uuid, o.id, '', o.natures, o.occurred_on, o.start_time, o.ciops_record,
	         o.place_city, o.place_neighborhood, '', o.narrative, '',
	         o.intel_participation, o.created_at, o.sipom_pendencias,
	         o.latitude, o.longitude, o.geo_precision
	    FROM app.ops_occurrences o
	   WHERE $2 AND o.deleted_at IS NULL
	     AND NOT ($1 AND o.ciops_record <> '' AND app.norm_ciops(o.ciops_record) <> ''
	              AND EXISTS (
	                SELECT 1 FROM app.incidents i
	                 WHERE i.deleted_at IS NULL AND i.ciops_record <> ''
	                   AND app.norm_ciops(i.ciops_record) = app.norm_ciops(o.ciops_record)))
	)`

var sortable = map[string]string{
	"occurred_on": "u.occurred_on %[1]s, u.occurred_time %[1]s NULLS LAST",
	"type":        "u.type %[1]s, u.occurred_on DESC",
	"updated_at":  "u.updated_at %[1]s",
}

// sharedWhere é o recorte comum às abas ($3–$9): território, período e a
// busca. A busca alcança os dois lados: descrição e histórico, ficha,
// naturezas e as pessoas — vinculadas ao cadastro ou citadas no relatório.
const sharedWhere = `
	WHERE ($3 = '' OR upper(u.city) = $3)
	  AND ($4 = '' OR upper(u.neighborhood) = $4)
	  AND ($5::date IS NULL OR u.occurred_on >= $5::date)
	  AND ($6::date IS NULL OR u.occurred_on <= $6::date)
	  AND ($7 = ''
	       OR app.norm_txt(u.description) LIKE app.norm_txt($8)
	       OR app.norm_txt(u.narrative) LIKE app.norm_txt($8)
	       OR app.norm_txt(array_to_string(u.natures, ' ')) LIKE app.norm_txt($8)
	       OR lower(u.ciops_record) LIKE $8
	       OR ($9 <> '' AND app.norm_ciops(u.ciops_record) LIKE '%' || $9 || '%')
	       OR (u.incident_id IS NOT NULL AND EXISTS (
	             SELECT 1 FROM app.incident_entities ie
	               JOIN app.entities e ON e.id = ie.entity_id
	               LEFT JOIN app.entity_persons p ON p.entity_id = e.id
	              WHERE ie.incident_id = u.incident_id
	                AND (app.norm_txt(e.name) LIKE app.norm_txt($8)
	                     OR lower(COALESCE(p.cpf, '')) LIKE $8
	                     OR EXISTS (SELECT 1 FROM unnest(COALESCE(p.aliases, '{}')) al
	                                 WHERE app.norm_txt(al) LIKE app.norm_txt($8)))))
	       OR (u.ops_id IS NOT NULL AND (
	             EXISTS (SELECT 1 FROM app.ops_occurrence_people p
	                      WHERE p.occurrence_id = u.ops_id
	                        AND (app.norm_txt(p.name) LIKE app.norm_txt($8)
	                             OR app.norm_txt(p.mother_name) LIKE app.norm_txt($8)))
	             OR EXISTS (SELECT 1 FROM app.ops_occurrence_vehicles v
	                         WHERE v.occurrence_id = u.ops_id AND lower(v.plate) LIKE $8)
	             OR EXISTS (SELECT 1 FROM app.ops_occurrence_weapons w
	                         WHERE w.occurrence_id = u.ops_id AND lower(w.serial) LIKE $8))))`

// tabWhere é o recorte da aba ($10–$13): categoria, origem, tipo e meio.
const tabWhere = `
	  AND ($10 = '' OR ($10 = 'homicidios' AND u.type = 'homicidio')
	                OR ($10 = 'produtividade' AND u.type <> 'homicidio'))
	  AND ($11 = '' OR ($11 = 'manual' AND u.incident_id IS NOT NULL)
	                OR ($11 = 'operacional' AND u.ops_id IS NOT NULL))
	  AND ($12 = '' OR u.type = $12)
	  AND ($13 = '' OR u.means = $13)`

// sharedArgs é quantos dos argumentos de listArgs pertencem a sharedWhere.
const sharedArgs = 9

// listArgs monta $1–$13 para unified + sharedWhere + tabWhere.
func listArgs(sc Scope, opts ListOpts) []any {
	search := strings.TrimSpace(opts.Search)
	return []any{
		sc.Manual, // $1
		sc.Ops,    // $2
		strings.ToUpper(strings.TrimSpace(opts.City)),         // $3
		strings.ToUpper(strings.TrimSpace(opts.Neighborhood)), // $4
		nilStr(strings.TrimSpace(opts.DateFrom)),              // $5
		nilStr(strings.TrimSpace(opts.DateTo)),                // $6
		search,                                                // $7
		"%" + strings.ToLower(search) + "%",                   // $8
		NormCIOPS(search),                                     // $9
		strings.TrimSpace(opts.Category),                      // $10
		strings.TrimSpace(opts.Source),                        // $11
		strings.TrimSpace(opts.Type),                          // $12
		strings.TrimSpace(opts.Means),                         // $13
	}
}

const rowSelect = `
	SELECT u.incident_id, u.ops_id, u.type, to_jsonb(u.natures)::text,
	       u.occurred_on, COALESCE(to_char(u.occurred_time, 'HH24:MI'), ''),
	       u.ciops_record, u.city, u.neighborhood,
	       CASE WHEN u.description <> '' THEN u.description ELSE u.narrative END,
	       u.means, u.intel, u.updated_at,
	       CASE WHEN u.incident_id IS NOT NULL
	            THEN (SELECT COUNT(*) FROM app.incident_entities ie WHERE ie.incident_id = u.incident_id)
	            ELSE (SELECT COUNT(*) FROM app.ops_occurrence_people p WHERE p.occurrence_id = u.ops_id)
	       END,
	       to_jsonb(u.pending)::text,
	       u.latitude, u.longitude, u.geo_precision
	  FROM u`

func scanRow(rows *sql.Rows) (*Row, error) {
	var row Row
	var incID, opsID sql.NullString
	var natures, pending string
	if err := rows.Scan(&incID, &opsID, &row.Type, &natures,
		&row.OccurredOn, &row.Time, &row.CIOPS, &row.City, &row.Neighborhood,
		&row.Description, &row.Means, &row.Intel, &row.UpdatedAt,
		&row.PeopleCount, &pending,
		&row.Latitude, &row.Longitude, &row.GeoPrecision); err != nil {
		return nil, err
	}
	if incID.Valid {
		row.IncidentID = &incID.String
	}
	if opsID.Valid {
		row.OpsID = &opsID.String
	}
	if err := decodeStrings(natures, &row.Natures); err != nil {
		return nil, err
	}
	if err := decodeStrings(pending, &row.Pending); err != nil {
		return nil, err
	}
	row.HasGeo = row.Latitude != nil && row.Longitude != nil
	return &row, nil
}

// List devolve uma página da listagem unificada, o total do recorte e o de
// cada aba.
func (r *Repo) List(ctx context.Context, sc Scope, opts ListOpts) (*ListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 25
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	args := listArgs(sc, opts)
	where := sharedWhere + tabWhere

	res := &ListResult{Items: []Row{}}
	if err := r.db.QueryRowContext(ctx,
		unified+` SELECT COUNT(*) FROM u`+where, args...).Scan(&res.Total); err != nil {
		return nil, fmt.Errorf("occurrences count: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, unified+`
		SELECT COUNT(*) FILTER (WHERE u.type <> 'homicidio'),
		       COUNT(*) FILTER (WHERE u.type = 'homicidio')
		  FROM u`+sharedWhere, args[:sharedArgs]...,
	).Scan(&res.Counts.Productivity, &res.Counts.Homicides); err != nil {
		return nil, fmt.Errorf("occurrences tab counts: %w", err)
	}

	order, ok := sortable[opts.SortBy]
	if !ok {
		order = sortable["occurred_on"]
	}
	dir := "DESC"
	if strings.ToLower(opts.SortDir) == "asc" {
		dir = "ASC"
	}
	rows, err := r.db.QueryContext(ctx, unified+rowSelect+where+`
		 ORDER BY `+fmt.Sprintf(order, dir)+`, u.updated_at DESC
		 LIMIT $14 OFFSET $15`,
		append(args, opts.Limit, opts.Offset)...)
	if err != nil {
		return nil, fmt.Errorf("occurrences list: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		row, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		res.Items = append(res.Items, *row)
	}
	return res, rows.Err()
}

// geoMaxPoints limita os pontos do mapa: um recorte largo numa base grande
// não deve derrubar o navegador. Quem chama avisa quando truncou.
const geoMaxPoints = 5000

// ListGeo devolve as ocorrências georreferenciadas do recorte, para o mapa —
// sem paginação (o período é o limitador). O total é o do recorte inteiro,
// com ou sem coordenada, para a tela dizer quantas ficaram de fora; truncated
// avisa que o teto de pontos cortou o resultado.
func (r *Repo) ListGeo(ctx context.Context, sc Scope, opts ListOpts) (items []Row, total int, truncated bool, err error) {
	args := listArgs(sc, opts)
	where := sharedWhere + tabWhere
	if err := r.db.QueryRowContext(ctx,
		unified+` SELECT COUNT(*) FROM u`+where, args...).Scan(&total); err != nil {
		return nil, 0, false, fmt.Errorf("occurrences geo count: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, unified+rowSelect+where+`
		   AND u.latitude IS NOT NULL AND u.longitude IS NOT NULL
		 ORDER BY u.occurred_on DESC, u.occurred_time DESC NULLS LAST
		 LIMIT $14`, append(args, geoMaxPoints+1)...)
	if err != nil {
		return nil, 0, false, fmt.Errorf("occurrences geo: %w", err)
	}
	defer rows.Close()
	items = []Row{}
	for rows.Next() {
		row, err := scanRow(rows)
		if err != nil {
			return nil, 0, false, err
		}
		items = append(items, *row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}
	if truncated = len(items) > geoMaxPoints; truncated {
		items = items[:geoMaxPoints]
	}
	return items, total, truncated, nil
}

// PlaceFacet é um município (Neighborhood vazio) ou um bairro dentro de um
// município, com a contagem de ocorrências.
type PlaceFacet struct {
	City         string
	Neighborhood string
	Count        int
}

// Locations devolve os municípios e bairros presentes na listagem unificada
// — o que popula o recorte territorial da tela.
func (r *Repo) Locations(ctx context.Context, sc Scope) (cities, neighborhoods []PlaceFacet, err error) {
	rows, err := r.db.QueryContext(ctx, unified+`
		SELECT upper(city), '' AS neighborhood, count(*)::int
		  FROM u WHERE city <> '' GROUP BY 1
		UNION ALL
		SELECT upper(city), upper(neighborhood), count(*)::int
		  FROM u WHERE city <> '' AND neighborhood <> '' GROUP BY 1, 2
		 ORDER BY 1, 2`, sc.Manual, sc.Ops)
	if err != nil {
		return nil, nil, fmt.Errorf("occurrences locations: %w", err)
	}
	defer rows.Close()
	cities, neighborhoods = []PlaceFacet{}, []PlaceFacet{}
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

// ─────────────────────────── Ficha e duplicidade ────────────────────────────

// Candidate é uma ocorrência já gravada, vista como possível repetição de
// outra.
type Candidate struct {
	Source       string // SourceManual | SourceOps
	ID           string
	Type         string   // só manual
	Natures      []string // só operacional
	OccurredOn   time.Time
	Time         string
	CIOPS        string
	City         string
	Neighborhood string
	// Reasons explica a suspeita (Reason*); vazio quando a ficha é a mesma.
	Reasons []string
}

func (c *Candidate) subject() Subject {
	return Subject{CIOPS: c.CIOPS, OccurredOn: c.OccurredOn, Time: c.Time,
		City: c.City, Neighborhood: c.Neighborhood}
}

const candidateManual = `
	SELECT 'manual', i.id::text, i.type, '[]', i.occurred_on,
	       COALESCE(to_char(i.occurred_time, 'HH24:MI'), ''),
	       i.ciops_record, i.city, i.neighborhood
	  FROM app.incidents i`

const candidateOps = `
	SELECT 'operacional', o.id::text, '', to_jsonb(o.natures)::text, o.occurred_on,
	       COALESCE(to_char(o.start_time, 'HH24:MI'), ''),
	       o.ciops_record, o.place_city, o.place_neighborhood
	  FROM app.ops_occurrences o`

func scanCandidate(rows *sql.Rows) (*Candidate, error) {
	var c Candidate
	var natures string
	if err := rows.Scan(&c.Source, &c.ID, &c.Type, &natures, &c.OccurredOn, &c.Time,
		&c.CIOPS, &c.City, &c.Neighborhood); err != nil {
		return nil, err
	}
	if err := decodeStrings(natures, &c.Natures); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repo) one(ctx context.Context, q string, args ...any) (*Candidate, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	return scanCandidate(rows)
}

// IncidentByCIOPS devolve o cadastro manual que já usa a ficha (nil se
// nenhum). excludeID tira da busca a própria ocorrência em edição.
func (r *Repo) IncidentByCIOPS(ctx context.Context, ciops, excludeID string) (*Candidate, error) {
	k := NormCIOPS(ciops)
	if k == "" {
		return nil, nil
	}
	return r.one(ctx, candidateManual+`
		 WHERE i.deleted_at IS NULL AND i.ciops_record <> ''
		   AND app.norm_ciops(i.ciops_record) = $1
		   AND ($2 = '' OR i.id::text <> $2)
		 ORDER BY i.created_at
		 LIMIT 1`, k, excludeID)
}

// OpsByCIOPS devolve a ocorrência do relatório operacional com a ficha (nil
// se nenhuma).
func (r *Repo) OpsByCIOPS(ctx context.Context, ciops string) (*Candidate, error) {
	k := NormCIOPS(ciops)
	if k == "" {
		return nil, nil
	}
	return r.one(ctx, candidateOps+`
		 WHERE o.deleted_at IS NULL AND o.ciops_record <> ''
		   AND app.norm_ciops(o.ciops_record) = $1
		 LIMIT 1`, k)
}

// IncidentsByCIOPS resolve várias fichas de uma vez: ficha normalizada → id
// do cadastro manual que a usa. Serve à prévia da importação.
func (r *Repo) IncidentsByCIOPS(ctx context.Context, fichas []string) (map[string]string, error) {
	out := map[string]string{}
	keys := make([]string, 0, len(fichas))
	for _, f := range fichas {
		if k := NormCIOPS(f); k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT app.norm_ciops(i.ciops_record), i.id::text
		  FROM app.incidents i
		 WHERE i.deleted_at IS NULL AND i.ciops_record <> ''
		   AND app.norm_ciops(i.ciops_record) = ANY($1)
		 ORDER BY i.created_at DESC`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, id string
		if err := rows.Scan(&k, &id); err != nil {
			return nil, err
		}
		out[k] = id
	}
	return out, rows.Err()
}

// DuplicateQuery descreve a busca por possíveis repetições.
type DuplicateQuery struct {
	Subject
	// ExcludeIncidentID tira da comparação o próprio cadastro (edição).
	ExcludeIncidentID string
	// Scope: em quais fontes procurar. Na importação do relatório só o
	// cadastro manual entra — duas fichas do PDF nunca são erro de digitação.
	Scope Scope
}

// PossibleDuplicates procura ocorrências de ficha DIFERENTE que parecem ser a
// mesma que a informada (ver matchReasons). Ficha igual não entra: é a mesma
// ocorrência, tratada por IncidentByCIOPS/OpsByCIOPS.
func (r *Repo) PossibleDuplicates(ctx context.Context, q DuplicateQuery) ([]Candidate, error) {
	if q.OccurredOn.IsZero() || (!q.Scope.Manual && !q.Scope.Ops) {
		return []Candidate{}, nil
	}
	// A janela de um dia para cada lado cobre a virada da meia-noite; a
	// regra fina (hora, lugar, ficha) roda em matchReasons.
	rows, err := r.db.QueryContext(ctx, `
		SELECT * FROM (`+candidateManual+`
		 WHERE $1 AND i.deleted_at IS NULL
		   AND i.occurred_on BETWEEN $3::date - 1 AND $3::date + 1
		   AND ($4 = '' OR i.id::text <> $4)
		UNION ALL`+candidateOps+`
		 WHERE $2 AND o.deleted_at IS NULL
		   AND o.occurred_on BETWEEN $3::date - 1 AND $3::date + 1
		) c`,
		q.Scope.Manual, q.Scope.Ops, q.OccurredOn.Format("2006-01-02"), q.ExcludeIncidentID)
	if err != nil {
		return nil, fmt.Errorf("occurrences duplicates: %w", err)
	}
	defer rows.Close()

	out := []Candidate{}
	for rows.Next() {
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		if c.Reasons = matchReasons(q.Subject, c.subject()); len(c.Reasons) > 0 {
			out = append(out, *c)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Mais indícios primeiro.
	sort.SliceStable(out, func(i, j int) bool { return len(out[i].Reasons) > len(out[j].Reasons) })
	return out, nil
}

// ─────────────────────────── helpers ────────────────────────────

func nilStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func decodeStrings(js string, dst *[]string) error {
	*dst = []string{}
	if js == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(js), dst); err != nil {
		return fmt.Errorf("decode array: %w", err)
	}
	return nil
}
