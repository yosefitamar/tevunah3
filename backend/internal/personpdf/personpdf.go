// Package personpdf lê os PDFs de consulta que o analista baixa de sistemas
// externos e anexa no cadastro de pessoa, devolvendo os dados já
// estruturados para preencher o formulário. Formatos aceitos, reconhecidos
// pelo título da primeira página:
//
//   - RELATÓRIO DE CPF (react-pdf): ficha da pessoa, RG, endereços,
//     telefones, filiação e histórico judicial (mandados, guias, alvarás).
//     Ver cpf_report.go.
//   - CONSULTA INTEGRADA (JasperReports "Espelho_Consulta"): dados da pessoa,
//     último endereço e ocorrências policiais como infrator/testemunha.
//     Ver consulta_integrada.go.
//
// A leitura parte das palavras com posição (pdftotext -bbox, o mesmo
// extrator do relatório operacional): o texto corrido gruda colunas
// vizinhas, e o x de cada palavra é o que diz a que coluna ela pertence.
//
// Os parsers são tolerantes: campo que não aparece fica vazio. O resultado só
// é rejeitado quando o formato não é reconhecido ou falta o mínimo (CPF no
// relatório de CPF, nome na consulta integrada).
package personpdf

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/opsreport"
)

// ErrUnknownFormat: o PDF não é de um formato reconhecido.
var ErrUnknownFormat = errors.New(
	"formato de PDF não reconhecido — envie o RELATÓRIO DE CPF ou a CONSULTA INTEGRADA")

// ErrIncomplete: o formato foi reconhecido, mas sem o dado mínimo.
var ErrIncomplete = errors.New("o PDF não traz a identificação da pessoa (nome/CPF)")

// Formatos reconhecidos (Report.Format) e a fonte gravada no histórico.
const (
	FormatCPFReport         = "cpf_report"
	FormatConsultaIntegrada = "consulta_integrada"

	SourceCPFReport         = "RELATÓRIO DE CPF"
	SourceConsultaIntegrada = "CONSULTA INTEGRADA"
)

// Report é o conteúdo aproveitável do PDF. Datas em YYYY-MM-DD, nomes e
// textos em MAIÚSCULAS, CPF/CEP/telefones/SIP só com dígitos.
type Report struct {
	Format string `json:"format"`
	Source string `json:"source"`

	Name         string   `json:"name"`
	CPF          string   `json:"cpf"`
	Aliases      []string `json:"aliases"`
	CriminalFlag bool     `json:"criminal_flag"` // "INDICATIVO CRIMINAL" (relatório de CPF)
	MotherName   string   `json:"mother_name"`
	FatherName   string   `json:"father_name"`
	BirthDate    string   `json:"birth_date"`
	Gender       string   `json:"gender"` // "M", "F" ou ""
	Birthplace   string   `json:"birthplace"`
	Nationality  string   `json:"nationality"`
	SIPNumber    string   `json:"sip_number"`

	RGs       []RG      `json:"rgs"`
	Phones    []string  `json:"phones"`
	Addresses []Address `json:"addresses"`
	Criminal  []Record  `json:"criminal"`

	// GeneratedAt é a data/hora de emissão do PDF (horário de Fortaleza).
	GeneratedAt *time.Time `json:"generated_at,omitempty"`
}

// RG é uma identidade.
type RG struct {
	Number   string `json:"number"`
	Issuer   string `json:"issuer"`
	State    string `json:"state"`
	IssuedOn string `json:"issued_on"`
}

// Address é um endereço.
type Address struct {
	Street       string `json:"street"`
	Number       string `json:"number"`
	Complement   string `json:"complement"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	State        string `json:"state"`
	CEP          string `json:"cep"`
}

// Record é um item do histórico criminal, no mesmo formato JSON que o
// cadastro grava (entities.CriminalRecord): o frontend repassa sem conversão.
type Record struct {
	Category      string `json:"category"` // "judicial" | "policial"
	Role          string `json:"role"`     // INFRATOR, TESTEMUNHA (policial)
	Title         string `json:"title"`    // tipo da peça / natureza do fato
	Detail        string `json:"detail"`   // espécie da peça
	ProcedureType string `json:"procedure_type"`
	Number        string `json:"number"` // processo / nº do procedimento
	Piece         string `json:"piece"`
	Date          string `json:"date"`     // expedição / data da ocorrência
	FiledOn       string `json:"filed_on"` // instauração
	Agency        string `json:"agency"`   // órgão expedidor / delegacia
	Court         string `json:"court"`
	District      string `json:"district"`
	Reason        string `json:"reason"`
	Source        string `json:"source"`
}

// Parse identifica o formato pelo título e lê o PDF.
func Parse(pages []opsreport.Page) (*Report, error) {
	ls := toLines(pages)
	if len(ls) == 0 {
		return nil, ErrUnknownFormat
	}
	var (
		rep *Report
		err error
	)
	switch fold(ls[0].text()) {
	case "relatorio de cpf":
		rep, err = parseCPFReport(ls)
	case "consulta integrada":
		rep, err = parseConsultaIntegrada(ls)
	default:
		return nil, ErrUnknownFormat
	}
	if err != nil {
		return nil, err
	}
	for _, s := range []*[]string{&rep.Aliases, &rep.Phones} {
		if *s == nil {
			*s = []string{}
		}
	}
	if rep.RGs == nil {
		rep.RGs = []RG{}
	}
	if rep.Addresses == nil {
		rep.Addresses = []Address{}
	}
	if rep.Criminal == nil {
		rep.Criminal = []Record{}
	}
	return rep, nil
}

// ─── Linhas ──────────────────────────────────────────────────────────

type line struct {
	page  int
	y, h  float64 // topo e altura (maior palavra) da linha
	words []opsreport.Word
}

func (l line) x0() float64 { return l.words[0].X0 }

func (l line) text() string {
	parts := make([]string, len(l.words))
	for i, w := range l.words {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
}

// cells quebra a linha onde o espaço entre palavras passa de cellGap. Espaço
// normal entre palavras fica abaixo de 5pt; entre células, acima de 9pt.
func (l line) cells() []string {
	const cellGap = 7
	var out []string
	cur := []string{l.words[0].Text}
	for i := 1; i < len(l.words); i++ {
		if l.words[i].X0-l.words[i-1].X1 > cellGap {
			out = append(out, strings.Join(cur, " "))
			cur = nil
		}
		cur = append(cur, l.words[i].Text)
	}
	return append(out, strings.Join(cur, " "))
}

const lineTol = 2.5 // palavras com topo a até 2,5pt formam a mesma linha

// toLines agrupa as palavras de cada página em linhas, de cima para baixo.
func toLines(pages []opsreport.Page) []line {
	var out []line
	for pi, p := range pages {
		ws := append([]opsreport.Word(nil), p.Words...)
		sort.SliceStable(ws, func(i, j int) bool { return ws[i].Y0 < ws[j].Y0 })
		var cur *line
		for _, w := range ws {
			if cur != nil && w.Y0-cur.y <= lineTol {
				cur.words = append(cur.words, w)
				if h := w.Y1 - w.Y0; h > cur.h {
					cur.h = h
				}
				continue
			}
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &line{page: pi, y: w.Y0, h: w.Y1 - w.Y0, words: []opsreport.Word{w}}
		}
		if cur != nil {
			out = append(out, *cur)
		}
	}
	for i := range out {
		sort.SliceStable(out[i].words, func(a, b int) bool {
			return out[i].words[a].X0 < out[i].words[b].X0
		})
	}
	return out
}

// ─── Utilitários ─────────────────────────────────────────────────────

// fortaleza é UTC-3 fixo (sem horário de verão desde 2019, e o Ceará não o
// adotava antes). Fixo evita depender do tzdata no container.
var fortaleza = time.FixedZone("America/Fortaleza", -3*3600)

var dateRe = regexp.MustCompile(`\b(\d{2})/(\d{2})/(\d{4})\b`)

// isoDate acha a primeira data dd/mm/aaaa do texto e devolve YYYY-MM-DD.
func isoDate(s string) string {
	m := dateRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	t, err := time.Parse("02/01/2006", m[1]+"/"+m[2]+"/"+m[3])
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func upper(s string) string { return strings.ToUpper(strings.Join(strings.Fields(s), " ")) }

// splitDash quebra em " - " (e em "-" nas pontas), descartando partes
// vazias. Sempre devolve ao menos um elemento.
func splitDash(s string) []string {
	var out []string
	for _, p := range strings.Split(s, " - ") {
		p = strings.TrimSpace(strings.Trim(strings.TrimSpace(p), "-"))
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// splitStreet separa "R S LUIZ DO OESTE, 92 - C" em logradouro, número e
// complemento. Número "0" é o "sem número" dos sistemas de origem.
func splitStreet(s string) (street, number, complement string) {
	s = strings.TrimSpace(s)
	k := strings.LastIndex(s, ",")
	if k < 0 {
		return upper(s), "", ""
	}
	street = upper(s[:k])
	rest := splitDash(s[k+1:])
	number = upper(rest[0])
	if number == "0" {
		number = "S/N"
	}
	if len(rest) > 1 {
		complement = upper(strings.Join(rest[1:], " - "))
	}
	return street, number, complement
}

// fold: minúsculas sem acento, para comparar rótulos e títulos.
func fold(s string) string {
	r := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i",
		"ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c",
	)
	return r.Replace(strings.ToLower(strings.Join(strings.Fields(s), " ")))
}

// emptyValue: marcadores de "sem dado" que os sistemas imprimem no campo.
func emptyValue(s string) bool {
	switch fold(s) {
	case "", "null", "-", "n/a", "nao informado":
		return true
	}
	return false
}
