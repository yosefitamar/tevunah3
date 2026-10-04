// Package sipom traduz as ocorrências do Tevunah para os códigos do SIPOM
// (sipom.pm.ce.gov.br), destino do envio. O catálogo vem do schema sipom
// (migrações 00051/00052, cópia fiel das tabelas de referência deles).
package sipom

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/belia/tevunah/backend/internal/intel"
)

// Companhia é uma unidade do SIPOM: serve tanto de área (companhia
// territorial do local do fato) quanto de OPM que atendeu.
type Companhia struct {
	ID        int
	Abreviado string // "1ªCIA/2ºBPRAIO"
}

// Natureza é um item de natureza_fatos.
type Natureza struct {
	ID     int
	Nome   string // texto oficial, o que o SIPOM grava
	Rotulo string // grafia sanitizada, só exibição
}

type cidade struct {
	id     int
	codigo int // IBGE
	nome   string
}

// Catalog é o catálogo do SIPOM em memória. Só muda por migração, então é
// carregado uma vez na subida do servidor.
type Catalog struct {
	naturezas    map[int]Natureza
	naturezaList []Natureza
	companhias   map[int]Companhia
	// companhia ativa por abreviado normalizado ("1 cia 2 bpraio").
	companhiaByKey map[string]Companhia
	cidadeByKey    map[string]cidade
	cidadeNome     map[int]string
	// bairro por cidade e nome normalizado.
	bairroByKey map[int]map[string]int
	bairroNome  map[int]string
	// Área de atuação: companhias por bairro e por cidade inteira.
	areaByBairro map[int][]int
	areaByCidade map[int][]int
	// Nomes dos tipos de policiamento e das funções da composição.
	policiamentos map[int]string
	funcoes       map[int]string
	cidadeIBGE    map[int]int
	// Posto/graduação por abreviado normalizado ("3 sgt").
	postoByKey map[string]int
}

// key é a forma de comparação de nomes: sem acento, caixa, pontuação nem
// ordinal ("1ªCIA/2ºBPRAIO" e "1ª CIA / 2º BPRAIO" viram "1 cia 2 bpraio").
func key(s string) string { return intel.Normalize(s) }

// Load lê o catálogo do banco. Companhias desativadas no SIPOM ficam
// conhecidas pelo id (para exibir o que já foi gravado), mas não entram nas
// buscas: não se envia para unidade extinta.
func Load(ctx context.Context, db *sql.DB) (*Catalog, error) {
	c := &Catalog{
		naturezas:      map[int]Natureza{},
		companhias:     map[int]Companhia{},
		companhiaByKey: map[string]Companhia{},
		cidadeByKey:    map[string]cidade{},
		cidadeNome:     map[int]string{},
		bairroByKey:    map[int]map[string]int{},
		bairroNome:     map[int]string{},
		areaByBairro:   map[int][]int{},
		areaByCidade:   map[int][]int{},
		policiamentos:  map[int]string{},
		funcoes:        map[int]string{},
		cidadeIBGE:     map[int]int{},
		postoByKey:     map[string]int{},
	}
	steps := []struct {
		name string
		sql  string
		scan func(*sql.Rows) error
	}{
		{"naturezas", `SELECT id, nome, rotulo FROM sipom.natureza_fatos ORDER BY rotulo`, func(r *sql.Rows) error {
			var n Natureza
			if err := r.Scan(&n.ID, &n.Nome, &n.Rotulo); err != nil {
				return err
			}
			c.naturezas[n.ID] = n
			c.naturezaList = append(c.naturezaList, n)
			return nil
		}},
		{"companhias", `SELECT id, abreviado, deletado_em IS NOT NULL FROM sipom.companhias`, func(r *sql.Rows) error {
			var co Companhia
			var deleted bool
			if err := r.Scan(&co.ID, &co.Abreviado, &deleted); err != nil {
				return err
			}
			c.companhias[co.ID] = co
			if !deleted {
				c.companhiaByKey[key(co.Abreviado)] = co
			}
			return nil
		}},
		{"cidades", `SELECT id, COALESCE(codigo, 0), nome FROM sipom.cidade`, func(r *sql.Rows) error {
			var ci cidade
			if err := r.Scan(&ci.id, &ci.codigo, &ci.nome); err != nil {
				return err
			}
			c.cidadeByKey[key(ci.nome)] = ci
			c.cidadeNome[ci.id] = ci.nome
			c.cidadeIBGE[ci.id] = ci.codigo
			return nil
		}},
		{"postos", `SELECT id, abreviado FROM sipom.postos_graduacoes`, func(r *sql.Rows) error {
			var id int
			var ab string
			if err := r.Scan(&id, &ab); err != nil {
				return err
			}
			c.postoByKey[key(ab)] = id
			return nil
		}},
		{"bairros", `SELECT id, cidade_id, nome FROM sipom.bairro WHERE deletado_em IS NULL`, func(r *sql.Rows) error {
			var id, cid int
			var nome string
			if err := r.Scan(&id, &cid, &nome); err != nil {
				return err
			}
			if c.bairroByKey[cid] == nil {
				c.bairroByKey[cid] = map[string]int{}
			}
			c.bairroByKey[cid][key(nome)] = id
			c.bairroNome[id] = nome
			return nil
		}},
		{"atuacao", `SELECT a.companhia_id, a.bairro_id, a.cidade_id
		               FROM sipom.companhia_atuacao a
		               JOIN sipom.companhias co ON co.id = a.companhia_id
		              WHERE co.deletado_em IS NULL
		              ORDER BY a.id`, func(r *sql.Rows) error {
			var comp int
			var bairro, cid sql.NullInt64
			if err := r.Scan(&comp, &bairro, &cid); err != nil {
				return err
			}
			switch {
			case bairro.Valid:
				c.areaByBairro[int(bairro.Int64)] = appendUnique(c.areaByBairro[int(bairro.Int64)], comp)
			case cid.Valid:
				c.areaByCidade[int(cid.Int64)] = appendUnique(c.areaByCidade[int(cid.Int64)], comp)
			}
			return nil
		}},
		{"policiamentos", `SELECT id, nome FROM sipom.policiamentos_tipos`, func(r *sql.Rows) error {
			var id int
			var nome string
			if err := r.Scan(&id, &nome); err != nil {
				return err
			}
			c.policiamentos[id] = nome
			return nil
		}},
		{"funcoes", `SELECT id, nome FROM sipom.policiamentos_funcoes`, func(r *sql.Rows) error {
			var id int
			var nome string
			if err := r.Scan(&id, &nome); err != nil {
				return err
			}
			c.funcoes[id] = nome
			return nil
		}},
	}
	for _, s := range steps {
		rows, err := db.QueryContext(ctx, s.sql)
		if err != nil {
			return nil, fmt.Errorf("sipom %s: %w", s.name, err)
		}
		for rows.Next() {
			if err := s.scan(rows); err != nil {
				rows.Close()
				return nil, fmt.Errorf("sipom %s: %w", s.name, err)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("sipom %s: %w", s.name, err)
		}
	}
	return c, nil
}

func appendUnique(ids []int, id int) []int {
	for _, x := range ids {
		if x == id {
			return ids
		}
	}
	return append(ids, id)
}

// CityAreas devolve as áreas que atuam na cidade: a regra da cidade inteira
// e as dos bairros dela. É a lista de escolha quando o bairro da ocorrência
// não tem regra no SIPOM.
func (c *Catalog) CityAreas(cidadeID int) []int {
	var out []int
	for _, id := range c.areaByCidade[cidadeID] {
		out = appendUnique(out, id)
	}
	for _, b := range c.bairroByKey[cidadeID] {
		for _, id := range c.areaByBairro[b] {
			out = appendUnique(out, id)
		}
	}
	// Ordem estável: o mapa de bairros não tem ordem, e a lista é de escolha.
	sort.Slice(out, func(i, j int) bool { return c.companhias[out[i]].Abreviado < c.companhias[out[j]].Abreviado })
	return out
}

// ActiveCompanies devolve todas as companhias ativas, em ordem de nome: a
// lista de escolha da área quando o catálogo não tem nenhuma para a cidade.
func (c *Catalog) ActiveCompanies() []Companhia {
	out := make([]Companhia, 0, len(c.companhiaByKey))
	for _, co := range c.companhiaByKey {
		out = append(out, co)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Abreviado < out[j].Abreviado })
	return out
}

// BattalionCompanies devolve as companhias ativas do batalhão da OPM
// ("2º BPRAIO" → 1ªCIA/2ºBPRAIO, 2ªCIA/2ºBPRAIO): a escolha quando a
// companhia que atendeu não foi encontrada.
func (c *Catalog) BattalionCompanies(bpm string) []Companhia {
	k := key(bpm)
	var out []Companhia
	for ck, co := range c.companhiaByKey {
		if k != "" && strings.HasSuffix(ck, " "+k) {
			out = append(out, co)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Abreviado < out[j].Abreviado })
	return out
}

// IsActiveCompany diz se o id é de companhia ativa no SIPOM.
func (c *Catalog) IsActiveCompany(id int) bool {
	co, ok := c.companhias[id]
	return ok && c.companhiaByKey[key(co.Abreviado)].ID == id
}

// Natureza devolve a natureza pelo id.
func (c *Catalog) Natureza(id int) (Natureza, bool) {
	n, ok := c.naturezas[id]
	return n, ok
}

// Naturezas devolve o catálogo inteiro, em ordem de rótulo.
func (c *Catalog) Naturezas() []Natureza { return c.naturezaList }

// Companhia devolve a companhia pelo id (inclusive desativada).
func (c *Catalog) Companhia(id int) (Companhia, bool) {
	co, ok := c.companhias[id]
	return co, ok
}

// CidadeNome e BairroNome devolvem o nome oficial pelo id.
func (c *Catalog) CidadeNome(id int) string { return c.cidadeNome[id] }
func (c *Catalog) BairroNome(id int) string { return c.bairroNome[id] }

// Policiamento e Funcao devolvem o nome do tipo de policiamento e da função.
func (c *Catalog) Policiamento(id int) string { return c.policiamentos[id] }
func (c *Catalog) Funcao(id int) string       { return c.funcoes[id] }

// CidadeIBGE devolve o código IBGE da cidade (0 se desconhecido).
func (c *Catalog) CidadeIBGE(id int) int { return c.cidadeIBGE[id] }

// Posto acha o posto/graduação do SIPOM pelo do relatório: "3ºSGT" e "2SGT"
// casam direto; "1º TEN QOPM" perde o quadro até casar; "ST" (subtenente) é
// "SUB" no SIPOM.
func (c *Catalog) Posto(rank string) (int, bool) {
	words := strings.Fields(key(rank))
	if len(words) == 1 && words[0] == "st" {
		words[0] = "sub"
	}
	for n := len(words); n > 0; n-- {
		if id, ok := c.postoByKey[strings.Join(words[:n], " ")]; ok {
			return id, true
		}
	}
	return 0, false
}
