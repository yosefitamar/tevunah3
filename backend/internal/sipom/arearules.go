package sipom

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Referência de área da unidade militar: cidade + bairro → companhia
// territorial, aprendida das escolhas do analista (app.sipom_area_rules).
// Vale sobre o catálogo do SIPOM, que em muitos bairros não tem regra ou tem
// mais de uma.

// ErrAreaRuleNoCity: sem cidade não há lugar a que associar a área.
var ErrAreaRuleNoCity = errors.New("ocorrência sem cidade: a área não vira referência")

// AreaRules são as referências em memória, por lugar.
type AreaRules struct {
	byPlace map[string]int
}

func placeKey(city, neighborhood string) string {
	return key(city) + "\x00" + key(neighborhood)
}

// Lookup devolve a área aprendida para o lugar. Só casa o par exato: a
// referência de um bairro não vale para a cidade inteira, nem o contrário.
func (m *AreaRules) Lookup(city, neighborhood string) (int, bool) {
	if m == nil || key(city) == "" {
		return 0, false
	}
	id, ok := m.byPlace[placeKey(city, neighborhood)]
	return id, ok
}

// AreaRuleRepo encapsula app.sipom_area_rules.
type AreaRuleRepo struct {
	db *sql.DB
}

func NewAreaRuleRepo(db *sql.DB) *AreaRuleRepo { return &AreaRuleRepo{db: db} }

// Load lê todas as referências.
func (r *AreaRuleRepo) Load(ctx context.Context) (*AreaRules, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT city_key, neighborhood_key, area_id FROM app.sipom_area_rules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := &AreaRules{byPlace: map[string]int{}}
	for rows.Next() {
		var c, n string
		var id int
		if err := rows.Scan(&c, &n, &id); err != nil {
			return nil, err
		}
		m.byPlace[c+"\x00"+n] = id
	}
	return m, rows.Err()
}

// Learn grava (ou substitui) a referência do lugar. Devolve true quando a
// referência mudou — é nova ou apontava para outra área.
func (r *AreaRuleRepo) Learn(ctx context.Context, city, neighborhood string, areaID int, actor string) (bool, error) {
	ck, nk := key(city), key(neighborhood)
	if ck == "" {
		return false, ErrAreaRuleNoCity
	}
	var prev sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT area_id FROM app.sipom_area_rules
		 WHERE city_key = $1 AND neighborhood_key = $2`, ck, nk).Scan(&prev)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if prev.Valid && int(prev.Int64) == areaID {
		return false, nil
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO app.sipom_area_rules
		  (city, neighborhood, city_key, neighborhood_key, area_id, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (city_key, neighborhood_key)
		DO UPDATE SET area_id = EXCLUDED.area_id, updated_at = now(), updated_by = EXCLUDED.updated_by`,
		strings.ToUpper(strings.TrimSpace(city)), strings.ToUpper(strings.TrimSpace(neighborhood)),
		ck, nk, areaID, actor)
	return err == nil, err
}
