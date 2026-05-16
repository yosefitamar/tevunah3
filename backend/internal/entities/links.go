package entities

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RelationType enumera as relações suportadas entre entidades. Fechado por
// CHECK na tabela; mudanças exigem migration que expanda o conjunto.
//
// Direcionais: o sentido from→to carrega a semântica (ex.: "from é MEMBRO DE
// to"). A UI traduz pra rótulo bidirecional quando exibe um link na perspectiva
// inversa. Simétricos: from/to são intercambiáveis na semântica, mas a coluna
// física continua direcional pra preservar a unique index e o audit.
type RelationType string

const (
	// Genérico — fallback usado quando nenhuma relação tipada se encaixa.
	RelationAssociatedWith RelationType = "associated_with"

	// Direcionais — "from possui to" (proprietário de veículo).
	RelationOwns RelationType = "owns"

	// Pessoa ↔ Pessoa (simétricos).
	RelationSpouse    RelationType = "spouse"
	RelationRelative  RelationType = "relative"
	RelationFriend    RelationType = "friend"
	RelationColleague RelationType = "colleague"
	RelationPartner   RelationType = "partner"

	// Pessoa → Organização (direcionais).
	RelationMemberOf   RelationType = "member_of"
	RelationLeaderOf   RelationType = "leader_of"
	RelationEmployeeOf RelationType = "employee_of"

	// Pessoa → Veículo (direcional).
	RelationDrives RelationType = "drives"

	// Pessoa → Lugar (direcional).
	RelationFrequents RelationType = "frequents"

	// Organização ↔ Organização.
	RelationSubsidiaryOf RelationType = "subsidiary_of" // direcional
	RelationPartnership  RelationType = "partnership"   // simétrico

	// Organização → Lugar (direcional).
	RelationBasedAt RelationType = "based_at"

	// Parentais (direcionais, pessoa → pessoa). from = pai/mãe; to = filho(a).
	// O sentido permite ao layout do grafo posicionar pais acima dos filhos.
	RelationFatherOf RelationType = "father_of"
	RelationMotherOf RelationType = "mother_of"

	// Irmandade (simétricas, pessoa ↔ pessoa). Inseridas automaticamente pela
	// rotina ResyncSiblings sempre que um vínculo parental é criado:
	//   sibling      = ambos pais em comum.
	//   half_sibling = apenas um dos pais em comum.
	RelationSibling     RelationType = "sibling"
	RelationHalfSibling RelationType = "half_sibling"
)

// relationDef descreve os kinds permitidos nas pontas de um relation_type.
// O conjunto from×to é cartesiano. O domain rejeita inserts fora desse par,
// blindando o vocabulário no servidor mesmo que o cliente esteja desatualizado.
type relationDef struct {
	fromKinds []Kind
	toKinds   []Kind
}

// allKinds é o conjunto irrestrito — usado por associated_with, que é o
// fallback "qualquer coisa com qualquer coisa".
var allKinds = []Kind{KindPerson, KindOrganization, KindPlace, KindVehicle}

// relationCatalog é a tabela canônica de pares permitidos. Mantida ordenada
// pela ordem do enum acima pra leitura rápida.
var relationCatalog = map[RelationType]relationDef{
	RelationAssociatedWith: {fromKinds: allKinds, toKinds: allKinds},

	RelationOwns: {
		fromKinds: []Kind{KindPerson, KindOrganization},
		toKinds:   []Kind{KindVehicle},
	},

	RelationSpouse:    {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPerson}},
	RelationRelative:  {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPerson}},
	RelationFriend:    {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPerson}},
	RelationColleague: {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPerson}},
	RelationPartner:   {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPerson}},

	RelationMemberOf:   {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindOrganization}},
	RelationLeaderOf:   {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindOrganization}},
	RelationEmployeeOf: {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindOrganization}},

	RelationDrives:    {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindVehicle}},
	RelationFrequents: {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPlace}},

	RelationSubsidiaryOf: {fromKinds: []Kind{KindOrganization}, toKinds: []Kind{KindOrganization}},
	RelationPartnership:  {fromKinds: []Kind{KindOrganization}, toKinds: []Kind{KindOrganization}},

	RelationBasedAt: {fromKinds: []Kind{KindOrganization}, toKinds: []Kind{KindPlace}},

	RelationFatherOf: {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPerson}},
	RelationMotherOf: {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPerson}},

	RelationSibling:     {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPerson}},
	RelationHalfSibling: {fromKinds: []Kind{KindPerson}, toKinds: []Kind{KindPerson}},
}

// IsValid devolve true para um RelationType suportado.
func (rt RelationType) IsValid() bool {
	_, ok := relationCatalog[rt]
	return ok
}

// allowsPair verifica se o tipo aceita esse par (fromKind, toKind). Usado em
// CreateLink pra rejeitar combinações sem sentido como "pessoa OWNS pessoa".
func (rt RelationType) allowsPair(from, to Kind) bool {
	def, ok := relationCatalog[rt]
	if !ok {
		return false
	}
	return containsKind(def.fromKinds, from) && containsKind(def.toKinds, to)
}

func containsKind(set []Kind, k Kind) bool {
	for _, x := range set {
		if x == k {
			return true
		}
	}
	return false
}

// Link representa uma aresta direcional entre duas entidades. A direção
// from→to é semântica do relation_type (ex.: "pessoa owns veículo" → from=pessoa).
type Link struct {
	ID            string
	FromEntityID  string
	ToEntityID    string
	RelationType  RelationType
	ValidFrom     *time.Time
	ValidTo       *time.Time
	Note          string
	CreatedAt     time.Time
	CreatedBy     string

	// Populados por join na listagem (não persistem).
	FromKind Kind
	FromName string
	ToKind   Kind
	ToName   string

	// Resumo dos attrs do veículo nas pontas — só preenchido quando a ponta
	// correspondente é kind=vehicle. Permite ao frontend renderizar rótulos
	// ricos ("MARCA MODELO COR · PLACA") sem N+1.
	FromVehicle *VehicleSummary
	ToVehicle   *VehicleSummary
}

// VehicleSummary é um snapshot mínimo dos attrs de um veículo, usado em
// payloads de relação pra evitar fetch adicional por linha.
type VehicleSummary struct {
	Plate    *string
	Brand    *string
	Model    *string
	Color    *string
	Category *string
}

// NewLink é o input do CreateLink.
type NewLink struct {
	FromEntityID string
	ToEntityID   string
	RelationType RelationType
	ValidFrom    *time.Time
	ValidTo      *time.Time
	Note         string
}

// Direction indica se o link sai (out) ou chega (in) na entidade consultada
// em ListLinksForEntity. Usado pela UI pra renderizar "vínculos saindo" vs
// "vínculos chegando".
type Direction string

const (
	DirectionOut Direction = "out"
	DirectionIn  Direction = "in"
)

// LinkWithDirection é o item retornado por ListLinksForEntity — anota o
// link com a perspectiva do entityID consultado.
type LinkWithDirection struct {
	Link
	Direction Direction
}

// Erros públicos do módulo de links.
var (
	ErrLinkNotFound      = errors.New("vínculo não encontrado")
	ErrLinkAlreadyExists = errors.New("vínculo já existe entre estas entidades com este tipo")
	ErrLinkInvalidType   = errors.New("tipo de relação inválido")
	ErrLinkInvalidPair   = errors.New("tipo de relação incompatível com as entidades informadas")
	ErrLinkSelfReference = errors.New("entidade não pode ser ligada a si mesma")
)

// CreateLink insere uma nova aresta. Valida o tipo, rejeita self-link e
// captura colisão de unique como ErrLinkAlreadyExists.
func (r *Repo) CreateLink(ctx context.Context, in NewLink, createdBy string) (*Link, error) {
	if !in.RelationType.IsValid() {
		return nil, ErrLinkInvalidType
	}
	if in.FromEntityID == in.ToEntityID {
		return nil, ErrLinkSelfReference
	}
	// Confirma que ambas entidades existem (não-deletadas) e captura o kind
	// das duas pontas pra validar o par contra o relation catalog.
	var fromKind, toKind string
	for _, ref := range []struct {
		id  string
		dst *string
	}{{in.FromEntityID, &fromKind}, {in.ToEntityID, &toKind}} {
		err := r.db.QueryRowContext(ctx,
			`SELECT kind FROM app.entities
			  WHERE id = $1 AND deleted_at IS NULL`, ref.id,
		).Scan(ref.dst)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("check entity %s: %w", ref.id, err)
		}
	}
	if !in.RelationType.allowsPair(Kind(fromKind), Kind(toKind)) {
		return nil, ErrLinkInvalidPair
	}

	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.entity_links
		  (from_entity_id, to_entity_id, relation_type, valid_from, valid_to,
		   note, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		in.FromEntityID, in.ToEntityID, string(in.RelationType),
		nilTime(in.ValidFrom), nilTime(in.ValidTo),
		nullableString(in.Note), createdBy,
	).Scan(&id)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key") {
			return nil, ErrLinkAlreadyExists
		}
		return nil, fmt.Errorf("insert link: %w", err)
	}

	return r.FindLink(ctx, id)
}

// FindLink busca um link pelo ID, hidratando os nomes/kinds das duas pontas.
// Não filtra deletados — útil pra audit pós-soft-delete.
func (r *Repo) FindLink(ctx context.Context, id string) (*Link, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT l.id, l.from_entity_id, l.to_entity_id, l.relation_type,
		       l.valid_from, l.valid_to, COALESCE(l.note,''), l.created_at, l.created_by,
		       ef.kind, ef.name, et.kind, et.name
		  FROM app.entity_links l
		  JOIN app.entities ef ON ef.id = l.from_entity_id
		  JOIN app.entities et ON et.id = l.to_entity_id
		 WHERE l.id = $1`, id)
	var l Link
	var validFrom, validTo sql.NullTime
	var fromKind, toKind string
	if err := row.Scan(
		&l.ID, &l.FromEntityID, &l.ToEntityID, (*string)(&l.RelationType),
		&validFrom, &validTo, &l.Note, &l.CreatedAt, &l.CreatedBy,
		&fromKind, &l.FromName, &toKind, &l.ToName,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrLinkNotFound
		}
		return nil, err
	}
	if validFrom.Valid {
		t := validFrom.Time
		l.ValidFrom = &t
	}
	if validTo.Valid {
		t := validTo.Time
		l.ValidTo = &t
	}
	l.FromKind = Kind(fromKind)
	l.ToKind = Kind(toKind)
	return &l, nil
}

// ListLinksForEntity devolve todos os vínculos vivos cujo from_entity_id OU
// to_entity_id é entityID. Anota cada item com Direction (out/in) na
// perspectiva da entidade consultada. Resultados ordenados por created_at desc.
//
// Filtro de clearance/classification da entidade do "outro lado" fica no
// handler HTTP que conhece o chamador.
func (r *Repo) ListLinksForEntity(ctx context.Context, entityID string) ([]LinkWithDirection, error) {
	// LEFT JOIN com entity_vehicles em ambas as pontas: traz placa/marca/
	// modelo/cor quando a ponta correspondente é kind=vehicle. Frontend
	// renderiza "MARCA MODELO COR · PLACA" sem precisar de fetch extra.
	rows, err := r.db.QueryContext(ctx, `
		SELECT l.id, l.from_entity_id, l.to_entity_id, l.relation_type,
		       l.valid_from, l.valid_to, COALESCE(l.note,''), l.created_at, l.created_by,
		       ef.kind, ef.name, et.kind, et.name,
		       vf.plate, vf.brand, vf.model, vf.color, vf.category,
		       vt.plate, vt.brand, vt.model, vt.color, vt.category,
		       (CASE WHEN l.from_entity_id = $1 THEN 'out' ELSE 'in' END) AS direction
		  FROM app.entity_links l
		  JOIN app.entities ef ON ef.id = l.from_entity_id AND ef.deleted_at IS NULL
		  JOIN app.entities et ON et.id = l.to_entity_id   AND et.deleted_at IS NULL
		  LEFT JOIN app.entity_vehicles vf ON vf.entity_id = l.from_entity_id
		  LEFT JOIN app.entity_vehicles vt ON vt.entity_id = l.to_entity_id
		 WHERE l.deleted_at IS NULL
		   AND (l.from_entity_id = $1 OR l.to_entity_id = $1)
		 ORDER BY l.created_at DESC`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LinkWithDirection, 0)
	for rows.Next() {
		var lw LinkWithDirection
		var validFrom, validTo sql.NullTime
		var fromKind, toKind, direction string
		var fPlate, fBrand, fModel, fColor, fCategory sql.NullString
		var tPlate, tBrand, tModel, tColor, tCategory sql.NullString
		if err := rows.Scan(
			&lw.ID, &lw.FromEntityID, &lw.ToEntityID, (*string)(&lw.RelationType),
			&validFrom, &validTo, &lw.Note, &lw.CreatedAt, &lw.CreatedBy,
			&fromKind, &lw.FromName, &toKind, &lw.ToName,
			&fPlate, &fBrand, &fModel, &fColor, &fCategory,
			&tPlate, &tBrand, &tModel, &tColor, &tCategory,
			&direction,
		); err != nil {
			return nil, err
		}
		if validFrom.Valid {
			t := validFrom.Time
			lw.ValidFrom = &t
		}
		if validTo.Valid {
			t := validTo.Time
			lw.ValidTo = &t
		}
		lw.FromKind = Kind(fromKind)
		lw.ToKind = Kind(toKind)
		if lw.FromKind == KindVehicle {
			lw.FromVehicle = &VehicleSummary{
				Plate:    nullStr(fPlate),
				Brand:    nullStr(fBrand),
				Model:    nullStr(fModel),
				Color:    nullStr(fColor),
				Category: nullStr(fCategory),
			}
		}
		if lw.ToKind == KindVehicle {
			lw.ToVehicle = &VehicleSummary{
				Plate:    nullStr(tPlate),
				Brand:    nullStr(tBrand),
				Model:    nullStr(tModel),
				Color:    nullStr(tColor),
				Category: nullStr(tCategory),
			}
		}
		lw.Direction = Direction(direction)
		out = append(out, lw)
	}
	return out, rows.Err()
}

// MaxGraphNodes limita o tamanho do subgrafo retornado por BuildGraph. Quando
// a BFS ultrapassa esse teto, a próxima camada é abortada e Graph.Truncated
// passa a true. Existe pra evitar payload abusivo em redes muito densas.
const MaxGraphNodes = 500

// GraphNode é a representação compacta de uma entidade no subgrafo retornado
// por BuildGraph. Inclui dados pra renderizar um cartão com foto + nome +
// vulgo + ORCRIM sem precisar de fetch extra por nó.
type GraphNode struct {
	ID             string
	Kind           Kind
	Name           string
	Classification int
	Version        int             // pra cache-bust da foto
	HasPhoto       bool            // pessoa/lugar têm foto primária
	Alias          *string         // person: primeiro vulgo; org: primeira sigla
	OrcrimAlias    *string         // person: alias da ORCRIM associada (se houver)
	Vehicle        *VehicleSummary // só para kind=vehicle
}

// GraphEdge é a aresta direcional no subgrafo. From/To são IDs das pontas.
type GraphEdge struct {
	ID           string
	From         string
	To           string
	RelationType RelationType
	Note         string
}

// Graph é o resultado de BuildGraph — subgrafo da rede de relações até depth
// saltos a partir de CenterID, já filtrado por clearance do chamador.
type Graph struct {
	CenterID  string
	Depth     int
	Nodes     []GraphNode
	Edges     []GraphEdge
	Truncated bool
}

// BuildGraph faz BFS por camadas a partir de centerID, parando em depth saltos.
// Cada camada faz uma única query que busca todos os links cujas pontas estão
// na fronteira corrente, hidratando ambos os endpoints. Nós com classification
// > clearance são podados, e as edges que os tinham como ponta caem junto.
//
// MaxGraphNodes funciona como circuit breaker: ao atingir o teto, a expansão
// para e Truncated vira true. O caller deve mostrar isso na UI.
func (r *Repo) BuildGraph(ctx context.Context, centerID string, depth, clearance int) (*Graph, error) {
	if depth < 1 {
		depth = 1
	}
	if depth > 3 {
		depth = 3
	}

	// Carrega o nó central explicitamente. Se não passar no clearance, devolve
	// not found pro caller esconder a existência.
	center, err := r.findGraphNode(ctx, centerID)
	if err != nil {
		return nil, err
	}
	if center.Classification > clearance {
		return nil, ErrNotFound
	}

	g := &Graph{CenterID: centerID, Depth: depth}
	seenNodes := map[string]bool{centerID: true}
	seenEdges := map[string]bool{}
	g.Nodes = append(g.Nodes, *center)

	frontier := []string{centerID}
	for hop := 0; hop < depth; hop++ {
		if len(frontier) == 0 {
			break
		}
		links, err := r.linksTouching(ctx, frontier)
		if err != nil {
			return nil, err
		}

		next := make([]string, 0)
		for _, e := range links {
			if seenEdges[e.edge.ID] {
				continue
			}
			// Aplica clearance em ambas as pontas. Se qualquer uma falhar, a
			// edge é descartada — não revelamos sequer a existência via ID.
			if e.fromNode.Classification > clearance || e.toNode.Classification > clearance {
				continue
			}
			seenEdges[e.edge.ID] = true

			for _, n := range []GraphNode{e.fromNode, e.toNode} {
				if seenNodes[n.ID] {
					continue
				}
				if len(g.Nodes) >= MaxGraphNodes {
					g.Truncated = true
					continue
				}
				seenNodes[n.ID] = true
				g.Nodes = append(g.Nodes, n)
				next = append(next, n.ID)
			}
			if !g.Truncated {
				g.Edges = append(g.Edges, e.edge)
			}
		}

		if g.Truncated {
			break
		}
		frontier = next
	}

	// Enriquece os nós com dados visuais (foto, vulgo, orcrim, version). Feito
	// em batch único ao final pra evitar N+1 durante a BFS.
	if err := r.enrichGraphNodes(ctx, g.Nodes); err != nil {
		return nil, err
	}

	return g, nil
}

// enrichGraphNodes faz uma única query que LEFT JOINta entidades, pessoas,
// organizações, lugares e a ORCRIM associada (quando existir), e popula
// version, has_photo, alias (vulgo/sigla) e orcrim_alias em cada node.
// Tudo opcional; nós sem child correspondente ficam com defaults.
func (r *Repo) enrichGraphNodes(ctx context.Context, nodes []GraphNode) error {
	if len(nodes) == 0 {
		return nil
	}
	placeholders := make([]string, len(nodes))
	args := make([]any, len(nodes))
	for i, n := range nodes {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = n.ID
	}
	q := `
		SELECT e.id, e.version,
		       p.aliases[1] AS person_alias,
		       o.aliases[1] AS org_alias,
		       oco.aliases[1] AS orcrim_alias,
		       (p.photo_path IS NOT NULL OR pl.photo_path IS NOT NULL OR v.photo_path IS NOT NULL) AS has_photo
		  FROM app.entities e
		  LEFT JOIN app.entity_persons p        ON p.entity_id = e.id
		  LEFT JOIN app.entity_organizations o  ON o.entity_id = e.id
		  LEFT JOIN app.entity_places pl        ON pl.entity_id = e.id
		  LEFT JOIN app.entity_vehicles v       ON v.entity_id = e.id
		  LEFT JOIN app.entity_organizations oco ON oco.entity_id = p.orcrim_id
		 WHERE e.id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	type enrich struct {
		version     int
		personAlias sql.NullString
		orgAlias    sql.NullString
		orcrimAlias sql.NullString
		hasPhoto    bool
	}
	byID := make(map[string]enrich, len(nodes))
	for rows.Next() {
		var id string
		var en enrich
		if err := rows.Scan(&id, &en.version, &en.personAlias, &en.orgAlias, &en.orcrimAlias, &en.hasPhoto); err != nil {
			return err
		}
		byID[id] = en
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range nodes {
		en, ok := byID[nodes[i].ID]
		if !ok {
			continue
		}
		nodes[i].Version = en.version
		nodes[i].HasPhoto = en.hasPhoto
		switch nodes[i].Kind {
		case KindPerson:
			if en.personAlias.Valid && en.personAlias.String != "" {
				s := en.personAlias.String
				nodes[i].Alias = &s
			}
			if en.orcrimAlias.Valid && en.orcrimAlias.String != "" {
				s := en.orcrimAlias.String
				nodes[i].OrcrimAlias = &s
			}
		case KindOrganization:
			if en.orgAlias.Valid && en.orgAlias.String != "" {
				s := en.orgAlias.String
				nodes[i].Alias = &s
			}
		}
	}
	return nil
}

// linksTouchingResult agrupa uma edge com os dois nós hidratados, evitando
// segundas queries pra recuperar kind/name/classification das pontas.
type linksTouchingResult struct {
	edge     GraphEdge
	fromNode GraphNode
	toNode   GraphNode
}

// linksTouching busca todos os vínculos vivos cuja origem OU destino está em
// ids, hidratando ambas as pontas (kind, name, classification + vehicle summary
// quando aplicável). Usa placeholders dinâmicos pra IN-list.
func (r *Repo) linksTouching(ctx context.Context, ids []string) ([]linksTouchingResult, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	inList := strings.Join(placeholders, ",")

	q := `
		SELECT l.id, l.from_entity_id, l.to_entity_id, l.relation_type, COALESCE(l.note,''),
		       ef.kind, ef.name, ef.classification,
		       et.kind, et.name, et.classification,
		       vf.plate, vf.brand, vf.model, vf.color, vf.category,
		       vt.plate, vt.brand, vt.model, vt.color, vt.category
		  FROM app.entity_links l
		  JOIN app.entities ef ON ef.id = l.from_entity_id AND ef.deleted_at IS NULL
		  JOIN app.entities et ON et.id = l.to_entity_id   AND et.deleted_at IS NULL
		  LEFT JOIN app.entity_vehicles vf ON vf.entity_id = l.from_entity_id
		  LEFT JOIN app.entity_vehicles vt ON vt.entity_id = l.to_entity_id
		 WHERE l.deleted_at IS NULL
		   AND (l.from_entity_id IN (` + inList + `) OR l.to_entity_id IN (` + inList + `))`

	// Placeholders $1..$N são posicionais — a query referencia o mesmo conjunto
	// de args nas duas cláusulas IN, então NÃO duplicamos os bindings.
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]linksTouchingResult, 0)
	for rows.Next() {
		var e GraphEdge
		var fromKind, toKind string
		var f, t GraphNode
		var fPlate, fBrand, fModel, fColor, fCategory sql.NullString
		var tPlate, tBrand, tModel, tColor, tCategory sql.NullString
		if err := rows.Scan(
			&e.ID, &e.From, &e.To, (*string)(&e.RelationType), &e.Note,
			&fromKind, &f.Name, &f.Classification,
			&toKind, &t.Name, &t.Classification,
			&fPlate, &fBrand, &fModel, &fColor, &fCategory,
			&tPlate, &tBrand, &tModel, &tColor, &tCategory,
		); err != nil {
			return nil, err
		}
		f.ID = e.From
		t.ID = e.To
		f.Kind = Kind(fromKind)
		t.Kind = Kind(toKind)
		if f.Kind == KindVehicle {
			f.Vehicle = &VehicleSummary{Plate: nullStr(fPlate), Brand: nullStr(fBrand), Model: nullStr(fModel), Color: nullStr(fColor), Category: nullStr(fCategory)}
		}
		if t.Kind == KindVehicle {
			t.Vehicle = &VehicleSummary{Plate: nullStr(tPlate), Brand: nullStr(tBrand), Model: nullStr(tModel), Color: nullStr(tColor), Category: nullStr(tCategory)}
		}
		out = append(out, linksTouchingResult{edge: e, fromNode: f, toNode: t})
	}
	return out, rows.Err()
}

// findGraphNode hidrata um único nó (sem percorrer links). Usado pra inicializar
// o centro do grafo e checar clearance antes da expansão.
func (r *Repo) findGraphNode(ctx context.Context, id string) (*GraphNode, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT e.id, e.kind, e.name, e.classification,
		       v.plate, v.brand, v.model, v.color, v.category
		  FROM app.entities e
		  LEFT JOIN app.entity_vehicles v ON v.entity_id = e.id
		 WHERE e.id = $1 AND e.deleted_at IS NULL`, id)
	var n GraphNode
	var kind string
	var plate, brand, model, color, category sql.NullString
	if err := row.Scan(&n.ID, &kind, &n.Name, &n.Classification, &plate, &brand, &model, &color, &category); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	n.Kind = Kind(kind)
	if n.Kind == KindVehicle {
		n.Vehicle = &VehicleSummary{Plate: nullStr(plate), Brand: nullStr(brand), Model: nullStr(model), Color: nullStr(color), Category: nullStr(category)}
	}
	return &n, nil
}

// ResyncSiblings recalcula os vínculos de irmandade do filho childID. Para
// cada candidato que compartilha ao menos um pai/mãe com childID, insere
// `sibling` (dois pais em comum) ou `half_sibling` (apenas um). Faz upgrade
// de half→full automaticamente quando o segundo parente coincide. Idempotente:
// se já existe o link no nível correto, não faz nada. Devolve os links recém-
// criados pra o caller (handler HTTP) poder auditar cada um.
//
// Direção dos vínculos simétricos: normalizamos pra que `from < to` (UUID
// como string) — preserva a unique index sem depender de quem inseriu antes.
func (r *Repo) ResyncSiblings(ctx context.Context, childID, actor string) ([]*Link, error) {
	// 1) Pais conhecidos do filho. Pode ser 0, 1 ou 2.
	rows, err := r.db.QueryContext(ctx, `
		SELECT from_entity_id, relation_type
		  FROM app.entity_links
		 WHERE to_entity_id = $1
		   AND deleted_at IS NULL
		   AND relation_type IN ('father_of','mother_of')`, childID)
	if err != nil {
		return nil, err
	}
	parents := map[string]string{} // parentID → relation_type
	for rows.Next() {
		var pid, rt string
		if err := rows.Scan(&pid, &rt); err != nil {
			rows.Close()
			return nil, err
		}
		parents[pid] = rt
	}
	rows.Close()
	if len(parents) == 0 {
		return nil, nil
	}

	// 2) Candidatos: outros filhos desses pais (excluindo o próprio child).
	pids := make([]string, 0, len(parents))
	for k := range parents {
		pids = append(pids, k)
	}
	phs := make([]string, len(pids))
	args := []any{childID}
	for i, p := range pids {
		phs[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, p)
	}
	candRows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT to_entity_id
		  FROM app.entity_links
		 WHERE deleted_at IS NULL
		   AND relation_type IN ('father_of','mother_of')
		   AND to_entity_id <> $1
		   AND from_entity_id IN (`+strings.Join(phs, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	candidates := make([]string, 0)
	for candRows.Next() {
		var id string
		if err := candRows.Scan(&id); err != nil {
			candRows.Close()
			return nil, err
		}
		candidates = append(candidates, id)
	}
	candRows.Close()

	// 3) Para cada candidato, contar pais em comum + upsert.
	created := make([]*Link, 0)
	for _, cand := range candidates {
		shared, err := r.countSharedParents(ctx, childID, cand)
		if err != nil {
			return nil, err
		}
		if shared == 0 {
			continue
		}
		desired := RelationHalfSibling
		if shared >= 2 {
			desired = RelationSibling
		}
		link, err := r.upsertSiblingLink(ctx, childID, cand, desired, actor)
		if err != nil {
			return nil, err
		}
		if link != nil {
			created = append(created, link)
		}
	}
	return created, nil
}

// countSharedParents conta quantos pais (father_of/mother_of) ambos compartilham.
// Útil pra decidir entre sibling (2) e half_sibling (1).
func (r *Repo) countSharedParents(ctx context.Context, a, b string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT la.from_entity_id
			  FROM app.entity_links la
			  JOIN app.entity_links lb
			    ON lb.from_entity_id = la.from_entity_id
			   AND lb.relation_type  = la.relation_type
			   AND lb.deleted_at IS NULL
			 WHERE la.to_entity_id = $1
			   AND lb.to_entity_id = $2
			   AND la.deleted_at IS NULL
			   AND la.relation_type IN ('father_of','mother_of')
		) shared`, a, b).Scan(&n)
	return n, err
}

// upsertSiblingLink garante a existência de um vínculo sibling/half_sibling
// canônico entre a e b. Faz upgrade de half→full quando aplicável. Devolve
// o link criado (nil se já estava correto).
func (r *Repo) upsertSiblingLink(ctx context.Context, a, b string, desired RelationType, actor string) (*Link, error) {
	// Normaliza pra que from < to — torna idempotente independente da ordem
	// de chamada e respeita a unique index (from, to, relation_type).
	from, to := a, b
	if from > to {
		from, to = to, from
	}
	// Existe algum sibling/half_sibling vivo entre os dois (em qualquer
	// direção)? Verificamos os dois sentidos pra detectar inserts antigos
	// que não usaram a normalização.
	var existingID string
	var existingType string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, relation_type FROM app.entity_links
		 WHERE deleted_at IS NULL
		   AND relation_type IN ('sibling','half_sibling')
		   AND ((from_entity_id = $1 AND to_entity_id = $2)
		     OR (from_entity_id = $2 AND to_entity_id = $1))
		 LIMIT 1`, from, to).Scan(&existingID, &existingType)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		// Já existe vínculo entre os dois. Só upgrade half→full faz sentido.
		if existingType == string(RelationHalfSibling) && desired == RelationSibling {
			if _, err := r.db.ExecContext(ctx, `
				UPDATE app.entity_links
				   SET deleted_at = now(), deleted_by = $1
				 WHERE id = $2 AND deleted_at IS NULL`, actor, existingID); err != nil {
				return nil, err
			}
			return r.insertSiblingLinkRaw(ctx, from, to, desired, actor)
		}
		return nil, nil
	}
	// Não existe ainda — insere.
	return r.insertSiblingLinkRaw(ctx, from, to, desired, actor)
}

// insertSiblingLinkRaw faz o INSERT bruto e devolve o link hidratado.
// Não chama CreateLink porque queremos pular a validação de par (já garantida)
// e poder fazer o insert mesmo durante o handler da link parental.
func (r *Repo) insertSiblingLinkRaw(ctx context.Context, from, to string, rt RelationType, actor string) (*Link, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.entity_links
		  (from_entity_id, to_entity_id, relation_type, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, from, to, string(rt), actor).Scan(&id)
	if err != nil {
		// Conflito de unique pode acontecer se uma chamada concorrente já
		// inseriu o mesmo vínculo — tratamos como sucesso silencioso.
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return nil, nil
		}
		return nil, fmt.Errorf("insert sibling: %w", err)
	}
	return r.FindLink(ctx, id)
}

// SoftDeleteLink marca o link como removido. Retorna o link como estava antes
// (pra audit). ErrLinkNotFound se já estava deletado ou nunca existiu.
func (r *Repo) SoftDeleteLink(ctx context.Context, id, deletedBy string) (*Link, error) {
	before, err := r.FindLink(ctx, id)
	if err != nil {
		return nil, err
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.entity_links
		   SET deleted_at = now(), deleted_by = $1
		 WHERE id = $2 AND deleted_at IS NULL`,
		deletedBy, id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrLinkNotFound
	}
	return before, nil
}
