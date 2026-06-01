package authz

// Catálogo central de ações RBAC.
//
// Antes, as ações só existiam implicitamente como linhas semeadas em migrations
// e literais string espalhados nos handlers — não havia fonte única que
// enumerasse "todas as ações possíveis". Sem isso, a matriz de permissões só
// conseguia exibir/editar células (papel, ação) que já tinham sido semeadas,
// e conceder uma autorização nova a um papel exigia migration.
//
// Este catálogo passa a ser a fonte de verdade: alimenta a grade cheia da
// matriz (produto cartesiano papéis × ações), fornece rótulos/descrições em
// PT-BR para a UI e valida strings de ação na escrita.

// ActionDef descreve uma ação do sistema.
type ActionDef struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Group       string `json:"group"`
	Description string `json:"description"`
	// Governance marca ações que governam o próprio RBAC. São protegidas pela
	// guarda anti-lockout: o sistema impede que a última via de administração
	// seja removida pela matriz.
	Governance bool `json:"governance"`
}

// Catalog é a lista canônica de todas as ações. Mantenha em sincronia com os
// literais passados a requirePerm/policy.Can nos handlers.
var Catalog = []ActionDef{
	// ─── Admin ───────────────────────────────────────────────────────────
	{"admin.permissions.read", "Ver matriz de permissões", "ADMIN", "Visualizar a matriz RBAC.", true},
	{"admin.permissions.update", "Editar matriz de permissões", "ADMIN", "Alterar permissões de papéis na matriz RBAC.", true},

	// ─── Papéis ──────────────────────────────────────────────────────────
	{"role.read", "Listar papéis", "PAPÉIS", "Visualizar os papéis cadastrados.", false},
	{"role.create", "Criar papel", "PAPÉIS", "Criar papel customizado.", false},
	{"role.update", "Editar papel", "PAPÉIS", "Renomear (label) um papel.", false},
	{"role.delete", "Excluir papel", "PAPÉIS", "Excluir papel customizado.", false},

	// ─── Auditoria ───────────────────────────────────────────────────────
	{"audit.read", "Ler trilha de auditoria", "AUDITORIA", "Consultar o log de auditoria.", false},

	// ─── Entidades ───────────────────────────────────────────────────────
	{"entity.list", "Listar entidades", "ENTIDADES", "Listar entidades cadastradas.", false},
	{"entity.read", "Ver entidade", "ENTIDADES", "Abrir o detalhe de uma entidade.", false},
	{"entity.create", "Criar entidade", "ENTIDADES", "Cadastrar nova entidade.", false},
	{"entity.update", "Editar entidade", "ENTIDADES", "Editar dados de uma entidade.", false},
	{"entity.delete", "Excluir entidade", "ENTIDADES", "Excluir (soft delete) uma entidade.", false},
	{"entity.restore", "Restaurar entidade", "ENTIDADES", "Restaurar entidade excluída.", false},

	// ─── Relatórios ──────────────────────────────────────────────────────
	{"report.read", "Ver relatórios", "RELATÓRIOS", "Visualizar relatórios de inteligência.", false},
	{"report.create", "Criar relatório", "RELATÓRIOS", "Criar rascunho de relatório.", false},
	{"report.update", "Editar relatório", "RELATÓRIOS", "Editar rascunho de relatório.", false},
	{"report.diffuse", "Difundir relatório", "RELATÓRIOS", "Difundir (publicar) um relatório, alocando número.", false},
	{"report.undiffuse", "Reverter difusão", "RELATÓRIOS", "Devolver relatório difundido ao status de rascunho.", false},
	{"report.archive", "Arquivar relatório", "RELATÓRIOS", "Arquivar relatório difundido.", false},
	{"report.download", "Baixar relatório", "RELATÓRIOS", "Baixar o PDF de um relatório.", false},
	{"report.destroy", "Excluir rascunho", "RELATÓRIOS", "Excluir (soft delete) rascunho de relatório.", false},

	// ─── Sistema ─────────────────────────────────────────────────────────
	{"system.settings.read", "Ver configurações", "SISTEMA", "Ler configurações do sistema.", false},
	{"system.settings.update", "Editar configurações", "SISTEMA", "Alterar configurações do sistema.", false},

	// ─── Agentes (usuários) ──────────────────────────────────────────────
	{"user.list", "Listar agentes", "AGENTES", "Listar usuários do sistema.", false},
	{"user.read.self", "Ver próprio perfil", "AGENTES", "Ler o próprio perfil.", false},
	{"user.update.self", "Editar próprio perfil", "AGENTES", "Editar o próprio perfil (email, nome).", false},
	{"user.update.others", "Editar outros agentes", "AGENTES", "Editar email/nome de outros usuários.", false},
	{"user.create", "Criar agente", "AGENTES", "Cadastrar novo usuário.", false},
	{"user.role.assign", "Atribuir papéis", "AGENTES", "Alterar os papéis de um usuário.", false},
	{"user.clearance.set", "Definir clearance", "AGENTES", "Definir o nível de clearance de um usuário.", false},
	{"user.deactivate", "Desativar agente", "AGENTES", "Desativar um usuário.", false},
	{"user.password.reset", "Resetar senha", "AGENTES", "Forçar reset de senha de um usuário.", false},
	{"user.totp.reset", "Resetar TOTP", "AGENTES", "Limpar o TOTP de um usuário e forçar novo cadastro.", false},
}

// actionSet indexa o catálogo por código para validação O(1).
var actionSet = func() map[string]ActionDef {
	m := make(map[string]ActionDef, len(Catalog))
	for _, a := range Catalog {
		m[a.Code] = a
	}
	return m
}()

// IsValidAction informa se o código pertence ao catálogo.
func IsValidAction(code string) bool {
	_, ok := actionSet[code]
	return ok
}

// LookupAction devolve a definição da ação (e se existe).
func LookupAction(code string) (ActionDef, bool) {
	a, ok := actionSet[code]
	return a, ok
}

// ActionCodes devolve os códigos de todas as ações, na ordem do catálogo.
func ActionCodes() []string {
	out := make([]string, len(Catalog))
	for i, a := range Catalog {
		out[i] = a.Code
	}
	return out
}
