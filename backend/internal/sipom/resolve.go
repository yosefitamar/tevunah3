package sipom

import (
	"regexp"
	"strings"
)

// Códigos de pendência gravados em ops_occurrences.sipom_pendencias. Os
// bloqueantes impedem o envio da ocorrência; os de composição não (o SIPOM
// aceita a ocorrência sem composição, que se completa depois).
const (
	PendNatureza   = "natureza"           // sem natureza SIPOM
	PendNatConfirm = "natureza_confirmar" // natureza veio de regra sugerida
	PendDataHora   = "data_hora"          // sem hora inicial
	PendFicha      = "ficha"              // sem ficha CIOPS (nº da ocorrência)
	PendLogradouro = "logradouro"         // endereço sem logradouro
	PendCidade     = "cidade"             // cidade fora do catálogo
	PendBairro     = "bairro"             // bairro em branco
	PendArea       = "area"               // área da unidade não resolvida
	PendAreaAmbig  = "area_ambigua"
	PendOPM        = "opm" // companhia que atendeu não está no catálogo

	PendComposicao      = "composicao"        // equipe não reconhecida
	PendComposicaoMulti = "composicao_multi"  // mais de uma equipe na ficha
	PendPessoaSemDossie = "pessoa_sem_dossie" // identificado sem dossiê: vai só nome e mãe
)

// Blocking diz se a pendência impede o envio.
func Blocking(code string) bool {
	return code != PendComposicao && code != PendComposicaoMulti && code != PendPessoaSemDossie
}

// PendingLabel é o texto da pendência para a tela.
var PendingLabel = map[string]string{
	PendNatureza:        "Natureza SIPOM não definida — nenhuma natureza da ficha tem correspondência",
	PendNatConfirm:      "Natureza SIPOM sugerida pelo de-para — confirme ou corrija",
	PendDataHora:        "Sem hora do fato",
	PendFicha:           "Sem nº da ocorrência (ficha CIOPS)",
	PendLogradouro:      "Endereço sem logradouro",
	PendCidade:          "Cidade fora do catálogo do SIPOM",
	PendBairro:          "Sem bairro",
	PendArea:            "Área da unidade militar não encontrada para o local",
	PendAreaAmbig:       "Mais de uma área possível para o local — escolha a correta",
	PendOPM:             "Companhia que atendeu não existe (ativa) no SIPOM",
	PendComposicao:      "Equipe não reconhecida (esperado VTRA ou RAIO)",
	PendComposicaoMulti: "Mais de uma equipe na ficha — defina a equipe de cada policial",
	PendPessoaSemDossie: "Envolvido identificado sem dossiê — vai ao SIPOM só com nome e mãe",
}

// Tipos de policiamento e funções do SIPOM (policiamentos_tipos/_funcoes).
const (
	PolicMotorizado        = 6
	PolicMotopatrulhamento = 7

	FuncComandante    = 1
	FuncSubcomandante = 2
	FuncTerceiroHomem = 3
	FuncGarupa        = 4
	FuncPatrulheiro   = 5
	FuncMotorista     = 7
	FuncQuintoHomem   = 13
)

// Input é o que a resolução lê da ocorrência.
type Input struct {
	NaturezaID *int // já decidida (de-para ou analista)
	// NaturezaSuggested: veio de regra 'sugerida' e o analista ainda não
	// confirmou.
	NaturezaSuggested bool
	HasTime           bool
	Ficha             string
	Address           string // linha "Local Ocorrência" como veio
	Neighborhood      string
	City              string
	CIA               string // "1ª CIA"
	BPM               string // "2º BPRAIO"
	Teams             string // "RAIO 01; RAIO 02"
	Officers          int    // policiais na composição, em ordem
	// UnlinkedPeople: envolvidos identificados sem dossiê vinculado.
	UnlinkedPeople int
}

// Officer é a tradução de um policial da composição.
type Officer struct {
	Equipe             string
	PoliciamentoTipoID *int
	FuncaoID           *int
}

// Result é a ocorrência nos códigos do SIPOM.
type Result struct {
	Logradouro string
	Numeral    string
	CidadeID   *int
	BairroID   *int
	AreaID     *int
	// AreaOptions são as áreas candidatas quando o local cai em mais de uma.
	AreaOptions []int
	OPMID       *int
	Officers    []Officer
	Pending     []string
}

// Resolve traduz a ocorrência. Pura: só consulta o catálogo em memória.
func (c *Catalog) Resolve(in Input) Result {
	var r Result
	switch {
	case in.NaturezaID == nil:
		r.Pending = append(r.Pending, PendNatureza)
	case in.NaturezaSuggested:
		r.Pending = append(r.Pending, PendNatConfirm)
	}
	if !in.HasTime {
		r.Pending = append(r.Pending, PendDataHora)
	}
	if strings.TrimSpace(in.Ficha) == "" {
		r.Pending = append(r.Pending, PendFicha)
	}

	r.Logradouro, r.Numeral = SplitAddress(in.Address)
	if r.Logradouro == "" {
		r.Pending = append(r.Pending, PendLogradouro)
	}

	if ci, ok := c.cidadeByKey[key(in.City)]; ok {
		id := ci.id
		r.CidadeID = &id
		if b, ok := c.bairroByKey[id][key(in.Neighborhood)]; ok && key(in.Neighborhood) != "" {
			r.BairroID = &b
		}
	} else {
		r.Pending = append(r.Pending, PendCidade)
	}
	if strings.TrimSpace(in.Neighborhood) == "" {
		r.Pending = append(r.Pending, PendBairro)
	}

	r.AreaOptions = c.areas(r.CidadeID, r.BairroID)
	switch {
	case len(r.AreaOptions) == 1:
		id := r.AreaOptions[0]
		r.AreaID = &id
		r.AreaOptions = nil
	case len(r.AreaOptions) > 1:
		r.Pending = append(r.Pending, PendAreaAmbig)
	case r.CidadeID != nil:
		r.Pending = append(r.Pending, PendArea)
	}

	if co, ok := c.OPM(in.CIA, in.BPM); ok {
		id := co.ID
		r.OPMID = &id
	} else {
		r.Pending = append(r.Pending, PendOPM)
	}

	var pend string
	r.Officers, pend = Composition(in.Teams, in.Officers)
	if pend != "" {
		r.Pending = append(r.Pending, pend)
	}
	if in.UnlinkedPeople > 0 {
		r.Pending = append(r.Pending, PendPessoaSemDossie)
	}
	return r
}

// areas devolve as companhias territoriais do local: a regra do bairro, se
// houver; senão a da cidade inteira.
func (c *Catalog) areas(cidadeID, bairroID *int) []int {
	if bairroID != nil {
		if ids := c.areaByBairro[*bairroID]; len(ids) > 0 {
			return append([]int(nil), ids...)
		}
	}
	if cidadeID != nil {
		return append([]int(nil), c.areaByCidade[*cidadeID]...)
	}
	return nil
}

// OPM acha a companhia que atendeu ("1ª CIA" + "2º BPRAIO" →
// "1ªCIA/2ºBPRAIO"). O pelotão não existe como unidade no SIPOM: a
// ocorrência vai para a companhia.
func (c *Catalog) OPM(cia, bpm string) (Companhia, bool) {
	if strings.TrimSpace(cia) == "" || strings.TrimSpace(bpm) == "" {
		return Companhia{}, false
	}
	co, ok := c.companhiaByKey[key(cia+"/"+bpm)]
	return co, ok
}

// ─── Endereço ───

var (
	reNumberStart = regexp.MustCompile(`(?i)^(n[º°o]?\.?\s*)?(\d+[a-z]?|s\s*/?\s*n[º°o]?)\b\.?\s*(.*)$`)
	reTrailingNum = regexp.MustCompile(`(?i)^(.*\D)\s+(n[º°o]?\.?\s*)?(\d+[a-z]?)$`)
	reSN          = regexp.MustCompile(`(?i)^s\s*/?\s*n[º°o]?$`)
)

// SplitAddress separa logradouro e número da linha de endereço do relatório.
// O número é o primeiro trecho, depois de uma vírgula, que começa por
// algarismo ou "S/N"; sem vírgula, um número no fim do primeiro trecho. O
// complemento que vier depois acompanha o número ("335 - Casa 39a"), que é o
// único campo livre que sobra no SIPOM.
//
//	"Rua Fabiano De Cristo, s/n"     → "Rua Fabiano De Cristo" | "S/N"
//	"Rua José Abílio 335, Casa 39a"  → "Rua José Abílio"       | "335 - Casa 39a"
//	"Rua 25 de Março, 100"           → "Rua 25 de Março"       | "100"
//	"Zona Rural"                     → "Zona Rural"            | ""
func SplitAddress(addr string) (logradouro, numeral string) {
	addr = strings.Join(strings.Fields(addr), " ")
	if addr == "" {
		return "", ""
	}
	parts := strings.Split(addr, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	for i := 1; i < len(parts); i++ {
		if m := reNumberStart.FindStringSubmatch(parts[i]); m != nil {
			rest := append([]string{m[3]}, parts[i+1:]...)
			return strings.Join(nonEmpty(parts[:i]), ", "), joinNumeral(m[2], rest)
		}
	}
	if m := reTrailingNum.FindStringSubmatch(parts[0]); m != nil && strings.TrimSpace(m[1]) != "" {
		return strings.TrimSpace(m[1]), joinNumeral(m[3], parts[1:])
	}
	return strings.Join(nonEmpty(parts), ", "), ""
}

func joinNumeral(num string, rest []string) string {
	if reSN.MatchString(strings.TrimSpace(num)) {
		num = "S/N"
	}
	if comp := strings.Join(nonEmpty(rest), ", "); comp != "" {
		return num + " - " + comp
	}
	return num
}

func nonEmpty(ss []string) []string {
	out := ss[:0:0]
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ─── Composição ───

// CompositionByTeam monta a composição quando o analista definiu a equipe
// de cada policial (ficha com mais de uma equipe): a regra VTRA/RAIO vale
// dentro de cada equipe, na ordem em que os policiais aparecem na lista.
// Equipe vazia ou não reconhecida devolve pendência.
func CompositionByTeam(teams []string) ([]Officer, string) {
	out := make([]Officer, len(teams))
	groups := map[string][]int{}
	var order []string
	for i, t := range teams {
		t = strings.ToUpper(strings.Join(strings.Fields(t), " "))
		if t == "" {
			return make([]Officer, len(teams)), PendComposicaoMulti
		}
		if _, ok := groups[t]; !ok {
			order = append(order, t)
		}
		groups[t] = append(groups[t], i)
	}
	for _, t := range order {
		idx := groups[t]
		offs, pend := Composition(t, len(idx))
		if pend != "" {
			return make([]Officer, len(teams)), pend
		}
		for k, i := range idx {
			out[i] = offs[k]
		}
	}
	return out, ""
}

// Composition deriva tipo de policiamento e função de cada policial a partir
// da equipe e da ordem na lista (regra do batalhão):
//
//	"VTRA ***" (viatura) → Motorizado: 1 Comandante, 2 Motorista, demais Patrulheiro.
//	"RAIO **"  (motos)   → Motopatrulhamento: 1 Comandante, 2 Subcomandante,
//	                       3 3º homem, 4 Garupa, 5 5º homem.
//
// Com mais de uma equipe na ficha não dá para saber quem estava em qual:
// volta pendente para o analista definir.
func Composition(teams string, n int) ([]Officer, string) {
	out := make([]Officer, n)
	if n == 0 {
		return out, ""
	}
	var list []string
	for _, t := range strings.FieldsFunc(teams, func(r rune) bool { return r == ';' || r == ',' }) {
		if t = strings.TrimSpace(t); t != "" {
			list = append(list, t)
		}
	}
	if len(list) > 1 {
		return out, PendComposicaoMulti
	}
	if len(list) == 0 {
		return out, PendComposicao
	}
	team := strings.ToUpper(strings.Join(strings.Fields(list[0]), " "))
	var tipo int
	var funcs []int
	switch {
	case strings.HasPrefix(key(team), "vtr"):
		tipo, funcs = PolicMotorizado, []int{FuncComandante, FuncMotorista}
	case strings.HasPrefix(key(team), "raio"):
		tipo, funcs = PolicMotopatrulhamento,
			[]int{FuncComandante, FuncSubcomandante, FuncTerceiroHomem, FuncGarupa, FuncQuintoHomem}
	default:
		return out, PendComposicao
	}
	// Mais de cinco numa equipe de motos não tem função no SIPOM.
	if tipo == PolicMotopatrulhamento && n > len(funcs) {
		return out, PendComposicao
	}
	for i := range out {
		t := tipo
		out[i] = Officer{Equipe: team, PoliciamentoTipoID: &t}
		f := FuncPatrulheiro // da viatura, do terceiro em diante
		if i < len(funcs) {
			f = funcs[i]
		}
		out[i].FuncaoID = &f
	}
	return out, ""
}
