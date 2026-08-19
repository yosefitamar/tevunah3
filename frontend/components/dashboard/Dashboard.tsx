"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowDown, ArrowRight, ArrowUp, RefreshCw } from "lucide-react";
import { getDashboard, type Dashboard as DashboardData } from "@/lib/dashboard-api";
import {
  INCIDENT_MEANS_COLOR,
  INCIDENT_MEANS_LABEL,
  INCIDENT_TYPE_LABEL,
  type IncidentMeans,
} from "@/lib/incidents-api";
import { RANGE_IDS, RANGE_LABEL, resolveRange, type RangeId } from "@/lib/date-ranges";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import Select from "../shared/Select";
import DateInput from "../shared/DateInput";
import FacetBars, { type FacetBar } from "./FacetBars";
import IncidentSeries from "./IncidentSeries";
import ProductionPanel from "./ProductionPanel";

const MONTHS_PT = ["JAN", "FEV", "MAR", "ABR", "MAI", "JUN", "JUL", "AGO", "SET", "OUT", "NOV", "DEZ"];

function intelDate(d: Date) {
  const dd = String(d.getDate()).padStart(2, "0");
  const mmm = MONTHS_PT[d.getMonth()];
  const yy = String(d.getFullYear()).slice(-2);
  return `${dd}${mmm}${yy}`;
}

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
          hint: `${inc.geocoded} de ${inc.total} ocorrência(s) do período com coordenadas`,
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

  const meansBars: FacetBar[] = useMemo(() => {
    if (!data?.incidents) return [];
    return data.incidents.means.map((f) => {
      const m = f.name as IncidentMeans;
      return {
        key: m || "ni",
        label: INCIDENT_MEANS_LABEL[m] ?? f.name,
        count: f.count,
        color: INCIDENT_MEANS_COLOR[m] ?? "var(--fg-3)",
      };
    });
  }, [data]);

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

  const periodLabel = useMemo(() => {
    if (range === "custom") {
      return `${customFrom ? formatBRDate(customFrom) : "…"} → ${customTo ? formatBRDate(customTo) : "…"}`;
    }
    return RANGE_LABEL[range];
  }, [range, customFrom, customTo]);

  const nothingVisible =
    data != null && !data.incidents && !data.reports && !data.informes && !data.entities;

  return (
    <div className="screen-fill">
      <div className="section-title">
        PAINEL OPERACIONAL · {intelDate(new Date())}
        <span style={{ color: "var(--fg-2)" }}>· {periodLabel}</span>
      </div>

      <div className="toolbar">
        <span className="muted" style={{ fontSize: 10, letterSpacing: "0.14em" }}>
          PERÍODO
        </span>
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
                · base {formatBRDate(data.previous.from)} → {formatBRDate(data.previous.to)}
              </>
            )}
          </span>
        )}
        <div style={{ marginLeft: "auto" }} />
        <button type="button" className="btn btn-ghost" onClick={reload} disabled={loading}>
          <RefreshCw size={13} strokeWidth={1.8} /> {loading ? "CARREGANDO…" : "ATUALIZAR"}
        </button>
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      {/* Cabeçalho e recorte ficam fixos; só os painéis rolam. O .content do
          shell é overflow:hidden — no Tevunah cada tela gerencia o próprio
          scroll, e sem esta área os cards de baixo ficavam fora de alcance. */}
      <div className="dash-scroll">

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
              <div className="grid-main">
                <div className="panel">
                  <div className="panel-hd">
                    <span className="ttl">OCORRÊNCIAS · 12 MESES</span>
                    <span className="meta">
                      {data.series_period.from.slice(0, 7).replace("-", "/")} —{" "}
                      {data.series_period.to.slice(0, 7).replace("-", "/")}
                    </span>
                  </div>
                  <div className="panel-bd">
                    <IncidentSeries series={data.incidents.series} />
                  </div>
                </div>

                <div className="panel">
                  <div className="panel-hd">
                    <span className="ttl">MEIO UTILIZADO</span>
                    <span className="meta">CVLI · PERÍODO</span>
                  </div>
                  <div className="panel-bd">
                    <FacetBars
                      items={meansBars}
                      empty="NENHUM HOMICÍDIO NO PERÍODO"
                      showPercent
                    />
                    {/* Sem CVLI no recorte a lista já diz isso; a nota viraria
                        "distribuição sobre os 0 homicídios". */}
                    {data.incidents.by_type.homicidio > 0 && (
                      <div className="dash-note muted">
                        Distribuição sobre os {data.incidents.by_type.homicidio}{" "}
                        {INCIDENT_TYPE_LABEL.homicidio.toLowerCase()}
                        {data.incidents.by_type.homicidio === 1 ? "" : "s"} do recorte.
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )}

            <div className="grid-bottom">
              {data.incidents && (
                <>
                  <div className="panel">
                    <div className="panel-hd">
                      <span className="ttl">MUNICÍPIOS</span>
                      <span className="meta">TOP 8</span>
                    </div>
                    <div className="panel-bd">
                      <FacetBars items={cityBars} empty="SEM MUNICÍPIO INFORMADO NO PERÍODO" />
                    </div>
                  </div>

                  <div className="panel">
                    <div className="panel-hd">
                      <span className="ttl">BAIRROS</span>
                      <span className="meta">TOP 8</span>
                    </div>
                    <div className="panel-bd">
                      <FacetBars items={neighborhoodBars} empty="SEM BAIRRO INFORMADO NO PERÍODO" />
                    </div>
                  </div>
                </>
              )}

              <ProductionPanel
                reports={data.reports}
                informes={data.informes}
                entities={data.entities}
                hasBaseline={hasBaseline}
              />
            </div>
          </>
        )}
      </div>
    </div>
  );
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
            <ArrowRight size={11} strokeWidth={2} />
            <span>estável · {previous} antes</span>
          </>
        ) : (
          <>
            <span className={favorable ? "up" : "dn"}>
              {diff > 0 ? <ArrowUp size={11} strokeWidth={2} /> : <ArrowDown size={11} strokeWidth={2} />}
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
