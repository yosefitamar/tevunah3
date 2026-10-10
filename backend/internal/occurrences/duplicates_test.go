package occurrences

import (
	"reflect"
	"testing"
	"time"
)

func TestNormCIOPS(t *testing.T) {
	for in, want := range map[string]string{
		"M20260705888":    "M20260705888",
		" m 2026-0705888": "M20260705888",
		"M.2026/0705888":  "M20260705888",
		"":                "",
		" - ":             "",
		"FICHA Nº 123":    "FICHAN123",
	} {
		if got := NormCIOPS(in); got != want {
			t.Errorf("NormCIOPS(%q) = %q, quer %q", in, got, want)
		}
	}
}

func TestEditDistance(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"M20260705888", "M20260705888", 0},
		{"M20260705888", "M20260705889", 1}, // dígito trocado
		{"M20260705888", "M2026070588", 1},  // dígito faltando
		{"M20260705888", "M20260750888", 1}, // vizinhos invertidos
		{"M20260705888", "M20260715898", 2},
		{"M20260705888", "M20260707060", 4},
		{"M20260705888", "M2026", 3}, // tamanho muito diferente: responde max+1
	} {
		if got := editDistance(c.a, c.b, 2); got != c.want {
			t.Errorf("editDistance(%q, %q) = %d, quer %d", c.a, c.b, got, c.want)
		}
	}
}

func TestMatchReasons(t *testing.T) {
	day := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	base := Subject{
		CIOPS: "M20260705888", OccurredOn: day, Time: "21:40",
		City: "JUAZEIRO DO NORTE", Neighborhood: "JOÃO CABRAL",
	}
	with := func(f func(*Subject)) Subject {
		s := base
		f(&s)
		return s
	}

	for _, c := range []struct {
		name string
		b    Subject
		want []string
	}{
		{
			name: "mesma ficha é identidade, não suspeita",
			b:    with(func(s *Subject) { s.CIOPS = "m 2026-0705888" }),
			want: nil,
		},
		{
			name: "ficha digitada errado, mesma hora e bairro",
			b:    with(func(s *Subject) { s.CIOPS = "M20260705889"; s.Neighborhood = "JOAO CABRAL" }),
			want: []string{ReasonSimilarCIOPS, ReasonSameTime, ReasonSameNeighborhood},
		},
		{
			name: "sem ficha, hora próxima no mesmo município",
			b:    with(func(s *Subject) { s.CIOPS = ""; s.Time = "22:05"; s.Neighborhood = "CENTRO" }),
			want: []string{ReasonSameTime, ReasonSameCity},
		},
		{
			name: "hora distante no mesmo bairro não é suspeita",
			b:    with(func(s *Subject) { s.CIOPS = "M20260709999"; s.Time = "09:00" }),
			want: nil,
		},
		{
			name: "sem hora de um lado: mesmo bairro no mesmo dia",
			b:    with(func(s *Subject) { s.CIOPS = "M20260709999"; s.Time = "" }),
			want: []string{ReasonSameNeighborhood},
		},
		{
			name: "sem hora e bairro diferente não é suspeita",
			b:    with(func(s *Subject) { s.CIOPS = "M20260709999"; s.Time = ""; s.Neighborhood = "CENTRO" }),
			want: nil,
		},
		{
			name: "município diferente descarta, mesmo com ficha parecida",
			b:    with(func(s *Subject) { s.CIOPS = "M20260705889"; s.City = "CRATO" }),
			want: nil,
		},
		{
			name: "ficha parecida no mesmo dia, sem lugar informado",
			b:    Subject{CIOPS: "M20260750888", OccurredOn: day},
			want: []string{ReasonSimilarCIOPS},
		},
		{
			name: "ficha parecida em outro dia não é suspeita",
			b:    Subject{CIOPS: "M20260705889", OccurredOn: day.AddDate(0, 0, 3)},
			want: nil,
		},
		{
			name: "virada da meia-noite: 23:50 e 00:10 do dia seguinte",
			b: Subject{
				CIOPS: "M20260711111", OccurredOn: day.AddDate(0, 0, 1), Time: "00:10",
				City: "JUAZEIRO DO NORTE",
			},
			want: nil, // base é 21:40 — longe
		},
	} {
		if got := matchReasons(base, c.b); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, quer %v", c.name, got, c.want)
		}
	}

	// Virada da meia-noite de verdade.
	late := with(func(s *Subject) { s.Time = "23:50" })
	next := Subject{CIOPS: "M20260711111", OccurredOn: day.AddDate(0, 0, 1), Time: "00:10", City: "Juazeiro do Norte"}
	if got, want := matchReasons(late, next), []string{ReasonSameTime, ReasonSameCity}; !reflect.DeepEqual(got, want) {
		t.Errorf("meia-noite: %v, quer %v", got, want)
	}
}
