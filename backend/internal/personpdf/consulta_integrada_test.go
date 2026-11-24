package personpdf

import (
	"reflect"
	"testing"

	"github.com/belia/tevunah/backend/internal/opsreport"
)

// Geometria da CONSULTA INTEGRADA real (rótulos em x=54, valores em x=159,
// segunda coluna em x≈328–420) com dados FICTÍCIOS.
func consultaFixture() []opsreport.Page {
	p1 := []fixLine{
		ln(81, 11.1, cell{226, "CONSULTA INTEGRADA"}),
		ln(108, 10.2, cell{257, "Dados da Pessoa"}),
		ln(123, 9.2, cell{54, "Nome:"}, cell{161, "FULANO DE TAL FICTICIO"}),
		ln(138, 9.2, cell{54, "Filiação:"}, cell{161, "BELTRANA DE TAL"}),
		ln(153, 9.2, cell{161, "null"}),
		ln(168, 9.2, cell{54, "Data de Nascimento:"}, cell{159, "03/08/2000"}, cell{381, "Nº SIP:"}, cell{426, "1234567890"}),
		ln(183, 9.2, cell{54, "RG:"}, cell{159, "1234567-8 - SSP"}, cell{381, "CPF:"}),
		ln(198, 9.2, cell{54, "Alcunhas:"}, cell{159, "TALZINHO, FULA"}),
		ln(216, 10.2, cell{259, "Último Endereço"}),
		ln(232, 10.2, cell{54, "R PRINCIPAL, 10 - CENTRO - CAUCAIA CE."}, cell{284, "Telefone: 85999990000"}),
		ln(247, 10.2, cell{233, "Ocorrência(s) como Infrator"}),
		ln(261, 9.2, cell{54, "Natureza do Fato:"}, cell{159, "ROUBO (OUTROS) - CONSUMADO"}),
		ln(275, 9.2, cell{54, "Delegacia:"}, cell{159, "DELEGACIA DO 31. DISTRITO POLICIAL"}),
		ln(289, 9.2, cell{54, "Tipo Procedimento:"}, cell{159, "INQUERITO"}, cell{328, "Nº Procedimento:"}, cell{420, "131 - 36/2021"}),
		ln(303, 9.2, cell{54, "Data Ocorrência:"}, cell{159, "20/04/2021 10:30:00"}, cell{328, "Data Instauração:"}, cell{420, "03/05/2021"}),
		ln(810, 7.4, cell{40, "Data Hora: 24/11/2026 00:43:15"}, cell{506, "Pag. 1 de 2"}),
	}
	p2 := []fixLine{
		ln(81, 11.1, cell{226, "CONSULTA INTEGRADA"}),
		ln(108, 10.2, cell{220, "Ocorrência(s) como Testemunha"}),
		ln(122, 9.2, cell{54, "Natureza Fato:"}, cell{159, "POSSE OU PORTE ILEGAL DE ARMA DE FOGO DE USO RESTRITO -"}),
		ln(136, 9.2, cell{54, "Delegacia:"}, cell{159, "DELEGACIA METROPOLITANA DE CAUCAIA"}),
		ln(150, 9.2, cell{54, "Tipo Procedimento:"}, cell{159, "BO"}, cell{328, "Nº Procedimento:"}, cell{420, "132 - 3888/2026"}),
		ln(164, 9.2, cell{54, "Data Ocorrência:"}, cell{159, "03/06/2021 10:00:00"}),
		ln(810, 7.4, cell{40, "Data Hora: 24/11/2026 00:43:15"}, cell{506, "Pag. 2 de 2"}),
	}
	return []opsreport.Page{page(p1...), page(p2...)}
}

func TestParseConsultaIntegrada(t *testing.T) {
	rep, err := Parse(consultaFixture())
	if err != nil {
		t.Fatal(err)
	}
	check := func(name, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	check("format", rep.Format, FormatConsultaIntegrada)
	check("name", rep.Name, "FULANO DE TAL FICTICIO")
	check("mother", rep.MotherName, "BELTRANA DE TAL")
	check("father", rep.FatherName, "")
	check("birth", rep.BirthDate, "2000-08-03")
	check("sip", rep.SIPNumber, "1234567890")
	check("cpf", rep.CPF, "")
	if rep.GeneratedAt == nil || rep.GeneratedAt.UTC().Format("2006-01-02T15:04:05") != "2026-11-24T03:43:15" {
		t.Errorf("generated_at = %v", rep.GeneratedAt)
	}
	if want := []string{"TALZINHO", "FULA"}; !reflect.DeepEqual(rep.Aliases, want) {
		t.Errorf("aliases = %v", rep.Aliases)
	}
	if want := []RG{{Number: "1234567-8", Issuer: "SSP"}}; !reflect.DeepEqual(rep.RGs, want) {
		t.Errorf("rgs = %+v", rep.RGs)
	}
	if want := []string{"85999990000"}; !reflect.DeepEqual(rep.Phones, want) {
		t.Errorf("phones = %v", rep.Phones)
	}
	wantAddr := []Address{{Street: "R PRINCIPAL", Number: "10", Neighborhood: "CENTRO", City: "CAUCAIA", State: "CE"}}
	if !reflect.DeepEqual(rep.Addresses, wantAddr) {
		t.Errorf("addresses = %+v", rep.Addresses)
	}
	wantRec := []Record{
		{
			Category: "policial", Role: "INFRATOR", Source: SourceConsultaIntegrada,
			Title: "ROUBO (OUTROS) - CONSUMADO", Agency: "DELEGACIA DO 31. DISTRITO POLICIAL",
			ProcedureType: "INQUERITO", Number: "131 - 36/2021", Date: "2021-04-20", FiledOn: "2021-05-03",
		},
		{
			Category: "policial", Role: "TESTEMUNHA", Source: SourceConsultaIntegrada,
			Title: "POSSE OU PORTE ILEGAL DE ARMA DE FOGO DE USO RESTRITO", Agency: "DELEGACIA METROPOLITANA DE CAUCAIA",
			ProcedureType: "BO", Number: "132 - 3888/2026", Date: "2021-06-03",
		},
	}
	if !reflect.DeepEqual(rep.Criminal, wantRec) {
		t.Errorf("criminal =\n%+v\nwant\n%+v", rep.Criminal, wantRec)
	}
}
