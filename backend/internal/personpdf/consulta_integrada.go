package personpdf

// CONSULTA INTEGRADA (JasperReports "Espelho_Consulta", ~2 páginas).
//
//   - Toda página abre com o título "CONSULTA INTEGRADA" e fecha com o rodapé
//     "Data Hora: dd/mm/aaaa hh:mm:ss    Pag. N de M" (data da consulta).
//   - Seções com título centralizado e sem ":": "Dados da Pessoa", "Último
//     Endereço", "Ocorrência(s) como Infrator", "Ocorrência(s) como
//     Testemunha" (o papel vem do título, qualquer que seja).
//   - Campos "Rótulo: valor", até dois por linha ("Data de Nascimento: … Nº
//     SIP: …"). Linha sem rótulo continua o último campo — é assim que a
//     Filiação traz mãe e pai em duas linhas ("null" quando não há).
//   - Cada ocorrência começa em "Natureza do Fato:" (ou "Natureza Fato:", na
//     seção de testemunha) seguida de Delegacia, Tipo/Nº Procedimento e
//     datas de ocorrência e instauração.

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ciLabelRe = regexp.MustCompile(`(?:^|\s)(Nome|Filiação|Data de Nascimento|Nº SIP|RG|CPF|Alcunhas|` +
		`Natureza do Fato|Natureza Fato|Delegacia|Tipo Procedimento|Nº Procedimento|` +
		`Data Ocorrência|Data Instauração):`)
	ciRoleRe      = regexp.MustCompile(`^Ocorrência\(s\) como (.+)$`)
	ciGeneratedRe = regexp.MustCompile(`Data Hora:\s*(\d{2})/(\d{2})/(\d{4})\s+(\d{2}):(\d{2})(?::(\d{2}))?`)
	ciPhoneRe     = regexp.MustCompile(`\(?\d{2}\)?\s*\d{4,5}-?\d{4}`)
	ciCEPRe       = regexp.MustCompile(`\b\d{5}-?\d{3}\b`)
	ciCityUFRe    = regexp.MustCompile(`^(.*\S)\s+([A-Z]{2})$`)
)

type ciField struct{ label, value string }

// ciFields separa os pares "Rótulo: valor" da linha. Nil se não houver rótulo.
func ciFields(t string) []ciField {
	idx := ciLabelRe.FindAllStringSubmatchIndex(t, -1)
	if idx == nil {
		return nil
	}
	out := make([]ciField, 0, len(idx))
	for i, m := range idx {
		end := len(t)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		out = append(out, ciField{
			label: t[m[2]:m[3]],
			value: strings.TrimSpace(t[m[1]:end]),
		})
	}
	return out
}

func parseConsultaIntegrada(ls []line) (*Report, error) {
	rep := &Report{Format: FormatConsultaIntegrada, Source: SourceConsultaIntegrada}

	person := map[string][]string{}
	var addrLines []string
	var cur *Record
	flush := func() {
		if cur != nil {
			cur.Title = strings.TrimSpace(strings.TrimSuffix(cur.Title, "-"))
			rep.Criminal = append(rep.Criminal, *cur)
			cur = nil
		}
	}

	section, role, last := "", "", ""
	for _, l := range ls {
		t := l.text()
		f := fold(t)
		switch {
		case f == "consulta integrada":
			continue
		case strings.HasPrefix(t, "Data Hora:"):
			if rep.GeneratedAt == nil {
				rep.GeneratedAt = parseCIGenerated(t)
			}
			continue
		case f == "dados da pessoa":
			section, last = "person", ""
			continue
		case f == "ultimo endereco":
			section, last = "address", ""
			continue
		}
		if m := ciRoleRe.FindStringSubmatch(t); m != nil {
			flush()
			section, role, last = "occ", upper(m[1]), ""
			continue
		}

		fields := ciFields(t)
		switch section {
		case "person":
			if fields == nil {
				if last != "" {
					person[last] = append(person[last], t)
				}
				continue
			}
			for _, fd := range fields {
				person[fd.label] = append(person[fd.label], fd.value)
				last = fd.label
			}
		case "address":
			addrLines = append(addrLines, t)
		case "occ":
			if fields == nil {
				// Natureza longa quebrada em duas linhas.
				if cur != nil && (last == "Natureza do Fato" || last == "Natureza Fato") {
					cur.Title += " " + upper(t)
				}
				continue
			}
			for _, fd := range fields {
				last = fd.label
				v := upper(fd.value)
				if fd.label == "Natureza do Fato" || fd.label == "Natureza Fato" {
					flush()
					cur = &Record{Category: "policial", Role: role, Title: v, Source: SourceConsultaIntegrada}
					continue
				}
				if cur == nil {
					continue
				}
				switch fd.label {
				case "Delegacia":
					cur.Agency = v
				case "Tipo Procedimento":
					cur.ProcedureType = v
				case "Nº Procedimento":
					cur.Number = v
				case "Data Ocorrência":
					cur.Date = isoDate(v)
				case "Data Instauração":
					cur.FiledOn = isoDate(v)
				}
			}
		}
	}
	flush()

	first := func(k string) string {
		if v := person[k]; len(v) > 0 && !emptyValue(v[0]) {
			return v[0]
		}
		return ""
	}
	rep.Name = upper(first("Nome"))
	if rep.Name == "" {
		return nil, ErrIncomplete
	}
	if fil := person["Filiação"]; len(fil) > 0 {
		if !emptyValue(fil[0]) {
			rep.MotherName = upper(fil[0])
		}
		if len(fil) > 1 && !emptyValue(fil[1]) {
			rep.FatherName = upper(fil[1])
		}
	}
	rep.BirthDate = isoDate(first("Data de Nascimento"))
	rep.SIPNumber = digits(first("Nº SIP"))
	if c := digits(first("CPF")); len(c) == 11 {
		rep.CPF = c
	}
	if rg := first("RG"); rg != "" {
		parts := splitDash(rg)
		r := RG{Number: upper(parts[0])}
		if len(parts) > 1 {
			org := upper(parts[1])
			if k := strings.LastIndex(org, "/"); k >= 0 && len(org)-k == 3 {
				org, r.State = org[:k], org[k+1:]
			}
			r.Issuer = org
		}
		rep.RGs = []RG{r}
	}
	for _, a := range strings.FieldsFunc(first("Alcunhas"), func(r rune) bool {
		return r == ',' || r == ';' || r == '/'
	}) {
		if a = upper(a); !emptyValue(a) {
			rep.Aliases = append(rep.Aliases, a)
		}
	}
	parseCIAddress(rep, strings.Join(addrLines, " "))
	return rep, nil
}

// parseCIAddress lê "R PRINCIPAL, 10 - TANUPABA - CAUCAIA CE. Telefone: 85…".
func parseCIAddress(rep *Report, s string) {
	s = strings.TrimSpace(s)
	if k := strings.Index(fold(s), "telefone"); k >= 0 {
		for _, m := range ciPhoneRe.FindAllString(s[k:], -1) {
			if d := digits(m); len(d) == 10 || len(d) == 11 {
				rep.Phones = append(rep.Phones, d)
			}
		}
		s = strings.TrimSpace(s[:k])
	}
	a := Address{}
	if m := ciCEPRe.FindString(s); m != "" {
		a.CEP = digits(m)
		s = strings.TrimSpace(strings.Replace(s, m, "", 1))
	}
	s = strings.TrimSpace(strings.TrimRight(s, ".- "))
	if emptyValue(s) {
		return
	}
	parts := splitDash(s)
	a.Street, a.Number, a.Complement = splitStreet(parts[0])
	if n := len(parts); n > 1 {
		city := upper(strings.TrimRight(parts[n-1], ". "))
		if m := ciCityUFRe.FindStringSubmatch(city); m != nil {
			a.City, a.State = m[1], m[2]
		} else {
			a.City = city
		}
		a.Neighborhood = upper(strings.Join(parts[1:n-1], " - "))
	}
	rep.Addresses = append(rep.Addresses, a)
}

func parseCIGenerated(s string) *time.Time {
	m := ciGeneratedRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	sec := 0
	if m[6] != "" {
		sec = n(6)
	}
	t := time.Date(n(3), time.Month(n(2)), n(1), n(4), n(5), sec, 0, fortaleza)
	return &t
}
