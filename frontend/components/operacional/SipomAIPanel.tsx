"use client";

import { useState } from "react";
import { GraduationCap, Sparkles, X } from "lucide-react";
import type { OpsOfficer, SipomChangeExtra, SipomOccurrence } from "@/lib/ops-reports-api";
import {
  CONFIANCA_LABEL,
  applySipomAISuggestions,
  preselect,
  suggestSipomAI,
  type SipomAISuggestion,
} from "@/lib/sipom-ai-api";
import { TOPIC_OF_KIND } from "@/lib/ai-guidance-api";
import type { ApiError } from "@/lib/api";
import GuidanceModal from "../ia/GuidanceModal";

const CONF_PILL: Record<SipomAISuggestion["confianca"], string> = { alta: "active", media: "hold", baixa: "cold" };

/**
 * Sugestões da IA com caixa de seleção. Só a aplicável pode ser marcada; a
 * outra fica à vista para o analista saber o que a IA não resolveu.
 */
export function SipomAISuggestionList({
  items,
  checked,
  onToggle,
  disabled,
  errors,
  onTeach,
}: {
  items: SipomAISuggestion[];
  checked: Set<number>;
  onToggle: (i: number, on: boolean) => void;
  disabled?: boolean;
  /** Erro ao aplicar, pela posição na lista. */
  errors?: Map<number, string>;
  /** ENSINAR A IA a partir desta sugestão. */
  onTeach?: (s: SipomAISuggestion) => void;
}) {
  return (
    <ul className="sai-list">
      {items.map((s, i) => (
        <li key={i} className={"sai-item" + (s.applicable ? "" : " sai-item--off")}>
          <label className="sai-check">
            <input
              type="checkbox"
              checked={checked.has(i)}
              disabled={disabled || !s.applicable}
              onChange={(e) => onToggle(i, e.target.checked)}
            />
          </label>
          <div className="sai-body">
            <div className="sai-line">
              <span className="sai-target">{s.target}</span>
              <span className="sai-arrow">→</span>
              <span className="sai-value">{s.value}</span>
              {s.current && !s.agrees && <span className="muted sai-current">(hoje: {s.current})</span>}
              {s.agrees && <span className="sai-agree">CONFIRMA A SUGESTÃO</span>}
              <span className={"pill " + CONF_PILL[s.confianca]}>{CONFIANCA_LABEL[s.confianca]}</span>
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
            {s.motivo && <div className="sai-motivo">{s.motivo}</div>}
            {s.guidance && (
              <div className="sai-guid">
                ↳ SEGUIU A ORIENTAÇÃO DE {s.guidance.author.toUpperCase()} ({s.guidance.date})
                {s.guidance.pending && " · AINDA SÓ SUA, AGUARDA APROVAÇÃO"}: “{s.guidance.body}”
              </div>
            )}
            {errors?.get(i) && <div className="sai-err">⚠ {errors.get(i)}</div>}
          </div>
        </li>
      ))}
    </ul>
  );
}

type Props = {
  occId: string;
  onApplied: (s: SipomOccurrence, officers?: OpsOfficer[], extra?: SipomChangeExtra) => void;
};

/**
 * RESOLVER COM IA na ficha: pede as sugestões, o analista marca as que
 * aceita e aplica — cada uma pelo endpoint de correção de sempre.
 */
export default function SipomAIPanel({ occId, onApplied }: Props) {
  const [items, setItems] = useState<SipomAISuggestion[] | null>(null);
  const [checked, setChecked] = useState<Set<number>>(new Set());
  const [errors, setErrors] = useState<Map<number, string>>(new Map());
  const [busy, setBusy] = useState<null | "ask" | "apply">(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [cidade, setCidade] = useState<{ id: number; nome: string } | null>(null);
  const [teach, setTeach] = useState<SipomAISuggestion | null>(null);
  const [taught, setTaught] = useState(false);

  async function ask() {
    setBusy("ask");
    setError(null);
    setNotice(null);
    setErrors(new Map());
    try {
      const r = await suggestSipomAI(occId);
      setCidade(r.cidade ?? null);
      setTaught(false);
      setItems(r.suggestions);
      setChecked(new Set(r.suggestions.flatMap((s, i) => (preselect(s) ? [i] : []))));
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
    if (last) onApplied(last.sipom, last.officers, last);
    const failedIdx = new Map<number, string>();
    failed.forEach((f) => failedIdx.set(items.indexOf(f.s), f.error));
    // Ficam na lista as que falharam e as que o analista não marcou.
    const keep = items.map((_, i) => i).filter((i) => !checked.has(i) || failedIdx.has(i));
    const nextItems = keep.map((i) => items[i]);
    const nextErrors = new Map<number, string>();
    keep.forEach((i, k) => failedIdx.has(i) && nextErrors.set(k, failedIdx.get(i)!));
    setItems(nextItems.length ? nextItems : null);
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
        {items ? (
          <button type="button" className="action-btn sai-close" onClick={() => setItems(null)} aria-label="Descartar sugestões" disabled={!!busy}>
            <X size={13} />
          </button>
        ) : (
          <button type="button" className="btn btn-sm sai-btn" onClick={ask} disabled={!!busy}>
            {busy === "ask" ? (
              <>
                <span className="spinner" aria-hidden /> CONSULTANDO…
              </>
            ) : (
              <>
                <Sparkles size={12} strokeWidth={1.8} /> RESOLVER COM IA
              </>
            )}
          </button>
        )}
      </div>
      {error && <div className="banner banner-error">⚠ {error}</div>}
      {notice && <div className="banner banner-info">✓ {notice}</div>}
      {items && items.length > 0 && (
        <>
          {taught && (
            <div className="banner banner-info">
              ✓ ORIENTAÇÃO SALVA — JÁ VALE PARA VOCÊ. CLIQUE EM CONSULTAR DE NOVO PARA A IA REVER A FICHA COM ELA.
            </div>
          )}
          <SipomAISuggestionList
            items={items}
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
            <button type="button" className={"btn btn-sm" + (taught ? "" : " btn-ghost")} onClick={ask} disabled={!!busy}>
              CONSULTAR DE NOVO
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
