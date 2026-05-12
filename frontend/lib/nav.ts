export type NavGroup = { group: string };
export type NavItem = {
  id: ModuleId;
  key: string;
  lbl: string;
  glyph: string;
  badge?: string;
  badgeKind?: "crit" | "warn";
};
export type NavEntry = NavGroup | NavItem;

export type ModuleId =
  | "dashboard"
  | "entidades"
  | "agentes"
  | "aprovacoes"
  | "auditoria"
  | "admin"
  | "sandbox";

export const NAV: NavEntry[] = [
  { group: "OPERACIONAL" },
  { id: "dashboard", key: "01", lbl: "DASHBOARD", glyph: "▦" },
  { id: "entidades", key: "02", lbl: "ENTIDADES", glyph: "◇" },
  { id: "agentes", key: "03", lbl: "AGENTES", glyph: "◬" },

  { group: "SUPERVISÃO" },
  { id: "aprovacoes", key: "04", lbl: "APROVAÇÕES", glyph: "✓" },
  { id: "auditoria", key: "05", lbl: "AUDITORIA", glyph: "▤" },

  { group: "SISTEMA" },
  { id: "admin", key: "99", lbl: "ADMIN", glyph: "⚙" },
  { id: "sandbox", key: "98", lbl: "SANDBOX", glyph: "⌬", badge: "DEV", badgeKind: "warn" },
];

export const MODULE_TITLES: Record<ModuleId, string> = {
  dashboard: "DASHBOARD",
  entidades: "ENTIDADES",
  agentes: "AGENTES",
  aprovacoes: "APROVAÇÕES",
  auditoria: "AUDITORIA",
  admin: "ADMINISTRAÇÃO",
  sandbox: "SANDBOX · MODAIS",
};
