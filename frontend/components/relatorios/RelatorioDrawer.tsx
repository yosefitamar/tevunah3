"use client";

import { useCallback, useEffect, useState } from "react";
import { Archive, Send, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  REPORT_STATUS_LABEL,
  REPORT_STATUS_PILL,
  archiveReport,
  diffuseReport,
  getReport,
  updateReport,
  type Qualification,
  type Report,
} from "@/lib/reports-api";
import {
  canArchiveReports,
  canDiffuseReports,
  canEditReports,
} from "@/lib/permissions";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import DateInput from "../shared/DateInput";
import RichTextEditor from "../shared/RichTextEditor";

type Props = {
  reportId: string;
  onClose: () => void;
  onChanged: () => void;
};

/**
 * Drawer do relatório. Versão atual entrega o "esqueleto":
 * - Cabeçalho com Nº/status/ações de transição
 * - Forms de metadata editáveis em status 'criado'
 * - Listagem das qualificações (read-only por enquanto)
 *
 * O editor TipTap do corpo + a UI completa de qualificações (picker civil
 * + form militar) ficam pra próxima entrega.
 */
export default function RelatorioDrawer({ reportId, onClose, onChanged }: Props) {
  const { user: me } = useAuth();
  const [data, setData] = useState<Report | null>(null);
  const [quals, setQuals] = useState<Qualification[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  // Buffer local do corpo (TipTap). Debounce de 700ms antes de enviar PATCH —
  // evita chamada por keystroke. saveStatus indica visualmente o estado.
  const [bodyDraft, setBodyDraft] = useState<string>("");
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved">("idle");

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const r = await getReport(reportId);
      setData(r.report);
      setBodyDraft(r.report.body_html);
      setQuals(r.qualifications);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [reportId]);

  useEffect(() => {
    reload();
  }, [reload]);

  // Debounce do corpo. Dispara PATCH 700ms após a última mudança.
  useEffect(() => {
    if (!data) return;
    if (bodyDraft === data.body_html) return;
    if (data.status !== "criado") return;
    setSaveStatus("saving");
    const h = window.setTimeout(async () => {
      try {
        const r = await updateReport(data.id, { body_html: bodyDraft });
        setData(r.report);
        setSaveStatus("saved");
        window.setTimeout(() => setSaveStatus("idle"), 1500);
        onChanged();
      } catch (e) {
        setError((e as ApiError).message || "Erro ao salvar corpo");
        setSaveStatus("idle");
      }
    }, 700);
    return () => window.clearTimeout(h);
    // data.body_html é referência relevante (após reload). Não inclui `data`
    // inteiro pra não disparar ao trocar de status.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bodyDraft, data?.id, data?.status, data?.body_html]);

  const editable = data?.status === "criado";
  const canEdit = canEditReports(me) && editable;
  const canDiffuse = canDiffuseReports(me) && data?.status === "criado";
  const canArchive = canArchiveReports(me) && data?.status === "difundido";

  async function patch(patch: Partial<Report>) {
    if (!data) return;
    setNotice(null);
    try {
      const r = await updateReport(data.id, patch);
      setData(r.report);
      onChanged();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gravar");
    }
  }

  async function onDiffuse() {
    if (!data) return;
    if (!confirm("Difundir relatório?\nApós difundido, o número é alocado e o documento NÃO pode mais ser editado.")) return;
    setActing(true);
    setError(null);
    try {
      const r = await diffuseReport(data.id);
      setData(r.report);
      setNotice(`RELATÓRIO DIFUNDIDO · Nº ${r.report.number}`);
      onChanged();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao difundir");
    } finally {
      setActing(false);
    }
  }

  async function onArchive() {
    if (!data) return;
    if (!confirm("Arquivar relatório?\nPermanece read-only.")) return;
    setActing(true);
    setError(null);
    try {
      const r = await archiveReport(data.id);
      setData(r.report);
      setNotice("RELATÓRIO ARQUIVADO");
      onChanged();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao arquivar");
    } finally {
      setActing(false);
    }
  }

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <aside
        className="drawer"
        onClick={(e) => e.stopPropagation()}
        aria-label="Relatório"
      >
        <div className="drawer-hd">
          <span>RELATÓRIO INTERNO</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>

        <div className="drawer-bd">
          {loading && <div className="muted">// CARREGANDO…</div>}
          {error && <div className="banner banner-error">⚠ {error}</div>}
          {notice && <div className="banner banner-info">✓ {notice}</div>}

          {data && (
            <>
              <div className="dossier-head">
                <div className="dossier-code">{data.number || "S/N · RASCUNHO"}</div>
                <div className="dossier-name">
                  {data.subject ? data.subject.toUpperCase() : "(SEM ASSUNTO)"}
                </div>
                <div className="dossier-meta">
                  <span className={"pill " + REPORT_STATUS_PILL[data.status]}>
                    {REPORT_STATUS_LABEL[data.status]}
                  </span>
                  <span>{formatBR(data.doc_date)}</span>
                </div>
              </div>

              {/* Forms editáveis só em 'criado'. */}
              <div className="entity-form" style={{ marginTop: 12 }}>
                <fieldset className="form-fieldset" disabled={!canEdit}>
                  <legend>CABEÇALHO</legend>
                  <div className="form-grid-2">
                    <div className="form-field">
                      <span>DATA</span>
                      <DateInput
                        value={data.doc_date}
                        onChange={(v) => patch({ ...data, doc_date: v } as Report) && undefined}
                        disabled={!canEdit}
                      />
                    </div>
                    <label className="form-field">
                      <span>ASSUNTO</span>
                      <input
                        type="text"
                        defaultValue={data.subject}
                        onBlur={(e) => e.target.value !== data.subject && patch({ subject: e.target.value } as Partial<Report>)}
                      />
                    </label>
                  </div>
                  <label className="form-field">
                    <span>ORIGEM</span>
                    <input
                      type="text"
                      defaultValue={data.origin}
                      onBlur={(e) => e.target.value !== data.origin && patch({ origin: e.target.value } as Partial<Report>)}
                    />
                  </label>
                  <label className="form-field">
                    <span>DIFUSÃO</span>
                    <input
                      type="text"
                      defaultValue={data.diffusion}
                      onBlur={(e) => e.target.value !== data.diffusion && patch({ diffusion: e.target.value } as Partial<Report>)}
                    />
                  </label>
                  <div className="form-grid-2">
                    <label className="form-field">
                      <span>DIF. ANT.</span>
                      <input
                        type="text"
                        defaultValue={data.prior_diffusion}
                        onBlur={(e) => e.target.value !== data.prior_diffusion && patch({ prior_diffusion: e.target.value } as Partial<Report>)}
                      />
                    </label>
                    <label className="form-field">
                      <span>REF.</span>
                      <input
                        type="text"
                        defaultValue={data.reference}
                        onBlur={(e) => e.target.value !== data.reference && patch({ reference: e.target.value } as Partial<Report>)}
                      />
                    </label>
                  </div>
                  <label className="form-field">
                    <span>ANEXO(S)</span>
                    <input
                      type="text"
                      defaultValue={data.attachments}
                      onBlur={(e) => e.target.value !== data.attachments && patch({ attachments: e.target.value } as Partial<Report>)}
                    />
                  </label>
                </fieldset>

                <fieldset className="form-fieldset">
                  <legend>
                    CORPO
                    {saveStatus === "saving" && (
                      <span className="muted" style={{ marginLeft: 8, fontSize: 9 }}>
                        // SALVANDO…
                      </span>
                    )}
                    {saveStatus === "saved" && (
                      <span
                        style={{ marginLeft: 8, fontSize: 9, color: "var(--accent)" }}
                      >
                        ✓ SALVO
                      </span>
                    )}
                  </legend>
                  <RichTextEditor
                    value={bodyDraft}
                    onChange={setBodyDraft}
                    disabled={!canEdit}
                  />
                </fieldset>

                <fieldset className="form-fieldset">
                  <legend>QUALIFICAÇÕES ({quals.length})</legend>
                  {quals.length === 0 && (
                    <div className="muted" style={{ fontSize: 11 }}>
                      // NENHUMA QUALIFICAÇÃO · PICKER NA PRÓXIMA ENTREGA
                    </div>
                  )}
                  {quals.map((q) => (
                    <div key={q.id} className="vehicle-link-row">
                      <span className="vehicle-link-label">
                        {q.kind.toUpperCase()} · {(q.data?.nome as string) || "(sem nome)"}
                      </span>
                      <span className="muted" style={{ fontSize: 10 }}>
                        {q.source}
                      </span>
                    </div>
                  ))}
                </fieldset>
              </div>

              <div className="drawer-section">
                <div className="drawer-section-title">AÇÕES</div>
                <div className="action-list">
                  {canDiffuse && (
                    <button
                      type="button"
                      className="action-row"
                      onClick={onDiffuse}
                      disabled={acting}
                    >
                      <span className="action-row-icon">
                        <Send size={14} />
                      </span>
                      <span className="action-row-label">DIFUNDIR</span>
                      <span className="action-row-hint">
                        ALOCA O NÚMERO E BLOQUEIA EDIÇÕES
                      </span>
                    </button>
                  )}
                  {canArchive && (
                    <button
                      type="button"
                      className="action-row"
                      onClick={onArchive}
                      disabled={acting}
                    >
                      <span className="action-row-icon">
                        <Archive size={14} />
                      </span>
                      <span className="action-row-label">ARQUIVAR</span>
                      <span className="action-row-hint">
                        MOVE PRO ARQUIVO · PERMANECE READ-ONLY
                      </span>
                    </button>
                  )}
                  {!canDiffuse && !canArchive && (
                    <div className="muted" style={{ fontSize: 11, padding: 8 }}>
                      // SEM AÇÕES DISPONÍVEIS NESTE STATUS
                    </div>
                  )}
                </div>
              </div>

              <dl className="dossier-list">
                <div>
                  <dt>CRIADO EM</dt>
                  <dd>{formatBR(data.created_at)}</dd>
                </div>
                <div>
                  <dt>ATUALIZADO EM</dt>
                  <dd>{formatBR(data.updated_at)}</dd>
                </div>
                {data.diffused_at && (
                  <div>
                    <dt>DIFUNDIDO EM</dt>
                    <dd>{formatBR(data.diffused_at)}</dd>
                  </div>
                )}
                {data.archived_at && (
                  <div>
                    <dt>ARQUIVADO EM</dt>
                    <dd>{formatBR(data.archived_at)}</dd>
                  </div>
                )}
              </dl>
            </>
          )}
        </div>
      </aside>
    </div>
  );
}
