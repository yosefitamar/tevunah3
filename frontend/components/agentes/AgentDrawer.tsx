"use client";

import { useEffect, useState } from "react";
import { Ban, KeyRound, Pencil, Shield, ShieldOff, UserX, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  ROLE_LABEL,
  ROLES_LIST,
  STATUS_LABEL,
  STATUS_PILL,
  clearanceLabel,
  type RoleCode,
  type User,
  type UserStatus,
} from "@/lib/types";
import { formatBR } from "@/lib/format";
import { deactivateUser, getUser } from "@/lib/users-api";
import {
  canCreateUsers,
  canRequestClearanceChange,
  canRequestRoleChange,
} from "@/lib/permissions";
import {
  requestSetClearance,
  requestSetRoles,
  type RoleAssignResponse,
} from "@/lib/approvals-api";
import type { ApiError } from "@/lib/api";

type Props = {
  userId: string;
  onClose: () => void;
  onChanged: () => void;
};

type ActionMode = "deactivate" | "set_roles" | "set_clearance";

export default function AgentDrawer({ userId, onClose, onChanged }: Props) {
  const { user: me } = useAuth();
  const [data, setData] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<ActionMode | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(null);
    getUser(userId)
      .then((d) => {
        if (alive) setData(d.user);
      })
      .catch((e: ApiError) => {
        if (alive) setError(e.message || "Erro ao carregar");
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [userId]);

  const isSelf = me?.id === userId;
  const isAdmin = canCreateUsers(me);
  const canRole = canRequestRoleChange(me);
  const canClr = canRequestClearanceChange(me);
  const targetActive = data?.status === "active";

  function closeAction() {
    setMode(null);
  }

  function afterMutation(msg?: string) {
    setMode(null);
    if (msg) setNotice(msg);
    onChanged();
  }

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <aside
        className="drawer"
        onClick={(e) => e.stopPropagation()}
        aria-label="Dossiê do agente"
      >
        <div className="drawer-hd">
          <span>DOSSIÊ DO AGENTE</span>
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
          {loading && <div className="muted">// CARREGANDO…</div>}
          {error && <div className="banner banner-error">⚠ {error}</div>}

          {data && (
            <>
              <div className="dossier-head">
                <div className="dossier-code">{data.code}</div>
                <div className="dossier-name">{data.display_name.toUpperCase()}</div>
                <div className="dossier-email">{data.email}</div>
                <div className="dossier-meta">
                  <span className={"pill " + STATUS_PILL[data.status as UserStatus]}>
                    {STATUS_LABEL[data.status as UserStatus]}
                  </span>
                  <span>{clearanceLabel(data.clearance_level)}</span>
                  <span>·</span>
                  <span>
                    {data.roles
                      .map((r) => ROLE_LABEL[r as RoleCode] ?? r.toUpperCase())
                      .join(" · ")}
                  </span>
                </div>
              </div>

              <dl className="dossier-list">
                <div>
                  <dt>ÚLTIMO LOGIN</dt>
                  <dd>{formatBR(data.last_login_at)}</dd>
                </div>
              </dl>

              {notice && (
                <div className="banner banner-info">✓ {notice}</div>
              )}

              <div className="drawer-section">
                <div className="drawer-section-title">AÇÕES</div>

                {mode === "deactivate" && (
                  <DeactivateBlock
                    user={data}
                    onCancel={closeAction}
                    onDone={() => {
                      onChanged();
                      onClose();
                    }}
                  />
                )}

                {mode === "set_roles" && (
                  <SetRolesBlock
                    user={data}
                    onCancel={closeAction}
                    onDone={(direct) =>
                      afterMutation(
                        direct
                          ? "PAPÉIS ATUALIZADOS."
                          : "SOLICITAÇÃO ENVIADA · AGUARDANDO APROVAÇÃO."
                      )
                    }
                  />
                )}

                {mode === "set_clearance" && (
                  <SetClearanceBlock
                    user={data}
                    onCancel={closeAction}
                    onDone={(direct) =>
                      afterMutation(
                        direct
                          ? "CLEARANCE ATUALIZADO."
                          : "SOLICITAÇÃO ENVIADA · AGUARDANDO APROVAÇÃO."
                      )
                    }
                  />
                )}

                {mode === null && (
                  <div className="action-list">
                    <ActionRow
                      icon={<Pencil size={14} />}
                      label="EDITAR PERFIL"
                      hint="EM BREVE · APENAS PRÓPRIO"
                      disabled
                    />
                    <ActionRow
                      icon={<Shield size={14} />}
                      label="ALTERAR PAPEL"
                      hint={
                        !canRole
                          ? "REQUER ADMINISTRADOR"
                          : isSelf
                            ? "NÃO É POSSÍVEL ALTERAR O PRÓPRIO PAPEL"
                            : !targetActive
                              ? "AGENTE DESATIVADO"
                              : "ADMIN · EFEITO IMEDIATO"
                      }
                      disabled={!canRole || isSelf || !targetActive}
                      onClick={() => setMode("set_roles")}
                    />
                    <ActionRow
                      icon={<ShieldOff size={14} />}
                      label="ALTERAR CLEARANCE"
                      hint={
                        !canClr
                          ? "REQUER ADMINISTRADOR"
                          : isSelf
                            ? "NÃO É POSSÍVEL ALTERAR O PRÓPRIO CLEARANCE"
                            : !targetActive
                              ? "AGENTE DESATIVADO"
                              : "ADMIN · EFEITO IMEDIATO"
                      }
                      disabled={!canClr || isSelf || !targetActive}
                      onClick={() => setMode("set_clearance")}
                    />
                    <ActionRow
                      icon={<KeyRound size={14} />}
                      label="RESETAR SENHA"
                      hint="EM BREVE"
                      disabled
                    />
                    <ActionRow
                      icon={<KeyRound size={14} />}
                      label="RESETAR TOTP"
                      hint="EM BREVE · 4-EYES · APROVAÇÃO DO GESTOR"
                      disabled
                    />
                    <ActionRow
                      icon={<Ban size={14} />}
                      label="DESATIVAR AGENTE"
                      hint={
                        !isAdmin
                          ? "REQUER ADMINISTRADOR"
                          : isSelf
                            ? "NÃO É POSSÍVEL DESATIVAR O PRÓPRIO USUÁRIO"
                            : !targetActive
                              ? "AGENTE JÁ DESATIVADO"
                              : "ADMIN · EFEITO IMEDIATO"
                      }
                      destructive
                      disabled={!isAdmin || isSelf || !targetActive}
                      onClick={() => setMode("deactivate")}
                    />
                  </div>
                )}
              </div>
            </>
          )}
        </div>
      </aside>
    </div>
  );
}

// ─────────────────────────── DeactivateBlock ────────────────────────────

function DeactivateBlock({
  user,
  onCancel,
  onDone,
}: {
  user: User;
  onCancel: () => void;
  onDone: () => void;
}) {
  const [reason, setReason] = useState("");
  const [acting, setActing] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function doIt() {
    setErr(null);
    setActing(true);
    try {
      await deactivateUser(user.id, reason.trim() || undefined);
      onDone();
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao desativar");
    } finally {
      setActing(false);
    }
  }

  return (
    <div className="confirm-block">
      <div className="confirm-msg">
        ⚠ Confirme a desativação de <b>{user.code} · {user.display_name.toUpperCase()}</b>.
        A ação é registrada no audit log e revoga todas as sessões ativas do agente.
      </div>
      <label className="form-field">
        <span>MOTIVO (OPCIONAL)</span>
        <input
          type="text"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder="ex.: desligamento, transferência, suspeita…"
        />
      </label>
      {err && <div className="banner banner-error">⚠ {err}</div>}
      <div className="confirm-actions">
        <button
          type="button"
          className="btn btn-ghost"
          onClick={onCancel}
          disabled={acting}
        >
          CANCELAR
        </button>
        <button
          type="button"
          className="btn btn-danger"
          onClick={doIt}
          disabled={acting}
        >
          <UserX size={14} />
          {acting ? "DESATIVANDO…" : "CONFIRMAR DESATIVAÇÃO"}
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────── SetRolesBlock ────────────────────────────

function SetRolesBlock({
  user,
  onCancel,
  onDone,
}: {
  user: User;
  onCancel: () => void;
  onDone: (direct: boolean) => void;
}) {
  const [roles, setRoles] = useState<RoleCode[]>([...user.roles]);
  const [acting, setActing] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  function toggle(r: RoleCode) {
    setRoles((cur) =>
      cur.includes(r) ? cur.filter((x) => x !== r) : [...cur, r]
    );
  }

  async function submit() {
    if (roles.length === 0) {
      setErr("Selecione ao menos um papel.");
      return;
    }
    setErr(null);
    setActing(true);
    try {
      const res: RoleAssignResponse = await requestSetRoles(user.id, roles);
      onDone(!res.approval);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao solicitar");
    } finally {
      setActing(false);
    }
  }

  const changed =
    roles.length !== user.roles.length ||
    roles.some((r) => !user.roles.includes(r));

  return (
    <div className="confirm-block">
      <div className="confirm-msg">
        Alterar papéis de <b>{user.code} · {user.display_name.toUpperCase()}</b>.
        A ação é registrada no audit log e tem efeito imediato.
      </div>
      <div className="form-field">
        <span>PAPÉIS</span>
        <div className="role-checks">
          {ROLES_LIST.map((r) => (
            <label key={r} className="role-check">
              <input
                type="checkbox"
                checked={roles.includes(r)}
                onChange={() => toggle(r)}
              />
              <span>{ROLE_LABEL[r]}</span>
            </label>
          ))}
        </div>
      </div>
      {err && <div className="banner banner-error">⚠ {err}</div>}
      <div className="confirm-actions">
        <button
          type="button"
          className="btn btn-ghost"
          onClick={onCancel}
          disabled={acting}
        >
          CANCELAR
        </button>
        <button
          type="button"
          className="btn btn-primary"
          onClick={submit}
          disabled={acting || !changed}
        >
          <Shield size={14} />
          {acting ? "ENVIANDO…" : "SOLICITAR ALTERAÇÃO"}
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────── SetClearanceBlock ────────────────────────────

function SetClearanceBlock({
  user,
  onCancel,
  onDone,
}: {
  user: User;
  onCancel: () => void;
  onDone: (direct: boolean) => void;
}) {
  const [level, setLevel] = useState<number>(user.clearance_level);
  const [acting, setActing] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit() {
    if (level < 1 || level > 5) {
      setErr("Clearance deve estar entre 1 e 5.");
      return;
    }
    setErr(null);
    setActing(true);
    try {
      const res: RoleAssignResponse = await requestSetClearance(user.id, level);
      onDone(!res.approval);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao solicitar");
    } finally {
      setActing(false);
    }
  }

  return (
    <div className="confirm-block">
      <div className="confirm-msg">
        Alterar clearance de <b>{user.code} · {user.display_name.toUpperCase()}</b>.
        Atual: <b>{clearanceLabel(user.clearance_level)}</b>. A ação é registrada
        no audit log e tem efeito imediato.
      </div>
      <label className="form-field">
        <span>NOVO CLEARANCE</span>
        <select value={level} onChange={(e) => setLevel(Number(e.target.value))}>
          {[1, 2, 3, 4, 5].map((n) => (
            <option key={n} value={n}>
              {clearanceLabel(n)}
            </option>
          ))}
        </select>
      </label>
      {err && <div className="banner banner-error">⚠ {err}</div>}
      <div className="confirm-actions">
        <button
          type="button"
          className="btn btn-ghost"
          onClick={onCancel}
          disabled={acting}
        >
          CANCELAR
        </button>
        <button
          type="button"
          className="btn btn-primary"
          onClick={submit}
          disabled={acting || level === user.clearance_level}
        >
          <ShieldOff size={14} />
          {acting ? "ENVIANDO…" : "SOLICITAR ALTERAÇÃO"}
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────── ActionRow ────────────────────────────

type ActionRowProps = {
  icon: React.ReactNode;
  label: string;
  hint: string;
  destructive?: boolean;
  disabled?: boolean;
  onClick?: () => void;
};

function ActionRow({ icon, label, hint, destructive, disabled, onClick }: ActionRowProps) {
  return (
    <button
      type="button"
      className={
        "action-row" +
        (destructive ? " action-row--danger" : "") +
        (disabled ? " action-row--disabled" : "")
      }
      onClick={onClick}
      disabled={disabled}
    >
      <span className="action-row-icon">{icon}</span>
      <span className="action-row-label">{label}</span>
      <span className="action-row-hint">{hint}</span>
    </button>
  );
}
