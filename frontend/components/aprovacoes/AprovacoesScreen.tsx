"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, Clock, ShieldAlert, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  ACTION_LABEL,
  STATUS_LABEL,
  STATUS_PILL,
  type Approval,
  type ApprovalStatus,
  approveApproval,
  cancelApproval,
  listApprovals,
  rejectApproval,
} from "@/lib/approvals-api";
import { getUser } from "@/lib/users-api";
import { hasRole } from "@/lib/permissions";
import { ROLE_LABEL, clearanceLabel, type RoleCode, type User } from "@/lib/types";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import Select from "../shared/Select";

type TabId = "pending_for_me" | "mine" | "all";

const TABS: { id: TabId; label: string; mode: "" | "mine" | "pending_for_me" }[] = [
  { id: "pending_for_me", label: "PARA APROVAR", mode: "pending_for_me" },
  { id: "mine", label: "MINHAS", mode: "mine" },
  { id: "all", label: "TODAS", mode: "" },
];

export default function AprovacoesScreen() {
  const { user: me } = useAuth();
  const [tab, setTab] = useState<TabId>("pending_for_me");
  const [statusFilter, setStatusFilter] = useState<"" | ApprovalStatus>("");
  const [items, setItems] = useState<Approval[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [userCache, setUserCache] = useState<Record<string, User>>({});
  const [selected, setSelected] = useState<Approval | null>(null);
  const [reloadTick, setReloadTick] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "requested_at", dir: "desc" });

  const canSeeAll = hasRole(me, "administrador", "gestor");
  const visibleTabs = useMemo(
    () => (canSeeAll ? TABS : TABS.filter((t) => t.id !== "all")),
    [canSeeAll]
  );

  const reload = useCallback(async () => {
    const t = TABS.find((x) => x.id === tab)!;
    setLoading(true);
    setError(null);
    try {
      const res = await listApprovals({
        mode: t.mode,
        status: statusFilter || undefined,
        sort_by: (sort?.field as "requested_at" | "action" | "status" | "expires_at") || undefined,
        sort_dir: sort?.dir,
      });
      setItems(res.items);
      setTotal(res.total);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao listar");
    } finally {
      setLoading(false);
    }
  }, [tab, statusFilter, sort]);

  useEffect(() => {
    reload();
  }, [reload, reloadTick]);

  // Pre-resolve users referenced (requested_by + resource_id)
  useEffect(() => {
    const ids = new Set<string>();
    for (const a of items) {
      ids.add(a.requested_by);
      if (a.resource_type === "user" && a.resource_id) ids.add(a.resource_id);
      if (a.decided_by) ids.add(a.decided_by);
    }
    const missing = [...ids].filter((id) => !userCache[id]);
    if (missing.length === 0) return;
    let alive = true;
    (async () => {
      const next: Record<string, User> = {};
      await Promise.all(
        missing.map(async (id) => {
          try {
            const r = await getUser(id);
            next[id] = r.user;
          } catch {
            /* ignore */
          }
        })
      );
      if (alive && Object.keys(next).length) {
        setUserCache((c) => ({ ...c, ...next }));
      }
    })();
    return () => {
      alive = false;
    };
  }, [items, userCache]);

  function userLabel(id: string | undefined): string {
    if (!id) return "—";
    const u = userCache[id];
    if (!u) return id.slice(0, 8) + "…";
    return `${u.code} · ${u.display_name.toUpperCase()}`;
  }

  return (
    <div className="screen-fill">
      <div className="section-title">
        APROVAÇÕES · {total} REGISTRO{total === 1 ? "" : "S"}
        <span style={{ color: "var(--fg-2)" }}>· FLUXO 4-EYES</span>
      </div>

      <div className="toolbar">
        <div className="tabs">
          {visibleTabs.map((t) => (
            <button
              key={t.id}
              type="button"
              className={"tab" + (tab === t.id ? " tab-active" : "")}
              onClick={() => setTab(t.id)}
            >
              {t.label}
            </button>
          ))}
        </div>

        <div style={{ marginLeft: "auto" }} />

        <label className="filter-field" style={{ minWidth: 180 }}>
          <span>STATUS</span>
          <Select
            value={statusFilter}
            onChange={(v) => setStatusFilter(v as "" | ApprovalStatus)}
            placeholder="TODOS"
            options={[
              { value: "", label: "TODOS" },
              { value: "pending", label: STATUS_LABEL.pending },
              { value: "approved", label: STATUS_LABEL.approved },
              { value: "rejected", label: STATUS_LABEL.rejected },
              { value: "expired", label: STATUS_LABEL.expired },
              { value: "cancelled", label: STATUS_LABEL.cancelled },
            ]}
          />
        </label>
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="requested_at" label="SOLICITADO" sort={sort} onChange={setSort} width={130} />
                <SortHeader field="action" label="AÇÃO" sort={sort} onChange={setSort} width={180} />
                <th>ALVO</th>
                <th>DETALHES</th>
                <th style={{ width: 220 }}>SOLICITANTE</th>
                <SortHeader field="status" label="STATUS" sort={sort} onChange={setSort} width={120} />
                <SortHeader field="expires_at" label="EXPIRA" sort={sort} onChange={setSort} width={130} />
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
                    // NENHUMA APROVAÇÃO
                  </td>
                </tr>
              )}
              {!loading &&
                items.map((a) => (
                  <tr
                    key={a.id}
                    className="row-clickable"
                    onClick={() => setSelected(a)}
                  >
                    <td className="muted">{formatBR(a.requested_at)}</td>
                    <td style={{ color: "var(--fg-0)", fontWeight: 600 }}>
                      {ACTION_LABEL[a.action] ?? a.action.toUpperCase()}
                    </td>
                    <td>
                      {a.resource_type === "user"
                        ? userLabel(a.resource_id)
                        : a.resource_id ?? "—"}
                    </td>
                    <td className="muted">{summarizePayload(a)}</td>
                    <td>{userLabel(a.requested_by)}</td>
                    <td>
                      <span className={"pill " + STATUS_PILL[a.status]}>
                        {STATUS_LABEL[a.status]}
                      </span>
                    </td>
                    <td className="muted">{formatBR(a.expires_at)}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </div>

      {selected && (
        <ApprovalDrawer
          approval={selected}
          userLabel={userLabel}
          onClose={() => setSelected(null)}
          onChanged={() => {
            setSelected(null);
            setReloadTick((t) => t + 1);
          }}
        />
      )}
    </div>
  );
}

function summarizePayload(a: Approval): string {
  const p = (a.payload ?? {}) as Record<string, unknown>;
  if (a.action === "user.role.assign" && Array.isArray(p.roles)) {
    return (p.roles as string[])
      .map((r) => ROLE_LABEL[r as RoleCode] ?? r.toUpperCase())
      .join(" · ");
  }
  if (a.action === "user.clearance.set" && typeof p.clearance_level === "number") {
    return clearanceLabel(p.clearance_level as number);
  }
  return "—";
}

// ─────────────────────────── ApprovalDrawer ────────────────────────────

type DrawerProps = {
  approval: Approval;
  userLabel: (id: string | undefined) => string;
  onClose: () => void;
  onChanged: () => void;
};

function ApprovalDrawer({ approval, userLabel, onClose, onChanged }: DrawerProps) {
  const { user: me } = useAuth();
  const [reason, setReason] = useState("");
  const [acting, setActing] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const isPending = approval.status === "pending";
  const isRequester = me?.id === approval.requested_by;
  const canDecide =
    isPending &&
    !isRequester &&
    !!me &&
    me.roles.includes(approval.required_approver_role as RoleCode);

  async function run(fn: () => Promise<unknown>) {
    setErr(null);
    setActing(true);
    try {
      await fn();
      onChanged();
    } catch (e) {
      setErr((e as ApiError).message || "Falha");
    } finally {
      setActing(false);
    }
  }

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <aside
        className="drawer"
        onClick={(e) => e.stopPropagation()}
        aria-label="Detalhe da aprovação"
      >
        <div className="drawer-hd">
          <span>APROVAÇÃO · {ACTION_LABEL[approval.action] ?? approval.action.toUpperCase()}</span>
          <button
            type="button"
            className="action-btn"
            onClick={onClose}
            aria-label="Fechar"
          >
            <X size={14} />
          </button>
        </div>

        <div className="drawer-bd">
          <div className="dossier-head">
            <div className="dossier-code">
              <span className={"pill " + STATUS_PILL[approval.status]}>
                {STATUS_LABEL[approval.status]}
              </span>
            </div>
            <div className="dossier-name">
              {ACTION_LABEL[approval.action] ?? approval.action.toUpperCase()}
            </div>
            <div className="dossier-meta">
              <span>{summarizePayload(approval)}</span>
            </div>
          </div>

          <dl className="dossier-list">
            <div>
              <dt>ALVO</dt>
              <dd>
                {approval.resource_type === "user"
                  ? userLabel(approval.resource_id)
                  : approval.resource_id ?? "—"}
              </dd>
            </div>
            <div>
              <dt>SOLICITANTE</dt>
              <dd>{userLabel(approval.requested_by)}</dd>
            </div>
            <div>
              <dt>SOLICITADO EM</dt>
              <dd>{formatBR(approval.requested_at)}</dd>
            </div>
            <div>
              <dt>APROVADOR EXIGIDO</dt>
              <dd>
                {ROLE_LABEL[approval.required_approver_role as RoleCode] ??
                  approval.required_approver_role.toUpperCase()}
              </dd>
            </div>
            <div>
              <dt>EXPIRA EM</dt>
              <dd>
                <Clock size={11} style={{ display: "inline", marginRight: 4 }} />
                {formatBR(approval.expires_at)}
              </dd>
            </div>
            {approval.decided_by && (
              <div>
                <dt>DECIDIDO POR</dt>
                <dd>
                  {userLabel(approval.decided_by)} · {formatBR(approval.decided_at)}
                </dd>
              </div>
            )}
            {approval.decision_reason && (
              <div>
                <dt>MOTIVO DA DECISÃO</dt>
                <dd>{approval.decision_reason}</dd>
              </div>
            )}
          </dl>

          {isPending && (canDecide || isRequester) && (
            <div className="drawer-section">
              <div className="drawer-section-title">
                {canDecide ? "DECISÃO" : "CANCELAMENTO"}
              </div>
              <label className="form-field">
                <span>MOTIVO (OPCIONAL)</span>
                <input
                  type="text"
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder="justificativa registrada no audit log…"
                />
              </label>
              {err && <div className="banner banner-error">⚠ {err}</div>}
              <div className="confirm-actions">
                {canDecide && (
                  <>
                    <button
                      type="button"
                      className="btn btn-danger"
                      disabled={acting}
                      onClick={() =>
                        run(() => rejectApproval(approval.id, reason.trim() || undefined))
                      }
                    >
                      <X size={14} />
                      {acting ? "PROCESSANDO…" : "REJEITAR"}
                    </button>
                    <button
                      type="button"
                      className="btn btn-primary"
                      disabled={acting}
                      onClick={() =>
                        run(() => approveApproval(approval.id, reason.trim() || undefined))
                      }
                    >
                      <Check size={14} />
                      {acting ? "PROCESSANDO…" : "APROVAR"}
                    </button>
                  </>
                )}
                {!canDecide && isRequester && (
                  <button
                    type="button"
                    className="btn btn-ghost"
                    disabled={acting}
                    onClick={() =>
                      run(() => cancelApproval(approval.id, reason.trim() || undefined))
                    }
                  >
                    {acting ? "PROCESSANDO…" : "CANCELAR SOLICITAÇÃO"}
                  </button>
                )}
              </div>
            </div>
          )}

          {isPending && !canDecide && !isRequester && (
            <div className="drawer-section">
              <div className="muted" style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <ShieldAlert size={14} />
                Você não tem o papel exigido para decidir esta aprovação.
              </div>
            </div>
          )}
        </div>
      </aside>
    </div>
  );
}
