// Package dashboard agrega os números do painel operacional.
//
// Este pacote só lê. Cada método devolve um bloco fechado do painel
// (ocorrências, relatórios, informes, entidades) e o handler decide quais
// blocos montar conforme as permissões efetivas de quem pediu — o painel é
// único e se recorta pela matriz RBAC, não pelo papel.
//
// Convenções que valem para todas as consultas:
//
//   - Soft delete: nada aqui conta linha com deleted_at preenchido.
//   - Ocorrências e informes datam por campo `date` (occurred_on), então não
//     há conversão de fuso. Já os carimbos de produção (created_at,
//     diffused_at) são timestamptz e precisam ser lidos no fuso da agência
//     antes de virar dia — senão a virada de mês escorrega três horas.
//   - Relatórios e informes repetem, nas agregações, o mesmo predicado de
//     visibilidade das listagens. Contar o que o solicitante não pode abrir
//     vazaria a existência do documento pelo número.
package dashboard

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

// Repo agrega sobre o banco da aplicação.
type Repo struct {
	db *sql.DB
	// tz é o fuso da agência (ex.: "America/Fortaleza"), usado para converter
	// timestamptz em dia local nas métricas de produção.
	tz string
}

// New cria o Repo. tz vazio cai em UTC.
func New(db *sql.DB, tz string) *Repo {
	if tz == "" {
		tz = "UTC"
	}
	return &Repo{db: db, tz: tz}
}

// Period é um recorte fechado [From, To] em datas ISO (YYYY-MM-DD). Campo
// vazio = sem limite daquele lado.
type Period struct {
	From string
	To   string
}

// Window reúne os três recortes que o painel usa de uma vez: o período
// corrente, o período imediatamente anterior (base da variação) e o início
// da série histórica de 12 meses.
type Window struct {
	Current  Period
	Previous Period
	// Series é sempre concreto: os 12 meses que terminam no mês final de
	// Current. É a régua contra a qual o número do período é lido.
	Series Period
}

// Access carrega o recorte de visibilidade do solicitante, espelhando o que
// as listagens de relatórios e informes já aplicam.
type Access struct {
	UserID    string
	Clearance int
	IsAdmin   bool
}

// Facet é um par rótulo/contagem. City só é preenchida nos bairros, onde o
// nome sozinho é ambíguo entre municípios.
type Facet struct {
	Name  string
	City  string
	Count int
}

// MonthPoint é um mês da série histórica de ocorrências.
type MonthPoint struct {
	Month     string // YYYY-MM
	Homicidio int
	Apreensao int
	Prisao    int
}

// IncidentStats é o bloco de ocorrências do painel.
type IncidentStats struct {
	ByType        map[string]int
	PrevByType    map[string]int
	Series        []MonthPoint
	Means         []Facet
	Cities        []Facet
	Neighborhoods []Facet
	// Geocoded conta quantas ocorrências do período têm coordenadas — mede a
	// cobertura do mapa do crime, que depende de preenchimento manual.
	Geocoded int
}

// ReportStats é o bloco de relatórios de inteligência (RIs).
type ReportStats struct {
	// ByStatus é o acervo visível inteiro (estoque), não o período.
	ByStatus map[string]int
	Created  int
	Diffused int
	// PrevDiffused é a difusão do período anterior, base da variação do KPI.
	PrevDiffused int
}

// InformeStats é o bloco de informes.
type InformeStats struct {
	Total   int
	Created int
	Prev    int
}

// EntityStats é o bloco de entidades cadastradas.
type EntityStats struct {
	ByKind  map[string]int
	Created int
	Prev    int
	// Deceased é o total de pessoas marcadas como falecidas no acervo.
	Deceased int
}

// nilDate transforma data vazia em NULL, para o SQL tratar o lado como
// "sem limite" em vez de comparar com string vazia.
func nilDate(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ─── Ocorrências ───────────────────────────────────────────────────────

// Incidents devolve o bloco de ocorrências. Ocorrência não tem sigilo por
// registro nesta versão — quem tem incident.read vê o conjunto inteiro.
func (r *Repo) Incidents(ctx context.Context, w Window) (*IncidentStats, error) {
	st := &IncidentStats{}

	var err error
	if st.ByType, err = r.countByType(ctx, w.Current); err != nil {
		return nil, err
	}
	if st.PrevByType, err = r.countByType(ctx, w.Previous); err != nil {
		return nil, err
	}
	if st.Series, err = r.series(ctx, w.Series); err != nil {
		return nil, err
	}
	if st.Means, err = r.meansFacets(ctx, w.Current); err != nil {
		return nil, err
	}
	if st.Cities, err = r.cityFacets(ctx, w.Current); err != nil {
		return nil, err
	}
	if st.Neighborhoods, err = r.neighborhoodFacets(ctx, w.Current); err != nil {
		return nil, err
	}
	if err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM app.incidents
		 WHERE deleted_at IS NULL
		   AND latitude IS NOT NULL AND longitude IS NOT NULL
		   AND ($1::date IS NULL OR occurred_on >= $1::date)
		   AND ($2::date IS NULL OR occurred_on <= $2::date)`,
		nilDate(w.Current.From), nilDate(w.Current.To),
	).Scan(&st.Geocoded); err != nil {
		return nil, err
	}
	return st, nil
}

func (r *Repo) countByType(ctx context.Context, p Period) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT type, COUNT(*)
		  FROM app.incidents
		 WHERE deleted_at IS NULL
		   AND ($1::date IS NULL OR occurred_on >= $1::date)
		   AND ($2::date IS NULL OR occurred_on <= $2::date)
		 GROUP BY type`, nilDate(p.From), nilDate(p.To))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[t] = n
	}
	return out, rows.Err()
}

// series devolve os meses do recorte em ordem cronológica, com os meses sem
// ocorrência presentes e zerados — o gráfico precisa do vão para mostrar a
// queda, e o banco só devolve o que existe.
func (r *Repo) series(ctx context.Context, p Period) ([]MonthPoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT to_char(date_trunc('month', occurred_on), 'YYYY-MM') AS ym,
		       type,
		       COUNT(*)
		  FROM app.incidents
		 WHERE deleted_at IS NULL
		   AND occurred_on >= $1::date
		   AND occurred_on <= $2::date
		 GROUP BY 1, 2`, p.From, p.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byMonth := map[string]*MonthPoint{}
	for rows.Next() {
		var ym, t string
		var n int
		if err := rows.Scan(&ym, &t, &n); err != nil {
			return nil, err
		}
		mp, ok := byMonth[ym]
		if !ok {
			mp = &MonthPoint{Month: ym}
			byMonth[ym] = mp
		}
		switch t {
		case "homicidio":
			mp.Homicidio = n
		case "apreensao":
			mp.Apreensao = n
		case "prisao":
			mp.Prisao = n
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	months, err := monthsBetween(p.From, p.To)
	if err != nil {
		return nil, err
	}
	out := make([]MonthPoint, 0, len(months))
	for _, ym := range months {
		if mp, ok := byMonth[ym]; ok {
			out = append(out, *mp)
			continue
		}
		out = append(out, MonthPoint{Month: ym})
	}
	return out, nil
}

// meansFacets distribui os homicídios do período pelo meio empregado. Só
// CVLI: em apreensão e prisão o campo não se aplica e entraria como ruído
// no balde "não informado".
func (r *Repo) meansFacets(ctx context.Context, p Period) ([]Facet, error) {
	return r.facets(ctx, `
		SELECT means, '', COUNT(*)
		  FROM app.incidents
		 WHERE deleted_at IS NULL
		   AND type = 'homicidio'
		   AND ($1::date IS NULL OR occurred_on >= $1::date)
		   AND ($2::date IS NULL OR occurred_on <= $2::date)
		 GROUP BY means
		 ORDER BY COUNT(*) DESC, means`, nilDate(p.From), nilDate(p.To))
}

func (r *Repo) cityFacets(ctx context.Context, p Period) ([]Facet, error) {
	return r.facets(ctx, `
		SELECT city, '', COUNT(*)
		  FROM app.incidents
		 WHERE deleted_at IS NULL
		   AND city <> ''
		   AND ($1::date IS NULL OR occurred_on >= $1::date)
		   AND ($2::date IS NULL OR occurred_on <= $2::date)
		 GROUP BY city
		 ORDER BY COUNT(*) DESC, city
		 LIMIT 8`, nilDate(p.From), nilDate(p.To))
}

// neighborhoodFacets agrega por (município, bairro): bairro homônimo em
// municípios diferentes é outro território, e somá-los inventaria uma
// concentração que não existe.
func (r *Repo) neighborhoodFacets(ctx context.Context, p Period) ([]Facet, error) {
	return r.facets(ctx, `
		SELECT neighborhood, city, COUNT(*)
		  FROM app.incidents
		 WHERE deleted_at IS NULL
		   AND neighborhood <> ''
		   AND ($1::date IS NULL OR occurred_on >= $1::date)
		   AND ($2::date IS NULL OR occurred_on <= $2::date)
		 GROUP BY neighborhood, city
		 ORDER BY COUNT(*) DESC, neighborhood
		 LIMIT 8`, nilDate(p.From), nilDate(p.To))
}

func (r *Repo) facets(ctx context.Context, query string, args ...any) ([]Facet, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Facet, 0, 8)
	for rows.Next() {
		var f Facet
		if err := rows.Scan(&f.Name, &f.City, &f.Count); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ─── Relatórios ────────────────────────────────────────────────────────

// Reports devolve o bloco de RIs, contando apenas o que o solicitante
// poderia abrir na listagem.
func (r *Repo) Reports(ctx context.Context, w Window, ac Access) (*ReportStats, error) {
	vis, args := reportVisibility(ac)
	st := &ReportStats{ByStatus: map[string]int{}}

	rows, err := r.db.QueryContext(ctx, `
		SELECT r.status, COUNT(*)
		  FROM app.reports r
		 WHERE r.deleted_at IS NULL AND `+vis+`
		 GROUP BY r.status`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		st.ByStatus[s] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if st.Created, err = r.countReports(ctx, ac, docDateExpr, w.Current); err != nil {
		return nil, err
	}
	if st.Diffused, err = r.countReports(ctx, ac, diffusedDateExpr, w.Current); err != nil {
		return nil, err
	}
	if st.PrevDiffused, err = r.countReports(ctx, ac, diffusedDateExpr, w.Previous); err != nil {
		return nil, err
	}
	return st, nil
}

// Expressões de data das métricas de relatório. `{tz}` é substituído pelo
// placeholder do fuso na montagem da query; expressão sem marcador não
// recebe o parâmetro, porque o Postgres recusa parâmetro que a query não
// referencia.
const (
	// doc_date é a data do documento (campo `date`), que é como a agência
	// conta "os RIs do mês" — e não o instante em que a linha nasceu.
	docDateExpr      = "r.doc_date"
	diffusedDateExpr = "(r.diffused_at AT TIME ZONE {tz})::date"
)

// countReports conta relatórios visíveis cuja data (expressa por dateExpr)
// cai no período. dateExpr é uma das constantes acima, nunca vem do request.
func (r *Repo) countReports(ctx context.Context, ac Access, dateExpr string, p Period) (int, error) {
	vis, args := reportVisibility(ac)
	args = append(args, nilDate(p.From), nilDate(p.To))
	from := "$" + strconv.Itoa(len(args)-1)
	to := "$" + strconv.Itoa(len(args))
	if strings.Contains(dateExpr, "{tz}") {
		args = append(args, r.tz)
		dateExpr = strings.ReplaceAll(dateExpr, "{tz}", "$"+strconv.Itoa(len(args)))
	}
	var out int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM app.reports r
		 WHERE r.deleted_at IS NULL
		   AND `+vis+`
		   AND `+dateExpr+` IS NOT NULL
		   AND (`+from+`::date IS NULL OR `+dateExpr+` >= `+from+`::date)
		   AND (`+to+`::date IS NULL OR `+dateExpr+` <= `+to+`::date)`,
		args...).Scan(&out)
	return out, err
}

// reportVisibility replica o predicado da listagem de relatórios: admin vê
// tudo; os demais veem o que criaram, mais o que é aberto (ou onde estão na
// lista de leitores) e cabe no seu clearance.
func reportVisibility(ac Access) (string, []any) {
	if ac.IsAdmin {
		return "TRUE", nil
	}
	return `(
		r.created_by = $1
		OR (
		     (r.visibility = 'aberto'
		      OR EXISTS (SELECT 1 FROM app.report_viewers v
		                  WHERE v.report_id = r.id AND v.user_id = $1))
		     AND r.required_clearance <= $2
		)
	)`, []any{ac.UserID, ac.Clearance}
}

// ─── Informes ──────────────────────────────────────────────────────────

// Informes devolve o bloco de informes, contando só o que o solicitante
// poderia abrir (autor ou clearance suficiente).
func (r *Repo) Informes(ctx context.Context, w Window, ac Access) (*InformeStats, error) {
	vis := "TRUE"
	var base []any
	if !ac.IsAdmin {
		vis = "(i.created_by = $1 OR i.required_clearance <= $2)"
		base = []any{ac.UserID, ac.Clearance}
	}
	n := len(base)

	st := &InformeStats{}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM app.informes i
		 WHERE i.deleted_at IS NULL AND `+vis, base...,
	).Scan(&st.Total); err != nil {
		return nil, err
	}

	count := func(p Period) (int, error) {
		args := append(append([]any{}, base...), nilDate(p.From), nilDate(p.To), r.tz)
		var out int
		err := r.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			  FROM app.informes i
			 WHERE i.deleted_at IS NULL
			   AND `+vis+`
			   AND ($`+strconv.Itoa(n+1)+`::date IS NULL
			        OR (i.created_at AT TIME ZONE $`+strconv.Itoa(n+3)+`)::date >= $`+strconv.Itoa(n+1)+`::date)
			   AND ($`+strconv.Itoa(n+2)+`::date IS NULL
			        OR (i.created_at AT TIME ZONE $`+strconv.Itoa(n+3)+`)::date <= $`+strconv.Itoa(n+2)+`::date)`,
			args...).Scan(&out)
		return out, err
	}

	var err error
	if st.Created, err = count(w.Current); err != nil {
		return nil, err
	}
	if st.Prev, err = count(w.Previous); err != nil {
		return nil, err
	}
	return st, nil
}

// ─── Entidades ─────────────────────────────────────────────────────────

// Entities devolve o bloco do acervo de entidades. Entidade não tem sigilo
// por registro — quem tem entity.list vê o conjunto.
func (r *Repo) Entities(ctx context.Context, w Window) (*EntityStats, error) {
	st := &EntityStats{ByKind: map[string]int{}}

	rows, err := r.db.QueryContext(ctx, `
		SELECT kind, COUNT(*)
		  FROM app.entities
		 WHERE deleted_at IS NULL
		 GROUP BY kind`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		st.ByKind[k] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	count := func(p Period) (int, error) {
		var out int
		err := r.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			  FROM app.entities
			 WHERE deleted_at IS NULL
			   AND ($1::date IS NULL OR (created_at AT TIME ZONE $3)::date >= $1::date)
			   AND ($2::date IS NULL OR (created_at AT TIME ZONE $3)::date <= $2::date)`,
			nilDate(p.From), nilDate(p.To), r.tz).Scan(&out)
		return out, err
	}
	if st.Created, err = count(w.Current); err != nil {
		return nil, err
	}
	if st.Prev, err = count(w.Previous); err != nil {
		return nil, err
	}

	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM app.entity_persons p
		  JOIN app.entities e ON e.id = p.entity_id AND e.deleted_at IS NULL
		 WHERE p.deceased`).Scan(&st.Deceased); err != nil {
		return nil, err
	}
	return st, nil
}
