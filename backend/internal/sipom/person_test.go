package sipom

import (
	"testing"
	"time"
)

func day(s string) *time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return &t
}

func TestTranslatePerson(t *testing.T) {
	fato := *day("2026-09-26")
	cases := []struct {
		name string
		in   PersonInput
		want Person
	}{
		{"acusado só do relatório",
			PersonInput{Role: "ACUSADO", Name: "thiago  rudnei", MotherName: "claudia", OccurredOn: fato},
			Person{Vinculo: VincInfrator, Nome: "THIAGO RUDNEI", Mae: "CLAUDIA", Fonte: FontePessoaRelat}},
		{"acusado com dossiê: dossiê prevalece",
			PersonInput{Role: "ACUSADO", Name: "THIAGO R", MotherName: "", Linked: true, EntName: "Thiago Rudnei Bezerra",
				EntMother: "Claudia Rodrigues", CPF: "083.715.903-26", Gender: "M", BirthDate: day("2001-01-02"),
				HasPhoto: true, OccurredOn: fato},
			Person{Vinculo: VincInfrator, Nome: "THIAGO RUDNEI BEZERRA", Mae: "CLAUDIA RODRIGUES", CPF: "08371590326",
				Sexo: SexoMasculino, Nascimento: day("2001-01-02"), Foto: true, Fonte: FontePessoaDoss}},
		{"vítima com óbito no relatório",
			PersonInput{Role: "VÍTIMA", Name: "WESLEY", Note: "ÓBITO", OccurredOn: fato},
			Person{Vinculo: VincVitima, Nome: "WESLEY", Morte: true, Fonte: FontePessoaRelat}},
		{"vítima não identificada",
			PersonInput{Role: "VÍTIMA", Name: "Indivíduo não identificado", OccurredOn: fato},
			Person{Vinculo: VincVitimaNI, Fonte: FontePessoaRelat}},
		{"acusado sem nome",
			PersonInput{Role: "ACUSADO", Name: "", OccurredOn: fato},
			Person{Vinculo: VincInfratorNI, Fonte: FontePessoaRelat}},
		{"testemunha",
			PersonInput{Role: "TESTEMUNHA", Name: "MARIA", OccurredOn: fato},
			Person{Vinculo: VincTestemunha, Nome: "MARIA", Fonte: FontePessoaRelat}},
		{"óbito no dossiê na data do fato",
			PersonInput{Role: "VÍTIMA", Name: "JOSE", Linked: true, EntName: "JOSE", Deceased: true,
				DeceasedOn: day("2026-09-26"), OccurredOn: fato},
			Person{Vinculo: VincVitima, Nome: "JOSE", Morte: true, Fonte: FontePessoaDoss}},
		{"óbito no dossiê em outra data não conta",
			PersonInput{Role: "ACUSADO", Name: "JOSE", Linked: true, EntName: "JOSE", Deceased: true,
				DeceasedOn: day("2026-10-01"), OccurredOn: fato},
			Person{Vinculo: VincInfrator, Nome: "JOSE", Fonte: FontePessoaDoss}},
	}
	for _, c := range cases {
		got := TranslatePerson(c.in)
		eqDate := (got.Nascimento == nil) == (c.want.Nascimento == nil) &&
			(got.Nascimento == nil || got.Nascimento.Equal(*c.want.Nascimento))
		got.Nascimento, c.want.Nascimento = nil, nil
		if got != c.want || !eqDate {
			t.Errorf("%s:\n got  %+v\n want %+v", c.name, got, c.want)
		}
	}
}
