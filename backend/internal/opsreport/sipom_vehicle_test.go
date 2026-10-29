package opsreport

import (
	"reflect"
	"testing"

	"github.com/belia/tevunah/backend/internal/sipom"
)

func TestVehiclePending(t *testing.T) {
	ano := 2019
	motoneta, moto, carro, cor, mm := sipom.VeiculoTipoMotoneta, 4, 6, 5, 2100
	completo := func(tipo *int) Vehicle {
		return Vehicle{SipomSituacao: 1, SipomTipoCodigo: tipo, SipomCorCodigo: &cor, SipomMarcaModeloCodigo: &mm}
	}
	confirmado := func(v Vehicle) Vehicle { v.SipomSemPlacaConfirmada = true; return v }
	cases := []struct {
		name string
		v    Vehicle
		want string
	}{
		{"placa e situação bastam", Vehicle{Plate: "ABC-1D23", SipomSituacao: 1}, ""},
		{"sem situação", Vehicle{Plate: "ABC1D23"}, sipom.PendVeiculo},
		{"recusada, sem ano", func() Vehicle { v := completo(&moto); v.Plate, v.SipomCadastroExigido = "ABC1D23", true; return v }(), sipom.PendVeiculo},
		{"recusada, completa", func() Vehicle {
			v := completo(&moto)
			v.Plate, v.SipomCadastroExigido, v.SipomAnoFabricacao = "ABC1D23", true, &ano
			return v
		}(), ""},
		{"sem placa, incompleto", Vehicle{Plate: " ", SipomSituacao: 1, SipomTipoCodigo: &moto}, sipom.PendVeiculo},
		{"motoneta sem placa", completo(&motoneta), ""},
		{"moto sem placa, sem confirmar", completo(&moto), sipom.PendVeiculoPlaca},
		{"carro sem placa, sem confirmar", completo(&carro), sipom.PendVeiculoPlaca},
		{"moto sem placa, confirmada", confirmado(completo(&moto)), ""},
		{"carro sem placa, confirmado", confirmado(completo(&carro)), ""},
	}
	for _, c := range cases {
		if got := VehiclePending(c.v); got != c.want {
			t.Errorf("%s: %q, quer %q", c.name, got, c.want)
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
