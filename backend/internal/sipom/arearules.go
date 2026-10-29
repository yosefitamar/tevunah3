package sipom

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
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
	// byCity: área de cada cidade em que todas as referências concordam
	// (as de bairro e a da cidade inteira) — vale onde o catálogo do SIPOM
	// põe mais de uma companhia na cidade toda, sem distinguir bairro.
	byCity map[string]int
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

// Find é o Lookup que também usa a cidade inteira quando cityWide: o
// catálogo atribui as áreas da cidade sem distinguir bairro (ex.: a
// companhia do BPM e a do BPRAIO em toda a cidade), então a escolha feita
// para um bairro vale para os outros — desde que as referências da cidade
// não se contradigam.
func (m *AreaRules) Find(city, neighborhood string, cityWide bool) (int, bool) {
	if id, ok := m.Lookup(city, neighborhood); ok {
		return id, true
	}
	if m == nil || !cityWide || key(city) == "" {
		return 0, false
	}
	id, ok := m.byCity[key(city)]
	return id, ok
}

// AreaRule é uma referência gravada (cidade e bairro já normalizados).
type AreaRule struct {
	City, Neighborhood string
	AreaID             int
}

// NewAreaRules monta as referências em memória. A cidade só ganha área
// própria (byCity) se todas as referências dela apontam para a mesma.
func NewAreaRules(list []AreaRule) *AreaRules {
	m := &AreaRules{byPlace: map[string]int{}, byCity: map[string]int{}}
	conflict := map[string]bool{}
	for _, x := range list {
		c, n := key(x.City), key(x.Neighborhood)
		m.byPlace[c+"\x00"+n] = x.AreaID
		if prev, ok := m.byCity[c]; ok && prev != x.AreaID {
			conflict[c] = true
		}
		m.byCity[c] = x.AreaID
	}
	for c := range conflict {
		delete(m.byCity, c)
	}
	return m
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
	var list []AreaRule
	for rows.Next() {
		var x AreaRule
		if err := rows.Scan(&x.City, &x.Neighborhood, &x.AreaID); err != nil {
			return nil, err
		}
		list = append(list, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return NewAreaRules(list), nil
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

// StoredAreaRule é a referência gravada, como a tela de referências mostra.
type StoredAreaRule struct {
	City, Neighborhood string // grafia de quem gravou
	AreaID             int
	UpdatedAt          time.Time
	UpdatedByName      string
}

// List devolve as referências, por cidade e bairro.
func (r *AreaRuleRepo) List(ctx context.Context) ([]StoredAreaRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.city, r.neighborhood, r.area_id, r.updated_at, COALESCE(u.display_name, '')
		  FROM app.sipom_area_rules r
		  LEFT JOIN app.users u ON u.id = r.updated_by
		 ORDER BY r.city_key, r.neighborhood_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StoredAreaRule{}
	for rows.Next() {
		var x StoredAreaRule
		if err := rows.Scan(&x.City, &x.Neighborhood, &x.AreaID, &x.UpdatedAt, &x.UpdatedByName); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// Delete apaga a referência do lugar. Devolve false se não havia.
func (r *AreaRuleRepo) Delete(ctx context.Context, city, neighborhood string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM app.sipom_area_rules WHERE city_key = $1 AND neighborhood_key = $2`,
		key(city), key(neighborhood))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
