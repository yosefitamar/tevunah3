package sipomai

import (
	"testing"

	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

func TestDelegaciaNeedsPending(t *testing.T) {
	so := &opsreport.StoredOccurrence{}
	so.Sipom.Pending = []string{sipom.PendPessoaSemDossie}
	if BuildDelegacia(&sipom.Catalog{}, so) != nil {
		t.Fatal("sem pendência de delegacia não pede")
	}
}

func TestDelegaciaParseRejectsUnknown(t *testing.T) {
	d := &DelegaciaAsk{cat: &sipom.Catalog{}, so: &opsreport.StoredOccurrence{}}
	for _, raw := range []string{`{"delegacia_id":0,"confianca":"alta","motivo":""}`, `{"delegacia_id":99999,"confianca":"alta","motivo":"x"}`} {
		got, err := d.Parse([]byte(raw))
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("%s → %v %v (esperava lista vazia)", raw, got, err)
		}
	}
}
