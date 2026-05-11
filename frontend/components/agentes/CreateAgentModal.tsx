"use client";

import { useState, type FormEvent } from "react";
import { Copy, X } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { createUser, type CreatedUser } from "@/lib/users-api";
import type { ApiError } from "@/lib/api";
import type { RoleCode } from "@/lib/types";

const TOTP_ISSUER = "Tevunah";

function otpauthURI(secret: string, account: string): string {
  const issuer = encodeURIComponent(TOTP_ISSUER);
  const label = `${issuer}:${encodeURIComponent(account)}`;
  return `otpauth://totp/${label}?secret=${secret}&issuer=${issuer}&algorithm=SHA1&digits=6&period=30`;
}

type Props = {
  onClose: () => void;
  onCreated: () => void;
};

const ROLES: { code: RoleCode; label: string }[] = [
  { code: "agente", label: "AGENTE" },
  { code: "analista", label: "ANALISTA" },
  { code: "gestor", label: "GESTOR" },
  { code: "administrador", label: "ADMINISTRADOR" },
];

export default function CreateAgentModal({ onClose, onCreated }: Props) {
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [clearance, setClearance] = useState(1);
  const [roles, setRoles] = useState<RoleCode[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [created, setCreated] = useState<CreatedUser | null>(null);
  const [copied, setCopied] = useState(false);

  function toggleRole(r: RoleCode) {
    setRoles((rs) => (rs.includes(r) ? rs.filter((x) => x !== r) : [...rs, r]));
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (roles.length === 0) {
      setErr("Selecione ao menos um papel");
      return;
    }
    setBusy(true);
    try {
      const result = await createUser({
        email: email.trim().toLowerCase(),
        display_name: name.trim(),
        password,
        roles,
        clearance_level: clearance,
      });
      setCreated(result);
    } catch (e) {
      setErr((e as ApiError).message || "Erro ao criar agente");
    } finally {
      setBusy(false);
    }
  }

  function copyTOTP() {
    if (!created) return;
    navigator.clipboard.writeText(created.totp_secret).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span>{created ? "AGENTE CRIADO" : "NOVO AGENTE"}</span>
          <button
            type="button"
            className="action-btn"
            onClick={onClose}
            aria-label="Fechar"
          >
            <X size={14} />
          </button>
        </div>

        {!created ? (
          <form className="modal-bd" onSubmit={onSubmit} autoComplete="off">
            <label className="form-field">
              <span>NOME DE EXIBIÇÃO</span>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                autoFocus
              />
            </label>

            <label className="form-field">
              <span>E-MAIL OPERACIONAL</span>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                spellCheck={false}
              />
            </label>

            <label className="form-field">
              <span>SENHA TEMPORÁRIA · MÍN 12 CARACTERES</span>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                minLength={12}
                required
              />
            </label>

            <fieldset className="form-fieldset">
              <legend>PAPÉIS</legend>
              <div className="checkbox-row">
                {ROLES.map((r) => (
                  <label key={r.code} className="check">
                    <input
                      type="checkbox"
                      checked={roles.includes(r.code)}
                      onChange={() => toggleRole(r.code)}
                    />
                    {r.label}
                  </label>
                ))}
              </div>
            </fieldset>

            <label className="form-field">
              <span>NÍVEL DE CLEARANCE</span>
              <select
                value={clearance}
                onChange={(e) => setClearance(Number(e.target.value))}
              >
                {[1, 2, 3, 4, 5].map((n) => (
                  <option key={n} value={n}>
                    CL-0{n}
                  </option>
                ))}
              </select>
            </label>

            {err && <div className="banner banner-error">⚠ {err}</div>}

            <div className="modal-ft">
              <button type="button" className="btn btn-ghost" onClick={onClose}>
                CANCELAR
              </button>
              <button type="submit" className="btn btn-primary" disabled={busy}>
                {busy ? "CRIANDO…" : "CRIAR AGENTE"}
              </button>
            </div>
          </form>
        ) : (
          <div className="modal-bd">
            <div className="success-box">
              ✓ Agente <b>{created.user.display_name.toUpperCase()}</b> ({created.user.code})
              criado com sucesso.
            </div>

            <div className="totp-box">
              <div className="totp-lbl">
                TOTP SECRET · CONFIGURE NO APP AUTENTICADOR AGORA
              </div>
              <div className="totp-val">
                <code>{created.totp_secret}</code>
                <button type="button" className="btn btn-ghost" onClick={copyTOTP}>
                  <Copy size={12} /> {copied ? "COPIADO" : "COPIAR"}
                </button>
              </div>
              <div className="totp-qr">
                <QRCodeSVG
                  value={otpauthURI(created.totp_secret, created.user.email)}
                  size={168}
                  bgColor="#ffffff"
                  fgColor="#000000"
                  level="M"
                  marginSize={2}
                />
                <div className="totp-qr-cap">
                  ESCANEIE COM AEGIS / 1PASSWORD / AUTHY / BITWARDEN
                </div>
              </div>
              <div className="totp-note">{created.note}</div>
            </div>

            <div className="modal-ft">
              <button type="button" className="btn btn-primary" onClick={onCreated}>
                CONCLUIR
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
