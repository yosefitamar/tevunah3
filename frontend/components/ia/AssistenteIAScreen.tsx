"use client";

import { useEffect, useState } from "react";
import { CheckCircle2, KeyRound, RefreshCw, ShieldAlert, Trash2 } from "lucide-react";
import { useModal } from "@/contexts/ModalContext";
import {
  deleteAIKey,
  getAISettings,
  saveAIKey,
  saveAIModel,
  verifyAIKey,
  type AIPeriod,
  type AISettings,
} from "@/lib/ai-api";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";

const USD = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 });
const USD_FINE = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 4 });
const INT = new Intl.NumberFormat("pt-BR");
const MONTH = new Intl.DateTimeFormat("pt-BR", { month: "long", year: "numeric", timeZone: "America/Fortaleza" });
const YEAR = new Intl.DateTimeFormat("pt-BR", { year: "numeric", timeZone: "America/Fortaleza" });

/** Centavos de dólar somem em gasto pequeno: abaixo de US$ 1, 4 casas. */
function money(v: number) {
  return v > 0 && v < 1 ? USD_FINE.format(v) : USD.format(v);
}

type Msg = { kind: "ok" | "error"; text: string } | null;

/**
 * Assistente de IA — configuração do próprio usuário: o gasto do mês e do
 * ano, a chave da API Anthropic (cifrada no servidor, usada só nas
 * solicitações dele) e o modelo que as funções de IA vão usar.
 */
export default function AssistenteIAScreen() {
  const modal = useModal();
  const [data, setData] = useState<AISettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [keyInput, setKeyInput] = useState("");
  const [keyBusy, setKeyBusy] = useState<null | "save" | "verify" | "delete">(null);
  const [keyMsg, setKeyMsg] = useState<Msg>(null);
  const [modelBusy, setModelBusy] = useState<string | null>(null);
  const [modelMsg, setModelMsg] = useState<Msg>(null);

  useEffect(() => {
    getAISettings()
      .then(setData)
      .catch((e) => setError((e as ApiError).message || "Erro ao carregar"))
      .finally(() => setLoading(false));
  }, []);

  async function runKey(kind: "save" | "verify" | "delete", fn: () => Promise<AISettings>, ok: string) {
    setKeyBusy(kind);
    setKeyMsg(null);
    try {
      setData(await fn());
      setKeyMsg({ kind: "ok", text: ok });
      return true;
    } catch (err) {
      setKeyMsg({ kind: "error", text: (err as ApiError).message || "Erro" });
      return false;
    } finally {
      setKeyBusy(null);
    }
  }

  async function onSaveKey(e: React.FormEvent) {
    e.preventDefault();
    if (!keyInput.trim()) return;
    const ok = await runKey("save", () => saveAIKey(keyInput.trim()), "Chave verificada na Anthropic e guardada cifrada.");
    if (ok) setKeyInput("");
  }

  async function onDelete() {
    const ok = await modal.confirm({
      variant: "warning",
      title: "REMOVER CHAVE DA API",
      message: "As funções de IA deixam de funcionar para você até cadastrar outra chave.",
      confirm: "REMOVER",
      cancel: "CANCELAR",
      danger: true,
    });
    if (ok) await runKey("delete", deleteAIKey, "Chave removida.");
  }

  async function onPickModel(id: string) {
    if (!data || id === data.model || modelBusy) return;
    setModelBusy(id);
    setModelMsg(null);
    try {
      setData(await saveAIModel(id));
      setModelMsg({ kind: "ok", text: "Modelo salvo." });
    } catch (err) {
      setModelMsg({ kind: "error", text: (err as ApiError).message || "Erro ao salvar" });
    } finally {
      setModelBusy(null);
    }
  }

  if (loading) {
    return (
      <div className="ia-center">
        <span className="spinner spinner--lg" aria-label="Carregando" />
      </div>
    );
  }
  if (error || !data) {
    return <div className="banner banner-error">⚠ {error ?? "Erro ao carregar"}</div>;
  }

  const now = new Date();
  const restricted = data.configured && data.available_models.length > 0;

  return (
    <div className="screen-fill ia-screen">
      <div className="section-title">
        ASSISTENTE DE IA
        <span style={{ color: "var(--fg-2)" }}>· API ANTHROPIC · CONFIGURAÇÃO PESSOAL</span>
      </div>

      {/* ── Gasto ─────────────────────────────────────────── */}
      <div className="ia-spend">
        <SpendCard label={`GASTO EM ${MONTH.format(now).toUpperCase()}`} p={data.spending.month} />
        <SpendCard label={`GASTO EM ${YEAR.format(now)}`} p={data.spending.year} />
      </div>

      {/* ── Chave ─────────────────────────────────────────── */}
      <div className="panel">
        <div className="panel-hd">
          <span className="ttl">CHAVE DA API</span>
          <span className="meta">USADA SÓ NAS SUAS SOLICITAÇÕES · O CUSTO VAI PARA A SUA CONTA ANTHROPIC</span>
        </div>
        <div className="panel-bd ia-key">
          {!data.secrets_ready && (
            <div className="banner banner-warn">
              <ShieldAlert size={13} /> O SERVIDOR AINDA NÃO TEM O COFRE DE SEGREDOS CONFIGURADO — A CHAVE NÃO
              PODE SER GUARDADA. FALE COM O ADMINISTRADOR.
            </div>
          )}

          <div className="ia-key-status">
            <span className={"ia-key-icon" + (data.configured ? " ia-key-icon--ok" : "")}>
              <KeyRound size={18} />
            </span>
            <div className="ia-key-info">
              <div className="ia-key-hint">{data.configured ? data.key_hint : "NENHUMA CHAVE CADASTRADA"}</div>
              <div className="muted">
                {data.configured
                  ? data.key_verified_at
                    ? `VERIFICADA EM ${formatBR(data.key_verified_at)}`
                    : "AINDA NÃO VERIFICADA"
                  : "Crie uma chave em console.anthropic.com → API Keys e cole abaixo."}
              </div>
            </div>
            {data.configured && (
              <div className="ia-key-actions">
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={() => runKey("verify", verifyAIKey, "Chave funcionando.")}
                  disabled={!!keyBusy}
                >
                  <RefreshCw size={13} className={keyBusy === "verify" ? "ia-spin" : undefined} /> TESTAR
                </button>
                <button type="button" className="btn btn-danger" onClick={onDelete} disabled={!!keyBusy}>
                  <Trash2 size={13} /> REMOVER
                </button>
              </div>
            )}
          </div>

          <form className="ia-key-form" onSubmit={onSaveKey} autoComplete="off">
            <label className="form-field">
              <span>{data.configured ? "SUBSTITUIR POR OUTRA CHAVE" : "CHAVE DA API"}</span>
              <input
                type="password"
                value={keyInput}
                onChange={(e) => setKeyInput(e.target.value)}
                placeholder="sk-ant-api03-…"
                autoComplete="new-password"
                spellCheck={false}
                disabled={!data.secrets_ready || !!keyBusy}
              />
            </label>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={!data.secrets_ready || !keyInput.trim() || !!keyBusy}
            >
              {keyBusy === "save" ? "VERIFICANDO…" : "VERIFICAR E SALVAR"}
            </button>
          </form>
          {keyMsg && (
            <div className={"banner " + (keyMsg.kind === "ok" ? "banner-info" : "banner-error")}>
              {keyMsg.kind === "ok" ? <CheckCircle2 size={13} /> : "⚠"} {keyMsg.text}
            </div>
          )}
        </div>
      </div>

      {/* ── Modelo ────────────────────────────────────────── */}
      <div className="panel">
        <div className="panel-hd">
          <span className="ttl">MODELO</span>
          <span className="meta">
            {modelMsg ? (
              <span className={modelMsg.kind === "ok" ? "ia-save-ok" : "ia-save-err"}>
                {modelMsg.kind === "ok" ? "✓" : "⚠"} {modelMsg.text}
              </span>
            ) : (
              "US$ POR MILHÃO DE TOKENS · ENTRADA / SAÍDA"
            )}
          </span>
        </div>
        <div className="panel-bd dense">
          {data.catalog.map((m) => {
            const on = data.model === m.id;
            const unavailable = restricted && !data.available_models.includes(m.id);
            return (
              <label key={m.id} className={"ia-model-row" + (on ? " ia-model-row--on" : "")}>
                <input
                  type="radio"
                  name="ia-model"
                  checked={on}
                  onChange={() => onPickModel(m.id)}
                  disabled={!!modelBusy}
                />
                <span className="ia-model-name">{m.name.toUpperCase()}</span>
                {m.recommended && <span className="ia-badge ia-badge--rec">PADRÃO</span>}
                {unavailable && <span className="ia-badge ia-badge--warn">NÃO LIBERADO NA CHAVE</span>}
                <span className="ia-model-desc muted">{m.description}</span>
                <span className="ia-model-price">
                  {USD.format(m.input_usd_per_mtok)} / {USD.format(m.output_usd_per_mtok)}
                </span>
              </label>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function SpendCard({ label, p }: { label: string; p: AIPeriod }) {
  return (
    <div className="panel kpi">
      <div className="kpi-lbl">{label}</div>
      <div className="kpi-val">{money(p.cost_usd)}</div>
      <div className="kpi-trend">
        {p.requests === 0
          ? "NENHUMA CHAMADA À IA NO PERÍODO"
          : `${INT.format(p.requests)} CHAMADA(S) · ${INT.format(p.input_tokens)} TOKENS DE ENTRADA · ${INT.format(p.output_tokens)} DE SAÍDA`}
      </div>
    </div>
  );
}
