package opsreport

import (
	"reflect"
	"testing"

	"github.com/belia/tevunah/backend/internal/sipom"
)

func TestVehicleSendable(t *testing.T) {
	ano := 2019
	moto, carro := sipom.VeiculoTipoMotoneta, 6
	cases := []struct {
		name     string
		v        Vehicle
		complete bool
		want     bool
	}{
		{"placa e situação bastam", Vehicle{Plate: "ABC-1D23", SipomSituacao: 1}, false, true},
		{"sem placa, incompleto", Vehicle{Plate: " ", SipomSituacao: 1}, false, false},
		{"duas rodas sem placa", Vehicle{SipomSituacao: 1, SipomTipoCodigo: &moto}, true, true},
		{"quatro rodas sem placa", Vehicle{SipomSituacao: 1, SipomTipoCodigo: &carro}, true, false},
		{"sem situação", Vehicle{Plate: "ABC1D23"}, true, false},
		{"recusada, incompleta", Vehicle{Plate: "ABC1D23", SipomSituacao: 1, SipomCadastroExigido: true}, false, false},
		{"recusada, sem ano", Vehicle{Plate: "ABC1D23", SipomSituacao: 1, SipomCadastroExigido: true}, true, false},
		{"recusada, completa", Vehicle{Plate: "ABC1D23", SipomSituacao: 2, SipomCadastroExigido: true, SipomAnoFabricacao: &ano}, true, true},
	}
	for _, c := range cases {
		if got := VehicleSendable(c.v, c.complete); got != c.want {
			t.Errorf("%s: %v, quer %v", c.name, got, c.want)
		}
	}
}

func TestRejectedVehicles(t *testing.T) {
	p := &sipom.Payload{Materiais: []sipom.PayloadMaterial{
		{ID: "arma", TipoID: sipom.MaterialArma},
		{ID: "v1", TipoID: sipom.MaterialVeiculo},
		{ID: "v2", TipoID: sipom.MaterialVeiculo},
	}}
	got := RejectedVehicles(p, map[string]string{
		"materiais.2.codigo_tipo":    "obrigatório",
		"materiais.2.ano_fabricacao": "obrigatório",
		"materiais.0.arma_tipo_id":   "inválido", // arma: não é veículo
		"materiais.9.placa":          "fora do corpo",
		"ocorrencia.data_hora":       "inválida",
	})
	if !reflect.DeepEqual(got, []string{"v2"}) {
		t.Errorf("RejectedVehicles = %v, quer [v2]", got)
	}
}
