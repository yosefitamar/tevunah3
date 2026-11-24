package personpdf

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/belia/tevunah/backend/internal/opsreport"
)

// A fixture reproduz a geometria do RELATÓRIO DE CPF real (posições x/y e
// alturas de fonte medidas no pdftotext -bbox) com dados FICTÍCIOS. Nenhum
// relatório real entra no repositório — eles trazem dados pessoais.

// cell é um trecho de texto que começa em x; as palavras ganham largura
// proporcional à fonte e espaço normal (~2,5pt) entre si.
type cell struct {
	x    float64
	text string
}

type fixLine struct {
	y, h  float64
	cells []cell
}

func page(lines ...fixLine) opsreport.Page {
	var p opsreport.Page
	for _, l := range lines {
		for _, c := range l.cells {
			x := c.x
			for _, w := range strings.Fields(c.text) {
				wd := float64(len([]rune(w))) * l.h * 0.5
				p.Words = append(p.Words, opsreport.Word{X0: x, Y0: l.y, X1: x + wd, Y1: l.y + l.h, Text: w})
				x += wd + 2.5
			}
		}
	}
	return p
}

func ln(y, h float64, cells ...cell) fixLine { return fixLine{y: y, h: h, cells: cells} }

func header() []fixLine {
	return []fixLine{
		ln(42, 8.3, cell{40, "RELATÓRIO DE CPF"}),
		ln(55, 16.6, cell{40, "Fulano de Tal Ficticio"}),
		ln(76, 9.2, cell{40, "CPF: 123.456.789-09"}),
	}
}

func footer(n int) fixLine {
	return ln(810, 7.4, cell{40, "Gerado em 5 de março de 2026 às 09:07"}, cell{506, "Página " + string(rune('0'+n)) + " de 3"})
}

func fixture() []opsreport.Page {
	p1 := append(header(),
		ln(134, 14.8, cell{141, "Fulano de Tal Ficticio"}),
		ln(155, 10.2, cell{141, "CPF: 123.456.789-09"}),
		ln(158, 9.2, cell{72, "Sem foto"}),
		ln(176, 7.4, cell{147, "INDICATIVO CRIMINAL"}),
		ln(219, 6.5, cell{57, "NOME DA MÃE"}, cell{299, "NOME DO PAI"}),
		ln(228, 8.3, cell{57, "Beltrana de Tal"}, cell{299, "Ciclano de Tal"}),
		ln(246, 6.5, cell{57, "DATA DE NASCIMENTO"}, cell{299, "SEXO"}),
		ln(255, 8.3, cell{57, "02/03/1990"}, cell{299, "Masculino"}),
		ln(272, 6.5, cell{57, "NATURALIDADE"}, cell{299, "NACIONALIDADE"}),
		ln(281, 8.3, cell{57, "SOBRAL/CE"}, cell{299, "Brasileiro"}),
		ln(325, 6.5, cell{57, "RG"}, cell{299, "CNS"}),
		// O fim do RG encosta no CNS: sem célula separada, só o x distingue.
		ln(334, 8.3, cell{57, "99999 - SSP - SSP - SECRETARIA DE SEGURANCA"}, cell{299, "700000000000000"}),
		ln(344, 8.3, cell{57, "PUBLICA/CE"}),
		ln(362, 6.5, cell{57, "RENDA"}, cell{299, "CLASSE SOCIAL"}),
		ln(371, 8.3, cell{57, "R$ 1.000,00"}, cell{299, "D"}),
		ln(462, 12.9, cell{99, "2"}, cell{203, "2"}),
		ln(477, 5.6, cell{89, "Endereços"}, cell{195, "Telefones"}),
		ln(520, 5.6, cell{92, "Veículos"}),
		footer(1),
	)
	p2 := append(header(),
		ln(106, 12.0, cell{40, "Histórico Criminal (2)"}),
		ln(130, 7.4, cell{48, "Estes registros têm caráter histórico e finalidade meramente informativa."}),
		ln(180, 9.2, cell{79, "Guia de Recolhimento"}),
		ln(184, 7.4, cell{58, "01"}),
		ln(205, 6.5, cell{84, "N/A"}, cell{110, "TJCE"}, cell{141, "Fortaleza"}),
		ln(225, 6.5, cell{53, "PROCESSO"}, cell{299, "PEÇA"}),
		ln(234, 8.3, cell{53, "0000001-11.2015.8.06.0001"}, cell{299, "0000001-11.2015.8.06.0001.11000115"}),
		ln(251, 6.5, cell{53, "EXPEDIÇÃO"}),
		ln(260, 8.3, cell{53, "16/05/2018"}),
		ln(297, 6.5, cell{61, "ÓRGÃO EXPEDIDOR"}),
		ln(306, 8.3, cell{61, "1ª VARA DAS EXECUÇOES PENAIS DA COMARCA DE FORTALEZA"}),
		ln(352, 9.2, cell{79, "Mandado de Prisão"}),
		ln(356, 7.4, cell{58, "02"}),
		ln(372, 6.5, cell{84, "Decorrente de condenação não transitada em julgado"}, cell{290, "TJCE"}, cell{320, "Caucaia"}),
		ln(392, 6.5, cell{53, "PROCESSO"}, cell{299, "PEÇA"}),
		ln(401, 8.3, cell{53, "0000002-22.2016.8.06.0064"}, cell{299, "0000002-22.2016.8.06.0064.01000312"}),
		ln(418, 6.5, cell{53, "EXPEDIÇÃO"}, cell{299, "MOTIVO"}),
		ln(427, 8.3, cell{53, "28/01/2021"}, cell{299, "Progressão de regime"}),
		ln(464, 6.5, cell{61, "ÓRGÃO EXPEDIDOR"}),
		ln(473, 8.3, cell{61, "4ª VARA CRIMINAL DA COMARCA DE CAUCAIA"}),
		ln(520, 12.0, cell{40, "Identidades - RG (1)"}),
		ln(560, 9.2, cell{93, "CE - SSP - SSP - SECRETARIA DE SEGURANCA PUBLICA"}),
		ln(578, 13.0, cell{93, "99999"}),
		ln(596, 7.4, cell{93, "Órgão Emissor: SSP - SSP - SECRETARIA DE SEGURANCA PUBLICA"}),
		ln(607, 7.4, cell{93, "Estado: CE"}),
		ln(618, 7.4, cell{93, "Data de Emissão: 02/09/2011"}),
		ln(664, 12.0, cell{40, "Endereços (2)"}),
		ln(702, 8.3, cell{50, "R DAS FLORES, 10 - A"}),
		ln(715, 7.4, cell{50, "CENTRO - SOBRAL CE/CE - CEP: 62010000"}),
		ln(746, 8.3, cell{50, "RUA SEM NOME, 0"}),
		ln(759, 7.4, cell{50, "- FORTALEZA/CE - CEP: 60000000"}),
		footer(2),
	)
	p3 := append(header(),
		ln(106, 12.0, cell{40, "Telefones (2)"}),
		ln(146, 8.3, cell{50, "(88) 999990000"}, cell{538, "A"}),
		ln(177, 8.3, cell{50, "(85) 3100-0000"}),
		ln(178, 6.5, cell{537, "D"}),
		ln(248, 12.0, cell{40, "Pessoas Próximas (1)"}),
		ln(289, 8.3, cell{112, "Ciclano de Tal"}),
		ln(301, 7.4, cell{112, "Grau: Pai"}),
		ln(311, 7.4, cell{112, "CPF: -"}),
		ln(315, 5.6, cell{65, "Sem foto"}),
		ln(400, 12.0, cell{40, "Vacinas (1)"}),
		ln(430, 8.3, cell{50, "COVID-19 FICTICIA"}),
		footer(3),
	)
	return []opsreport.Page{page(p1...), page(p2...), page(p3...)}
}

func TestParseFixture(t *testing.T) {
	rep, err := Parse(fixture())
	if err != nil {
		t.Fatal(err)
	}
	check := func(name, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	check("name", rep.Name, "FULANO DE TAL FICTICIO")
	check("cpf", rep.CPF, "12345678909")
	check("mother", rep.MotherName, "BELTRANA DE TAL")
	check("father", rep.FatherName, "CICLANO DE TAL")
	check("birth", rep.BirthDate, "1990-03-02")
	check("gender", rep.Gender, "M")
	check("birthplace", rep.Birthplace, "SOBRAL/CE")
	check("nationality", rep.Nationality, "BRASILEIRO")
	check("format", rep.Format, FormatCPFReport)
	if !rep.CriminalFlag {
		t.Error("criminal_flag = false, want true")
	}
	if rep.GeneratedAt == nil || rep.GeneratedAt.UTC().Format("2006-01-02T15:04") != "2026-03-05T12:07" {
		t.Errorf("generated_at = %v", rep.GeneratedAt)
	}

	wantRG := []RG{{Number: "99999", Issuer: "SSP", State: "CE", IssuedOn: "2011-09-02"}}
	if !reflect.DeepEqual(rep.RGs, wantRG) {
		t.Errorf("rgs = %+v", rep.RGs)
	}
	if want := []string{"88999990000", "8531000000"}; !reflect.DeepEqual(rep.Phones, want) {
		t.Errorf("phones = %v, want %v", rep.Phones, want)
	}
	wantAddr := []Address{
		{Street: "R DAS FLORES", Number: "10", Complement: "A", Neighborhood: "CENTRO", City: "SOBRAL", State: "CE", CEP: "62010000"},
		{Street: "RUA SEM NOME", Number: "S/N", City: "FORTALEZA", State: "CE", CEP: "60000000"},
	}
	if !reflect.DeepEqual(rep.Addresses, wantAddr) {
		t.Errorf("addresses =\n%+v\nwant\n%+v", rep.Addresses, wantAddr)
	}
	wantCrim := []Record{
		{
			Category: "judicial", Source: SourceCPFReport,
			Title: "GUIA DE RECOLHIMENTO", Court: "TJCE", District: "FORTALEZA",
			Number: "0000001-11.2015.8.06.0001", Piece: "0000001-11.2015.8.06.0001.11000115",
			Date: "2018-05-16", Agency: "1ª VARA DAS EXECUÇOES PENAIS DA COMARCA DE FORTALEZA",
		},
		{
			Category: "judicial", Source: SourceCPFReport,
			Title: "MANDADO DE PRISÃO", Detail: "DECORRENTE DE CONDENAÇÃO NÃO TRANSITADA EM JULGADO",
			Court: "TJCE", District: "CAUCAIA",
			Number: "0000002-22.2016.8.06.0064", Piece: "0000002-22.2016.8.06.0064.01000312",
			Date: "2021-01-28", Reason: "PROGRESSÃO DE REGIME",
			Agency: "4ª VARA CRIMINAL DA COMARCA DE CAUCAIA",
		},
	}
	if !reflect.DeepEqual(rep.Criminal, wantCrim) {
		t.Errorf("criminal =\n%+v\nwant\n%+v", rep.Criminal, wantCrim)
	}
}

// Sem a seção "Identidades - RG", vale o RG da ficha; sem o pai na ficha,
// vale o de "Pessoas Próximas".
func TestParseFallbacks(t *testing.T) {
	p := page(append(header(),
		ln(219, 6.5, cell{57, "NOME DA MÃE"}),
		ln(228, 8.3, cell{57, "Beltrana de Tal"}),
		ln(325, 6.5, cell{57, "RG"}, cell{299, "CNS"}),
		ln(334, 8.3, cell{57, "12345 - SSP - SECRETARIA"}, cell{299, "700000000000000"}),
		ln(344, 8.3, cell{57, "PUBLICA/PI"}),
		ln(400, 12.0, cell{40, "Pessoas Próximas (1)"}),
		ln(430, 8.3, cell{112, "Ciclano de Tal"}),
		ln(442, 7.4, cell{112, "Grau: Pai"}),
		footer(1),
	)...)
	rep, err := Parse([]opsreport.Page{p})
	if err != nil {
		t.Fatal(err)
	}
	if want := []RG{{Number: "12345", Issuer: "SSP", State: "PI"}}; !reflect.DeepEqual(rep.RGs, want) {
		t.Errorf("rgs = %+v", rep.RGs)
	}
	if rep.FatherName != "CICLANO DE TAL" {
		t.Errorf("father = %q", rep.FatherName)
	}
}

func TestParseRejectsOtherPDF(t *testing.T) {
	p := page(ln(42, 8.3, cell{40, "RELATÓRIO DIÁRIO DE OCORRÊNCIAS"}))
	if _, err := Parse([]opsreport.Page{p}); err != ErrUnknownFormat {
		t.Fatalf("err = %v, want ErrUnknownFormat", err)
	}
}

// TestParseSample roda o parser em PDFs reais guardados fora do git
// (PERSONPDF_SAMPLE=/caminho/a.pdf:/caminho/b.pdf) e imprime o resultado.
// Precisa do pdftotext no PATH.
func TestParseSample(t *testing.T) {
	env := os.Getenv("PERSONPDF_SAMPLE")
	if env == "" {
		t.Skip("PERSONPDF_SAMPLE não definido")
	}
	for _, path := range strings.Split(env, ":") {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		pages, err := opsreport.Extract(context.Background(), data)
		if err != nil {
			t.Fatal(err)
		}
		rep, err := Parse(pages)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		out, _ := json.MarshalIndent(rep, "", "  ")
		t.Logf("%s\n%s", path, out)
	}
}
