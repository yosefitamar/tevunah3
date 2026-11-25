package sipomai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// Delegacia: quando a tradução automática não identifica a delegacia (o
// relatório escreve "DMC", "PARAIPABA-CE", ou nada), a IA escolhe no
// catálogo do SIPOM pelo campo Delegacia, pelo histórico ("apresentado na
// delegacia municipal de Paraipaba") e pelo número do procedimento.
// Chamada própria: no schema principal não cabe mais nada.

// KindDelegacia: PUT .../sipom/procedimento com delegacia_id.
const KindDelegacia = "delegacia"

// DelegaciaAsk é o pedido da delegacia.
type DelegaciaAsk struct {
	Prompt string
	cat    *sipom.Catalog
	so     *opsreport.StoredOccurrence
}

// BuildDelegacia monta o pedido; nil quando a delegacia não está pendente.
func BuildDelegacia(cat *sipom.Catalog, so *opsreport.StoredOccurrence) *DelegaciaAsk {
	pend := false
	for _, p := range so.Sipom.Pending {
		pend = pend || p == sipom.PendDelegacia
	}
	if !pend || len(cat.Delegacias()) == 0 {
		return nil
	}
	o := &so.Occurrence
	var b strings.Builder
	b.WriteString("# DELEGACIA — escolha a delegacia do SIPOM em que a ocorrência foi apresentada\n")
	b.WriteString("Use o campo Delegacia do relatório (sigla, cidade ou nome), o que o histórico diz (\"apresentado na delegacia de…\") ")
	b.WriteString("e o código que abre o número do procedimento (\"939-1234/2026\" → delegacia 939). Delegacia municipal é a da cidade. ")
	b.WriteString("Se nenhuma servir com segurança, delegacia_id 0.\n\n")
	fmt.Fprintf(&b, "Cidade da ocorrência: %s\n", orDash(o.PlaceCity))
	fmt.Fprintf(&b, "Delegacia (relatório): %s\n", orDash(o.PoliceStation))
	fmt.Fprintf(&b, "Delegado(a) (relatório): %s\n", orDash(o.Delegate))
	fmt.Fprintf(&b, "Procedimento (relatório): %s %s\n", o.ProcedureType, o.ProcedureNumber)
	fmt.Fprintf(&b, "\n## HISTÓRICO\n%s\n", strings.TrimSpace(o.Narrative))
	b.WriteString("\n## DELEGACIAS DO SIPOM (id | nome — o número antes do hífen é o código)\n")
	for _, d := range cat.Delegacias() {
		fmt.Fprintf(&b, "%d | %s\n", d.ID, d.Nome)
	}
	return &DelegaciaAsk{Prompt: b.String(), cat: cat, so: so}
}

// DelegaciaSchema é o formato da resposta.
var DelegaciaSchema = obj(map[string]any{
	"delegacia_id": map[string]any{"type": "integer"},
	"confianca":    confianca, "motivo": motivo,
})

// Parse confere a resposta: só delegacia que existe no catálogo.
func (d *DelegaciaAsk) Parse(raw []byte) ([]Suggestion, error) {
	var a struct {
		ID        int    `json:"delegacia_id"`
		Confianca string `json:"confianca"`
		Motivo    string `json:"motivo"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if a.ID <= 0 {
		return []Suggestion{}, nil
	}
	dl, ok := d.cat.Delegacia(a.ID)
	if !ok {
		return []Suggestion{}, nil
	}
	cur := strings.TrimSpace(d.so.PoliceStation)
	if cur == "" {
		cur = "vazio"
	}
	return []Suggestion{{Kind: KindDelegacia, Code: sipom.PendDelegacia, Target: "Procedimento: Delegacia",
		Value: dl.Nome, Current: "relatório: " + cur, Confianca: confOf(a.Confianca), Motivo: strings.TrimSpace(a.Motivo),
		Applicable: true, Apply: map[string]any{"delegacia_id": dl.ID}}}, nil
}
