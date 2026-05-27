"use client";

import { useEffect, useRef, useState } from "react";
import { Download, X } from "lucide-react";
import { downloadReportPDF } from "@/lib/reports-api";

type Props = {
  reportId: string;
  /** Mostrado no cabeçalho do modal (ex.: "RI Nº 07/2026"). */
  reportNumber?: string;
  /** Pode baixar o PDF a partir do preview? (mesma permissão que abriu o preview). */
  canDownload?: boolean;
  onClose: () => void;
};

/**
 * Modal de visualização do PDF do relatório. Consome o MESMO endpoint do
 * download — então:
 *   - exige a mesma permissão `report.download`
 *   - gera um novo registro em `app.report_downloads` (cada preview deixa
 *     rastro próprio, com SHA-256 dos bytes que o usuário viu)
 *   - o PDF chega já com QR + carimbo invisível assados pelo servidor
 *
 * Os bytes ficam num Blob URL escopado à origem; revogado ao fechar pra
 * que não fique pendurado em memória. O download ID e o SHA-256 são
 * exibidos no rodapé do modal pra reforçar a ciência do usuário de que
 * cada preview é auditado.
 */
export default function RelatorioPreviewModal({
  reportId,
  reportNumber,
  canDownload,
  onClose,
}: Props) {
  const [blobURL, setBlobURL] = useState<string | null>(null);
  const [filename, setFilename] = useState<string>("");
  const [downloadID, setDownloadID] = useState<string>("");
  const [sha256, setSha256] = useState<string>("");
  const [err, setErr] = useState<string | null>(null);
  const blobRef = useRef<Blob | null>(null);

  useEffect(() => {
    let revoked = false;
    let urlForCleanup: string | null = null;
    (async () => {
      try {
        const { blob, filename, downloadID, sha256 } = await downloadReportPDF(reportId);
        if (revoked) return;
        blobRef.current = blob;
        const url = URL.createObjectURL(blob);
        urlForCleanup = url;
        setBlobURL(url);
        setFilename(filename);
        setDownloadID(downloadID);
        setSha256(sha256);
      } catch (e) {
        setErr((e as Error).message || "Falha ao gerar PDF");
      }
    })();
    return () => {
      revoked = true;
      if (urlForCleanup) URL.revokeObjectURL(urlForCleanup);
    };
  }, [reportId]);

  function onSaveLocal() {
    if (!blobRef.current || !blobURL) return;
    const a = document.createElement("a");
    a.href = blobURL;
    a.download = filename || `relatorio-${reportId}.pdf`;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className="modal modal--pdf-preview"
        role="dialog"
        aria-modal="true"
        aria-labelledby="preview-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-hd">
          <span id="preview-title">
            VISUALIZAR PDF{reportNumber ? ` · ${reportNumber}` : ""}
          </span>
          <button
            type="button"
            className="action-btn"
            onClick={onClose}
            aria-label="Fechar"
          >
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd modal-bd--flush">
          {err && (
            <div className="banner banner-error" style={{ margin: 12 }}>
              ⚠ {err}
            </div>
          )}
          {!err && !blobURL && (
            <div className="muted" style={{ padding: 24, textAlign: "center", fontSize: 11 }}>
              // GERANDO PDF…
            </div>
          )}
          {blobURL && (
            <iframe
              src={blobURL}
              title="Pré-visualização do PDF"
              className="pdf-preview-frame"
            />
          )}
        </div>
        <div className="modal-ft modal-ft--preview">
          <div className="pdf-preview-meta">
            {downloadID && (
              <span title="ID do registro de download em app.report_downloads">
                ID · {downloadID.slice(0, 8)}…
              </span>
            )}
            {sha256 && (
              <span title="SHA-256 dos bytes do PDF entregue">
                SHA · {sha256.slice(0, 12)}…
              </span>
            )}
            <span className="muted">CADA ABERTURA É REGISTRADA</span>
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            {canDownload && blobURL && (
              <button type="button" className="btn btn-ghost" onClick={onSaveLocal}>
                <Download size={13} style={{ marginRight: 4 }} />
                SALVAR LOCALMENTE
              </button>
            )}
            <button type="button" className="btn btn-primary" onClick={onClose}>
              FECHAR
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
