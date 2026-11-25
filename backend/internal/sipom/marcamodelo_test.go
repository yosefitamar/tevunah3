package sipom

import "testing"

func mmCatalog(descs ...string) *Catalog {
	c := &Catalog{marcaModeloByCode: map[int]MarcaModelo{}, marcaModeloKey: map[string]MarcaModelo{}}
	for i, d := range descs {
		m := MarcaModelo{Codigo: 100 + i, Descricao: d}
		c.marcasModelos = append(c.marcasModelos, m)
		c.marcaModeloByCode[m.Codigo] = m
		c.marcaModeloKey[key(d)] = m
	}
	return c
}

func TestSuggestMarcasModelosTypo(t *testing.T) {
	c := mmCatalog("HONDA/NXR160 BROS ESDD", "HONDA/NXR 160 BROS", "HONDA/CG 160 FAN", "YAMAHA/XTZ150 CROSSER Z")
	if got := c.SearchMarcasModelos("HONDA NXR160 BROZ", 20); len(got) != 0 {
		t.Fatalf("busca exata não devia achar: %v", got)
	}
	got := c.SuggestMarcasModelos("HONDA NXR160 BROZ", 20)
	if len(got) != 2 || got[0].Codigo != 100 || got[1].Codigo != 101 {
		t.Fatalf("esperava as duas NXR160 BROS, veio %v", got)
	}
	// Palavra curta não é tolerada ("CB" não vira "CG").
	if got := c.SuggestMarcasModelos("HONDA CB 160", 20); len(got) != 0 {
		t.Fatalf("CB não devia casar com CG: %v", got)
	}
	// Achando pela exata, a tolerante não entra.
	if got := c.SuggestMarcasModelos("CG 160", 20); len(got) != 1 || got[0].Codigo != 102 {
		t.Fatalf("exata: %v", got)
	}
}

func TestResolveVehicleTypoOnlyCandidates(t *testing.T) {
	c := mmCatalog("HONDA/NXR160 BROS ESDD", "HONDA/CG 160 FAN")
	c.veiculoTipos, c.veiculoCores = newCodeList(), newCodeList()
	r := c.ResolveVehicle(VehicleInput{Kind: "MOTOCICLETA", Brand: "HONDA", Model: "NXR160 BROZ"}, nil)
	if r.MarcaModeloCodigo != nil {
		t.Fatalf("erro de digitação não pode escolher sozinho: %d", *r.MarcaModeloCodigo)
	}
	if len(r.MarcaModeloCandidates) != 1 || r.MarcaModeloCandidates[0].Codigo != 100 {
		t.Fatalf("candidatos: %v", r.MarcaModeloCandidates)
	}
}

func TestEditDistanceAtMostOne(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"broz", "bros", true}, {"bros", "bross", true}, {"bros", "bro", true}, {"bros", "bros", true}, {"broz", "brsx", false}, {"fan", "fanxx", false}} {
		if got := editDistanceAtMostOne([]rune(c.a), []rune(c.b)); got != c.want {
			t.Errorf("%s/%s = %v", c.a, c.b, got)
		}
	}
}
