// Package sipom traduz as ocorrências do Tevunah para os códigos do SIPOM
// (sipom.pm.ce.gov.br), destino do envio. O catálogo vem do schema sipom
// (migrações 00051/00052, cópia fiel das tabelas de referência deles).
package sipom

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
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

	// Procedimento: tipos, delegacias (por código de unidade e por id) e
	// delegados ativos (por id e por nome normalizado).
	procedimentos   map[int]string
	delegacias      map[int]Delegacia
	delegaciaByCode map[int]int // código ("201") → id
	delegaciaList   []Delegacia
	delegados       map[int]Delegado
	delegadoList    []Delegado // só ativos, em ordem de nome

	// Materiais: listas dos selects do modal "Material".
	armaTipos         *refList
	armaMarcas        *refList
	armaCalibres      *refList
	drogas            map[int]Droga
	drogaList         []Droga
	celularMarcas     *refList
	veiculoTipos      *codeList
	veiculoCores      *codeList
	marcasModelos     []MarcaModelo          // tabela DENATRAN inteira, para busca
	marcaModeloKey    map[string]MarcaModelo // descrição normalizada → linha
	marcaModeloByCode map[int]MarcaModelo
}

// Delegacia é uma unidade da Polícia Civil. Code é o número que abre o nome
// no SIPOM ("201-DELEGACIA METROPOLITANA DE CAUCAIA") — e que também abre o
// número do procedimento no relatório ("201-1234/2026").
type Delegacia struct {
	ID   int
	Code int
	Nome string
}

// Delegado é um delegado do catálogo (Key = nome normalizado).
type Delegado struct {
	ID   int
	Nome string
	Key  string
}

// Droga é um item da lista de drogas do SIPOM, com a unidade de medida em
// que a quantidade é informada.
type Droga struct {
	ID      int
	Nome    string
	Unidade string
}

// MarcaModelo é uma linha da tabela DENATRAN de marca/modelo de veículo.
type MarcaModelo struct {
	Codigo    int
	Descricao string // "HONDA/CG 160 FAN"
}

// Ref é um item de lista de referência (id + nome).
type Ref struct {
	ID   int
	Nome string
}

// refList é uma lista de referência por id e por nome normalizado.
type refList struct {
	items []Ref
	byID  map[int]string
	byKey map[string]int
}

func newRefList() *refList {
	return &refList{byID: map[int]string{}, byKey: map[string]int{}}
}

func (l *refList) add(id int, nome string) {
	l.items = append(l.items, Ref{ID: id, Nome: nome})
	l.byID[id] = nome
	l.byKey[key(nome)] = id
}

// codeList é uma lista cujo valor enviado ao SIPOM é um código (DENATRAN),
// não o id da tabela.
type codeList struct {
	items  []Ref // ID = código
	byCode map[int]string
	byKey  map[string]int
}

func newCodeList() *codeList {
	return &codeList{byCode: map[int]string{}, byKey: map[string]int{}}
}

func (l *codeList) add(code int, nome string) {
	l.items = append(l.items, Ref{ID: code, Nome: nome})
	l.byCode[code] = nome
	l.byKey[key(nome)] = code
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

		procedimentos: map[int]string{}, delegacias: map[int]Delegacia{}, delegaciaByCode: map[int]int{},
		delegados: map[int]Delegado{},
		armaTipos: newRefList(), armaMarcas: newRefList(), armaCalibres: newRefList(),
		drogas: map[int]Droga{}, celularMarcas: newRefList(),
		veiculoTipos: newCodeList(), veiculoCores: newCodeList(),
		marcaModeloKey: map[string]MarcaModelo{}, marcaModeloByCode: map[int]MarcaModelo{},
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
		{"procedimentos", `SELECT id, nome FROM sipom.procedimentos ORDER BY id`, func(r *sql.Rows) error {
			var id int
			var nome string
			if err := r.Scan(&id, &nome); err != nil {
				return err
			}
			c.procedimentos[id] = nome
			return nil
		}},
		{"delegacias", `SELECT id, nome FROM sipom.delegacias ORDER BY nome`, func(r *sql.Rows) error {
			var d Delegacia
			if err := r.Scan(&d.ID, &d.Nome); err != nil {
				return err
			}
			if m := reDelegaciaCode.FindStringSubmatch(d.Nome); m != nil {
				d.Code, _ = strconv.Atoi(m[1])
				if _, dup := c.delegaciaByCode[d.Code]; !dup {
					c.delegaciaByCode[d.Code] = d.ID
				}
			}
			c.delegacias[d.ID] = d
			c.delegaciaList = append(c.delegaciaList, d)
			return nil
		}},
		{"delegados", `SELECT id, nome, deletado_em IS NOT NULL FROM sipom.delegados ORDER BY nome`, func(r *sql.Rows) error {
			var d Delegado
			var deleted bool
			if err := r.Scan(&d.ID, &d.Nome, &deleted); err != nil {
				return err
			}
			d.Nome = strings.ToUpper(strings.Join(strings.Fields(d.Nome), " "))
			d.Key = key(d.Nome)
			c.delegados[d.ID] = d
			if !deleted {
				c.delegadoList = append(c.delegadoList, d)
			}
			return nil
		}},
		{"arma_tipos", `SELECT id, nome FROM sipom.arma_tipos WHERE deletado_em IS NULL ORDER BY id`, refScan(c.armaTipos)},
		{"arma_marcas", `SELECT id, nome FROM sipom.arma_marcas WHERE deletado_em IS NULL ORDER BY id`, refScan(c.armaMarcas)},
		{"arma_calibres", `SELECT id, nome FROM sipom.arma_calibres WHERE deletado_em IS NULL ORDER BY id`, refScan(c.armaCalibres)},
		{"celular_marcas", `SELECT id, nome FROM sipom.celular_marcas ORDER BY id`, refScan(c.celularMarcas)},
		{"drogas", `SELECT id, nome, unidade FROM sipom.drogas ORDER BY id`, func(r *sql.Rows) error {
			var d Droga
			if err := r.Scan(&d.ID, &d.Nome, &d.Unidade); err != nil {
				return err
			}
			c.drogas[d.ID] = d
			c.drogaList = append(c.drogaList, d)
			return nil
		}},
		{"veiculo_tipos", `SELECT codigo_tipos_veiculos, nome FROM sipom.veiculo_tipos WHERE nome <> '' ORDER BY id`, codeScan(c.veiculoTipos)},
		{"veiculo_cores", `SELECT codigo_cor, nome FROM sipom.veiculo_cores ORDER BY id`, codeScan(c.veiculoCores)},
		{"marcasmodelos", `SELECT codigo, COALESCE(descricao, '') FROM sipom.marcasmodelos ORDER BY id`, func(r *sql.Rows) error {
			var m MarcaModelo
			if err := r.Scan(&m.Codigo, &m.Descricao); err != nil {
				return err
			}
			if m.Descricao == "" {
				return nil
			}
			c.marcasModelos = append(c.marcasModelos, m)
			c.marcaModeloByCode[m.Codigo] = m
			if _, dup := c.marcaModeloKey[key(m.Descricao)]; !dup {
				c.marcaModeloKey[key(m.Descricao)] = m
			}
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

// reDelegaciaCode: o código da unidade que abre o nome da delegacia.
var reDelegaciaCode = regexp.MustCompile(`^\s*(\d+)\s*-`)

func refScan(l *refList) func(*sql.Rows) error {
	return func(r *sql.Rows) error {
		var id int
		var nome string
		if err := r.Scan(&id, &nome); err != nil {
			return err
		}
		l.add(id, nome)
		return nil
	}
}

func codeScan(l *codeList) func(*sql.Rows) error {
	return func(r *sql.Rows) error {
		var code int
		var nome string
		if err := r.Scan(&code, &nome); err != nil {
			return err
		}
		l.add(code, nome)
		return nil
	}
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

// ─── Procedimento e materiais: acesso ───

// Procedimento devolve o nome do tipo de procedimento.
func (c *Catalog) Procedimento(id int) (string, bool) {
	n, ok := c.procedimentos[id]
	return n, ok
}

// Procedimentos devolve os tipos de procedimento em ordem de id.
func (c *Catalog) Procedimentos() []Ref {
	out := make([]Ref, 0, len(c.procedimentos))
	for id, n := range c.procedimentos {
		out = append(out, Ref{ID: id, Nome: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Delegacia devolve a delegacia pelo id.
func (c *Catalog) Delegacia(id int) (Delegacia, bool) {
	d, ok := c.delegacias[id]
	return d, ok
}

// DelegaciaByCode devolve a delegacia pelo código de unidade ("939").
func (c *Catalog) DelegaciaByCode(code int) (Delegacia, bool) {
	id, ok := c.delegaciaByCode[code]
	if !ok {
		return Delegacia{}, false
	}
	return c.delegacias[id], true
}

// Delegacias devolve a lista inteira, em ordem de nome.
func (c *Catalog) Delegacias() []Delegacia { return c.delegaciaList }

// Delegado devolve o delegado pelo id (inclusive excluído).
func (c *Catalog) Delegado(id int) (Delegado, bool) {
	d, ok := c.delegados[id]
	return d, ok
}

// Delegados devolve os delegados ativos, em ordem de nome.
func (c *Catalog) Delegados() []Delegado { return c.delegadoList }

// ArmaTipos, ArmaMarcas, ArmaCalibres e CelularMarcas devolvem as listas
// dos selects do material; os nomes por id ficam em *Nome.
func (c *Catalog) ArmaTipos() []Ref              { return c.armaTipos.items }
func (c *Catalog) ArmaMarcas() []Ref             { return c.armaMarcas.items }
func (c *Catalog) ArmaCalibres() []Ref           { return c.armaCalibres.items }
func (c *Catalog) CelularMarcas() []Ref          { return c.celularMarcas.items }
func (c *Catalog) ArmaTipoNome(id int) string    { return c.armaTipos.byID[id] }
func (c *Catalog) ArmaMarcaNome(id int) string   { return c.armaMarcas.byID[id] }
func (c *Catalog) ArmaCalibreNome(id int) string { return c.armaCalibres.byID[id] }

// Drogas devolve a lista de drogas; Droga, uma pelo id.
func (c *Catalog) Drogas() []Droga { return c.drogaList }
func (c *Catalog) Droga(id int) (Droga, bool) {
	d, ok := c.drogas[id]
	return d, ok
}

// VeiculoTipos e VeiculoCores devolvem as listas (ID = código DENATRAN).
func (c *Catalog) VeiculoTipos() []Ref             { return c.veiculoTipos.items }
func (c *Catalog) VeiculoCores() []Ref             { return c.veiculoCores.items }
func (c *Catalog) VeiculoTipoNome(code int) string { return c.veiculoTipos.byCode[code] }
func (c *Catalog) VeiculoCorNome(code int) string  { return c.veiculoCores.byCode[code] }

// MarcaModelo devolve a linha DENATRAN pelo código.
func (c *Catalog) MarcaModelo(code int) (MarcaModelo, bool) {
	m, ok := c.marcaModeloByCode[code]
	return m, ok
}

// SearchMarcasModelos procura marca/modelo por texto: todas as palavras do
// termo precisam aparecer na descrição. É a busca do select do SIPOM.
func (c *Catalog) SearchMarcasModelos(q string, limit int) []MarcaModelo {
	words := strings.Fields(key(q))
	if len(words) == 0 {
		return nil
	}
	var out []MarcaModelo
	for _, m := range c.marcasModelos {
		k := key(m.Descricao)
		ok := true
		for _, w := range words {
			if !strings.Contains(k, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, m)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}
