package sipom

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/belia/tevunah/backend/internal/intel"
)

// Procedimento (aba Procedimentos do SIPOM): tipo, número/ano, delegacia e
// delegado, a partir do que o relatório operacional escreve em texto.

// Pendências do procedimento e dos materiais.
const (
	PendProcedimento = "procedimento"        // relatório sem procedimento
	PendProcTipo     = "procedimento_tipo"   // tipo não reconhecido
	PendProcNumero   = "procedimento_numero" // número fora do padrão "nnn/aaaa"
	PendDelegacia    = "delegacia"           // delegacia não identificada
	PendDelegado     = "delegado"            // delegado não identificado (aviso: não é obrigatório)
	PendArma         = "arma"                // alguma arma sem tipo, marca ou calibre do SIPOM
	PendDroga        = "droga"               // alguma droga sem correspondência ou sem quantidade na unidade
	PendVeiculo      = "veiculo"             // algum veículo sem tipo, cor, marca/modelo ou situação
	// PendVeiculoPlaca: veículo sem placa (que não é ciclomotor) — o analista
	// informa a placa ou confirma que não foi possível identificá-la.
	PendVeiculoPlaca = "veiculo_placa"
)

// Tipos de procedimento (sipom.procedimentos).
const (
	ProcIP             = 1
	ProcTCO            = 2
	ProcBO             = 3
	ProcAtoInfracional = 4
)

// ProcInput é o que a resolução do procedimento lê da ocorrência.
type ProcInput struct {
	Type     string // "BO", "IP", "TCO"… como o relatório escreve
	Number   string // "939-7635/2026"
	Station  string // "Delegacia:" do relatório ("DMC", "22° DP")
	Delegate string // "Delegado(a):" do relatório, em geral abreviado
	Terms    *TermMap

	// Fixados pelo analista (valem sobre o automático).
	ProcedimentoID, DelegaciaID, DelegadoID     *int
	ManualProc, ManualDelegacia, ManualDelegado bool
}

// ProcItem é um procedimento da ocorrência nos códigos do SIPOM, com o texto
// do relatório de onde saiu.
type ProcItem struct {
	ProcedimentoID *int
	Numero, Ano    string
	// Code é o código de unidade que abre o número ("939"); 0 se não há.
	Code                   int
	TipoTexto, NumeroTexto string
}

// ProcResult é o procedimento nos códigos do SIPOM. Uma ocorrência pode ter
// mais de um — o caso típico é prisão de maiores com apreensão de menor: um
// IP e um AI (ato infracional), cada um com seu número. O relatório os
// escreve no mesmo campo, separados por ";" ou ",". Delegacia e delegado são
// os mesmos para todos.
type ProcResult struct {
	// Itens: um por procedimento, na ordem do relatório.
	Itens []ProcItem
	// ProcedimentoID, Numero, Ano e Code são os do primeiro item.
	ProcedimentoID *int
	Numero, Ano    string
	Code           int
	DelegaciaID    *int
	DelegadoID     *int
	// DelegadoCandidates: delegados que casam com o nome abreviado quando
	// mais de um serve — o analista escolhe.
	DelegadoCandidates []int
	Pending            []string
}

var (
	// "939-7635/2026", "7635/2026", "939 - 7635 / 2026".
	reProcNumber = regexp.MustCompile(`^\s*(?:(\d+)\s*-\s*)?(\d+)\s*/\s*(\d{4})\s*$`)
	// "22° DP", "22º DP", "22 DP", "22o DP", "DP 22".
	reDistrito = regexp.MustCompile(`(?i)(?:^|\D)(\d{1,2})\s*[°ºo]?\s*(?:dp|distrito)|(?:dp|distrito)\s*(\d{1,2})(?:\D|$)`)
)

// procTypes: como o relatório escreve o tipo → sipom.procedimentos.
var procTypes = map[string]int{
	"ip": ProcIP, "inquerito": ProcIP, "inquerito policial": ProcIP,
	"tco": ProcTCO, "termo circunstanciado": ProcTCO, "termo circunstanciado de ocorrencia": ProcTCO,
	"bo": ProcBO, "boletim": ProcBO, "boletim de ocorrencia": ProcBO, "bop": ProcBO,
	"ai": ProcAtoInfracional, "ato infracional": ProcAtoInfracional, "baai": ProcAtoInfracional,
}

// splitProc separa o campo do relatório que traz mais de um procedimento
// ("AI;IP", "939-874/2026, 939-64/2026").
func splitProc(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == ',' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ResolveProcedure traduz o procedimento. Pura: só consulta o catálogo em
// memória e os termos aprendidos.
func (c *Catalog) ResolveProcedure(in ProcInput) ProcResult {
	var r ProcResult
	typ, num := strings.TrimSpace(in.Type), strings.TrimSpace(in.Number)
	if typ == "" && num == "" && !in.ManualProc {
		r.Pending = append(r.Pending, PendProcedimento)
		return r
	}

	// Um item por procedimento: tipo e número são casados pela posição
	// ("AI;IP" + "874/2026;64/2026" → AI 874, IP 64). Um tipo só para vários
	// números vale para todos; número não se repete.
	types, nums := splitProc(typ), splitProc(num)
	n := max(len(types), len(nums), 1)
	missType, missNum := false, false
	for i := 0; i < n; i++ {
		var it ProcItem
		switch {
		case i < len(types):
			it.TipoTexto = types[i]
		case len(types) == 1:
			it.TipoTexto = types[0]
		}
		if i < len(nums) {
			it.NumeroTexto = nums[i]
		}
		// Tipo: o que o analista fixou vale para o primeiro (o único, quando a
		// ocorrência tem um procedimento só).
		if i == 0 && in.ManualProc {
			it.ProcedimentoID = in.ProcedimentoID
		} else if id, ok := procTypes[key(it.TipoTexto)]; ok {
			it.ProcedimentoID = &id
		}
		// Número/ano, com o código da delegacia que abre o número.
		if m := reProcNumber.FindStringSubmatch(it.NumeroTexto); m != nil {
			it.Code, _ = strconv.Atoi(m[1])
			it.Numero, it.Ano = m[2], m[3]
		}
		missType = missType || it.ProcedimentoID == nil
		missNum = missNum || it.Numero == ""
		if r.Code == 0 {
			r.Code = it.Code
		}
		r.Itens = append(r.Itens, it)
	}
	r.ProcedimentoID, r.Numero, r.Ano = r.Itens[0].ProcedimentoID, r.Itens[0].Numero, r.Itens[0].Ano
	if missType {
		r.Pending = append(r.Pending, PendProcTipo)
	}
	if missNum {
		r.Pending = append(r.Pending, PendProcNumero)
	}

	// Delegacia: a fixada; senão o código do número; senão o termo
	// aprendido; senão "N° DP" → distrito policial (código 100 + N).
	switch {
	case in.ManualDelegacia:
		r.DelegaciaID = in.DelegaciaID
	default:
		if r.Code != 0 {
			if d, ok := c.DelegaciaByCode(r.Code); ok {
				id := d.ID
				r.DelegaciaID = &id
			}
		}
		if r.DelegaciaID == nil {
			if id, ok := in.Terms.Lookup(TermDelegacia, in.Station); ok {
				if _, exists := c.delegacias[id]; exists {
					r.DelegaciaID = &id
				}
			}
		}
		if r.DelegaciaID == nil {
			if n := distritoNumber(in.Station); n > 0 {
				if d, ok := c.DelegaciaByCode(100 + n); ok {
					id := d.ID
					r.DelegaciaID = &id
				}
			}
		}
	}
	if r.DelegaciaID == nil {
		r.Pending = append(r.Pending, PendDelegacia)
	}

	// Delegado: a fixada; senão o termo aprendido; senão a busca pelo nome
	// (abreviado) no catálogo.
	switch {
	case in.ManualDelegado:
		r.DelegadoID = in.DelegadoID
	default:
		if id, ok := in.Terms.Lookup(TermDelegado, in.Delegate); ok {
			if _, exists := c.delegados[id]; exists {
				r.DelegadoID = &id
			}
		}
		if r.DelegadoID == nil && strings.TrimSpace(in.Delegate) != "" {
			id, cands := c.MatchDelegado(in.Delegate)
			if id != 0 {
				r.DelegadoID = &id
			} else {
				r.DelegadoCandidates = cands
			}
		}
	}
	if r.DelegadoID == nil && !in.ManualDelegado {
		r.Pending = append(r.Pending, PendDelegado)
	}
	return r
}

// distritoNumber extrai o número do distrito policial de "22° DP"; 0 se não
// há.
func distritoNumber(station string) int {
	m := reDistrito.FindStringSubmatch(station)
	if m == nil {
		return 0
	}
	for _, g := range m[1:] {
		if g != "" {
			n, _ := strconv.Atoi(g)
			return n
		}
	}
	return 0
}

// Conectivos que não distinguem nomes.
var nameStop = map[string]bool{"de": true, "da": true, "do": true, "das": true, "dos": true, "e": true, "dr": true, "dra": true}

func nameTokens(s string) []string {
	var out []string
	for _, t := range strings.Fields(key(s)) {
		if !nameStop[t] {
			out = append(out, t)
		}
	}
	return out
}

// MatchDelegado procura o delegado pelo nome como o relatório o escreve —
// em geral abreviado ("Ítalo Renno Alves" para ITALO RENNO ALVES FEITOSA) e
// muitas vezes com erro de digitação. Casa quando as palavras do nome
// aparecem, na ordem, entre as do nome do catálogo; uma inicial ("R.") casa
// qualquer palavra que comece por ela.
//
// A comparação afrouxa em três passos, e só passa ao seguinte quando o
// anterior não achou ninguém: palavra igual; mesmo som ("Sousa"/"Souza",
// "Renno"/"Reno"); som a uma letra de distância ("Rodruigues", "Galinho").
// Em cada passo o nome completo vence; senão, um único candidato é aceito;
// vários voltam como candidatos para o analista. Só delegados ativos entram.
func (c *Catalog) MatchDelegado(name string) (id int, candidates []int) {
	want := nameTokens(name)
	if len(want) == 0 {
		return 0, nil
	}
	for _, eq := range []func(a, b string) bool{sameWord, sameSound, nearSound} {
		candidates = nil
		for _, d := range c.delegadoList {
			have := nameTokens(d.Nome)
			if !tokensSubsequence(want, have, eq) {
				continue
			}
			if len(want) == len(have) {
				return d.ID, nil
			}
			candidates = append(candidates, d.ID)
		}
		if len(candidates) == 1 {
			return candidates[0], nil
		}
		if len(candidates) > 1 {
			return 0, candidates
		}
	}
	return 0, nil
}

func sameWord(a, b string) bool  { return a == b }
func sameSound(a, b string) bool { return intel.SoundKey(a) == intel.SoundKey(b) }
func nearSound(a, b string) bool { return intel.NearSound(a, b) }

// tokensSubsequence diz se want aparece, na ordem, dentro de have, com eq
// decidindo se duas palavras são a mesma. O primeiro nome tem que ser o
// primeiro: "ALVES FEITOSA" não é o Ítalo.
func tokensSubsequence(want, have []string, eq func(a, b string) bool) bool {
	if len(want) == 0 || len(have) == 0 || !tokenMatch(want[0], have[0], eq) {
		return false
	}
	i := 1
	for _, h := range have[1:] {
		if i == len(want) {
			break
		}
		if tokenMatch(want[i], h, eq) {
			i++
		}
	}
	return i == len(want)
}

func tokenMatch(want, have string, eq func(a, b string) bool) bool {
	if len(want) == 1 {
		return strings.HasPrefix(have, want)
	}
	return eq(want, have)
}
