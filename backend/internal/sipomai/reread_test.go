package sipomai

import (
	"testing"
	"time"

	"github.com/belia/tevunah/backend/internal/opsreport"
)

const rereadSource = `Natureza: TRÁFICO DE DROGAS
Data: 31/08/2026 Hora Inicial: 18h30   Hora Final: 01h15   Equipe: VTRA 088   Ficha Ciops: M20260640275
Ocorrência Local: Rua Raimunda Moreira de Castro, 86 Bairro Ocorrência: CONJUNTO NOVA ESPERANÇA   Cidade
Ocorrência: Paracuru
ROBSON DE SOUSA NERI        DANIELE BATISTA DE SOUSA      21
Delegacia: DMC Delegado(a): José Carlos Alan Pereira
Nº do Proced.: 939-6868/2026
Objetos Agratas:
01(um) aparelho celular Redmi; 01(um) iPhone; 01(uma)
tesoura; 01(um) carregador telefônico.`

func rereadOcc() *opsreport.StoredOccurrence {
	age := 12
	so := &opsreport.StoredOccurrence{ID: "o1"}
	so.OccurredOn = time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	so.CIOPS = "M20260640275OCORRNCIALOCALRUA"
	so.Teams = "VTRA 088"
	so.PlaceCity = "PARACURU ABORDAGEM LOCAL:"
	so.PlaceNeighborhood = "CONJUNTO NOVA ESPERANÇA"
	so.ProcedureNumber = "939-6868/2026 Objetos Agratas: 01(um) aparelho celular Redmi"
	so.People = []opsreport.StoredPerson{{ID: "p1", Person: opsreport.Person{Role: "ACUSADO", Name: "ROBSON DE SOUSA NERI", Age: &age}}}
	return so
}

func TestRereadParse(t *testing.T) {
	so := rereadOcc()
	if !NeedsReread(so) {
		t.Fatal("ficha fora do formato devia pedir releitura")
	}
	rr := BuildReread(so, rereadSource)
	raw := []byte(`{"campos":[
		{"campo":"ciops_record","valor":"M20260640275","confianca":"alta","motivo":"x"},
		{"campo":"start_time","valor":"18:30","confianca":"alta","motivo":"x"},
		{"campo":"end_time","valor":"01:15","confianca":"alta","motivo":"x"},
		{"campo":"place_city","valor":"Paracuru","confianca":"alta","motivo":"x"},
		{"campo":"place_address","valor":"Rua Raimunda Moreira de Castro, 86","confianca":"alta","motivo":"x"},
		{"campo":"procedure_number","valor":"939-6868/2026","confianca":"alta","motivo":"x"},
		{"campo":"seized_objects","valor":"01(um) aparelho celular Redmi; 01(um) iPhone; 01(uma) tesoura; 01(um) carregador telefônico.","confianca":"alta","motivo":"x"},
		{"campo":"teams","valor":"VTRA 088","confianca":"alta","motivo":"igual: some"},
		{"campo":"place_neighborhood","valor":"Conjunto Nova Esperança","confianca":"alta","motivo":"igual sem caixa: some"},
		{"campo":"delegate","valor":"José Carlos Alan Pereira Filho","confianca":"alta","motivo":"inventado: some"},
		{"campo":"start_time","valor":"19:30","confianca":"alta","motivo":"repetido: some"},
		{"campo":"occurred_on","valor":"2026-09-01","confianca":"alta","motivo":"fora do texto: some"}
	],"pessoas":[
		{"ref":"P1","nome":"ROBSON DE SOUSA NERI","mae":"DANIELE BATISTA DE SOUSA","idade":21,"confianca":"alta","motivo":"x"},
		{"ref":"P9","nome":"X","mae":"","idade":0,"confianca":"alta","motivo":"ref inexistente"}
	]}`)
	got, err := rr.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"ciops_record": "M20260640275", "start_time": "18:30", "end_time": "01:15", "place_city": "Paracuru",
		"place_address": "Rua Raimunda Moreira de Castro, 86", "procedure_number": "939-6868/2026",
		"seized_objects": "01(um) aparelho celular Redmi; 01(um) iPhone; 01(uma) tesoura; 01(um) carregador telefônico.",
	}
	var person *Suggestion
	n := 0
	for i, s := range got {
		if s.Kind == KindPessoaDados {
			person = &got[i]
			continue
		}
		n++
		f, _ := s.Apply["field"].(string)
		if want[f] != s.Value {
			t.Errorf("%s = %q, esperado %q", f, s.Value, want[f])
		}
	}
	if n != len(want) {
		t.Errorf("campos: %d, esperado %d (%+v)", n, len(want), got)
	}
	if person == nil || person.PersonID != "p1" || person.Apply["mother_name"] != "DANIELE BATISTA DE SOUSA" ||
		person.Apply["age"] != 21 || person.Apply["name"] != nil {
		t.Errorf("pessoa: %+v", person)
	}
}

func TestLooksMisread(t *testing.T) {
	so := rereadOcc()
	so.CIOPS = "M20260640275"
	so.PlaceCity, so.ProcedureNumber = "PARACURU", "939-6868/2026"
	if LooksMisread(&so.Occurrence) {
		t.Error("ficha limpa não devia parecer mal lida")
	}
	so.ProcedureNumber = "939-6868/2026 Objetos Agratas: x"
	if !LooksMisread(&so.Occurrence) {
		t.Error("rótulo grudado devia parecer mal lido")
	}
}
