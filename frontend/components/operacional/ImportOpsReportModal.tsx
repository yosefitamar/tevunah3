"use client";

import { useRef, useState } from "react";
import { ChevronDown, ChevronRight, FileUp, Link2, X } from "lucide-react";
import {
  importOpsReport,
  opsTimeRange,
  opsUnitLabel,
  previewOpsReport,
  type OpsImportResult,
  type OpsPerson,
  type OpsPreview,
} from "@/lib/ops-reports-api";
import { useFileDrop } from "@/lib/useFileDrop";
import { formatBR, formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import OpsOccurrenceBody from "./OpsOccurrenceBody";
import { IntelPill } from "./IntelStatus";

type Props = {
  onClose: () => void;
  /** Chamado depois da gravação, com o resultado. */
  onImported: (r: OpsImportResult) => void;
};

/**
 * Importação do Relatório Diário de Ocorrências (PDF do CPRAIO).
 *
 * Dois tempos: o PDF é lido e a prévia mostra exatamente o que será gravado
 * (só as ocorrências do batalhão, com as já existentes marcadas e os vínculos
 * automáticos de pessoa); o analista confere e confirma. Na confirmação o
 * servidor relê o arquivo — a prévia nunca é usada como entrada.
 */
export default function ImportOpsReportModal({ onClose, onImported }: Props) {
  const fileRef = useRef<HTMLInputElement | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<OpsPreview | null>(null);
  const [busy, setBusy] = useState<"" | "reading" | "importing">("");
  const [err, setErr] = useState<string | null>(null);
  const [open, setOpen] = useState<Set<number>>(new Set());

  async function pick(f: File | null) {
    if (!f) return;
    if (f.type && f.type !== "application/pdf") {
      setErr("Envie o relatório em PDF.");
      return;
    }
    setErr(null);
    setFile(f);
    setPreview(null);
    setOpen(new Set());
    setBusy("reading");
    try {
      setPreview(await previewOpsReport(f));
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao ler o PDF");
      setFile(null);
    } finally {
      setBusy("");
    }
  }

  const { dragging, handlers } = useFileDrop(pick, busy !== "");

  async function confirm() {
    if (!file) return;
    setErr(null);
    setBusy("importing");
    try {
      onImported(await importOpsReport(file));
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao importar");
      setBusy("");
    }
  }

  function toggle(i: number) {
    setOpen((s) => {
      const n = new Set(s);
      if (n.has(i)) n.delete(i);
      else n.add(i);
      return n;
    });
  }

  const others = preview
    ? Object.entries(preview.by_unit)
        .filter(([u]) => u !== preview.unit)
        .sort(([a], [b]) => a.localeCompare(b, "pt-BR", { numeric: true }))
    : [];
  const canImport = !!preview && !preview.already_imported && preview.new_total > 0 && busy === "";

  return (
    <div className="modal-backdrop" onClick={busy ? undefined : onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 920, width: "100%" }}>
        <div className="modal-hd">
          <span>IMPORTAR RELATÓRIO OPERACIONAL</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar" disabled={!!busy}>
            <X size={14} />
          </button>
        </div>

        <div className="modal-bd">
          <input
            ref={fileRef}
            type="file"
            accept="application/pdf"
            style={{ display: "none" }}
            onChange={(e) => {
              pick(e.target.files?.[0] ?? null);
              e.target.value = "";
            }}
          />

          {!preview && (
            <div
              className={"ops-drop" + (dragging ? " ops-drop--over" : "")}
              role="button"
              tabIndex={0}
              onClick={() => busy === "" && fileRef.current?.click()}
              onKeyDown={(e) => {
                if ((e.key === "Enter" || e.key === " ") && busy === "") fileRef.current?.click();
              }}
              {...handlers}
            >
              {busy === "reading" ? (
                <>
                  <span className="ops-spinner" aria-hidden />
                  <span>LENDO {file?.name.toUpperCase()}…</span>
                </>
              ) : (
                <>
                  <FileUp size={40} strokeWidth={1.2} />
                  <span>SOLTE O PDF DO RELATÓRIO DIÁRIO OU CLIQUE PARA ESCOLHER</span>
                  <span className="muted" style={{ fontSize: 11.5 }}>
                    SÓ AS OCORRÊNCIAS DO BATALHÃO ENTRAM NO ACERVO · NADA É GRAVADO ANTES DA CONFIRMAÇÃO
                  </span>
                </>
              )}
            </div>
          )}

          {preview && (
            <>
              <div className="ops-summary">
                <div className="stat">
                  <span className="lbl">RELATÓRIO DE</span>
                  <span className="val">{preview.report_date ? formatBRDate(preview.report_date) : "—"}</span>
                </div>
                <div className="stat">
                  <span className="lbl">NO PDF</span>
                  <span className="val">{preview.total}</span>
                </div>
                <div className="stat">
                  <span className="lbl">{preview.unit}</span>
                  <span className="val">{preview.unit_total}</span>
                </div>
                <div className="stat">
                  <span className="lbl">NOVAS</span>
                  <span className="val" style={{ color: preview.new_total ? "var(--accent)" : undefined }}>
                    {preview.new_total}
                  </span>
                </div>
                <div className="stat" style={{ borderRight: 0 }}>
                  <span className="lbl">JÁ NO ACERVO</span>
                  <span className="val">{preview.unit_total - preview.new_total}</span>
                </div>
              </div>

              <div className="muted ops-file-line" title={preview.file_name}>
                {preview.file_name}
                {others.length > 0 && (
                  <> · IGNORADAS: {others.map(([u, n]) => `${u} (${n})`).join(", ")}</>
                )}
              </div>

              {preview.already_imported && (
                <div className="banner banner-warn">
                  ⚠ Este PDF já foi importado em {formatBR(preview.already_imported.created_at)}
                  {preview.already_imported.created_by_name && ` por ${preview.already_imported.created_by_name}`}.
                </div>
              )}
              {!preview.already_imported && preview.unit_total === 0 && (
                <div className="banner banner-warn">⚠ O relatório não traz ocorrências do {preview.unit}.</div>
              )}
              {preview.warnings.map((w, i) => (
                <div key={i} className="banner banner-warn">
                  ⚠ {w}
                </div>
              ))}

              <div className="ops-preview-list">
                {preview.occurrences.map((o, i) => {
                  const isOpen = open.has(i);
                  const dup = o.status === "duplicate";
                  const autos = o.people.filter((p) => p.link_mode === "auto").length;
                  return (
                    <div key={i} className={"ops-preview-item" + (dup ? " ops-preview-item--dup" : "")}>
                      <button type="button" className="ops-preview-row" onClick={() => toggle(i)}>
                        {isOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
                        <span className={"pill " + (dup ? "cold" : "active")}>{dup ? "JÁ EXISTE" : "NOVA"}</span>
                        <span className="ops-preview-when">
                          {formatBRDate(o.occurred_on)}
                          {opsTimeRange(o) && <span className="muted"> · {opsTimeRange(o)}</span>}
                        </span>
                        <span className="ops-preview-nature" title={o.natures.join("; ")}>
                          {o.natures.join(" · ") || "—"}
                        </span>
                        <span className="muted ops-preview-unit">{opsUnitLabel(o)}</span>
                        <span className="muted mono">{o.ciops_record}</span>
                        {o.intel_participation && (
                          <IntelPill title={"Termos encontrados: " + o.intel_matched.join(", ")} />
                        )}
                        {autos > 0 && (
                          <span className="pill info" title="Pessoas que serão vinculadas a dossiês existentes">
                            <Link2 size={10} /> {autos}
                          </span>
                        )}
                        {o.warnings.length > 0 && <span className="pill hold">{o.warnings.length} AVISO(S)</span>}
                      </button>
                      {isOpen && (
                        <div className="ops-preview-body">
                          <OpsOccurrenceBody occ={o} renderPerson={dup ? undefined : (p) => <PreviewLink p={p} />} />
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </>
          )}

          {err && (
            <div className="banner banner-error" style={{ marginTop: 8 }}>
              ⚠ {err}
            </div>
          )}
        </div>

        <div className="modal-ft">
          {preview && (
            <button
              type="button"
              className="btn btn-ghost"
              style={{ marginRight: "auto" }}
              onClick={() => fileRef.current?.click()}
              disabled={busy !== ""}
            >
              TROCAR ARQUIVO
            </button>
          )}
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy !== ""}>
            CANCELAR
          </button>
          <button type="button" className="btn btn-primary" onClick={confirm} disabled={!canImport}>
            <FileUp size={13} />
            {busy === "importing"
              ? "IMPORTANDO…"
              : preview && preview.new_total > 0
                ? `IMPORTAR ${preview.new_total} OCORRÊNCIA${preview.new_total > 1 ? "S" : ""}`
                : "IMPORTAR"}
          </button>
        </div>
      </div>
    </div>
  );
}

// O que a importação fará com a pessoa: vincular (casou nome + mãe com um
// único dossiê) ou deixar para o analista, com o motivo.
function PreviewLink({ p }: { p: OpsPerson }) {
  if (p.link_mode === "auto") {
    return (
      <span className="ops-link ops-link--auto" title="Nome e mãe iguais a um único dossiê">
        <Link2 size={12} /> VINCULA A {p.entity_name || "DOSSIÊ"}
      </span>
    );
  }
  if (p.link_warning) {
    return <span className="ops-link ops-link--warn">{p.link_warning}</span>;
  }
  return <span className="ops-link muted">SEM DOSSIÊ</span>;
}
