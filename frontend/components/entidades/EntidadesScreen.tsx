"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Filter, Plus, RotateCcw, Search, ShieldAlert, Trash2 } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useModal } from "@/contexts/ModalContext";
import { listEntities, restoreEntity, type EntitiesList } from "@/lib/entities-api";
import {
  canListEntities,
  canCreateEntities,
  canRestoreEntities,
} from "@/lib/permissions";
import {
  ENTITY_KIND_LABEL,
  ENTITY_KINDS,
  isVehicle,
  vehiclePrimaryLabel,
  type EntityKind,
} from "@/lib/entities-types";
import { formatBR } from "@/lib/format";
import SortHeader, { type SortState } from "../shared/SortHeader";
import CreateEntidadeModal from "./CreateEntidadeModal";
import EntidadeDrawer from "./EntidadeDrawer";

const PAGE_SIZE = 25;

type Filters = {
  kind: EntityKind | "";
  tag: string;
};

const EMPTY_FILTERS: Filters = { kind: "", tag: "" };

function activeFilterCount(f: Filters) {
  let n = 0;
  if (f.kind) n++;
  if (f.tag.trim()) n++;
  return n;
}

export default function EntidadesScreen() {
  const { user: me } = useAuth();
  const modal = useModal();
  const [data, setData] = useState<EntitiesList | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "name", dir: "asc" });
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [trashMode, setTrashMode] = useState(false);
  const [restoringId, setRestoringId] = useState<string | null>(null);

  const reload = useCallback(
    async (q: string, f: Filters, p: number, s: SortState, trash: boolean) => {
      setLoading(true);
      setError(null);
      try {
        const res = await listEntities({
          limit: PAGE_SIZE,
          offset: p * PAGE_SIZE,
          search: q || undefined,
          kind: f.kind || undefined,
          tag: f.tag.trim() || undefined,
          sort_by:
            (s?.field as "name" | "kind" | "created_at" | "updated_at") ||
            undefined,
          sort_dir: s?.dir,
          deleted: trash ? "only" : undefined,
        });
        setData(res);
      } catch (e) {
        setError((e as Error).message || "Erro ao listar entidades");
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    if (canListEntities(me)) reload(search, filters, page, sort, trashMode);
  }, [me, page, search, filters, sort, trashMode, reload]);

  async function handleRestore(id: string, name: string) {
    const ok = await modal.confirm({
      variant: "info",
      title: "RESTAURAR ENTIDADE",
      message: (
        <>
          Confirmar restauração de <strong>{name.toUpperCase()}</strong>? O
          registro voltará a aparecer no cadastro ativo.
        </>
      ),
      confirm: "RESTAURAR",
      cancel: "CANCELAR",
    });
    if (!ok) return;
    setRestoringId(id);
    try {
      await restoreEntity(id);
      await reload(search, filters, page, sort, trashMode);
    } catch (e) {
      await modal.alert({
        variant: "error",
        title: "FALHA AO RESTAURAR",
        message: (e as Error).message || "Erro desconhecido",
      });
    } finally {
      setRestoringId(null);
    }
  }

  function changeSort(next: SortState) {
    setPage(0);
    setSort(next);
  }

  if (!canListEntities(me)) {
    return (
      <div className="placeholder">
        <div className="ph-tag">// MOD-02 / ENTIDADES</div>
        <div className="ph-ttl" style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <ShieldAlert size={22} /> ACESSO RESTRITO
        </div>
        <div className="ph-sub">
          Seu papel não permite visualizar o cadastro de entidades.
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
        {trashMode ? "LIXEIRA" : "ENTIDADES"} · {total} REGISTRO{total === 1 ? "" : "S"}
        <span style={{ color: "var(--fg-2)" }}>
          · {trashMode ? "REGISTROS EXCLUÍDOS" : "CADASTRO POLIMÓRFICO"}
        </span>
      </div>

      <div className="toolbar">
        <form onSubmit={onSearchSubmit} className="toolbar-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
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

        {canRestoreEntities(me) && (
          <button
            type="button"
            className={"btn" + (trashMode ? " btn-active" : "")}
            onClick={() => {
              setPage(0);
              setSelectedId(null);
              setTrashMode((v) => !v);
            }}
            title={trashMode ? "VOLTAR PARA ATIVOS" : "VER REGISTROS EXCLUÍDOS"}
          >
            <Trash2 size={14} strokeWidth={1.8} /> LIXEIRA
          </button>
        )}

        {!trashMode && canCreateEntities(me) && (
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => setCreateOpen(true)}
          >
            <Plus size={14} strokeWidth={2} /> NOVA ENTIDADE
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
        />
      )}

      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="kind" label="TIPO" sort={sort} onChange={changeSort} width={130} />
                <SortHeader field="name" label="NOME" sort={sort} onChange={changeSort} />
                <th>TAGS</th>
                <SortHeader
                  field="updated_at"
                  label={trashMode ? "EXCLUÍDO EM" : "ATUALIZADO"}
                  sort={sort}
                  onChange={changeSort}
                  width={150}
                />
                {trashMode && <th style={{ width: 140 }}></th>}
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={trashMode ? 5 : 4} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // CARREGANDO…
                  </td>
                </tr>
              )}
              {!loading && data?.items.length === 0 && (
                <tr>
                  <td colSpan={trashMode ? 5 : 4} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // {trashMode ? "LIXEIRA VAZIA" : "NENHUM REGISTRO ENCONTRADO"}
                  </td>
                </tr>
              )}
              {!loading &&
                data?.items.map((e) => (
                  <tr
                    key={e.id}
                    onClick={trashMode ? undefined : () => setSelectedId(e.id)}
                    className={trashMode ? undefined : "row-clickable"}
                  >
                    <td className="muted">{ENTITY_KIND_LABEL[e.kind]}</td>
                    <td style={{ color: "var(--fg-0)", fontWeight: 600 }}>
                      {isVehicle(e)
                        ? vehiclePrimaryLabel(e.name, e.attrs?.plate).toUpperCase()
                        : e.name.toUpperCase()}
                    </td>
                    <td>
                      {e.tags.length === 0 ? (
                        <span className="muted">—</span>
                      ) : (
                        <div className="tag-row">
                          {e.tags.map((t) => (
                            <button
                              key={t}
                              type="button"
                              className={
                                "tag-chip tag-chip--button" +
                                (filters.tag === t ? " tag-chip--active" : "")
                              }
                              onClick={(ev) => {
                                ev.stopPropagation();
                                setPage(0);
                                setFilters((f) =>
                                  f.tag === t ? { ...f, tag: "" } : { ...f, tag: t },
                                );
                              }}
                              title={
                                filters.tag === t
                                  ? `Remover filtro #${t}`
                                  : `Filtrar por #${t}`
                              }
                            >
                              #{t}
                            </button>
                          ))}
                        </div>
                      )}
                    </td>
                    <td className="muted">
                      {formatBR(trashMode && e.deleted_at ? e.deleted_at : e.updated_at)}
                    </td>
                    {trashMode && (
                      <td style={{ textAlign: "right" }}>
                        <button
                          type="button"
                          className="btn btn-ghost"
                          disabled={restoringId === e.id}
                          onClick={(ev) => {
                            ev.stopPropagation();
                            void handleRestore(e.id, e.name);
                          }}
                          title="RESTAURAR"
                        >
                          <RotateCcw size={13} strokeWidth={1.8} />{" "}
                          {restoringId === e.id ? "RESTAURANDO…" : "RESTAURAR"}
                        </button>
                      </td>
                    )}
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
        <CreateEntidadeModal
          onClose={() => setCreateOpen(false)}
          onCreated={(id) => {
            setCreateOpen(false);
            setSelectedId(id);
            reload(search, filters, page, sort, trashMode);
          }}
        />
      )}

      {selectedId && (
        <EntidadeDrawer
          entityId={selectedId}
          onClose={() => setSelectedId(null)}
          onChanged={() => reload(search, filters, page, sort, trashMode)}
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
        <label className="filter-field">
          <span>TIPO</span>
          <select
            value={local.kind}
            onChange={(e) => setLocal({ ...local, kind: e.target.value as EntityKind | "" })}
          >
            <option value="">TODOS</option>
            {ENTITY_KINDS.map((k) => (
              <option key={k} value={k}>
                {ENTITY_KIND_LABEL[k]}
              </option>
            ))}
          </select>
        </label>

        <label className="filter-field">
          <span>TAG</span>
          <input
            type="text"
            value={local.tag}
            onChange={(e) => setLocal({ ...local, tag: e.target.value })}
          />
        </label>
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
