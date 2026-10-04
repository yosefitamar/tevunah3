package sipom

import "testing"

func TestNatureKey(t *testing.T) {
	same := [][2]string{
		{"FURTO - ART. 155/CPB", "FURTO"},
		{"ORGANIZAÇÃO CRIMINOSA - ART. 2°/ LEI Nº 12.850/2013", "organizacao criminosa"},
		{"TENTATIVA DE HOMICIDIO - ART. 121 C/C ART. 14, II/CPB", "Tentativa de Homicídio"},
		{"EMBRIAGUEZ AO VOLANTE - ART. 306/CTB", "EMBRIAGUEZ AO VOLANTE"},
	}
	for _, p := range same {
		if NatureKey(p[0]) != NatureKey(p[1]) {
			t.Errorf("NatureKey(%q)=%q != NatureKey(%q)=%q", p[0], NatureKey(p[0]), p[1], NatureKey(p[1]))
		}
	}
	// "APREENSÃO DE ARMA" não pode virar "APREENSÃO" — só "- ART" corta.
	if NatureKey("APRESENTAÇÃO E APREENSÃO DE ARMA DE FOGO") == NatureKey("APRESENTAÇÃO E APREENSÃO") {
		t.Error("cortou texto que não é citação legal")
	}
}

func ip(i int) *int { return &i }

func TestChoose(t *testing.T) {
	m := &NatureMap{rules: map[string]NatureRule{}}
	for _, r := range []NatureRule{
		{Source: "TRÁFICO DE DROGAS", NaturezaID: ip(10), Confianca: ConfDireta, Prioridade: 35},
		{Source: "APRESENTAÇÃO E APREENSÃO DE ARMA DE FOGO", NaturezaID: ip(76), Confianca: ConfSugerida, Prioridade: 45},
		{Source: "APRESENTAÇÃO E APREENSÃO DE DROGAS", NaturezaID: ip(10), Confianca: ConfSugerida, Prioridade: 46},
		{Source: "OUTROS", NaturezaID: nil, Prioridade: 99},
	} {
		m.rules[NatureKey(r.Source)] = r
	}
	cases := []struct {
		natures   []string
		id        int // 0 = nenhuma
		suggested bool
	}{
		{[]string{"TRÁFICO DE DROGAS"}, 10, false},
		{[]string{"APRESENTAÇÃO E APREENSÃO DE ARMA DE FOGO", "APRESENTAÇÃO E APREENSÃO DE DROGAS"}, 76, true},
		{[]string{"APRESENTAÇÃO E APREENSÃO DE DROGAS", "TRÁFICO DE DROGAS"}, 10, false},
		{[]string{"OUTROS"}, 0, false},
		{[]string{"NATUREZA NOVA"}, 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		got := m.Choose(c.natures)
		gid := 0
		if got.NaturezaID != nil {
			gid = *got.NaturezaID
		}
		if gid != c.id || got.Suggested != c.suggested {
			t.Errorf("Choose(%v) = %d suggested=%v, want %d %v", c.natures, gid, got.Suggested, c.id, c.suggested)
		}
	}
}
