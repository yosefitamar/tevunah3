"use client";

import { useCallback, useEffect, useState } from "react";
import { Archive, Download, Eye, RotateCcw, Send, Trash, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useModal } from "@/contexts/ModalContext";
import {
  CONFIDENTIALITY_LABEL,
  REPORT_STATUS_LABEL,
  REPORT_STATUS_PILL,
  archiveReport,
  destroyReport,
  diffuseReport,
  downloadReportPDF,
  getReport,
  undiffuseReport,
  updateReport,
  type Qualification,
  type Report,
  type ReportConfidentiality,
} from "@/lib/reports-api";
import {
  canArchiveReports,
  canDestroyReports,
  canDiffuseReports,
  canDownloadReports,
  canEditReports,
  canUndiffuseReports,
} from "@/lib/permissions";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import DateInput from "../shared/DateInput";
import RichTextEditor from "../shared/RichTextEditor";
import RefMultiSelect from "./RefMultiSelect";
import QualificationPicker from "./QualificationPicker";
import UndiffuseModal from "./UndiffuseModal";
import RelatorioPreviewModal from "./RelatorioPreviewModal";
import VisibilityFieldset from "./VisibilityFieldset";
import Select from "../shared/Select";
import { Camera, Trash2 } from "lucide-react";
import {
  deleteQualification,
  qualificationPhotoURL,
  uploadQualificationPhoto,
} from "@/lib/reports-api";
import { photoURL } from "@/lib/entities-api";

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
  const modal = useModal();
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
  const [undiffuseOpen, setUndiffuseOpen] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);

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
  const canUndiffuse = canUndiffuseReports(me) && data?.status === "difundido";
  const canDownload = canDownloadReports(me) && !!data;
  // Destruir: rascunho + role permitida + autor OU admin. Espelha o gate
  // server-side (authorOrAdmin + status='criado').
  const isAuthorOrAdmin =
    !!me &&
    !!data &&
    (me.id === data.created_by || (me.roles ?? []).includes("administrador"));
  const canDestroy =
    canDestroyReports(me) && data?.status === "criado" && isAuthorOrAdmin;

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
    const ok = await modal.confirm({
      title: "DIFUNDIR RELATÓRIO",
      message:
        "Ao difundir, o número é alocado em definitivo e o documento NÃO poderá mais ser editado. Confirmar?",
      confirm: "DIFUNDIR",
      cancel: "CANCELAR",
      variant: "warning",
    });
    if (!ok) return;
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

  async function onDownload() {
    if (!data) return;
    if (data.status === "criado") {
      const ok = await modal.confirm({
        title: "BAIXAR RASCUNHO",
        message:
          "Este relatório ainda não foi difundido — o PDF sairá marcado como RASCUNHO e sem número. Confirmar?",
        confirm: "BAIXAR ASSIM MESMO",
        cancel: "CANCELAR",
        variant: "warning",
      });
      if (!ok) return;
    }
    setActing(true);
    setError(null);
    try {
      const { blob, filename } = await downloadReportPDF(data.id);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      // Revoga depois de um tick — alguns navegadores ainda estão consumindo a URL no momento do click.
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      setNotice(`PDF GERADO · ${filename}`);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gerar PDF");
    } finally {
      setActing(false);
    }
  }

  async function onUndiffuseConfirm(reason: string) {
    if (!data) return;
    setActing(true);
    setError(null);
    try {
      const r = await undiffuseReport(data.id, reason);
      setData(r.report);
      setNotice("DIFUSÃO REVERTIDA · RELATÓRIO REABERTO PARA EDIÇÃO");
      setUndiffuseOpen(false);
      onChanged();
    } catch (e) {
      const msg = (e as ApiError).message || "Erro ao reverter difusão";
      // Propaga pro modal exibir inline e mantém o estado aberto pro usuário retentar.
      throw new Error(msg);
    } finally {
      setActing(false);
    }
  }

  async function onDestroy() {
    if (!data) return;
    const ok = await modal.confirm({
      title: "DESTRUIR RASCUNHO",
      message:
        "Esta ação removerá permanentemente o rascunho do sistema. Os dados não poderão ser recuperados pela interface. A operação fica registrada na auditoria. Confirmar?",
      confirm: "DESTRUIR",
      cancel: "CANCELAR",
      variant: "warning",
      danger: true,
    });
    if (!ok) return;
    setActing(true);
    setError(null);
    try {
      await destroyReport(data.id);
      onChanged();
      onClose();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao destruir rascunho");
      setActing(false);
    }
  }

  async function onArchive() {
    if (!data) return;
    const ok = await modal.confirm({
      title: "ARQUIVAR RELATÓRIO",
      message:
        "O relatório vai para o arquivo e permanece somente leitura. Confirmar?",
      confirm: "ARQUIVAR",
      cancel: "CANCELAR",
      variant: "warning",
    });
    if (!ok) return;
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
    <>
    <div className="drawer-backdrop" onClick={onClose}>
      <aside
        className="drawer drawer--xl"
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
                    <div className="form-field">
                      <span>REF.</span>
                      <RefMultiSelect
                        value={data.reference}
                        disabled={!canEdit}
                        onChange={(next) => {
                          if (next !== data.reference) {
                            patch({ reference: next } as Partial<Report>);
                          }
                        }}
                      />
                    </div>
                  </div>
                  <div className="form-grid-2">
                    <label className="form-field">
                      <span>ANEXO(S)</span>
                      <input
                        type="text"
                        defaultValue={data.attachments}
                        onBlur={(e) => e.target.value !== data.attachments && patch({ attachments: e.target.value } as Partial<Report>)}
                      />
                    </label>
                    <div className="form-field">
                      <span>CONFIDENCIALIDADE</span>
                      <Select
                        value={data.confidentiality}
                        disabled={!canEdit}
                        onChange={(v) => {
                          if (v !== data.confidentiality) {
                            patch({ confidentiality: v as ReportConfidentiality } as Partial<Report>);
                          }
                        }}
                        options={[
                          { value: "sigiloso", label: CONFIDENTIALITY_LABEL.sigiloso },
                          { value: "secreto", label: CONFIDENTIALITY_LABEL.secreto },
                          { value: "ultrassecreto", label: CONFIDENTIALITY_LABEL.ultrassecreto },
                        ]}
                      />
                    </div>
                  </div>
                </fieldset>

                <VisibilityFieldset
                  report={data}
                  meID={me?.id ?? ""}
                  canManage={
                    !!me &&
                    (me.id === data.created_by || (me.roles ?? []).includes("administrador"))
                  }
                  canEdit={
                    data.status === "criado" &&
                    !!me &&
                    (me.id === data.created_by || (me.roles ?? []).includes("administrador"))
                  }
                  onReportUpdated={(r) => setData(r)}
                />

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
                    onUploadImage={fileToInlineImage}
                  />
                </fieldset>

                <fieldset className="form-fieldset">
                  <legend>QUALIFICAÇÕES ({quals.length})</legend>
                  {quals.length === 0 && (
                    <div className="muted" style={{ fontSize: 11 }}>
                      // NENHUMA QUALIFICAÇÃO ADICIONADA
                    </div>
                  )}
                  {quals.map((q) => {
                    const nome = (q.data?.nome as string) || "(sem nome)";
                    const aliases = (q.data?.aliases as string[] | undefined) ?? [];
                    const apelido = aliases.length > 0 ? aliases[0] : "";
                    const display = apelido
                      ? `${nome.toUpperCase()} (${apelido.toUpperCase()})`
                      : nome.toUpperCase();
                    const posto = (q.data?.posto as string) || "";
                    const meta = [
                      q.kind === "militar" ? posto : "",
                      q.source,
                    ]
                      .filter(Boolean)
                      .join(" · ");
                    const hasCivilPhoto =
                      q.kind === "civil" && !!q.entity_id;
                    const hasMilPhoto =
                      q.kind === "militar" && !!(q.data?.photo_path as string | undefined);
                    const thumbSrc = hasCivilPhoto
                      ? photoURL(q.entity_id!, 0)
                      : hasMilPhoto
                        ? qualificationPhotoURL(data.id, q.id, q.id)
                        : "";
                    return (
                      <div key={q.id} className="qual-row">
                        <span
                          className={
                            "pill " + (q.kind === "militar" ? "active" : "hold")
                          }
                          style={{ fontSize: 9 }}
                        >
                          {q.kind.toUpperCase()}
                        </span>
                        <div className="qual-row-info">
                          <div className="qual-row-name">{display}</div>
                          {meta && <div className="qual-row-meta">{meta}</div>}
                        </div>
                        {thumbSrc && (
                          <img
                            className="qual-thumb"
                            src={thumbSrc}
                            alt=""
                            aria-hidden
                            onError={(ev) => {
                              (ev.currentTarget as HTMLImageElement).style.display = "none";
                            }}
                          />
                        )}
                        {canEdit && q.kind === "militar" && (
                          <button
                            type="button"
                            className="action-btn"
                            aria-label={hasMilPhoto ? "Trocar foto" : "Adicionar foto"}
                            title={hasMilPhoto ? "Trocar foto" : "Adicionar foto"}
                            onClick={() => {
                              const input = document.createElement("input");
                              input.type = "file";
                              input.accept = "image/jpeg,image/png";
                              input.onchange = async () => {
                                const f = input.files?.[0];
                                if (!f) return;
                                if (f.size > 5 * 1024 * 1024) {
                                  setError("Foto deve ter no máximo 5 MiB");
                                  return;
                                }
                                try {
                                  await uploadQualificationPhoto(data.id, q.id, f);
                                  reload();
                                  onChanged();
                                } catch (e) {
                                  setError(
                                    (e as ApiError).message ||
                                      "Erro ao subir foto",
                                  );
                                }
                              };
                              input.click();
                            }}
                          >
                            <Camera size={12} />
                          </button>
                        )}
                        {canEdit && (
                          <button
                            type="button"
                            className="action-btn"
                            aria-label="Remover qualificação"
                            onClick={async () => {
                              const ok = await modal.confirm({
                                title: "REMOVER QUALIFICAÇÃO",
                                message: "Esta qualificação será removida do relatório. Confirmar?",
                                confirm: "REMOVER",
                                cancel: "CANCELAR",
                                variant: "warning",
                                danger: true,
                              });
                              if (!ok) return;
                              try {
                                await deleteQualification(data.id, q.id);
                                reload();
                                onChanged();
                              } catch (e) {
                                setError(
                                  (e as ApiError).message ||
                                    "Erro ao remover qualificação",
                                );
                              }
                            }}
                          >
                            <Trash2 size={12} />
                          </button>
                        )}
                      </div>
                    );
                  })}
                  {canEdit && (
                    <div style={{ marginTop: 12 }}>
                      <QualificationPicker
                        reportId={data.id}
                        onAdded={() => {
                          reload();
                          onChanged();
                        }}
                      />
                    </div>
                  )}
                </fieldset>
              </div>

              <div className="drawer-section">
                <div className="drawer-section-title">AÇÕES</div>
                <div className="action-list action-list--inline">
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
                  {canDownload && (
                    <button
                      type="button"
                      className="action-row"
                      onClick={() => setPreviewOpen(true)}
                      disabled={acting}
                    >
                      <span className="action-row-icon">
                        <Eye size={14} />
                      </span>
                      <span className="action-row-label">VISUALIZAR PDF</span>
                      <span className="action-row-hint">
                        ABRE INLINE · REGISTRADO NA AUDITORIA COMO DOWNLOAD
                      </span>
                    </button>
                  )}
                  {canDownload && (
                    <button
                      type="button"
                      className="action-row"
                      onClick={onDownload}
                      disabled={acting}
                    >
                      <span className="action-row-icon">
                        <Download size={14} />
                      </span>
                      <span className="action-row-label">
                        {acting ? "GERANDO PDF…" : "BAIXAR PDF"}
                      </span>
                      <span className="action-row-hint">
                        {data.status === "criado"
                          ? "RASCUNHO · MARCA D'ÁGUA"
                          : "REGISTRA O DOWNLOAD NA AUDITORIA"}
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
                  {canUndiffuse && (
                    <button
                      type="button"
                      className="action-row action-row--danger"
                      onClick={() => setUndiffuseOpen(true)}
                      disabled={acting}
                    >
                      <span className="action-row-icon">
                        <RotateCcw size={14} />
                      </span>
                      <span className="action-row-label">REVERTER DIFUSÃO</span>
                      <span className="action-row-hint">
                        DEVOLVE PRO STATUS CRIADO · EXIGE MOTIVO · AUDITADO
                      </span>
                    </button>
                  )}
                  {canDestroy && (
                    <button
                      type="button"
                      className="action-row action-row--danger"
                      onClick={onDestroy}
                      disabled={acting}
                    >
                      <span className="action-row-icon">
                        <Trash size={14} />
                      </span>
                      <span className="action-row-label">DESTRUIR</span>
                      <span className="action-row-hint">
                        REMOVE O RASCUNHO · NÃO RECUPERÁVEL · AUDITADO
                      </span>
                    </button>
                  )}
                  {!canDiffuse && !canArchive && !canUndiffuse && !canDownload && !canDestroy && (
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
    {undiffuseOpen && data && (
      <UndiffuseModal
        reportNumber={data.number}
        onCancel={() => setUndiffuseOpen(false)}
        onConfirm={onUndiffuseConfirm}
      />
    )}
    {previewOpen && data && (
      <RelatorioPreviewModal
        reportId={data.id}
        reportNumber={data.number}
        canDownload={canDownload}
        onClose={() => setPreviewOpen(false)}
      />
    )}
    </>
  );
}

// fileToInlineImage carrega o arquivo escolhido, reescala via canvas se a
// largura ultrapassar 1600px (preservando proporção) e devolve uma data URI
// base64 pronta pra ser usada como `src` do <img>. JPEG q=0.85 para fotos;
// PNG quando o arquivo original era PNG (preserva transparência).
//
// Optamos por inline data URI em vez de upload separado: simplifica o
// pipeline (sem orphan files, sem auth extra em /uploads), e o wkhtmltopdf
// já está acostumado a renderizar data: URIs (brasoes do RELINT funcionam
// pela mesma rota).
async function fileToInlineImage(file: File): Promise<string> {
  const MAX_WIDTH = 1600;
  const QUALITY = 0.85;
  const url = URL.createObjectURL(file);
  try {
    const img = await new Promise<HTMLImageElement>((resolve, reject) => {
      const i = new window.Image();
      i.onload = () => resolve(i);
      i.onerror = () => reject(new Error("falha ao decodificar a imagem"));
      i.src = url;
    });
    const scale = img.naturalWidth > MAX_WIDTH ? MAX_WIDTH / img.naturalWidth : 1;
    const w = Math.round(img.naturalWidth * scale);
    const h = Math.round(img.naturalHeight * scale);
    const canvas = document.createElement("canvas");
    canvas.width = w;
    canvas.height = h;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("canvas indisponível neste navegador");
    ctx.drawImage(img, 0, 0, w, h);
    const mime = file.type === "image/png" ? "image/png" : "image/jpeg";
    return canvas.toDataURL(mime, QUALITY);
  } finally {
    URL.revokeObjectURL(url);
  }
}
