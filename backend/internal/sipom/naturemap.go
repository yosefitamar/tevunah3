package sipom

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
)

// De-para de naturezas: relatório operacional → natureza_fatos do SIPOM
// (tabela app.sipom_natureza_map, gerida pelo administrador).

const (
	ConfDireta   = "direta"   // aplica sozinha
	ConfSugerida = "sugerida" // aplica, mas pede confirmação do analista
	// Aplica sozinha quando a ficha traz a apreensão que corrobora a natureza
	// (arma de fogo na tabela de armas / item na tabela de drogas); sem a
	// evidência, vale como sugerida.
	ConfComArma  = "com_arma"
	ConfComDroga = "com_droga"
)

// Evidence é o que a ficha traz de apreensão, usado para decidir se uma
// regra condicionada à evidência aplica sozinha.
type Evidence struct {
	Firearm bool // arma de fogo na tabela de armas apreendidas
	Drug    bool // item na tabela de drogas apreendidas
}

// firearmWords: tipos de arma do relatório que são arma de fogo. Arma branca,
// simulacro e o que não se reconhece não corroboram a natureza.
var firearmWords = []string{
	"pistola", "revolver", "espingarda", "carabina", "fuzil", "rifle", "metralhadora",
	"submetralhadora", "garrucha", "escopeta", "artesanal", "arma de fogo",
}

// IsFirearm diz se o tipo de arma, como o relatório escreve, é arma de fogo.
func IsFirearm(kind string) bool {
	k := key(kind)
	if k == "" || strings.Contains(k, "simulacro") || strings.Contains(k, "branca") {
		return false
	}
	for _, w := range firearmWords {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
}

// direct diz se a regra aplica sozinha diante do que a ficha traz.
func (r NatureRule) direct(ev Evidence) bool {
	switch r.Confianca {
	case ConfDireta:
		return true
	case ConfComArma:
		return ev.Firearm
	case ConfComDroga:
		return ev.Drug
	}
	return false
}

var ErrEmptySource = errors.New("informe a natureza do relatório")

// legalRef corta a citação legal: "FURTO - ART. 155/CPB" e "FURTO" caem na
// mesma regra.
var legalRef = regexp.MustCompile(`(?i)\s*-\s*ART\b.*$`)

// NatureKey é a forma de comparação de uma natureza do relatório.
func NatureKey(s string) string { return key(legalRef.ReplaceAllString(s, "")) }

// NatureRule é uma linha do de-para.
type NatureRule struct {
	ID         string
	Source     string
	NaturezaID *int
	Confianca  string
	Prioridade int
	UpdatedAt  time.Time
}

// NatureMap é o de-para em memória, por chave.
type NatureMap struct {
	rules map[string]NatureRule
}

// NatureChoice é a natureza SIPOM escolhida para uma ocorrência.
type NatureChoice struct {
	NaturezaID *int
	// Suggested: veio de regra 'sugerida' — pendente até o analista
	// confirmar.
	Suggested bool
}

// Choose escolhe a natureza principal entre as da ficha: a regra mapeada de
// menor prioridade (a mais grave); empate fica com a que vem primeiro.
// Naturezas sem regra, ou com regra sem destino, não concorrem. ev é a
// apreensão que a ficha traz: decide as regras condicionadas à evidência.
//
// Quando as naturezas da ficha levam a mais de uma natureza do SIPOM (arma e
// droga na mesma ocorrência, por exemplo), a escolha é sempre sugerida: qual
// delas prevalece depende do inquérito, e quem confere é o analista. Natureza
// que ainda não tem linha no de-para conta como outra — não se sabe para
// onde iria; a que tem linha sem destino foi descartada de propósito.
func (m *NatureMap) Choose(natures []string, ev Evidence) NatureChoice {
	var best *NatureRule
	targets := map[int]bool{}
	unknown := false
	for _, n := range natures {
		r, ok := m.rules[NatureKey(n)]
		if !ok {
			unknown = unknown || strings.TrimSpace(n) != ""
			continue
		}
		if r.NaturezaID == nil {
			continue
		}
		targets[*r.NaturezaID] = true
		if best == nil || r.Prioridade < best.Prioridade {
			r := r
			best = &r
		}
	}
	if best == nil {
		return NatureChoice{}
	}
	id := *best.NaturezaID
	return NatureChoice{NaturezaID: &id, Suggested: len(targets) > 1 || unknown || !best.direct(ev)}
}

// Has diz se a natureza já tem linha no de-para (mapeada ou não).
func (m *NatureMap) Has(nature string) bool {
	_, ok := m.rules[NatureKey(nature)]
	return ok
}

// MapRepo encapsula app.sipom_natureza_map.
type MapRepo struct {
	db *sql.DB
}

func NewMapRepo(db *sql.DB) *MapRepo { return &MapRepo{db: db} }

// List devolve as regras em ordem de prioridade.
func (r *MapRepo) List(ctx context.Context) ([]NatureRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source, sipom_natureza_id, confianca, prioridade, updated_at
		  FROM app.sipom_natureza_map
		 ORDER BY prioridade, source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NatureRule{}
	for rows.Next() {
		var x NatureRule
		if err := rows.Scan(&x.ID, &x.Source, &x.NaturezaID, &x.Confianca, &x.Prioridade, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// Load monta o de-para em memória (lido a cada importação/recálculo: a
// tabela é pequena e muda pelo Admin).
func (r *MapRepo) Load(ctx context.Context) (*NatureMap, error) {
	rules, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	m := &NatureMap{rules: map[string]NatureRule{}}
	for _, x := range rules {
		m.rules[NatureKey(x.Source)] = x
	}
	return m, nil
}

// Upsert grava a regra da natureza (criando se não existir). source é
// gravado sem a citação legal, como chave legível.
func (r *MapRepo) Upsert(ctx context.Context, source string, naturezaID *int, confianca string,
	prioridade int, actor string,
) (*NatureRule, error) {
	source = strings.Join(strings.Fields(legalRef.ReplaceAllString(source, "")), " ")
	if NatureKey(source) == "" {
		return nil, ErrEmptySource
	}
	switch confianca {
	case ConfDireta, ConfComArma, ConfComDroga:
	default:
		confianca = ConfSugerida
	}
	// A chave de comparação é a do Go (NatureKey); o índice único do banco
	// (norm_txt) só impede a mesma grafia duas vezes. Por isso a busca da
	// linha existente é feita aqui.
	rules, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	var id string
	for _, x := range rules {
		if NatureKey(x.Source) == NatureKey(source) {
			id = x.ID
		}
	}
	var natID any
	if naturezaID != nil {
		natID = *naturezaID
	}
	if id == "" {
		err = r.db.QueryRowContext(ctx, `
			INSERT INTO app.sipom_natureza_map (source, sipom_natureza_id, confianca, prioridade, updated_by)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			source, natID, confianca, prioridade, actor).Scan(&id)
	} else {
		_, err = r.db.ExecContext(ctx, `
			UPDATE app.sipom_natureza_map
			   SET sipom_natureza_id = $2, confianca = $3, prioridade = $4,
			       updated_at = now(), updated_by = $5
			 WHERE id = $1`, id, natID, confianca, prioridade, actor)
	}
	if err != nil {
		return nil, err
	}
	var x NatureRule
	err = r.db.QueryRowContext(ctx, `
		SELECT id, source, sipom_natureza_id, confianca, prioridade, updated_at
		  FROM app.sipom_natureza_map WHERE id = $1`, id).
		Scan(&x.ID, &x.Source, &x.NaturezaID, &x.Confianca, &x.Prioridade, &x.UpdatedAt)
	return &x, err
}
