"use client";

import { useCallback, useEffect, useState } from "react";
import { MapPin, Plus, Search, ShieldAlert, Users } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  INCIDENT_TYPE_LABEL,
  INCIDENT_TYPE_PILL,
  INCIDENT_TYPES,
  listIncidents,
  type Incident,
  type IncidentType,
  type IncidentsList,
} from "@/lib/incidents-api";
import { canCreateIncidents, canReadIncidents } from "@/lib/permissions";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import Select from "../shared/Select";
import CreateOcorrenciaModal from "./CreateOcorrenciaModal";
import OcorrenciaDrawer from "./OcorrenciaDrawer";

const PAGE_SIZE = 25;

type Filters = {
  type: "" | IncidentType;
  intel: boolean;
  search: string;
};

const EMPTY_FILTERS: Filters = { type: "", intel: false, search: "" };

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

  const reload = useCallback(async () => {
    if (!canRead) return;
    setLoading(true);
    setError(null);
    try {
      const res = await listIncidents({
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        type: filters.type || undefined,
        intel: filters.intel || undefined,
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
          value={filters.intel ? "1" : ""}
          onChange={(v) => {
            setFilters({ ...filters, intel: v === "1" });
            setPage(0);
          }}
          className="sel--toolbar"
          placeholder="INTEL · TODAS"
          options={[
            { value: "", label: "INTEL · TODAS" },
            { value: "1", label: "INTEL · COM PARTICIPAÇÃO" },
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
                <th>FICHA CIOPS</th>
                <th style={{ width: 70 }}>INTEL</th>
                <th>DESCRIÇÃO</th>
                <th style={{ width: 110 }}>ENVOLVIDOS</th>
                <SortHeader field="updated_at" label="ATUALIZADO" sort={sort} onChange={setSort} width={140} />
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={7} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // CARREGANDO…
                  </td>
                </tr>
              )}
              {!loading && items.length === 0 && (
                <tr>
                  <td colSpan={7} className="muted" style={{ textAlign: "center", padding: 32 }}>
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
        {formatBR(incident.occurred_on)}
        {incident.occurred_time ? (
          <span className="muted"> · {incident.occurred_time}</span>
        ) : null}
      </td>
      <td className="muted">{incident.ciops_record || "—"}</td>
      <td>
        {incident.intel_participation ? (
          <span className="pill info">SIM</span>
        ) : (
          <span className="muted">—</span>
        )}
      </td>
      <td style={{ color: "var(--fg-0)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          {(incident.latitude != null || incident.longitude != null) && (
            <MapPin size={12} strokeWidth={1.6} className="muted" />
          )}
          {short ? short : <span className="muted">(sem descrição)</span>}
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
