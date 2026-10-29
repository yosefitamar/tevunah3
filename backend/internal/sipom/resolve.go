package sipom

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
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
	// PendBairroCatalogo: o bairro do relatório não casa com nenhum do
	// catálogo da cidade — o SIPOM exige bairro_id.
	PendBairroCatalogo = "bairro_catalogo"
	PendArea           = "area" // área da unidade não resolvida
	PendAreaAmbig      = "area_ambigua"
	PendOPM            = "opm" // companhia que atendeu não está no catálogo

	PendComposicao      = "composicao"        // equipe não reconhecida
	PendComposicaoMulti = "composicao_multi"  // mais de uma equipe na ficha
	PendPessoaSemDossie = "pessoa_sem_dossie" // identificado sem dossiê: vai só nome e mãe
	// PendPessoaSexo: envolvido identificado sem sexo (nem no dossiê nem
	// informado na ficha) — o SIPOM exige sexo de toda pessoa.
	PendPessoaSexo = "pessoa_sexo"
	// PendPessoaSexoConfirm: há sexo sugerido (nome, termo aprendido ou
	// histórico) esperando a confirmação do analista.
	PendPessoaSexoConfirm = "pessoa_sexo_confirmar"
	// PendCoordenada: o endereço não foi localizado no mapa. O SIPOM exige
	// latitude e longitude.
	PendCoordenada = "coordenada"

	// Tabela de vínculos do SIPOM ausente (sipom.vinculos, 00066): sem ela
	// não há vinculo_id para mandar.
	PendCatalogoVinculo = "catalogo_vinculo"
)

// Notice diz se o código é só um aviso: não impede nada, mas a ocorrência
// ainda não conta como verificada. O delegado entra aqui porque o SIPOM não
// o exige — mas é desejável, e por isso continua pendência da ocorrência.
func Notice(code string) bool {
	return code == PendComposicao || code == PendComposicaoMulti || code == PendPessoaSemDossie ||
		code == PendDelegado
}

// Blocking diz se a pendência impede o envio ao SIPOM.
func Blocking(code string) bool {
	return !Notice(code)
}

// Etapas da ficha da ocorrência em que cada pendência se resolve — o mesmo
// agrupamento que a tela usa na trilha de etapas.
const (
	StepFato         = "fato"
	StepLocal        = "local"
	StepEfetivo      = "efetivo"
	StepPessoas      = "pessoas"
	StepProcedimento = "procedimento"
	StepApreensoes   = "apreensoes"
)

// PendingStep diz a etapa de cada pendência.
var PendingStep = map[string]string{
	PendNatureza: StepFato, PendNatConfirm: StepFato, PendDataHora: StepFato, PendFicha: StepFato,
	PendLogradouro: StepLocal, PendCidade: StepLocal, PendBairro: StepLocal, PendBairroCatalogo: StepLocal, PendArea: StepLocal,
	PendAreaAmbig: StepLocal, PendCoordenada: StepLocal,
	PendOPM: StepEfetivo, PendComposicao: StepEfetivo, PendComposicaoMulti: StepEfetivo,
	PendPessoaSemDossie: StepPessoas, PendPessoaSexo: StepPessoas, PendPessoaSexoConfirm: StepPessoas, PendCatalogoVinculo: StepPessoas,
	PendProcedimento: StepProcedimento, PendProcTipo: StepProcedimento, PendProcNumero: StepProcedimento,
	PendDelegacia: StepProcedimento, PendDelegado: StepProcedimento,
	PendArma: StepApreensoes, PendDroga: StepApreensoes, PendVeiculo: StepApreensoes,
}

// StepCodes devolve as pendências da etapa; NoticeCodes, as que não impedem
// o envio.
func StepCodes(step string) []string {
	var out []string
	for code, s := range PendingStep {
		if s == step {
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out
}

func NoticeCodes() []string {
	var out []string
	for code := range PendingStep {
		if !Blocking(code) {
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out
}

// PendingLabel é o texto da pendência para a tela.
var PendingLabel = map[string]string{
	PendNatureza:          "Natureza SIPOM não definida — nenhuma natureza da ficha tem correspondência",
	PendNatConfirm:        "Natureza SIPOM sugerida pelo de-para — confirme ou corrija",
	PendDataHora:          "Sem hora do fato",
	PendFicha:             "Sem nº da ocorrência (ficha CIOPS)",
	PendLogradouro:        "Endereço sem logradouro",
	PendCidade:            "Cidade fora do catálogo do SIPOM",
	PendBairro:            "Sem bairro",
	PendBairroCatalogo:    "Bairro fora do catálogo do SIPOM — escolha o bairro da cidade",
	PendArea:              "Área da unidade militar não encontrada para o local",
	PendAreaAmbig:         "Mais de uma área possível para o local — escolha a correta",
	PendOPM:               "Companhia que atendeu não existe (ativa) no SIPOM",
	PendComposicao:        "Equipe não reconhecida (esperado VTRA ou RAIO)",
	PendComposicaoMulti:   "Mais de uma equipe na ficha — defina a equipe de cada policial",
	PendPessoaSemDossie:   "Envolvido identificado sem dossiê — vai ao SIPOM só com nome e mãe",
	PendPessoaSexo:        "Envolvido identificado sem sexo — informe na ficha ou no dossiê",
	PendPessoaSexoConfirm: "Sexo do envolvido sugerido (pelo nome ou pelo histórico) — confirme",
	PendCoordenada:        "Sem coordenada — o endereço não foi localizado no mapa",
	PendCatalogoVinculo:   "Tabela de vínculos do SIPOM não carregada — envolvidos não podem ir",
	PendProcedimento:      "Sem procedimento no relatório",
	PendProcTipo:          "Tipo de procedimento não reconhecido (IP, TCO, BO ou Ato Infracional)",
	PendProcNumero:        "Número do procedimento fora do padrão (ex.: 939-7635/2026)",
	PendDelegacia:         "Delegacia não identificada — escolha a do catálogo do SIPOM",
	PendDelegado:          "Delegado não identificado pelo nome — escolha, se souber (não é obrigatório)",
	PendArma:              "Arma sem tipo do SIPOM",
	PendDroga:             "Droga sem correspondência na lista do SIPOM, ou sem quantidade na unidade dela",
	PendVeiculo:           "Veículo sem tipo, cor, marca/modelo ou situação do SIPOM",
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
	// People: envolvidos (identificados ou não); PeopleWithoutSex: os
	// identificados sem sexo nem sugestão; PeopleSexSuggested: os que só têm
	// a sugestão, à espera da confirmação.
	People, PeopleWithoutSex, PeopleSexSuggested int
	// GeoRequired: o geocodificador está ligado, então ocorrência sem
	// coordenada (HasGeo) fica pendente.
	GeoRequired bool
	HasGeo      bool
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
		if b, ok := c.MatchBairro(id, in.Neighborhood); ok {
			r.BairroID = &b
		}
	} else {
		r.Pending = append(r.Pending, PendCidade)
	}
	switch {
	case strings.TrimSpace(in.Neighborhood) == "":
		r.Pending = append(r.Pending, PendBairro)
	case r.CidadeID != nil && r.BairroID == nil:
		r.Pending = append(r.Pending, PendBairroCatalogo)
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
	if in.PeopleWithoutSex > 0 {
		r.Pending = append(r.Pending, PendPessoaSexo)
	}
	if in.PeopleSexSuggested > 0 {
		r.Pending = append(r.Pending, PendPessoaSexoConfirm)
	}
	if in.People > 0 && len(c.vinculos) == 0 {
		r.Pending = append(r.Pending, PendCatalogoVinculo)
	}
	if in.GeoRequired && !in.HasGeo {
		r.Pending = append(r.Pending, PendCoordenada)
	}
	return r
}

// Areas devolve as companhias territoriais do local: a regra do bairro, se
// houver; senão a da cidade inteira.
func (c *Catalog) Areas(cidadeID, bairroID *int) []int { return c.areas(cidadeID, bairroID) }

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
// ocorrência vai para a companhia. Vale também a companhia já desativada no
// SIPOM: se o relatório a cita, ela atendeu (a 3ª e a 4ª CIA do 2º BPRAIO
// constam como desativadas desde julho/2026, mas atuaram até agosto, e o
// próprio SIPOM registra assim as ocorrências delas).
func (c *Catalog) OPM(cia, bpm string) (Companhia, bool) {
	if strings.TrimSpace(cia) == "" || strings.TrimSpace(bpm) == "" {
		return Companhia{}, false
	}
	k := key(cia + "/" + bpm)
	if co, ok := c.companhiaByKey[k]; ok {
		return co, true
	}
	co, ok := c.companhiaRetired[k]
	return co, ok
}

// MatchBairro procura o bairro do relatório entre os da cidade no catálogo.
// O nome exato vence; senão vale o que soa igual ("Pial" → PIAU) e, por
// último, o que difere do som em uma letra por palavra ("Jereissate I" →
// JEREISSATI I, "Sepupará" → Sapupara). Sem bairro parecido na cidade — ou
// com mais de um — não casa: o nome fica como o relatório escreveu.
func (c *Catalog) MatchBairro(cidadeID int, name string) (int, bool) {
	k := key(name)
	if k == "" {
		return 0, false
	}
	bairros := c.bairroByKey[cidadeID]
	if id, ok := bairros[k]; ok {
		return id, true
	}
	want := strings.Fields(k)
	for _, eq := range []func(a, b string) bool{sameSound, nearSound} {
		found, n := 0, 0
		for bk, id := range bairros {
			have := strings.Fields(bk)
			if len(have) != len(want) {
				continue
			}
			ok := true
			for i := range want {
				// Palavra curta é numeral ou sigla ("I", "II", "3"): só a
				// própria, senão "Jereissati I" e "II" viram o mesmo bairro.
				if short := len(want[i]) <= 3 || len(have[i]) <= 3; (short && want[i] != have[i]) || (!short && !eq(want[i], have[i])) {
					ok = false
					break
				}
			}
			if ok {
				found, n = id, n+1
			}
		}
		if n == 1 {
			return found, true
		}
		if n > 1 {
			return 0, false
		}
	}
	return 0, false
}

// CanonNeighborhood devolve o bairro como o catálogo o escreve, quando o do
// relatório só casa com um da cidade pelo som (erro de digitação); senão, o
// próprio nome. É o que vai à busca no mapa.
func (c *Catalog) CanonNeighborhood(city, neighborhood string) string {
	ci, ok := c.cidadeByKey[key(city)]
	if !ok {
		return neighborhood
	}
	id, ok := c.MatchBairro(ci.id, neighborhood)
	if !ok || key(c.bairroNome[id]) == key(neighborhood) {
		return neighborhood
	}
	// O catálogo tem nome com sujeira na ponta (": MONGUBA").
	return strings.TrimFunc(c.bairroNome[id], func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
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
// Com mais de uma equipe na ficha vale a primeira citada, para todos os
// policiais: o relatório não diz quem estava em qual, e a primeira é a que
// atendeu. O analista ainda pode definir a equipe de cada um na ficha.
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
