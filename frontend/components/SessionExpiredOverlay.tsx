"use client";

import { useState, type FormEvent } from "react";
import { LogIn } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import type { ApiError } from "@/lib/api";
import TevunahLogo from "./TevunahLogo";

/**
 * Overlay que cobre o shell quando a sessão expira no meio do uso. Bloqueia
 * interação com o conteúdo atrás e exige re-autenticação. Após login bem-
 * sucedido, o AuthContext zera `sessionExpired` e o overlay some — o estado
 * das telas embaixo é preservado.
 *
 * Sugere o e-mail do usuário atual (auth.user) — em sistemas onde o terminal
 * pode ser compartilhado, isso é informativo e não vulnerabilidade (o e-mail
 * já apareceu no header durante a sessão). A senha e o TOTP são obrigatórios.
 */
export default function SessionExpiredOverlay() {
  const { user, login } = useAuth();
  const [email, setEmail] = useState(user?.email ?? "");
  const [password, setPassword] = useState("");
  const [totp, setTotp] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      await login({ email: email.trim(), password, totp_code: totp.trim() });
      // Sucesso: AuthContext zera sessionExpired → overlay desmonta.
    } catch (e) {
      setErr((e as ApiError).message || "Falha na autenticação");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="session-overlay" role="dialog" aria-modal="true" aria-label="Sessão expirada">
      <form className="login-card session-overlay-card" onSubmit={onSubmit} autoComplete="off">
        <div className="login-brand">
          <div className="brand-glyph">
            <TevunahLogo />
          </div>
          <div className="brand-heb">תבונה</div>
        </div>

        <div className="login-tag" style={{ color: "var(--warn)" }}>
          // SESSÃO EXPIRADA · RE-AUTENTICAÇÃO NECESSÁRIA
        </div>

        <label className="login-field">
          <span>E-MAIL OPERACIONAL</span>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            spellCheck={false}
            autoComplete="username"
          />
        </label>

        <label className="login-field">
          <span>SENHA</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoFocus
            autoComplete="current-password"
          />
        </label>

        <label className="login-field">
          <span>CÓDIGO TOTP · 6 DÍGITOS</span>
          <input
            type="text"
            inputMode="numeric"
            pattern="\d{6}"
            maxLength={6}
            value={totp}
            onChange={(e) => setTotp(e.target.value.replace(/\D/g, ""))}
            required
            autoComplete="one-time-code"
          />
        </label>

        {err && <div className="login-error">⚠ {err}</div>}

        <button type="submit" disabled={busy} className="login-submit">
          <LogIn size={14} strokeWidth={1.6} />
          {busy ? "AUTENTICANDO…" : "RESTAURAR SESSÃO"}
        </button>

        <div className="login-foot">
          // O CONTEÚDO ATRÁS ESTÁ BLOQUEADO. TENTATIVAS DE AUTENTICAÇÃO SÃO REGISTRADAS.
        </div>
      </form>
    </div>
  );
}
