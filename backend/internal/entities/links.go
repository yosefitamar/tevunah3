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
type RelationType string

const (
	// RelationOwns: pessoa/organização é a proprietária registrada do veículo.
	RelationOwns RelationType = "owns"
	// RelationAssociatedWith: vínculo vago — entidade A é associada à B sem
	// qualificação formal (observação de campo, hipótese investigativa).
	RelationAssociatedWith RelationType = "associated_with"
)

// IsValid devolve true para um RelationType suportado.
func (rt RelationType) IsValid() bool {
	switch rt {
	case RelationOwns, RelationAssociatedWith:
		return true
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
	// Confirma que ambas entidades existem (não-deletadas). Não fazemos check
	// de clearance aqui — fica a cargo do handler HTTP, que conhece o usuário.
	for _, id := range []string{in.FromEntityID, in.ToEntityID} {
		var ok bool
		err := r.db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM app.entities
			                 WHERE id = $1 AND deleted_at IS NULL)`, id,
		).Scan(&ok)
		if err != nil {
			return nil, fmt.Errorf("check entity %s: %w", id, err)
		}
		if !ok {
			return nil, ErrNotFound
		}
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
	rows, err := r.db.QueryContext(ctx, `
		SELECT l.id, l.from_entity_id, l.to_entity_id, l.relation_type,
		       l.valid_from, l.valid_to, COALESCE(l.note,''), l.created_at, l.created_by,
		       ef.kind, ef.name, et.kind, et.name,
		       (CASE WHEN l.from_entity_id = $1 THEN 'out' ELSE 'in' END) AS direction
		  FROM app.entity_links l
		  JOIN app.entities ef ON ef.id = l.from_entity_id AND ef.deleted_at IS NULL
		  JOIN app.entities et ON et.id = l.to_entity_id   AND et.deleted_at IS NULL
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
		if err := rows.Scan(
			&lw.ID, &lw.FromEntityID, &lw.ToEntityID, (*string)(&lw.RelationType),
			&validFrom, &validTo, &lw.Note, &lw.CreatedAt, &lw.CreatedBy,
			&fromKind, &lw.FromName, &toKind, &lw.ToName, &direction,
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
		lw.Direction = Direction(direction)
		out = append(out, lw)
	}
	return out, rows.Err()
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
