package sipomai

import (
	"strings"
	"testing"
)

func TestPreviousPrompt(t *testing.T) {
	if (&Previous{}).Prompt() != "" || (*Previous)(nil).Prompt() != "" {
		t.Fatal("sem sugestões, sem seção")
	}
	p := &Previous{When: "25/11/2026 14:00", Reviewed: true,
		Suggestions: []Suggestion{
			{Target: "Natureza", Value: "ROUBO", Applicable: true},
			{Target: "Bairro", Value: "CENTRO", Applicable: true, Motivo: "parecido"},
			{Target: "Droga 1", Value: "MACONHA", Applicable: true},
			{Target: "Veículo 0", Value: "falta marca/modelo"},
		},
		Applied: map[int]bool{0: true}, Rejected: map[int]bool{1: true}, Failed: map[int]string{2: "droga inválida"},
	}
	got := p.Prompt()
	for _, want := range []string{"25/11/2026 14:00", "Aplicadas pelo analista", "- Natureza → ROUBO",
		"Não aceitas", "- Bairro → CENTRO — seu motivo foi: parecido", "erro ao gravar: droga inválida",
		"Sem decisão registrada", "não aplicável"} {
		if !strings.Contains(got, want) {
			t.Errorf("faltou %q em:\n%s", want, got)
		}
	}
}
