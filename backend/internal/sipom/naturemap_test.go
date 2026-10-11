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
		{Source: "APREENSÃO DE ARMA", NaturezaID: ip(76), Confianca: ConfComArma, Prioridade: 47},
		{Source: "APREENSÃO DE ENTORPECENTE", NaturezaID: ip(10), Confianca: ConfComDroga, Prioridade: 48},
		{Source: "OUTROS", NaturezaID: nil, Prioridade: 99},
	} {
		m.rules[NatureKey(r.Source)] = r
	}
	arma, droga := Evidence{Firearm: true}, Evidence{Drug: true}
	cases := []struct {
		natures   []string
		ev        Evidence
		id        int // 0 = nenhuma
		suggested bool
	}{
		// Regra condicionada à evidência: direta com a apreensão na ficha,
		// sugerida sem ela (ou com a evidência de outro tipo).
		{[]string{"APREENSÃO DE ARMA"}, arma, 76, false},
		{[]string{"APREENSÃO DE ARMA"}, Evidence{}, 76, true},
		{[]string{"APREENSÃO DE ARMA"}, droga, 76, true},
		{[]string{"APREENSÃO DE ENTORPECENTE"}, droga, 10, false},
		{[]string{"APREENSÃO DE ENTORPECENTE"}, arma, 10, true},
		// A evidência não promove regra sugerida comum.
		{[]string{"APRESENTAÇÃO E APREENSÃO DE DROGAS"}, droga, 10, true},
		{[]string{"TRÁFICO DE DROGAS"}, Evidence{}, 10, false},
		{[]string{"APRESENTAÇÃO E APREENSÃO DE ARMA DE FOGO", "APRESENTAÇÃO E APREENSÃO DE DROGAS"}, Evidence{}, 76, true},
		{[]string{"APRESENTAÇÃO E APREENSÃO DE DROGAS", "TRÁFICO DE DROGAS"}, Evidence{}, 10, false},
		// Destinos diferentes na mesma ficha: sugerida mesmo com regra direta
		// e com a apreensão na ficha — o analista decide qual prevalece.
		{[]string{"TRÁFICO DE DROGAS", "APREENSÃO DE ARMA"}, Evidence{Firearm: true, Drug: true}, 10, true},
		{[]string{"APREENSÃO DE ARMA", "APREENSÃO DE ENTORPECENTE"}, Evidence{Firearm: true, Drug: true}, 76, true},
		// Linha sem destino não concorre; natureza fora do de-para, sim.
		{[]string{"TRÁFICO DE DROGAS", "OUTROS"}, Evidence{}, 10, false},
		{[]string{"TRÁFICO DE DROGAS", "NATUREZA NOVA"}, Evidence{}, 10, true},
		{[]string{"OUTROS"}, Evidence{}, 0, false},
		{[]string{"NATUREZA NOVA"}, Evidence{}, 0, false},
		{nil, Evidence{}, 0, false},
	}
	for _, c := range cases {
		got := m.Choose(c.natures, c.ev)
		gid := 0
		if got.NaturezaID != nil {
			gid = *got.NaturezaID
		}
		if gid != c.id || got.Suggested != c.suggested {
			t.Errorf("Choose(%v, %+v) = %d suggested=%v, want %d %v", c.natures, c.ev, gid, got.Suggested, c.id, c.suggested)
		}
	}
}

func TestIsFirearm(t *testing.T) {
	for kind, want := range map[string]bool{
		"PISTOLA": true, "Revólver": true, "ESPINGARDA": true, "SUBMETRALHADORA": true, "ARTESANAL": true,
		"": false, "FACA": false, "ARMA BRANCA": false, "SIMULACRO DE PISTOLA": false,
		"HTTPS://SGR.PM.CE.GOV.BR/IMPRESSAO_OCORRENCIA_OPERACIONAL/": false, "02/01/26, 06:25": false,
	} {
		if got := IsFirearm(kind); got != want {
			t.Errorf("IsFirearm(%q) = %v, want %v", kind, got, want)
		}
	}
}
