package sipom

import "testing"

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
		if got := tokensSubsequence(nameTokens(c.want), nameTokens(c.have)); got != c.ok {
			t.Errorf("%q em %q = %v, quer %v", c.want, c.have, got, c.ok)
		}
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
