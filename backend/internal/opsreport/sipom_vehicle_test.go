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
		{"placa sem cor nem marca/modelo", Vehicle{Plate: "ABC-1D23", SipomSituacao: 1}, sipom.PendVeiculo},
		{"placa sem marca/modelo", Vehicle{Plate: "ABC-1D23", SipomSituacao: 1, SipomCorCodigo: &cor}, sipom.PendVeiculo},
		{"placa, cor e marca/modelo", Vehicle{Plate: "ABC-1D23", SipomSituacao: 1, SipomCorCodigo: &cor, SipomMarcaModeloCodigo: &mm}, ""},
		{"sem situação", Vehicle{Plate: "ABC1D23"}, sipom.PendVeiculo},
		{"recusada, sem ano", func() Vehicle { v := completo(&moto); v.Plate, v.SipomCadastroExigido = "ABC1D23", true; return v }(), sipom.PendVeiculo},
		{"recusada, completa", func() Vehicle {
			v := completo(&moto)
			v.Plate, v.SipomCadastroExigido, v.SipomAnoFabricacao = "ABC1D23", true, &ano
			return v
		}(), ""},
		// Sem placa, a falta de situação, cor ou marca/modelo não importa: vai
		// como "Outros" (recomendação do SIPOM).
		{"sem placa, incompleto", Vehicle{Plate: " ", SipomSituacao: 1, SipomTipoCodigo: &moto}, sipom.PendVeiculoPlaca},
		{"sem placa, sem tipo, confirmada", confirmado(Vehicle{}), ""},
		{"motoneta sem placa, sem situação", Vehicle{SipomTipoCodigo: &motoneta}, ""},
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

func TestVehicleAsOutros(t *testing.T) {
	motoneta, moto := sipom.VeiculoTipoMotoneta, 4
	v := Vehicle{Kind: "MOTOCICLETA", Brand: "HONDA", Model: "TITAN", Color: "VERMELHA", SipomTipoCodigo: &moto}
	if VehicleAsOutros(v) {
		t.Fatal("moto sem placa só vira Outros depois da confirmação")
	}
	v.SipomSemPlacaConfirmada = true
	if !VehicleAsOutros(v) || OutrosDescricao(v) != "MOTOCICLETA HONDA TITAN, cor VERMELHA, sem placa identificada" {
		t.Fatalf("descrição montada: %q", OutrosDescricao(v))
	}
	v.SipomOutrosDescricao = "Honda Titan vermelha, chassi raspado"
	if OutrosDescricao(v) != "Honda Titan vermelha, chassi raspado" {
		t.Fatal("descrição do analista deve valer")
	}
	if !VehicleAsOutros(Vehicle{SipomTipoCodigo: &motoneta}) {
		t.Fatal("ciclomotor sem placa vai direto como Outros")
	}
	if VehicleAsOutros(Vehicle{Plate: "ABC1D23", SipomSemPlacaConfirmada: true}) {
		t.Fatal("com placa não é Outros")
	}
}
