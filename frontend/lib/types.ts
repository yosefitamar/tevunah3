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

// ─────────────────────────── Permissions (matriz RBAC) ────────────────

export type Permission = {
  role_code: RoleCode;
  action: string;
  allowed: boolean;
  requires_dual_approval: boolean;
  approver_role?: RoleCode | null;
  updated_at: string;
  updated_by?: string | null;
};

// Grupo legível a partir do prefixo da ação (mesmo helper do audit).
// Reutiliza actionGroup, definida abaixo.

// ─────────────────────────── Audit ────────────────────────────

export type AuditEntry = {
  id: number;
  ts: string;
  actor_user_id?: string | null;
  actor_user_code?: string | null;
  actor_display_name?: string | null;
  actor_session_id?: string | null;
  actor_ip?: string | null;
  actor_terminal?: string | null;
  actor_user_agent?: string | null;
  action: string;
  resource_type?: string | null;
  resource_id?: string | null;
  resource_classification?: number | null;
  before?: unknown;
  after?: unknown;
  reason?: string | null;
  prev_hash: string;
  hash: string;
};

// Extrai um resumo legível do User-Agent (sem libs externas).
// Heurística simples; cobre o suficiente para audit/forense em ambiente office.
export function summarizeUA(ua: string | null | undefined): { os: string; browser: string } {
  if (!ua) return { os: "—", browser: "—" };
  const s = ua;

  let os = "—";
  if (/iPhone|iPad|iPod/.test(s))          os = "iOS";
  else if (/Android/.test(s))              os = "Android";
  else if (/Mac OS X|Macintosh/.test(s))   os = "macOS";
  else if (/Windows NT/.test(s))           os = "Windows";
  else if (/Linux/.test(s))                os = "Linux";

  let browser = "—";
  const ver = (re: RegExp) => {
    const m = s.match(re);
    return m ? m[1].split(".")[0] : "";
  };
  if (/Edg\//.test(s))                            browser = `Edge ${ver(/Edg\/([\d.]+)/)}`.trim();
  else if (/OPR\//.test(s))                       browser = `Opera ${ver(/OPR\/([\d.]+)/)}`.trim();
  else if (/Firefox\//.test(s))                   browser = `Firefox ${ver(/Firefox\/([\d.]+)/)}`.trim();
  else if (/Chrome\//.test(s))                    browser = `Chrome ${ver(/Chrome\/([\d.]+)/)}`.trim();
  else if (/Safari\//.test(s) && !/Chrome/.test(s)) browser = `Safari ${ver(/Version\/([\d.]+)/)}`.trim();

  return { os, browser };
}

// Grupo legível a partir do prefixo da action (ex.: "auth.login" -> "AUTENTICAÇÃO").
export function actionGroup(action: string): string {
  const head = action.split(".")[0] ?? "";
  switch (head) {
    case "auth": return "AUTENTICAÇÃO";
    case "user": return "AGENTES";
    case "approval":
    case "approvals": return "APROVAÇÕES";
    case "audit": return "AUDITORIA";
    case "admin": return "ADMIN";
    default: return head.toUpperCase() || "—";
  }
}

export function primaryRole(u: User): string {
  if (u.roles.length === 0) return "—";
  // Ordem de "patente" — gestor > administrador > analista > agente
  const order: RoleCode[] = ["gestor", "administrador", "analista", "agente"];
  for (const r of order) if (u.roles.includes(r)) return ROLE_LABEL[r];
  return ROLE_LABEL[u.roles[0]];
}
