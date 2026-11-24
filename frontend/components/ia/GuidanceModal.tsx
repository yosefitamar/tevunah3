"use client";

import { useState } from "react";
import { createPortal } from "react-dom";
import { GraduationCap, X } from "lucide-react";
import {
  GUIDANCE_TOPIC_LABEL,
  createGuidance,
  updateGuidance,
  type Guidance,
  type GuidanceTopic,
} from "@/lib/ai-guidance-api";
import type { ApiError } from "@/lib/api";
import Select from "../shared/Select";

type Props = {
  /** Editar uma orientação pendente; sem isto, cria. */
  editing?: Guidance;
  topic: GuidanceTopic;
  /** Cidade da ocorrência: a orientação pode valer só nela. */
  cidade?: { id: number; nome: string } | null;
  occurrenceId?: string;
  /** O caso que motivou a orientação ("Bairro (ZONA RURAL) → nenhum bairro serve"). */
  context?: string;
  onClose: () => void;
  onSaved: (g: Guidance) => void;
};

const EXAMPLES: Record<GuidanceTopic, string> = {
  bairro: "Ex.: Em Cascavel, \"Zona Rural\" e as localidades rurais vão como bairro CENTRO.",
  natureza: "Ex.: Resistência sem outro crime na ficha vai como CONTRAVENÇÃO PENAL.",
  sexo: "Ex.: \"Nacional\" sem outro indício não define o sexo; use o nome.",
  apreensoes: "Ex.: \"Trouxinha\" de maconha conta como uma unidade de 1 g quando o peso não vier.",
  geral: "Ex.: Ficha da Operação X sempre se refere ao 2º BPRAIO.",
};

/**
 * ENSINAR A IA: o analista escreve, em português, como a IA deve agir num
 * tipo de caso. Vale para ele na hora; para todos, depois que o
 * administrador aprovar.
 */
export default function GuidanceModal({ editing, topic: topic0, cidade, occurrenceId, context, onClose, onSaved }: Props) {
  const [topic, setTopic] = useState<GuidanceTopic>(editing?.topic ?? topic0);
  const cityOption = editing?.cidade_id ? { id: editing.cidade_id, nome: editing.cidade } : cidade ?? null;
  // Bairro é sempre da cidade; os outros temas costumam valer para todas.
  const [onlyCity, setOnlyCity] = useState(editing ? editing.cidade_id != null : topic0 === "bairro" && !!cidade);
  const [body, setBody] = useState(editing?.body ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setBusy(true);
    setError(null);
    const input = {
      topic,
      cidade_id: onlyCity && cityOption ? cityOption.id : null,
      body,
      occurrence_id: occurrenceId,
    };
    try {
      const r = editing ? await updateGuidance(editing.id, input) : await createGuidance(input);
      onSaved(r.item);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao salvar");
      setBusy(false);
    }
  }

  const len = body.trim().length;
  const node = (
    <div className="modal-backdrop dup-resolver-backdrop" onClick={(e) => e.stopPropagation()}>
      <div className="modal" style={{ width: 560 }} onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span>
            <GraduationCap size={14} style={{ verticalAlign: -2 }} /> {editing ? "EDITAR ORIENTAÇÃO" : "ENSINAR A IA"}
          </span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar" disabled={busy}>
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd guid-form">
          {context && (
            <div className="guid-context">
              <span className="muted">CASO:</span> {context}
            </div>
          )}
          <div className="form-grid-2">
            <label className="form-field">
              <span>TEMA</span>
              <Select
                value={topic}
                onChange={(v) => setTopic(v as GuidanceTopic)}
                options={(Object.keys(GUIDANCE_TOPIC_LABEL) as GuidanceTopic[]).map((k) => ({
                  value: k,
                  label: GUIDANCE_TOPIC_LABEL[k],
                }))}
                disabled={busy}
              />
            </label>
            <div className="form-field">
              <span>ONDE VALE</span>
              {cityOption ? (
                <label className="check guid-city">
                  <input type="checkbox" checked={onlyCity} onChange={(e) => setOnlyCity(e.target.checked)} disabled={busy} />
                  SÓ EM {cityOption.nome.toUpperCase()}
                </label>
              ) : (
                <div className="muted guid-city">TODAS AS CIDADES</div>
              )}
            </div>
          </div>
          <label className="form-field">
            <span>COMO A IA DEVE AGIR</span>
            <textarea
              rows={5}
              value={body}
              maxLength={1000}
              onChange={(e) => setBody(e.target.value)}
              placeholder={EXAMPLES[topic]}
              disabled={busy}
              autoFocus
            />
          </label>
          <div className="muted guid-hint">
            A IA continua escolhendo só opções do catálogo do SIPOM — a orientação diz qual usar. Vale para você
            imediatamente; para os demais analistas, depois da aprovação de um administrador. · {len}/1000
          </div>
          {error && <div className="banner banner-error">⚠ {error}</div>}
        </div>
        <div className="modal-ft">
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
            CANCELAR
          </button>
          <button type="button" className="btn btn-primary" onClick={save} disabled={busy || len < 5}>
            {busy ? "SALVANDO…" : "SALVAR ORIENTAÇÃO"}
          </button>
        </div>
      </div>
    </div>
  );
  return typeof document === "undefined" ? node : createPortal(node, document.body);
}
