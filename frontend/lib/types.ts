export type RoleCode = "agente" | "analista" | "gestor" | "administrador";

export const ROLE_LABEL: Record<RoleCode, string> = {
  agente: "AGENTE",
  analista: "ANALISTA",
  gestor: "GESTOR",
  administrador: "ADMINISTRADOR",
};

export const ROLES_LIST: RoleCode[] = ["agente", "analista", "gestor", "administrador"];

export type UserStatus = "active" | "suspended" | "deactivated";

export type User = {
  id: string;
  code: string;
  email: string;
  display_name: string;
  clearance_level: number;
  status: UserStatus;
  roles: RoleCode[];
  last_login_at?: string;
};

export const STATUS_LABEL: Record<UserStatus, string> = {
  active: "ATIVO",
  suspended: "SUSPENSO",
  deactivated: "DESATIVADO",
};

// Mapeia status -> classe da .pill (verde/amarelo/cinza)
export const STATUS_PILL: Record<UserStatus, string> = {
  active: "active",
  suspended: "hold",
  deactivated: "cold",
};

export function clearanceLabel(level: number): string {
  return `CL-${String(level).padStart(2, "0")}`;
}

export function primaryRole(u: User): string {
  if (u.roles.length === 0) return "—";
  // Ordem de "patente" — gestor > administrador > analista > agente
  const order: RoleCode[] = ["gestor", "administrador", "analista", "agente"];
  for (const r of order) if (u.roles.includes(r)) return ROLE_LABEL[r];
  return ROLE_LABEL[u.roles[0]];
}
