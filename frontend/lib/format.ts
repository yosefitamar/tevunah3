// Formatadores de data padronizados em horário de Fortaleza (BRT).

const FORTALEZA_DT = new Intl.DateTimeFormat("pt-BR", {
  timeZone: "America/Fortaleza",
  day: "2-digit",
  month: "2-digit",
  year: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

// formatBR converte ISO 8601 -> "DD/MM/AA HH:MM" em BRT.
export function formatBR(iso: string | null | undefined): string {
  if (!iso) return "—";
  return FORTALEZA_DT.format(new Date(iso));
}
