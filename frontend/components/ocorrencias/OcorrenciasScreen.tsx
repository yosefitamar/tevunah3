"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { MapPin, Plus, Search, ShieldAlert, SlidersHorizontal, Users, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  INCIDENT_MEANS_LABEL,
  INCIDENT_MEANS_SHORT,
  INCIDENT_TYPE_LABEL,
  INCIDENT_TYPE_PILL,
  listIncidents,
  type Incident,
  type IncidentsList,
} from "@/lib/incidents-api";
import { canCreateIncidents, canReadIncidents } from "@/lib/permissions";
import { useIncidentLocations } from "@/lib/useIncidentLocations";
import { RANGE_LABEL, resolveRange } from "@/lib/date-ranges";
import { formatBR, formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import IncidentFiltersModal, {
  type IncidentFilters,
} from "../shared/IncidentFiltersModal";
import CreateOcorrenciaModal from "./CreateOcorrenciaModal";
import OcorrenciaDrawer from "./OcorrenciaDrawer";

const PAGE_SIZE = 25;

// A listagem é o acervo inteiro: nasce sem recorte nenhum. (O mapa parte de
// CVLI no mês atual porque é uma leitura territorial, não um índice.)
const DEFAULT_FILTERS: IncidentFilters = {
  range: "tudo",
  from: "",
  to: "",
  type: "",
  means: "",
  city: "",
  neighborhood: "",
};

export default function OcorrenciasScreen() {
  const { user: me } = useAuth();
  const [data, setData] = useState<IncidentsList | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<IncidentFilters>(DEFAULT_FILTERS);
  const [showFilters, setShowFilters] = useState(false);
  // Busca livre fica fora do modal: é o gesto mais frequente ("cadê a
  // ocorrência do fulano?"), não um recorte que se configura uma vez.
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "occurred_on", dir: "desc" });
  const [showCreate, setShowCreate] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);

  const canRead = canReadIncidents(me);
  const canCreate = canCreateIncidents(me);
  const { cities, neighborhoodsOf } = useIncidentLocations();

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
    try {
      const res = await listIncidents({
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        type: filters.type || undefined,
        means: filters.means || undefined,
        city: filters.city || undefined,
        neighborhood: filters.neighborhood || undefined,
        date_from: period.from || undefined,
        date_to: period.to || undefined,
        search: debouncedSearch || undefined,
        sort_by: (sort?.field as "occurred_on" | "type" | "created_at" | "updated_at") || undefined,
        sort_dir: sort?.dir,
      });
      setData(res);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [
    canRead,
    filters.type,
    filters.means,
    filters.city,
    filters.neighborhood,
    period.from,
    period.to,
    debouncedSearch,
    page,
    sort,
  ]);

  useEffect(() => {
    reload();
  }, [reload]);

  // Badge do botão: quantos recortes fogem do padrão da tela — casa com o
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

  // Resumo ao lado do botão: com os campos dentro do modal, o recorte
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

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="screen-fill">
      <div className="toolbar">
        <div className="toolbar-search toolbar-search--wide">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            value={search}
            onChange={(e) => {
              // A página 3 do recorte anterior não existe mais depois que o
              // termo muda.
              setSearch(e.target.value);
              setPage(0);
            }}
            placeholder="buscar por nome, CPF, descrição ou ficha CIOPS…"
          />
          {search && (
            <button
              type="button"
              className="action-btn"
              onClick={() => {
                setSearch("");
                setPage(0);
              }}
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
        <span className="muted filter-summary" title={filterSummary}>
          {filterSummary}
        </span>
        <div style={{ marginLeft: "auto" }} />
        {canCreate && (
          <button type="button" className="btn btn-primary" onClick={() => setShowCreate(true)}>
            <Plus size={14} strokeWidth={2} /> NOVA OCORRÊNCIA
          </button>
        )}
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="type" label="TIPO" sort={sort} onChange={setSort} width={130} />
                <SortHeader field="occurred_on" label="DATA / HORA" sort={sort} onChange={setSort} width={150} />
                <th style={{ width: 110 }}>MEIO</th>
                <th>FICHA CIOPS</th>
                <th style={{ width: 190 }}>LOCAL</th>
                <th>DESCRIÇÃO</th>
                <th style={{ width: 110 }}>ENVOLVIDOS</th>
                <SortHeader field="updated_at" label="ATUALIZADO" sort={sort} onChange={setSort} width={140} />
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={8} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // CARREGANDO…
                  </td>
                </tr>
              )}
              {!loading && items.length === 0 && (
                <tr>
                  <td colSpan={8} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // NENHUMA OCORRÊNCIA ENCONTRADA
                  </td>
                </tr>
              )}
              {!loading &&
                items.map((it) => (
                  <Row key={it.id} incident={it} onOpen={() => setOpenId(it.id)} />
                ))}
            </tbody>
          </table>
        </div>

        <div className="pagination">
          <span className="muted">
            {total === 0
              ? "—"
              : `${page * PAGE_SIZE + 1}–${Math.min((page + 1) * PAGE_SIZE, total)} de ${total}`}
          </span>
          <div className="pagination-controls">
            <button type="button" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
              ‹ ANTERIOR
            </button>
            <span>
              PÁGINA {page + 1} / {pages}
            </span>
            <button
              type="button"
              disabled={page >= pages - 1}
              onClick={() => setPage((p) => Math.min(pages - 1, p + 1))}
            >
              PRÓXIMA ›
            </button>
          </div>
        </div>
      </div>

      {showFilters && (
        <IncidentFiltersModal
          title="FILTROS DAS OCORRÊNCIAS"
          value={filters}
          defaults={DEFAULT_FILTERS}
          cities={cities}
          neighborhoodsOf={neighborhoodsOf}
          onApply={(f) => {
            setFilters(f);
            setPage(0);
            setShowFilters(false);
          }}
          onClose={() => setShowFilters(false)}
        />
      )}

      {showCreate && (
        <CreateOcorrenciaModal
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            setShowCreate(false);
            setOpenId(id);
            reload();
          }}
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

function Row({ incident, onOpen }: { incident: Incident; onOpen: () => void }) {
  const desc = incident.description.trim();
  const short = desc.length > 90 ? desc.slice(0, 90) + "…" : desc;
  return (
    <tr onClick={onOpen} className="row-clickable">
      <td>
        <span className={"pill " + INCIDENT_TYPE_PILL[incident.type]}>
          {INCIDENT_TYPE_LABEL[incident.type]}
        </span>
      </td>
      <td style={{ whiteSpace: "nowrap" }}>
        {formatBRDate(incident.occurred_on)}
        {incident.occurred_time ? (
          <span className="muted"> · {incident.occurred_time}</span>
        ) : null}
      </td>
      <td
        className={incident.means ? undefined : "muted"}
        title={incident.means ? INCIDENT_MEANS_LABEL[incident.means] : undefined}
      >
        {incident.means ? INCIDENT_MEANS_SHORT[incident.means] : "—"}
      </td>
      <td className="muted">{incident.ciops_record || "—"}</td>
      <td>
        {incident.city || incident.neighborhood ? (
          <>
            <div style={{ color: "var(--fg-0)" }}>{incident.neighborhood || "—"}</div>
            <div className="muted" style={{ fontSize: 10 }}>
              {incident.city}
            </div>
          </>
        ) : (
          <span className="muted">—</span>
        )}
      </td>
      <td style={{ color: "var(--fg-0)" }}>
        <div className="tbl-desc-cell" title={desc || undefined}>
          {(incident.latitude != null || incident.longitude != null) && (
            <MapPin size={12} strokeWidth={1.6} className="muted" />
          )}
          <span className="tbl-desc-text">
            {short ? short : <span className="muted">(sem descrição)</span>}
          </span>
        </div>
      </td>
      <td className="muted">
        {incident.involved.length > 0 ? (
          <span style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
            <Users size={12} strokeWidth={1.6} />
            {incident.involved.length}
          </span>
        ) : (
          "—"
        )}
      </td>
      <td className="muted">{formatBR(incident.updated_at)}</td>
    </tr>
  );
}
