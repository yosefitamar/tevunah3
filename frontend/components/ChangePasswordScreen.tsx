"use client";

import { useState, type FormEvent } from "react";
import { KeyRound } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useSystemSettings } from "@/contexts/SystemSettingsContext";
import { changeOwnPassword } from "@/lib/users-api";
import type { ApiError } from "@/lib/api";
import TevunahLogo from "./TevunahLogo";

/**
 * Tela de troca forçada de senha — exibida quando o agente loga após reset
 * por admin (must_change_password=true). Pede senha atual (a temporária
 * recém-recebida) + nova senha. Após sucesso, refresca o user e o gate
 * libera o shell normal.
 */
export default function ChangePasswordScreen() {
  const { logout, refreshUser } = useAuth();
  const { settings } = useSystemSettings();
  const agencyLabel = settings?.agency_name || "—";
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (next.length < 12) {
      setErr("A nova senha deve ter ao menos 12 caracteres");
      return;
    }
    if (next !== confirm) {
      setErr("A confirmação não confere com a nova senha");
      return;
    }
    if (next === current) {
      setErr("A nova senha deve ser diferente da atual");
      return;
    }
    setBusy(true);
    try {
      await changeOwnPassword(current, next);
      await refreshUser();
    } catch (e) {
      const ae = e as ApiError;
      setErr(ae.message || "Falha ao trocar senha");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="login-shell">
      <div className="classification">
        <span>◆ {agencyLabel}</span>
        <span className="sep">//</span>
        <span>TEVUNAH</span>
        <span className="sep">//</span>
        <span>TROCA DE SENHA OBRIGATÓRIA</span>
      </div>

      <div className="login-body">
        <form
          className="login-card"
          onSubmit={onSubmit}
          autoComplete="off"
          style={{ maxWidth: 420 }}
        >
          <div className="login-brand">
            <div className="brand-glyph">
              <TevunahLogo />
            </div>
            <div className="brand-heb">תבונה</div>
          </div>

          <div className="login-tag">// TROCA DE SENHA REQUERIDA</div>

          <div className="totp-setup-note">
            <KeyRound size={14} strokeWidth={1.6} /> Sua senha foi resetada
            pelo admin. Defina uma nova senha (mínimo 12 caracteres) para
            prosseguir.
          </div>

          <label className="login-field">
            <span>SENHA ATUAL (TEMPORÁRIA)</span>
            <input
              type="password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
              required
              autoFocus
              autoComplete="current-password"
            />
          </label>

          <label className="login-field">
            <span>NOVA SENHA</span>
            <input
              type="password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
              required
              minLength={12}
              autoComplete="new-password"
            />
          </label>

          <label className="login-field">
            <span>CONFIRMAR NOVA SENHA</span>
            <input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              minLength={12}
              autoComplete="new-password"
            />
          </label>

          {err && <div className="login-error">⚠ {err}</div>}

          <button type="submit" disabled={busy} className="login-submit">
            {busy ? "GRAVANDO…" : "TROCAR SENHA"}
          </button>

          <button
            type="button"
            className="login-secondary"
            onClick={logout}
            disabled={busy}
          >
            SAIR
          </button>

          <div className="login-foot">
            // SENHA MÍNIMA: 12 CARACTERES · TROCA REGISTRADA NO AUDIT
          </div>
        </form>
      </div>

      <div className="classification bottom">
        <span>◆ {agencyLabel} // TEVUNAH</span>
        <span className="sep">//</span>
        <span>USO MONITORADO</span>
      </div>
    </div>
  );
}
