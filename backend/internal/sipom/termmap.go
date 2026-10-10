package sipom

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Termos aprendidos: o relatório escreve em texto livre ("DMC", "REVOLVER",
// "PRETO") o que o SIPOM quer em lista. Cada escolha do analista vira
// referência (campo + termo → id) em app.sipom_term_map e vale para as
// próximas ocorrências — a mesma ideia da referência de área.

// Campos do mapa de termos.
const (
	TermDelegacia         = "delegacia"            // texto "Delegacia:" do relatório → delegacias.id
	TermDelegado          = "delegado"             // texto "Delegado(a):" → delegados.id
	TermArmaTipo          = "arma_tipo"            // tipo da arma → arma_tipos.id
	TermArmaMarca         = "arma_marca"           // marca → arma_marcas.id
	TermArmaCalibre       = "arma_calibre"         // calibre → arma_calibres.id
	TermDroga             = "droga"                // descrição da droga → drogas.id
	TermVeiculoTipo       = "veiculo_tipo"         // tipo do veículo → código DENATRAN
	TermVeiculoCor        = "veiculo_cor"          // cor → código DENATRAN
	TermVeiculoMarcaModel = "veiculo_marca_modelo" // "marca/modelo" → código DENATRAN
)

var ErrEmptyTerm = errors.New("termo vazio: não há o que aprender")

// TermMap são os termos aprendidos em memória.
type TermMap struct {
	byField map[string]map[string]int
}

// Lookup devolve o id aprendido para o termo do campo.
func (m *TermMap) Lookup(field, term string) (int, bool) {
	if m == nil {
		return 0, false
	}
	k := key(term)
	if k == "" {
		return 0, false
	}
	id, ok := m.byField[field][k]
	return id, ok
}

// TermRepo encapsula app.sipom_term_map.
type TermRepo struct {
	db *sql.DB
}

func NewTermRepo(db *sql.DB) *TermRepo { return &TermRepo{db: db} }

// Load lê todos os termos.
func (r *TermRepo) Load(ctx context.Context) (*TermMap, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT field, term_key, target_id FROM app.sipom_term_map`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := &TermMap{byField: map[string]map[string]int{}}
	for rows.Next() {
		var f, k string
		var id int
		if err := rows.Scan(&f, &k, &id); err != nil {
			return nil, err
		}
		if m.byField[f] == nil {
			m.byField[f] = map[string]int{}
		}
		m.byField[f][k] = id
	}
	return m, rows.Err()
}

// Learn grava (ou substitui) a referência do termo. Devolve true quando a
// referência mudou — é nova ou apontava para outro id.
func (r *TermRepo) Learn(ctx context.Context, field, term string, targetID int, actor string) (bool, error) {
	k := key(term)
	if k == "" {
		return false, ErrEmptyTerm
	}
	var prev sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT target_id FROM app.sipom_term_map WHERE field = $1 AND term_key = $2`, field, k).Scan(&prev)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if prev.Valid && int(prev.Int64) == targetID {
		return false, nil
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO app.sipom_term_map (field, term, term_key, target_id, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (field, term_key)
		DO UPDATE SET target_id = EXCLUDED.target_id, updated_at = now(), updated_by = EXCLUDED.updated_by`,
		field, strings.ToUpper(strings.Join(strings.Fields(term), " ")), k, targetID, actor)
	return err == nil, err
}
