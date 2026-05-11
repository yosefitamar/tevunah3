"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, Check, Loader2, Lock, RefreshCcw, Search } from "lucide-react";
import { listPermissions, updatePermission } from "@/lib/admin-api";
import {
  ROLE_LABEL,
  ROLES_LIST,
  actionGroup,
  type Permission,
  type RoleCode,
} from "@/lib/types";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";

type RowState = "idle" | "saving" | "saved" | "error";

type RowKey = string; // `${role_code}|${action}`

function keyOf(p: Permission): RowKey {
  return `${p.role_code}|${p.action}`;
}

export default function PermissionsMatrix() {
  const [items, setItems] = useState<Permission[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState<RoleCode | "">("");
  const [rowState, setRowState] = useState<Record<RowKey, RowState>>({});
  const [rowError, setRowError] = useState<Record<RowKey, string | null>>({});
  const [sort, setSort] = useState<SortState>({ field: "action", dir: "asc" });

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await listPermissions();
      setItems(res.items ?? []);
    } catch (e) {
      setError((e as Error).message || "Erro ao carregar matriz");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const filtered = useMemo(() => {
    const s = search.trim().toLowerCase();
    const out = items.filter((p) => {
      if (roleFilter && p.role_code !== roleFilter) return false;
      if (!s) return true;
      return (
        p.action.toLowerCase().includes(s) ||
        actionGroup(p.action).toLowerCase().includes(s) ||
        ROLE_LABEL[p.role_code].toLowerCase().includes(s)
      );
    });
    if (!sort) return out;
    const dir = sort.dir === "asc" ? 1 : -1;
    const compare = (a: Permission, b: Permission): number => {
      switch (sort.field) {
        case "role":
          return ROLE_LABEL[a.role_code].localeCompare(ROLE_LABEL[b.role_code]) * dir;
        case "action":
          return a.action.localeCompare(b.action) * dir;
        case "group":
          return actionGroup(a.action).localeCompare(actionGroup(b.action)) * dir;
        case "allowed":
          return (Number(a.allowed) - Number(b.allowed)) * dir;
        case "dual":
          return (Number(a.requires_dual_approval) - Number(b.requires_dual_approval)) * dir;
        case "approver":
          return (a.approver_role ?? "").localeCompare(b.approver_role ?? "") * dir;
        case "updated_at":
          return a.updated_at.localeCompare(b.updated_at) * dir;
        default:
          return 0;
      }
    };
    // Sort estável: usa keyOf como desempate.
    return [...out].sort((a, b) => {
      const c = compare(a, b);
      return c !== 0 ? c : keyOf(a).localeCompare(keyOf(b));
    });
  }, [items, search, roleFilter, sort]);

  async function apply(
    p: Permission,
    patch: {
      allowed?: boolean;
      requires_dual_approval?: boolean;
      approver_role?: RoleCode | null;
    },
  ) {
    const k = keyOf(p);

    // Aplica otimista no estado local.
    const optimistic: Permission = {
      ...p,
      ...(patch.allowed !== undefined ? { allowed: patch.allowed } : {}),
      ...(patch.requires_dual_approval !== undefined
        ? { requires_dual_approval: patch.requires_dual_approval }
        : {}),
      ...(patch.approver_role !== undefined
        ? { approver_role: patch.approver_role }
        : {}),
    };
    // Consistência local: 4-eyes off zera approver.
    if (optimistic.requires_dual_approval === false) {
      optimistic.approver_role = null;
    }

    setItems((cur) => cur.map((x) => (keyOf(x) === k ? optimistic : x)));
    setRowState((s) => ({ ...s, [k]: "saving" }));
    setRowError((s) => ({ ...s, [k]: null }));

    try {
      const res = await updatePermission(p.role_code, p.action, {
        allowed: optimistic.allowed,
        requires_dual_approval: optimistic.requires_dual_approval,
        approver_role: optimistic.approver_role,
      });
      // Atualiza com o que o servidor devolveu (updated_at correto).
      setItems((cur) =>
        cur.map((x) => (keyOf(x) === k ? res.permission : x)),
      );
      setRowState((s) => ({ ...s, [k]: "saved" }));
      // Volta pra idle depois de 1.2s
      window.setTimeout(() => {
        setRowState((s) => (s[k] === "saved" ? { ...s, [k]: "idle" } : s));
      }, 1200);
    } catch (e) {
      // Rollback
      setItems((cur) => cur.map((x) => (keyOf(x) === k ? p : x)));
      setRowState((s) => ({ ...s, [k]: "error" }));
      setRowError((s) => ({
        ...s,
        [k]: (e as ApiError).message || "Falha ao salvar",
      }));
    }
  }

  const total = filtered.length;

  return (
    <div className="screen-fill">
      <div className="toolbar">
        <div className="toolbar-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            placeholder="filtrar por ação, grupo ou papel…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>

        <select
          value={roleFilter}
          onChange={(e) => setRoleFilter(e.target.value as RoleCode | "")}
          className="toolbar-select"
          aria-label="Filtrar por papel"
        >
          <option value="">PAPEL · TODOS</option>
          {ROLES_LIST.map((r) => (
            <option key={r} value={r}>
              PAPEL · {ROLE_LABEL[r]}
            </option>
          ))}
        </select>

        <button
          type="button"
          className="btn"
          onClick={reload}
          disabled={loading}
          title="Recarregar"
        >
          <RefreshCcw size={14} strokeWidth={1.8} /> ATUALIZAR
        </button>
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="role" label="PAPEL" sort={sort} onChange={setSort} width={140} />
                <SortHeader field="action" label="AÇÃO" sort={sort} onChange={setSort} />
                <SortHeader field="group" label="GRUPO" sort={sort} onChange={setSort} width={160} />
                <SortHeader field="allowed" label="PERMITIDO" sort={sort} onChange={setSort} width={110} align="center" />
                <SortHeader field="dual" label="4-EYES" sort={sort} onChange={setSort} width={110} align="center" />
                <SortHeader field="approver" label="APROVADOR" sort={sort} onChange={setSort} width={170} />
                <SortHeader field="updated_at" label="ATUALIZADO" sort={sort} onChange={setSort} width={150} />
                <th style={{ width: 36 }}></th>
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
              {!loading && total === 0 && (
                <tr>
                  <td colSpan={8} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // NENHUMA LINHA CORRESPONDE AOS FILTROS
                  </td>
                </tr>
              )}
              {!loading &&
                filtered.map((p) => {
                  const k = keyOf(p);
                  const state = rowState[k] ?? "idle";
                  const err = rowError[k];
                  const locked = p.role_code === "administrador";
                  const lockTitle = locked
                    ? "PROTEGIDO · O papel administrador não pode perder permissões nem ganhar 4-eyes"
                    : undefined;
                  return (
                    <tr key={k} className={locked ? "row-locked" : undefined}>
                      <td style={{ color: "var(--fg-0)", fontWeight: 600 }}>
                        <span
                          style={{ display: "inline-flex", alignItems: "center", gap: 6 }}
                        >
                          {locked && (
                            <Lock
                              size={12}
                              strokeWidth={1.8}
                              style={{ color: "var(--fg-3)" }}
                              title={lockTitle}
                            />
                          )}
                          {ROLE_LABEL[p.role_code]}
                        </span>
                      </td>
                      <td className="mono" style={{ fontSize: 11 }}>
                        {p.action}
                      </td>
                      <td className="muted">{actionGroup(p.action)}</td>
                      <td style={{ textAlign: "center" }} title={lockTitle}>
                        <Toggle
                          on={p.allowed}
                          disabled={locked || state === "saving"}
                          onChange={(v) => apply(p, { allowed: v })}
                        />
                      </td>
                      <td style={{ textAlign: "center" }} title={lockTitle}>
                        <Toggle
                          on={p.requires_dual_approval}
                          disabled={locked || state === "saving" || !p.allowed}
                          onChange={(v) =>
                            apply(p, {
                              requires_dual_approval: v,
                              approver_role: v ? p.approver_role ?? "gestor" : null,
                            })
                          }
                        />
                      </td>
                      <td title={lockTitle}>
                        <select
                          value={p.approver_role ?? ""}
                          disabled={
                            locked ||
                            !p.requires_dual_approval ||
                            state === "saving"
                          }
                          onChange={(e) =>
                            apply(p, {
                              approver_role: (e.target.value || null) as
                                | RoleCode
                                | null,
                            })
                          }
                          className="row-select"
                        >
                          {!p.requires_dual_approval && (
                            <option value="">—</option>
                          )}
                          {p.requires_dual_approval && (
                            <>
                              {!p.approver_role && (
                                <option value="" disabled>
                                  SELECIONE…
                                </option>
                              )}
                              {ROLES_LIST.map((r) => (
                                <option key={r} value={r}>
                                  {ROLE_LABEL[r]}
                                </option>
                              ))}
                            </>
                          )}
                        </select>
                      </td>
                      <td className="muted">{formatBR(p.updated_at)}</td>
                      <td>
                        <RowStatus state={state} error={err} />
                      </td>
                    </tr>
                  );
                })}
            </tbody>
          </table>
        </div>

        <div className="pagination">
          <span className="muted">{total} linha{total === 1 ? "" : "s"}</span>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────── Subcomponents ────────────────────────────

function Toggle({
  on,
  disabled,
  onChange,
}: {
  on: boolean;
  disabled?: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <button
      type="button"
      className={
        "toggle" + (on ? " toggle--on" : "") + (disabled ? " toggle--disabled" : "")
      }
      disabled={disabled}
      onClick={() => !disabled && onChange(!on)}
      aria-pressed={on}
    >
      <span className="toggle-dot" />
    </button>
  );
}

function RowStatus({ state, error }: { state: RowState; error: string | null }) {
  if (state === "saving") {
    return <Loader2 size={14} className="spin" />;
  }
  if (state === "saved") {
    return <Check size={14} style={{ color: "var(--accent)" }} />;
  }
  if (state === "error") {
    return (
      <AlertTriangle
        size={14}
        style={{ color: "var(--crit)" }}
        title={error ?? "Erro"}
      />
    );
  }
  return null;
}
