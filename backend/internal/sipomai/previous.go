package sipomai

import (
	"fmt"
	"strings"
)

// Previous é a análise anterior da ocorrência, com o resultado da revisão
// do analista. Entra em cada pedido da nova verificação: o que já foi
// aplicado não se repete, o que o analista não aceitou é reconsiderado e o
// que falhou ao gravar tem a causa corrigida.
type Previous struct {
	When        string // dd/mm/aaaa hh:mm
	Suggestions []Suggestion
	Applied     map[int]bool
	Rejected    map[int]bool
	Failed      map[int]string
	Reviewed    bool // o analista aplicou algo (há resultado registrado)
}

// Prompt é a seção do pedido; vazio quando a anterior não tinha sugestões.
func (p *Previous) Prompt() string {
	if p == nil || len(p.Suggestions) == 0 {
		return ""
	}
	line := func(s Suggestion) string {
		l := fmt.Sprintf("- %s → %s", s.Target, s.Value)
		if s.Current != "" {
			l += fmt.Sprintf(" (estava: %s)", s.Current)
		}
		return l
	}
	var applied, rejected, failed, open []string
	for i, s := range p.Suggestions {
		switch {
		case p.Applied[i]:
			applied = append(applied, line(s))
		case p.Failed[i] != "":
			failed = append(failed, line(s)+" — erro ao gravar: "+p.Failed[i])
		case p.Rejected[i]:
			r := line(s)
			if m := strings.TrimSpace(s.Motivo); m != "" {
				r += " — seu motivo foi: " + m
			}
			rejected = append(rejected, r)
		default:
			r := line(s)
			if !s.Applicable {
				r += " — não aplicável (faltava dado válido)"
			}
			open = append(open, r)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n# SUA ANÁLISE ANTERIOR DESTA OCORRÊNCIA (%s) — use para chegar a um resultado melhor\n", p.When)
	section := func(title string, items []string) {
		if len(items) > 0 {
			b.WriteString(title + "\n" + strings.Join(items, "\n") + "\n")
		}
	}
	section("Aplicadas pelo analista (já estão na ficha; não repita):", applied)
	section("Não aceitas pelo analista (ele revisou e não marcou): reconsidere — proponha de novo só com base mais forte no texto, e diga em motivo o que mudou; havendo outra opção melhor, prefira-a:", rejected)
	section("Falharam ao gravar: corrija a causa (opção inválida, dado faltando) ou não proponha:", failed)
	if p.Reviewed {
		section("Sem decisão registrada:", open)
	} else {
		section("Ainda não revisadas pelo analista (pediu nova verificação sem aplicar — revise com mais cuidado, sobretudo as de confiança baixa ou não aplicáveis):", open)
	}
	return b.String()
}
