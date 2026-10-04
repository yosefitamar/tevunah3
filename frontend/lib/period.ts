// Período de consulta — padrão de filtro temporal do sistema inteiro.
//
// Um atalho relativo (recalculado a cada carga), um mês fechado ou um
// intervalo livre. O atalho guarda o id, não as datas, para que "ANO ATUAL"
// continue correto se a tela ficar aberta na virada do ano. Datas em
// YYYY-MM-DD no fuso do navegador, como em date-ranges.ts.

import { RANGE_LABEL, resolveRange, type RangeId } from "./date-ranges";
import { formatBRDate } from "./format";

export type PresetId = Exclude<RangeId, "custom">;
export type PeriodSelection =
  | { kind: "preset"; id: PresetId }
  | { kind: "month"; year: number; month: number } // month 0–11
  | { kind: "range"; from: string; to: string };

export const DEFAULT_PERIOD: PeriodSelection = { kind: "preset", id: "ano_atual" };

/** Atalhos oferecidos por padrão, na ordem do modal. */
export const PERIOD_PRESETS: PresetId[] = [
  "ano_atual",
  "mes_atual",
  "mes_passado",
  "ultimos_3m",
  "ultimos_6m",
  "ultimos_12m",
  "ano_passado",
  "tudo",
];

export const MONTH_NAMES = [
  "JANEIRO", "FEVEREIRO", "MARÇO", "ABRIL", "MAIO", "JUNHO",
  "JULHO", "AGOSTO", "SETEMBRO", "OUTUBRO", "NOVEMBRO", "DEZEMBRO",
];
export const WEEKDAYS = ["D", "S", "T", "Q", "Q", "S", "S"];

export function pad(n: number): string {
  return String(n).padStart(2, "0");
}

export function toIso(y: number, m: number, d: number): string {
  return `${y}-${pad(m + 1)}-${pad(d)}`;
}

export function todayIso(): string {
  const t = new Date();
  return toIso(t.getFullYear(), t.getMonth(), t.getDate());
}

function monthBounds(year: number, month: number): { from: string; to: string } {
  return { from: toIso(year, month, 1), to: toIso(year, month, new Date(year, month + 1, 0).getDate()) };
}

export function daysBetween(from: string, to: string): number {
  const a = Date.parse(from + "T00:00:00Z");
  const b = Date.parse(to + "T00:00:00Z");
  return Math.round((b - a) / 86_400_000) + 1;
}

/** Datas efetivas do recorte. "tudo" devolve strings vazias (sem filtro). */
export function periodBounds(p: PeriodSelection): { from: string; to: string } {
  switch (p.kind) {
    case "preset":
      return resolveRange(p.id);
    case "month":
      return monthBounds(p.year, p.month);
    case "range":
      return { from: p.from, to: p.to };
  }
}

/** Parâmetros date_from/date_to das APIs (ou all, sem recorte). */
export function periodQuery(p: PeriodSelection): { all?: boolean; date_from?: string; date_to?: string } {
  if (p.kind === "preset" && p.id === "tudo") return { all: true };
  const b = periodBounds(p);
  return { date_from: b.from, date_to: b.to };
}

/** Rótulo do botão que abre o modal. */
export function periodLabel(p: PeriodSelection): string {
  switch (p.kind) {
    case "preset":
      return RANGE_LABEL[p.id];
    case "month":
      return `${MONTH_NAMES[p.month]}/${p.year}`;
    case "range":
      return `${formatBRDate(p.from)} → ${formatBRDate(p.to)}`;
  }
}

export function isPeriodComplete(p: PeriodSelection): boolean {
  return p.kind !== "range" || (p.from !== "" && p.to !== "" && p.from <= p.to);
}


/** Recorte sem data ("TODO O PERÍODO"). */
export const ALL_PERIOD: PeriodSelection = { kind: "preset", id: "tudo" };

export function isAllPeriod(p: PeriodSelection): boolean {
  return p.kind === "preset" && p.id === "tudo";
}
