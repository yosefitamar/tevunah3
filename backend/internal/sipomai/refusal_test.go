package sipomai

import (
	"strings"
	"testing"

	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// A recusa do SIPOM vira pedido próprio, liga "materiais.N" ao item da ficha
// e só aceita ano que está no texto ou vem de orientação citada.
func TestRefusal(t *testing.T) {
	so := &opsreport.StoredOccurrence{ID: "x"}
	so.Narrative = "Abordada a motocicleta Honda Fan de placa PMB3E12, fabricada em 2019."
	so.Sipom.Pending = []string{sipom.PendEnvioRecusado}
	so.Sipom.Send.Erro = "SIPOM 422 validacao: Revise os campos indicados.; materiais.0.ano_fabricacao: Campo obrigatório."
	cor, mm, tipo := 5, 2100, 4
	so.Vehicles = []opsreport.Vehicle{
		{Kind: "MOTOCICLETA", Brand: "HONDA", Model: "FAN", Plate: "PMB3E12", Color: "PRATA", SipomSituacao: 1,
			SipomTipoCodigo: &tipo, SipomCorCodigo: &cor, SipomMarcaModeloCodigo: &mm, SipomCadastroExigido: true},
	}
	guid := []Guidance{{ID: "g1", Topic: "recusa", Body: "Sem ano, vai como OUTROS", Author: "A"}}
	req, ok := Build(nil, nil, so, guid)
	if !ok || req.RefusalPrompt == "" || req.Prompt != "" {
		t.Fatalf("só a recusa deveria gerar pedido: ok=%v principal=%q", ok, req.Prompt)
	}
	if !strings.Contains(req.RefusalPrompt, "materiais.0 = veiculo 0") || !strings.Contains(req.RefusalPrompt, "Sem ano, vai como OUTROS") {
		t.Fatalf("pedido da recusa sem o item ou a orientação:\n%s", req.RefusalPrompt)
	}
	got, err := req.ParseRefusal([]byte(`{"explicacao":"Falta o ano de fabricação.","acoes":[
		{"acao":"ano","material":"veiculo","item":0,"ano_fabricacao":2015,"ano_modelo":0,"descricao":"","confianca":"alta","motivo":"x","orientacao":""},
		{"acao":"ano","material":"veiculo","item":0,"ano_fabricacao":2019,"ano_modelo":2020,"descricao":"","confianca":"alta","motivo":"texto","orientacao":""},
		{"acao":"outros","material":"veiculo","item":0,"ano_fabricacao":0,"ano_modelo":0,"descricao":"Moto Honda Fan prata, placa PMB3E12","confianca":"alta","motivo":"orientação","orientacao":"G1"},
		{"acao":"remover","material":"droga","item":3,"ano_fabricacao":0,"ano_modelo":0,"descricao":"","confianca":"alta","motivo":"x","orientacao":""}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]Suggestion{}
	for _, s := range got {
		kinds[s.Kind+":"+s.Value] = s
	}
	if len(got) != 3 {
		t.Fatalf("quer explicação + ano 2019 + outros (o ano estimado e o item inexistente caem): %+v", got)
	}
	if s, ok := kinds["veiculo_ano:ano de fabricação 2019 · modelo 2020"]; !ok || s.Confianca != "media" {
		t.Errorf("ano do texto, sem orientação, vale com confiança média: %+v", got)
	}
	if s, ok := kinds["veiculo_como_outros:enviar como OUTROS: Moto Honda Fan prata, placa PMB3E12"]; !ok || s.Confianca != "alta" || s.Guidance == nil {
		t.Errorf("OUTROS pela orientação mantém confiança alta e a citação: %+v", got)
	}
}

func TestPayloadMaterialRefs(t *testing.T) {
	tipo, droga, q := 1, 1, 10.0
	so := &opsreport.StoredOccurrence{}
	so.Weapons = []opsreport.Weapon{{SipomTipoID: &tipo}, {}}
	so.Drugs = []opsreport.Drug{{SipomDrogaID: &droga, SipomQuantidade: &q}}
	so.Vehicles = []opsreport.Vehicle{{Plate: "ABC1D23", SipomCadastroExigido: true}}
	refs := opsreport.PayloadMaterialRefs(so)
	want := []opsreport.MaterialRef{{Kind: "armas", Index: 0}, {Kind: "drogas", Index: 0}, {Kind: "veiculos", Index: 0}}
	if len(refs) != len(want) {
		t.Fatalf("refs: %+v", refs)
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Errorf("materiais.%d: %+v, quer %+v", i, refs[i], want[i])
		}
	}
}
