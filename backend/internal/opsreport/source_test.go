package opsreport

import (
	"strings"
	"testing"
)

const layoutSample = `01/09/26, 18:55                                   Relatório de Ocorrências

                     RELATÓRIO DIÁRIO DE OCORRÊNCIAS
                                2ºBPRAIO
                            Data: 31/08/2026

         Natureza: ROUBO
         Base: CAUCAIA-1ªCIA/2ºBPRAIO
         Data: 31/08/2026 Hora Inicial: 10:00   Equipe: RAIO 01   Ficha Ciops: M20260640001
         Histórico: outra ocorrência.

         Natureza: TRÁFICO DE DROGAS
         Base: PARACURU-2ªCIA/2ºBPRAIO
         Data: 31/08/2026 Hora Inicial: 18h30   Hora Final: 01h15   Equipe: VTRA 088   Ficha Ciops: M20260640275
         Ocorrência Local: Rua A, 86 Bairro Ocorrência: CENTRO        Cidade
         Ocorrência: Paracuru
https://sgr.pm.ce.gov.br/impressao_ocorrencia_operacional/                 5/30
` + "\f" + `01/09/26, 18:55                                   Relatório de Ocorrências


         Nº do Proced.: 939-6868/2026
         Histórico:
         Texto da ficha.

         Natureza: FURTO
         Data: 31/08/2026 Hora Inicial: 20:00   Ficha Ciops: M20260640999
`

func TestOccurrenceSource(t *testing.T) {
	got := OccurrenceSource(layoutSample, "M20260640275OCORRNCIALOCAL")
	if !strings.HasPrefix(got, "Natureza: TRÁFICO DE DROGAS") {
		t.Fatalf("início errado:\n%s", got)
	}
	for _, want := range []string{"Ficha Ciops: M20260640275", "Ocorrência: Paracuru", "939-6868/2026", "Texto da ficha."} {
		if !strings.Contains(got, want) {
			t.Errorf("faltou %q:\n%s", want, got)
		}
	}
	for _, bad := range []string{"M20260640001", "M20260640999", "https://", "Relatório de Ocorrências", "FURTO"} {
		if strings.Contains(got, bad) {
			t.Errorf("não devia ter %q:\n%s", bad, got)
		}
	}
	if OccurrenceSource(layoutSample, "M20269999999") != "" || OccurrenceSource(layoutSample, "lixo") != "" {
		t.Error("ficha ausente devia dar vazio")
	}
}
