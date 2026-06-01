// Checagens leves no cliente. Servidor é a fonte de verdade.
// Aqui só evita renderizar botões/views que vão sempre dar 403.
//
// O gating é por PERMISSÃO EFETIVA (ações que o usuário pode executar, vindas
// do /me em user.permissions), não por nome de papel. Assim, conceder uma ação
// a um papel na matriz RBAC já reflete na UI — sem editar este arquivo — e
// papéis customizados funcionam automaticamente.

import type { RoleCode, User } from "./types";

// can verifica se o usuário tem a ação na sua lista de permissões efetivas.
export function can(user: User | null, action: string): boolean {
  if (!user) return false;
  return (user.permissions ?? []).includes(action);
}

// hasRole permanece para checagens legítimas por papel (ex.: visibilidade
// "vê todos os RIs"), que não são ações da matriz. Evite usar para gating de
// funcionalidade — prefira can().
export function hasRole(user: User | null, ...roles: RoleCode[]): boolean {
  if (!user) return false;
  return user.roles.some((r) => roles.includes(r));
}

export const canListUsers = (u: User | null) => can(u, "user.list");
export const canCreateUsers = (u: User | null) => can(u, "user.create");
export const canReadAudit = (u: User | null) => can(u, "audit.read");

// Admin: gerenciar matriz RBAC e demais parâmetros do sistema.
export const canManagePermissions = (u: User | null) => can(u, "admin.permissions.update");
// Acesso à área admin: quem pode ler a matriz ou editar configurações do sistema.
export const canAccessAdmin = (u: User | null) =>
  can(u, "admin.permissions.read") || can(u, "system.settings.update");

// Solicitar mudança de papel/clearance de outro agente.
export const canRequestRoleChange = (u: User | null) => can(u, "user.role.assign");
export const canRequestClearanceChange = (u: User | null) => can(u, "user.clearance.set");

// Entidades.
export const canListEntities = (u: User | null) => can(u, "entity.list");
export const canCreateEntities = (u: User | null) => can(u, "entity.create");
export const canEditEntities = (u: User | null) => can(u, "entity.update");
export const canDeleteEntities = (u: User | null) => can(u, "entity.delete");
export const canRestoreEntities = (u: User | null) => can(u, "entity.restore");

// Relatórios.
export const canReadReports = (u: User | null) => can(u, "report.read");
export const canCreateReports = (u: User | null) => can(u, "report.create");
export const canEditReports = (u: User | null) => can(u, "report.update");
export const canDiffuseReports = (u: User | null) => can(u, "report.diffuse");
export const canArchiveReports = (u: User | null) => can(u, "report.archive");
// Reverter difusão devolve o RI pra edição.
export const canUndiffuseReports = (u: User | null) => can(u, "report.undiffuse");
export const canDownloadReports = (u: User | null) => can(u, "report.download");
// Destruir (soft delete) rascunho. O backend ainda exige que o caller seja autor OU admin.
export const canDestroyReports = (u: User | null) => can(u, "report.destroy");
