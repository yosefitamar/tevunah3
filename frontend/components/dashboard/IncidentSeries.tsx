"use client";

import { useMemo } from "react";
import { INCIDENT_TYPE_LABEL, type IncidentType } from "@/lib/incidents-api";
import type { DashMonth } from "@/lib/dashboard-api";

// Cor por tipo na série. Homicídio herda o vermelho crítico que a ocorrência
// já usa na listagem e no mapa; os demais se separam por tokens da paleta
// ativa, para o gráfico acompanhar a troca de paleta sem tabela paralela.
const TYPE_COLOR: Record<IncidentType, string> = {
  homicidio: "var(--crit)",
  apreensao: "var(--info)",
  prisao: "var(--accent)",
};

const MONTH_ABBR = ["JAN", "FEV", "MAR", "ABR", "MAI", "JUN", "JUL", "AGO", "SET", "OUT", "NOV", "DEZ"];

const TYPES: IncidentType[] = ["homicidio", "apreensao", "prisao"];

function monthLabel(ym: string): string {
  const m = Number(ym.slice(5, 7));
  return MONTH_ABBR[m - 1] ?? ym;
}

/**
 * Ocorrências dos últimos 12 meses, empilhadas por tipo.
 *
 * Colunas empilhadas (e não três séries lado a lado) porque a primeira
 * pergunta do painel é o volume total do mês; a composição vem em seguida, na
 * mesma coluna. A escala é o maior mês da janela — a série existe justamente
 * para dar régua ao número do período corrente, então o pico precisa tocar o
 * topo em vez de se diluir num teto arbitrário.
 */
export default function IncidentSeries({ series }: { series: DashMonth[] }) {
  const max = useMemo(
    () => Math.max(...series.map((m) => m.homicidio + m.apreensao + m.prisao), 1),
    [series],
  );

  const empty = series.every((m) => m.homicidio + m.apreensao + m.prisao === 0);

  return (
    <div className="mchart">
      <div className="mchart-body">
        <div className="mchart-scale">
          <span>{max}</span>
          <span>0</span>
        </div>
        <div className="mchart-plot">
          {empty && <div className="mchart-empty muted">// SEM OCORRÊNCIAS NA JANELA</div>}
          {series.map((m) => {
            const total = m.homicidio + m.apreensao + m.prisao;
            const year = m.month.slice(2, 4);
            return (
              <div
                key={m.month}
                className="mchart-col"
                title={`${monthLabel(m.month)}/${year} · ${total} ocorrência(s)${TYPES.map(
                  (t) => (m[t] > 0 ? `\n${INCIDENT_TYPE_LABEL[t]}: ${m[t]}` : ""),
                ).join("")}`}
              >
                <div className="mchart-stack">
                  {TYPES.map((t) =>
                    m[t] > 0 ? (
                      <span
                        key={t}
                        className="mchart-seg"
                        style={{ height: `${(m[t] / max) * 100}%`, background: TYPE_COLOR[t] }}
                      />
                    ) : null,
                  )}
                </div>
                <div className="mchart-total">{total > 0 ? total : ""}</div>
                {/* Só o mês: a janela inteira já está datada no cabeçalho do
                    painel, e o ano na coluna não cabe em doze divisões — encostava
                    no rótulo vizinho. */}
                <div className="mchart-lbl">{monthLabel(m.month)}</div>
              </div>
            );
          })}
        </div>
      </div>
      <div className="mchart-legend">
        {TYPES.map((t) => (
          <span key={t} className="mchart-legend-item">
            <span className="dot" style={{ background: TYPE_COLOR[t] }} aria-hidden />
            {INCIDENT_TYPE_LABEL[t]}
          </span>
        ))}
      </div>
    </div>
  );
}
