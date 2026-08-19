"use client";

import { useState } from "react";
import { ClipboardPaste, X } from "lucide-react";
import { parseIncidentReport, type ParsedReport } from "@/lib/incidents-api";
import type { ApiError } from "@/lib/api";

type Props = {
  onClose: () => void;
  /** Recebe os campos reconhecidos para pré-preencher o formulário. */
  onApply: (parsed: ParsedReport) => void;
};

/**
 * Importação do relatório de CVLI que circula nos grupos operacionais.
 *
 * O analista cola o texto como recebeu; o servidor lê os rótulos
 * (NATUREZA:, DATA:, HORÁRIO:, ENDEREÇO:) e as seções numeradas e devolve os
 * campos. Nada é gravado aqui — o resultado cai no formulário de cadastro
 * para conferência campo a campo, porque relatório de grupo tem erro de
 * digitação, hora aproximada e nome sem confirmação.
 */
export default function ImportReportModal({ onClose, onApply }: Props) {
  const [text, setText] = useState("");
  const [resolveLink, setResolveLink] = useState(true);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function run() {
    if (!text.trim()) {
      setErr("Cole o texto do relatório.");
      return;
    }
    setErr(null);
    setBusy(true);
    try {
      const r = await parseIncidentReport(text, resolveLink);
      onApply(r.parsed);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao ler o relatório");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 640 }}>
        <div className="modal-hd">
          <span>IMPORTAR RELATÓRIO</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>

        <div className="modal-bd">
          <label className="form-field">
            <span>TEXTO DO RELATÓRIO</span>
            <textarea
              value={text}
              onChange={(e) => setText(e.target.value)}
              rows={14}
              autoFocus
              placeholder={"RELATÓRIO DE CVLI\nNATUREZA: …\nDATA: …\nHORÁRIO: …\nENDEREÇO: …"}
              style={{ fontFamily: "var(--font-mono, monospace)", fontSize: 11.5 }}
            />
          </label>

          <label className="check" style={{ marginTop: 4 }}>
            <input
              type="checkbox"
              checked={resolveLink}
              onChange={(e) => setResolveLink(e.target.checked)}
            />
            <span>
              Obter as coordenadas do link do Google Maps
              <span className="muted"> — o servidor abre o link no Google</span>
            </span>
          </label>

          <div className="muted" style={{ fontSize: 10.5, marginTop: 8 }}>
            // O RESULTADO É SUGESTÃO: NADA É GRAVADO ATÉ VOCÊ CONFERIR E REGISTRAR
          </div>

          {err && (
            <div className="banner banner-error" style={{ marginTop: 8 }}>
              ⚠ {err}
            </div>
          )}
        </div>

        <div className="modal-ft">
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
            CANCELAR
          </button>
          <button type="button" className="btn btn-primary" onClick={run} disabled={busy}>
            <ClipboardPaste size={13} />
            {busy ? "LENDO…" : "EXTRAIR CAMPOS"}
          </button>
        </div>
      </div>
    </div>
  );
}
