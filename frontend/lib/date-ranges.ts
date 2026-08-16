// Recortes temporais pré-definidos (usados pelo mapa do crime).
//
// Todas as datas são calculadas no fuso local do navegador e serializadas
// como YYYY-MM-DD — o backend compara contra occurred_on, que é `date`,
// então não há conversão de timezone envolvida.

export type RangeId =
  | "mes_atual"
  | "mes_passado"
  | "ultimos_3m"
  | "ultimos_6m"
  | "ano_atual"
  | "ano_passado"
  | "ultimos_12m"
  | "tudo"
  | "custom";

export type DateRange = { from: string; to: string };

export const RANGE_LABEL: Record<RangeId, string> = {
  mes_atual: "MÊS ATUAL",
  mes_passado: "MÊS PASSADO",
  ultimos_3m: "ÚLTIMOS 3 MESES",
  ultimos_6m: "ÚLTIMOS 6 MESES",
  ano_atual: "ANO ATUAL",
  ano_passado: "ANO PASSADO",
  ultimos_12m: "ÚLTIMOS 12 MESES",
  tudo: "TODO O PERÍODO",
  custom: "PERSONALIZADO",
};

export const RANGE_IDS: RangeId[] = [
  "mes_atual",
  "mes_passado",
  "ultimos_3m",
  "ultimos_6m",
  "ano_atual",
  "ano_passado",
  "ultimos_12m",
  "tudo",
  "custom",
];

function iso(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// resolveRange devolve o intervalo do preset. "tudo" e "custom" devolvem
// strings vazias — o chamador ignora o filtro ou usa as datas manuais.
export function resolveRange(id: RangeId, now = new Date()): DateRange {
  const y = now.getFullYear();
  const m = now.getMonth();
  switch (id) {
    case "mes_atual":
      return { from: iso(new Date(y, m, 1)), to: iso(new Date(y, m + 1, 0)) };
    case "mes_passado":
      return { from: iso(new Date(y, m - 1, 1)), to: iso(new Date(y, m, 0)) };
    case "ultimos_3m":
      return { from: iso(new Date(y, m - 2, 1)), to: iso(new Date(y, m + 1, 0)) };
    case "ultimos_6m":
      return { from: iso(new Date(y, m - 5, 1)), to: iso(new Date(y, m + 1, 0)) };
    case "ano_atual":
      return { from: iso(new Date(y, 0, 1)), to: iso(new Date(y, 11, 31)) };
    case "ano_passado":
      return { from: iso(new Date(y - 1, 0, 1)), to: iso(new Date(y - 1, 11, 31)) };
    case "ultimos_12m":
      return { from: iso(new Date(y - 1, m + 1, 1)), to: iso(new Date(y, m + 1, 0)) };
    default:
      return { from: "", to: "" };
  }
}
