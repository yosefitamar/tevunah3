package opsreport

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSplitBase(t *testing.T) {
	cases := []struct{ raw, city, cia, pel, bpm string }{
		{"CAUCAIA-1ªCIA/2ºBPRAIO", "CAUCAIA", "1ª CIA", "", "2º BPRAIO"},
		{"ACOPIARA-2ºPEL/2ªCIA/5ºBPRAIO", "ACOPIARA", "2ª CIA", "2º PEL", "5º BPRAIO"},
		{"JUAZEIRO DO NORTE-1ªCIA/5ºBPRAIO", "JUAZEIRO DO NORTE", "1ª CIA", "", "5º BPRAIO"},
		{"TIANGUÁ-3ºPEL/1ªCIA/2ºBPRAIO", "TIANGUÁ", "1ª CIA", "3º PEL", "2º BPRAIO"},
		{"1ªCIA/2° BPRAIO", "", "1ª CIA", "", "2º BPRAIO"},
		{"CAUCAIA", "CAUCAIA", "", "", ""},
	}
	for _, c := range cases {
		city, cia, pel, bpm := splitBase(c.raw)
		if city != c.city || cia != c.cia || pel != c.pel || bpm != c.bpm {
			t.Errorf("splitBase(%q) = %q %q %q %q", c.raw, city, cia, pel, bpm)
		}
	}
}

func TestUnitKey(t *testing.T) {
	for _, s := range []string{"2º BPRAIO", "2ºBPRAIO", "2° BPRAIO", "2 bpraio"} {
		if UnitKey(s) != "2BPRAIO" {
			t.Errorf("UnitKey(%q) = %q", s, UnitKey(s))
		}
	}
	if UnitKey("12º BPRAIO") == UnitKey("2º BPRAIO") {
		t.Error("12º e 2º não podem colidir")
	}
}

func TestParseOfficer(t *testing.T) {
	cases := []struct{ raw, reg, rank, num, name string }{
		{"30079116-2SGT 22410 SOARES", "30079116", "2SGT", "22410", "SOARES"},
		{"3057091X-CB 28129 HEMANUEL", "3057091X", "CB", "28129", "HEMANUEL"},
		{"30531515-3º SGT 28093 VALCINER", "30531515", "3ºSGT", "28093", "VALCINER"},
		{"30592417-CB 28440 R JÚNIOR", "30592417", "CB", "28440", "R JÚNIOR"},
		{"13495513-ST CARDOSO", "13495513", "ST", "", "CARDOSO"},
		{"10382718-2ºTEN DUARTE", "10382718", "2ºTEN", "", "DUARTE"},
		{"84395986-1º TEN QOPM JOELLYSSON", "84395986", "1º TEN QOPM", "", "JOELLYSSON"},
	}
	for _, c := range cases {
		o := parseOfficer(c.raw)
		if o.Registration != c.reg || o.Rank != c.rank || o.Number != c.num || o.WarName != c.name {
			t.Errorf("parseOfficer(%q) = %+v", c.raw, o)
		}
	}
	if o := parseOfficer("SOARES"); o.WarName != "" {
		t.Errorf("linha sem matrícula deveria ficar fora do padrão: %+v", o)
	}
}

func TestParseDecimal(t *testing.T) {
	for in, want := range map[string]float64{"4.3": 4.3, "4,3": 4.3, "1.234,5": 1234.5, "582": 582} {
		if got, ok := parseDecimal(in); !ok || got != want {
			t.Errorf("parseDecimal(%q) = %v %v", in, got, ok)
		}
	}
	if _, ok := parseDecimal(""); ok {
		t.Error("vazio não é número")
	}
}

// Rótulo e valor quebrados em linhas diferentes (como o PDF faz com
// "Tipo\nde Proced.:" e com o nome do delegado).
func TestReadLabelsWrapped(t *testing.T) {
	text := "Delegacia: JUAZEIRO DO NORTE-CE Delegado(a): Déborah Rogéria Gurgel Dos Santos Tipo de Proced.: TCO Nº do Proced.: 488-2865/2026"
	got := readLabels(text, procLabels, procLabelREs)
	if got["Delegado(a):"] != "Déborah Rogéria Gurgel Dos Santos" || got["Tipo de Proced.:"] != "TCO" ||
		got["Nº do Proced.:"] != "488-2865/2026" || got["Delegacia:"] != "JUAZEIRO DO NORTE-CE" {
		t.Errorf("readLabels = %#v", got)
	}
	// Rótulo ausente não desalinha os seguintes.
	got = readLabels("Delegacia: DMC Nº do Proced.: 1/2026", procLabels, procLabelREs)
	if got["Delegacia:"] != "DMC" || got["Nº do Proced.:"] != "1/2026" || got["Delegado(a):"] != "" {
		t.Errorf("readLabels com ausente = %#v", got)
	}
}

// Tabela com células vazias: só a posição x diz a qual coluna cada valor
// pertence.
func TestTableByPosition(t *testing.T) {
	w := func(x0, x1, y float64, s string) Word { return Word{X0: x0, X1: x1, Y0: y, Y1: y + 10, Text: s} }
	pages := []Page{{Words: []Word{
		w(57, 94, 10, "Natureza:"), w(99, 150, 10, "OUTROS"),
		w(57, 78, 22, "Base:"), w(82, 190, 22, "CAUCAIA-1ªCIA/2ºBPRAIO"),
		w(250, 330, 50, "Armas"), w(332, 390, 50, "Apreendidas"),
		w(100, 118, 70, "Tipo"), w(200, 230, 70, "Modelo"), w(300, 325, 70, "Marca"),
		w(390, 420, 70, "Calibre"), w(480, 490, 70, "Nº"), w(492, 515, 70, "Série"),
		w(92, 126, 90, "Revolver"), w(292, 333, 90, "TAURUS"), w(400, 410, 90, "38"), w(478, 520, 90, "GI63237"),
		w(92, 126, 105, "Carabina"),
		w(57, 90, 130, "Delegacia:"), w(95, 110, 130, "DMC"),
	}}}
	r := Parse(pages)
	if len(r.Occurrences) != 1 {
		t.Fatalf("ocorrências = %d", len(r.Occurrences))
	}
	ws := r.Occurrences[0].Weapons
	if len(ws) != 2 {
		t.Fatalf("armas = %+v", ws)
	}
	if ws[0] != (Weapon{Kind: "REVOLVER", Brand: "TAURUS", Caliber: "38", Serial: "GI63237"}) {
		t.Errorf("revólver = %+v", ws[0])
	}
	if ws[1] != (Weapon{Kind: "CARABINA"}) {
		t.Errorf("carabina = %+v", ws[1])
	}
}

// Relatório real de 26/09/2026 (33 páginas, 29 ocorrências de 8 batalhões).
func TestParseRealReport(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext (poppler-utils) ausente")
	}
	b, err := os.ReadFile("testdata/relatorio_2026-09-26.pdf")
	if err != nil {
		t.Fatal(err)
	}
	pages, err := Extract(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	r := Parse(pages)
	if got := r.ReportDate.Format("2006-01-02"); got != "2026-09-26" {
		t.Errorf("data do relatório = %s", got)
	}
	if len(r.Occurrences) != 29 {
		t.Fatalf("ocorrências = %d, esperado 29", len(r.Occurrences))
	}
	if len(r.Warnings) != 0 {
		t.Errorf("avisos do relatório: %v", r.Warnings)
	}
	for _, o := range r.Occurrences {
		if len(o.Warnings) != 0 {
			t.Errorf("p%d avisos: %v", o.Page, o.Warnings)
		}
		if o.CIOPS == "" || o.OccurredOn.IsZero() || len(o.Natures) == 0 || len(o.Officers) == 0 || o.Narrative == "" {
			t.Errorf("p%d incompleta: %+v", o.Page, o)
		}
	}

	mine := r.ForUnit("2º BPRAIO")
	if len(mine) != 3 {
		t.Fatalf("2º BPRAIO = %d ocorrências, esperado 3", len(mine))
	}
	// Um histórico do 1º BPRAIO poderia citar "2º BPRAIO"; o recorte é pela Base.
	for _, o := range mine {
		if o.BPM != "2º BPRAIO" || o.CIA != "1ª CIA" || o.BaseCity != "CAUCAIA" {
			t.Errorf("unidade de %s = %q/%q/%q", o.CIOPS, o.BaseCity, o.CIA, o.BPM)
		}
	}

	a := mine[0]
	if a.CIOPS != "M20260705632" || a.Teams != "RAIO 01; RAIO 02" || a.StartTime != "12:15" || a.EndTime != "14:30" {
		t.Errorf("identificação: %+v", a)
	}
	if a.PlaceAddress != "Rua Fabiano De Cristo, s/n" || a.PlaceNeighborhood != "MARECHAL RONDON" || a.PlaceCity != "CAUCAIA" {
		t.Errorf("local: %q %q %q", a.PlaceAddress, a.PlaceNeighborhood, a.PlaceCity)
	}
	if a.PoliceStation != "DMC" || a.Delegate != "Roberto De Sousa Leon" || a.ProcedureType != "BO" || a.ProcedureNumber != "939-7635/2026" {
		t.Errorf("procedimento: %q %q %q %q", a.PoliceStation, a.Delegate, a.ProcedureType, a.ProcedureNumber)
	}
	if len(a.Weapons) != 1 || a.Weapons[0] != (Weapon{Kind: "REVOLVER", Brand: "TAURUS", Caliber: "38", Serial: "GI63237"}) {
		t.Errorf("armas: %+v", a.Weapons)
	}
	if len(a.Officers) != 4 || a.Officers[2].WarName != "HENRIQUE COSTA" || a.Officers[2].Registration != "3055681X" {
		t.Errorf("composição: %+v", a.Officers)
	}

	b2 := mine[1]
	if len(b2.Natures) != 2 || b2.PlaceCity != "FORTALEZA" {
		t.Errorf("naturezas/cidade: %v %q", b2.Natures, b2.PlaceCity)
	}
	if len(b2.Weapons) != 1 || b2.Weapons[0] != (Weapon{Kind: "SUBMETRALHADORA", Model: "ARTESANAL", Caliber: "9MM"}) {
		t.Errorf("submetralhadora: %+v", b2.Weapons)
	}
	if len(b2.Drugs) != 1 || b2.Drugs[0].Description != "MACONHA" || *b2.Drugs[0].Grams != 582 || *b2.Drugs[0].Packages != 0 {
		t.Errorf("drogas: %+v", b2.Drugs)
	}

	if c := mine[2]; len(c.Weapons) != 1 || c.Weapons[0] != (Weapon{Kind: "CARABINA"}) {
		t.Errorf("carabina: %+v", c.Weapons)
	}

	byPage := map[int]Occurrence{}
	for _, o := range r.Occurrences {
		byPage[o.Page] = o
	}
	// Natureza quebrada em duas linhas.
	if n := byPage[28].Natures; len(n) != 3 || n[2] != "TRÁFICO DE DROGAS" {
		t.Errorf("natureza em duas linhas: %v", n)
	}
	// "(ÓBITO)" sai do nome e vira anotação.
	if p := byPage[28].People[0]; p.Name != "WESLEY DONINETE NUNES" || p.Note != "ÓBITO" {
		t.Errorf("anotação: %+v", p)
	}
	// Testemunha com endereço; idade 0 = não informada.
	if p := byPage[16].People[1]; p.Role != RoleTestemunha || p.Age == nil || *p.Age != 26 ||
		!strings.HasPrefix(p.Address, "CORREDOR DOS DOMINGOS") {
		t.Errorf("testemunha: %+v", p)
	}
	if p := byPage[8].People[0]; p.Age != nil {
		t.Errorf("idade 0 deveria ser nil: %+v", p)
	}
	// Ocorrência que continua na página seguinte (sem cabeçalho).
	if o := byPage[6]; o.ProcedureNumber != "939-7632/2026" || !strings.Contains(o.SeizedObjects, "Balança de precisão") {
		t.Errorf("continuação de página: %q %q", o.ProcedureNumber, o.SeizedObjects)
	}
	// Local de abordagem com rótulo quebrado ("Cidade\nAbordagem:").
	if o := byPage[24]; o.ApproachNeighborhood != "GITIRANA" || o.ApproachCity != "ARACOIABA" {
		t.Errorf("abordagem: %q %q %q", o.ApproachAddress, o.ApproachNeighborhood, o.ApproachCity)
	}
	// O fecho do documento não contamina a última composição.
	last := r.Occurrences[len(r.Occurrences)-1]
	if w := last.Officers[len(last.Officers)-1].WarName; w != "F SOARES" {
		t.Errorf("último policial = %q", w)
	}
}
