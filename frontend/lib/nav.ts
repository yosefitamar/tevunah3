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
  | "ocorrencias"
  | "relatorios"
  | "informes"
  | "agentes"
  | "aprovacoes"
  | "auditoria"
  | "admin"
  | "sandbox";

// Ambiente de desenvolvimento (next dev). No build de produção (next build)
// NODE_ENV é "production" — usado pra esconder telas só-de-dev como o Sandbox.
export const IS_DEV = process.env.NODE_ENV === "development";

export const NAV: NavEntry[] = [
  { group: "OPERACIONAL" },
  { id: "dashboard", key: "01", lbl: "DASHBOARD", glyph: "▦" },
  { id: "entidades", key: "02", lbl: "ENTIDADES", glyph: "◇" },
  { id: "relatorios", key: "03", lbl: "RELATÓRIOS", glyph: "▥" },
  { id: "informes", key: "04", lbl: "INFORMES", glyph: "✎" },
  { id: "ocorrencias", key: "05", lbl: "OCORRÊNCIAS", glyph: "◎" },
  { id: "agentes", key: "06", lbl: "AGENTES", glyph: "◬" },

  { group: "SUPERVISÃO" },
  { id: "aprovacoes", key: "07", lbl: "APROVAÇÕES", glyph: "✓" },
  { id: "auditoria", key: "08", lbl: "AUDITORIA", glyph: "▤" },

  { group: "SISTEMA" },
  { id: "admin", key: "99", lbl: "ADMIN", glyph: "⚙" },
  // Sandbox de modais — ferramenta de dev; só aparece em desenvolvimento.
  ...(IS_DEV
    ? ([{ id: "sandbox", key: "98", lbl: "SANDBOX", glyph: "⌬", badge: "DEV", badgeKind: "warn" }] as NavEntry[])
    : []),
];

export const MODULE_TITLES: Record<ModuleId, string> = {
  dashboard: "DASHBOARD",
  entidades: "ENTIDADES",
  relatorios: "RELATÓRIOS",
  informes: "INFORMES",
  ocorrencias: "OCORRÊNCIAS",
  agentes: "AGENTES",
  aprovacoes: "APROVAÇÕES",
  auditoria: "AUDITORIA",
  admin: "ADMINISTRAÇÃO",
  sandbox: "SANDBOX · MODAIS",
};
