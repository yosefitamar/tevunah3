"use client";

import { useEffect, useMemo, useState } from "react";
import { CheckCircle2, KeyRound, RefreshCw, ShieldAlert, Sparkles, Trash2 } from "lucide-react";
import { useModal } from "@/contexts/ModalContext";
import {
  deleteAIKey,
  estimateUSD,
  getAISettings,
  saveAIKey,
  saveAISettings,
  verifyAIKey,
  type AISettings,
  type AIUsageProfile,
} from "@/lib/ai-api";
import { formatBR, formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";

const USD = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 });
const USD_FINE = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 4 });
const BRL = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL", maximumFractionDigits: 2 });
const INT = new Intl.NumberFormat("pt-BR");

/** Campos do perfil de uso: rótulo, limites e passo. */
const USAGE_FIELDS: {
  key: keyof AIUsageProfile;
  label: string;
  hint: string;
  min: number;
  max: number;
  step: number;
}[] = [
  { key: "requests_per_day", label: "REQUISIÇÕES POR DIA ÚTIL", hint: "Quantas vezes por dia a IA será acionada", min: 0, max: 100000, step: 1 },
  { key: "work_days_per_month", label: "DIAS ÚTEIS POR MÊS", hint: "Dias com uso no mês", min: 1, max: 31, step: 1 },
  { key: "input_tokens_per_request", label: "TOKENS DE ENTRADA / REQ.", hint: "≈ 4 caracteres por token; uma página ≈ 600", min: 0, max: 1000000, step: 100 },
  { key: "output_tokens_per_request", label: "TOKENS DE SAÍDA / REQ.", hint: "Resposta + raciocínio do modelo", min: 0, max: 128000, step: 100 },
  { key: "usd_brl", label: "COTAÇÃO US$ → R$", hint: "Ajuste para a cotação do dia", min: 0.01, max: 100, step: 0.01 },
];

/**
 * Assistente de IA — configuração do próprio usuário: a chave da API
 * Anthropic (cifrada no servidor, usada só nas solicitações dele), o modelo
 * que as features de IA vão usar e a estimativa de custo mensal/anual.
 */
export default function AssistenteIAScreen() {
  const modal = useModal();
  const [data, setData] = useState<AISettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [keyInput, setKeyInput] = useState("");
  const [keyBusy, setKeyBusy] = useState<null | "save" | "verify" | "delete">(null);
  const [keyMsg, setKeyMsg] = useState<{ kind: "ok" | "error"; text: string } | null>(null);

  const [model, setModel] = useState("");
  const [usage, setUsage] = useState<AIUsageProfile | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState<{ kind: "ok" | "error"; text: string } | null>(null);

  function apply(s: AISettings) {
    setData(s);
    setModel(s.model);
    setUsage(s.usage);
  }

  useEffect(() => {
    getAISettings()
      .then(apply)
      .catch((e) => setError((e as ApiError).message || "Erro ao carregar"))
      .finally(() => setLoading(false));
  }, []);

  const dirty =
    !!data && !!usage && (model !== data.model || JSON.stringify(usage) !== JSON.stringify(data.usage));
  const usageValid =
    !!usage && USAGE_FIELDS.every((f) => Number.isFinite(usage[f.key]) && usage[f.key] >= f.min && usage[f.key] <= f.max);

  const estimates = useMemo(() => {
    if (!data || !usage) return [];
    return data.catalog.map((m) => ({ model: m, ...estimateUSD(m, usage) }));
  }, [data, usage]);
  const selected = estimates.find((e) => e.model.id === model);

  async function onSaveKey(e: React.FormEvent) {
    e.preventDefault();
    if (!keyInput.trim()) return;
    setKeyBusy("save");
    setKeyMsg(null);
    try {
      const s = await saveAIKey(keyInput.trim());
      setKeyInput("");
      setData(s);
      setKeyMsg({ kind: "ok", text: "Chave verificada na Anthropic e guardada cifrada." });
    } catch (err) {
      setKeyMsg({ kind: "error", text: (err as ApiError).message || "Erro ao salvar a chave" });
    } finally {
      setKeyBusy(null);
    }
  }

  async function onVerify() {
    setKeyBusy("verify");
    setKeyMsg(null);
    try {
      const s = await verifyAIKey();
      setData(s);
      setKeyMsg({ kind: "ok", text: "Chave funcionando." });
    } catch (err) {
      setKeyMsg({ kind: "error", text: (err as ApiError).message || "Erro ao verificar" });
    } finally {
      setKeyBusy(null);
    }
  }

  async function onDelete() {
    const ok = await modal.confirm({
      variant: "warning",
      title: "REMOVER CHAVE DA API",
      message:
        "As funções de IA deixam de funcionar para você até cadastrar outra chave. O modelo escolhido e a estimativa continuam salvos.",
      confirm: "REMOVER",
      cancel: "CANCELAR",
      danger: true,
    });
    if (!ok) return;
    setKeyBusy("delete");
    setKeyMsg(null);
    try {
      setData(await deleteAIKey());
      setKeyMsg({ kind: "ok", text: "Chave removida." });
    } catch (err) {
      setKeyMsg({ kind: "error", text: (err as ApiError).message || "Erro ao remover" });
    } finally {
      setKeyBusy(null);
    }
  }

  async function onSaveSettings() {
    if (!usage) return;
    setSaving(true);
    setSaveMsg(null);
    try {
      apply(await saveAISettings(model, usage));
      setSaveMsg({ kind: "ok", text: "Configuração salva." });
    } catch (err) {
      setSaveMsg({ kind: "error", text: (err as ApiError).message || "Erro ao salvar" });
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="ia-center">
        <span className="spinner spinner--lg" aria-label="Carregando" />
      </div>
    );
  }
  if (error || !data || !usage) {
    return <div className="banner banner-error">⚠ {error ?? "Erro ao carregar"}</div>;
  }

  const restricted = data.configured && data.available_models.length > 0;

  return (
    <div className="screen-fill ia-screen">
      <div className="section-title">
        ASSISTENTE DE IA
        <span style={{ color: "var(--fg-2)" }}>· API ANTHROPIC · CONFIGURAÇÃO PESSOAL</span>
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

          {data.configured ? (
            <div className="ia-key-status">
              <span className="ia-key-icon ia-key-icon--ok">
                <KeyRound size={18} />
              </span>
              <div className="ia-key-info">
                <div className="ia-key-hint">{data.key_hint}</div>
                <div className="muted">
                  {data.key_verified_at
                    ? `VERIFICADA EM ${formatBR(data.key_verified_at)}`
                    : "AINDA NÃO VERIFICADA"}
                  {data.available_models.length > 0 &&
                    ` · ${data.available_models.length} MODELO(S) LIBERADO(S) NA CHAVE`}
                </div>
              </div>
              <div className="ia-key-actions">
                <button type="button" className="btn btn-ghost" onClick={onVerify} disabled={!!keyBusy}>
                  <RefreshCw size={13} className={keyBusy === "verify" ? "ia-spin" : undefined} /> TESTAR
                </button>
                <button type="button" className="btn btn-danger" onClick={onDelete} disabled={!!keyBusy}>
                  <Trash2 size={13} /> REMOVER
                </button>
              </div>
            </div>
          ) : (
            <div className="ia-key-status">
              <span className="ia-key-icon">
                <KeyRound size={18} />
              </span>
              <div className="ia-key-info">
                <div className="ia-key-hint">NENHUMA CHAVE CADASTRADA</div>
                <div className="muted">
                  Crie uma chave em console.anthropic.com → API Keys e cole abaixo. Ela é verificada na
                  Anthropic antes de ser guardada.
                </div>
              </div>
            </div>
          )}

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
          <small className="muted">
            // A CHAVE É CIFRADA NO SERVIDOR E NUNCA É MOSTRADA DE VOLTA. NINGUÉM MAIS USA A SUA CHAVE.
          </small>
        </div>
      </div>

      {/* ── Modelo ────────────────────────────────────────── */}
      <div className="panel">
        <div className="panel-hd">
          <span className="ttl">MODELO</span>
          <span className="meta">
            PREÇO DE TABELA EM US$ POR MILHÃO DE TOKENS · REFERÊNCIA {formatBRDate(data.prices_as_of)}
          </span>
        </div>
        <div className="panel-bd ia-models">
          {data.catalog.map((m) => {
            const unavailable = restricted && !data.available_models.includes(m.id);
            const est = estimates.find((e) => e.model.id === m.id);
            return (
              <label
                key={m.id}
                className={
                  "ia-model" + (model === m.id ? " ia-model--on" : "") + (unavailable ? " ia-model--off" : "")
                }
              >
                <input
                  type="radio"
                  name="ia-model"
                  value={m.id}
                  checked={model === m.id}
                  onChange={() => setModel(m.id)}
                />
                <div className="ia-model-hd">
                  <span className="ia-model-name">
                    <Sparkles size={13} /> {m.name.toUpperCase()}
                  </span>
                  {m.recommended && <span className="ia-badge ia-badge--rec">RECOMENDADO</span>}
                  {unavailable && <span className="ia-badge ia-badge--warn">NÃO LIBERADO NA CHAVE</span>}
                </div>
                <div className="ia-model-desc">{m.description}</div>
                <div className="ia-model-prices">
                  <span>
                    ENTRADA <b>{USD.format(m.input_usd_per_mtok)}</b>
                  </span>
                  <span>
                    SAÍDA <b>{USD.format(m.output_usd_per_mtok)}</b>
                  </span>
                  <span>CONTEXTO {m.context}</span>
                </div>
                {est && (
                  <div className="ia-model-est muted">
                    ≈ {BRL.format(est.monthly * usage.usd_brl)}/MÊS NO SEU PERFIL
                  </div>
                )}
              </label>
            );
          })}
        </div>
      </div>

      {/* ── Estimativa ────────────────────────────────────── */}
      <div className="panel">
        <div className="panel-hd">
          <span className="ttl">ESTIMATIVA DE CUSTO</span>
          <span className="meta">AJUSTE O PERFIL DE USO — O CÁLCULO É REFEITO NA HORA</span>
        </div>
        <div className="panel-bd">
          <div className="ia-usage">
            {USAGE_FIELDS.map((f) => (
              <label key={f.key} className="form-field">
                <span>{f.label}</span>
                <input
                  type="number"
                  inputMode="decimal"
                  min={f.min}
                  max={f.max}
                  step={f.step}
                  value={Number.isFinite(usage[f.key]) ? usage[f.key] : ""}
                  onChange={(e) => setUsage({ ...usage, [f.key]: e.target.valueAsNumber })}
                />
                <small className="muted">{f.hint}</small>
              </label>
            ))}
          </div>

          {selected && usageValid && (
            <div className="grid-kpi ia-kpis">
              <div className="panel kpi">
                <div className="kpi-lbl">POR REQUISIÇÃO</div>
                <div className="kpi-val">{BRL.format(selected.perRequest * usage.usd_brl)}</div>
                <div className="kpi-trend">{USD_FINE.format(selected.perRequest)}</div>
              </div>
              <div className="panel kpi">
                <div className="kpi-lbl">REQUISIÇÕES / MÊS</div>
                <div className="kpi-val">{INT.format(usage.requests_per_day * usage.work_days_per_month)}</div>
                <div className="kpi-trend">
                  {INT.format(usage.requests_per_day)} × {usage.work_days_per_month} DIAS
                </div>
              </div>
              <div className="panel kpi">
                <div className="kpi-lbl">CUSTO MENSAL</div>
                <div className="kpi-val">{BRL.format(selected.monthly * usage.usd_brl)}</div>
                <div className="kpi-trend">{USD.format(selected.monthly)}</div>
              </div>
              <div className="panel kpi">
                <div className="kpi-lbl">CUSTO ANUAL</div>
                <div className="kpi-val">{BRL.format(selected.annual * usage.usd_brl)}</div>
                <div className="kpi-trend">{USD.format(selected.annual)}</div>
              </div>
            </div>
          )}
          {!usageValid && (
            <div className="banner banner-warn" style={{ marginTop: 10 }}>
              ⚠ CONFIRA O PERFIL DE USO — HÁ VALOR VAZIO OU FORA DO LIMITE.
            </div>
          )}

          {usageValid && (
            <table className="ia-compare">
              <thead>
                <tr>
                  <th>MODELO</th>
                  <th className="num">POR REQ.</th>
                  <th className="num">MENSAL</th>
                  <th className="num">ANUAL</th>
                  <th className="num">ANUAL (US$)</th>
                </tr>
              </thead>
              <tbody>
                {estimates.map((e) => (
                  <tr
                    key={e.model.id}
                    className={e.model.id === model ? "ia-compare-on" : undefined}
                    onClick={() => setModel(e.model.id)}
                  >
                    <td>
                      {e.model.name.toUpperCase()}
                      {e.model.id === model && <span className="ia-badge ia-badge--rec">ESCOLHIDO</span>}
                    </td>
                    <td className="num">{BRL.format(e.perRequest * usage.usd_brl)}</td>
                    <td className="num">{BRL.format(e.monthly * usage.usd_brl)}</td>
                    <td className="num">{BRL.format(e.annual * usage.usd_brl)}</td>
                    <td className="num muted">{USD.format(e.annual)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          <small className="muted ia-notes">
            // ESTIMATIVA PELO PREÇO DE TABELA, SEM CACHE DE PROMPT NEM PROCESSAMENTO EM LOTE (QUE BARATEIAM).
            O RACIOCÍNIO DO MODELO É COBRADO COMO SAÍDA. NÃO INCLUI IOF NEM IMPOSTOS DO CARTÃO. O VALOR REAL
            APARECE NO CONSOLE DA ANTHROPIC.
          </small>
        </div>
      </div>

      <div className="ia-footer">
        {saveMsg && (
          <span className={saveMsg.kind === "ok" ? "ia-save-ok" : "ia-save-err"}>
            {saveMsg.kind === "ok" ? "✓" : "⚠"} {saveMsg.text}
          </span>
        )}
        <button
          type="button"
          className="btn btn-primary"
          onClick={onSaveSettings}
          disabled={!dirty || !usageValid || saving}
        >
          {saving ? "SALVANDO…" : "SALVAR MODELO E PERFIL"}
        </button>
      </div>
    </div>
  );
}
