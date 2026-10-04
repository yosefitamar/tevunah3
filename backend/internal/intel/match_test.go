package intel

import (
	"reflect"
	"testing"
)

var seedTerms = []string{
	"SAI do 2º BPRAIO",
	"SAI 2º BPRAIO",
	"S.A.I.",
	"serviço de inteligência",
	"agência de inteligência",
	"P2",
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"SAI do 2º BPRAIO":        "sai do 2 bpraio",
		"SAI/2ºBPRAIO":            "sai 2 bpraio",
		"S.A.I.":                  "s a i",
		"S. A. I.":                "s a i",
		"P-2":                     "p 2",
		"Serviço de Inteligência": "servico de inteligencia",
		"  ...  ":                 "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatch(t *testing.T) {
	m := NewMatcher(seedTerms)
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"padronizado", "Com apoio da SAI do 2º BPRAIO, a equipe abordou", []string{"SAI do 2º BPRAIO"}},
		{"sem ordinal e caixa baixa", "informações da sai do 2 bpraio", []string{"SAI do 2º BPRAIO"}},
		{"colado com barra", "levantamento da SAI/2ºBPRAIO", []string{"SAI 2º BPRAIO"}},
		{"sigla pontuada", "Após receber informações da S.A.I. (serviço de inteligência do 2º BPRAIO)",
			[]string{"S.A.I.", "serviço de inteligência"}},
		{"sem acento", "repassado pela agencia de inteligencia", []string{"agência de inteligência"}},
		{"P2 com hífen", "a P-2 do batalhão informou", []string{"P2"}},
		{"verbo sair não casa", "o suspeito saiu do 2º BPRAIO e sai correndo", nil},
		{"palavra inteira", "a equipe SAIDA do 2º BPRAIO", nil},
		{"P2 dentro de outra palavra", "modelo XP2000 apreendido", nil},
		{"nada", "patrulhamento de rotina, sem alterações", nil},
	}
	for _, c := range cases {
		if got := m.Match(c.text); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Match(%q) = %v, want %v", c.name, c.text, got, c.want)
		}
	}
}

// Um termo não pode casar emendando o fim de um campo com o começo de outro.
func TestMatchAcrossFields(t *testing.T) {
	m := NewMatcher([]string{"SAI do 2º BPRAIO"})
	if got := m.Match("EQUIPE SAI", "do 2º BPRAIO"); got != nil {
		t.Errorf("casou entre campos: %v", got)
	}
}

func TestEmptyMatcher(t *testing.T) {
	var m *Matcher
	if got := m.Match("SAI do 2º BPRAIO"); got != nil {
		t.Errorf("matcher nil casou: %v", got)
	}
	if got := NewMatcher([]string{"...", ""}).Match("qualquer coisa"); got != nil {
		t.Errorf("termos vazios casaram: %v", got)
	}
}
