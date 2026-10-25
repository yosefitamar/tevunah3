package sipom

import "testing"

func weaponCatalog() *Catalog {
	list := func(items ...Ref) *refList {
		l := newRefList()
		for _, it := range items {
			l.items = append(l.items, it)
			l.byID[it.ID] = it.Nome
			l.byKey[key(it.Nome)] = it.ID
		}
		return l
	}
	return &Catalog{
		armaTipos:    list(Ref{ID: 1, Nome: "Pistola"}, Ref{ID: 7, Nome: "Espingarda"}, Ref{ID: 8, Nome: "Artesanal"}),
		armaMarcas:   list(Ref{ID: 3, Nome: "Taurus"}, Ref{ID: 40, Nome: "Suprimida"}),
		armaCalibres: list(Ref{ID: 2, Nome: "12"}, Ref{ID: 10, Nome: "38"}),
	}
}

// Arma artesanal não tem fabricante: a marca vai como "Suprimida".
func TestResolveWeaponHomemadeBrand(t *testing.T) {
	c := weaponCatalog()
	for _, tc := range []struct {
		name  string
		in    WeaponInput
		marca int // 0 = sem marca
		ok    bool
	}{
		{"modelo artesanal", WeaponInput{Kind: "ESPINGARDA", Model: "ARTESANAL", Caliber: "12"}, 40, true},
		{"tipo artesanal", WeaponInput{Kind: "ARTESANAL", Caliber: "12"}, 40, true},
		{"arma caseira", WeaponInput{Kind: "Arma caseira", Caliber: "38"}, 40, true},
		{"artesanal com marca escrita: vale a marca", WeaponInput{Kind: "PISTOLA", Brand: "Taurus", Model: "ARTESANAL", Caliber: "38"}, 3, true},
		// Só o tipo é obrigatório no SIPOM: sem calibre ou sem marca, a arma
		// vai assim mesmo.
		{"artesanal sem calibre: marca resolvida, vai sem calibre", WeaponInput{Kind: "ESPINGARDA", Model: "ARTESANAL"}, 40, true},
		{"industrial sem marca: vai sem marca", WeaponInput{Kind: "PISTOLA", Caliber: "38"}, 0, true},
		{"sem tipo: pendente", WeaponInput{Kind: "TRABUCO", Caliber: "38"}, 0, false},
	} {
		r := c.ResolveWeapon(tc.in, nil)
		got := 0
		if r.MarcaID != nil {
			got = *r.MarcaID
		}
		if got != tc.marca || r.OK != tc.ok {
			t.Errorf("%s: marca=%d ok=%v, quer %d %v", tc.name, got, r.OK, tc.marca, tc.ok)
		}
	}
}
