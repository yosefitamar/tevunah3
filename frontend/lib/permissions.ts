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
