package sipom

import (
	"strings"
	"testing"
)

func TestCaliberKey(t *testing.T) {
	for in, want := range map[string]string{
		".38": "38", "38": "38", "38 SPL": "38", ".380": "380", "9mm": "9", "9MM": "9", "9x19": "9",
		"5.56": "556", "5,56": "556", "556": "556", "7.62": "762", ".40 S&W": "40", "12": "12", "": "",
	} {
		if got := caliberKey(in); got != want {
			t.Errorf("caliberKey(%q) = %q, quer %q", in, got, want)
		}
	}
}

func TestDistritoNumber(t *testing.T) {
	for in, want := range map[string]int{
		"22° DP": 22, "22º DP": 22, "22 DP": 22, "22o DP": 22, "DP 22": 22, "2° DISTRITO": 2,
		"DMC": 0, "": 0, "DELEGACIA METROPOLITANA DE CAUCAIA": 0,
	} {
		if got := distritoNumber(in); got != want {
			t.Errorf("distritoNumber(%q) = %d, quer %d", in, got, want)
		}
	}
}

func TestTokensSubsequence(t *testing.T) {
	for _, c := range []struct {
		want, have string
		ok         bool
	}{
		{"Ítalo Renno Alves", "ITALO RENNO ALVES FEITOSA", true},
		{"Eduardo Coutinho do Rego", "EDUARDO COUTINHO DO REGO", true},
		{"R. de Sousa Leon", "ROBERTO DE SOUSA LEON", true},
		{"Roberto Leon", "ROBERTO DE SOUSA LEON", true},
		{"Alves Feitosa", "ITALO RENNO ALVES FEITOSA", false}, // não começa pelo primeiro nome
		{"Italo Alves", "ITALO RENNO ALVES FEITOSA", true},
		{"Italo Souza", "ITALO RENNO ALVES FEITOSA", false},
		{"", "ITALO RENNO ALVES FEITOSA", false},
	} {
		if got := tokensSubsequence(nameTokens(c.want), nameTokens(c.have), sameWord); got != c.ok {
			t.Errorf("%q em %q = %v, quer %v", c.want, c.have, got, c.ok)
		}
	}
}

func TestMatchDelegado(t *testing.T) {
	c := &Catalog{}
	for i, n := range []string{
		"CAIO HENRIQUE DE SOUZA XAVIER", "CARLOS HENRIQUE DE SOUZA XAVIER", "ITALO RENNO ALVES FEITOSA",
		"ALEXANDRE VIEIRA GALINDO", "JOSE CLEOFILO RODRIGUES MELO", "PRISCILA KARINE LOPES DE OLIVEIRA",
		"DEBORA CAVALCANTE DE FALCONERI TEIXEIRA", "AUGUSTO SOARES FLAVIO", "MARIA LIMA", "MARIA LIRA",
		"THIAGO REZENDE COSTA", "WAGNER MELLO",
	} {
		c.delegadoList = append(c.delegadoList, Delegado{ID: i + 1, Nome: n})
	}
	for name, want := range map[string]int{
		"Caio Henrique de Sousa Xavier":          1, // s/z
		"Carlos Henrique De Souza Xavier":        2,
		"ÍTALO RENO ALVES FEITOSA":               3, // letra dobrada
		"Italo Reno Alves":                       3, // abreviado e com outro som
		"ALEXANDRE VIEIRA GALINHO":               4, // uma letra trocada
		"JOSE CLEOFILO RODRUIGUES MELO":          5, // uma letra a mais
		"Pricila Karine Lopes de Oliveira":       6, // uma letra a menos
		"Debora Cavalcante De Falconeri Texeira": 7, // ei/e
		"ALGUSTO SOARES FLÁVIO":                  8,
		"Tiago Resende":                          11, // h mudo, s/z
		"Vagner Melo":                            12, // w/v
		"Maria Lima":                             9,  // exato vence o parecido
		"Maria Lisa":                             0,  // palavra curta não casa por aproximação
		"Fulano de Tal":                          0,
	} {
		if got, _ := c.MatchDelegado(name); got != want {
			t.Errorf("MatchDelegado(%q) = %d, quer %d", name, got, want)
		}
	}
	// Abreviado que serve a dois: não escolhe, devolve os candidatos.
	if id, cands := c.MatchDelegado("C. Henrique Xavier"); id != 0 || len(cands) != 2 {
		t.Errorf("ambíguo: id=%d cands=%v", id, cands)
	}
}

func TestReProcNumber(t *testing.T) {
	for in, want := range map[string][3]string{
		"939-7635/2026":     {"939", "7635", "2026"},
		"7635/2026":         {"", "7635", "2026"},
		"939 - 7635 / 2026": {"939", "7635", "2026"},
		"7635":              {"", "", ""},
		"":                  {"", "", ""},
	} {
		m := reProcNumber.FindStringSubmatch(in)
		var got [3]string
		if m != nil {
			got = [3]string{m[1], m[2], m[3]}
		}
		if got != want {
			t.Errorf("%q: %v, quer %v", in, got, want)
		}
	}
}

func TestSplitProc(t *testing.T) {
	for in, want := range map[string][]string{
		"AI;IP":                     {"AI", "IP"},
		"939-874/2026;939-64/2026":  {"939-874/2026", "939-64/2026"},
		"205-13/2026; 205-01/2026":  {"205-13/2026", "205-01/2026"},
		"939-874/2026, 939-64/2026": {"939-874/2026", "939-64/2026"},
		"IP":                        {"IP"},
		" ; ":                       nil,
		"":                          nil,
	} {
		got := splitProc(in)
		if len(got) != len(want) {
			t.Errorf("splitProc(%q) = %v, quer %v", in, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("splitProc(%q) = %v, quer %v", in, got, want)
			}
		}
	}
}

func TestResolveProcedureMulti(t *testing.T) {
	c := &Catalog{}
	type item struct {
		proc        int // 0 = não reconhecido
		numero, ano string
	}
	for _, tc := range []struct {
		typ, num string
		items    []item
		pending  []string
	}{
		// Maiores e menor na mesma ocorrência: IP e ato infracional.
		{"AI;IP", "939-874/2026;939-64/2026", []item{{ProcAtoInfracional, "874", "2026"}, {ProcIP, "64", "2026"}}, nil},
		{"IP, TCO", "205-13/2026, 205-01/2026", []item{{ProcIP, "13", "2026"}, {ProcTCO, "01", "2026"}}, nil},
		{"IP", "939-7635/2026", []item{{ProcIP, "7635", "2026"}}, nil},
		// Um tipo para dois números vale para os dois.
		{"IP", "10/2026;11/2026", []item{{ProcIP, "10", "2026"}, {ProcIP, "11", "2026"}}, nil},
		// Dois tipos e um número só: falta o número do segundo.
		{"IP;TCO", "10/2026", []item{{ProcIP, "10", "2026"}, {ProcTCO, "", ""}}, []string{PendProcNumero}},
		{"XX;IP", "10/2026;11/2026", []item{{0, "10", "2026"}, {ProcIP, "11", "2026"}}, []string{PendProcTipo}},
	} {
		r := c.ResolveProcedure(ProcInput{Type: tc.typ, Number: tc.num, ManualDelegacia: true, ManualDelegado: true})
		if len(r.Itens) != len(tc.items) {
			t.Errorf("%q %q: %d itens, quer %d", tc.typ, tc.num, len(r.Itens), len(tc.items))
			continue
		}
		for i, w := range tc.items {
			g := r.Itens[i]
			gp := 0
			if g.ProcedimentoID != nil {
				gp = *g.ProcedimentoID
			}
			if gp != w.proc || g.Numero != w.numero || g.Ano != w.ano {
				t.Errorf("%q %q item %d: tipo %d nº %s/%s, quer %d %s/%s", tc.typ, tc.num, i, gp, g.Numero, g.Ano, w.proc, w.numero, w.ano)
			}
		}
		var got []string
		for _, p := range r.Pending {
			if p == PendProcTipo || p == PendProcNumero {
				got = append(got, p)
			}
		}
		if strings.Join(got, ",") != strings.Join(tc.pending, ",") {
			t.Errorf("%q %q: pendências %v, quer %v", tc.typ, tc.num, got, tc.pending)
		}
	}
}
