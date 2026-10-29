package opsreport

import (
	"testing"

	"github.com/belia/tevunah/backend/internal/sipom"
)

func TestIntelPayload(t *testing.T) {
	so := &StoredOccurrence{IntelParticipation: true}
	so.IntelOfficers = []IntelOfficer{
		{UserID: "a", Registration: "30079116-2", FuncaoID: sipom.FuncaoAgente},    // já na guarnição
		{UserID: "b", Registration: " 3001234-5 ", FuncaoID: sipom.FuncaoAnalista}, // entra
		{UserID: "c", Registration: "", FuncaoID: sipom.FuncaoAgente},              // sem matrícula
		{UserID: "d", Registration: "3009999-1", FuncaoID: 1},                      // função de outro policiamento
		{UserID: "e", Registration: "30012345", FuncaoID: sipom.FuncaoGestor},      // repetida
		{UserID: "f", Registration: "3055681-x", FuncaoID: sipom.FuncaoAgente},     // já na guarnição (X)
	}
	seen := map[string]bool{RegistrationKey("300791162"): true, RegistrationKey("3055681X"): true}
	got := intelPayload(so, seen)
	if len(got) != 1 {
		t.Fatalf("composição = %+v, quer só o agente b", got)
	}
	if g := got[0]; g.ID != "intel-b" || g.Matricula != "3001234-5" ||
		g.PoliciamentoTipoID != sipom.PoliciamentoInteligencia || g.FuncaoID != sipom.FuncaoAnalista {
		t.Errorf("item = %+v", g)
	}

	so.IntelParticipation = false
	if got := intelPayload(so, map[string]bool{}); len(got) != 0 {
		t.Errorf("sem participação, composição = %+v", got)
	}
}
