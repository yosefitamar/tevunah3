"use client";

import { useState, type FormEvent } from "react";
import { AlertTriangle, X } from "lucide-react";

type Props = {
  /** Número do relatório (exibido no aviso). */
  reportNumber?: string;
  onCancel: () => void;
  onConfirm: (reason: string) => Promise<void>;
};

const MIN_LEN = 5;
const MAX_LEN = 1000;

/**
 * Modal de confirmação para reverter a difusão de um RI. Substitui o
 * window.prompt do navegador por um diálogo no padrão visual do sistema.
 * O motivo informado é validado no cliente (mínimo {@link MIN_LEN} chars)
 * e enviado ao backend, que grava na auditoria.
 */
export default function UndiffuseModal({ reportNumber, onCancel, onConfirm }: Props) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const trimmed = reason.trim();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (trimmed.length < MIN_LEN) {
      setErr(`Motivo obrigatório · mínimo ${MIN_LEN} caracteres`);
      return;
    }
    setBusy(true);
    try {
      await onConfirm(trimmed);
    } catch (e) {
      setErr((e as Error).message || "Falha ao reverter difusão");
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={busy ? undefined : onCancel}>
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="undiffuse-title"
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: 560 }}
      >
        <div className="modal-hd">
          <span id="undiffuse-title">REVERTER DIFUSÃO</span>
          <button
            type="button"
            className="action-btn"
            onClick={onCancel}
            aria-label="Fechar"
            disabled={busy}
          >
            <X size={14} />
          </button>
        </div>
        <form onSubmit={onSubmit}>
          <div className="modal-bd">
            <div className="undiffuse-warn">
              <span className="undiffuse-warn-icon">
                <AlertTriangle size={16} />
              </span>
              <div>
                O relatório <b>{reportNumber || "(sem número)"}</b> voltará ao
                status <b>CRIADO</b> e poderá ser editado novamente.
                <br />
                O número original é preservado · a ação fica registrada na
                trilha de auditoria.
              </div>
            </div>

            <label className="form-field" style={{ marginTop: 14 }}>
              <span>MOTIVO · OBRIGATÓRIO</span>
              <textarea
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                rows={5}
                maxLength={MAX_LEN}
                autoFocus
                disabled={busy}
                placeholder="Descreva por que esta difusão precisa ser revertida…"
              />
              <span
                style={{
                  fontSize: 10,
                  letterSpacing: "0.06em",
                  color:
                    trimmed.length < MIN_LEN
                      ? "var(--crit)"
                      : "var(--fg-3)",
                  marginTop: 4,
                }}
              >
                {trimmed.length}/{MAX_LEN} · MÍNIMO {MIN_LEN} CARACTERES
              </span>
            </label>

            {err && (
              <div className="banner banner-error" style={{ marginTop: 8 }}>
                ⚠ {err}
              </div>
            )}
          </div>
          <div className="modal-ft">
            <button
              type="button"
              className="btn btn-ghost"
              onClick={onCancel}
              disabled={busy}
            >
              CANCELAR
            </button>
            <button
              type="submit"
              className="btn btn-danger"
              disabled={busy || trimmed.length < MIN_LEN}
            >
              {busy ? "REVERTENDO…" : "CONFIRMAR REVERSÃO"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
