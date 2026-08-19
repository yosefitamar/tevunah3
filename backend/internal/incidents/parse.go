// Extração de campos a partir do relatório textual de CVLI que circula nos
// grupos operacionais (WhatsApp). O documento é rotulado — NATUREZA:, DATA:,
// HORÁRIO:, ENDEREÇO: — e dividido em seções (VÍTIMA, CENÁRIO, ENVOLVIDOS),
// o que torna a leitura determinística: nada aqui adivinha, só reconhece
// rótulos e formatos conhecidos.
//
// Cada batalhão escreve à sua maneira: rótulos variam ("HORÁRIO" / "HORA",
// "ENDEREÇO" / "LOCAL DO FATO", "ORCRIM" / "ORCRIM ATUANTE NO LOCAL"), o
// texto vem com negrito do WhatsApp (*NATUREZA:*), com bullets, com o
// cabeçalho de mensagem colado junto ("[09:55, 18/08/2026] Sd Fulano:") e
// com a vítima ora em bloco rotulado, ora em lista. O parser tolera tudo
// isso; o que não reconhecer volta vazio, nunca chutado.
//
// Duas cautelas que o domínio impõe:
//
//   - VÍTIMA registra ÓBITO no acervo inteiro. Quem o relatório marca como
//     LESIONADA/FERIDO, e quem aparece numa TENTATIVA de homicídio, volta
//     SEM papel, com aviso — o analista escolhe.
//   - O que o modelo de ocorrência não comporta (veículo, ORCRIM,
//     logradouro) volta em Warnings, não some em silêncio.
//
// A lista de municípios em ce_municipios.go é gerada de
// frontend/lib/ce-municipios.ts; se a lista do front mudar, regerar.
package incidents

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Situação da pessoa citada, quando o relatório a declara.
const (
	StatusUnknown  = ""
	StatusDeceased = "obito"
	StatusAlive    = "vivo" // lesionado, ferido, socorrido — sobreviveu
)

// ParsedPerson é uma pessoa citada no relatório. Não é uma entidade: o
// cadastro (ou o casamento com um dossiê existente) acontece depois, na
// tela, via busca de homônimos.
type ParsedPerson struct {
	Name        string // MAIÚSCULAS
	MotherName  string // MAIÚSCULAS; "" = não informado
	DateOfBirth string // YYYY-MM-DD; "" = não informado
	CPF         string // só dígitos; "" = não informado
	Alias       string // vulgo/alcunha; "" = não informado
	// Role é RoleVitima | RoleAcusado, ou "" quando o papel não pode ser
	// deduzido com segurança (sobrevivente, tentativa de homicídio).
	Role string
	// Status é o que o relatório declarou sobre a pessoa (ÓBITO/LESIONADA).
	Status string
	// Notes reúne o que o relatório trouxe e a ocorrência não guarda
	// (endereço, antecedentes, naturalidade, pai). Vira descrição inicial do
	// dossiê, se o analista cadastrar a pessoa.
	Notes string
}

// ParsedReport é o que o relatório rendeu. Campos vazios = não reconhecidos;
// cabe à UI manter o valor que o usuário já tinha digitado.
type ParsedReport struct {
	Type         string // homicidio|apreensao|prisao; "" = indeterminado
	Means        string // paf|arma_branca|asfixia|contundente|outros; "" = não informado
	MeansDetail  string
	OccurredOn   string // YYYY-MM-DD
	OccurredTime string // HH:MM
	CIOPSRecord  string // nº da ocorrência/ficha CIOPS
	City         string // MAIÚSCULAS, casado com a lista do Ceará
	Neighborhood string // MAIÚSCULAS
	Description  string // MAIÚSCULAS (como os demais textos livres do sistema)
	MapsURL      string // link cru; a resolução em coordenadas é feita fora
	Latitude     *float64
	Longitude    *float64
	People       []ParsedPerson
	// Warnings são avisos ao analista: dado presente no texto que o cadastro
	// não comporta, reconhecimento parcial, ou decisão que ele precisa tomar.
	Warnings []string
}

// ─── API ───────────────────────────────────────────────────────────────

// ParseReport lê o relatório e devolve os campos reconhecidos. É pura: não
// faz I/O nem consulta o banco (a resolução do link do Maps é do handler).
func ParseReport(text string) ParsedReport {
	var out ParsedReport
	lines := splitLines(text)
	head, sections := splitSections(lines)

	// ── Natureza: tipo e meio ──
	nature := findLabel(head, "NATUREZA", "TIPO DE OCORRENCIA", "TIPO DO FATO", "TIPO")
	// "TENTATIVA DE HOMICÍDIO" não é homicídio: a vítima sobreviveu. Deixar o
	// tipo em branco é melhor do que registrar um CVLI que não houve.
	attempt := strings.Contains(fold(nature), "TENTATIVA")
	if !attempt {
		out.Type = detectType(nature, text)
	}
	out.Means, out.MeansDetail = detectMeans(nature, sectionText(sections, secCenario))

	// ── Data e hora ──
	if d := parseReportDate(findLabel(head, "DATA DA OCORRENCIA", "DATA DO FATO",
		"DATA DO CVLI", "DATA/HORA", "DATA")); d != "" {
		out.OccurredOn = d
	}
	if t := parseReportTime(findLabel(head, "HORARIO DO FATO", "HORA DO FATO",
		"HORARIO", "HORA")); t != "" {
		out.OccurredTime = t
	}
	// "DATA: 17AGO2026 - 17h35" resolve os dois campos de uma linha só.
	if out.OccurredTime == "" {
		out.OccurredTime = parseReportTime(findLabel(head, "DATA/HORA", "DATA"))
	}

	// ── Ficha CIOPS ──
	// Exato, nunca por prefixo: "DATA DA OCORRÊNCIA" também começa com o
	// rótulo e traria a data para cá.
	out.CIOPSRecord = sanitizeRecord(findLabel(head, "OCORRENCIA", "FICHA CIOPS",
		"CIOPS", "REGISTRO CIOPS", "N DA OCORRENCIA", "NUMERO DA OCORRENCIA"))

	// ── Lugar ──
	addr := findLabel(head, "ENDERECO", "LOCAL DO FATO", "LOCAL DA OCORRENCIA",
		"LOCALIZACAO", "LOCAL")
	city, hood, addrWarn := parsePlace(addr)
	out.City, out.Neighborhood = city, hood
	if addrWarn != "" {
		out.Warnings = append(out.Warnings, addrWarn)
	}
	if addr != "" && city == "" {
		out.Warnings = append(out.Warnings,
			"Município não reconhecido no endereço — preencha manualmente.")
	}

	out.MapsURL = firstURL(findLabel(head, "LINK*", "GOOGLE MAPS*", "COORDENADAS", "MAPS*"))
	if out.MapsURL == "" {
		out.MapsURL = firstURL(strings.Join(head, "\n"))
	}
	if lat, lng, ok := coordsFromText(addr + " " + out.MapsURL); ok {
		out.Latitude, out.Longitude = &lat, &lng
	}

	// ── Corpo ──
	out.Description = buildDescription(sections, lines)
	people, peopleWarns := parsePeople(sections, attempt)
	out.People = people
	out.Warnings = append(out.Warnings, peopleWarns...)

	// ── Decisões que sobram para o analista ──
	if attempt {
		out.Warnings = append(out.Warnings,
			"A natureza indica TENTATIVA — o tipo da ocorrência ficou em branco, "+
				"porque o cadastro só registra homicídio consumado, apreensão e prisão.")
	}
	if orcrim := findLabel(head, "ORCRIM*", "AREA DE ATUACAO*", "FACCAO*"); orcrim != "" {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"ORCRIM citada (%s) — a ocorrência não tem campo próprio; consta na descrição.",
			strings.ToUpper(orcrim)))
	}
	for _, plate := range findPlates(text) {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"Veículo citado (placa %s) — vincule pelo dossiê do veículo; a ocorrência só aceita pessoas como envolvidos.",
			plate))
	}
	return out
}

// ─── Seções ────────────────────────────────────────────────────────────

// Chaves canônicas das seções do relatório.
const (
	secVitima    = "VITIMA"
	secCenario   = "CENARIO"
	secEnvolvido = "ENVOLVIDOS"
	secRevide    = "REVIDE"
)

// section é um bloco do relatório: o título como veio escrito e suas linhas.
type section struct {
	key   string // canônico (secVitima…); "" = não reconhecido
	title string // como aparece no texto, p/ recompor a descrição
	lines []string
}

// sectionHeader casa "1. VÍTIMA", "2 - CENÁRIO", "VÍTIMA(S):", "DOS FATOS" —
// título em caixa alta, sozinho na linha, com ou sem numeração. Caixa alta é
// o que separa um cabeçalho de uma frase do relato.
var sectionHeader = regexp.MustCompile(`^\s*(?:(\d{1,2})\s*[.)\-–]\s*)?([A-ZÀ-Ú][A-ZÀ-Ú\s/º°()]{2,40}?)\s*:?\s*$`)

// splitSections separa o preâmbulo (cabeçalho rotulado) das seções.
func splitSections(lines []string) (head []string, secs []section) {
	cur := -1
	for _, ln := range lines {
		if key, title, ok := matchSectionHeader(ln); ok {
			secs = append(secs, section{key: key, title: title})
			cur = len(secs) - 1
			continue
		}
		if cur < 0 {
			head = append(head, ln)
			continue
		}
		secs[cur].lines = append(secs[cur].lines, ln)
	}
	return head, secs
}

// sectionWords mapeia palavra do título → seção canônica. O casamento é por
// palavra inteira: "RELATÓRIO" não pode virar "RELATO", e "DOS FATOS" tem de
// achar "FATOS" na segunda posição.
var sectionWords = map[string]string{
	"VITIMA": secVitima, "VITIMAS": secVitima, "VITIMA(S)": secVitima,
	"CENARIO": secCenario, "FATO": secCenario, "FATOS": secCenario,
	"HISTORICO": secCenario, "RELATO": secCenario, "DINAMICA": secCenario,
	"NARRATIVA": secCenario, "OCORRIDO": secCenario,
	"ENVOLVIDO": secEnvolvido, "ENVOLVIDOS": secEnvolvido, "AUTORIA": secEnvolvido,
	"AUTOR": secEnvolvido, "AUTORES": secEnvolvido, "SUSPEITO": secEnvolvido,
	"SUSPEITOS": secEnvolvido, "ACUSADO": secEnvolvido, "ACUSADOS": secEnvolvido,
	"REVIDE": secRevide,
}

func matchSectionHeader(line string) (key, title string, ok bool) {
	m := sectionHeader.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	// O título entra na descrição como está escrito, numeração inclusive —
	// o relato reconstruído deve ficar reconhecível ao lado do original.
	title = strings.TrimSpace(line)
	numbered := m[1] != ""
	for _, w := range strings.Fields(fold(m[2])) {
		if k, found := sectionWords[w]; found {
			return k, title, true
		}
	}
	// Seção numerada desconhecida ainda é seção — entra na descrição com o
	// título. Sem número, exige-se palavra conhecida para não confundir uma
	// linha em caixa alta do relato com um cabeçalho.
	if numbered {
		return "", title, true
	}
	return "", "", false
}

func sectionText(secs []section, key string) string {
	for _, s := range secs {
		if s.key == key {
			return strings.TrimSpace(strings.Join(s.lines, "\n"))
		}
	}
	return ""
}

// buildDescription monta o relato com todas as seções menos a da vítima
// (essa vira vínculo, não texto). Cenário, envolvidos, inteligência e risco
// de revide entram porque o cadastro não tem outro lugar para guardá-los.
func buildDescription(secs []section, lines []string) string {
	var blocks []string
	for _, s := range secs {
		if s.key == secVitima {
			continue
		}
		if b := blockOf(s); strings.TrimSpace(b) != "" {
			blocks = append(blocks, b)
		}
	}
	if len(blocks) == 0 {
		// Texto sem nenhuma seção reconhecida: melhor entregar tudo do que nada.
		return upperTrimText(strings.Join(lines, "\n"))
	}
	return upperTrimText(strings.Join(blocks, "\n\n"))
}

func blockOf(s section) string {
	body := strings.TrimSpace(strings.Join(s.lines, "\n"))
	if s.title == "" {
		return body
	}
	if body == "" {
		return s.title
	}
	return s.title + "\n" + body
}

// ─── Rótulos ───────────────────────────────────────────────────────────

// findLabel devolve o valor do primeiro rótulo casado, comparando sem acento
// e sem caixa. Um padrão terminado em "*" casa por prefixo — "ORCRIM*" pega
// "ORCRIM", "ORCRIM DO LOCAL" e "ORCRIM ATUANTE NO LOCAL". Sem o "*" o
// casamento é exato, o que evita "DATA" abocanhar "DATA DE NASCIMENTO".
// A ordem dos padrões é a prioridade: do mais específico ao mais genérico.
func findLabel(lines []string, labels ...string) string {
	for _, want := range labels {
		prefix := strings.HasSuffix(want, "*")
		w := fold(strings.TrimSuffix(want, "*"))
		for i, ln := range lines {
			idx := strings.IndexByte(ln, ':')
			if idx < 0 {
				continue
			}
			key := fold(ln[:idx])
			if key != w && !(prefix && strings.HasPrefix(key, w)) {
				continue
			}
			v := strings.TrimSpace(ln[idx+1:])
			// Valor na linha seguinte ("LINK NO GOOGLE MAPS:" sozinho, URL
			// embaixo). A URL contém ":", então o corte não pode ser "tem
			// dois-pontos" — tem de ser "parece um rótulo".
			if v == "" && i+1 < len(lines) && !looksLikeLabel(lines[i+1]) {
				v = strings.TrimSpace(lines[i+1])
			}
			if v != "" {
				return v
			}
		}
	}
	return ""
}

// reLabelLine reconhece uma linha que abre um rótulo ("ENDEREÇO: ...").
var reLabelLine = regexp.MustCompile(`^\s*[A-Za-zÀ-ÿ][A-Za-zÀ-ÿ\s/º°().]{1,40}:`)

func looksLikeLabel(line string) bool {
	if strings.TrimSpace(line) == "" {
		return true // linha vazia encerra o rótulo tanto quanto outro rótulo
	}
	return reLabelLine.MatchString(line)
}

// sanitizeRecord filtra o valor da ficha CIOPS: número/protocolo curto. Uma
// frase inteira ali é sinal de que o rótulo significava outra coisa.
func sanitizeRecord(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || len([]rune(v)) > 60 || len(strings.Fields(v)) > 3 {
		return ""
	}
	if !strings.ContainsFunc(v, unicode.IsDigit) {
		return ""
	}
	return strings.ToUpper(v)
}

// ─── Tipo e meio ───────────────────────────────────────────────────────

func detectType(nature, full string) string {
	n := fold(nature)
	switch {
	case containsAny(n, "HOMICIDIO", "FEMINICIDIO", "LATROCINIO", "MORTE", "CVLI",
		"LESAO CORPORAL SEGUIDA DE MORTE"):
		return TypeHomicidio
	case containsAny(n, "APREENSAO", "APREENDID"):
		return TypeApreensao
	case containsAny(n, "PRISAO", "CAPTURA", "PRESO", "MANDADO"):
		return TypePrisao
	}
	// Sem NATUREZA utilizável, o próprio título do documento resolve o caso
	// mais comum ("RELATÓRIO DE CVLI").
	if containsAny(fold(full), "RELATORIO DE CVLI", "CVLI") {
		return TypeHomicidio
	}
	return ""
}

// meansPatterns mapeia expressões do relatório para o meio canônico. Ordem
// importa: o primeiro casamento vence.
var meansPatterns = []struct {
	means string
	terms []string
}{
	{MeansPAF, []string{"ARMA DE FOGO", "PAF", "PROJETIL", "DISPARO", "TIRO", "BALEAD",
		"FUZIL", "PISTOLA", "REVOLVER", "ESTOJO", "CAPSULA", "DEFLAGRAD"}},
	{MeansArmaBranca, []string{"ARMA BRANCA", "FACA", "FACAO", "PUNHAL", "CANIVETE",
		"PERFUROCORTANTE", "PERFURO CORTANTE", "ESFAQUEAD", "GOLPES DE FACA"}},
	{MeansAsfixia, []string{"ASFIXIA", "ESTRANGULAMENTO", "ENFORCAMENTO", "SUFOCAMENTO"}},
	{MeansContundente, []string{"CONTUNDENTE", "PAUADA", "PEDRADA", "MARTELO",
		"BARRA DE FERRO", "ESPANCAMENTO", "AGRESSAO FISICA"}},
}

// outrosPatterns são meios reconhecidos que não têm código próprio: viram
// "outros" com o detalhe preenchido.
var outrosPatterns = []struct{ term, detail string }{
	{"CARBONIZ", "CARBONIZAÇÃO"},
	{"INCINER", "INCINERAÇÃO"},
	{"ATROPELAMENTO", "ATROPELAMENTO"},
	{"AFOGAMENTO", "AFOGAMENTO"},
	{"ENVENENAMENTO", "ENVENENAMENTO"},
	{"EXPLOSIVO", "EXPLOSIVO"},
}

// detectMeans lê o meio na NATUREZA e, se ela não disser ("HOMICÍDIO
// DOLOSO"), no cenário — onde estão as lesões, os calibres e os estojos.
func detectMeans(nature, scenario string) (means, detail string) {
	for _, src := range []string{nature, scenario} {
		s := fold(src)
		if s == "" {
			continue
		}
		for _, p := range meansPatterns {
			if containsAny(s, p.terms...) {
				return p.means, ""
			}
		}
		for _, p := range outrosPatterns {
			if strings.Contains(s, p.term) {
				return MeansOutros, p.detail
			}
		}
	}
	return MeansUnknown, ""
}

// ─── Data e hora ───────────────────────────────────────────────────────

var (
	// 17/08/2026, 17-08-26, 17.08.2026
	reNumericDate = regexp.MustCompile(`\b(\d{1,2})\s*[/.\-]\s*(\d{1,2})\s*[/.\-]\s*(\d{2,4})\b`)
	// 17AGO2026, 17 AGO 26, 17 DE AGOSTO DE 2026
	reMilitaryDate = regexp.MustCompile(`\b(\d{1,2})\s*(?:DE\s+)?([A-Z]{3})[A-Z]*\.?\s*(?:DE\s+)?(\d{2,4})\b`)
	// 17h35, 17:35, 23h01min, 2h. Sem \b no fim: em "23h01min" o "m" cola
	// no minuto e a fronteira nunca fecha.
	reClock = regexp.MustCompile(`\b(\d{1,2})\s*(?:[hH:])\s*(\d{2})?`)
)

var ptMonths = map[string]int{
	"JAN": 1, "FEV": 2, "MAR": 3, "ABR": 4, "MAI": 5, "JUN": 6,
	"JUL": 7, "AGO": 8, "SET": 9, "OUT": 10, "NOV": 11, "DEZ": 12,
}

// parseReportDate aceita as grafias usadas nos relatórios e devolve
// YYYY-MM-DD. Data impossível (32/13) volta vazia.
func parseReportDate(s string) string {
	if s == "" {
		return ""
	}
	up := fold(s)
	if m := reNumericDate.FindStringSubmatch(up); m != nil {
		return buildDate(atoi(m[1]), atoi(m[2]), atoi(m[3]))
	}
	if m := reMilitaryDate.FindStringSubmatch(up); m != nil {
		if mon, ok := ptMonths[m[2]]; ok {
			return buildDate(atoi(m[1]), mon, atoi(m[3]))
		}
	}
	return ""
}

// buildDate valida o trio e normaliza o ano de dois dígitos (26 → 2026).
func buildDate(day, month, year int) string {
	if year < 100 {
		year += 2000
	}
	if year < 1900 || year > 2200 || month < 1 || month > 12 || day < 1 || day > 31 {
		return ""
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	// Rejeita 31/02 e afins: o Date normaliza para março em vez de falhar.
	if t.Day() != day || int(t.Month()) != month {
		return ""
	}
	return t.Format("2006-01-02")
}

// parseReportTime devolve HH:MM. "2h" vira "02:00"; "23h01min" vira "23:01".
func parseReportTime(s string) string {
	if s == "" {
		return ""
	}
	m := reClock.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	h := atoi(m[1])
	min := 0
	if m[2] != "" {
		min = atoi(m[2])
	}
	if h > 23 || min > 59 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", h, min)
}

// ─── Lugar ─────────────────────────────────────────────────────────────

// reStateSuffix remove o "-CE" / "/CE" / "(CE)" do fim do nome da cidade.
var reStateSuffix = regexp.MustCompile(`\s*[-/,(]\s*CE\s*\)?\s*$`)

// streetPrefixes identificam um logradouro — que a ocorrência não guarda.
var streetPrefixes = []string{"RUA ", "R. ", "AV ", "AV. ", "AVENIDA ", "TRAVESSA ",
	"TV. ", "ROD ", "ROD. ", "RODOVIA ", "ESTRADA ", "SITIO ", "SÍTIO ",
	"CE-", "BR-", "PRACA ", "PRAÇA ", "BECO ", "CONJ. "}

// genericPlaceParts são segmentos que existem no endereço mas não nomeiam o
// bairro. "ZONA RURAL, QUIXERÉ" tem como bairro a localidade anterior, não a
// zona; um número de porta também não é bairro.
var genericPlaceParts = []string{"ZONA RURAL", "ZONA URBANA", "AREA RURAL",
	"AREA URBANA", "CENTRO URBANO", "S/N", "SN", "PROXIMO", "PROXIMIDADES"}

// parsePlace extrai município e bairro do valor de ENDEREÇO. Só devolve
// município que exista na lista do Ceará — grafia livre fragmentaria a
// estatística territorial, que é o que o campo existe para sustentar.
func parsePlace(addr string) (city, neighborhood, warning string) {
	if strings.TrimSpace(addr) == "" {
		return "", "", ""
	}
	parts := splitAddr(addr)
	cityIdx := -1
	for i := len(parts) - 1; i >= 0; i-- {
		cand := reStateSuffix.ReplaceAllString(trimPunct(parts[i]), "")
		if m, ok := matchMunicipio(cand); ok {
			city, cityIdx = m, i
			break
		}
	}
	// Bairro: rótulo explícito primeiro; senão, o segmento útil mais próximo
	// do município, pulando número de porta, "zona rural" e logradouro.
	for i, p := range parts {
		if i == cityIdx {
			continue
		}
		if rest, ok := trimPrefixFold(p, "BAIRRO", "B.", "BR."); ok {
			neighborhood = upperTrimText(trimPunct(rest))
			break
		}
	}
	if neighborhood == "" && cityIdx > 0 {
		for i := cityIdx - 1; i >= 0; i-- {
			cand := trimPunct(parts[i])
			if usableAsNeighborhood(cand) {
				neighborhood = upperTrimText(cand)
				break
			}
		}
	}
	for _, p := range parts {
		if hasStreetPrefix(p) {
			warning = "Logradouro citado no endereço — a ocorrência guarda apenas município e bairro; o endereço completo ficou na descrição."
			break
		}
	}
	return city, neighborhood, warning
}

// usableAsNeighborhood recusa o que não nomeia um bairro: número de porta,
// designação genérica de zona e logradouro.
func usableAsNeighborhood(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || hasStreetPrefix(s) {
		return false
	}
	if !strings.ContainsFunc(s, unicode.IsLetter) {
		return false // "2597"
	}
	k := fold(s)
	for _, g := range genericPlaceParts {
		if k == g {
			return false
		}
	}
	return true
}

func splitAddr(addr string) []string {
	raw := strings.FieldsFunc(addr, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// matchMunicipio casa o nome com a lista do Ceará ignorando caixa e acento.
func matchMunicipio(s string) (string, bool) {
	k := fold(s)
	if k == "" {
		return "", false
	}
	for _, m := range ceMunicipios {
		if fold(m) == k {
			return m, true
		}
	}
	return "", false
}

// FindCityMention devolve o município do Ceará citado num texto livre, ou ""
// se nenhum aparecer. Usado para confrontar o endereço a que o link do Maps
// resolveu com o município que o relatório declarou — divergência aí costuma
// ser link colado do lugar errado, e o mapa do crime é agregado por
// município.
func FindCityMention(text string) string {
	// A pontuação gruda no nome ("Limoeiro do Norte, CE", "Cascavel/CE"), então
	// tudo que não é letra ou dígito vira separador antes da comparação.
	clean := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, string(foldRunes(text)))
	joined := " " + strings.Join(strings.Fields(clean), " ") + " "
	best := ""
	for _, m := range ceMunicipios {
		k := " " + fold(m) + " "
		if strings.Contains(joined, k) && len(m) > len(best) {
			best = m // nome mais longo vence ("SÃO LUÍS DO CURU" sobre "CURU")
		}
	}
	return best
}

func hasStreetPrefix(s string) bool {
	u := strings.ToUpper(strings.TrimSpace(s))
	for _, p := range streetPrefixes {
		if strings.HasPrefix(u, p) {
			return true
		}
	}
	return false
}

// trimPunct remove pontuação de borda ("QUIXERÉ." → "QUIXERÉ").
func trimPunct(s string) string {
	return strings.Trim(strings.TrimSpace(s), ".;:-–—\"'“”")
}

// trimPrefixFold remove um prefixo (comparado sem acento/caixa) e devolve o
// resto sem os separadores que costumam segui-lo. A comparação é feita em
// runas: "Mãe" e "MAE" têm o mesmo tamanho em runas, não em bytes.
//
// Exige separador (":", "-", "." ou espaço) depois do prefixo, senão "DN"
// casaria dentro de um nome como "DNIEL".
func trimPrefixFold(s string, prefixes ...string) (string, bool) {
	rs := []rune(s)
	fs := foldRunes(s)
	for _, p := range prefixes {
		fp := foldRunes(p)
		if len(fp) == 0 || len(fp) > len(rs) {
			continue
		}
		if string(fs[:len(fp)]) != string(fp) {
			continue
		}
		if len(rs) > len(fp) && !strings.ContainsRune(".:- ", rs[len(fp)]) {
			continue
		}
		rest := strings.TrimSpace(string(rs[len(fp):]))
		rest = strings.TrimLeft(rest, ".:- ")
		if rest != "" {
			return rest, true
		}
	}
	return "", false
}

// ─── Coordenadas ───────────────────────────────────────────────────────

var (
	reURL = regexp.MustCompile(`https?://[^\s<>"')]+`)
	// Formas ANCORADAS: o par vem preso a um marcador do Maps, então é a
	// coordenada do ponto, não um número qualquer da página.
	reCoordAnchored = []*regexp.Regexp{
		regexp.MustCompile(`[@](-?\d{1,3}\.\d{3,}),\s*(-?\d{1,3}\.\d{3,})`),
		regexp.MustCompile(`!3d(-?\d{1,3}\.\d{3,})!4d(-?\d{1,3}\.\d{3,})`),
		regexp.MustCompile(`[?&](?:q|ll|sll|center|daddr)=(-?\d{1,3}\.\d{3,}),\s*(-?\d{1,3}\.\d{3,})`),
	}
	// Par solto ("COORDENADAS: -3.85, -40.92"). Vale para texto digitado e
	// para a URL, NUNCA para o HTML da página: lá qualquer float vira
	// candidato e o ponto do crime sai parar na Groenlândia.
	reCoordLoose = regexp.MustCompile(`(-?\d{1,2}\.\d{4,})\s*,\s*(-?\d{1,3}\.\d{4,})\b`)
)

func firstURL(s string) string { return reURL.FindString(s) }

// plausibleCoord recusa o que não pode ser um ponto do acervo. O cadastro
// só aceita municípios do Ceará, então uma coordenada fora do Brasil é
// necessariamente lixo de parsing — melhor campo vazio que ponto errado no
// mapa do crime.
func plausibleCoord(lat, lng float64) bool {
	return lat >= -34.5 && lat <= 6.5 && lng >= -74.5 && lng <= -32.0
}

// coordsFromText procura o par lat/long em texto livre ou numa URL.
func coordsFromText(s string) (lat, lng float64, ok bool) {
	return matchCoords(s, append(append([]*regexp.Regexp{}, reCoordAnchored...), reCoordLoose))
}

// coordsFromPage é a versão para o HTML do Google Maps: só padrões
// ancorados.
func coordsFromPage(s string) (lat, lng float64, ok bool) {
	return matchCoords(s, reCoordAnchored)
}

// matchCoords devolve o primeiro par plausível. Percorre todas as
// ocorrências de cada padrão: um primeiro match implausível não pode
// enterrar o par correto que vem depois.
func matchCoords(s string, patterns []*regexp.Regexp) (lat, lng float64, ok bool) {
	for _, re := range patterns {
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			la, err1 := strconv.ParseFloat(m[1], 64)
			lo, err2 := strconv.ParseFloat(m[2], 64)
			if err1 != nil || err2 != nil || !plausibleCoord(la, lo) {
				continue
			}
			return la, lo, true
		}
	}
	return 0, 0, false
}

// ─── Pessoas ───────────────────────────────────────────────────────────

// rePlate cobre a placa antiga (ABC1234) e a Mercosul (ABC1D23). Só aceita
// as letras coladas aos números (com hífen, no máximo): admitir espaço fazia
// "por volta DAS 2H00" virar a placa DAS2H00.
var rePlate = regexp.MustCompile(`\b([A-Z]{3})-?(\d[A-Z0-9]\d{2})\b`)

// reParenStatus captura a situação declarada no fim do nome: "FULANO (ÓBITO)".
var reParenStatus = regexp.MustCompile(`\s*\(([^)]{2,30})\)\s*$`)

// statusWords traduz o que o relatório declara sobre a pessoa.
var statusWords = map[string]string{
	"OBITO": StatusDeceased, "MORTO": StatusDeceased, "MORTA": StatusDeceased,
	"FALECIDO": StatusDeceased, "FALECIDA": StatusDeceased, "VITIMA FATAL": StatusDeceased,
	"LESIONADO": StatusAlive, "LESIONADA": StatusAlive, "FERIDO": StatusAlive,
	"FERIDA": StatusAlive, "BALEADO": StatusAlive, "BALEADA": StatusAlive,
	"SOBREVIVENTE": StatusAlive, "SOCORRIDO": StatusAlive, "SOCORRIDA": StatusAlive,
}

// noteLabels são campos que o relatório traz sobre a pessoa e a ocorrência
// não guarda. Vão para Notes, que vira a descrição do dossiê no cadastro.
var noteLabels = []string{"ENDERECO", "END", "ANTECEDENTES", "NATURALIDADE",
	"PAI", "PROFISSAO", "RG", "OBSERVACAO", "OBS"}

// parsePeople lê a seção da vítima e, quando o autor está qualificado, a de
// envolvidos. Devolve também os avisos sobre papel que não pôde ser deduzido.
func parsePeople(secs []section, attempt bool) ([]ParsedPerson, []string) {
	var out []ParsedPerson
	var warns []string
	for _, s := range secs {
		var sectionRole string
		switch s.key {
		case secVitima:
			sectionRole = RoleVitima
		case secEnvolvido:
			sectionRole = RoleAcusado
		default:
			continue
		}
		for _, blk := range splitBlocks(s.lines) {
			p, ok := parsePersonBlock(blk)
			if !ok {
				continue
			}
			p.Role = sectionRole
			// Na seção de envolvidos o texto é predominantemente narrativo;
			// só aceita quem vier qualificado (mãe ou nascimento), senão
			// qualquer frase em caixa alta viraria um acusado.
			if sectionRole == RoleAcusado && p.MotherName == "" && p.DateOfBirth == "" && p.CPF == "" {
				continue
			}
			// VÍTIMA marca óbito no acervo inteiro. Sobrevivente e tentativa
			// não podem receber esse papel automaticamente.
			if sectionRole == RoleVitima {
				switch {
				case p.Status == StatusAlive:
					p.Role = ""
					warns = append(warns, fmt.Sprintf(
						"%s consta como sobrevivente no relatório — não foi marcada como VÍTIMA "+
							"(esse papel registra óbito no acervo). Escolha o papel ao vincular.", p.Name))
				case attempt:
					p.Role = ""
					warns = append(warns, fmt.Sprintf(
						"%s foi citada numa tentativa de homicídio — não foi marcada como VÍTIMA "+
							"(esse papel registra óbito no acervo). Escolha o papel ao vincular.", p.Name))
				}
			}
			out = append(out, p)
		}
	}
	return out, warns
}

// splitBlocks quebra as linhas da seção em blocos separados por linha vazia
// — o formato usado quando o relatório traz mais de uma vítima. Quando não
// há linha em branco, um novo nome (linha solta ou "Nome:") abre outro bloco.
func splitBlocks(lines []string) [][]string {
	var out [][]string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
			cur = nil
		}
	}
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			flush()
			continue
		}
		if startsPerson(ln) && blockHasPerson(cur) {
			flush()
		}
		cur = append(cur, ln)
	}
	flush()
	return out
}

// startsPerson diz se a linha abre a qualificação de alguém.
func startsPerson(ln string) bool {
	if _, ok := trimPrefixFold(ln, "NOME", "NOME COMPLETO"); ok {
		return true
	}
	return looksLikeName(stripStatus(ln))
}

// blockHasPerson evita cortar o bloco antes de ele ter um nome.
func blockHasPerson(lines []string) bool {
	for _, ln := range lines {
		if startsPerson(ln) {
			return true
		}
	}
	return false
}

func parsePersonBlock(lines []string) (ParsedPerson, bool) {
	var p ParsedPerson
	var notes []string
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if v, ok := trimPrefixFold(ln, "MAE", "NOME DA MAE", "FILIACAO"); ok {
			p.MotherName = upperTrimText(trimPunct(v))
			continue
		}
		if v, ok := trimPrefixFold(ln, "DN", "DATA DE NASCIMENTO", "DATA NASCIMENTO",
			"DATA DE NASC", "NASCIMENTO", "NASC"); ok {
			p.DateOfBirth = parseReportDate(v)
			continue
		}
		if v, ok := trimPrefixFold(ln, "CPF"); ok {
			p.CPF = digitsOf(v)
			continue
		}
		if v, ok := trimPrefixFold(ln, "VULGO", "ALCUNHA", "APELIDO", "CONHECIDO COMO"); ok {
			p.Alias = upperTrimText(trimPunct(v))
			continue
		}
		if v, ok := trimPrefixFold(ln, noteLabels...); ok {
			notes = append(notes, upperTrimText(ln[:strings.IndexByte(ln, ':')])+": "+upperTrimText(v))
			continue
		}
		if p.Name != "" {
			continue
		}
		if v, ok := trimPrefixFold(ln, "NOME", "NOME COMPLETO"); ok {
			p.Name, p.Status = nameAndStatus(v)
			continue
		}
		if cand := stripStatus(ln); looksLikeName(cand) {
			p.Name, p.Status = nameAndStatus(ln)
		}
	}
	p.Notes = strings.Join(notes, "\n")
	return p, p.Name != ""
}

// nameAndStatus separa "FULANO DE TAL (ÓBITO)" em nome e situação.
func nameAndStatus(s string) (name, status string) {
	s = strings.TrimSpace(s)
	if m := reParenStatus.FindStringSubmatch(s); m != nil {
		if st, ok := statusWords[fold(m[1])]; ok {
			status = st
		}
		s = strings.TrimSpace(s[:len(s)-len(m[0])])
	}
	return upperTrimText(strings.TrimRight(s, ".;,")), status
}

// stripStatus remove o parêntese de situação para a linha poder ser avaliada
// como nome.
func stripStatus(s string) string {
	return strings.TrimSpace(reParenStatus.ReplaceAllString(strings.TrimSpace(s), ""))
}

// looksLikeName aceita a linha solta que abre o bloco da vítima. Recusa
// rótulos (contêm ":"), frases longas e linhas com dígitos ou pontuação de
// prosa — o nome vem sozinho na linha nesse formato de relatório.
func looksLikeName(s string) bool {
	s = strings.TrimSpace(strings.TrimRight(s, ".;,"))
	if s == "" || len(s) > 80 || strings.ContainsAny(s, ":;0123456789()\"") {
		return false
	}
	words := strings.Fields(s)
	if len(words) < 2 || len(words) > 8 {
		return false
	}
	for _, w := range words {
		for _, r := range w {
			if !isLetter(r) && r != '\'' && r != '-' && r != '.' {
				return false
			}
		}
	}
	return true
}

// findPlates devolve as placas citadas, sem repetir.
func findPlates(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range rePlate.FindAllStringSubmatch(strings.ToUpper(text), -1) {
		plate := m[1] + m[2]
		if !seen[plate] {
			seen[plate] = true
			out = append(out, plate)
		}
	}
	return out
}

// ─── Preparação do texto ───────────────────────────────────────────────

var (
	// Cabeçalho de mensagem exportada do WhatsApp:
	// "[09:55, 18/08/2026] Sd Alcantara SAI 15° BPM: " ou
	// "18/08/2026 09:55 - +55 88 9378-7827: ".
	reChatPrefix = regexp.MustCompile(
		`^\s*(?:\[\s*\d{1,2}:\d{2}(?::\d{2})?\s*(?:,\s*\d{1,2}/\d{1,2}/\d{2,4}\s*)?\]` +
			`|\d{1,2}/\d{1,2}/\d{2,4}[, ]+\d{1,2}:\d{2}(?::\d{2})?\s*-)\s*[^:]{0,60}:\s*`)
	// Bullets e travessões que abrem item de lista.
	reBullet = regexp.MustCompile(`^[\s*•·◦▪·\-–—]+`)
)

// splitLines normaliza o texto para leitura: tira o cabeçalho de mensagem, a
// marcação de negrito do WhatsApp, os bullets e os invisíveis que o
// aplicativo intercala. Preserva as linhas vazias — elas separam os blocos
// de vítima.
func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.NewReplacer(
		"\u200e", "", "\u200f", "", "\ufeff", "", "\u2060", "",
		"\u00ad", "", "\u200b", "", "\u00a0", " ",
	).Replace(text)
	raw := strings.Split(text, "\n")
	out := make([]string, 0, len(raw))
	for _, ln := range raw {
		ln = reChatPrefix.ReplaceAllString(ln, "")
		// O negrito do WhatsApp cerca o rótulo inteiro (*NATUREZA:*) e
		// atrapalha tanto o corte do rótulo quanto o teste de nome.
		ln = strings.ReplaceAll(ln, "*", "")
		ln = reBullet.ReplaceAllString(ln, "")
		out = append(out, strings.TrimRight(ln, " \t"))
	}
	return out
}

// ─── Utilitários ───────────────────────────────────────────────────────

// fold devolve a forma comparável: MAIÚSCULAS, sem acento, espaços colapsados.
func fold(s string) string {
	return strings.Join(strings.Fields(string(foldRunes(s))), " ")
}

// foldRunes é o fold que PRESERVA o tamanho em runas — cada runa vira
// exatamente uma runa. É o que permite cortar um prefixo por posição sem
// desalinhar em texto acentuado ("Mãe:" tem 4 runas e 5 bytes).
func foldRunes(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		out = append(out, foldRune(r))
	}
	return out
}

func foldRune(r rune) rune {
	r = unicode.ToUpper(r)
	if f, ok := diacriticRunes[r]; ok {
		return f
	}
	return r
}

// diacriticRunes cobre os diacríticos do português (o parser lê texto em
// português; nome estrangeiro não muda o casamento de rótulo).
var diacriticRunes = map[rune]rune{
	'Á': 'A', 'À': 'A', 'Â': 'A', 'Ã': 'A', 'Ä': 'A',
	'É': 'E', 'È': 'E', 'Ê': 'E', 'Ë': 'E',
	'Í': 'I', 'Ì': 'I', 'Î': 'I', 'Ï': 'I',
	'Ó': 'O', 'Ò': 'O', 'Ô': 'O', 'Õ': 'O', 'Ö': 'O',
	'Ú': 'U', 'Ù': 'U', 'Û': 'U', 'Ü': 'U',
	'Ç': 'C', 'Ñ': 'N', 'Ý': 'Y',
}

// upperTrimText sobe para MAIÚSCULAS preservando acento — a forma com que os
// textos livres são persistidos.
func upperTrimText(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func digitsOf(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func isLetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r >= 0x00C0
}
