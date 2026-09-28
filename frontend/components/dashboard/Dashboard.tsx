"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowDown, ArrowRight, ArrowUp, RefreshCw } from "lucide-react";
import { getDashboard, type Dashboard as DashboardData } from "@/lib/dashboard-api";
import { INCIDENT_MEANS_LABEL, type IncidentMeans } from "@/lib/incidents-api";
import { RANGE_IDS, RANGE_LABEL, resolveRange, type RangeId } from "@/lib/date-ranges";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import Select from "../shared/Select";
import DateInput from "../shared/DateInput";
import FacetBars, { type FacetBar } from "./FacetBars";
import IncidentSeries from "./IncidentSeries";

// Recorte territorial exibido no ranking. Municípios e bairros dividem o mesmo
// painel: lado a lado, cada um ficava com metade da largura e o nome do
// bairro não cabia.
type Territory = "cities" | "neighborhoods";

// KPI do painel. `lowerIsBetter` separa o que se quer ver caindo (crime) do
// que se quer ver subindo (produção da agência): a seta mostra para onde o
// número foi, a cor diz se isso é bom — sem isso, um mês com mais homicídios
// apareceria em verde só por ter crescido.
type Kpi = {
  key: string;
  label: string;
  value: number;
  previous: number | null;
  lowerIsBetter: boolean;
  hint: string;
};

export default function Dashboard() {
  const [range, setRange] = useState<RangeId>("mes_atual");
  const [customFrom, setCustomFrom] = useState("");
  const [customTo, setCustomTo] = useState("");
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [territory, setTerritory] = useState<Territory>("cities");

  const query = useMemo(() => {
    if (range === "tudo") return { all: true };
    if (range === "custom") {
      return { date_from: customFrom || undefined, date_to: customTo || undefined };
    }
    const r = resolveRange(range);
    return { date_from: r.from, date_to: r.to };
  }, [range, customFrom, customTo]);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await getDashboard(query));
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar o painel");
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    reload();
  }, [reload]);

  const hasBaseline = Boolean(data?.previous);

  const kpis: Kpi[] = useMemo(() => {
    if (!data) return [];
    const out: Kpi[] = [];
    const inc = data.incidents;
    if (inc) {
      out.push(
        {
          key: "homicidio",
          label: "HOMICÍDIOS · CVLI",
          value: inc.by_type.homicidio,
          previous: hasBaseline ? inc.prev_by_type.homicidio : null,
          lowerIsBetter: true,
          hint: mainMeansHint(inc.means, inc.by_type.homicidio),
        },
        {
          key: "apreensao",
          label: "APREENSÕES",
          value: inc.by_type.apreensao,
          previous: hasBaseline ? inc.prev_by_type.apreensao : null,
          lowerIsBetter: false,
          hint: "",
        },
        {
          key: "prisao",
          label: "PRISÕES",
          value: inc.by_type.prisao,
          previous: hasBaseline ? inc.prev_by_type.prisao : null,
          lowerIsBetter: false,
          hint: "",
        },
      );
    }
    if (data.reports) {
      out.push({
        key: "difundidos",
        label: "RIs DIFUNDIDOS",
        value: data.reports.diffused,
        previous: hasBaseline ? data.reports.prev_diffused : null,
        lowerIsBetter: false,
        hint: "",
      });
    }
    return out;
  }, [data, hasBaseline]);

  const cityBars: FacetBar[] = useMemo(
    () => (data?.incidents?.cities ?? []).map((f) => ({ key: f.name, label: f.name, count: f.count })),
    [data],
  );

  const neighborhoodBars: FacetBar[] = useMemo(
    () =>
      (data?.incidents?.neighborhoods ?? []).map((f) => ({
        key: `${f.city}/${f.name}`,
        label: f.name,
        sub: f.city,
        count: f.count,
      })),
    [data],
  );

  const nothingVisible =
    data != null && !data.incidents && !data.reports && !data.informes && !data.entities;

  const territoryBars = territory === "cities" ? cityBars : neighborhoodBars;

  // Painel enxuto para caber inteiro em 1920×1080 sem rolagem: indicadores,
  // tendência e território. Produção da agência e meio utilizado saíram —
  // o primeiro repetia o KPI de RIs difundidos, o segundo virou a linha de
  // apoio do KPI de homicídios.
  return (
    <div className="screen-fill">
      <div className="toolbar">
        <Select
          value={range}
          onChange={(v) => setRange(v as RangeId)}
          options={RANGE_IDS.map((id) => ({ value: id, label: RANGE_LABEL[id] }))}
          className="dash-range"
        />
        {range === "custom" && (
          <>
            <DateInput value={customFrom} onChange={setCustomFrom} />
            <span className="muted">→</span>
            <DateInput value={customTo} onChange={setCustomTo} />
          </>
        )}
        {data?.period.from && (
          <span className="muted filter-summary">
            {formatBRDate(data.period.from)} → {formatBRDate(data.period.to)}
            {data.previous && (
              <>
                {" "}
                · comparado a {formatBRDate(data.previous.from)} → {formatBRDate(data.previous.to)}
              </>
            )}
          </span>
        )}
        <div style={{ marginLeft: "auto" }} />
        <button type="button" className="btn btn-ghost" onClick={reload} disabled={loading}>
          <RefreshCw size={14} strokeWidth={1.8} /> {loading ? "CARREGANDO…" : "ATUALIZAR"}
        </button>
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      {/* Cabeçalho fixo; o corpo se ajusta à altura e só rola se a escala da
          interface passar do que cabe. O .content do shell é overflow:hidden —
          no Tevunah cada tela gerencia o próprio scroll. */}
      <div className="dash">
        {!data && loading && <div className="muted dash-loading">// LEVANTANDO NÚMEROS…</div>}

        {nothingVisible && (
          <div className="placeholder" style={{ minHeight: 240 }}>
            <div className="ph-tag">// MOD-01 / DASHBOARD</div>
            <div className="ph-ttl">SEM MÓDULOS LIBERADOS</div>
            <div className="ph-sub">
              Seu perfil não tem leitura de ocorrências, relatórios, informes ou entidades — não há
              números a exibir. Contate o administrador.
            </div>
          </div>
        )}

        {data && !nothingVisible && (
          <>
            {kpis.length > 0 && (
              <div className="grid-kpi">
                {kpis.map((k) => (
                  <KpiCard key={k.key} kpi={k} />
                ))}
              </div>
            )}

            {data.incidents && (
              <div className="dash-main">
                <div className="panel">
                  <div className="panel-hd">
                    <span className="ttl">OCORRÊNCIAS · 12 MESES</span>
                    <span className="meta">
                      {data.series_period.from.slice(0, 7).replace("-", "/")} —{" "}
                      {data.series_period.to.slice(0, 7).replace("-", "/")}
                    </span>
                  </div>
                  <div className="panel-bd dash-series">
                    <IncidentSeries series={data.incidents.series} />
                  </div>
                </div>

                <div className="panel">
                  <div className="panel-hd">
                    <div className="panel-tabs" role="tablist" aria-label="Recorte territorial">
                      <button
                        type="button"
                        role="tab"
                        aria-selected={territory === "cities"}
                        className={"panel-tab" + (territory === "cities" ? " on" : "")}
                        onClick={() => setTerritory("cities")}
                      >
                        MUNICÍPIOS
                      </button>
                      <button
                        type="button"
                        role="tab"
                        aria-selected={territory === "neighborhoods"}
                        className={"panel-tab" + (territory === "neighborhoods" ? " on" : "")}
                        onClick={() => setTerritory("neighborhoods")}
                      >
                        BAIRROS
                      </button>
                    </div>
                    <span className="meta">TOP 8 · PERÍODO</span>
                  </div>
                  <div className="panel-bd">
                    <FacetBars
                      items={territoryBars}
                      empty={
                        territory === "cities"
                          ? "SEM MUNICÍPIO INFORMADO NO PERÍODO"
                          : "SEM BAIRRO INFORMADO NO PERÍODO"
                      }
                    />
                  </div>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

// Linha de apoio do KPI de homicídios: o meio mais frequente e sua fatia.
// "Não informado" não entra na disputa — dizer que o principal meio é
// desconhecido não orienta ninguém.
function mainMeansHint(means: { name: string; count: number }[], homicides: number): string {
  if (homicides === 0) return "";
  const top = means
    .filter((m) => m.name !== "" && m.count > 0)
    .reduce<{ name: string; count: number } | null>((a, m) => (a && a.count >= m.count ? a : m), null);
  if (!top) return "";
  const label = INCIDENT_MEANS_LABEL[top.name as IncidentMeans] ?? top.name;
  return `${label} em ${Math.round((top.count / homicides) * 100)}% dos casos`;
}

function KpiCard({ kpi }: { kpi: Kpi }) {
  const { value, previous, lowerIsBetter } = kpi;
  const diff = previous == null ? null : value - previous;
  // Sem base anterior (recorte aberto) ou base zerada, o percentual seria
  // ficção: mostra-se o absoluto.
  const pct = previous != null && previous > 0 ? Math.round((diff! / previous) * 100) : null;
  const favorable = diff == null || diff === 0 ? null : lowerIsBetter ? diff < 0 : diff > 0;

  return (
    <div className="panel kpi">
      <div className="kpi-lbl">{kpi.label}</div>
      <div className="kpi-val">{value}</div>
      <div className="kpi-trend">
        {diff == null ? (
          <span className="muted">sem base de comparação</span>
        ) : diff === 0 ? (
          <>
            <ArrowRight size={13} strokeWidth={2} />
            <span>estável · {previous} antes</span>
          </>
        ) : (
          <>
            <span className={favorable ? "up" : "dn"}>
              {diff > 0 ? <ArrowUp size={13} strokeWidth={2} /> : <ArrowDown size={13} strokeWidth={2} />}
            </span>
            <span className={favorable ? "up" : "dn"}>
              {pct != null ? `${Math.abs(pct)}%` : `${diff > 0 ? "+" : "−"}${Math.abs(diff)}`}
            </span>
            <span className="muted">vs. {previous}</span>
          </>
        )}
      </div>
      {kpi.hint && <div className="kpi-hint muted">{kpi.hint}</div>}
    </div>
  );
}
