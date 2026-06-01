package authz

import "testing"

func TestCatalogUniqueCodes(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range Catalog {
		if a.Code == "" {
			t.Fatalf("ação com código vazio: %+v", a)
		}
		if seen[a.Code] {
			t.Fatalf("código duplicado no catálogo: %s", a.Code)
		}
		seen[a.Code] = true
		if a.Label == "" || a.Group == "" {
			t.Errorf("ação %s sem label/grupo", a.Code)
		}
	}
}

func TestIsValidAction(t *testing.T) {
	if !IsValidAction("report.diffuse") {
		t.Error("report.diffuse deveria ser válida")
	}
	if IsValidAction("report.inexistente") {
		t.Error("ação inexistente não deveria validar")
	}
}

func TestGovernanceFlag(t *testing.T) {
	// As únicas ações de governança esperadas são admin.permissions.*.
	want := map[string]bool{
		"admin.permissions.read":   true,
		"admin.permissions.update": true,
	}
	for _, a := range Catalog {
		if a.Governance != want[a.Code] {
			t.Errorf("ação %s: Governance=%v, esperado %v", a.Code, a.Governance, want[a.Code])
		}
	}
}

func TestActionCodesMatchesCatalog(t *testing.T) {
	if len(ActionCodes()) != len(Catalog) {
		t.Fatalf("ActionCodes (%d) != Catalog (%d)", len(ActionCodes()), len(Catalog))
	}
}
