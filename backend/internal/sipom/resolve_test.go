package sipom

import (
	"reflect"
	"testing"
)

func TestSplitAddress(t *testing.T) {
	cases := []struct{ in, log, num string }{
		{"Rua Fabiano De Cristo, s/n", "Rua Fabiano De Cristo", "S/N"},
		{"Rua E, S/N", "Rua E", "S/N"},
		{"Rua José Abílio 335, Casa 39a", "Rua José Abílio", "335 - Casa 39a"},
		{"Rua São Jerônimo, 1588", "Rua São Jerônimo", "1588"},
		{"Rua 25 de Março, 100", "Rua 25 de Março", "100"},
		{"Av. Central, nº 45, Bloco B", "Av. Central", "45 - Bloco B"},
		{"Rua Uruguai 1000", "Rua Uruguai", "1000"},
		{"Casa De Telha", "Casa De Telha", ""},
		{"Zona Rural", "Zona Rural", ""},
		{"  ", "", ""},
	}
	for _, c := range cases {
		log, num := SplitAddress(c.in)
		if log != c.log || num != c.num {
			t.Errorf("SplitAddress(%q) = %q | %q, want %q | %q", c.in, log, num, c.log, c.num)
		}
	}
}

func ids(o []Officer) (tipos, funcs []int) {
	for _, x := range o {
		if x.PoliciamentoTipoID != nil {
			tipos = append(tipos, *x.PoliciamentoTipoID)
		}
		if x.FuncaoID != nil {
			funcs = append(funcs, *x.FuncaoID)
		}
	}
	return
}

func TestComposition(t *testing.T) {
	cases := []struct {
		teams string
		n     int
		tipo  int
		funcs []int
		pend  string
	}{
		{"VTRA 121", 3, PolicMotorizado, []int{1, 7, 5}, ""},
		{"VTRA 079", 4, PolicMotorizado, []int{1, 7, 5, 5}, ""},
		{"RAIO 03", 4, PolicMotopatrulhamento, []int{1, 2, 3, 4}, ""},
		{"RAIO 02", 5, PolicMotopatrulhamento, []int{1, 2, 3, 4, 13}, ""},
		{"raio 01", 3, PolicMotopatrulhamento, []int{1, 2, 3}, ""},
		{"RAIO 01; RAIO 02", 4, 0, nil, PendComposicaoMulti},
		{"VTRA 108; RAIO 02", 4, 0, nil, PendComposicaoMulti},
		{"BASE", 2, 0, nil, PendComposicao},
		{"", 2, 0, nil, PendComposicao},
		{"RAIO 01", 0, 0, nil, ""},
	}
	for _, c := range cases {
		o, pend := Composition(c.teams, c.n)
		if pend != c.pend || len(o) != c.n {
			t.Errorf("Composition(%q,%d): pend=%q len=%d", c.teams, c.n, pend, len(o))
			continue
		}
		tipos, funcs := ids(o)
		if c.pend == "" && c.n > 0 {
			if !reflect.DeepEqual(funcs, c.funcs) || tipos[0] != c.tipo {
				t.Errorf("Composition(%q,%d) = tipo %v funcs %v", c.teams, c.n, tipos, funcs)
			}
		}
		if c.pend != "" && len(funcs) > 0 {
			t.Errorf("Composition(%q): pendente mas atribuiu funções %v", c.teams, funcs)
		}
	}
}

func TestCompositionByTeam(t *testing.T) {
	// VTRA 108; RAIO 02: PM1 e PM3 na viatura, PM2 e PM4 na moto.
	o, pend := CompositionByTeam([]string{"VTRA 108", "RAIO 02", "vtra 108", "RAIO 02"})
	if pend != "" {
		t.Fatalf("pend=%q", pend)
	}
	want := [][2]int{
		{PolicMotorizado, FuncComandante}, {PolicMotopatrulhamento, FuncComandante},
		{PolicMotorizado, FuncMotorista}, {PolicMotopatrulhamento, FuncSubcomandante},
	}
	for i, w := range want {
		if *o[i].PoliciamentoTipoID != w[0] || *o[i].FuncaoID != w[1] {
			t.Errorf("PM%d = tipo %d funcao %d, want %v", i+1, *o[i].PoliciamentoTipoID, *o[i].FuncaoID, w)
		}
	}
	if _, pend := CompositionByTeam([]string{"RAIO 01", ""}); pend != PendComposicaoMulti {
		t.Errorf("equipe vazia: pend=%q", pend)
	}
	if _, pend := CompositionByTeam([]string{"BASE"}); pend != PendComposicao {
		t.Errorf("equipe desconhecida: pend=%q", pend)
	}
}

// Sexto policial numa equipe de motos não tem função no SIPOM.
func TestCompositionTooManyRaio(t *testing.T) {
	o, pend := Composition("RAIO 01", 6)
	if _, funcs := ids(o); pend != PendComposicao || len(funcs) != 0 {
		t.Errorf("6 PMs em RAIO: pend=%q funcs=%v", pend, funcs)
	}
}
