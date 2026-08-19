package incidents

import (
	"strings"
	"testing"
)

// Relatórios reais de quatro batalhões, colados sem edição. Cada um traz uma
// variação que já quebrou o parser: cabeçalho de mensagem do WhatsApp,
// negrito, bullets, rótulo diferente, vítima sobrevivente e tentativa.

const reportCascavel = `[09:55, 18/08/2026] Sd Alcantara SAI 15° BPM: RELATÓRIO DE CVLI / AIS15 

NATUREZA: HOMICÍDIO DOLOSO
DATA: 18/08/2026
HORÁRIO: 23h01min
ENDEREÇO: Rua Coronel Bia, 2597, Centro, Cascavel/CE
ORCRIM DO LOCAL: CV

LINK DO GOOGLE MAPS: https://maps.app.goo.gl/J23V6T4zKw5pwiiy5

VÍTIMA(S):
* Francisco Denilson Galdino da Silva (ÓBITO)
* Data Nascimento: 12/03/1999
* Mãe: Maria Iseuda Galdino Candido
* Endereço: Rua Marinheiro Antonio Silva, 2382, Centro, Cascavel/CE.
* Antecedentes: Já foi preso, porém, não consta registros no CIP.

* Ana Melissa Antunes Pinto (LESIONADA)
* Data Nascimento: 11/09/2003
* Mãe: Bianca Maria Oliveira Antunes
* Endereço: Rua Desembargador Lauro Nogueira, 457, Papicu, Fortaleza/CE.
* Antecedentes: Não Há.

CENÁRIO:
A vítima Francisco Denilson Galdino da Silva, 27 anos, foi encontrada já sem vida, apresentando múltiplos disparos, inclusive na cabeça. A perícia recolheu 06 estojos e 02 projéteis, com calibre provável .380.

ENVOLVIDOS:
Não identificados até o momento.

REVIDE:
Não.`

const reportQuixere = `[10:05, 18/08/2026] +55 88 9378-7827: RELATÓRIO 
1° BPM - AIS 01

NATUREZA: TENTATIVA DE HOMICIDIO 
DATA DA OCORRÊNCIA: 17/08/2026
HORA: 06:50
ENDEREÇO: PEDREIRA, ZONA RURAL, QUIXERÉ.

ORCRIM ATUANTE NO LOCAL: Predominância PCC, "em disputa CV x PCC".
LINK NO GOOGLE MAPS:
https://maps.app.goo.gl/4xJVy2oo7KgJX67MA

VÍTIMA: 
NOME: FRANCISCA ECILIANA OLIVEIRA
NASCIMENTO: 03/12/1990
MÃE: ROSILENE DA CONCEIÇÃO OLIVEIRA


CENÁRIO:
No dia 17/08/2026, por volta das 06h50min, à composição foi acionada para atender a uma ocorrência acerca de uma mulher que estaria caída ao solo, lesionada por disparos de arma de fogo, na localidade de Pedreira, zona rural do município de Quixeré/CE.

INTELIGÊNCIA:
Informes de que a vítima teria se deslocado até uma área sob domínio da ORCRIM PCC.

ENVOLVIDOS:
Sem informações 

REVIDE:
Sim, por parte da ORCRIM CV.`

const reportCamocim = `RELATÓRIO DE CVLI
28° BPM – AIS 28 – CAMOCIM

OCORRÊNCIA: M20260604946
NATUREZA: Homicídio PAF
DATA: 18AGO2026
HORÁRIO: 2h
LOCAL DO FATO: Rua da Quadra, Bitupitá, Barroquinha-CE
LINK NO GOOGLE MAPS: https://maps.app.goo.gl/pw41KvVUxWpN9yMH9

ORCRIM: Comando Vermelho (CV)

1. VÍTIMA
   Nome: Danilo Soares de Oliveira
   CPF: 080.500.713-08
   Data de Nascimento: 15JUN1990
   Mãe: Maria Marta Soares
   Pai: Benedito Costa Oliveira
   Naturalidade: Camocim – CE
   End.: Situação de rua
   Antecedentes: Sem antecedentes.

2. DOS FATOS
Na data de 18AGO2026, por volta das 2h00, a vítima foi encontrada sem vida em uma rede de dormir armada na trave da quadra de esportes, apresentando lesões por disparos de arma de fogo.

3. CENÁRIO LOCAL
A vítima encontrava-se em situação de rua e dormia no local no momento da execução.

4. ENVOLVIDOS
Indivíduos não identificados, vinculados ao CV.

5. REVIDE
   Não há indícios de revide.`

const reportRussas = `*RELATÓRIO DE CVLI*
*1° BPM - AIS 01*

*NATUREZA:* HOMICIDIO POR ARMA DE FOGO
*DATA DA OCORRÊNCIA:* 17/08/2026
*HORA:* 04:00
*ENDEREÇO:* BARRAGEM DAS PEDRINHAS, ZONA RURAL, RUSSAS.

*ORCRIM ATUANTE NO LOCAL:* CV, "em disputa CV x PCC".
*LINK NO GOOGLE MAPS:*
https://maps.app.goo.gl/LPC4Fk55h8qJU12W8?g_st=ic

*VÍTIMA:*
*NOME:* ALLAN CHARLLYS DOS SANTOS DA SILVA
*NASCIMENTO:* 07/12/1994
*MÃE:* FRANCISCA SOLANGE FRANÇA SANTOS


*CENÁRIO:*
No dia 17/08/2026, por volta das 04h00min, a composição foi acionada via CIOPS para atender a uma ocorrência de possível homicídio.

*ENVOLVIDOS:*
Sem informações 

*REVIDE:*
Sim, por parte da ORCRIM CV.`

func TestParseReport_Cascavel(t *testing.T) {
	got := ParseReport(reportCascavel)

	// O cabeçalho da mensagem ([09:55, 18/08/2026]) não pode virar data nem
	// hora da ocorrência.
	if got.OccurredOn != "2026-08-18" {
		t.Errorf("OccurredOn = %q, quero 2026-08-18", got.OccurredOn)
	}
	if got.OccurredTime != "23:01" {
		t.Errorf("OccurredTime = %q, quero 23:01 (não a hora da mensagem)", got.OccurredTime)
	}
	if got.Type != TypeHomicidio {
		t.Errorf("Type = %q", got.Type)
	}
	// NATUREZA não diz o meio ("HOMICÍDIO DOLOSO"); o cenário diz.
	if got.Means != MeansPAF {
		t.Errorf("Means = %q, quero paf (deduzido do cenário)", got.Means)
	}
	if got.City != "CASCAVEL" || got.Neighborhood != "CENTRO" {
		t.Errorf("lugar = %q/%q, quero CASCAVEL/CENTRO", got.City, got.Neighborhood)
	}
	if got.MapsURL != "https://maps.app.goo.gl/J23V6T4zKw5pwiiy5" {
		t.Errorf("MapsURL = %q", got.MapsURL)
	}

	if len(got.People) != 2 {
		t.Fatalf("People = %d, quero 2: %+v", len(got.People), got.People)
	}
	morta, viva := got.People[0], got.People[1]
	if morta.Name != "FRANCISCO DENILSON GALDINO DA SILVA" || morta.Role != RoleVitima {
		t.Errorf("vítima fatal = %+v", morta)
	}
	if morta.MotherName != "MARIA ISEUDA GALDINO CANDIDO" || morta.DateOfBirth != "1999-03-12" {
		t.Errorf("qualificação da vítima fatal = %+v", morta)
	}
	if !strings.Contains(morta.Notes, "ANTECEDENTES") {
		t.Errorf("antecedentes não foram preservados: %q", morta.Notes)
	}
	// O ponto crítico: LESIONADA sobreviveu. Marcá-la como VÍTIMA registraria
	// óbito de uma pessoa viva no acervo inteiro.
	if viva.Name != "ANA MELISSA ANTUNES PINTO" {
		t.Errorf("segunda pessoa = %+v", viva)
	}
	if viva.Role != "" {
		t.Errorf("sobrevivente recebeu papel %q — VÍTIMA implica óbito", viva.Role)
	}
	if viva.Status != StatusAlive {
		t.Errorf("Status = %q, quero %q", viva.Status, StatusAlive)
	}
	if !hasWarning(got.Warnings, "sobrevivente") {
		t.Errorf("faltou aviso sobre a sobrevivente: %v", got.Warnings)
	}
	// O endereço da vítima não pode contaminar o município da ocorrência.
	if strings.Contains(got.Description, "PAPICU") {
		t.Error("endereço da vítima vazou para a descrição da ocorrência")
	}
}

func TestParseReport_QuixereTentativa(t *testing.T) {
	got := ParseReport(reportQuixere)

	// Tentativa não é CVLI consumado: preencher "homicídio" registraria uma
	// morte que não houve.
	if got.Type != "" {
		t.Errorf("Type = %q, quero vazio (TENTATIVA DE HOMICIDIO)", got.Type)
	}
	if !hasWarning(got.Warnings, "TENTATIVA") {
		t.Errorf("faltou aviso de tentativa: %v", got.Warnings)
	}
	if got.OccurredOn != "2026-08-17" || got.OccurredTime != "06:50" {
		t.Errorf("data/hora = %q %q", got.OccurredOn, got.OccurredTime)
	}
	// "ZONA RURAL" não é bairro — a localidade anterior é.
	if got.City != "QUIXERÉ" || got.Neighborhood != "PEDREIRA" {
		t.Errorf("lugar = %q/%q, quero QUIXERÉ/PEDREIRA", got.City, got.Neighborhood)
	}
	// URL na linha seguinte ao rótulo.
	if got.MapsURL != "https://maps.app.goo.gl/4xJVy2oo7KgJX67MA" {
		t.Errorf("MapsURL = %q", got.MapsURL)
	}
	if len(got.People) != 1 {
		t.Fatalf("People = %d: %+v", len(got.People), got.People)
	}
	v := got.People[0]
	if v.Name != "FRANCISCA ECILIANA OLIVEIRA" || v.MotherName != "ROSILENE DA CONCEIÇÃO OLIVEIRA" {
		t.Errorf("vítima = %+v", v)
	}
	if v.DateOfBirth != "1990-12-03" {
		t.Errorf("nascimento = %q", v.DateOfBirth)
	}
	if v.Role != "" {
		t.Errorf("papel = %q, quero vazio (a vítima sobreviveu)", v.Role)
	}
	// A seção INTELIGÊNCIA tem valor operacional e não pode se perder.
	if !strings.Contains(got.Description, "INTELIGÊNCIA") {
		t.Errorf("descrição perdeu a seção de inteligência: %.200q", got.Description)
	}
}

func TestParseReport_Camocim(t *testing.T) {
	got := ParseReport(reportCamocim)

	if got.CIOPSRecord != "M20260604946" {
		t.Errorf("CIOPSRecord = %q, quero M20260604946", got.CIOPSRecord)
	}
	if got.Type != TypeHomicidio || got.Means != MeansPAF {
		t.Errorf("tipo/meio = %q/%q", got.Type, got.Means)
	}
	if got.OccurredOn != "2026-08-18" || got.OccurredTime != "02:00" {
		t.Errorf("data/hora = %q %q", got.OccurredOn, got.OccurredTime)
	}
	if got.City != "BARROQUINHA" || got.Neighborhood != "BITUPITÁ" {
		t.Errorf("lugar = %q/%q", got.City, got.Neighborhood)
	}
	if len(got.People) != 1 {
		t.Fatalf("People = %d: %+v", len(got.People), got.People)
	}
	v := got.People[0]
	if v.Name != "DANILO SOARES DE OLIVEIRA" || v.Role != RoleVitima {
		t.Errorf("vítima = %+v", v)
	}
	if v.CPF != "08050071308" {
		t.Errorf("CPF = %q, quero só dígitos", v.CPF)
	}
	if v.DateOfBirth != "1990-06-15" {
		t.Errorf("nascimento = %q (formato militar)", v.DateOfBirth)
	}
	if v.MotherName != "MARIA MARTA SOARES" {
		t.Errorf("mãe = %q", v.MotherName)
	}
	// "2. DOS FATOS" é o relato — não pode ficar de fora da descrição só por
	// não se chamar "CENÁRIO".
	if !strings.Contains(got.Description, "DOS FATOS") ||
		!strings.Contains(got.Description, "REDE DE DORMIR") {
		t.Errorf("descrição perdeu a seção dos fatos: %.200q", got.Description)
	}
	// "Indivíduos não identificados" não é gente qualificada.
	for _, p := range got.People {
		if p.Role == RoleAcusado {
			t.Errorf("prosa da seção de envolvidos virou acusado: %+v", p)
		}
	}
}

func TestParseReport_RussasNegrito(t *testing.T) {
	got := ParseReport(reportRussas)

	// Todo o relatório vem com o negrito do WhatsApp (*RÓTULO:*).
	if got.Type != TypeHomicidio || got.Means != MeansPAF {
		t.Errorf("tipo/meio = %q/%q", got.Type, got.Means)
	}
	if got.OccurredOn != "2026-08-17" || got.OccurredTime != "04:00" {
		t.Errorf("data/hora = %q %q", got.OccurredOn, got.OccurredTime)
	}
	if got.City != "RUSSAS" || got.Neighborhood != "BARRAGEM DAS PEDRINHAS" {
		t.Errorf("lugar = %q/%q", got.City, got.Neighborhood)
	}
	if got.MapsURL != "https://maps.app.goo.gl/LPC4Fk55h8qJU12W8?g_st=ic" {
		t.Errorf("MapsURL = %q", got.MapsURL)
	}
	if len(got.People) != 1 {
		t.Fatalf("People = %d: %+v", len(got.People), got.People)
	}
	v := got.People[0]
	if v.Name != "ALLAN CHARLLYS DOS SANTOS DA SILVA" || v.Role != RoleVitima {
		t.Errorf("vítima = %+v", v)
	}
	if v.DateOfBirth != "1994-12-07" || v.MotherName != "FRANCISCA SOLANGE FRANÇA SANTOS" {
		t.Errorf("qualificação = %+v", v)
	}
	if !hasWarning(got.Warnings, "ORCRIM") {
		t.Errorf("faltou aviso da ORCRIM: %v", got.Warnings)
	}
}

func hasWarning(warnings []string, needle string) bool {
	for _, w := range warnings {
		if strings.Contains(w, needle) {
			return true
		}
	}
	return false
}
