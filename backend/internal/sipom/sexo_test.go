package sipom

import "testing"

func testNames() *NameSex {
	return &NameSex{byName: map[string][2]int32{
		"maria":     {11_000_000, 10_000},
		"francisco": {5_000, 1_700_000},
		"ariel":     {2_755, 27_128}, // 91% masculino: não decide
		"zuleica":   {12, 0},         // raro demais
	}}
}

func TestNameSexGuess(t *testing.T) {
	n := testNames()
	for name, want := range map[string]int{
		"MARIA": SexoFeminino, "Francisco": SexoMasculino, "ARIEL": 0, "ZULEICA": 0, "JERLLYSON": 0,
	} {
		if got := n.Guess(name); got != want {
			t.Errorf("Guess(%q) = %d, quer %d", name, got, want)
		}
	}
}

func TestSuggestSexo(t *testing.T) {
	names := testNames()
	terms := &TermMap{byField: map[string]map[string]int{TermSexoNome: {"jerllyson": SexoMasculino, "ariel": SexoFeminino}}}
	cases := []struct {
		name  string
		in    SexoInput
		sexo  int
		fonte string
	}{
		{"censo", SexoInput{Name: "MARIA DAS DORES", Role: "ACUSADO"}, SexoFeminino, SexoFonteNome},
		// O histórico no masculino genérico não contraria o nome.
		{"censo vence histórico", SexoInput{Name: "MARIA SILVA", Role: "ACUSADO", Single: true,
			Narrative: "o indivíduo foi conduzido"}, SexoFeminino, SexoFonteNome},
		{"aprendido", SexoInput{Name: "JERLLYSON SOUZA", Role: "ACUSADO"}, SexoMasculino, SexoFonteAprendido},
		{"aprendido onde o censo não decide", SexoInput{Name: "ARIEL", Role: "VÍTIMA"}, SexoFeminino, SexoFonteAprendido},
		{"histórico no feminino", SexoInput{Name: "NAGILA X", Role: "ACUSADO", Single: true,
			Narrative: "A MESMA foi conduzida à delegacia"}, SexoFeminino, SexoFonteHistorico},
		{"histórico com mais de um identificado", SexoInput{Name: "NAGILA X", Role: "ACUSADO",
			Narrative: "a acusada foi presa"}, 0, ""},
		{"histórico de testemunha", SexoInput{Name: "NAGILA X", Role: "TESTEMUNHA", Single: true,
			Narrative: "a acusada foi presa"}, 0, ""},
		{"sem pista", SexoInput{Name: "NAGILA X", Role: "ACUSADO", Single: true,
			Narrative: "a motocicleta foi abordada"}, 0, ""},
		{"não identificado", SexoInput{Name: "NÃO IDENTIFICADO", Role: "ACUSADO"}, 0, ""},
	}
	for _, c := range cases {
		s, f := SuggestSexo(c.in, names, terms)
		if s != c.sexo || f != c.fonte {
			t.Errorf("%s: (%d, %q), quer (%d, %q)", c.name, s, f, c.sexo, c.fonte)
		}
	}
}
