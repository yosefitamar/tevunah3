"use client";

import { useEffect, useState } from "react";
import { Copy, X } from "lucide-react";
import { getSipomPayload } from "@/lib/sipom-api";
import type { ApiError } from "@/lib/api";

/**
 * Prévia do envio ao SIPOM: o JSON exato do POST para esta ocorrência
 * (fotos sem o conteúdo, só o tipo). Serve para conferir e para mostrar ao
 * desenvolvimento do SIPOM um caso real.
 */
export default function SipomPayloadModal({ occurrenceId, onClose }: { occurrenceId: string; onClose: () => void }) {
  const [text, setText] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    getSipomPayload(occurrenceId)
      .then((r) => {
        if (!r.ready) {
          setError("Ocorrência com pendências: " + r.pendencias.map((p) => p.label).join("; "));
          return;
        }
        setText(JSON.stringify(r.payload, null, 2));
      })
      .catch((e) => setError((e as ApiError).message || "Erro ao montar a prévia"));
  }, [occurrenceId]);

  async function copy() {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setError("Não foi possível copiar");
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal modal--wide sipom-payload" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div className="modal-hd">
          <span>PRÉVIA DO ENVIO AO SIPOM</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd">
          {error && <div className="banner banner-error">⚠ {error}</div>}
          {!error && !text && <div className="muted">// MONTANDO…</div>}
          {text && <pre className="sipom-json">{text}</pre>}
        </div>
        <div className="filters-modal-ft">
          <span className="muted" style={{ fontSize: 12 }}>
            Fotos aparecem sem o conteúdo (só o tipo).
          </span>
          <div style={{ marginLeft: "auto" }} />
          <button type="button" className="btn" onClick={copy} disabled={!text}>
            <Copy size={13} strokeWidth={1.8} /> {copied ? "COPIADO" : "COPIAR JSON"}
          </button>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            FECHAR
          </button>
        </div>
      </div>
    </div>
  );
}
