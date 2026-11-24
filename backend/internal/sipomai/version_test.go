package sipomai

import "testing"

func TestVersionInText(t *testing.T) {
	cases := []struct {
		desc, text string
		want       bool
	}{
		{"HONDA/CG 125 TITAN", "motocicleta Honda Titan vermelha", false},
		{"HONDA/CG 160 TITAN", "uma Honda CG 160 Titan", true},
		{"FIAT/PALIO", "Fiat Palio prata", true},
		{"VW/GOL 1.0", "VW Gol 1.0 branco", true},
	}
	for _, c := range cases {
		if got := versionInText(c.desc, c.text); got != c.want {
			t.Errorf("versionInText(%q, %q) = %v, quer %v", c.desc, c.text, got, c.want)
		}
	}
}

// O structured outputs recusa schema com mais de 16 campos com união de
// tipos (nullable/anyOf).
func TestSchemaUnionLimit(t *testing.T) {
	n := 0
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if _, ok := x["anyOf"]; ok {
				n++
			}
			for _, c := range x {
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(Schema)
	if n > 16 {
		t.Fatalf("schema com %d campos com união de tipos (limite 16)", n)
	}
}
