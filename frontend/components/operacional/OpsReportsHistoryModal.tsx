"use client";

import { useEffect, useState } from "react";
import { FileText, X } from "lucide-react";
import { listOpsReports, opsReportFileURL, type OpsReport } from "@/lib/ops-reports-api";
import { formatBR, formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";

type Props = {
  onClose: () => void;
  /** Filtra a listagem pelas ocorrências deste relatório. */
  onPick: (r: OpsReport) => void;
};

/** PDFs já importados: quando, por quem e o que cada um rendeu. */
export default function OpsReportsHistoryModal({ onClose, onPick }: Props) {
  const [items, setItems] = useState<OpsReport[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    listOpsReports(100)
      .then((r) => setItems(r.items))
      .catch((e) => setErr((e as ApiError).message || "Erro ao carregar"));
  }, []);

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 860, width: "100%" }}>
        <div className="modal-hd">
          <span>RELATÓRIOS IMPORTADOS</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd">
          {err && <div className="banner banner-error">⚠ {err}</div>}
          <div className="table-scroll" style={{ maxHeight: "60vh" }}>
            <table className="tbl">
              <thead>
                <tr>
                  <th style={{ width: 130, whiteSpace: "nowrap" }}>RELATÓRIO DE</th>
                  <th style={{ width: 80, textAlign: "right" }}>NO PDF</th>
                  <th style={{ width: 90, textAlign: "right" }}>BATALHÃO</th>
                  <th style={{ width: 90, textAlign: "right" }}>GRAVADAS</th>
                  <th style={{ width: 110, textAlign: "right", whiteSpace: "nowrap" }}>JÁ EXISTIAM</th>
                  <th>IMPORTADO</th>
                  <th style={{ width: 44 }} />
                </tr>
              </thead>
              <tbody>
                {!items && !err && (
                  <tr>
                    <td colSpan={7} className="muted" style={{ textAlign: "center", padding: 24 }}>
                      // CARREGANDO…
                    </td>
                  </tr>
                )}
                {items?.length === 0 && (
                  <tr>
                    <td colSpan={7} className="muted" style={{ textAlign: "center", padding: 24 }}>
                      // NENHUM RELATÓRIO IMPORTADO
                    </td>
                  </tr>
                )}
                {items?.map((r) => (
                  <tr key={r.id} className="row-clickable" onClick={() => onPick(r)} title="Ver as ocorrências deste relatório">
                    <td>{r.report_date ? formatBRDate(r.report_date) : "—"}</td>
                    <td className="mono" style={{ textAlign: "right" }}>{r.total_occurrences}</td>
                    <td className="mono" style={{ textAlign: "right" }}>{r.unit_occurrences}</td>
                    <td className="mono" style={{ textAlign: "right" }}>{r.imported_occurrences}</td>
                    <td className="mono muted" style={{ textAlign: "right" }}>{r.skipped_occurrences || "—"}</td>
                    <td>
                      <div>{formatBR(r.created_at)}</div>
                      <div className="muted" style={{ fontSize: 11.5 }}>{r.created_by_name}</div>
                    </td>
                    <td>
                      <a
                        className="action-btn"
                        href={opsReportFileURL(r.id)}
                        target="_blank"
                        rel="noreferrer"
                        title={r.file_name}
                        aria-label="Abrir PDF original"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <FileText size={13} />
                      </a>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}
