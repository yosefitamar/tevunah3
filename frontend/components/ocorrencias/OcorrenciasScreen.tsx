"use client";

import { useCallback, useEffect, useState } from "react";
import { MapPin, Plus, Search, ShieldAlert, Users } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  INCIDENT_MEANS,
  INCIDENT_MEANS_LABEL,
  INCIDENT_MEANS_SHORT,
  INCIDENT_TYPE_LABEL,
  INCIDENT_TYPE_PILL,
  INCIDENT_TYPES,
  listIncidents,
  type Incident,
  type IncidentMeans,
  type IncidentType,
  type IncidentsList,
} from "@/lib/incidents-api";
import { canCreateIncidents, canReadIncidents } from "@/lib/permissions";
import { useIncidentLocations } from "@/lib/useIncidentLocations";
import { formatBR, formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import Select from "../shared/Select";
import CreateOcorrenciaModal from "./CreateOcorrenciaModal";
import OcorrenciaDrawer from "./OcorrenciaDrawer";

const PAGE_SIZE = 25;

type Filters = {
  type: "" | IncidentType;
  means: IncidentMeans;
  city: string;
  search: string;
};

const EMPTY_FILTERS: Filters = { type: "", means: "", city: "", search: "" };

export default function OcorrenciasScreen() {
  const { user: me } = useAuth();
  const [data, setData] = useState<IncidentsList | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "occurred_on", dir: "desc" });
  const [showCreate, setShowCreate] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);

  const canRead = canReadIncidents(me);
  const canCreate = canCreateIncidents(me);
  const { cities } = useIncidentLocations();

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
        search: filters.search.trim() || undefined,
        sort_by: (sort?.field as "occurred_on" | "type" | "created_at" | "updated_at") || undefined,
        sort_dir: sort?.dir,
      });
      setData(res);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [canRead, filters, page, sort]);

  useEffect(() => {
    reload();
  }, [reload]);

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
        <div className="toolbar-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            value={filters.search}
            onChange={(e) => {
              setFilters({ ...filters, search: e.target.value });
              setPage(0);
            }}
            placeholder="buscar por descrição ou ficha CIOPS…"
          />
        </div>
        <Select
          value={filters.type}
          onChange={(v) => {
            setFilters({ ...filters, type: v as "" | IncidentType });
            setPage(0);
          }}
          className="sel--toolbar"
          placeholder="TIPO · TODOS"
          options={[
            { value: "", label: "TIPO · TODOS" },
            ...INCIDENT_TYPES.map((t) => ({ value: t, label: `TIPO · ${INCIDENT_TYPE_LABEL[t]}` })),
          ]}
        />
        <Select
          value={filters.means}
          onChange={(v) => {
            setFilters({ ...filters, means: v as IncidentMeans });
            setPage(0);
          }}
          className="sel--toolbar"
          placeholder="MEIO · TODOS"
          options={[
            { value: "", label: "MEIO · TODOS" },
            ...INCIDENT_MEANS.map((m) => ({ value: m, label: `MEIO · ${INCIDENT_MEANS_SHORT[m]}` })),
          ]}
        />
        <Select
          value={filters.city}
          onChange={(v) => {
            setFilters({ ...filters, city: v });
            setPage(0);
          }}
          className="sel--toolbar"
          placeholder="MUNICÍPIO · TODOS"
          options={[
            { value: "", label: "MUNICÍPIO · TODOS" },
            ...cities.map((c) => ({
              value: c.city,
              label: `${c.city} (${c.count})`,
            })),
          ]}
        />
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
