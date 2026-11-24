package personpdf

// RELATÓRIO DE CPF (react-pdf).
//
//   - Toda página repete o cabeçalho (RELATÓRIO DE CPF / nome / CPF) e o
//     rodapé ("Gerado em … às HH:MM    Página N de M"). Os dois saem antes
//     de ler o corpo; o rodapé dá a data/hora da consulta.
//   - Página 1: ficha com grade de rótulo (~6,5pt, MAIÚSCULAS) sobre valor
//     (~8,3pt), em duas colunas. "INDICATIVO CRIMINAL" sob o nome é o alerta
//     do sistema de origem.
//   - Depois, seções com título de ~12pt no formato "Nome (N)": Histórico
//     Criminal, Identidades - RG, Endereços, Telefones, Pessoas Próximas…
//     Seções sem uso no cadastro (renda, empréstimos, vacinas, propensões,
//     linha do tempo) são ignoradas.

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Margens verticais da página A4 (842pt): cabeçalho repetido acima de
// cpfHeaderBottom, rodapé abaixo de cpfFooterTop.
const (
	cpfHeaderBottom = 95
	cpfFooterTop    = 790
)

var (
	cpfRe       = regexp.MustCompile(`\d{3}\.?\d{3}\.?\d{3}-?\d{2}`)
	sectionRe   = regexp.MustCompile(`^(.+?)\s*\((\d+)\)$`)
	phoneRe     = regexp.MustCompile(`\((\d{2})\)\s*(\d{4,5}-?\d{4})`)
	generatedRe = regexp.MustCompile(`Gerado em (\d{1,2}) de (\p{L}+) de (\d{4}) às (\d{1,2}):(\d{2})`)
	onlyNumRe   = regexp.MustCompile(`^\d{1,3}$`)
)

func parseCPFReport(all []line) (*Report, error) {
	rep := &Report{Format: FormatCPFReport, Source: SourceCPFReport}

	// Cabeçalho da 1ª página: título, nome (fonte grande), "CPF: …".
	// Rodapés: "Gerado em …". O corpo fica entre os dois.
	var body []line
	for _, l := range all {
		switch {
		case l.y < cpfHeaderBottom:
			if l.page != 0 {
				continue
			}
			t := l.text()
			if rep.CPF == "" && strings.HasPrefix(t, "CPF:") {
				rep.CPF = digits(cpfRe.FindString(t))
			} else if rep.Name == "" && l.h > 13 {
				rep.Name = upper(t)
			}
		case l.y > cpfFooterTop:
			if rep.GeneratedAt == nil {
				rep.GeneratedAt = parseGenerated(l.text())
			}
		default:
			body = append(body, l)
		}
	}
	if len(rep.CPF) != 11 {
		return nil, ErrIncomplete
	}

	// Separa a ficha (antes da 1ª seção) das seções.
	type section struct {
		name  string
		lines []line
	}
	var profile []line
	var sections []section
	for _, l := range body {
		if l.h >= 10.5 && l.h <= 13 && l.x0() < 60 {
			if m := sectionRe.FindStringSubmatch(l.text()); m != nil {
				sections = append(sections, section{name: fold(m[1])})
				continue
			}
		}
		if len(sections) == 0 {
			profile = append(profile, l)
		} else {
			sections[len(sections)-1].lines = append(sections[len(sections)-1].lines, l)
		}
	}

	parseProfile(rep, profile)
	var relatives []relative
	for _, s := range sections {
		switch s.name {
		case "historico criminal":
			rep.Criminal = parseJudicial(s.lines)
		case "identidades - rg":
			if rgs := parseRGs(s.lines); len(rgs) > 0 {
				rep.RGs = rgs
			}
		case "enderecos":
			rep.Addresses = parseAddresses(s.lines)
		case "telefones":
			rep.Phones = parsePhones(s.lines)
		case "pessoas proximas":
			relatives = parseRelatives(s.lines)
		}
	}

	// Filiação: a ficha é a fonte principal; "Pessoas Próximas" cobre a
	// ficha sem o campo preenchido.
	for _, r := range relatives {
		switch r.degree {
		case "MAE":
			if rep.MotherName == "" {
				rep.MotherName = r.name
			}
		case "PAI":
			if rep.FatherName == "" {
				rep.FatherName = r.name
			}
		}
	}
	return rep, nil
}

// ─── Ficha (página 1) ────────────────────────────────────────────────

func parseProfile(rep *Report, ls []line) {
	for _, l := range ls {
		if fold(l.text()) == "indicativo criminal" {
			rep.CriminalFlag = true
		}
	}
	g := readGrids(ls)
	rep.MotherName = upper(g["NOME DA MÃE"])
	rep.FatherName = upper(g["NOME DO PAI"])
	rep.BirthDate = isoDate(g["DATA DE NASCIMENTO"])
	switch fold(g["SEXO"]) {
	case "masculino":
		rep.Gender = "M"
	case "feminino":
		rep.Gender = "F"
	}
	rep.Birthplace = upper(g["NATURALIDADE"])
	rep.Nationality = upper(g["NACIONALIDADE"])

	// RG da ficha: "20079756519 - SSP - SSP - SECRETARIA … PUBLICA/CE". Só
	// vale se a seção "Identidades - RG" não vier (ela é mais limpa e a
	// sobrescreve depois).
	if rg := strings.TrimSpace(g["RG"]); rg != "" {
		parts := splitDash(rg)
		r := RG{Number: parts[0]}
		if len(parts) > 1 {
			r.Issuer = upper(parts[1])
		}
		if i := strings.LastIndex(rg, "/"); i >= 0 && len(rg)-i == 3 {
			r.State = upper(rg[i+1:])
		}
		rep.RGs = []RG{r}
	}
}

// isLabelLine: rótulo da grade — só MAIÚSCULAS, sem dígitos, fonte pequena.
func isLabelLine(l line) bool {
	if l.h > 7.2 {
		return false
	}
	hasLetter := false
	for _, r := range l.text() {
		if unicode.IsLower(r) || unicode.IsDigit(r) {
			return false
		}
		if unicode.IsLetter(r) {
			hasLetter = true
		}
	}
	return hasLetter
}

// readGrids lê todas as grades rótulo/valor das linhas. Cada palavra de
// valor vai para o rótulo mais à direita que começa antes dela; linhas de
// valor seguidas (quebra de linha dentro da célula) são concatenadas.
func readGrids(ls []line) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(ls); i++ {
		if !isLabelLine(ls[i]) {
			continue
		}
		type col struct {
			x    float64
			name string
		}
		var cols []col
		lbl := ls[i]
		cur := col{x: lbl.words[0].X0, name: lbl.words[0].Text}
		for k := 1; k < len(lbl.words); k++ {
			if lbl.words[k].X0-lbl.words[k-1].X1 > 7 {
				cols = append(cols, cur)
				cur = col{x: lbl.words[k].X0, name: lbl.words[k].Text}
				continue
			}
			cur.name += " " + lbl.words[k].Text
		}
		cols = append(cols, cur)

		vals := make([][]string, len(cols))
		prevY := lbl.y
		j := i + 1
		for ; j < len(ls); j++ {
			v := ls[j]
			if isLabelLine(v) || v.h <= lbl.h+0.5 || v.page != lbl.page || v.y-prevY > 2*v.h+2 {
				break
			}
			for _, w := range v.words {
				c := 0
				for ci := range cols {
					if w.X0+2 >= cols[ci].x {
						c = ci
					}
				}
				vals[c] = append(vals[c], w.Text)
			}
			prevY = v.y
		}
		for ci, c := range cols {
			if s := strings.Join(vals[ci], " "); s != "" {
				out[c.name] = s
			}
		}
		i = j - 1
	}
	return out
}

// ─── Histórico criminal (peças judiciais) ────────────────────────────

// parseJudicial: cada peça é título (~9pt) + linha de espécie/tribunal/
// comarca + grade (PROCESSO/PEÇA, EXPEDIÇÃO/MOTIVO, ÓRGÃO EXPEDIDOR). O
// número de ordem ("01") sai numa linha própria e é ignorado. A âncora é o
// rótulo PROCESSO: as duas linhas úteis antes dele são o subtítulo e o título.
func parseJudicial(ls []line) []Record {
	var useful []line
	for _, l := range ls {
		if onlyNumRe.MatchString(l.text()) {
			continue
		}
		useful = append(useful, l)
	}
	var anchors []int
	for i, l := range useful {
		if isLabelLine(l) && strings.HasPrefix(l.text(), "PROCESSO") && i >= 2 {
			anchors = append(anchors, i)
		}
	}
	out := make([]Record, 0, len(anchors))
	for n, a := range anchors {
		end := len(useful)
		if n+1 < len(anchors) {
			end = anchors[n+1] - 2
		}
		rec := Record{
			Category: "judicial",
			Source:   SourceCPFReport,
			Title:    upper(useful[a-2].text()),
		}
		sub := useful[a-1].cells()
		if k := len(sub); k >= 3 {
			rec.District = upper(sub[k-1])
			rec.Court = upper(sub[k-2])
			rec.Detail = upper(strings.Join(sub[:k-2], " "))
		} else if k == 2 {
			rec.Court, rec.District = upper(sub[0]), upper(sub[1])
		}
		if rec.Detail == "N/A" {
			rec.Detail = ""
		}
		g := readGrids(useful[a:end])
		rec.Number = g["PROCESSO"]
		rec.Piece = g["PEÇA"]
		rec.Date = isoDate(g["EXPEDIÇÃO"])
		rec.Reason = upper(g["MOTIVO"])
		rec.Agency = upper(g["ÓRGÃO EXPEDIDOR"])
		out = append(out, rec)
	}
	return out
}

// ─── Identidades ─────────────────────────────────────────────────────

func parseRGs(ls []line) []RG {
	var out []RG
	for _, l := range ls {
		t := l.text()
		switch {
		case l.h > 11 && len(l.words) == 1:
			out = append(out, RG{Number: t})
		case len(out) == 0:
			continue
		case strings.HasPrefix(t, "Órgão Emissor:"):
			out[len(out)-1].Issuer = upper(splitDash(strings.TrimPrefix(t, "Órgão Emissor:"))[0])
		case strings.HasPrefix(t, "Estado:"):
			out[len(out)-1].State = upper(strings.TrimSpace(strings.TrimPrefix(t, "Estado:")))
		case strings.HasPrefix(t, "Data de Emissão:"):
			out[len(out)-1].IssuedOn = isoDate(t)
		}
	}
	return out
}

// ─── Endereços ───────────────────────────────────────────────────────

// parseAddresses: logradouro numa linha ("R MARIO MENDES, 762"), localidade
// na seguinte ("PRQ POTIRA JUREMA - CAUCAIA/CE - CEP: 61648110").
func parseAddresses(ls []line) []Address {
	out := []Address{}
	var street []string
	for _, l := range ls {
		t := l.text()
		i := strings.Index(t, "CEP:")
		if i < 0 {
			street = append(street, t)
			continue
		}
		a := Address{CEP: digits(t[i+4:])}
		if len(a.CEP) != 8 {
			a.CEP = ""
		}
		a.Street, a.Number, a.Complement = splitStreet(strings.Join(street, " "))
		street = nil

		loc := splitDash(t[:i])
		if n := len(loc); n > 0 {
			cityUF := loc[n-1]
			if k := strings.LastIndex(cityUF, "/"); k >= 0 {
				a.City, a.State = upper(cityUF[:k]), upper(cityUF[k+1:])
			} else {
				a.City = upper(cityUF)
			}
			// "ITAITINGA CE/CE": a UF às vezes vem repetida no nome da cidade.
			a.City = strings.TrimSpace(strings.TrimSuffix(a.City, " "+a.State))
			a.Neighborhood = upper(strings.Join(loc[:n-1], " - "))
		}
		out = append(out, a)
	}
	return out
}

// ─── Telefones e parentes ────────────────────────────────────────────

func parsePhones(ls []line) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, l := range ls {
		for _, m := range phoneRe.FindAllStringSubmatch(l.text(), -1) {
			d := m[1] + digits(m[2])
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	return out
}

type relative struct{ name, degree string }

// parseRelatives lê "Pessoas Próximas": nome (linha sem ":") seguido de
// "Grau: Mae". "Sem foto" é o marcador da foto ausente.
func parseRelatives(ls []line) []relative {
	var out []relative
	name := ""
	for _, l := range ls {
		t := l.text()
		switch {
		case t == "Sem foto":
		case strings.HasPrefix(t, "Grau:"):
			if name != "" {
				out = append(out, relative{name: name, degree: strings.ToUpper(fold(strings.TrimPrefix(t, "Grau:")))})
			}
			name = ""
		case !strings.Contains(t, ":"):
			name = upper(t)
		}
	}
	return out
}

// ─── Rodapé ──────────────────────────────────────────────────────────

var months = map[string]time.Month{
	"janeiro": 1, "fevereiro": 2, "marco": 3, "abril": 4, "maio": 5, "junho": 6,
	"julho": 7, "agosto": 8, "setembro": 9, "outubro": 10, "novembro": 11, "dezembro": 12,
}

func parseGenerated(s string) *time.Time {
	m := generatedRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	mon, ok := months[fold(m[2])]
	if !ok {
		return nil
	}
	day, _ := strconv.Atoi(m[1])
	year, _ := strconv.Atoi(m[3])
	hh, _ := strconv.Atoi(m[4])
	mm, _ := strconv.Atoi(m[5])
	t := time.Date(year, mon, day, hh, mm, 0, 0, fortaleza)
	return &t
}
