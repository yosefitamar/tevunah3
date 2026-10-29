package sipom

import "testing"

func TestAreaRulesCityWide(t *testing.T) {
	m := NewAreaRules([]AreaRule{
		{City: "SÃO GONÇALO DO AMARANTE", Neighborhood: "PECÉM", AreaID: 33},
		{City: "SÃO GONÇALO DO AMARANTE", Neighborhood: "CENTRO", AreaID: 33},
		{City: "CAUCAIA", Neighborhood: "POTIRA", AreaID: 174},
		{City: "CAUCAIA", Neighborhood: "CENTRO", AreaID: 175},
	})
	// Bairro sem referência: vale a da cidade só onde o catálogo não
	// distingue bairro.
	if id, ok := m.Find("Sao Goncalo do Amarante", "TAÍBA", true); !ok || id != 33 {
		t.Errorf("cidade inteira = %d %v, quer 33", id, ok)
	}
	if _, ok := m.Find("SÃO GONÇALO DO AMARANTE", "TAÍBA", false); ok {
		t.Error("catálogo por bairro: a referência de outro bairro não vale")
	}
	// Referências da cidade em conflito: nada vale para a cidade.
	if _, ok := m.Find("CAUCAIA", "SÃO MIGUEL", true); ok {
		t.Error("cidade com referências em conflito não deveria ter área")
	}
	// O par exato vale sempre.
	if id, ok := m.Find("CAUCAIA", "potira", false); !ok || id != 174 {
		t.Errorf("par exato = %d %v", id, ok)
	}
}

func TestCityWideAreas(t *testing.T) {
	c := &Catalog{
		areaByCidade: map[int][]int{1: {10, 11}},
		areaByBairro: map[int][]int{7: {12}},
	}
	one, two, b7, b8 := 1, 2, 7, 8
	cases := []struct {
		cidade, bairro *int
		want           bool
	}{
		{&one, nil, true}, {&one, &b8, true}, {&one, &b7, false}, {&two, nil, false}, {nil, nil, false},
	}
	for _, k := range cases {
		if got := c.CityWideAreas(k.cidade, k.bairro); got != k.want {
			t.Errorf("CityWideAreas(%v, %v) = %v", k.cidade, k.bairro, got)
		}
	}
}
