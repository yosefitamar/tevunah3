package incidents

import (
	"strings"
	"testing"
)

// Relatório real de grupo operacional, colado sem qualquer edição — é assim
// que o texto chega ao analista.
const sampleReport = `RELATÓRIO DE CVLI
1ªCIA/27ºBPM/UBAJARA
NATUREZA: HOMICIDIO POR ARMA DE FOGO
DATA: 17AGO2026  
HORÁRIO: 17h35
ENDEREÇO: Bairro São Sebastião, UBAJARA-CE
LINK NO GOOGLE MAPS:  https://maps.app.goo.gl/TDUFg2LSqbvamRkm9?g_st=ic
AREA DE ATUAÇÃO DA ORCRIM DO LOCAL DO CRIME: CV

1. VÍTIMA
ERISVALDO LIMA DOS SANTOS
Mãe: Antônia Bernardo De Lima
DN: 12/05/2005


2. CENÁRIO
Segundo informações de populares, por volta das 10h, foram ouvidos estampidos semelhantes a disparos de arma. Posteriormente, a vítima foi encontrada sem vida, apresentando lesões provocadas por disparos de arma de fogo, sendo constatadas, preliminarmente, aproximadamente duas perfurações. O corpo estava localizado em uma região de matagal, nas proximidades da zona urbana. Ao lado da vítima foi encontrada uma motocicleta Honda CG Fan 150, de cor preta, placa NRD9J36, a qual possuía registro de roubo/furto.

3. ENVOLVIDOS
Até o momento, não há informações acerca da autoria do crime. Contudo, há informações de que a vítima possuía envolvimento com a ORCRIM CV e, possivelmente, teria “rasgado a camisa”, passando a integrar ou colaborar com a ORCRIM TCP. 

4. RISCO DE REVIDE
Há possibilidade de revide, considerando o conflito entre as cidades vizinhas IBIAPINA entre as ORCRIMS TCP e CV.`

func TestParseReport_RelatorioCVLI(t *testing.T) {
	got := ParseReport(sampleReport)

	if got.Type != TypeHomicidio {
		t.Errorf("Type = %q, quero %q", got.Type, TypeHomicidio)
	}
	if got.Means != MeansPAF {
		t.Errorf("Means = %q, quero %q", got.Means, MeansPAF)
	}
	if got.OccurredOn != "2026-08-17" {
		t.Errorf("OccurredOn = %q, quero 2026-08-17", got.OccurredOn)
	}
	// O cabeçalho manda: o "por volta das 10h" do cenário é hora de estampido
	// ouvido por populares, não o horário do registro.
	if got.OccurredTime != "17:35" {
		t.Errorf("OccurredTime = %q, quero 17:35", got.OccurredTime)
	}
	if got.City != "UBAJARA" {
		t.Errorf("City = %q, quero UBAJARA", got.City)
	}
	if got.Neighborhood != "SÃO SEBASTIÃO" {
		t.Errorf("Neighborhood = %q, quero SÃO SEBASTIÃO", got.Neighborhood)
	}
	if got.MapsURL != "https://maps.app.goo.gl/TDUFg2LSqbvamRkm9?g_st=ic" {
		t.Errorf("MapsURL = %q", got.MapsURL)
	}
	// Short link não carrega coordenadas: quem resolve é o handler.
	if got.Latitude != nil || got.Longitude != nil {
		t.Errorf("coordenadas = %v/%v, quero nil (short link)", got.Latitude, got.Longitude)
	}

	if len(got.People) != 1 {
		t.Fatalf("People = %d, quero 1: %+v", len(got.People), got.People)
	}
	v := got.People[0]
	if v.Name != "ERISVALDO LIMA DOS SANTOS" {
		t.Errorf("nome = %q", v.Name)
	}
	if v.MotherName != "ANTÔNIA BERNARDO DE LIMA" {
		t.Errorf("mãe = %q", v.MotherName)
	}
	if v.DateOfBirth != "2005-05-12" {
		t.Errorf("DN = %q", v.DateOfBirth)
	}
	if v.Role != RoleVitima {
		t.Errorf("papel = %q", v.Role)
	}

	// Descrição começa no cenário e carrega envolvidos e risco de revide —
	// não há outro campo no cadastro que os guarde.
	if !strings.HasPrefix(got.Description, "2. CENÁRIO") {
		t.Errorf("descrição não começa no cenário: %.60q", got.Description)
	}
	for _, want := range []string{"MATAGAL", "3. ENVOLVIDOS", "4. RISCO DE REVIDE", "IBIAPINA"} {
		if !strings.Contains(got.Description, want) {
			t.Errorf("descrição não contém %q", want)
		}
	}
	if strings.Contains(got.Description, "ERISVALDO") {
		t.Error("descrição não deve repetir o bloco da vítima")
	}

	// O que o modelo não comporta vira aviso, não silêncio.
	joined := strings.Join(got.Warnings, " | ")
	if !strings.Contains(joined, "NRD9J36") {
		t.Errorf("faltou aviso da placa: %q", joined)
	}
	if !strings.Contains(joined, "ORCRIM") {
		t.Errorf("faltou aviso da ORCRIM: %q", joined)
	}
}

func TestParseReport_TextoNaoRelatorio(t *testing.T) {
	// Mensagem solta de grupo: nada é reconhecido, nada é chutado.
	got := ParseReport("bom dia, alguém tem notícia do caso de ontem?")
	if got.Type != "" || got.OccurredOn != "" || got.City != "" || len(got.People) != 0 {
		t.Errorf("texto sem rótulo rendeu campos: %+v", got)
	}
}

func TestParseReportDate(t *testing.T) {
	cases := map[string]string{
		"17AGO2026":            "2026-08-17",
		"17 AGO 26":            "2026-08-17",
		"1 de setembro de 2025": "2025-09-01",
		"17/08/2026":           "2026-08-17",
		"07.03.2024":           "2024-03-07",
		"31/02/2026":           "", // data impossível
		"32AGO2026":            "",
		"17XXX2026":            "",
		"":                     "",
	}
	for in, want := range cases {
		if got := parseReportDate(in); got != want {
			t.Errorf("parseReportDate(%q) = %q, quero %q", in, got, want)
		}
	}
}

func TestParseReportTime(t *testing.T) {
	cases := map[string]string{
		"17h35":       "17:35",
		"17:35":       "17:35",
		"08h":         "08:00",
		"por volta das 22h10": "22:10",
		"25h00":       "",
		"17h75":       "",
		"":            "",
	}
	for in, want := range cases {
		if got := parseReportTime(in); got != want {
			t.Errorf("parseReportTime(%q) = %q, quero %q", in, got, want)
		}
	}
}

func TestParsePlace(t *testing.T) {
	cases := []struct {
		addr, city, hood string
		wantWarn         bool
	}{
		{"Bairro São Sebastião, UBAJARA-CE", "UBAJARA", "SÃO SEBASTIÃO", false},
		{"Rua José de Alencar, 120, Centro, Sobral/CE", "SOBRAL", "CENTRO", true},
		{"Conjunto Palmeiras, Fortaleza", "FORTALEZA", "CONJUNTO PALMEIRAS", false},
		{"Bairro Centro, Xique-Xique-BA", "", "CENTRO", false}, // fora do Ceará: município não casa
		{"", "", "", false},
	}
	for _, c := range cases {
		city, hood, warn := parsePlace(c.addr)
		if city != c.city || hood != c.hood {
			t.Errorf("parsePlace(%q) = (%q, %q), quero (%q, %q)", c.addr, city, hood, c.city, c.hood)
		}
		if (warn != "") != c.wantWarn {
			t.Errorf("parsePlace(%q) aviso = %q, queria aviso? %v", c.addr, warn, c.wantWarn)
		}
	}
}

func TestDetectMeans(t *testing.T) {
	cases := []struct{ nature, scenario, means, detail string }{
		{"HOMICIDIO POR ARMA DE FOGO", "", MeansPAF, ""},
		{"HOMICÍDIO", "vítima esfaqueada no tórax", MeansArmaBranca, ""},
		{"HOMICÍDIO", "corpo carbonizado dentro do veículo", MeansOutros, "CARBONIZAÇÃO"},
		{"HOMICÍDIO", "sem detalhes das lesões", MeansUnknown, ""},
		// A natureza tem prioridade sobre o cenário.
		{"HOMICIDIO POR ARMA BRANCA", "houve também disparos de arma de fogo contra a casa",
			MeansArmaBranca, ""},
	}
	for _, c := range cases {
		m, d := detectMeans(c.nature, c.scenario)
		if m != c.means || d != c.detail {
			t.Errorf("detectMeans(%q, %q) = (%q, %q), quero (%q, %q)",
				c.nature, c.scenario, m, d, c.means, c.detail)
		}
	}
}

func TestCoordsFromText(t *testing.T) {
	cases := []struct {
		in       string
		lat, lng float64
		ok       bool
	}{
		{"https://www.google.com/maps/place/x/@-3.8541234,-40.9212345,17z/", -3.8541234, -40.9212345, true},
		{"...!3d-3.8541234!4d-40.9212345...", -3.8541234, -40.9212345, true},
		{"https://maps.google.com/?q=-3.85412,-40.92123", -3.85412, -40.92123, true},
		{"COORDENADAS: -3.85412, -40.92123", -3.85412, -40.92123, true},
		{"https://maps.app.goo.gl/TDUFg2LSqbvamRkm9?g_st=ic", 0, 0, false},
		{"nada aqui", 0, 0, false},
		// Fora do Brasil = lixo de parsing: o cadastro só aceita municípios
		// do Ceará, então isso nunca é um ponto legítimo.
		{"COORDENADAS: 68.10751841316, -38.6072576", 0, 0, false},
	}
	for _, c := range cases {
		lat, lng, ok := coordsFromText(c.in)
		if ok != c.ok || (ok && (lat != c.lat || lng != c.lng)) {
			t.Errorf("coordsFromText(%q) = (%v, %v, %v), quero (%v, %v, %v)",
				c.in, lat, lng, ok, c.lat, c.lng, c.ok)
		}
	}
}

func TestCoordsFromPage_IgnoraFloatSolto(t *testing.T) {
	// Trecho representativo do HTML do Maps: floats por toda parte, e a
	// coordenada real só no marcador !3d!4d.
	html := `["x",68.10751841316,-38.6072576],{"a":1},null,` +
		`[[[-3.8541234,-40.9212345]]],!3d-5.0842907!4d-37.8631109`
	lat, lng, ok := coordsFromPage(html)
	if !ok {
		t.Fatal("não achou a coordenada ancorada")
	}
	if lat != -5.0842907 || lng != -37.8631109 {
		t.Errorf("coordenadas = %v/%v — pegou float solto do HTML", lat, lng)
	}
	// Sem marcador ancorado, o HTML não rende coordenada nenhuma.
	if _, _, ok := coordsFromPage(`["x",68.10751841316,-38.6072576]`); ok {
		t.Error("float solto do HTML virou coordenada")
	}
}

func TestParsePeople_MultiplasVitimasEAcusadoQualificado(t *testing.T) {
	report := `NATUREZA: DUPLO HOMICÍDIO POR ARMA DE FOGO
DATA: 03MAR2026

1. VÍTIMAS
JOAO DA SILVA SAURO
Mãe: Maria da Silva
DN: 01/01/1990

PEDRO ALVES COSTA
Vulgo: PEDRINHO
DN: 15/06/1988

3. ENVOLVIDOS
Suspeita-se da participação de integrantes da facção rival.

CARLOS EDUARDO MOTA
Mãe: Rita Mota
`
	got := ParseReport(report)
	if len(got.People) != 3 {
		t.Fatalf("People = %d, quero 3: %+v", len(got.People), got.People)
	}
	if got.People[0].Role != RoleVitima || got.People[1].Role != RoleVitima {
		t.Errorf("papéis das vítimas: %+v", got.People[:2])
	}
	if got.People[1].Alias != "PEDRINHO" {
		t.Errorf("vulgo = %q", got.People[1].Alias)
	}
	acusado := got.People[2]
	if acusado.Name != "CARLOS EDUARDO MOTA" || acusado.Role != RoleAcusado {
		t.Errorf("acusado = %+v", acusado)
	}
	// A prosa da seção de envolvidos não pode virar pessoa.
	for _, p := range got.People {
		if strings.HasPrefix(p.Name, "SUSPEITA") {
			t.Errorf("prosa virou pessoa: %+v", p)
		}
	}
}

func TestFindPlates(t *testing.T) {
	cases := map[string][]string{
		"motocicleta placa NRD9J36 com registro de roubo": {"NRD9J36"},
		"veículo de placa ABC-1234":                       {"ABC1234"},
		// Falso positivo real: "DAS 2H00" tem a forma de placa se o espaço
		// entre letras e números for tolerado.
		"por volta das 2h00, a vítima foi encontrada": nil,
		"recolhidas 14 cápsulas de calibre 9mm":       nil,
	}
	for in, want := range cases {
		got := findPlates(in)
		if len(got) != len(want) {
			t.Errorf("findPlates(%q) = %v, quero %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("findPlates(%q) = %v, quero %v", in, got, want)
			}
		}
	}
}

func TestFindCityMention(t *testing.T) {
	cases := map[string]string{
		"Barragem das Pedrinhas - Estr. Pasta-Morros, 136 - Limoeiro do Norte, CE, 62930-000": "LIMOEIRO DO NORTE",
		"Rua Coronel Bia, 2597, Centro, Cascavel/CE":                                          "CASCAVEL",
		"Avenida sem cidade nenhuma":                                                          "",
	}
	for in, want := range cases {
		if got := FindCityMention(in); got != want {
			t.Errorf("FindCityMention(%q) = %q, quero %q", in, got, want)
		}
	}
}
