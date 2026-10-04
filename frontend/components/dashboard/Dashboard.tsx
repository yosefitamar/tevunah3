"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowDown, ArrowRight, ArrowUp, RefreshCw } from "lucide-react";
import { getDashboard, type Dashboard as DashboardData, type DashFacet } from "@/lib/dashboard-api";
import { INCIDENT_MEANS_LABEL, type IncidentMeans } from "@/lib/incidents-api";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import FacetBars, { type FacetBar } from "./FacetBars";
import IncidentSeries from "./IncidentSeries";
import PeriodButton from "../shared/PeriodButton";
import { DEFAULT_PERIOD, periodQuery, type PeriodSelection } from "@/lib/period";

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
  /** Formata valor e diferença (ex.: drogas em g/kg). Padrão: o número puro. */
  format?: (n: number) => string;
};

// Peso de droga: grama até 1 kg, quilo daí para cima — "582 g", "1,25 kg".
function formatWeight(g: number): string {
  if (Math.abs(g) < 1000) return `${g.toLocaleString("pt-BR", { maximumFractionDigits: 1 })} g`;
  return `${(g / 1000).toLocaleString("pt-BR", { maximumFractionDigits: 2 })} kg`;
}

// "2 ESPINGARDA · 1 CARABINA": a composição dos itens que o card soma.
function kindsHint(kinds: DashFacet[]): string {
  return kinds.map((k) => `${k.count} ${k.name}`).join(" · ");
}

export default function Dashboard() {
  const [period, setPeriod] = useState<PeriodSelection>(DEFAULT_PERIOD);
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [territory, setTerritory] = useState<Territory>("cities");

  const query = useMemo(() => periodQuery(period), [period]);

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
    const ops = data.operational;
    if (inc) {
      out.push({
        key: "homicidio",
        label: "HOMICÍDIOS · CVLI",
        value: inc.by_type.homicidio,
        previous: hasBaseline ? inc.prev_by_type.homicidio : null,
        lowerIsBetter: true,
        hint: mainMeansHint(inc.means, inc.by_type.homicidio),
      });
    }
    // Produção operacional em quantidades, do relatório diário do CPRAIO.
    if (ops) {
      const prev = hasBaseline ? ops.previous : null;
      out.push(
        {
          key: "armas",
          label: "ARMAS APREENDIDAS",
          value: ops.current.weapons,
          previous: prev ? prev.weapons : null,
          lowerIsBetter: false,
          hint: kindsHint(ops.weapon_kinds),
        },
        {
          key: "drogas",
          label: "DROGAS APREENDIDAS",
          value: ops.current.drugs_grams,
          previous: prev ? prev.drugs_grams : null,
          lowerIsBetter: false,
          hint: ops.drug_kinds.map((d) => `${d.name} ${formatWeight(d.grams)}`).join(" · "),
          format: formatWeight,
        },
        {
          key: "conduzidos",
          label: "CONDUZIDOS",
          value: ops.current.accused,
          previous: prev ? prev.accused : null,
          lowerIsBetter: false,
          hint:
            ops.current.adolescents > 0
              ? `${ops.current.adolescents} adolescente${ops.current.adolescents > 1 ? "s" : ""}`
              : "",
        },
        {
          key: "veiculos",
          label: "VEÍCULOS",
          value: ops.current.vehicles,
          previous: prev ? prev.vehicles : null,
          lowerIsBetter: false,
          hint: kindsHint(ops.vehicle_kinds),
        },
      );
    } else if (inc) {
      // Sem acesso ao relatório operacional, valem os tipos do cadastro manual.
      out.push(
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
    () => (data?.territory?.cities ?? []).map((f) => ({ key: f.name, label: f.name, count: f.count })),
    [data],
  );

  const neighborhoodBars: FacetBar[] = useMemo(
    () =>
      (data?.territory?.neighborhoods ?? []).map((f) => ({
        key: `${f.city}/${f.name}`,
        label: f.name,
        sub: f.city,
        count: f.count,
      })),
    [data],
  );

  const nothingVisible =
    data != null &&
    !data.incidents &&
    !data.operational &&
    !data.reports &&
    !data.informes &&
    !data.entities;

  const territoryBars = territory === "cities" ? cityBars : neighborhoodBars;

  // Painel enxuto para caber inteiro em 1920×1080 sem rolagem: indicadores,
  // tendência e território. Produção da agência e meio utilizado saíram —
  // o primeiro repetia o KPI de RIs difundidos, o segundo virou a linha de
  // apoio do KPI de homicídios.
  return (
    <div className="screen-fill">
      <div className="toolbar">
        <PeriodButton value={period} onChange={setPeriod} title="PERÍODO DO PAINEL" />
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
              Seu perfil não tem leitura de ocorrências, relatório operacional, relatórios, informes ou
              entidades — não há
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

            {(data.incidents || data.operational) && (
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
                    <IncidentSeries
                      series={data.incidents?.series}
                      operational={data.operational?.series}
                    />
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
  const fmt = kpi.format ?? ((n: number) => String(n));
  const diff = previous == null ? null : value - previous;
  // Sem base anterior (recorte aberto) ou base zerada, o percentual seria
  // ficção: mostra-se o absoluto.
  const pct = previous != null && previous > 0 ? Math.round((diff! / previous) * 100) : null;
  const favorable = diff == null || diff === 0 ? null : lowerIsBetter ? diff < 0 : diff > 0;

  return (
    <div className="panel kpi">
      <div className="kpi-lbl">{kpi.label}</div>
      <div className="kpi-val">{fmt(value)}</div>
      <div className="kpi-trend">
        {diff == null ? (
          <span className="muted">sem base de comparação</span>
        ) : diff === 0 ? (
          <>
            <ArrowRight size={13} strokeWidth={2} />
            <span>estável · {fmt(previous!)} antes</span>
          </>
        ) : (
          <>
            <span className={favorable ? "up" : "dn"}>
              {diff > 0 ? <ArrowUp size={13} strokeWidth={2} /> : <ArrowDown size={13} strokeWidth={2} />}
            </span>
            <span className={favorable ? "up" : "dn"}>
              {pct != null ? `${Math.abs(pct)}%` : `${diff > 0 ? "+" : "−"}${fmt(Math.abs(diff))}`}
            </span>
            <span className="muted">vs. {fmt(previous!)}</span>
          </>
        )}
      </div>
      {kpi.hint && <div className="kpi-hint muted">{kpi.hint}</div>}
    </div>
  );
}
