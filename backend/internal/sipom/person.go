package sipom

import (
	"strings"
	"time"
)

// Envolvidos no formato do modal "Adicionar Pessoa à Ocorrência" do SIPOM:
// vínculo*, CPF, sexo, morte*, nome, nascimento, mãe e foto. O relatório
// traz papel, nome, mãe, idade e anotação; o resto vem do dossiê vinculado,
// que também prevalece no nome e na mãe (é o cadastro curado).

// Vínculos do SIPOM. No formulário deles o valor é um token que muda a cada
// carregamento; o envio escolhe a opção pelo texto (VinculoLabel).
const (
	VincVitima       = "vitima"
	VincInfrator     = "infrator"
	VincTestemunha   = "testemunha"
	VincVitimaNI     = "vitima_ni"
	VincInfratorNI   = "infrator_ni"
	SexoMasculino    = 1
	SexoFeminino     = 2
	FontePessoaDoss  = "dossie"
	FontePessoaRelat = "relatorio"
)

// VinculoLabel é o texto da opção no SIPOM, grafia deles inclusive.
var VinculoLabel = map[string]string{
	VincVitima:     "Vitima",
	VincInfrator:   "Infrator",
	VincTestemunha: "Testemunha",
	VincVitimaNI:   "Vitma - Não Identificada",
	VincInfratorNI: "Infrator - Não identificado",
}

// PersonInput é o envolvido como o Tevunah o tem: papel e dados do
// relatório, mais o dossiê quando vinculado.
type PersonInput struct {
	Role       string // ACUSADO | VÍTIMA | TESTEMUNHA
	Name       string
	MotherName string
	Note       string // anotação do relatório ("ÓBITO", "MENOR"…)

	Linked     bool
	EntName    string
	EntMother  string
	CPF        string
	Gender     string // "M" | "F" | ""
	BirthDate  *time.Time
	Deceased   bool
	DeceasedOn *time.Time
	HasPhoto   bool

	OccurredOn time.Time
}

// Person é o envolvido pronto para o SIPOM.
type Person struct {
	Vinculo    string
	Nome       string
	CPF        string // só dígitos
	Sexo       int    // 0 = não informado
	Nascimento *time.Time
	Mae        string
	Morte      bool
	Foto       bool
	Fonte      string // de onde vieram os dados: dossiê ou só o relatório
}

// Linked diz se o envolvido tem dossiê — sem ele só vão nome e mãe.
func (p Person) Linked() bool { return p.Fonte == FontePessoaDoss }

// TranslatePerson monta o envolvido no formato do SIPOM.
func TranslatePerson(in PersonInput) Person {
	p := Person{Fonte: FontePessoaRelat, Nome: clean(in.Name), Mae: clean(in.MotherName)}
	if in.Linked {
		p.Fonte = FontePessoaDoss
		if n := clean(in.EntName); n != "" {
			p.Nome = n
		}
		if m := clean(in.EntMother); m != "" {
			p.Mae = m
		}
		p.CPF = digits(in.CPF)
		switch strings.ToUpper(strings.TrimSpace(in.Gender)) {
		case "M":
			p.Sexo = SexoMasculino
		case "F":
			p.Sexo = SexoFeminino
		}
		p.Nascimento = in.BirthDate
		p.Foto = in.HasPhoto
	}

	unidentified := Unidentified(p.Nome)
	if unidentified {
		p.Nome = ""
	}
	switch in.Role {
	case "ACUSADO":
		p.Vinculo = VincInfrator
		if unidentified {
			p.Vinculo = VincInfratorNI
		}
	case "VÍTIMA":
		p.Vinculo = VincVitima
		if unidentified {
			p.Vinculo = VincVitimaNI
		}
	default:
		p.Vinculo = VincTestemunha
	}

	// Morte é desta ocorrência: anotação "ÓBITO" no relatório, ou dossiê em
	// óbito na data do fato. Óbito posterior (outro fato) não conta.
	p.Morte = strings.Contains(key(in.Note), "obito") ||
		(in.Linked && in.Deceased && in.DeceasedOn != nil && sameDay(*in.DeceasedOn, in.OccurredOn))
	return p
}

// Unidentified reconhece os nomes que o relatório usa para quem não foi
// identificado.
func Unidentified(name string) bool {
	k := key(name)
	if k == "" {
		return true
	}
	for _, s := range []string{"nao identificad", "desconhecid", "ignorad", "sem identificacao"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func clean(s string) string { return strings.ToUpper(strings.Join(strings.Fields(s), " ")) }

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
