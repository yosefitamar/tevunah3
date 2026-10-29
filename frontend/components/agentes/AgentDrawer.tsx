"use client";

import { useEffect, useState } from "react";
import { Ban, BadgeCheck, KeyRound, Pencil, Shield, ShieldOff, UserX, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  STATUS_LABEL,
  STATUS_PILL,
  clearanceLabel,
  type RoleCode,
  type User,
  type UserStatus,
} from "@/lib/types";
import { formatBR } from "@/lib/format";
import {
  changeOwnPassword,
  deactivateUser,
  getUser,
  listSipomPostos,
  resetUserPassword,
  resetUserTOTP,
  updateUserProfile,
  updateUserService,
} from "@/lib/users-api";
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
import Select from "../shared/Select";

type Props = {
  userId: string;
  onClose: () => void;
  onChanged: () => void;
};

type ActionMode =
  | "deactivate"
  | "set_roles"
  | "set_clearance"
  | "edit_profile"
  | "edit_service"
  | "reset_password"
  | "change_password"
  | "reset_totp";

export default function AgentDrawer({ userId, onClose, onChanged }: Props) {
  const { user: me, roleLabel } = useAuth();
  const [data, setData] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<ActionMode | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [postos, setPostos] = useState<{ id: number; nome: string }[]>([]);

  // Posto/graduação vem do catálogo do SIPOM; sem ele, o cadastro mostra só
  // o que está gravado.
  useEffect(() => {
    listSipomPostos()
      .then((r) => setPostos(r.items))
      .catch(() => setPostos([]));
  }, []);

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
                    {data.roles.map((r) => roleLabel(r)).join(" · ")}
                  </span>
                </div>
              </div>

              <dl className="dossier-list">
                <div>
                  <dt>ÚLTIMO LOGIN</dt>
                  <dd>{formatBR(data.last_login_at)}</dd>
                </div>
                <div>
                  <dt>MATRÍCULA</dt>
                  <dd className="mono">{data.registration || "—"}</dd>
                </div>
                <div>
                  <dt>POSTO/GRAD.</dt>
                  <dd>{postos.find((p) => p.id === data.rank_id)?.nome ?? "—"}</dd>
                </div>
                <div>
                  <dt>NUMERAL</dt>
                  <dd className="mono">{data.number || "—"}</dd>
                </div>
                <div>
                  <dt>NOME DE GUERRA</dt>
                  <dd>{data.war_name || "—"}</dd>
                </div>
                <div>
                  <dt>COMPOSIÇÃO DO SIPOM</dt>
                  <dd>
                    {data.sipom_composition ? (
                      <span className="pill active">ENTRA</span>
                    ) : (
                      <span className="pill cold">NÃO ENTRA</span>
                    )}
                  </dd>
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

                {mode === "edit_profile" && (
                  <EditProfileBlock
                    user={data}
                    onCancel={closeAction}
                    onDone={() => afterMutation("PERFIL ATUALIZADO.")}
                  />
                )}

                {mode === "edit_service" && (
                  <EditServiceBlock
                    user={data}
                    postos={postos}
                    onCancel={closeAction}
                    onDone={(u) => {
                      setData(u);
                      afterMutation("DADOS FUNCIONAIS ATUALIZADOS.");
                    }}
                  />
                )}

                {mode === "reset_password" && (
                  <ResetPasswordBlock
                    user={data}
                    onCancel={closeAction}
                    onDone={() => afterMutation("SENHA RESETADA · TEMPORÁRIA EMITIDA.")}
                  />
                )}

                {mode === "change_password" && (
                  <ChangePasswordBlock
                    onCancel={closeAction}
                    onDone={() => afterMutation("SENHA ALTERADA.")}
                  />
                )}

                {mode === "reset_totp" && (
                  <ResetTOTPBlock
                    user={data}
                    onCancel={closeAction}
                    onDone={() => afterMutation("TOTP RESETADO · O AGENTE RECEBERÁ O NOVO QR NO PRÓXIMO LOGIN.")}
                  />
                )}

                {mode === null && (
                  <div className="action-list">
                    <ActionRow
                      icon={<Pencil size={14} />}
                      label="EDITAR PERFIL"
                      hint={
                        isSelf
                          ? "PRÓPRIO · NOME DE EXIBIÇÃO + E-MAIL"
                          : !isAdmin
                            ? "REQUER ADMINISTRADOR"
                            : !targetActive
                              ? "AGENTE DESATIVADO"
                              : "ADMIN · NOME + E-MAIL"
                      }
                      disabled={!isSelf && (!isAdmin || !targetActive)}
                      onClick={() => setMode("edit_profile")}
                    />
                    <ActionRow
                      icon={<BadgeCheck size={14} />}
                      label="DADOS FUNCIONAIS"
                      hint={
                        !isAdmin
                          ? "REQUER ADMINISTRADOR"
                          : !targetActive
                            ? "AGENTE DESATIVADO"
                            : "ADMIN · MATRÍCULA, POSTO, NUMERAL, NOME DE GUERRA E COMPOSIÇÃO DO SIPOM"
                      }
                      disabled={!isAdmin || !targetActive}
                      onClick={() => setMode("edit_service")}
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
                    {isSelf ? (
                      <ActionRow
                        icon={<KeyRound size={14} />}
                        label="TROCAR SENHA"
                        hint="PRÓPRIO · INFORMAR SENHA ATUAL"
                        onClick={() => setMode("change_password")}
                      />
                    ) : (
                      <ActionRow
                        icon={<KeyRound size={14} />}
                        label="RESETAR SENHA"
                        hint={
                          !isAdmin
                            ? "REQUER ADMINISTRADOR"
                            : !targetActive
                              ? "AGENTE DESATIVADO"
                              : "ADMIN · GERA TEMPORÁRIA · TROCA OBRIGATÓRIA"
                        }
                        disabled={!isAdmin || !targetActive}
                        onClick={() => setMode("reset_password")}
                      />
                    )}
                    <ActionRow
                      icon={<KeyRound size={14} />}
                      label="RESETAR TOTP"
                      hint={
                        !isAdmin
                          ? "REQUER ADMINISTRADOR"
                          : !targetActive
                            ? "AGENTE DESATIVADO"
                            : isSelf
                              ? "ADMIN · VOCÊ RECEBERÁ NOVO QR NO PRÓXIMO LOGIN"
                              : "ADMIN · O AGENTE RECEBE QR NO PRÓXIMO LOGIN"
                      }
                      disabled={!isAdmin || !targetActive}
                      onClick={() => setMode("reset_totp")}
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
  const { roles: allRoles, roleLabel } = useAuth();
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
          {allRoles.map((r) => (
            <label key={r.code} className="role-check">
              <input
                type="checkbox"
                checked={roles.includes(r.code)}
                onChange={() => toggle(r.code)}
              />
              <span>{roleLabel(r.code)}</span>
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
      <div className="form-field">
        <span>NOVO CLEARANCE</span>
        <Select
          value={String(level)}
          onChange={(v) => setLevel(Number(v))}
          options={[1, 2, 3, 4, 5].map((n) => ({
            value: String(n),
            label: clearanceLabel(n),
          }))}
        />
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

// ─────────────────────────── EditProfileBlock ────────────────────────────

function EditProfileBlock({
  user,
  onCancel,
  onDone,
}: {
  user: User;
  onCancel: () => void;
  onDone: () => void;
}) {
  const { refreshUser, user: me } = useAuth();
  const isSelf = me?.id === user.id;
  const [name, setName] = useState(user.display_name);
  const [email, setEmail] = useState(user.email);
  const [acting, setActing] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function doIt() {
    setErr(null);
    const payload: { display_name?: string; email?: string } = {};
    const trimName = name.trim();
    const trimEmail = email.trim().toLowerCase();
    if (trimName === "") {
      setErr("Nome de exibição não pode ser vazio");
      return;
    }
    if (trimEmail === "" || !trimEmail.includes("@")) {
      setErr("E-mail inválido");
      return;
    }
    if (trimName !== user.display_name) payload.display_name = trimName;
    if (trimEmail !== user.email) payload.email = trimEmail;
    if (Object.keys(payload).length === 0) {
      setErr("Nenhuma mudança para gravar");
      return;
    }
    setActing(true);
    try {
      await updateUserProfile(user.id, payload);
      if (isSelf) {
        await refreshUser();
      }
      onDone();
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao atualizar");
    } finally {
      setActing(false);
    }
  }

  return (
    <div className="confirm-block">
      <div className="confirm-msg">
        Editar perfil de <b>{user.code} · {user.display_name.toUpperCase()}</b>.
        A ação é registrada no audit log.
      </div>
      <label className="form-field">
        <span>NOME DE EXIBIÇÃO</span>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={120}
        />
      </label>
      <label className="form-field">
        <span>E-MAIL</span>
        <input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          autoComplete="off"
        />
      </label>
      {err && <div className="banner banner-error">⚠ {err}</div>}
      <div className="confirm-actions">
        <button type="button" className="btn btn-ghost" onClick={onCancel} disabled={acting}>
          CANCELAR
        </button>
        <button type="button" className="btn btn-primary" onClick={doIt} disabled={acting}>
          <Pencil size={14} /> {acting ? "GRAVANDO…" : "GRAVAR"}
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────── EditServiceBlock ────────────────────────────

/**
 * Dados funcionais do agente. Marcado em "entrar na composição do SIPOM",
 * ele vai, com o tipo de policiamento Inteligência, em toda ocorrência com
 * participação da inteligência — o analista ainda pode retirá-lo na ficha.
 */
function EditServiceBlock({
  user,
  postos,
  onCancel,
  onDone,
}: {
  user: User;
  postos: { id: number; nome: string }[];
  onCancel: () => void;
  onDone: (u: User) => void;
}) {
  const [registration, setRegistration] = useState(user.registration ?? "");
  const [rankID, setRankID] = useState(user.rank_id ? String(user.rank_id) : "");
  const [number, setNumber] = useState(user.number ?? "");
  const [warName, setWarName] = useState(user.war_name ?? "");
  const [composition, setComposition] = useState(!!user.sipom_composition);
  const [acting, setActing] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function doIt() {
    setErr(null);
    if (composition && registration.trim() === "") {
      setErr("Informe a matrícula para o agente entrar na composição do SIPOM");
      return;
    }
    setActing(true);
    try {
      const r = await updateUserService(user.id, {
        registration: registration.trim(),
        rank_id: rankID ? Number(rankID) : null,
        number: number.trim(),
        war_name: warName.trim(),
        sipom_composition: composition,
      });
      onDone(r.user);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao atualizar");
    } finally {
      setActing(false);
    }
  }

  return (
    <div className="confirm-block">
      <div className="confirm-msg">
        Dados funcionais de <b>{user.code} · {user.display_name.toUpperCase()}</b>. A ação é registrada no audit log.
      </div>
      <label className="form-field">
        <span>MATRÍCULA</span>
        <input
          type="text"
                    value={registration}
          onChange={(e) => setRegistration(e.target.value.toUpperCase().replace(/[^0-9X]/g, ""))}
          maxLength={12}
          placeholder="COMO NO RELATÓRIO · EX.: 3055681X"
        />
      </label>
      <div className="form-field">
        <span>POSTO/GRAD.</span>
        <Select
          value={rankID}
          placeholder={postos.length ? "ESCOLHA" : "CATÁLOGO DO SIPOM INDISPONÍVEL"}
          disabled={postos.length === 0}
          onChange={setRankID}
          options={postos.map((p) => ({ value: String(p.id), label: p.nome }))}
        />
      </div>
      <label className="form-field">
        <span>NUMERAL</span>
        <input type="text" value={number} onChange={(e) => setNumber(e.target.value)} maxLength={10} />
      </label>
      <label className="form-field">
        <span>NOME DE GUERRA</span>
        <input
          type="text"
          value={warName}
          onChange={(e) => setWarName(e.target.value.toUpperCase())}
          maxLength={40}
        />
      </label>
      <label className="role-check">
        <input type="checkbox" checked={composition} onChange={(e) => setComposition(e.target.checked)} />
        <span>ENTRAR NA COMPOSIÇÃO DO SIPOM</span>
      </label>
      <div className="muted">
        Marcado, o agente entra com o tipo de policiamento Inteligência nas ocorrências com participação da
        inteligência.
      </div>
      {err && <div className="banner banner-error">⚠ {err}</div>}
      <div className="confirm-actions">
        <button type="button" className="btn btn-ghost" onClick={onCancel} disabled={acting}>
          CANCELAR
        </button>
        <button type="button" className="btn btn-primary" onClick={doIt} disabled={acting}>
          <BadgeCheck size={14} /> {acting ? "GRAVANDO…" : "GRAVAR"}
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────── ResetPasswordBlock ──────────────────────────

function ResetPasswordBlock({
  user,
  onCancel,
  onDone,
}: {
  user: User;
  onCancel: () => void;
  onDone: () => void;
}) {
  const [acting, setActing] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [temp, setTemp] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function doIt() {
    setErr(null);
    setActing(true);
    try {
      const r = await resetUserPassword(user.id);
      setTemp(r.temp_password);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao resetar senha");
    } finally {
      setActing(false);
    }
  }

  async function copyTemp() {
    if (!temp) return;
    try {
      await navigator.clipboard.writeText(temp);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      /* ignore */
    }
  }

  if (temp) {
    return (
      <div className="confirm-block">
        <div className="banner banner-info">
          ✓ Senha temporária gerada. Repasse de forma segura ao agente.
          Sessões ativas foram revogadas.
        </div>
        <div className="totp-setup-secret">
          <span className="totp-setup-secret-label">// SENHA TEMPORÁRIA</span>
          <code>{temp}</code>
          <button type="button" className="action-btn" onClick={copyTemp} title="Copiar">
            {copied ? "✓" : "⧉"}
          </button>
        </div>
        <div className="confirm-actions">
          <button type="button" className="btn btn-primary" onClick={onDone}>
            ENTENDIDO
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="confirm-block">
      <div className="confirm-msg">
        ⚠ Confirme o reset de senha de <b>{user.code} · {user.display_name.toUpperCase()}</b>.
        Uma senha temporária será gerada e exibida UMA vez. O agente deverá
        trocá-la no próximo login. Todas as sessões ativas serão revogadas.
      </div>
      {err && <div className="banner banner-error">⚠ {err}</div>}
      <div className="confirm-actions">
        <button type="button" className="btn btn-ghost" onClick={onCancel} disabled={acting}>
          CANCELAR
        </button>
        <button type="button" className="btn btn-danger" onClick={doIt} disabled={acting}>
          <KeyRound size={14} /> {acting ? "GERANDO…" : "RESETAR SENHA"}
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────── ChangePasswordBlock ─────────────────────────

function ChangePasswordBlock({
  onCancel,
  onDone,
}: {
  onCancel: () => void;
  onDone: () => void;
}) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [acting, setActing] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function doIt() {
    setErr(null);
    if (next.length < 8) {
      setErr("A nova senha deve ter ao menos 8 caracteres");
      return;
    }
    if (next !== confirm) {
      setErr("A confirmação não confere");
      return;
    }
    if (next === current) {
      setErr("A nova senha deve ser diferente da atual");
      return;
    }
    setActing(true);
    try {
      await changeOwnPassword(current, next);
      onDone();
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao trocar senha");
    } finally {
      setActing(false);
    }
  }

  return (
    <div className="confirm-block">
      <div className="confirm-msg">
        Trocar a própria senha. Informe a senha atual e a nova (mínimo 12
        caracteres). A ação é registrada no audit log.
      </div>
      <label className="form-field">
        <span>SENHA ATUAL</span>
        <input
          type="password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
          autoComplete="current-password"
        />
      </label>
      <label className="form-field">
        <span>NOVA SENHA</span>
        <input
          type="password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          autoComplete="new-password"
          minLength={12}
        />
      </label>
      <label className="form-field">
        <span>CONFIRMAR NOVA SENHA</span>
        <input
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          autoComplete="new-password"
          minLength={12}
        />
      </label>
      {err && <div className="banner banner-error">⚠ {err}</div>}
      <div className="confirm-actions">
        <button type="button" className="btn btn-ghost" onClick={onCancel} disabled={acting}>
          CANCELAR
        </button>
        <button type="button" className="btn btn-primary" onClick={doIt} disabled={acting}>
          <KeyRound size={14} /> {acting ? "GRAVANDO…" : "TROCAR SENHA"}
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────── ResetTOTPBlock ──────────────────────────────

function ResetTOTPBlock({
  user,
  onCancel,
  onDone,
}: {
  user: User;
  onCancel: () => void;
  onDone: () => void;
}) {
  const [acting, setActing] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function doIt() {
    setErr(null);
    setActing(true);
    try {
      await resetUserTOTP(user.id);
      onDone();
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao resetar TOTP");
    } finally {
      setActing(false);
    }
  }

  return (
    <div className="confirm-block">
      <div className="confirm-msg">
        ⚠ Confirme o reset de TOTP de <b>{user.code} · {user.display_name.toUpperCase()}</b>.
        O secret atual será apagado e o agente receberá um novo QR no
        próximo login. O admin <b>não vê</b> o novo secret — apenas o
        agente, no momento do enrollment. Sessões ativas serão revogadas.
      </div>
      {err && <div className="banner banner-error">⚠ {err}</div>}
      <div className="confirm-actions">
        <button type="button" className="btn btn-ghost" onClick={onCancel} disabled={acting}>
          CANCELAR
        </button>
        <button type="button" className="btn btn-danger" onClick={doIt} disabled={acting}>
          <KeyRound size={14} /> {acting ? "RESETANDO…" : "RESETAR TOTP"}
        </button>
      </div>
    </div>
  );
}
