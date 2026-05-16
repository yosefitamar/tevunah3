"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Filter, Plus, Search, ShieldAlert } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { listUsers, type UsersList } from "@/lib/users-api";
import { canListUsers, canCreateUsers } from "@/lib/permissions";
import {
  ROLE_LABEL,
  ROLES_LIST,
  STATUS_LABEL,
  STATUS_PILL,
  clearanceLabel,
  type RoleCode,
  type UserStatus,
} from "@/lib/types";
import { formatBR } from "@/lib/format";
import CreateAgentModal from "./CreateAgentModal";
import AgentDrawer from "./AgentDrawer";
import SortHeader, { type SortState } from "../shared/SortHeader";
import Select from "../shared/Select";

const PAGE_SIZE = 25;

type Filters = {
  role: RoleCode | "";
  clearance: number; // 0 = todos
  status: "" | UserStatus;
};

const EMPTY_FILTERS: Filters = { role: "", clearance: 0, status: "" };

function activeFilterCount(f: Filters) {
  let n = 0;
  if (f.role) n++;
  if (f.clearance) n++;
  if (f.status) n++;
  return n;
}

export default function AgentesScreen() {
  const { user: me } = useAuth();
  const [data, setData] = useState<UsersList | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "code", dir: "asc" });
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const reload = useCallback(
    async (q: string, f: Filters, p: number, s: SortState) => {
      setLoading(true);
      setError(null);
      try {
        const res = await listUsers({
          limit: PAGE_SIZE,
          offset: p * PAGE_SIZE,
          search: q || undefined,
          role: f.role || undefined,
          status: f.status || undefined,
          clearance: f.clearance || undefined,
          sort_by: (s?.field as "code" | "display_name" | "email" | "clearance_level" | "status" | "last_login_at") || undefined,
          sort_dir: s?.dir,
        });
        setData(res);
      } catch (e) {
        setError((e as Error).message || "Erro ao listar agentes");
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    if (canListUsers(me)) reload(search, filters, page, sort);
  }, [me, page, search, filters, sort, reload]);

  function changeSort(next: SortState) {
    setPage(0);
    setSort(next);
  }

  if (!canListUsers(me)) {
    return (
      <div className="placeholder">
        <div className="ph-tag">// MOD-02 / AGENTES</div>
        <div className="ph-ttl" style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <ShieldAlert size={22} /> ACESSO RESTRITO
        </div>
        <div className="ph-sub">
          Seu papel não permite visualizar o cadastro de agentes. Esta seção é restrita a gestores
          e administradores. Tentativas de acesso são registradas no trilho de auditoria.
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
        AGENTES · {total} REGISTRO{total === 1 ? "" : "S"}
        <span style={{ color: "var(--fg-2)" }}>· CADASTRO UNIFICADO</span>
      </div>

      <div className="toolbar">
        <form onSubmit={onSearchSubmit} className="toolbar-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            placeholder="buscar por código, e-mail ou nome…"
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

        {canCreateUsers(me) && (
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => setCreateOpen(true)}
          >
            <Plus size={14} strokeWidth={2} /> NOVO AGENTE
          </button>
        )}
      </div>

      {filtersOpen && (
        <FilterPanel
          value={filters}
          onApply={(f) => {
            setPage(0);
            setFilters(f);
            setFiltersOpen(false);
          }}
          onClose={() => setFiltersOpen(false)}
        />
      )}

      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="code" label="CÓDIGO" sort={sort} onChange={changeSort} width={90} />
                <SortHeader field="display_name" label="NOME" sort={sort} onChange={changeSort} />
                <SortHeader field="email" label="E-MAIL" sort={sort} onChange={changeSort} />
                <th style={{ width: 200 }}>PAPEL</th>
                <SortHeader field="clearance_level" label="CL" sort={sort} onChange={changeSort} width={60} />
                <SortHeader field="status" label="STATUS" sort={sort} onChange={changeSort} width={110} />
                <SortHeader field="last_login_at" label="ÚLT. LOGIN" sort={sort} onChange={changeSort} width={130} />
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
                data?.items.map((u) => (
                  <tr
                    key={u.id}
                    onClick={() => setSelectedId(u.id)}
                    className="row-clickable"
                  >
                    <td className="id">{u.code}</td>
                    <td style={{ color: "var(--fg-0)", fontWeight: 600 }}>
                      {u.display_name.toUpperCase()}
                    </td>
                    <td className="muted">{u.email}</td>
                    <td>
                      {u.roles
                        .map((r) => ROLE_LABEL[r as RoleCode] ?? r.toUpperCase())
                        .join(" · ")}
                    </td>
                    <td>{clearanceLabel(u.clearance_level)}</td>
                    <td>
                      <span className={"pill " + STATUS_PILL[u.status as UserStatus]}>
                        {STATUS_LABEL[u.status as UserStatus]}
                      </span>
                    </td>
                    <td className="muted">{formatBR(u.last_login_at)}</td>
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

      {createOpen && (
        <CreateAgentModal
          onClose={() => setCreateOpen(false)}
          onCreated={() => {
            setCreateOpen(false);
            reload(search, filters, page, sort);
          }}
        />
      )}

      {selectedId && (
        <AgentDrawer
          userId={selectedId}
          onClose={() => setSelectedId(null)}
          onChanged={() => reload(search, filters, page)}
        />
      )}
    </div>
  );
}

// ─────────────────────────── FilterPanel ────────────────────────────

type FilterPanelProps = {
  value: Filters;
  onApply: (f: Filters) => void;
  onClose: () => void;
};

function FilterPanel({ value, onApply }: FilterPanelProps) {
  const [local, setLocal] = useState<Filters>(value);

  return (
    <div className="filter-panel">
      <div className="filter-row">
        <div className="filter-field">
          <span>PAPEL</span>
          <Select
            value={local.role}
            onChange={(v) => setLocal({ ...local, role: v as RoleCode | "" })}
            placeholder="TODOS"
            options={[
              { value: "", label: "TODOS" },
              ...ROLES_LIST.map((r) => ({ value: r, label: ROLE_LABEL[r] })),
            ]}
          />
        </div>

        <div className="filter-field">
          <span>CLEARANCE</span>
          <Select
            value={String(local.clearance)}
            onChange={(v) => setLocal({ ...local, clearance: Number(v) })}
            placeholder="TODOS"
            options={[
              { value: "0", label: "TODOS" },
              ...[1, 2, 3, 4, 5].map((n) => ({
                value: String(n),
                label: `CL-0${n}`,
              })),
            ]}
          />
        </div>

        <div className="filter-field">
          <span>STATUS</span>
          <Select
            value={local.status}
            onChange={(v) =>
              setLocal({ ...local, status: v as "" | UserStatus })
            }
            placeholder="TODOS"
            options={[
              { value: "", label: "TODOS" },
              { value: "active", label: STATUS_LABEL.active },
              { value: "suspended", label: STATUS_LABEL.suspended },
              { value: "deactivated", label: STATUS_LABEL.deactivated },
            ]}
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
