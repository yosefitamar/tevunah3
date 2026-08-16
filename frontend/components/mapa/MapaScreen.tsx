"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { MapPinned, RefreshCw, Search, ShieldAlert, SlidersHorizontal, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  INCIDENT_MEANS,
  INCIDENT_MEANS_COLOR,
  INCIDENT_MEANS_LABEL,
  INCIDENT_TYPE_LABEL,
  getIncident,
  listIncidents,
  listIncidentsGeo,
  type Incident,
  type IncidentMeans,
} from "@/lib/incidents-api";
import { canReadIncidents } from "@/lib/permissions";
import { RANGE_LABEL, resolveRange } from "@/lib/date-ranges";
import { useIncidentLocations } from "@/lib/useIncidentLocations";
import { useNavigation } from "@/contexts/NavigationContext";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import OcorrenciaDrawer from "../ocorrencias/OcorrenciaDrawer";
import FiltrosModal, { type MapFilters } from "./FiltrosModal";

// O Leaflet acessa `window` já no import — carrega só no cliente.
const CrimeMap = dynamic(() => import("./CrimeMap"), {
  ssr: false,
  loading: () => (
    <div className="map-loading muted">// CARREGANDO MAPA…</div>
  ),
});

const DEFAULT_FILTERS: MapFilters = {
  range: "mes_atual",
  from: "",
  to: "",
  // O mapa nasce focado em CVLI — é o caso de uso que originou a tela.
  type: "homicidio",
  means: "",
  city: "",
  neighborhood: "",
};

export default function MapaScreen() {
  const { user: me } = useAuth();
  const [filters, setFilters] = useState<MapFilters>(DEFAULT_FILTERS);
  const [showFilters, setShowFilters] = useState(false);
  // Busca livre: fica fora do modal porque é o gesto mais frequente do
  // analista ("cadê a ocorrência do fulano?") e não um recorte que se
  // configura uma vez.
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [items, setItems] = useState<Incident[]>([]);
  const [truncated, setTruncated] = useState(false);
  // Total de ocorrências do recorte (com ou sem coordenadas), pra medir a
  // cobertura de georreferenciamento.
  const [totalInRange, setTotalInRange] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dark, setDark] = useState(true);
  const [openId, setOpenId] = useState<string | null>(null);

  const canRead = canReadIncidents(me);
  const colorFor = useResolvedMeansColors();
  const { cities, neighborhoodsOf } = useIncidentLocations();
  const { pending, consumePending } = useNavigation();
  // Ponto a destacar quando se chega aqui pela ficha de uma vítima.
  const [focusId, setFocusId] = useState<string | null>(null);

  // Chegada com foco (ex.: "ver no mapa" na ficha de quem morreu): o recorte
  // corrente provavelmente não contém aquela ocorrência, então ele é
  // reajustado para o mês do fato antes de centralizar o pino.
  // handledRef evita repetir o foco; consumir o `pending` aqui no início
  // re-renderizaria e cancelaria a própria busca antes de ela responder.
  const handledFocusRef = useRef<string | null>(null);

  useEffect(() => {
    const target = pending?.incidentId;
    if (!target || handledFocusRef.current === target) return;
    handledFocusRef.current = target;
    getIncident(target)
      .then(({ incident }) => {
        const [y, m] = incident.occurred_on.split("-").map(Number);
        const last = new Date(y, m, 0).getDate();
        setFilters({
          range: "custom",
          from: `${incident.occurred_on.slice(0, 7)}-01`,
          to: `${incident.occurred_on.slice(0, 7)}-${String(last).padStart(2, "0")}`,
          type: "",
          means: "",
          city: "",
          neighborhood: "",
        });
        setFocusId(target);
        setOpenId(target);
      })
      .catch(() => undefined)
      .finally(() => consumePending());
  }, [pending, consumePending]);

  // Debounce: cada tecla dispararia duas consultas (geo + total).
  useEffect(() => {
    const h = window.setTimeout(() => setDebouncedSearch(search.trim()), 350);
    return () => window.clearTimeout(h);
  }, [search]);

  const period = useMemo(() => {
    if (filters.range === "custom") return { from: filters.from, to: filters.to };
    return resolveRange(filters.range);
  }, [filters.range, filters.from, filters.to]);

  const reload = useCallback(async () => {
    if (!canRead) return;
    setLoading(true);
    setError(null);
    const query = {
      type: filters.type || undefined,
      means: filters.means || undefined,
      city: filters.city || undefined,
      neighborhood: filters.neighborhood || undefined,
      search: debouncedSearch || undefined,
      date_from: period.from || undefined,
      date_to: period.to || undefined,
    };
    try {
      const [geo, all] = await Promise.all([
        listIncidentsGeo(query),
        // limit=1: só interessa o `total` do mesmo recorte.
        listIncidents({ ...query, limit: 1 }),
      ]);
      setItems(geo.items);
      setTruncated(geo.truncated);
      setTotalInRange(all.total);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar o mapa");
    } finally {
      setLoading(false);
    }
  }, [
    canRead,
    filters.type,
    filters.means,
    filters.city,
    filters.neighborhood,
    debouncedSearch,
    period.from,
    period.to,
  ]);

  useEffect(() => {
    reload();
  }, [reload]);

  // Distribuição por meio utilizado no recorte plotado (alimenta a legenda).
  const byMeans = useMemo(() => {
    const acc = new Map<IncidentMeans, number>();
    for (const i of items) acc.set(i.means, (acc.get(i.means) ?? 0) + 1);
    return acc;
  }, [items]);

  // Bairros mais atingidos no recorte — é a leitura que a agência faz para
  // decidir onde concentrar esforço.
  const topNeighborhoods = useMemo(() => {
    const acc = new Map<string, number>();
    for (const i of items) {
      const key = i.neighborhood || "(SEM BAIRRO)";
      acc.set(key, (acc.get(key) ?? 0) + 1);
    }
    return [...acc.entries()].sort((a, b) => b[1] - a[1]).slice(0, 8);
  }, [items]);

  // Badge do botão: quantos recortes fogem do padrão da tela. Casa com o
  // "LIMPAR" do modal, que restaura exatamente esse padrão.
  const activeCount = useMemo(() => {
    let n = 0;
    if (filters.range !== DEFAULT_FILTERS.range) n++;
    if (filters.type !== DEFAULT_FILTERS.type) n++;
    if (filters.means) n++;
    if (filters.city) n++;
    if (filters.neighborhood) n++;
    return n;
  }, [filters]);

  // Resumo ao lado do botão — com os campos dentro do modal, o recorte
  // corrente precisa continuar visível sem abrir nada.
  const filterSummary = useMemo(() => {
    const parts = [
      filters.range === "custom"
        ? `${filters.from ? formatBRDate(filters.from) : "…"} → ${
            filters.to ? formatBRDate(filters.to) : "…"
          }`
        : RANGE_LABEL[filters.range],
      filters.type ? INCIDENT_TYPE_LABEL[filters.type] : "TODOS OS TIPOS",
    ];
    if (filters.means) parts.push(INCIDENT_MEANS_LABEL[filters.means]);
    if (filters.city) parts.push(filters.city);
    if (filters.neighborhood) parts.push(filters.neighborhood);
    return parts.join(" · ");
  }, [filters]);

  if (!canRead) {
    return (
      <div className="placeholder">
        <ShieldAlert size={36} strokeWidth={1.2} />
        <div className="ph-tag">// ACESSO RESTRITO</div>
        <div className="ph-ttl">SEM PERMISSÃO DE LEITURA DE OCORRÊNCIAS</div>
        <div className="ph-sub">Contate o administrador.</div>
      </div>
    );
  }

  const semGeo = totalInRange != null ? Math.max(0, totalInRange - items.length) : null;

  return (
    <div className="screen-fill">
      <div className="toolbar">
        <div className="toolbar-search map-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="buscar por nome, CPF ou descrição…"
          />
          {search && (
            <button
              type="button"
              className="action-btn"
              onClick={() => setSearch("")}
              aria-label="Limpar busca"
            >
              <X size={12} />
            </button>
          )}
        </div>
        <button
          type="button"
          className={"btn" + (activeCount > 0 ? " btn-primary" : "")}
          onClick={() => setShowFilters(true)}
        >
          <SlidersHorizontal size={13} strokeWidth={1.8} /> FILTROS
          {activeCount > 0 && <span className="btn-count">{activeCount}</span>}
        </button>
        <span className="muted map-filter-summary" title={filterSummary}>
          {filterSummary}
        </span>
        <div style={{ marginLeft: "auto" }} />
        <button
          type="button"
          className={"btn" + (dark ? " btn-primary" : " btn-ghost")}
          onClick={() => setDark((d) => !d)}
          aria-pressed={dark}
          title="Alterna entre o mapa escurecido e as cores originais do OpenStreetMap"
        >
          MODO ESCURO
        </button>
        <button type="button" className="btn btn-ghost" onClick={reload} disabled={loading}>
          <RefreshCw size={13} strokeWidth={1.8} /> {loading ? "CARREGANDO…" : "ATUALIZAR"}
        </button>
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}
      {truncated && (
        <div className="banner banner-warn">
          ⚠ O recorte excedeu o limite de pontos do servidor e foi truncado. Estreite o
          período para uma leitura fiel.
        </div>
      )}
      {semGeo != null && semGeo > 0 && (
        <div className="banner">
          {semGeo} ocorrência{semGeo > 1 ? "s" : ""} do recorte {semGeo > 1 ? "estão" : "está"}{" "}
          sem coordenadas e não {semGeo > 1 ? "aparecem" : "aparece"} no mapa.
        </div>
      )}

      <div className="map-layout">
        <div className="panel panel--fill map-panel">
          {loading && items.length === 0 ? (
            <div className="map-loading muted">// CARREGANDO PONTOS…</div>
          ) : (
            <CrimeMap
              items={items}
              colorFor={colorFor}
              dark={dark}
              focusId={focusId}
              onOpen={(id) => setOpenId(id)}
            />
          )}
        </div>

        <aside className="map-side">
          <div className="panel">
            <div className="panel-hd">
              <span className="ttl">RECORTE</span>
            </div>
            <div className="panel-bd">
              <div className="kpi" style={{ padding: 0 }}>
                <div className="kpi-lbl">PONTOS PLOTADOS</div>
                <div className="kpi-val">{items.length}</div>
              </div>
              <dl className="dossier-list" style={{ marginTop: 12 }}>
                <div>
                  <dt>PERÍODO</dt>
                  <dd>
                    {formatBRDate(period.from)} → {formatBRDate(period.to)}
                  </dd>
                </div>
                <div>
                  <dt>TERRITÓRIO</dt>
                  <dd>
                    {filters.city
                      ? [filters.neighborhood, filters.city].filter(Boolean).join(" · ")
                      : "TODOS OS MUNICÍPIOS"}
                  </dd>
                </div>
                <div>
                  <dt>NO RECORTE</dt>
                  <dd>{totalInRange ?? "—"} ocorrência(s)</dd>
                </div>
              </dl>
            </div>
          </div>

          <div className="panel">
            <div className="panel-hd">
              <span className="ttl">MEIO UTILIZADO</span>
            </div>
            <div className="panel-bd">
              {items.length === 0 && (
                <div className="muted" style={{ fontSize: 11 }}>
                  // NENHUM PONTO NO RECORTE
                </div>
              )}
              {items.length > 0 &&
                ([...INCIDENT_MEANS, "" as IncidentMeans] as IncidentMeans[])
                  .filter((m) => (byMeans.get(m) ?? 0) > 0)
                  .map((m) => {
                    const n = byMeans.get(m) ?? 0;
                    const pct = Math.round((n / items.length) * 100);
                    return (
                      <div key={m || "ni"} className="map-legend-row">
                        <span
                          className="map-legend-dot"
                          style={{ background: colorFor(m) }}
                          aria-hidden
                        />
                        <span className="map-legend-lbl">{INCIDENT_MEANS_LABEL[m]}</span>
                        <span className="map-legend-val">
                          {n} <span className="muted">· {pct}%</span>
                        </span>
                      </div>
                    );
                  })}
            </div>
          </div>

          <div className="panel">
            <div className="panel-hd">
              <span className="ttl">BAIRROS</span>
              <span className="meta">TOP 8</span>
            </div>
            <div className="panel-bd">
              {topNeighborhoods.length === 0 && (
                <div className="muted" style={{ fontSize: 11 }}>
                  // NENHUM PONTO NO RECORTE
                </div>
              )}
              {topNeighborhoods.map(([name, n]) => (
                <button
                  key={name}
                  type="button"
                  className="map-legend-row map-legend-row--action"
                  // Clicar aprofunda o recorte no bairro (exceto o balde dos
                  // registros sem bairro informado, que não é filtrável).
                  disabled={name === "(SEM BAIRRO)" || !filters.city}
                  onClick={() => setFilters({ ...filters, neighborhood: name })}
                  title={
                    !filters.city
                      ? "Selecione um município para filtrar por bairro"
                      : `Filtrar por ${name}`
                  }
                >
                  <span className="map-legend-lbl">{name}</span>
                  <span className="map-legend-val">{n}</span>
                </button>
              ))}
            </div>
          </div>

          <div className="panel">
            <div className="panel-hd">
              <span className="ttl">COMO LER</span>
            </div>
            <div className="panel-bd">
              <div className="muted" style={{ fontSize: 10.5, lineHeight: 1.7 }}>
                <MapPinned size={12} strokeWidth={1.6} /> Cada ponto é uma ocorrência
                georreferenciada. Clique para ver os dados, os envolvidos e abrir o
                dossiê completo.
              </div>
            </div>
          </div>
        </aside>
      </div>

      {showFilters && (
        <FiltrosModal
          value={filters}
          defaults={DEFAULT_FILTERS}
          cities={cities}
          neighborhoodsOf={neighborhoodsOf}
          onApply={(f) => {
            setFilters(f);
            setShowFilters(false);
          }}
          onClose={() => setShowFilters(false)}
        />
      )}

      {openId && (
        <OcorrenciaDrawer
          incidentId={openId}
          onClose={() => setOpenId(null)}
          onChanged={reload}
        />
      )}
    </div>
  );
}

/**
 * Resolve as cores dos pontos a partir das CSS custom properties da paleta
 * ativa — o Leaflet pinta o SVG por atributo e não herda `var(--x)` de forma
 * confiável. Re-resolve quando o usuário troca de paleta (data-palette).
 */
function useResolvedMeansColors(): (means: IncidentMeans) => string {
  const [resolved, setResolved] = useState<Record<string, string>>({});

  useEffect(() => {
    const root = document.documentElement;
    const read = () => {
      const cs = getComputedStyle(root);
      const next: Record<string, string> = {};
      for (const [means, token] of Object.entries(INCIDENT_MEANS_COLOR)) {
        const varName = token.replace(/^var\(|\)$/g, "").trim();
        next[means] = cs.getPropertyValue(varName).trim() || "#888888";
      }
      setResolved(next);
    };
    read();
    const obs = new MutationObserver(read);
    obs.observe(root, { attributes: true, attributeFilter: ["data-palette"] });
    return () => obs.disconnect();
  }, []);

  return useCallback((means: IncidentMeans) => resolved[means] ?? "#888888", [resolved]);
}
