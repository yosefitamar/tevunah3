package entities

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// CriminalRecord é um item do histórico criminal da pessoa. Um formato só
// para as duas origens que o cadastro importa:
//
//   - judicial: peça do RELATÓRIO DE CPF (mandado de prisão, guia de
//     recolhimento, alvará). Title = tipo da peça, Number = processo CNJ,
//     Date = expedição, Agency = órgão expedidor (vara).
//   - policial: ocorrência da CONSULTA INTEGRADA. Title = natureza do fato,
//     Role = INFRATOR/TESTEMUNHA, Number = nº do procedimento, Date = data da
//     ocorrência, FiledOn = instauração, Agency = delegacia.
//
// Datas em YYYY-MM-DD; textos em MAIÚSCULAS.
type CriminalRecord struct {
	Category      string `json:"category"`
	Role          string `json:"role"`
	Title         string `json:"title"`
	Detail        string `json:"detail"`
	ProcedureType string `json:"procedure_type"`
	Number        string `json:"number"`
	Piece         string `json:"piece"`
	Date          string `json:"date"`
	FiledOn       string `json:"filed_on"`
	Agency        string `json:"agency"`
	Court         string `json:"court"`
	District      string `json:"district"`
	Reason        string `json:"reason"`
	Source        string `json:"source"`
}

// maxCriminalRecords limita a lista por pessoa: a CONSULTA INTEGRADA de um
// reincidente passa de dezenas de ocorrências, mas não de centenas.
const maxCriminalRecords = 300

// ErrCriminalRecords indica lista de histórico inválida (o handler devolve 400).
var ErrCriminalRecords = errors.New("histórico criminal inválido")

// normalizeCriminalRecords põe os textos em MAIÚSCULAS, valida datas e
// categoria e descarta itens vazios. nil continua nil (update: mantém).
func normalizeCriminalRecords(in []CriminalRecord) ([]CriminalRecord, error) {
	if in == nil {
		return nil, nil
	}
	if len(in) > maxCriminalRecords {
		return nil, ErrCriminalRecords
	}
	date := func(s string) string {
		s = strings.TrimSpace(s)
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return ""
		}
		return s
	}
	out := make([]CriminalRecord, 0, len(in))
	for _, r := range in {
		n := CriminalRecord{
			Category:      strings.ToLower(strings.TrimSpace(r.Category)),
			Role:          upperTrim(r.Role),
			Title:         upperTrim(r.Title),
			Detail:        upperTrim(r.Detail),
			ProcedureType: upperTrim(r.ProcedureType),
			Number:        upperTrim(r.Number),
			Piece:         upperTrim(r.Piece),
			Date:          date(r.Date),
			FiledOn:       date(r.FiledOn),
			Agency:        upperTrim(r.Agency),
			Court:         upperTrim(r.Court),
			District:      upperTrim(r.District),
			Reason:        upperTrim(r.Reason),
			Source:        upperTrim(r.Source),
		}
		if n.Category != "judicial" && n.Category != "policial" {
			n.Category = ""
		}
		probe := n
		probe.Category, probe.Source = "", ""
		if probe == (CriminalRecord{}) {
			continue // item sem conteúdo (linha deixada em branco no form)
		}
		out = append(out, n)
	}
	return out, nil
}

// criminalRecordsJSON normaliza e serializa para a coluna jsonb. nil → nil.
func criminalRecordsJSON(in []CriminalRecord) ([]byte, error) {
	recs, err := normalizeCriminalRecords(in)
	if err != nil || recs == nil {
		return nil, err
	}
	return json.Marshal(recs)
}

// digitsPtr deixa só os dígitos (nº SIP). nil continua nil.
func digitsPtr(p *string) *string {
	if p == nil {
		return nil
	}
	var b strings.Builder
	for _, r := range *p {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	v := b.String()
	return &v
}
