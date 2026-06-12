package entities

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// PersonAddress representa um endereço associado a uma pessoa (1-N).
// Modelagem inline: não vira Place no grafo (decisão de Phase 1). Cada
// endereço carrega um label livre tipo "casa", "casa da mãe", "trabalho".
type PersonAddress struct {
	ID           string
	PersonID     string
	Label        *string
	CEP          *string
	Street       *string
	Number       *string
	Complement   *string
	Neighborhood *string
	City         *string
	State        *string
	CreatedAt    time.Time
	CreatedBy    string
	UpdatedAt    time.Time
	UpdatedBy    string
}

// NewPersonAddress é o input para CreateAddress/UpdateAddress. Campos vazios
// vão como NULL no banco.
type NewPersonAddress struct {
	Label        string
	CEP          string
	Street       string
	Number       string
	Complement   string
	Neighborhood string
	City         string
	State        string
}

// ErrAddressNotFound retornado quando o address_id não existe (ou já foi
// soft-deletado) ou não pertence à pessoa anunciada na URL.
var ErrAddressNotFound = errors.New("endereço não encontrado")

// ListAddresses devolve os endereços vivos da pessoa, ordenados pelo created_at.
func (r *Repo) ListAddresses(ctx context.Context, personID string) ([]PersonAddress, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, person_id, label, cep, street, number, complement,
		       neighborhood, city, state,
		       created_at, created_by, updated_at, updated_by
		  FROM app.person_addresses
		 WHERE person_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at ASC`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PersonAddress, 0)
	for rows.Next() {
		var a PersonAddress
		var label, cep, street, number, complement, neighborhood, city, state sql.NullString
		if err := rows.Scan(
			&a.ID, &a.PersonID, &label, &cep, &street, &number, &complement,
			&neighborhood, &city, &state,
			&a.CreatedAt, &a.CreatedBy, &a.UpdatedAt, &a.UpdatedBy,
		); err != nil {
			return nil, err
		}
		a.Label = nullStr(label)
		a.CEP = nullStr(cep)
		a.Street = nullStr(street)
		a.Number = nullStr(number)
		a.Complement = nullStr(complement)
		a.Neighborhood = nullStr(neighborhood)
		a.City = nullStr(city)
		a.State = nullStr(state)
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateAddress insere um endereço. Verifica que a pessoa existe e é do kind
// correto (entity_persons garante isso via FK + 1:1).
func (r *Repo) CreateAddress(ctx context.Context, personID string, in NewPersonAddress, createdBy string) (*PersonAddress, error) {
	// Sanity check: pessoa existe (e ainda não foi deletada).
	var ok bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM app.entities e
		   JOIN app.entity_persons p ON p.entity_id = e.id
		   WHERE e.id = $1 AND e.kind = 'person' AND e.deleted_at IS NULL
		)`, personID,
	).Scan(&ok); err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}

	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.person_addresses
		  (person_id, label, cep, street, number, complement,
		   neighborhood, city, state, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		RETURNING id`,
		personID,
		nullableString(in.Label),
		nullableString(normalizeCEP(in.CEP)),
		nullableString(upperTrim(in.Street)),
		nullableString(upperTrim(in.Number)),
		nullableString(upperTrim(in.Complement)),
		nullableString(upperTrim(in.Neighborhood)),
		nullableString(upperTrim(in.City)),
		nullableString(upperTrim(in.State)),
		createdBy,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.findAddress(ctx, id)
}

// UpdateAddress substitui o conteúdo. Patch total — campo vazio em NewPersonAddress
// vira NULL. Mantém simples; o front envia tudo que conhece.
func (r *Repo) UpdateAddress(ctx context.Context, personID, addressID string, in NewPersonAddress, updatedBy string) (*PersonAddress, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.person_addresses SET
		  label        = $1,
		  cep          = $2,
		  street       = $3,
		  number       = $4,
		  complement   = $5,
		  neighborhood = $6,
		  city         = $7,
		  state        = $8,
		  updated_at   = now(),
		  updated_by   = $9
		WHERE id = $10 AND person_id = $11 AND deleted_at IS NULL`,
		nullableString(in.Label),
		nullableString(normalizeCEP(in.CEP)),
		nullableString(upperTrim(in.Street)),
		nullableString(upperTrim(in.Number)),
		nullableString(upperTrim(in.Complement)),
		nullableString(upperTrim(in.Neighborhood)),
		nullableString(upperTrim(in.City)),
		nullableString(upperTrim(in.State)),
		updatedBy, addressID, personID,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrAddressNotFound
	}
	return r.findAddress(ctx, addressID)
}

// DeleteAddress faz soft-delete. ErrAddressNotFound se não existir ativo na
// pessoa indicada.
func (r *Repo) DeleteAddress(ctx context.Context, personID, addressID, deletedBy string) (*PersonAddress, error) {
	before, err := r.findAddress(ctx, addressID)
	if err != nil {
		return nil, err
	}
	if before.PersonID != personID {
		return nil, ErrAddressNotFound
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.person_addresses
		   SET deleted_at = now(), deleted_by = $1
		 WHERE id = $2 AND person_id = $3 AND deleted_at IS NULL`,
		deletedBy, addressID, personID,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrAddressNotFound
	}
	return before, nil
}

// findAddress busca um endereço ativo pelo ID (sem filtro de pessoa).
func (r *Repo) findAddress(ctx context.Context, id string) (*PersonAddress, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, person_id, label, cep, street, number, complement,
		       neighborhood, city, state,
		       created_at, created_by, updated_at, updated_by
		  FROM app.person_addresses
		 WHERE id = $1 AND deleted_at IS NULL`, id)
	var a PersonAddress
	var label, cep, street, number, complement, neighborhood, city, state sql.NullString
	if err := row.Scan(
		&a.ID, &a.PersonID, &label, &cep, &street, &number, &complement,
		&neighborhood, &city, &state,
		&a.CreatedAt, &a.CreatedBy, &a.UpdatedAt, &a.UpdatedBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAddressNotFound
		}
		return nil, err
	}
	a.Label = nullStr(label)
	a.CEP = nullStr(cep)
	a.Street = nullStr(street)
	a.Number = nullStr(number)
	a.Complement = nullStr(complement)
	a.Neighborhood = nullStr(neighborhood)
	a.City = nullStr(city)
	a.State = nullStr(state)
	return &a, nil
}

// upperTrim normaliza campos textuais de endereço: trim + maiúsculas
// (Unicode-aware, preserva acentos).
func upperTrim(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// upperTrimPtr versão pointer-aware de upperTrim: nil continua nil (campo
// ausente no patch), preservando a semântica de update parcial.
func upperTrimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := upperTrim(*p)
	return &v
}

// normalizeCEP remove caracteres não-numéricos. CEP brasileiro tem 8 dígitos;
// se vier nada, devolve vazio.
func normalizeCEP(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			out = append(out, c)
		}
	}
	return string(out)
}
