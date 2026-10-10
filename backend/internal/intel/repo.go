package intel

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound  = errors.New("termo não encontrado")
	ErrEmpty     = errors.New("informe um termo com letras ou números")
	ErrDuplicate = errors.New("termo já cadastrado")
)

// Keyword é um termo que indica participação da inteligência.
type Keyword struct {
	ID            string
	Term          string
	Active        bool
	CreatedAt     time.Time
	CreatedByName string
}

// Repo encapsula app.intel_keywords.
type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// List devolve todos os termos, ativos primeiro, em ordem alfabética.
func (r *Repo) List(ctx context.Context) ([]Keyword, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT k.id, k.term, k.active, k.created_at, COALESCE(u.display_name, '')
		  FROM app.intel_keywords k
		  LEFT JOIN app.users u ON u.id = k.created_by
		 ORDER BY k.active DESC, app.norm_txt(k.term)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Keyword{}
	for rows.Next() {
		var k Keyword
		if err := rows.Scan(&k.ID, &k.Term, &k.Active, &k.CreatedAt, &k.CreatedByName); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// Matcher monta o comparador com os termos ativos.
func (r *Repo) Matcher(ctx context.Context) (*Matcher, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT term FROM app.intel_keywords WHERE active ORDER BY created_at, term`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var terms []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		terms = append(terms, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return NewMatcher(terms), nil
}

// Create cadastra um termo. Dois termos com a mesma forma normalizada
// ("S.A.I." e "S A I") seriam redundantes e são recusados.
func (r *Repo) Create(ctx context.Context, term, actor string) (*Keyword, error) {
	term = strings.Join(strings.Fields(term), " ")
	norm := Normalize(term)
	if norm == "" {
		return nil, ErrEmpty
	}
	existing, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, k := range existing {
		if Normalize(k.Term) == norm {
			return nil, ErrDuplicate
		}
	}
	var id string
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO app.intel_keywords (term, created_by) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
		RETURNING id`, term, actor).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDuplicate
	}
	if err != nil {
		return nil, err
	}
	return r.find(ctx, id)
}

// SetActive liga ou desliga um termo sem perder o histórico.
func (r *Repo) SetActive(ctx context.Context, id string, active bool) (*Keyword, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE app.intel_keywords SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return r.find(ctx, id)
}

// Delete remove o termo. As ocorrências já marcadas por ele continuam com o
// nome do termo em intel_matched até a próxima reaplicação.
func (r *Repo) Delete(ctx context.Context, id string) (*Keyword, error) {
	k, err := r.find(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM app.intel_keywords WHERE id = $1`, id); err != nil {
		return nil, err
	}
	return k, nil
}

func (r *Repo) find(ctx context.Context, id string) (*Keyword, error) {
	var k Keyword
	err := r.db.QueryRowContext(ctx, `
		SELECT k.id, k.term, k.active, k.created_at, COALESCE(u.display_name, '')
		  FROM app.intel_keywords k
		  LEFT JOIN app.users u ON u.id = k.created_by
		 WHERE k.id = $1`, id,
	).Scan(&k.ID, &k.Term, &k.Active, &k.CreatedAt, &k.CreatedByName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &k, err
}
