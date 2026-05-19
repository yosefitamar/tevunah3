"use client";

import { useState, type FormEvent } from "react";
import { CheckCircle2, Copy, Smartphone } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { useAuth } from "@/contexts/AuthContext";
import { setupTOTP } from "@/lib/users-api";
import type { ApiError } from "@/lib/api";
import TevunahLogo from "./TevunahLogo";

const TOTP_ISSUER = "Tevunah";

function otpauthURI(secret: string, account: string): string {
  const issuer = encodeURIComponent(TOTP_ISSUER);
  const label = `${issuer}:${encodeURIComponent(account)}`;
  return `otpauth://totp/${label}?secret=${secret}&issuer=${issuer}&algorithm=SHA1&digits=6&period=30`;
}

/**
 * Tela de enrollment TOTP — exibida quando o agente loga após reset por
 * admin. Mostra QR + secret (pra digitação manual), pede o primeiro código
 * gerado pelo authenticator e confirma o setup no backend.
 */
export default function TOTPSetupScreen() {
  const { user, pendingTOTPSetup, clearPendingTOTPSetup, refreshUser, logout } = useAuth();
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  if (!user || !pendingTOTPSetup) {
    return null;
  }
  const { secret, email } = pendingTOTPSetup;
  const uri = otpauthURI(secret, email);

  async function copySecret() {
    try {
      await navigator.clipboard.writeText(secret);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      /* ignore */
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      await setupTOTP(code.trim());
      clearPendingTOTPSetup();
      await refreshUser();
    } catch (e) {
      const ae = e as ApiError;
      setErr(ae.message || "Falha ao confirmar TOTP");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="login-shell">
      <div className="classification">
        <span>◆ SAI 2º BPRAIO</span>
        <span className="sep">//</span>
        <span>TEVUNAH</span>
        <span className="sep">//</span>
        <span>ENROLLMENT TOTP</span>
      </div>

      <div className="login-body">
        <form
          className="login-card"
          onSubmit={onSubmit}
          autoComplete="off"
          style={{ maxWidth: 460 }}
        >
          <div className="login-brand">
            <div className="brand-glyph">
              <TevunahLogo />
            </div>
            <div className="brand-heb">תבונה</div>
          </div>

          <div className="login-tag">// CONFIGURAÇÃO DE 2FA</div>

          <div className="totp-setup-note">
            <Smartphone size={14} strokeWidth={1.6} /> O TOTP foi resetado pelo
            admin. Escaneie o QR no seu authenticator (Google Authenticator,
            Authy, 1Password, etc.) e confirme com o primeiro código gerado.
          </div>

          <div className="totp-setup-qr">
            <QRCodeSVG
              value={uri}
              size={180}
              bgColor="var(--bg-0)"
              fgColor="var(--accent)"
              level="M"
            />
          </div>

          <div className="totp-setup-secret">
            <span className="totp-setup-secret-label">// SECRET MANUAL</span>
            <code>{secret}</code>
            <button
              type="button"
              className="action-btn"
              onClick={copySecret}
              title="Copiar secret"
            >
              {copied ? <CheckCircle2 size={12} /> : <Copy size={12} />}
            </button>
          </div>

          <label className="login-field">
            <span>CÓDIGO GERADO · 6 DÍGITOS</span>
            <input
              type="text"
              inputMode="numeric"
              pattern="\d{6}"
              maxLength={6}
              value={code}
              autoFocus
              onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
              required
              autoComplete="one-time-code"
            />
          </label>

          {err && <div className="login-error">⚠ {err}</div>}

          <button type="submit" disabled={busy || code.length !== 6} className="login-submit">
            {busy ? "CONFIRMANDO…" : "CONFIRMAR ENROLLMENT"}
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
            // O ADMIN NÃO TEM ACESSO AO SECRET. GUARDE-O AGORA.
          </div>
        </form>
      </div>

      <div className="classification bottom">
        <span>◆ SAI 2º BPRAIO // TEVUNAH</span>
        <span className="sep">//</span>
        <span>USO MONITORADO</span>
      </div>
    </div>
  );
}
