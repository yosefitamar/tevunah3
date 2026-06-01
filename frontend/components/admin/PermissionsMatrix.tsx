"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, Check, Loader2, RefreshCcw, Search } from "lucide-react";
import { listPermissions, updatePermission } from "@/lib/admin-api";
import {
  actionGroup,
  type ActionDef,
  type Permission,
  type RoleCode,
  type RoleRow,
} from "@/lib/types";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import Select from "../shared/Select";

type RowState = "idle" | "saving" | "saved" | "error";

type RowKey = string; // `${role_code}|${action}`

function keyOf(p: Permission): RowKey {
  return `${p.role_code}|${p.action}`;
}

export default function PermissionsMatrix() {
  const [items, setItems] = useState<Permission[]>([]);
  const [rolesList, setRolesList] = useState<RoleRow[]>([]);
  const [actionsMeta, setActionsMeta] = useState<Record<string, ActionDef>>({});
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
      setRolesList(res.roles ?? []);
      const meta: Record<string, ActionDef> = {};
      for (const a of res.actions ?? []) meta[a.code] = a;
      setActionsMeta(meta);
    } catch (e) {
      setError((e as Error).message || "Erro ao carregar matriz");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const labelOf = (action: string) => actionsMeta[action]?.label ?? action;
  const groupOf = (action: string) => actionsMeta[action]?.group ?? actionGroup(action);
  const descOf = (action: string) => actionsMeta[action]?.description ?? "";
  const roleLab = (code: string) =>
    rolesList.find((r) => r.code === code)?.label?.toUpperCase() ?? code.toUpperCase();

  const filtered = useMemo(() => {
    const s = search.trim().toLowerCase();
    const out = items.filter((p) => {
      if (roleFilter && p.role_code !== roleFilter) return false;
      if (!s) return true;
      return (
        p.action.toLowerCase().includes(s) ||
        (actionsMeta[p.action]?.label ?? "").toLowerCase().includes(s) ||
        (actionsMeta[p.action]?.group ?? actionGroup(p.action)).toLowerCase().includes(s) ||
        roleLab(p.role_code).toLowerCase().includes(s)
      );
    });
    if (!sort) return out;
    const dir = sort.dir === "asc" ? 1 : -1;
    const compare = (a: Permission, b: Permission): number => {
      switch (sort.field) {
        case "role":
          return roleLab(a.role_code).localeCompare(roleLab(b.role_code)) * dir;
        case "action":
          return a.action.localeCompare(b.action) * dir;
        case "group":
          return (actionsMeta[a.action]?.group ?? actionGroup(a.action)).localeCompare(
            actionsMeta[b.action]?.group ?? actionGroup(b.action),
          ) * dir;
        case "allowed":
          return (Number(a.allowed) - Number(b.allowed)) * dir;
        case "dual":
          return (Number(a.requires_dual_approval) - Number(b.requires_dual_approval)) * dir;
        case "approver":
          return (a.approver_role ?? "").localeCompare(b.approver_role ?? "") * dir;
        case "updated_at":
          return (a.updated_at ?? "").localeCompare(b.updated_at ?? "") * dir;
        default:
          return 0;
      }
    };
    // Sort estável: usa keyOf como desempate.
    return [...out].sort((a, b) => {
      const c = compare(a, b);
      return c !== 0 ? c : keyOf(a).localeCompare(keyOf(b));
    });
  }, [items, search, roleFilter, sort, actionsMeta, rolesList]);

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

        <Select
          value={roleFilter}
          onChange={(v) => setRoleFilter(v as RoleCode | "")}
          className="sel--toolbar"
          placeholder="PAPEL · TODOS"
          options={[
            { value: "", label: "PAPEL · TODOS" },
            ...rolesList.map((r) => ({
              value: r.code,
              label: `PAPEL · ${roleLab(r.code)}`,
            })),
          ]}
        />

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
                  return (
                    <tr key={k}>
                      <td style={{ color: "var(--fg-0)", fontWeight: 600 }}>
                        {roleLab(p.role_code)}
                      </td>
                      <td title={descOf(p.action)}>
                        <span style={{ color: "var(--fg-0)" }}>{labelOf(p.action)}</span>
                        <span className="mono muted" style={{ fontSize: 10, display: "block" }}>
                          {p.action}
                        </span>
                      </td>
                      <td className="muted">{groupOf(p.action)}</td>
                      <td style={{ textAlign: "center" }}>
                        <Toggle
                          on={p.allowed}
                          disabled={state === "saving"}
                          onChange={(v) => apply(p, { allowed: v })}
                        />
                      </td>
                      <td style={{ textAlign: "center" }}>
                        <Toggle
                          on={p.requires_dual_approval}
                          disabled={state === "saving" || !p.allowed}
                          onChange={(v) =>
                            apply(p, {
                              requires_dual_approval: v,
                              approver_role: v ? p.approver_role ?? "gestor" : null,
                            })
                          }
                        />
                      </td>
                      <td>
                        <Select
                          value={p.approver_role ?? ""}
                          disabled={
                            !p.requires_dual_approval ||
                            state === "saving"
                          }
                          onChange={(v) =>
                            apply(p, {
                              approver_role: (v || null) as RoleCode | null,
                            })
                          }
                          className="sel--row"
                          placeholder={
                            p.requires_dual_approval ? "SELECIONE…" : "—"
                          }
                          options={
                            p.requires_dual_approval
                              ? rolesList.map((r) => ({
                                  value: r.code,
                                  label: roleLab(r.code),
                                }))
                              : [{ value: "", label: "—" }]
                          }
                        />
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
      <span title={error ?? "Erro"} style={{ display: "inline-flex" }}>
        <AlertTriangle size={14} style={{ color: "var(--crit)" }} />
      </span>
    );
  }
  return null;
}
