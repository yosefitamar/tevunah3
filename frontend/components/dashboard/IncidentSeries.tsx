"use client";

import { useMemo } from "react";
import { INCIDENT_TYPE_LABEL, type IncidentType } from "@/lib/incidents-api";
import type { DashMonth } from "@/lib/dashboard-api";

// Cor por série. Homicídio herda o vermelho crítico que a ocorrência já usa
// na listagem e no mapa; os demais se separam por tokens da paleta ativa,
// para o gráfico acompanhar a troca de paleta sem tabela paralela.
type SeriesKey = IncidentType | "operacional";

const SERIES_COLOR: Record<SeriesKey, string> = {
  homicidio: "var(--crit)",
  apreensao: "var(--info)",
  prisao: "var(--accent)",
  operacional: "var(--warn)",
};

const SERIES_LABEL: Record<SeriesKey, string> = {
  ...INCIDENT_TYPE_LABEL,
  operacional: "REL. OPERACIONAL",
};

const MONTH_ABBR = ["JAN", "FEV", "MAR", "ABR", "MAI", "JUN", "JUL", "AGO", "SET", "OUT", "NOV", "DEZ"];

const INCIDENT_KEYS: IncidentType[] = ["homicidio", "apreensao", "prisao"];

function monthLabel(ym: string): string {
  const m = Number(ym.slice(5, 7));
  return MONTH_ABBR[m - 1] ?? ym;
}

type Props = {
  /** Cadastro manual (ausente sem incident.read). */
  series?: DashMonth[];
  /** Relatório operacional por mês (ausente sem opsreport.read). */
  operational?: { month: string; count: number }[];
};

type Col = { month: string } & Record<SeriesKey, number>;

/**
 * Ocorrências dos últimos 12 meses, empilhadas por fonte/tipo.
 *
 * Colunas empilhadas (e não séries lado a lado) porque a primeira pergunta do
 * painel é o volume total do mês; a composição vem em seguida, na mesma
 * coluna. A escala é o maior mês da janela — a série existe para dar régua
 * ao número do período corrente, então o pico precisa tocar o topo.
 *
 * O relatório operacional entra como série própria: suas naturezas (tráfico,
 * porte, receptação…) não cabem nos três tipos do cadastro manual. A
 * ocorrência que existe nos dois lados já vem descontada pelo servidor.
 */
export default function IncidentSeries({ series, operational }: Props) {
  const cols: Col[] = useMemo(() => {
    const byMonth = new Map<string, Col>();
    const get = (month: string) => {
      let c = byMonth.get(month);
      if (!c) {
        c = { month, homicidio: 0, apreensao: 0, prisao: 0, operacional: 0 };
        byMonth.set(month, c);
      }
      return c;
    };
    for (const m of series ?? []) {
      const c = get(m.month);
      c.homicidio = m.homicidio;
      c.apreensao = m.apreensao;
      c.prisao = m.prisao;
    }
    for (const m of operational ?? []) get(m.month).operacional = m.count;
    return [...byMonth.values()].sort((a, b) => a.month.localeCompare(b.month));
  }, [series, operational]);

  // Legenda e pilha só com o que a janela tem (homicídio sempre, quando o
  // cadastro manual é visível): apreensão e prisão manuais estão em desuso
  // desde que o relatório operacional passou a alimentar o painel.
  const keys: SeriesKey[] = useMemo(() => {
    const out: SeriesKey[] = [];
    if (series) {
      for (const k of INCIDENT_KEYS) {
        if (k === "homicidio" || cols.some((c) => c[k] > 0)) out.push(k);
      }
    }
    if (operational) out.push("operacional");
    return out;
  }, [cols, series, operational]);

  const total = (c: Col) => keys.reduce((n, k) => n + c[k], 0);
  const max = Math.max(...cols.map(total), 1);
  const empty = cols.every((c) => total(c) === 0);

  return (
    <div className="mchart">
      <div className="mchart-body">
        <div className="mchart-scale">
          <span>{max}</span>
          <span>0</span>
        </div>
        <div className="mchart-plot">
          {empty && <div className="mchart-empty muted">// SEM OCORRÊNCIAS NA JANELA</div>}
          {cols.map((c) => {
            const t = total(c);
            const year = c.month.slice(2, 4);
            return (
              <div
                key={c.month}
                className="mchart-col"
                title={`${monthLabel(c.month)}/${year} · ${t} ocorrência(s)${keys
                  .map((k) => (c[k] > 0 ? `\n${SERIES_LABEL[k]}: ${c[k]}` : ""))
                  .join("")}`}
              >
                <div className="mchart-stack">
                  {keys.map((k) =>
                    c[k] > 0 ? (
                      <span
                        key={k}
                        className="mchart-seg"
                        style={{ height: `${(c[k] / max) * 100}%`, background: SERIES_COLOR[k] }}
                      />
                    ) : null,
                  )}
                </div>
                <div className="mchart-total">{t > 0 ? t : ""}</div>
                {/* Só o mês: a janela inteira já está datada no cabeçalho do
                    painel, e o ano na coluna não cabe em doze divisões. */}
                <div className="mchart-lbl">{monthLabel(c.month)}</div>
              </div>
            );
          })}
        </div>
      </div>
      <div className="mchart-legend">
        {keys.map((k) => (
          <span key={k} className="mchart-legend-item">
            <span className="dot" style={{ background: SERIES_COLOR[k] }} aria-hidden />
            {SERIES_LABEL[k]}
          </span>
        ))}
      </div>
    </div>
  );
}
