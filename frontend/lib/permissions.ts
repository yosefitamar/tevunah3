// Checagens leves no cliente. Servidor é a fonte de verdade.
// Aqui só evita renderizar botões/views que vão sempre dar 403.

import type { RoleCode, User } from "./types";

export function hasRole(user: User | null, ...roles: RoleCode[]): boolean {
  if (!user) return false;
  return user.roles.some((r) => roles.includes(r));
}

export const canListUsers = (u: User | null) => hasRole(u, "gestor", "administrador");
export const canCreateUsers = (u: User | null) => hasRole(u, "administrador");
export const canReadAudit = (u: User | null) => hasRole(u, "gestor", "administrador");

// Admin: gerenciar matriz RBAC e demais parâmetros do sistema.
export const canAccessAdmin = (u: User | null) => hasRole(u, "administrador");
export const canManagePermissions = (u: User | null) => hasRole(u, "administrador");

// Solicitar mudança de papel/clearance: matriz hoje exige admin (4-eyes -> gestor decide).
// UI mostra os botões para admin; servidor é a fonte de verdade.
export const canRequestRoleChange = (u: User | null) => hasRole(u, "administrador");
export const canRequestClearanceChange = (u: User | null) => hasRole(u, "administrador");

// Entidades — espelha a matriz da migration 00010_entities_permissions.
// Agente só lê; analista/gestor/admin criam e editam; só gestor/admin excluem.
export const canListEntities = (u: User | null) =>
  hasRole(u, "agente", "analista", "gestor", "administrador");
export const canCreateEntities = (u: User | null) =>
  hasRole(u, "analista", "gestor", "administrador");
export const canEditEntities = canCreateEntities;
export const canDeleteEntities = (u: User | null) =>
  hasRole(u, "gestor", "administrador");
export const canRestoreEntities = canDeleteEntities;

// Relatórios — espelha a matriz da migration 00027_reports.
export const canReadReports = (u: User | null) =>
  hasRole(u, "agente", "analista", "gestor", "administrador");
export const canCreateReports = (u: User | null) =>
  hasRole(u, "analista", "gestor", "administrador");
export const canEditReports = canCreateReports;
export const canDiffuseReports = (u: User | null) =>
  hasRole(u, "gestor", "administrador");
export const canArchiveReports = canDiffuseReports;
// Reverter difusão devolve o RI pra edição — restrito a administrador.
// Espelha a permissão report.undiffuse (migration 00028).
export const canUndiffuseReports = (u: User | null) =>
  hasRole(u, "administrador");
export const canDownloadReports = (u: User | null) =>
  hasRole(u, "analista", "gestor", "administrador");
// Destruir (soft delete) rascunho — só faz sentido pra quem cria/edita.
// O backend ainda exige que o caller seja autor OU admin.
export const canDestroyReports = (u: User | null) =>
  hasRole(u, "analista", "gestor", "administrador");
