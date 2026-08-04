package pdf

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// sampleData é um RI completo — todo campo que poderia identificar a
// instituição ou o agente vem preenchido, pra que a ausência no modo
// descaracterizado seja resultado do template e não de dado faltando.
func sampleData(declassified bool) ReportData {
	d := ReportData{
		Number:          "07/2026",
		Status:          "difundido",
		DocDateBR:       "04/08/2026",
		DocDateShort:    "04AGO26",
		Subject:         "ASSUNTO SENSIVEL",
		Origin:          "SETOR DE INTELIGENCIA",
		Diffusion:       "COMANDO GERAL",
		PriorDiffusion:  "DIFUSAO ANTERIOR",
		Reference:       "REFERENCIA X",
		Attachments:     "ANEXO Y",
		Classification:  "S E C R E T O",
		BodyHTML:        template.HTML("<p>CORPO DO RELATORIO</p>"),
		GeneratedAtBR:   "04/08/2026 20:35",
		GeneratedByName: "FULANO DE TAL",
		AgentCode:       "7626",
		Declassified:    declassified,
		Qualifications: []QualificationData{
			{Name: "JOAO TESTE", Fields: []KV{{K: "ALCUNHA", V: "ZE"}}},
		},
	}
	d.TitleLine = titleLine(d)
	// Assets que o Render só produz fora do modo descaracterizado. Aqui vêm
	// preenchidos de propósito: se algum dia o Render mudar e passar a
	// carregá-los, os condicionais do template ainda precisam segurar.
	d.QRCodeData = template.URL("data:image/png;base64,QRPAYLOAD")
	d.BrasaoSAIData = template.URL("data:image/png;base64,BRASAOSAI")
	d.BrasaoInstitucional = template.URL("data:image/png;base64,BRASAOINST")
	d.FooterRibbonData = template.URL("data:image/png;base64,RIBBON")
	return d
}

func renderAll(t *testing.T, d ReportData) string {
	t.Helper()
	var all bytes.Buffer
	for name, tmpl := range map[string]*template.Template{
		"index":  indexTmpl,
		"header": headerTmpl,
		"footer": footerTmpl,
	} {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, d); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		all.Write(buf.Bytes())
	}
	return all.String()
}

// TestDeclassifiedOmitsIdentifiers é a garantia central da feature: nada que
// ligue o documento à instituição ou a quem o gerou pode sobreviver.
func TestDeclassifiedOmitsIdentifiers(t *testing.T) {
	out := renderAll(t, sampleData(true))

	forbidden := map[string]string{
		"RELATÓRIO INTERNO":     "barra de título",
		"SETOR DE INTELIGENCIA": "origem (tabela de metadados/título)",
		"COMANDO GERAL":         "difusão",
		"ASSUNTO SENSIVEL":      "assunto",
		"DIFUSAO ANTERIOR":      "difusão anterior",
		"REFERENCIA X":          "referência",
		"ANEXO Y":               "anexo",
		"04AGO26":               "data do documento",
		"FULANO DE TAL":         "nome do agente (carimbo forense)",
		"7626":                  "código do agente (carimbo forense)",
		"20:35":                 "hora da geração (carimbo forense)",
		"BRASAOSAI":             "brasão SAI",
		"BRASAOINST":            "brasão institucional",
		"RIBBON":                "faixa ornamental do rodapé",
		"QRPAYLOAD":             "QR code",
		"hidden-stamp":          "div do carimbo invisível",
	}
	for needle, what := range forbidden {
		if strings.Contains(out, needle) {
			t.Errorf("descaracterizado ainda expõe %s (encontrou %q)", what, needle)
		}
	}
}

// O que o descaracterizado PRECISA manter: conteúdo, qualificações, sigilo e
// o aviso legal (que não cita a instituição).
func TestDeclassifiedKeepsContent(t *testing.T) {
	out := renderAll(t, sampleData(true))

	required := map[string]string{
		"CORPO DO RELATORIO": "corpo do relatório",
		"JOAO TESTE":         "qualificações",
		"S E C R E T O":      "marcação de sigilo",
		"12.527":             "aviso legal do rodapé",
	}
	for needle, what := range required {
		if !strings.Contains(out, needle) {
			t.Errorf("descaracterizado perdeu %s (não encontrou %q)", what, needle)
		}
	}
}

// O RI normal não pode ter sido afetado pelos condicionais.
func TestNormalKeepsIdentifiers(t *testing.T) {
	out := renderAll(t, sampleData(false))

	required := map[string]string{
		"RELATÓRIO INTERNO":     "barra de título",
		"SETOR DE INTELIGENCIA": "origem",
		"ASSUNTO SENSIVEL":      "assunto",
		"FULANO DE TAL":         "carimbo forense",
		"BRASAOINST":            "brasão institucional",
		"RIBBON":                "faixa do rodapé",
		"QRPAYLOAD":             "QR code",
	}
	for needle, what := range required {
		if !strings.Contains(out, needle) {
			t.Errorf("RI normal perdeu %s (não encontrou %q)", what, needle)
		}
	}
}
