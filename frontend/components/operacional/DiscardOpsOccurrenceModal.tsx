"use client";

import { useState, type FormEvent } from "react";
import { Trash2, X } from "lucide-react";
import { discardOpsOccurrence, type OpsOccurrence } from "@/lib/ops-reports-api";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";

type Props = {
  occ: OpsOccurrence;
  onClose: () => void;
  onDiscarded: () => void;
};

/**
 * Descarte da ocorrência importada que não deveria estar no acervo. Pede o
 * motivo, que vai para a auditoria; a ficha CIOPS descartada não volta numa
 * reimportação.
 */
export default function DiscardOpsOccurrenceModal({ occ, onClose, onDiscarded }: Props) {
  const [motivo, setMotivo] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const ok = motivo.trim().length >= 5;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!ok || !occ.id) return;
    setBusy(true);
    setErr(null);
    try {
      await discardOpsOccurrence(occ.id, motivo.trim());
      onDiscarded();
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao descartar a ocorrência");
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={busy ? undefined : onClose}>
      <div className="modal ops-edit" onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span>
            <Trash2 size={13} strokeWidth={1.8} /> DESCARTAR OCORRÊNCIA · {occ.ciops_record || "SEM FICHA"}
          </span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar" disabled={busy}>
            <X size={14} />
          </button>
        </div>
        <form className="modal-form" onSubmit={onSubmit}>
          <div className="modal-bd ops-edit-bd">
            <div className="muted">
              {formatBRDate(occ.occurred_on)} · {occ.natures.join(" · ") || "—"} · {occ.place_city || "—"}
            </div>
            <div className="banner banner-warn">
              ⚠ A OCORRÊNCIA SAI DO ACERVO, DA FILA DO SIPOM E DOS DOSSIÊS. A REIMPORTAÇÃO DO RELATÓRIO NÃO A TRAZ DE
              VOLTA.
            </div>
            <label className="form-field">
              <span>MOTIVO DO DESCARTE</span>
              <textarea
                rows={3}
                maxLength={500}
                value={motivo}
                onChange={(e) => setMotivo(e.target.value)}
                placeholder="Ex.: ocorrência de outro batalhão; duplicada com outra ficha"
                autoFocus
              />
            </label>
            {err && <div className="banner banner-error">⚠ {err}</div>}
          </div>
          <div className="modal-ft ops-edit-ft">
            <span className="muted ops-edit-note">O MOTIVO VAI PARA A AUDITORIA</span>
            <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
              CANCELAR
            </button>
            <button type="submit" className="btn btn-danger" disabled={busy || !ok}>
              {busy ? "DESCARTANDO…" : "DESCARTAR"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
