// Package opsreport lê o Relatório Diário de Ocorrências do CPRAIO (PDF
// gerado pelo sistema do comando, uma ocorrência por página) e o grava no
// acervo — só as ocorrências do batalhão da agência.
//
// A leitura é determinística e em duas etapas:
//
//  1. Extract: pdftotext -bbox devolve cada palavra com sua posição.
//  2. Parse: agrupa as palavras em linhas, separa as ocorrências e lê os
//     campos. Campos rotulados ("Base:", "Equipe:") são lidos sobre o texto
//     corrido do bloco, o que tolera rótulo ou valor quebrados em duas linhas;
//     as tabelas (acusados, armas, drogas, …) são lidas pela posição x, porque
//     célula vazia só aparece como distância entre as vizinhas.
package opsreport

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Papéis de pessoa na ocorrência — os mesmos do cadastro de ocorrências.
const (
	RoleAcusado    = "ACUSADO"
	RoleVitima     = "VÍTIMA"
	RoleTestemunha = "TESTEMUNHA"
)

// Report é o relatório inteiro, de todas as unidades.
type Report struct {
	// ReportDate é a data que o cabeçalho das páginas traz (a do serviço
	// relatado, não a da publicação). Zero se nenhuma página a trouxer.
	ReportDate  time.Time
	Occurrences []Occurrence
	Warnings    []string
}

// Occurrence é uma ocorrência como o relatório a descreve.
type Occurrence struct {
	// Page é a página (1-based) onde a ocorrência começa — referência para
	// conferir no PDF.
	Page int
	// HeaderUnit é o batalhão do cabeçalho da página ("2º BPRAIO").
	HeaderUnit string

	Natures []string
	// BaseRaw é a linha "Base:" como veio; BaseCity/CIA/PEL/BPM são as
	// partes dela ("CAUCAIA-1ªCIA/2ºBPRAIO" → CAUCAIA, 1ª CIA, "", 2º BPRAIO).
	BaseRaw  string
	BaseCity string
	CIA      string
	PEL      string
	BPM      string

	OccurredOn time.Time
	StartTime  string // "HH:MM" ou ""
	EndTime    string
	Teams      string
	CIOPS      string

	PlaceAddress         string
	PlaceNeighborhood    string
	PlaceCity            string
	ApproachAddress      string
	ApproachNeighborhood string
	ApproachCity         string

	PoliceStation   string
	Delegate        string
	ProcedureType   string
	ProcedureNumber string

	SeizedObjects string
	Narrative     string

	People   []Person
	Weapons  []Weapon
	Drugs    []Drug
	Vehicles []Vehicle
	Officers []Officer

	// IntelMatched são os termos de inteligência encontrados no histórico ou
	// na equipe (preenchido por quem importa, com os termos configurados).
	IntelMatched []string

	// Sipom é a ocorrência nos códigos do SIPOM (preenchido por quem importa,
	// com o catálogo do schema sipom).
	Sipom SipomFields
	// peopleLinked[i]: People[i] tem dossiê vinculado (só no recálculo do
	// gravado; na importação o vínculo ainda não existe).
	peopleLinked []bool

	Warnings []string
}

// SipomFields são os campos que o envio ao SIPOM exige, já como ids do
// catálogo deles. A composição fica em Officer (Sipom*).
type SipomFields struct {
	NaturezaID *int
	Logradouro string
	Numeral    string
	CidadeID   *int
	BairroID   *int
	AreaID     *int
	OPMID      *int
	// Pending são os códigos de pendência (ver internal/sipom).
	Pending []string
	// Manual são os campos que o analista fixou; o recálculo não os toca.
	Manual []string
}

// IntelTexts são os campos onde se procura a participação da inteligência:
// o histórico, onde a tropa narra de onde veio a informação, e a equipe,
// onde a SAI aparece quando compõe a ação.
func (o *Occurrence) IntelTexts() []string {
	return []string{o.Narrative, o.Teams}
}

// Person é um acusado, vítima ou testemunha.
type Person struct {
	Role       string
	Name       string
	MotherName string
	Age        *int
	Address    string
	// Note é o que o relatório anota entre parênteses junto ao nome
	// ("ÓBITO"). Fica registrado, mas não marca nada no acervo.
	Note string
}

// Weapon é uma arma apreendida.
type Weapon struct {
	Kind, Model, Brand, Caliber, Serial string
}

// Drug é uma droga apreendida. Grams/Packages nil = não informado.
type Drug struct {
	Description string
	Grams       *float64
	Packages    *int
}

// Vehicle é um veículo apreendido.
type Vehicle struct {
	Kind, Brand, Model, Plate, Color string
}

// Officer é um policial da composição ("30079116-2SGT 22410 SOARES").
type Officer struct {
	Registration string // matrícula
	Rank         string // posto/graduação
	Number       string // numeral
	WarName      string // nome de guerra
	Raw          string

	// Tradução para a composição do SIPOM (tipo de policiamento e função).
	SipomEquipe             string
	SipomPoliciamentoTipoID *int
	SipomFuncaoID           *int
}

// ─────────────────────────── Unidade ────────────────────────────

var unitRE = regexp.MustCompile(`^(\d+)\s*[ºª°]?\s*([A-Z]+)$`)

// canonUnit escreve a unidade num formato só ("1ªCIA" → "1ª CIA",
// "2ºBPRAIO" → "2º BPRAIO"), para agrupar e filtrar sem depender da grafia
// de quem lançou.
func canonUnit(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	m := unitRE.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	n, _ := strconv.Atoi(m[1])
	if m[2] == "CIA" {
		return fmt.Sprintf("%dª %s", n, m[2])
	}
	return fmt.Sprintf("%dº %s", n, m[2])
}

// UnitKey reduz a unidade à forma de comparação: "2º BPRAIO", "2ºBPRAIO" e
// "2° BPRAIO" viram todos "2BPRAIO".
func UnitKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) && r != 'º' && r != 'ª' || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitBase separa "JUAZEIRO DO NORTE-1ªCIA/5ºBPRAIO" em cidade e unidades.
// A hierarquia vem da menor para a maior: [PEL/]CIA/BPM.
func splitBase(raw string) (city, cia, pel, bpm string) {
	raw = strings.TrimSpace(raw)
	head, rest, hasSlash := strings.Cut(raw, "/")
	units := []string{}
	if i := strings.LastIndex(head, "-"); i >= 0 {
		city = strings.TrimSpace(head[:i])
		units = append(units, head[i+1:])
	} else if hasSlash {
		units = append(units, head)
	} else {
		return strings.ToUpper(head), "", "", ""
	}
	if hasSlash {
		units = append(units, strings.Split(rest, "/")...)
	}
	for i, u := range units {
		u = canonUnit(u)
		switch {
		case i == len(units)-1:
			bpm = u
		case strings.HasSuffix(u, "CIA"):
			cia = u
		case strings.HasSuffix(u, "PEL"):
			pel = u
		}
	}
	return strings.ToUpper(city), cia, pel, bpm
}

// Unit devolve o batalhão da ocorrência: o da linha Base, que é o dado da
// própria ocorrência, e na falta dele o do cabeçalho da página.
func (o *Occurrence) Unit() string {
	if o.BPM != "" {
		return o.BPM
	}
	return o.HeaderUnit
}

// ─────────────────────────── Linhas ────────────────────────────

type line struct {
	page  int
	y     float64 // centro vertical
	h     float64 // altura
	words []Word
	text  string
}

// buildLines agrupa as palavras de cada página em linhas (mesmo centro
// vertical, com tolerância para rótulo e valor em fontes diferentes) e ordena
// cada linha por x. O poppler nem sempre emite a linha inteira em sequência.
func buildLines(pages []Page) []line {
	var out []line
	for pi, p := range pages {
		ws := append([]Word(nil), p.Words...)
		sort.SliceStable(ws, func(i, j int) bool {
			return (ws[i].Y0+ws[i].Y1)/2 < (ws[j].Y0+ws[j].Y1)/2
		})
		var cur *line
		var pageLines []line
		for _, w := range ws {
			cy := (w.Y0 + w.Y1) / 2
			if cur == nil || math.Abs(cy-cur.y) > 3.5 {
				pageLines = append(pageLines, line{page: pi + 1, y: cy, h: w.Y1 - w.Y0})
				cur = &pageLines[len(pageLines)-1]
			}
			cur.words = append(cur.words, w)
		}
		for i := range pageLines {
			l := &pageLines[i]
			sort.SliceStable(l.words, func(a, b int) bool { return l.words[a].X0 < l.words[b].X0 })
			parts := make([]string, len(l.words))
			for k, w := range l.words {
				parts[k] = w.Text
			}
			l.text = strings.Join(parts, " ")
		}
		out = append(out, pageLines...)
	}
	return out
}

// fold tira acento e caixa: base de comparação de rótulos e títulos. Runa
// por runa, então o tamanho em runas se mantém.
func fold(s string) string {
	var b strings.Builder
	for _, r := range s {
		r = unicode.ToLower(r)
		if f, ok := diacriticRunes[r]; ok {
			r = f
		}
		b.WriteRune(r)
	}
	return b.String()
}

// diacriticRunes cobre os diacríticos do português.
var diacriticRunes = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n', 'ý': 'y',
}

// alnum reduz a letras e dígitos sem acento ("Qtd(gramas)" → "qtdgramas",
// "Nº Série" → "nserie").
func alnum(s string) string {
	var b strings.Builder
	for _, r := range fold(s) {
		if r != 'º' && r != 'ª' && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hasPrefixFold(text, prefix string) bool {
	return strings.HasPrefix(fold(text), fold(prefix))
}

// afterPrefix devolve o que segue o rótulo (já conferido por hasPrefixFold).
// O corte é por runa: fold preserva a contagem, bytes não.
func afterPrefix(text, prefix string) string {
	r := []rune(text)
	n := len([]rune(prefix))
	if n > len(r) {
		return ""
	}
	return strings.TrimSpace(string(r[n:]))
}

// ─────────────────────────── Parse ────────────────────────────

var headerDateRE = regexp.MustCompile(`^Data:\s*(\d{2}/\d{2}/\d{4})$`)

// isNoise reconhece o que se repete no topo de toda página com cabeçalho e o
// fecho do documento.
func isNoise(t string) bool {
	switch {
	case hasPrefixFold(t, "Comando de Policiamento"),
		t == "log_rai",
		hasPrefixFold(t, "RELATÓRIO DIÁRIO DE OCORRÊNCIAS"),
		hasPrefixFold(t, "Assinatura Oficial"),
		hasPrefixFold(t, "Fiscal responsável"),
		strings.Trim(t, "_ ") == "":
		return true
	}
	return false
}

// Parse lê o relatório a partir das palavras extraídas do PDF.
func Parse(pages []Page) *Report {
	rep := &Report{}
	lines := buildLines(pages)

	dateVotes := map[string]int{}
	var headerUnit string
	var cur []line
	var curUnit string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		o := parseOccurrence(cur)
		o.HeaderUnit = curUnit
		rep.Occurrences = append(rep.Occurrences, o)
		cur = nil
	}

	// Numa página com cabeçalho, tudo antes do primeiro "Natureza:" é
	// cabeçalho (comando, título, batalhão, data). Página sem cabeçalho é
	// continuação da ocorrência anterior.
	inHeader := false
	for i, l := range lines {
		if i == 0 || l.page != lines[i-1].page {
			inHeader = false
			for _, x := range lines[i:] {
				if x.page != l.page {
					break
				}
				if hasPrefixFold(x.text, "RELATÓRIO DIÁRIO DE OCORRÊNCIAS") {
					inHeader = true
					break
				}
			}
		}
		if hasPrefixFold(l.text, "Natureza:") {
			flush()
			inHeader = false
			curUnit = headerUnit
			cur = append(cur, l)
			continue
		}
		if inHeader {
			if m := headerDateRE.FindStringSubmatch(l.text); m != nil {
				dateVotes[m[1]]++
			} else if !isNoise(l.text) && unitRE.MatchString(strings.ToUpper(l.text)) {
				headerUnit = canonUnit(l.text)
			}
			continue
		}
		if isNoise(l.text) {
			continue
		}
		if cur == nil {
			rep.Warnings = append(rep.Warnings,
				fmt.Sprintf("Página %d: texto fora de qualquer ocorrência ignorado (%q).", l.page, clip(l.text, 60)))
			continue
		}
		cur = append(cur, l)
	}
	flush()

	best, bestN := "", 0
	for d, n := range dateVotes {
		if n > bestN || n == bestN && d > best {
			best, bestN = d, n
		}
	}
	if best != "" {
		rep.ReportDate, _ = time.Parse("02/01/2006", best)
	}
	if len(dateVotes) > 1 {
		rep.Warnings = append(rep.Warnings,
			fmt.Sprintf("O cabeçalho traz mais de uma data; considerada %s.", best))
	}
	return rep
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Rótulos do bloco de identificação, na ordem em que o relatório os imprime.
var headLabels = []string{
	"Natureza:", "Base:", "Data:", "Hora Inicial:", "Hora Final:", "Equipe:",
	"Ficha Ciops:",
	"Local Ocorrência:", "Bairro Ocorrência:", "Cidade Ocorrência:",
	"Local Abordagem:", "Bairro Abordagem:", "Cidade Abordagem:",
}

var procLabels = []string{"Delegacia:", "Delegado(a):", "Tipo de Proced.:", "Nº do Proced.:"}

// labelRE casa o rótulo com qualquer espaço (inclusive quebra de linha já
// convertida em espaço) entre as palavras.
func labelRE(label string) *regexp.Regexp {
	parts := strings.Fields(label)
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile(`(?i)` + strings.Join(parts, `\s+`))
}

var (
	headLabelREs = compileLabels(headLabels)
	procLabelREs = compileLabels(procLabels)
)

func compileLabels(ls []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(ls))
	for i, l := range ls {
		out[i] = labelRE(l)
	}
	return out
}

// readLabels lê "Rótulo: valor Rótulo: valor…" em texto corrido. Os rótulos
// são procurados em ordem, cada um depois do anterior; o valor vai até o
// próximo rótulo encontrado. Rótulo ausente fica sem valor, sem desalinhar os
// demais.
func readLabels(text string, labels []string, res []*regexp.Regexp) map[string]string {
	type hit struct {
		label      string
		start, end int
	}
	var hits []hit
	pos := 0
	for i, re := range res {
		loc := re.FindStringIndex(text[pos:])
		if loc == nil {
			continue
		}
		hits = append(hits, hit{labels[i], pos + loc[0], pos + loc[1]})
		pos += loc[1]
	}
	out := map[string]string{}
	for i, h := range hits {
		end := len(text)
		if i+1 < len(hits) {
			end = hits[i+1].start
		}
		out[h.label] = strings.Join(strings.Fields(text[h.end:end]), " ")
	}
	return out
}

func joinText(ls []line) string {
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = l.text
	}
	return strings.Join(parts, " ")
}

// paragraphs junta linhas de texto corrido, abrindo parágrafo onde o
// espaçamento vertical denuncia uma linha em branco.
func paragraphs(ls []line) string {
	var b strings.Builder
	for i, l := range ls {
		if i > 0 {
			prev := ls[i-1]
			if prev.page == l.page && l.y-prev.y > 1.8*math.Max(prev.h, 8) {
				b.WriteString("\n\n")
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString(l.text)
	}
	return strings.TrimSpace(b.String())
}

var (
	timeRE    = regexp.MustCompile(`^(\d{1,2}):(\d{2})`)
	listNumRE = regexp.MustCompile(`^\d+\.\s`)
	// Praça: "30079116-2SGT 22410 SOARES" (matrícula-graduação numeral nome).
	// A graduação às vezes vem com o ordinal separado ("3º SGT").
	enlistedRE = regexp.MustCompile(`^(\S+?)-(\d+\s*[ºª°]?\s+[A-Za-z]+|\S+)\s+(\d+)\s+(.+)$`)
	// Oficial/subtenente não tem numeral: "84395986-1º TEN QOPM JOELLYSSON".
	officerRE = regexp.MustCompile(`^(\S+?)-(.+)$`)
	ordinalRE = regexp.MustCompile(`^\d+[ºª°]?$`)
)

// rankWords são as palavras que compõem posto de oficial/subtenente e quadro.
var rankWords = map[string]bool{
	"ST": true, "SUBTEN": true, "TEN": true, "CAP": true, "MAJ": true,
	"TC": true, "TEN-CEL": true, "CEL": true, "ASP": true, "AL": true,
	"PM": true, "QOPM": true, "QOAPM": true, "QOEPM": true, "QOSPM": true,
}

func isRankWord(w string) bool {
	w = strings.ToUpper(w)
	if rankWords[w] || ordinalRE.MatchString(w) {
		return true
	}
	// "2ºTEN", "1°TEN": ordinal colado ao posto.
	if i := strings.IndexAny(w, "º°ª"); i > 0 {
		_, size := utf8.DecodeRuneInString(w[i:])
		return rankWords[w[i+size:]]
	}
	return false
}

// parseOfficer lê "matrícula-graduação numeral NOME" (praças) ou
// "matrícula-POSTO [QUADRO] NOME" (oficiais e subtenentes, sem numeral).
// WarName vazio = linha fora do padrão.
func parseOfficer(raw string) Officer {
	off := Officer{Raw: raw}
	if m := enlistedRE.FindStringSubmatch(raw); m != nil {
		// "3º SGT" → "3ºSGT", a forma das demais linhas.
		off.Registration, off.Number = m[1], m[3]
		off.Rank = strings.ToUpper(strings.Join(strings.Fields(m[2]), ""))
		off.WarName = strings.ToUpper(strings.TrimSpace(m[4]))
		return off
	}
	m := officerRE.FindStringSubmatch(raw)
	if m == nil {
		return off
	}
	words := strings.Fields(m[2])
	n := 0
	for n < len(words)-1 && isRankWord(words[n]) {
		n++
	}
	if n == 0 {
		return off
	}
	off.Registration = m[1]
	off.Rank = strings.ToUpper(strings.Join(words[:n], " "))
	off.WarName = strings.ToUpper(strings.Join(words[n:], " "))
	return off
}

func parseTime(s string) string {
	m := timeRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return ""
	}
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	if h > 23 || mi > 59 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", h, mi)
}

// parseOccurrence lê uma ocorrência. As partes vêm sempre na mesma ordem:
// identificação, tabelas, procedimento, objetos, histórico e composição.
func parseOccurrence(ls []line) Occurrence {
	o := Occurrence{Page: ls[0].page}

	const (
		secHead = iota
		secTables
		secProc
		secObjects
		secNarrative
		secOfficers
	)
	sec := secHead
	var head, tables, proc, objects, narrative, officers []line
	for _, l := range ls {
		switch {
		case sec < secProc && hasPrefixFold(l.text, "Delegacia:"):
			sec = secProc
		case sec < secObjects && hasPrefixFold(l.text, "Objetos Apreendidos:"):
			sec = secObjects
			if rest := afterPrefix(l.text, "Objetos Apreendidos:"); rest != "" {
				objects = append(objects, line{page: l.page, y: l.y, h: l.h, text: rest})
			}
			continue
		case sec < secNarrative && hasPrefixFold(l.text, "Histórico:"):
			sec = secNarrative
			if rest := afterPrefix(l.text, "Histórico:"); rest != "" {
				narrative = append(narrative, line{page: l.page, y: l.y, h: l.h, text: rest})
			}
			continue
		case sec < secOfficers && hasPrefixFold(l.text, "Composição:"):
			sec = secOfficers
			continue
		case sec == secHead && tableKind(l.text) != "":
			sec = secTables
		}
		switch sec {
		case secHead:
			head = append(head, l)
		case secTables:
			tables = append(tables, l)
		case secProc:
			proc = append(proc, l)
		case secObjects:
			objects = append(objects, l)
		case secNarrative:
			narrative = append(narrative, l)
		case secOfficers:
			officers = append(officers, l)
		}
	}

	h := readLabels(joinText(head), headLabels, headLabelREs)
	for _, n := range strings.Split(h["Natureza:"], ";") {
		if n = strings.TrimSpace(n); n != "" {
			o.Natures = append(o.Natures, strings.ToUpper(n))
		}
	}
	o.BaseRaw = h["Base:"]
	o.BaseCity, o.CIA, o.PEL, o.BPM = splitBase(o.BaseRaw)
	if d, err := time.Parse("02/01/2006", h["Data:"]); err == nil {
		o.OccurredOn = d
	} else {
		o.Warnings = append(o.Warnings, fmt.Sprintf("Data da ocorrência ilegível (%q).", h["Data:"]))
	}
	o.StartTime = parseTime(h["Hora Inicial:"])
	o.EndTime = parseTime(h["Hora Final:"])
	o.Teams = h["Equipe:"]
	o.CIOPS = strings.ToUpper(strings.ReplaceAll(h["Ficha Ciops:"], " ", ""))
	o.PlaceAddress = h["Local Ocorrência:"]
	o.PlaceNeighborhood = strings.ToUpper(h["Bairro Ocorrência:"])
	o.PlaceCity = strings.ToUpper(h["Cidade Ocorrência:"])
	o.ApproachAddress = h["Local Abordagem:"]
	o.ApproachNeighborhood = strings.ToUpper(h["Bairro Abordagem:"])
	o.ApproachCity = strings.ToUpper(h["Cidade Abordagem:"])
	if len(o.Natures) == 0 {
		o.Warnings = append(o.Warnings, "Natureza não informada.")
	}
	if o.BaseRaw == "" {
		o.Warnings = append(o.Warnings, "Linha \"Base\" ausente — unidade tirada do cabeçalho da página.")
	}
	if o.CIOPS == "" {
		o.Warnings = append(o.Warnings, "Ficha CIOPS ausente — a ocorrência não tem como ser deduplicada.")
	}

	p := readLabels(joinText(proc), procLabels, procLabelREs)
	o.PoliceStation = p["Delegacia:"]
	o.Delegate = p["Delegado(a):"]
	o.ProcedureType = strings.ToUpper(p["Tipo de Proced.:"])
	o.ProcedureNumber = p["Nº do Proced.:"]

	o.SeizedObjects = strings.Join(strings.Fields(joinText(objects)), " ")
	o.Narrative = paragraphs(narrative)

	for _, l := range officers {
		// Só a primeira célula: o fecho do documento ("Fiscal responsável…")
		// pode cair na altura da última linha da composição.
		cs := cells(l)
		if len(cs) == 0 || !listNumRE.MatchString(cs[0].text) {
			continue
		}
		off := parseOfficer(strings.TrimSpace(listNumRE.ReplaceAllString(cs[0].text, "")))
		if off.WarName == "" {
			o.Warnings = append(o.Warnings, fmt.Sprintf("Composição fora do padrão: %q.", off.Raw))
		}
		o.Officers = append(o.Officers, off)
	}

	parseTables(&o, tables)
	return o
}

// ─────────────────────────── Tabelas ────────────────────────────

// tableKind identifica o título de tabela. As tabelas conhecidas são as que
// o relatório imprime quando há o que registrar.
func tableKind(text string) string {
	switch k := alnum(text); {
	case k == "acusados":
		return "acusados"
	case k == "vitimas":
		return "vitimas"
	case k == "testemunhas":
		return "testemunhas"
	case strings.HasPrefix(k, "armasapreendidas"):
		return "armas"
	case strings.HasPrefix(k, "drogasapreendidas"):
		return "drogas"
	case strings.HasPrefix(k, "veiculosapreendidos"):
		return "veiculos"
	}
	return ""
}

type cell struct {
	x0, x1 float64
	text   string
}

func (c cell) center() float64 { return (c.x0 + c.x1) / 2 }

// cells agrupa as palavras da linha em células: dentro da célula as palavras
// distam um espaço (~2pt); entre colunas, bem mais.
func cells(l line) []cell {
	var out []cell
	for _, w := range l.words {
		if n := len(out); n > 0 && w.X0-out[n-1].x1 < 7 {
			out[n-1].x1 = w.X1
			out[n-1].text += " " + w.Text
			continue
		}
		out = append(out, cell{x0: w.X0, x1: w.X1, text: w.Text})
	}
	return out
}

type table struct {
	kind   string
	title  string
	header []cell // células do cabeçalho; text = chave alnum ("placachassi")
	rows   []map[string]string
}

// looksLikeHeader: linha de cabeçalho de colunas — três ou mais células, todas
// iniciadas por maiúscula seguida de minúscula (Nome, Mãe, Qtd(gramas)) ou "Nº".
func looksLikeHeader(l line) bool {
	cs := cells(l)
	if len(cs) < 3 {
		return false
	}
	for _, c := range cs {
		r := []rune(c.text)
		if len(r) < 2 || !unicode.IsUpper(r[0]) || !(unicode.IsLower(r[1]) || r[1] == 'º') {
			return false
		}
	}
	return true
}

func parseTables(o *Occurrence, ls []line) {
	var tabs []*table
	var cur *table
	for i, l := range ls {
		kind := tableKind(l.text)
		// Título que o parser não conhece: o que vem logo abaixo é cabeçalho
		// de colunas. Os dados dela não se perdem — vão para os avisos.
		if kind == "" && (cur == nil || cur.header != nil) && len(cells(l)) == 1 &&
			i+1 < len(ls) && looksLikeHeader(ls[i+1]) {
			kind = "desconhecida"
		}
		if kind != "" {
			cur = &table{kind: kind, title: l.text}
			tabs = append(tabs, cur)
			continue
		}
		if cur == nil {
			o.Warnings = append(o.Warnings, fmt.Sprintf("Linha não reconhecida: %q.", clip(l.text, 80)))
			continue
		}
		if cur.header == nil {
			cur.header = cells(l)
			for k := range cur.header {
				cur.header[k].text = alnum(cur.header[k].text)
			}
			continue
		}
		row := map[string]string{}
		for _, c := range cells(l) {
			key := nearestColumn(cur.header, c)
			row[key] = strings.TrimSpace(row[key] + " " + c.text)
		}
		// Sem a primeira coluna, é a célula longa da linha de cima que quebrou.
		if n := len(cur.rows); n > 0 && row[cur.header[0].text] == "" {
			for k, v := range row {
				cur.rows[n-1][k] = strings.TrimSpace(cur.rows[n-1][k] + " " + v)
			}
			continue
		}
		cur.rows = append(cur.rows, row)
	}

	for _, t := range tabs {
		switch t.kind {
		case "acusados", "vitimas", "testemunhas":
			role := map[string]string{"acusados": RoleAcusado, "vitimas": RoleVitima, "testemunhas": RoleTestemunha}[t.kind]
			for _, r := range t.rows {
				name, note := splitNameNote(strings.ToUpper(r["nome"]))
				p := Person{
					Role:       role,
					Name:       name,
					Note:       note,
					MotherName: strings.ToUpper(r["mae"]),
					Address:    strings.ToUpper(r["endereco"]),
				}
				if n, err := strconv.Atoi(strings.TrimSpace(r["idade"])); err == nil && n > 0 && n < 130 {
					p.Age = &n
				}
				if p.Name == "" {
					o.Warnings = append(o.Warnings, fmt.Sprintf("%s sem nome ignorado.", t.title))
					continue
				}
				o.People = append(o.People, p)
			}
		case "armas":
			for _, r := range t.rows {
				o.Weapons = append(o.Weapons, Weapon{
					Kind: strings.ToUpper(r["tipo"]), Model: strings.ToUpper(r["modelo"]),
					Brand: strings.ToUpper(r["marca"]), Caliber: strings.ToUpper(r["calibre"]),
					Serial: strings.ToUpper(r["nserie"]),
				})
			}
		case "veiculos":
			for _, r := range t.rows {
				o.Vehicles = append(o.Vehicles, Vehicle{
					Kind: strings.ToUpper(r["tipo"]), Brand: strings.ToUpper(r["marca"]),
					Model: strings.ToUpper(r["modelo"]),
					Plate: strings.ToUpper(strings.ReplaceAll(r["placachassi"], " ", "")),
					Color: strings.ToUpper(r["cor"]),
				})
			}
		case "drogas":
			for _, r := range t.rows {
				d := Drug{Description: strings.ToUpper(r["descricao"])}
				if g, ok := parseDecimal(r["qtdgramas"]); ok {
					d.Grams = &g
				}
				if n, err := strconv.Atoi(strings.TrimSpace(r["qtdembalagens"])); err == nil {
					d.Packages = &n
				}
				o.Drugs = append(o.Drugs, d)
			}
		default:
			var rows []string
			for _, r := range t.rows {
				var vals []string
				for _, h := range t.header {
					if v := r[h.text]; v != "" {
						vals = append(vals, v)
					}
				}
				rows = append(rows, strings.Join(vals, " | "))
			}
			o.Warnings = append(o.Warnings, fmt.Sprintf("Tabela não reconhecida %q: %s.",
				t.title, strings.Join(rows, "; ")))
		}
	}
}

var nameNoteRE = regexp.MustCompile(`^(.*?)\s*\(([^()]*)\)\s*$`)

// splitNameNote separa "FULANO (ÓBITO)" em nome e anotação — o nome limpo é o
// que casa com o dossiê.
func splitNameNote(s string) (string, string) {
	if m := nameNoteRE.FindStringSubmatch(s); m != nil && m[1] != "" {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
	}
	return strings.TrimSpace(s), ""
}

// nearestColumn: o relatório centraliza a célula sob o cabeçalho, então a
// coluna é a de centro mais próximo.
func nearestColumn(header []cell, c cell) string {
	best, bestD := "", math.MaxFloat64
	for _, h := range header {
		if d := math.Abs(h.center() - c.center()); d < bestD {
			best, bestD = h.text, d
		}
	}
	return best
}

// parseDecimal aceita "4.3", "4,3" e "1.234,5".
func parseDecimal(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, false
	}
	return f, true
}

// ForUnit devolve as ocorrências do batalhão (comparação por UnitKey, então
// "2º BPRAIO" e "2ºBPRAIO" são o mesmo). O recorte é pelo BPM da linha Base:
// pelotão ou companhia novos entram sem cadastro prévio.
func (r *Report) ForUnit(unit string) []Occurrence {
	key := UnitKey(unit)
	var out []Occurrence
	for _, o := range r.Occurrences {
		if UnitKey(o.Unit()) == key {
			out = append(out, o)
		}
	}
	return out
}
