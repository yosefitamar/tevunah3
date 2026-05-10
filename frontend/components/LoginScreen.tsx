"use client";

import { useState, type FormEvent } from "react";
import { LogIn } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import type { ApiError } from "@/lib/api";
import TevunahLogo from "./TevunahLogo";

export default function LoginScreen() {
  const { login } = useAuth();
  const [email, setEmail] = useState("");
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
    } catch (e) {
      const ae = e as ApiError;
      setErr(ae.message || "Falha na autenticação");
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
        <span>ACESSO RESTRITO</span>
      </div>

      <div className="login-body">
        <form className="login-card" onSubmit={onSubmit} autoComplete="off">
          <div className="login-brand">
            <div className="brand-glyph">
              <TevunahLogo />
            </div>
            <div className="brand-heb">תבונה</div>
          </div>

          <div className="login-tag">// AUTENTICAÇÃO REQUERIDA</div>

          <label className="login-field">
            <span>E-MAIL OPERACIONAL</span>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoFocus
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
            {busy ? "AUTENTICANDO…" : "ACESSAR"}
          </button>

          <div className="login-foot">
            // FALHAS DE AUTENTICAÇÃO SÃO REGISTRADAS NO TRILHO DE AUDITORIA
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
