package dashboard

import (
	"context"
)

// ─── Relatório operacional ─────────────────────────────────────────────
//
// O relatório diário do CPRAIO é a fonte da produção operacional (armas,
// drogas, veículos, conduzidos). O painel lê quantidades, não ocorrências:
// uma abordagem com três armas conta três.
//
// Uma mesma ocorrência pode existir nos dois lados — importada do relatório e
// cadastrada à mão como CVLI. Onde o painel conta OCORRÊNCIAS (série e
// território), a ficha CIOPS decide: se o cadastro manual já tem a ficha, a
// cópia do relatório não conta de novo.

// OperationalStats é o bloco do relatório operacional.
type OperationalStats struct {
	Current  OperationalTotals
	Previous OperationalTotals
	// Composição do período corrente, para as linhas de apoio dos KPIs.
	WeaponKinds  []Facet
	DrugKinds    []DrugFacet
	VehicleKinds []Facet
	Series       []OperationalMonth
}

// OperationalTotals são as quantidades de um recorte.
type OperationalTotals struct {
	Occurrences int
	Weapons     int
	DrugsGrams  float64
	Accused     int
	// Adolescents são os acusados com idade informada abaixo de 18 anos.
	Adolescents int
	Vehicles    int
}

// DrugFacet é uma droga com o peso somado no período.
type DrugFacet struct {
	Name  string
	Grams float64
}

// OperationalMonth é um mês da série de ocorrências do relatório.
type OperationalMonth struct {
	Month string // YYYY-MM
	Count int
}

// dedupeClause exclui a ocorrência do relatório cuja ficha CIOPS já está no
// cadastro manual. Só entra quando quem pediu também enxerga o cadastro
// manual — senão a ocorrência sumiria do painel dele sem aparecer em outro.
const dedupeClause = `
	AND NOT EXISTS (
	  SELECT 1 FROM app.incidents i
	   WHERE i.deleted_at IS NULL AND i.ciops_record <> ''
	     AND upper(replace(i.ciops_record, ' ', '')) = o.ciops_record)`

func opsDedupe(withIncidents bool) string {
	if withIncidents {
		return dedupeClause
	}
	return ""
}

// Operational devolve o bloco do relatório operacional. withIncidents indica
// se o solicitante também lê o cadastro manual (liga a deduplicação na série).
func (r *Repo) Operational(ctx context.Context, w Window, withIncidents bool) (*OperationalStats, error) {
	st := &OperationalStats{}
	var err error
	if st.Current, err = r.opsTotals(ctx, w.Current); err != nil {
		return nil, err
	}
	if w.Previous.From != "" || w.Previous.To != "" {
		if st.Previous, err = r.opsTotals(ctx, w.Previous); err != nil {
			return nil, err
		}
	}
	if st.WeaponKinds, err = r.facets(ctx, `
		SELECT COALESCE(NULLIF(x.kind, ''), 'NÃO INFORMADO'), '', COUNT(*)
		  FROM app.ops_occurrence_weapons x
		  JOIN app.ops_occurrences o ON o.id = x.occurrence_id
		 WHERE o.deleted_at IS NULL
		   AND ($1::date IS NULL OR o.occurred_on >= $1::date)
		   AND ($2::date IS NULL OR o.occurred_on <= $2::date)
		 GROUP BY 1 ORDER BY 3 DESC, 1 LIMIT 3`,
		nilDate(w.Current.From), nilDate(w.Current.To)); err != nil {
		return nil, err
	}
	if st.VehicleKinds, err = r.facets(ctx, `
		SELECT COALESCE(NULLIF(x.kind, ''), 'NÃO INFORMADO'), '', COUNT(*)
		  FROM app.ops_occurrence_vehicles x
		  JOIN app.ops_occurrences o ON o.id = x.occurrence_id
		 WHERE o.deleted_at IS NULL
		   AND ($1::date IS NULL OR o.occurred_on >= $1::date)
		   AND ($2::date IS NULL OR o.occurred_on <= $2::date)
		 GROUP BY 1 ORDER BY 3 DESC, 1 LIMIT 3`,
		nilDate(w.Current.From), nilDate(w.Current.To)); err != nil {
		return nil, err
	}
	if st.DrugKinds, err = r.drugKinds(ctx, w.Current); err != nil {
		return nil, err
	}
	if st.Series, err = r.opsSeries(ctx, w.Series, withIncidents); err != nil {
		return nil, err
	}
	return st, nil
}

func (r *Repo) opsTotals(ctx context.Context, p Period) (OperationalTotals, error) {
	var t OperationalTotals
	err := r.db.QueryRowContext(ctx, `
		WITH occ AS (
		  SELECT id FROM app.ops_occurrences
		   WHERE deleted_at IS NULL
		     AND ($1::date IS NULL OR occurred_on >= $1::date)
		     AND ($2::date IS NULL OR occurred_on <= $2::date))
		SELECT
		  (SELECT COUNT(*) FROM occ),
		  (SELECT COUNT(*) FROM app.ops_occurrence_weapons  x WHERE x.occurrence_id IN (SELECT id FROM occ)),
		  (SELECT COALESCE(SUM(grams), 0)::float8 FROM app.ops_occurrence_drugs x WHERE x.occurrence_id IN (SELECT id FROM occ)),
		  (SELECT COUNT(*) FROM app.ops_occurrence_people   x WHERE x.occurrence_id IN (SELECT id FROM occ) AND x.role = 'ACUSADO'),
		  (SELECT COUNT(*) FROM app.ops_occurrence_people   x WHERE x.occurrence_id IN (SELECT id FROM occ) AND x.role = 'ACUSADO' AND x.age < 18),
		  (SELECT COUNT(*) FROM app.ops_occurrence_vehicles x WHERE x.occurrence_id IN (SELECT id FROM occ))`,
		nilDate(p.From), nilDate(p.To),
	).Scan(&t.Occurrences, &t.Weapons, &t.DrugsGrams, &t.Accused, &t.Adolescents, &t.Vehicles)
	return t, err
}

func (r *Repo) drugKinds(ctx context.Context, p Period) ([]DrugFacet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(x.description, ''), 'NÃO INFORMADA'), COALESCE(SUM(x.grams), 0)::float8
		  FROM app.ops_occurrence_drugs x
		  JOIN app.ops_occurrences o ON o.id = x.occurrence_id
		 WHERE o.deleted_at IS NULL
		   AND ($1::date IS NULL OR o.occurred_on >= $1::date)
		   AND ($2::date IS NULL OR o.occurred_on <= $2::date)
		 GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 3`, nilDate(p.From), nilDate(p.To))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DrugFacet{}
	for rows.Next() {
		var f DrugFacet
		if err := rows.Scan(&f.Name, &f.Grams); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *Repo) opsSeries(ctx context.Context, p Period, withIncidents bool) ([]OperationalMonth, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT to_char(date_trunc('month', o.occurred_on), 'YYYY-MM'), COUNT(*)
		  FROM app.ops_occurrences o
		 WHERE o.deleted_at IS NULL
		   AND o.occurred_on >= $1::date
		   AND o.occurred_on <= $2::date`+opsDedupe(withIncidents)+`
		 GROUP BY 1`, p.From, p.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byMonth := map[string]int{}
	for rows.Next() {
		var ym string
		var n int
		if err := rows.Scan(&ym, &n); err != nil {
			return nil, err
		}
		byMonth[ym] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	months, err := monthsBetween(p.From, p.To)
	if err != nil {
		return nil, err
	}
	out := make([]OperationalMonth, 0, len(months))
	for _, ym := range months {
		out = append(out, OperationalMonth{Month: ym, Count: byMonth[ym]})
	}
	return out, nil
}

// ─── Território ────────────────────────────────────────────────────────

// Territory reúne municípios e bairros das duas fontes de ocorrência que o
// solicitante enxerga — cadastro manual e relatório operacional —, contando
// uma vez a ocorrência que está nas duas (mesma ficha CIOPS).
func (r *Repo) Territory(ctx context.Context, p Period, withIncidents, withOps bool) (cities, neighborhoods []Facet, err error) {
	src := territorySource(withIncidents, withOps)
	if src == "" {
		return []Facet{}, []Facet{}, nil
	}
	args := []any{nilDate(p.From), nilDate(p.To)}
	if cities, err = r.facets(ctx, `
		WITH src AS (`+src+`)
		SELECT city, '', COUNT(*) FROM src
		 WHERE city <> ''
		 GROUP BY city ORDER BY COUNT(*) DESC, city LIMIT 8`, args...); err != nil {
		return nil, nil, err
	}
	// Bairro agrega por (município, bairro): homônimos em cidades diferentes
	// são outro território.
	if neighborhoods, err = r.facets(ctx, `
		WITH src AS (`+src+`)
		SELECT neighborhood, city, COUNT(*) FROM src
		 WHERE neighborhood <> ''
		 GROUP BY neighborhood, city ORDER BY COUNT(*) DESC, neighborhood LIMIT 8`, args...); err != nil {
		return nil, nil, err
	}
	return cities, neighborhoods, nil
}

func territorySource(withIncidents, withOps bool) string {
	const incidents = `
		SELECT city, neighborhood FROM app.incidents
		 WHERE deleted_at IS NULL
		   AND ($1::date IS NULL OR occurred_on >= $1::date)
		   AND ($2::date IS NULL OR occurred_on <= $2::date)`
	ops := `
		SELECT o.place_city AS city, o.place_neighborhood AS neighborhood
		  FROM app.ops_occurrences o
		 WHERE o.deleted_at IS NULL
		   AND ($1::date IS NULL OR o.occurred_on >= $1::date)
		   AND ($2::date IS NULL OR o.occurred_on <= $2::date)` + opsDedupe(withIncidents)
	switch {
	case withIncidents && withOps:
		return incidents + ` UNION ALL ` + ops
	case withIncidents:
		return incidents
	case withOps:
		return ops
	}
	return ""
}
