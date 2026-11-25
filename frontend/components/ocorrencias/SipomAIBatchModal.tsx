"use client";

import { useEffect, useRef, useState } from "react";
import { Sparkles, Square, X } from "lucide-react";
import {
  applySipomAISuggestions,
  getSipomAIAnalysis,
  hasOpenSuggestions,
  recordSipomAIResult,
  reviewResult,
  selectable,
  suggestSipomAI,
  type SipomAIAnalysis,
  type SipomAIResult,
  type SipomAISuggestion,
} from "@/lib/sipom-ai-api";
import { formatBR } from "@/lib/format";
import {
  SelectAllBox,
  SipomAISuggestionList,
  SuggestionHeader,
  preselectIdx,
  selectableIdx,
} from "../operacional/SipomAIPanel";
import { TOPIC_OF_KIND } from "@/lib/ai-guidance-api";
import type { ApiError } from "@/lib/api";
import GuidanceModal from "../ia/GuidanceModal";

/** Ocorrência pendente que a IA pode ajudar a resolver. */
export type AIBatchItem = {
  id: string;
  ficha: string;
  date: string;
  natures: string[];
  /** Pendências que a IA ajuda a resolver, em texto curto. */
  pendencias: string[];
};

type Props = {
  items: AIBatchItem[];
  onClose: () => void;
  /** Depois de aplicar algo: a lista precisa recarregar. */
  onDone: () => void;
};

// Consultas simultâneas à IA: rápido sem estourar o limite da conta.
const PARALLEL = 3;

type Phase = "intro" | "asking" | "review" | "applying" | "done";
type Row = {
  status: "fila" | "consultando" | "ok" | "vazia" | "erro" | "aplicada";
  info?: string;
  suggestions: SipomAISuggestion[];
  checked: Set<number>;
  errors: Map<number, string>;
  cidade?: { id: number; nome: string } | null;
  /** A análise guardada desta ficha. */
  analysis?: SipomAIAnalysis | null;
  /** Veio da análise guardada (sem consultar a IA agora). */
  saved?: boolean;
};

const empty = (): Row => ({ status: "fila", suggestions: [], checked: new Set(), errors: new Map() });

/**
 * RESOLVER COM IA em lote: consulta a IA para cada ocorrência pendente, mostra
 * as sugestões agrupadas por ficha e aplica só as que o analista mantiver
 * marcadas — pelos mesmos endpoints da correção na ficha.
 */
export default function SipomAIBatchModal({ items, onClose, onDone }: Props) {
  const [phase, setPhase] = useState<Phase>("intro");
  // Ocorrências escolhidas para a análise (todas, de início).
  const [chosen, setChosen] = useState<Set<string>>(() => new Set(items.map((it) => it.id)));
  const queue = items.filter((it) => chosen.has(it.id));
  const [rows, setRows] = useState<Record<string, Row>>(() => Object.fromEntries(items.map((it) => [it.id, empty()])));
  const [fatal, setFatal] = useState<string | null>(null);
  const [cost, setCost] = useState(0);
  const [applied, setApplied] = useState(0);
  const [teach, setTeach] = useState<{ occId: string; s: SipomAISuggestion } | null>(null);
  // Refazer as fichas que já têm análise guardada (a IA usa a anterior).
  const [redo, setRedo] = useState(false);
  const [taught, setTaught] = useState(0);
  const stopRef = useRef(false);
  const busy = phase === "asking" || phase === "applying";

  useEffect(() => {
    if (!busy) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [busy]);

  const patch = (id: string, p: Partial<Row>) => setRows((cur) => ({ ...cur, [id]: { ...cur[id], ...p } }));

  async function ask() {
    stopRef.current = false;
    setFatal(null);
    setPhase("asking");
    const todo = queue;
    let next = 0;
    const worker = async () => {
      while (!stopRef.current && next < todo.length) {
        const it = todo[next++];
        patch(it.id, { status: "consultando" });
        try {
          // A análise guardada vale sem nova consulta quando ainda é da ficha
          // como está e tem o que aplicar. Toda aplicada e a ficha ainda
          // pendente: nova verificação (a IA parte da anterior).
          let r: SipomAIResult | null = null;
          let saved = false;
          if (!redo) {
            const g = await getSipomAIAnalysis(it.id).catch(() => null);
            if (g?.analysis && !g.analysis.stale && hasOpenSuggestions(g)) {
              r = g;
              saved = true;
            }
          }
          if (!r) {
            r = await suggestSipomAI(it.id);
            setCost((c) => c + (r!.cost_usd ?? 0));
          }
          const st = r.analysis?.status ?? {};
          patch(it.id, {
            status: r.suggestions.length ? "ok" : "vazia",
            suggestions: r.suggestions,
            checked: preselectIdx(r.suggestions, st),
            errors: new Map(Object.entries(r.analysis?.errors ?? {}).map(([k, v]) => [Number(k), v])),
            cidade: r.cidade ?? null,
            analysis: r.analysis ?? null,
            saved,
          });
        } catch (e) {
          const err = e as ApiError;
          patch(it.id, { status: "erro", info: err.message || "erro" });
          // Sem chave, chave recusada ou sem permissão: nenhuma outra vai dar certo.
          if (err.status === 412 || err.status === 403) {
            setFatal(err.message);
            stopRef.current = true;
          }
        }
      }
    };
    await Promise.all(Array.from({ length: PARALLEL }, worker));
    setPhase("review");
  }

  async function apply() {
    stopRef.current = false;
    setPhase("applying");
    let total = 0;
    for (const it of queue) {
      if (stopRef.current) break;
      const row = rows[it.id];
      if (!row || row.checked.size === 0) continue;
      const idx = [...row.checked].sort((a, b) => a - b);
      const { failed } = await applySipomAISuggestions(
        it.id,
        idx.map((i) => row.suggestions[i]),
      );
      total += idx.length - failed.length;
      const errors = new Map<number, string>();
      failed.forEach((f) => errors.set(row.suggestions.indexOf(f.s), f.error));
      // O que foi feito fica na análise: a próxima verificação usa.
      let analysis = row.analysis ?? null;
      if (analysis) {
        try {
          analysis = (
            await recordSipomAIResult(it.id, analysis.id, reviewResult(row.suggestions, idx, failed, analysis.status))
          ).analysis;
        } catch {
          // Auxiliar: as correções já foram gravadas.
        }
      }
      patch(it.id, {
        status: failed.length ? "erro" : "aplicada",
        info: failed.length ? `${failed.length} não aplicada(s)` : `${idx.length} aplicada(s)`,
        checked: new Set(),
        errors,
        analysis,
      });
    }
    setApplied(total);
    setPhase("done");
  }

  const list = (phase === "intro" ? items : queue).map((it) => ({ it, row: rows[it.id] }));
  const toggleChosen = (id: string, on: boolean) =>
    setChosen((cur) => {
      const n = new Set(cur);
      if (on) n.add(id);
      else n.delete(id);
      return n;
    });
  const asked = list.filter((x) => !["fila", "consultando"].includes(x.row.status)).length;
  const selected = list.reduce((n, x) => n + x.row.checked.size, 0);
  const withSuggestions = list.filter((x) => x.row.suggestions.length > 0);
  const suggestionCount = withSuggestions.reduce((n, x) => n + x.row.suggestions.length, 0);
  const selectableTotal = withSuggestions.reduce(
    (n, x) => n + selectableIdx(x.row.suggestions, x.row.analysis?.status).length,
    0,
  );
  const savedCount = list.filter((x) => x.row.saved).length;
  const reviewing = phase === "review";
  // Seleção em massa: todas as aplicáveis, só as de alta confiança, nenhuma.
  const selectWhere = (pick: (s: SipomAISuggestion) => boolean) =>
    setRows((cur) => {
      const next = { ...cur };
      for (const { it, row } of withSuggestions) {
        const st = row.analysis?.status ?? {};
        next[it.id] = {
          ...row,
          checked: new Set(row.suggestions.flatMap((s, i) => (selectable(s, st[String(i)]) && pick(s) ? [i] : []))),
        };
      }
      return next;
    });

  const close = () => {
    if (busy) return;
    if (applied > 0) onDone();
    onClose();
  };

  return (
    <div className="modal-backdrop" onClick={close}>
      <div className="modal modal--wide sai-batch" onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span>
            <Sparkles size={13} strokeWidth={1.8} style={{ verticalAlign: -2 }} /> RESOLVER PENDÊNCIAS COM IA ·{" "}
            {phase === "intro" ? `${chosen.size} DE ${items.length}` : queue.length} OCORRÊNCIA(S)
          </span>
          <button type="button" className="action-btn" onClick={close} aria-label="Fechar" disabled={busy}>
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd">
          {phase === "intro" && (
            <div className="banner banner-info">
              A IA LÊ CADA FICHA E SUGERE NATUREZA, BAIRRO, SEXO DOS ENVOLVIDOS E A TRADUÇÃO DAS APREENSÕES — SÓ ENTRE AS
              OPÇÕES DO CATÁLOGO DO SIPOM. NADA É GRAVADO ANTES DE VOCÊ REVISAR. O CUSTO VAI PARA A SUA CONTA ANTHROPIC.
              FICHA JÁ ANALISADA (E SEM MUDANÇA DESDE ENTÃO) REAPROVEITA A ANÁLISE SALVA, SEM CUSTO. ESCOLHA ABAIXO AS
              OCORRÊNCIAS QUE A IA VAI ANALISAR.
            </div>
          )}
          {phase === "intro" && (
            <div className="sai-batch-pick">
              <label className="check">
                <input
                  type="checkbox"
                  checked={chosen.size === items.length}
                  ref={(el) => {
                    if (el) el.indeterminate = chosen.size > 0 && chosen.size < items.length;
                  }}
                  onChange={(e) => setChosen(e.target.checked ? new Set(items.map((it) => it.id)) : new Set())}
                />
                {chosen.size === items.length ? "TODAS SELECIONADAS" : `${chosen.size} DE ${items.length} SELECIONADA(S)`}
              </label>
              <label className="check sai-batch-redo">
                <input type="checkbox" checked={redo} onChange={(e) => setRedo(e.target.checked)} />
                REFAZER AS QUE JÁ TÊM ANÁLISE SALVA (A IA USA A ANTERIOR PARA MELHORAR)
              </label>
            </div>
          )}
          {phase !== "intro" && (
            <div className="banner banner-info" role="status">
              {phase === "asking" && `CONSULTANDO A IA… ${asked} DE ${queue.length}`}
              {phase === "review" &&
                "REVISE: À ESQUERDA COMO ESTÁ, À DIREITA O QUE A IA SUGERE. SÓ O QUE ESTIVER MARCADO É GRAVADO · ALTA CONFIANÇA JÁ VEM MARCADA"}
              {phase === "applying" && "APLICANDO…"}
              {phase === "done" && `CONCLUÍDO · ${applied} SUGESTÃO(ÕES) APLICADA(S)`}
              {savedCount > 0 && ` · ${savedCount} ANÁLISE(S) SALVA(S) REAPROVEITADA(S), SEM CUSTO`}
              {cost > 0 && ` · CUSTO DA CONSULTA: US$ ${cost.toFixed(4)}`}
            </div>
          )}
          {fatal && <div className="banner banner-error">⚠ {fatal}</div>}
          {taught > 0 && phase === "review" && (
            <div className="banner banner-info">
              ✓ {taught} ORIENTAÇÃO(ÕES) SALVA(S) — VALEM PARA VOCÊ NAS PRÓXIMAS CONSULTAS (NOVO LOTE OU NA FICHA).
            </div>
          )}

          {phase !== "intro" && suggestionCount > 0 && (
            <div className="sai-batch-bar">
              <SelectAllBox
                total={selectableTotal}
                checked={selected}
                disabled={!reviewing}
                label={selected === selectableTotal && selected > 0 ? "TODAS SELECIONADAS" : "SELECIONAR TODAS"}
                onChange={(on) => selectWhere(() => on)}
              />
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                disabled={!reviewing}
                onClick={() => selectWhere((s) => s.confianca === "alta" || !!s.agrees)}
              >
                SÓ ALTA CONFIANÇA
              </button>
              <button type="button" className="btn btn-ghost btn-sm" disabled={!reviewing || selected === 0} onClick={() => selectWhere(() => false)}>
                LIMPAR
              </button>
              <span className="sai-batch-count">
                <b>{selected}</b> DE {selectableTotal} MARCADA(S) · {suggestionCount} SUGESTÃO(ÕES) EM {withSuggestions.length} FICHA(S)
              </span>
            </div>
          )}

          <div className="sai-batch-list">
            {phase !== "intro" && suggestionCount > 0 && (
              <div className="sai-batch-colhd">
                <SuggestionHeader />
              </div>
            )}
            {list.map(({ it, row }) => (
              <section key={it.id} className={"sai-batch-occ" + (row.suggestions.length ? "" : " sai-batch-occ--quiet")}>
                <div className="sai-batch-hd">
                  {phase !== "intro" && row.suggestions.length > 0 && (
                    <SelectAllBox
                      total={selectableIdx(row.suggestions, row.analysis?.status).length}
                      checked={row.checked.size}
                      disabled={!reviewing}
                      onChange={(on) =>
                        patch(it.id, {
                          checked: on ? new Set(selectableIdx(row.suggestions, row.analysis?.status)) : new Set(),
                        })
                      }
                    />
                  )}
                  {phase === "intro" && (
                    <input
                      type="checkbox"
                      checked={chosen.has(it.id)}
                      onChange={(e) => toggleChosen(it.id, e.target.checked)}
                      aria-label={`Analisar a ficha ${it.ficha}`}
                    />
                  )}
                  <span className="mono">{it.ficha || "—"}</span>
                  <span className="muted">{it.date}</span>
                  <span className="sai-batch-nat">{it.natures.join(" · ")}</span>
                  {row.suggestions.length > 0 && (
                    <span className="sai-batch-n">
                      {row.checked.size}/{selectableIdx(row.suggestions, row.analysis?.status).length}
                    </span>
                  )}
                  {row.saved && row.analysis && (
                    <span className="sai-batch-saved" title={`Análise de ${row.analysis.author}`}>
                      SALVA · {formatBR(row.analysis.created_at)}
                    </span>
                  )}
                  <span className={"pill " + PILL[row.status]}>{LABEL[row.status]}</span>
                </div>
                {phase === "intro" && it.pendencias.length > 0 && (
                  <div className="sai-batch-pend">PENDÊNCIAS: {it.pendencias.join(" · ")}</div>
                )}
                {row.info && <div className={row.status === "erro" ? "sai-err" : "muted"}>{row.info}</div>}
                {row.suggestions.length > 0 && (
                  <SipomAISuggestionList
                    header={false}
                    items={row.suggestions}
                    status={row.analysis?.status}
                    checked={row.checked}
                    errors={row.errors}
                    disabled={phase !== "review"}
                    onTeach={phase === "review" ? (s) => setTeach({ occId: it.id, s }) : undefined}
                    onToggle={(i, on) => {
                      const n = new Set(row.checked);
                      if (on) n.add(i);
                      else n.delete(i);
                      patch(it.id, { checked: n });
                    }}
                  />
                )}
              </section>
            ))}
          </div>
        </div>
        {teach && (
          <GuidanceModal
            topic={TOPIC_OF_KIND[teach.s.kind] ?? "geral"}
            cidade={rows[teach.occId]?.cidade ?? null}
            occurrenceId={teach.occId}
            context={`${teach.s.target} → ${teach.s.value}`}
            onClose={() => setTeach(null)}
            onSaved={() => {
              setTeach(null);
              setTaught((n) => n + 1);
            }}
          />
        )}
        <div className="modal-ft">
          {busy ? (
            <button type="button" className="btn" onClick={() => (stopRef.current = true)}>
              <Square size={12} strokeWidth={2} /> PARAR
            </button>
          ) : phase === "intro" ? (
            <>
              <button type="button" className="btn btn-ghost" onClick={close}>
                CANCELAR
              </button>
              <button type="button" className="btn btn-primary" onClick={ask} disabled={chosen.size === 0}>
                <Sparkles size={12} strokeWidth={2} /> ANALISAR {chosen.size} OCORRÊNCIA(S)
              </button>
            </>
          ) : phase === "review" ? (
            <>
              <button type="button" className="btn btn-ghost" onClick={close}>
                DESCARTAR
              </button>
              <button type="button" className="btn btn-primary" onClick={apply} disabled={selected === 0}>
                APLICAR SELECIONADAS ({selected})
              </button>
            </>
          ) : (
            <button type="button" className="btn btn-primary" onClick={close}>
              FECHAR
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

const LABEL: Record<Row["status"], string> = {
  fila: "NA FILA",
  consultando: "CONSULTANDO",
  ok: "COM SUGESTÃO",
  vazia: "SEM SUGESTÃO",
  erro: "ERRO",
  aplicada: "APLICADA",
};

const PILL: Record<Row["status"], string> = {
  fila: "cold",
  consultando: "info",
  ok: "hold",
  vazia: "cold",
  erro: "crit",
  aplicada: "active",
};
