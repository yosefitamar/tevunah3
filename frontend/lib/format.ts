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

// formatBRDate converte uma data simples "YYYY-MM-DD" (sem hora) -> "DD/MM/AAAA".
// Não usa Date(), que interpretaria a string como UTC e poderia recuar um dia
// ao converter pro fuso de Fortaleza (BRT). Apenas reordena os componentes.
export function formatBRDate(date: string | null | undefined): string {
  if (!date) return "—";
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(date);
  if (!m) return date;
  return `${m[3]}/${m[2]}/${m[1]}`;
}

// formatPhoneBR formata telefone só com dígitos: "85989577505" → "(85) 98957-7505",
// "8531012330" → "(85) 3101-2330". Outros tamanhos saem como vieram.
export function formatPhoneBR(digits: string): string {
  const d = digits.replace(/\D/g, "");
  if (d.length === 11) return `(${d.slice(0, 2)}) ${d.slice(2, 7)}-${d.slice(7)}`;
  if (d.length === 10) return `(${d.slice(0, 2)}) ${d.slice(2, 6)}-${d.slice(6)}`;
  return digits;
}

// formatRG junta número, órgão e UF: "20079756519 SSP/CE".
export function formatRG(number?: string, issuer?: string, state?: string): string {
  if (!number) return "—";
  const org = [issuer, state].filter(Boolean).join("/");
  return org ? `${number} ${org}` : number;
}
