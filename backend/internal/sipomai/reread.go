package sipomai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/intel"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// Releitura do relatório original: quando a importação leu a página do SGR
// errado (rótulo grudado no valor, valor no campo vizinho, hora "18h30",
// linha quebrada), a IA compara o que foi gravado com o trecho do PDF e
// propõe a correção. A correção passa pelo PATCH de sempre da ocorrência
// (ou do envolvido) — e só vale o valor que está escrito no trecho.

const (
	// KindCampo: campo do cabeçalho (PATCH /api/ops-occurrences/{id}).
	KindCampo = "campo"
	// KindPessoaDados: nome, mãe, idade ou endereço do envolvido (PATCH .../people/{pid}).
	KindPessoaDados = "pessoa_dados"
	// KindPessoaRemover: envolvido lido a mais (linha da tabela quebrada em
	// duas, repetido) — PUT .../people/{pid}/remocao.
	KindPessoaRemover = "pessoa_remover"
)

// RereadCodes são as pendências que podem vir de leitura errada do PDF.
var RereadCodes = []string{
	sipom.PendDataHora, sipom.PendFicha, sipom.PendLogradouro, sipom.PendCidade, sipom.PendBairro,
	sipom.PendProcedimento, sipom.PendProcTipo, sipom.PendProcNumero, sipom.PendDelegacia, sipom.PendDelegado,
}

// rereadField é um campo do cabeçalho que a releitura pode corrigir: a
// chave do PATCH, o rótulo e o valor gravado.
type rereadField struct {
	key, label string
	get        func(o *opsreport.Occurrence) string
}

var rereadFields = []rereadField{
	{"ciops_record", "Ficha CIOPS", func(o *opsreport.Occurrence) string { return o.CIOPS }},
	{"occurred_on", "Data (AAAA-MM-DD)", func(o *opsreport.Occurrence) string {
		if o.OccurredOn.IsZero() {
			return ""
		}
		return o.OccurredOn.Format("2006-01-02")
	}},
	{"start_time", "Hora inicial (HH:MM)", func(o *opsreport.Occurrence) string { return o.StartTime }},
	{"end_time", "Hora final (HH:MM)", func(o *opsreport.Occurrence) string { return o.EndTime }},
	{"teams", "Equipe", func(o *opsreport.Occurrence) string { return o.Teams }},
	{"place_address", "Local da ocorrência (logradouro, número)", func(o *opsreport.Occurrence) string { return o.PlaceAddress }},
	{"place_neighborhood", "Bairro da ocorrência", func(o *opsreport.Occurrence) string { return o.PlaceNeighborhood }},
	{"place_city", "Cidade da ocorrência", func(o *opsreport.Occurrence) string { return o.PlaceCity }},
	{"approach_address", "Local da abordagem", func(o *opsreport.Occurrence) string { return o.ApproachAddress }},
	{"approach_neighborhood", "Bairro da abordagem", func(o *opsreport.Occurrence) string { return o.ApproachNeighborhood }},
	{"approach_city", "Cidade da abordagem", func(o *opsreport.Occurrence) string { return o.ApproachCity }},
	{"police_station", "Delegacia", func(o *opsreport.Occurrence) string { return o.PoliceStation }},
	{"delegate", "Delegado(a)", func(o *opsreport.Occurrence) string { return o.Delegate }},
	{"procedure_type", "Tipo de procedimento", func(o *opsreport.Occurrence) string { return o.ProcedureType }},
	{"procedure_number", "Nº do procedimento", func(o *opsreport.Occurrence) string { return o.ProcedureNumber }},
	{"seized_objects", "Objetos apreendidos", func(o *opsreport.Occurrence) string { return o.SeizedObjects }},
}

var reCIOPS = regexp.MustCompile(`^M\d{11}$`)

// NeedsReread diz se vale reler o PDF: há pendência que pode ser leitura
// errada, ou um campo tem cara de leitura errada (ficha fora do formato,
// rótulo "Xxx:" grudado no valor).
func NeedsReread(so *opsreport.StoredOccurrence) bool {
	for _, p := range so.Sipom.Pending {
		for _, c := range RereadCodes {
			if p == c {
				return true
			}
		}
	}
	return LooksMisread(&so.Occurrence)
}

// LooksMisread: ficha fora do formato ou dois-pontos num campo de texto do
// cabeçalho (só a hora tem dois-pontos legítimo).
func LooksMisread(o *opsreport.Occurrence) bool {
	if !reCIOPS.MatchString(strings.TrimSpace(o.CIOPS)) {
		return true
	}
	for _, f := range rereadFields {
		switch f.key {
		case "ciops_record", "occurred_on", "start_time", "end_time", "seized_objects":
			continue
		}
		if strings.Contains(f.get(o), ":") {
			return true
		}
	}
	return false
}

// Reread é o pedido da releitura.
type Reread struct {
	Prompt string
	so     *opsreport.StoredOccurrence
	source string
	flat   string   // trecho normalizado, sem espaços, para conferir os valores
	tokens []string // palavras do trecho normalizado, na ordem
	people map[string]opsreport.StoredPerson
}

// BuildReread monta o pedido com o trecho original (source) da ocorrência.
func BuildReread(so *opsreport.StoredOccurrence, source string) *Reread {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}
	rr := &Reread{so: so, source: source, flat: flat(source), tokens: strings.Fields(intel.Normalize(source)),
		people: map[string]opsreport.StoredPerson{}}
	o := &so.Occurrence
	var b strings.Builder
	b.WriteString("# RELEITURA DO RELATÓRIO ORIGINAL\n")
	b.WriteString("A importação automática leu a página do relatório (PDF do SGR) e gravou os campos abaixo. ")
	b.WriteString("Alguns podem ter saído errados: rótulo grudado no valor, valor no campo vizinho, linha quebrada, hora escrita \"18h30\". ")
	b.WriteString("Compare cada campo com o TRECHO ORIGINAL e proponha, em campos, só os que estão errados ou vazios e que o trecho traz.\n")
	b.WriteString("Regras: copie o valor como está escrito no trecho (sem inventar, completar ou corrigir a grafia); sem o rótulo; ")
	b.WriteString("data em AAAA-MM-DD; hora em HH:MM (\"18h30\" → \"18:30\"); ficha no formato M + 11 dígitos. Campo vazio no trecho fica vazio: não proponha. ")
	b.WriteString("Campo certo: não inclua.\n")
	if len(so.Sipom.Pending) > 0 {
		fmt.Fprintf(&b, "Pendências do envio ao SIPOM nesta ficha: %s.\n", strings.Join(so.Sipom.Pending, ", "))
	}
	b.WriteString("\n## CAMPOS GRAVADOS (campo | rótulo | valor gravado)\n")
	for _, f := range rereadFields {
		fmt.Fprintf(&b, "%s | %s | %s\n", f.key, f.label, quoteOrEmpty(f.get(o)))
	}
	if len(so.People) > 0 {
		b.WriteString("\n## ENVOLVIDOS GRAVADOS (ref | papel | nome | mãe | idade | endereço)\n")
		b.WriteString("Em pessoas, corrija nome, mãe, idade ou endereço lidos errado. Célula da tabela quebrada em duas linhas costuma virar uma pessoa a mais: ")
		b.WriteString("junte o que falta na pessoa verdadeira (nome, endereço completos como no trecho) e marque remover = true na que não existe, com o motivo. ")
		b.WriteString("Nunca remova pessoa que o trecho lista de fato. Campo que não muda: texto vazio (idade 0). No motivo, cite as pessoas pelo nome, não pela ref.\n")
		for i, p := range so.People {
			ref := fmt.Sprintf("P%d", i+1)
			rr.people[ref] = p
			age := "—"
			if p.Age != nil {
				age = strconv.Itoa(*p.Age)
			}
			fmt.Fprintf(&b, "%s | %s | %s | %s | %s | %s\n", ref, p.Role, quoteOrEmpty(p.Name), quoteOrEmpty(p.MotherName), age, quoteOrEmpty(p.Address))
		}
	}
	fmt.Fprintf(&b, "\n## TRECHO ORIGINAL (texto do PDF, na disposição impressa)\n%s\n", source)
	rr.Prompt = b.String()
	return rr
}

func quoteOrEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(vazio)"
	}
	return fmt.Sprintf("%q", s)
}

// RereadSchema é o formato da releitura.
var RereadSchema = obj(map[string]any{
	"campos": arr(obj(map[string]any{
		"campo":     map[string]any{"type": "string", "enum": rereadKeys()},
		"valor":     map[string]any{"type": "string"},
		"confianca": confianca, "motivo": motivo,
	})),
	"pessoas": arr(obj(map[string]any{
		"ref":       map[string]any{"type": "string"},
		"nome":      map[string]any{"type": "string"},
		"mae":       map[string]any{"type": "string"},
		"idade":     map[string]any{"type": "integer"},
		"endereco":  map[string]any{"type": "string"},
		"remover":   map[string]any{"type": "boolean"},
		"confianca": confianca, "motivo": motivo,
	})),
})

func rereadKeys() []string {
	out := make([]string, 0, len(rereadFields))
	for _, f := range rereadFields {
		out = append(out, f.key)
	}
	return out
}

type rereadAnswer struct {
	Campos []struct {
		Campo     string `json:"campo"`
		Valor     string `json:"valor"`
		Confianca string `json:"confianca"`
		Motivo    string `json:"motivo"`
	} `json:"campos"`
	Pessoas []struct {
		Ref       string `json:"ref"`
		Nome      string `json:"nome"`
		Mae       string `json:"mae"`
		Idade     int    `json:"idade"`
		Endereco  string `json:"endereco"`
		Remover   bool   `json:"remover"`
		Confianca string `json:"confianca"`
		Motivo    string `json:"motivo"`
	} `json:"pessoas"`
}

// Parse confere a resposta: só passa o valor que difere do gravado, está no
// formato do campo e está escrito no trecho original.
func (rr *Reread) Parse(raw []byte) ([]Suggestion, error) {
	var a rereadAnswer
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	o := &rr.so.Occurrence
	byKey := map[string]rereadField{}
	for _, f := range rereadFields {
		byKey[f.key] = f
	}
	out := []Suggestion{}
	seen := map[string]bool{}
	for _, c := range a.Campos {
		f, ok := byKey[c.Campo]
		if !ok || seen[c.Campo] {
			continue
		}
		v, ok := rr.fieldValue(c.Campo, c.Valor)
		if !ok {
			continue
		}
		cur := f.get(o)
		if flat(v) == flat(cur) {
			continue
		}
		seen[c.Campo] = true
		if strings.TrimSpace(cur) == "" {
			cur = "vazio"
		}
		out = append(out, Suggestion{Kind: KindCampo, Code: codeOfField(c.Campo), Target: "Relatório: " + strings.SplitN(f.label, " (", 2)[0],
			Value: v, Current: cur, Confianca: confOf(c.Confianca), Motivo: strings.TrimSpace(c.Motivo),
			Applicable: true, Apply: map[string]any{"field": c.Campo, "value": v}})
	}
	for _, p := range a.Pessoas {
		sp, ok := rr.people[strings.TrimSpace(p.Ref)]
		if !ok || seen[p.Ref] {
			continue
		}
		if p.Remover {
			// Tirar alguém da ficha é decisão do analista: no máximo média,
			// e só com o motivo.
			why := strings.TrimSpace(p.Motivo)
			if why == "" {
				continue
			}
			conf := confOf(p.Confianca)
			if conf == "alta" {
				conf = "media"
			}
			seen[p.Ref] = true
			out = append(out, Suggestion{Kind: KindPessoaRemover, Code: "releitura", Target: "Envolvido: " + sp.Name, PersonID: sp.ID,
				Value: "remover da ficha (lido a mais na importação)", Confianca: conf, Motivo: why, Applicable: true,
				Apply: map[string]any{"motivo": "Sugerido pela IA (releitura do relatório): " + why}})
			continue
		}
		apply := map[string]any{}
		var parts []string
		if n := clean(p.Nome); n != "" && flat(n) != flat(sp.Name) && rr.inSource(n) {
			apply["name"] = n
			parts = append(parts, "nome "+n)
		}
		if m := clean(p.Mae); m != "" && flat(m) != flat(sp.MotherName) && rr.inSource(m) {
			apply["mother_name"] = m
			parts = append(parts, "mãe "+m)
		}
		if p.Idade > 0 && p.Idade < 120 && (sp.Age == nil || *sp.Age != p.Idade) && rr.inSource(strconv.Itoa(p.Idade)) {
			apply["age"] = p.Idade
			parts = append(parts, fmt.Sprintf("idade %d", p.Idade))
		}
		if e := clean(p.Endereco); e != "" && flat(e) != flat(sp.Address) && rr.inSource(e) {
			apply["address"] = e
			parts = append(parts, "endereço "+e)
		}
		if len(apply) == 0 {
			continue
		}
		seen[p.Ref] = true
		cur := sp.Name
		if sp.MotherName != "" {
			cur += " · mãe " + sp.MotherName
		}
		if sp.Age != nil {
			cur += fmt.Sprintf(" · idade %d", *sp.Age)
		}
		out = append(out, Suggestion{Kind: KindPessoaDados, Code: "releitura", Target: "Envolvido: " + sp.Name, PersonID: sp.ID,
			Value: strings.Join(parts, " · "), Current: cur, Confianca: confOf(p.Confianca), Motivo: strings.TrimSpace(p.Motivo),
			Applicable: true, Apply: apply})
	}
	return out, nil
}

// fieldValue normaliza e confere o valor proposto para o campo.
func (rr *Reread) fieldValue(key, raw string) (string, bool) {
	v := clean(raw)
	if v == "" {
		return "", false
	}
	switch key {
	case "ciops_record":
		v = strings.ToUpper(strings.ReplaceAll(v, " ", ""))
		return v, reCIOPS.MatchString(v) && strings.Contains(rr.source, v)
	case "occurred_on":
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return "", false
		}
		return v, strings.Contains(rr.source, t.Format("02/01/2006"))
	case "start_time", "end_time":
		t, err := time.Parse("15:04", v)
		if err != nil {
			return "", false
		}
		v = t.Format("15:04")
		h, m := t.Format("15"), t.Format("04")
		src := strings.ToLower(strings.ReplaceAll(rr.source, " ", ""))
		ok := strings.Contains(src, h+":"+m) || strings.Contains(src, h+"h"+m)
		if m == "00" {
			ok = ok || regexp.MustCompile(`(^|\D)`+h+`h(\D|$)`).MatchString(src)
		}
		return v, ok
	}
	return v, rr.inSource(v)
}

// inSource: o valor está escrito no trecho (sem diferença de caixa, acento,
// pontuação, espaço ou quebra de linha) — inteiro ou, quando a célula da
// tabela quebrou em duas linhas com texto de outra coluna no meio, com todas
// as palavras na mesma ordem do trecho. Palavra que não está lá não passa.
func (rr *Reread) inSource(v string) bool {
	f := flat(v)
	if f == "" {
		return false
	}
	if strings.Contains(rr.flat, f) {
		return true
	}
	words := strings.Fields(intel.Normalize(v))
	j := 0
	for _, t := range rr.tokens {
		if j < len(words) && t == words[j] {
			j++
		}
	}
	return j == len(words)
}

func flat(s string) string { return strings.ReplaceAll(intel.Normalize(s), " ", "") }

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

func confOf(c string) string {
	switch c {
	case "alta", "media", "baixa":
		return c
	}
	return "media"
}

// codeOfField diz a pendência do SIPOM que o campo costuma resolver (a tela
// agrupa por ela); sem uma específica, "releitura".
func codeOfField(key string) string {
	switch key {
	case "ciops_record":
		return sipom.PendFicha
	case "occurred_on", "start_time", "end_time":
		return sipom.PendDataHora
	case "place_address":
		return sipom.PendLogradouro
	case "place_city":
		return sipom.PendCidade
	case "place_neighborhood":
		return sipom.PendBairro
	case "police_station":
		return sipom.PendDelegacia
	case "delegate":
		return sipom.PendDelegado
	case "procedure_type":
		return sipom.PendProcTipo
	case "procedure_number":
		return sipom.PendProcNumero
	}
	return "releitura"
}
