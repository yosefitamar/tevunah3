package authz

import "testing"

func TestCombine(t *testing.T) {
	cases := []struct {
		name  string
		perms []Permission
		want  Decision
	}{
		{
			name:  "empty -> denied",
			perms: nil,
			want:  Decision{},
		},
		{
			name: "single role allowed, no dual",
			perms: []Permission{
				{RoleCode: "administrador", Allowed: true},
			},
			want: Decision{Allowed: true},
		},
		{
			name: "single role allowed with dual",
			perms: []Permission{
				{RoleCode: "administrador", Allowed: true, RequiresDualApproval: true, ApproverRole: "gestor"},
			},
			want: Decision{Allowed: true, RequiresDualApproval: true, ApproverRole: "gestor"},
		},
		{
			name: "multi role: dual + no-dual -> no dual wins (most permissive)",
			perms: []Permission{
				{RoleCode: "administrador", Allowed: true, RequiresDualApproval: true, ApproverRole: "gestor"},
				{RoleCode: "gestor", Allowed: true},
			},
			want: Decision{Allowed: true},
		},
		{
			name: "multi role: both require dual",
			perms: []Permission{
				{RoleCode: "administrador", Allowed: true, RequiresDualApproval: true, ApproverRole: "gestor"},
				{RoleCode: "analista", Allowed: true, RequiresDualApproval: true, ApproverRole: "gestor"},
			},
			want: Decision{Allowed: true, RequiresDualApproval: true, ApproverRole: "gestor"},
		},
		{
			name: "explicit deny ignored when another role permits",
			perms: []Permission{
				{RoleCode: "agente", Allowed: false},
				{RoleCode: "analista", Allowed: true},
			},
			want: Decision{Allowed: true},
		},
		{
			name: "only denied rows -> denied",
			perms: []Permission{
				{RoleCode: "agente", Allowed: false},
				{RoleCode: "analista", Allowed: false},
			},
			want: Decision{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Combine(c.perms)
			if got != c.want {
				t.Fatalf("Combine() = %+v, want %+v", got, c.want)
			}
		})
	}
}
