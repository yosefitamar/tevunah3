package entities

import (
	"context"
	"database/sql"
	"errors"
)

// CPFLookup é o resultado de procurar uma pessoa pelo CPF.
//
//   - Entity preenchida: a pessoa existe, está ativa e o usuário pode vê-la.
//   - Blocked: o CPF já está gravado numa pessoa que o usuário NÃO pode usar
//     (excluída ou acima do clearance). O índice único de CPF cobre também os
//     registros excluídos, então criar outra pessoa com ele vai falhar.
//   - Tudo vazio: CPF livre.
type CPFLookup struct {
	Entity  *Entity
	Blocked bool
}

// FindPersonByCPF procura a pessoa dona do CPF (11 dígitos, sem máscara).
func (r *Repo) FindPersonByCPF(ctx context.Context, cpf string, maxClearance int) (*CPFLookup, error) {
	if cpf == "" {
		return &CPFLookup{}, nil
	}
	var id string
	var classification int
	var deleted bool
	err := r.db.QueryRowContext(ctx, `
		SELECT e.id, e.classification, e.deleted_at IS NOT NULL
		  FROM app.entity_persons p
		  JOIN app.entities e ON e.id = p.entity_id
		 WHERE p.cpf = $1
		 LIMIT 1`, cpf,
	).Scan(&id, &classification, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return &CPFLookup{}, nil
	}
	if err != nil {
		return nil, err
	}
	if deleted || classification > maxClearance {
		return &CPFLookup{Blocked: true}, nil
	}
	e, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &CPFLookup{Entity: e}, nil
}
