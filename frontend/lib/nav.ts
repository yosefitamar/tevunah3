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

export type ModuleId = "dashboard" | "agentes" | "auditoria" | "admin";

export const NAV: NavEntry[] = [
  { group: "OPERACIONAL" },
  { id: "dashboard", key: "01", lbl: "DASHBOARD", glyph: "▦" },
  { id: "agentes", key: "02", lbl: "AGENTES", glyph: "◬" },

  { group: "SUPERVISÃO" },
  { id: "auditoria", key: "03", lbl: "AUDITORIA", glyph: "▤" },

  { group: "SISTEMA" },
  { id: "admin", key: "99", lbl: "ADMIN", glyph: "⚙" },
];

export const MODULE_TITLES: Record<ModuleId, string> = {
  dashboard: "DASHBOARD",
  agentes: "AGENTES",
  auditoria: "AUDITORIA",
  admin: "ADMINISTRAÇÃO",
};
