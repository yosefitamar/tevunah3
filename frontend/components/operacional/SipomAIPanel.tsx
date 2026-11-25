"use client";

import { useEffect, useState } from "react";
import { Check, GraduationCap, RefreshCw, Sparkles, X } from "lucide-react";
import {
  getOpsOccurrence,
  type OpsOccurrence,
  type OpsOfficer,
  type SipomChangeExtra,
  type SipomOccurrence,
} from "@/lib/ops-reports-api";
import {
  CONFIANCA_LABEL,
  applySipomAISuggestions,
  changesReport,
  getSipomAIAnalysis,
  hasOpenSuggestions,
  preselect,
  recordSipomAIResult,
  reviewResult,
  selectable,
  suggestSipomAI,
  type SipomAIAnalysis,
  type SipomAIStatus,
  type SipomAIResult,
  type SipomAISuggestion,
} from "@/lib/sipom-ai-api";
import { TOPIC_OF_KIND } from "@/lib/ai-guidance-api";
import type { ApiError } from "@/lib/api";
import GuidanceModal from "../ia/GuidanceModal";
import { formatBR } from "@/lib/format";

const CONF_PILL: Record<SipomAISuggestion["confianca"], string> = { alta: "active", media: "hold", baixa: "cold" };

/** "Envolvido: FULANO" → ["ENVOLVIDO", "FULANO"]; sem dois-pontos, tudo é o rótulo. */
function splitTarget(t: string): [string, string] {
  const i = t.indexOf(": ");
  return i < 0 ? [t, ""] : [t.slice(0, i), t.slice(i + 2)];
}

/** Índices das sugestões que podem ser marcadas (aplicáveis e ainda não aplicadas). */
export function selectableIdx(items: SipomAISuggestion[], status: Record<string, SipomAIStatus> = {}) {
  return items.flatMap((s, i) => (selectable(s, status[String(i)]) ? [i] : []));
}

/** Marcadas de início: as de alta confiança ainda não aplicadas nem recusadas. */
export function preselectIdx(items: SipomAISuggestion[], status: Record<string, SipomAIStatus> = {}) {
  return new Set(items.flatMap((s, i) => (preselect(s, status[String(i)]) ? [i] : [])));
}

/** "ANÁLISE DE 25/11/2026 14:02 · FULANO · US$ 0,0123 · REFINADA" */
export function AnalysisInfo({ a }: { a: SipomAIAnalysis }) {
  return (
    <span className="sai-info">
      ANÁLISE SALVA DE {formatBR(a.created_at)} · {(a.author || "—").toUpperCase()}
      {a.cost_usd > 0 && ` · US$ ${a.cost_usd.toFixed(4)}`}
      {a.refined && " · REFINADA A PARTIR DA ANTERIOR"}
    </span>
  );
}

/**
 * Caixa "selecionar todas" de um grupo: marcada quando todas as aplicáveis
 * estão marcadas, indeterminada quando só parte.
 */
export function SelectAllBox({
  total,
  checked,
  onChange,
  disabled,
  label,
}: {
  total: number;
  checked: number;
  onChange: (on: boolean) => void;
  disabled?: boolean;
  label?: string;
}) {
  return (
    <label className={"sai-all" + (disabled || total === 0 ? " sai-all--off" : "")}>
      <input
        type="checkbox"
        checked={total > 0 && checked === total}
        ref={(el) => {
          if (el) el.indeterminate = checked > 0 && checked < total;
        }}
        disabled={disabled || total === 0}
        onChange={(e) => onChange(e.target.checked)}
      />
      {label && <span>{label}</span>}
    </label>
  );
}

/**
 * Sugestões da IA em grade: o campo, como está hoje e o que a IA sugere, lado
 * a lado, com a confiança e o porquê embaixo. Só a aplicável pode ser
 * marcada; a outra fica à vista para o analista saber o que a IA não resolveu.
 */
export function SipomAISuggestionList({
  items,
  checked,
  onToggle,
  disabled,
  errors,
  onTeach,
  header = true,
  status = {},
}: {
  items: SipomAISuggestion[];
  checked: Set<number>;
  onToggle: (i: number, on: boolean) => void;
  disabled?: boolean;
  /** Erro ao aplicar, pela posição na lista. */
  errors?: Map<number, string>;
  /** ENSINAR A IA a partir desta sugestão. */
  onTeach?: (s: SipomAISuggestion) => void;
  /** Linha de títulos das colunas (o lote põe uma só, no topo). */
  header?: boolean;
  /** Situação na análise guardada, pela posição. */
  status?: Record<string, SipomAIStatus>;
}) {
  // As já aplicadas ficam recolhidas: não há o que fazer com elas.
  const [showDone, setShowDone] = useState(false);
  const doneCount = items.filter((_, i) => status[String(i)] === "aplicada").length;
  return (
    <div className="sai-grid" role="table">
      {header && <SuggestionHeader />}
      {items.map((s, i) => {
        if (status[String(i)] === "aplicada" && !showDone) return null;
        const [lbl, detail] = splitTarget(s.target);
        const on = checked.has(i);
        const st = status[String(i)];
        const done = st === "aplicada";
        return (
          <div
            key={i}
            role="row"
            className={
              "sai-row" +
              (done ? " sai-row--done" : s.applicable ? "" : " sai-row--off") +
              (on ? " sai-row--on" : "")
            }
          >
            <label className="sai-cell sai-cell--check">
              <input
                type="checkbox"
                checked={on}
                disabled={disabled || !selectable(s, st)}
                onChange={(e) => onToggle(i, e.target.checked)}
                aria-label={`Aplicar: ${s.target} → ${s.value}`}
              />
            </label>
            <div className="sai-cell sai-field">
              <span className="sai-field-lbl">{lbl}</span>
              {detail && <span className="sai-field-detail">{detail}</span>}
            </div>
            <div className="sai-cell sai-now">
              {s.agrees ? (
                <span className="sai-keep">MANTÉM O ATUAL</span>
              ) : s.current ? (
                s.current
              ) : (
                <span className="sai-none">—</span>
              )}
            </div>
            <div className="sai-cell sai-to" aria-hidden>
              →
            </div>
            <div className="sai-cell sai-new">
              <span className="sai-new-val">{s.value}</span>
              {done && (
                <span className="sai-st sai-st--done">
                  <Check size={10} strokeWidth={2.5} /> APLICADA
                </span>
              )}
              {st === "recusada" && <span className="sai-st">VOCÊ NÃO ACEITOU NA REVISÃO ANTERIOR</span>}
              {!done && !s.applicable && <span className="sai-na">NÃO APLICÁVEL · RESOLVA NA FICHA</span>}
            </div>
            <div className="sai-cell sai-conf">
              <span className={"pill " + CONF_PILL[s.confianca]}>{CONFIANCA_LABEL[s.confianca]}</span>
            </div>
            <div className="sai-cell sai-act">
              {onTeach && (
                <button
                  type="button"
                  className="ops-field-btn"
                  onClick={() => onTeach(s)}
                  disabled={disabled}
                  title="Escreva como a IA deve agir neste tipo de caso"
                >
                  <GraduationCap size={11} /> ENSINAR
                </button>
              )}
            </div>
            {(s.motivo || s.guidance || errors?.get(i)) && (
              <div className="sai-why">
                {s.motivo && <div className="sai-motivo">{s.motivo}</div>}
                {s.guidance && (
                  <div className="sai-guid">
                    ↳ SEGUIU A ORIENTAÇÃO DE {s.guidance.author.toUpperCase()} ({s.guidance.date})
                    {s.guidance.pending && " · AINDA SÓ SUA, AGUARDA APROVAÇÃO"}: “{s.guidance.body}”
                  </div>
                )}
                {errors?.get(i) && <div className="sai-err">⚠ {errors.get(i)}</div>}
              </div>
            )}
          </div>
        );
      })}
      {doneCount > 0 && (
        <button type="button" className="sai-done-toggle" onClick={() => setShowDone((v) => !v)}>
          <Check size={11} strokeWidth={2.5} /> {doneCount} JÁ APLICADA(S) NESTA ANÁLISE ·{" "}
          {showDone ? "OCULTAR" : "MOSTRAR"}
        </button>
      )}
    </div>
  );
}

/** Títulos das colunas da grade de sugestões. */
export function SuggestionHeader() {
  return (
    <div className="sai-row sai-row--hd" role="row">
      <span />
      <span>CAMPO</span>
      <span>COMO ESTÁ</span>
      <span />
      <span>SUGESTÃO DA IA</span>
      <span>CONFIANÇA</span>
      <span />
    </div>
  );
}

type Props = {
  occId: string;
  onApplied: (s: SipomOccurrence, officers?: OpsOfficer[], extra?: SipomChangeExtra) => void;
  /** Correção do texto do relatório (releitura do PDF): a ficha inteira mudou. */
  onOccurrence?: (occ: OpsOccurrence) => void;
};

/**
 * RESOLVER COM IA na ficha: pede as sugestões, o analista marca as que
 * aceita e aplica — cada uma pelo endpoint de correção de sempre.
 */
export default function SipomAIPanel({ occId, onApplied, onOccurrence }: Props) {
  const [items, setItems] = useState<SipomAISuggestion[] | null>(null);
  const [analysis, setAnalysis] = useState<SipomAIAnalysis | null>(null);
  const [checked, setChecked] = useState<Set<number>>(new Set());
  const [errors, setErrors] = useState<Map<number, string>>(new Map());
  const [busy, setBusy] = useState<null | "load" | "ask" | "apply">("load");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [cidade, setCidade] = useState<{ id: number; nome: string } | null>(null);
  const [teach, setTeach] = useState<SipomAISuggestion | null>(null);
  const [taught, setTaught] = useState(false);
  const [hidden, setHidden] = useState(false);
  const status = analysis?.status ?? {};

  function show(r: SipomAIResult) {
    const st = r.analysis?.status ?? {};
    setCidade(r.cidade ?? null);
    setAnalysis(r.analysis ?? null);
    setItems(r.suggestions);
    setChecked(preselectIdx(r.suggestions, st));
    setErrors(new Map(Object.entries(r.analysis?.errors ?? {}).map(([k, v]) => [Number(k), v])));
    setHidden(false);
  }

  // A análise guardada aparece sem consultar a IA de novo.
  useEffect(() => {
    let alive = true;
    getSipomAIAnalysis(occId)
      .then((r) => alive && r.analysis && show(r))
      .catch(() => undefined)
      .finally(() => alive && setBusy(null));
    return () => {
      alive = false;
    };
  }, [occId]);

  async function ask() {
    setBusy("ask");
    setError(null);
    setNotice(null);
    try {
      const r = await suggestSipomAI(occId);
      setTaught(false);
      show(r);
      if (r.suggestions.length === 0) setNotice("A IA não tem sugestão para as pendências desta ficha.");
    } catch (e) {
      setError((e as ApiError).message || "Erro ao consultar a IA");
    } finally {
      setBusy(null);
    }
  }

  async function apply() {
    if (!items) return;
    const idx = [...checked].sort((a, b) => a - b);
    setBusy("apply");
    setError(null);
    const { last, failed } = await applySipomAISuggestions(
      occId,
      idx.map((i) => items[i]),
    );
    // O que foi feito com a análise fica nela: a próxima verificação usa.
    if (analysis) {
      try {
        const r = await recordSipomAIResult(occId, analysis.id, reviewResult(items, idx, failed, status));
        setAnalysis(r.analysis);
      } catch {
        // O registro é auxiliar: as correções já foram gravadas.
      }
    }
    const okList = idx.map((i) => items[i]).filter((s) => !failed.some((f) => f.s === s));
    if (onOccurrence && okList.some(changesReport)) {
      // O texto mudou (endereço, data…): relê a ficha inteira, já recalculada.
      try {
        onOccurrence((await getOpsOccurrence(occId)).occurrence);
      } catch {
        if (last) onApplied(last.sipom, last.officers, last);
      }
    } else if (last) onApplied(last.sipom, last.officers, last);
    const nextErrors = new Map<number, string>();
    failed.forEach((f) => nextErrors.set(items.indexOf(f.s), f.error));
    setChecked(new Set());
    setErrors(nextErrors);
    setNotice(`${idx.length - failed.length} sugestão(ões) aplicada(s)` + (failed.length ? ` · ${failed.length} com erro` : "."));
    setBusy(null);
  }

  return (
    <div className="sai-panel">
      <div className="sai-hd">
        <Sparkles size={13} strokeWidth={1.8} />
        <span className="sai-ttl">ASSISTENTE DE IA</span>
        <span className="muted">SUGERE A CORREÇÃO DAS PENDÊNCIAS · VOCÊ CONFIRMA ANTES DE GRAVAR</span>
        {items && !hidden ? (
          <button type="button" className="action-btn sai-close" onClick={() => setHidden(true)} aria-label="Ocultar sugestões" disabled={!!busy}>
            <X size={13} />
          </button>
        ) : analysis ? (
          <button type="button" className="btn btn-sm sai-btn" onClick={() => setHidden(false)} disabled={!!busy}>
            <Sparkles size={12} strokeWidth={1.8} /> VER ANÁLISE SALVA
          </button>
        ) : (
          <button type="button" className="btn btn-sm sai-btn" onClick={ask} disabled={!!busy}>
            {busy === "ask" ? (
              <>
                <span className="spinner" aria-hidden /> CONSULTANDO…
              </>
            ) : busy === "load" ? (
              <>
                <span className="spinner" aria-hidden /> CARREGANDO…
              </>
            ) : (
              <>
                <Sparkles size={12} strokeWidth={1.8} /> RESOLVER COM IA
              </>
            )}
          </button>
        )}
      </div>
      {analysis && !hidden && (
        <div className="sai-saved">
          <AnalysisInfo a={analysis} />
          {analysis.stale && <span className="sai-stale">A FICHA MUDOU DEPOIS DESTA ANÁLISE</span>}
        </div>
      )}
      {error && <div className="banner banner-error">⚠ {error}</div>}
      {notice && <div className="banner banner-info">✓ {notice}</div>}
      {analysis && !hidden && items?.length === 0 && (
        <div className="sai-ft">
          <span className="muted sai-empty">A IA NÃO TEVE SUGESTÃO NESTA ANÁLISE.</span>
          <button type="button" className="btn btn-sm" onClick={ask} disabled={!!busy}>
            <RefreshCw size={12} /> {busy === "ask" ? "CONSULTANDO…" : "NOVA VERIFICAÇÃO"}
          </button>
        </div>
      )}
      {analysis && !hidden && items && items.length > 0 && !hasOpenSuggestions({ suggestions: items, analysis }) && (
        <div className="banner banner-info">
          AS SUGESTÕES DESTA ANÁLISE JÁ FORAM APLICADAS (OU NÃO SÃO APLICÁVEIS). SE A FICHA AINDA TEM PENDÊNCIA, PEÇA UMA
          NOVA VERIFICAÇÃO — A IA PARTE DESTA.
        </div>
      )}
      {items && items.length > 0 && !hidden && (
        <>
          {taught && (
            <div className="banner banner-info">
              ✓ ORIENTAÇÃO SALVA — JÁ VALE PARA VOCÊ. PEÇA UMA NOVA VERIFICAÇÃO PARA A IA REVER A FICHA COM ELA.
            </div>
          )}
          <SipomAISuggestionList
            items={items}
            status={status}
            checked={checked}
            errors={errors}
            disabled={!!busy}
            onTeach={setTeach}
            onToggle={(i, on) =>
              setChecked((cur) => {
                const n = new Set(cur);
                if (on) n.add(i);
                else n.delete(i);
                return n;
              })
            }
          />
          <div className="sai-ft">
            <SelectAllBox
              total={selectableIdx(items, status).length}
              checked={checked.size}
              disabled={!!busy}
              label="SELECIONAR TODAS"
              onChange={(on) => setChecked(on ? new Set(selectableIdx(items, status)) : new Set())}
            />
            <button
              type="button"
              className={
                "btn btn-sm" +
                (taught || analysis?.stale || !hasOpenSuggestions({ suggestions: items, analysis }) ? "" : " btn-ghost")
              }
              onClick={ask}
              disabled={!!busy}
              title="A IA refaz a análise usando esta (o que você aplicou, não aceitou e o que falhou)"
            >
              <RefreshCw size={12} /> {busy === "ask" ? "CONSULTANDO…" : "NOVA VERIFICAÇÃO"}
            </button>
            <button type="button" className="btn btn-primary btn-sm" onClick={apply} disabled={!!busy || checked.size === 0}>
              {busy === "apply" ? "APLICANDO…" : `APLICAR SELECIONADAS (${checked.size})`}
            </button>
          </div>
        </>
      )}
      {teach && (
        <GuidanceModal
          topic={TOPIC_OF_KIND[teach.kind] ?? "geral"}
          cidade={cidade}
          occurrenceId={occId}
          context={`${teach.target} → ${teach.value}`}
          onClose={() => setTeach(null)}
          onSaved={() => {
            setTeach(null);
            setTaught(true);
          }}
        />
      )}
    </div>
  );
}
