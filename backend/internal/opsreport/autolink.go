package opsreport

import (
	"fmt"
	"strings"
)

// ReportsSubdir é a subpasta de PHOTO_DIR onde ficam os PDFs importados
// ("<sha256>.pdf").
const ReportsSubdir = "ops-reports"

// AutoLinkCandidate é um dossiê homônimo da pessoa do relatório;
// MotherMatched diz se o nome da mãe também bate.
type AutoLinkCandidate struct {
	ID            string
	MotherMatched bool
}

// DecideAutoLink aplica a regra do vínculo automático pessoa → dossiê, a
// mesma para a importação pela tela e em lote.
//
// Vincula sozinho só quando nome E mãe batem com exatamente UM dossiê, e esse
// dossiê não está em óbito (a menos que o próprio relatório anote o óbito).
// Qualquer outro caso volta sem vínculo e com o aviso para o analista. Sem
// nome da mãe — vítimas e testemunhas nunca o trazem — não há vínculo
// automático.
func DecideAutoLink(p Person, cands []AutoLinkCandidate, deceased func(entityID string) (bool, error)) (auto, warning string, err error) {
	var strong []string
	for _, c := range cands {
		if c.MotherMatched {
			strong = append(strong, c.ID)
		}
	}
	switch {
	case len(cands) == 0:
		return "", "", nil
	case p.MotherName == "":
		return "", "Sem nome da mãe no relatório — confira o homônimo antes de vincular.", nil
	case len(strong) == 0:
		return "", "Homônimo com mãe diferente — não vinculado.", nil
	case len(strong) > 1:
		return "", fmt.Sprintf("%d dossiês com o mesmo nome e mãe — escolha o correto.", len(strong)), nil
	}
	dead, err := deceased(strong[0])
	if err != nil {
		return "", "", err
	}
	if dead && !strings.Contains(strings.ToUpper(p.Note), "ÓBITO") {
		return "", "O dossiê que casa está marcado como óbito — confira antes de vincular.", nil
	}
	return strong[0], "", nil
}
