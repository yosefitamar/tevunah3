"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Filter, RefreshCcw, Search, ShieldAlert } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { listAudit, type AuditList } from "@/lib/audit-api";
import { canReadAudit } from "@/lib/permissions";
import { actionGroup, type AuditEntry } from "@/lib/types";
import { formatBR } from "@/lib/format";
import AuditDetail from "./AuditDetail";
import SortHeader, { type SortState } from "../shared/SortHeader";
import Select from "../shared/Select";
import DateInput from "../shared/DateInput";

const PAGE_SIZE = 25;

const ACTION_PREFIXES: Array<{ value: string; label: string }> = [
  { value: "", label: "TODAS" },
  { value: "auth.*", label: "AUTENTICAÇÃO (auth.*)" },
  { value: "user.*", label: "AGENTES (user.*)" },
  { value: "approval.*", label: "APROVAÇÕES (approval.*)" },
  { value: "audit.*", label: "AUDITORIA (audit.*)" },
];

const RESOURCE_TYPES: Array<{ value: string; label: string }> = [
  { value: "", label: "TODOS" },
  { value: "user", label: "AGENTE" },
  { value: "operation", label: "OPERAÇÃO" },
];

type Filters = {
  action: string;        // ex.: "auth.*"
  resourceType: string;
  from: string;          // YYYY-MM-DD
  to: string;
};

const EMPTY_FILTERS: Filters = { action: "", resourceType: "", from: "", to: "" };

function activeFilterCount(f: Filters) {
  let n = 0;
  if (f.action) n++;
  if (f.resourceType) n++;
  if (f.from) n++;
  if (f.to) n++;
  return n;
}

function shortHash(h: string): string {
  return h ? h.slice(0, 10) : "—";
}

function resourceCell(e: AuditEntry): string {
  if (!e.resource_type && !e.resource_id) return "—";
  const t = (e.resource_type ?? "").toUpperCase();
  const id = e.resource_id ?? "";
  if (!id) return t;
  const short = id.length > 12 ? id.slice(0, 8) + "…" : id;
  return t ? `${t}:${short}` : short;
}

function actorCell(e: AuditEntry): string {
  if (!e.actor_user_id) return "// SISTEMA";
  const code = e.actor_user_code ?? "";
  const name = (e.actor_display_name ?? "").toUpperCase();
  if (code && name) return `${code} · ${name}`;
  return code || name || (e.actor_user_id.slice(0, 8) + "…");
}

export default function AuditoriaScreen() {
  const { user: me } = useAuth();
  const [data, setData] = useState<AuditList | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "id", dir: "desc" });
  const [selectedId, setSelectedId] = useState<number | null>(null);

  const reload = useCallback(
    async (q: string, f: Filters, p: number, s: SortState) => {
      setLoading(true);
      setError(null);
      try {
        const res = await listAudit({
          limit: PAGE_SIZE,
          offset: p * PAGE_SIZE,
          search: q || undefined,
          action: f.action || undefined,
          resource_type: f.resourceType || undefined,
          from: f.from || undefined,
          to: f.to || undefined,
          sort_by: (s?.field as "id" | "ts" | "action" | "actor" | "resource") || undefined,
          sort_dir: s?.dir,
        });
        setData(res);
      } catch (e) {
        setError((e as Error).message || "Erro ao carregar trilho de auditoria");
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    if (canReadAudit(me)) reload(search, filters, page, sort);
  }, [me, page, search, filters, sort, reload]);

  function changeSort(next: SortState) {
    setPage(0);
    setSort(next);
  }

  if (!canReadAudit(me)) {
    return (
      <div className="placeholder">
        <div className="ph-tag">// MOD-04 / AUDITORIA</div>
        <div className="ph-ttl" style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <ShieldAlert size={22} /> ACESSO RESTRITO
        </div>
        <div className="ph-sub">
          Seu papel não permite consultar o trilho de auditoria. Esta seção é restrita
          a gestores e administradores. Tentativas de acesso são registradas.
        </div>
      </div>
    );
  }

  function onSearchSubmit(e: FormEvent) {
    e.preventDefault();
    setPage(0);
    setSearch(searchInput.trim());
  }

  const activeCount = activeFilterCount(filters);
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="screen-fill">
      <div className="section-title">
        TRILHO DE AUDITORIA · {total} REGISTRO{total === 1 ? "" : "S"}
        <span style={{ color: "var(--fg-2)" }}>· APPEND-ONLY · CADEIA DE HASH</span>
      </div>

      <div className="toolbar">
        <form onSubmit={onSearchSubmit} className="toolbar-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            placeholder="buscar por ação, código, e-mail ou id de recurso…"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
          />
          <button type="submit" className="btn btn-ghost">
            BUSCAR
          </button>
        </form>

        <button
          type="button"
          className={"btn" + (activeCount > 0 || filtersOpen ? " btn-active" : "")}
          onClick={() => setFiltersOpen((o) => !o)}
        >
          <Filter size={14} strokeWidth={1.8} /> FILTROS
          {activeCount > 0 && <span className="btn-badge">{activeCount}</span>}
        </button>

        <button
          type="button"
          className="btn"
          onClick={() => reload(search, filters, page, sort)}
          disabled={loading}
          title="Recarregar"
        >
          <RefreshCcw size={14} strokeWidth={1.8} /> ATUALIZAR
        </button>
      </div>

      {filtersOpen && (
        <FilterPanel
          value={filters}
          onApply={(f) => {
            setPage(0);
            setFilters(f);
            setFiltersOpen(false);
          }}
        />
      )}

      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="id" label="ID" sort={sort} onChange={changeSort} width={64} />
                <SortHeader field="ts" label="TIMESTAMP" sort={sort} onChange={changeSort} width={160} />
                <SortHeader field="action" label="AÇÃO" sort={sort} onChange={changeSort} width={220} />
                <SortHeader field="actor" label="ATOR" sort={sort} onChange={changeSort} />
                <SortHeader field="resource" label="RECURSO" sort={sort} onChange={changeSort} width={200} />
                <th style={{ width: 130 }}>IP</th>
                <th style={{ width: 120 }}>HASH</th>
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
              {!loading && data?.items.length === 0 && (
                <tr>
                  <td colSpan={7} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // NENHUM REGISTRO ENCONTRADO
                  </td>
                </tr>
              )}
              {!loading &&
                data?.items.map((e) => (
                  <tr
                    key={e.id}
                    onClick={() => setSelectedId(e.id)}
                    className="row-clickable"
                  >
                    <td className="id">#{e.id}</td>
                    <td className="muted">{formatBR(e.ts)}</td>
                    <td>
                      <span style={{ color: "var(--fg-0)", fontWeight: 600 }}>
                        {e.action}
                      </span>
                      <span className="muted" style={{ marginLeft: 6, fontSize: 9 }}>
                        / {actionGroup(e.action)}
                      </span>
                    </td>
                    <td>{actorCell(e)}</td>
                    <td className="muted">{resourceCell(e)}</td>
                    <td className="muted">{e.actor_ip ?? "—"}</td>
                    <td
                      className="muted"
                      style={{ fontFamily: "var(--font-mono)", letterSpacing: 0 }}
                      title={e.hash}
                    >
                      {shortHash(e.hash)}
                    </td>
                  </tr>
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
            <button
              type="button"
              disabled={page === 0}
              onClick={() => setPage((p) => Math.max(0, p - 1))}
            >
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

      {selectedId !== null && (
        <AuditDetail
          entryId={selectedId}
          onClose={() => setSelectedId(null)}
        />
      )}
    </div>
  );
}

// ─────────────────────────── FilterPanel ────────────────────────────

type FilterPanelProps = {
  value: Filters;
  onApply: (f: Filters) => void;
};

function FilterPanel({ value, onApply }: FilterPanelProps) {
  const [local, setLocal] = useState<Filters>(value);

  return (
    <div className="filter-panel">
      <div className="filter-row">
        <div className="filter-field">
          <span>FAMÍLIA DE AÇÃO</span>
          <Select
            value={local.action}
            onChange={(v) => setLocal({ ...local, action: v })}
            options={ACTION_PREFIXES.map((p) => ({
              value: p.value,
              label: p.label,
            }))}
          />
        </div>

        <div className="filter-field">
          <span>TIPO DE RECURSO</span>
          <Select
            value={local.resourceType}
            onChange={(v) => setLocal({ ...local, resourceType: v })}
            options={RESOURCE_TYPES.map((p) => ({
              value: p.value,
              label: p.label,
            }))}
          />
        </div>

        <div className="filter-field">
          <span>DE</span>
          <DateInput
            value={local.from}
            onChange={(v) => setLocal({ ...local, from: v })}
          />
        </div>

        <div className="filter-field">
          <span>ATÉ</span>
          <DateInput
            value={local.to}
            onChange={(v) => setLocal({ ...local, to: v })}
          />
        </div>
      </div>
      <div className="filter-actions">
        <button type="button" className="btn btn-ghost" onClick={() => onApply(EMPTY_FILTERS)}>
          LIMPAR
        </button>
        <button type="button" className="btn btn-primary" onClick={() => onApply(local)}>
          APLICAR
        </button>
      </div>
    </div>
  );
}
