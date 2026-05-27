"use client";

import { useEffect, useState, type FormEvent } from "react";
import { X } from "lucide-react";
import {
  CONFIDENTIALITY_LABEL,
  createReport,
  type ReportConfidentiality,
} from "@/lib/reports-api";
import type { ApiError } from "@/lib/api";
import { useSystemSettings } from "@/contexts/SystemSettingsContext";
import DateInput from "../shared/DateInput";
import Select from "../shared/Select";

type Props = {
  onClose: () => void;
  onCreated: (id: string) => void;
};

/**
 * Modal de criação de relatório. Só pede os campos mínimos do cabeçalho
 * (data, assunto, origem, difusão). O detalhamento (corpo, qualificações)
 * acontece no drawer de edição que abre após a criação.
 */
export default function CreateRelatorioModal({ onClose, onCreated }: Props) {
  const today = new Date().toISOString().slice(0, 10);
  const { settings, loading: settingsLoading } = useSystemSettings();
  const [docDate, setDocDate] = useState(today);
  const [subject, setSubject] = useState("");
  // Origin é inicializado vazio e populado quando as settings chegam — o
  // usuário ainda pode sobrescrever manualmente. Se o admin não configurou
  // document_title, o campo abre vazio (sem o default antigo "CCINT/ASINT/PMCE",
  // que agora vive em system_settings).
  const [origin, setOrigin] = useState("");
  const [originTouched, setOriginTouched] = useState(false);
  useEffect(() => {
    if (originTouched) return;
    if (settings?.document_title) setOrigin(settings.document_title);
  }, [settings, originTouched]);
  const [diffusion, setDiffusion] = useState("");
  const [confidentiality, setConfidentiality] = useState<ReportConfidentiality>("secreto");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      const r = await createReport({
        kind: "interno",
        doc_date: docDate,
        subject: subject.trim(),
        origin: origin.trim(),
        diffusion: diffusion.trim(),
        confidentiality,
      });
      onCreated(r.report.id);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao criar relatório");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 540 }}>
        <div className="modal-hd">
          <span>NOVO RELATÓRIO INTERNO</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <form onSubmit={onSubmit}>
          <div className="modal-bd">
            <div className="muted" style={{ fontSize: 11, marginBottom: 12 }}>
              // O NÚMERO É ALOCADO NA DIFUSÃO. ENQUANTO RASCUNHO, O RELATÓRIO
              FICA EM <b>CRIADO</b> E PODE SER EDITADO LIVREMENTE.
            </div>

            <div className="form-field">
              <span>DATA DO DOCUMENTO</span>
              <DateInput value={docDate} onChange={setDocDate} />
            </div>

            <label className="form-field">
              <span>ASSUNTO</span>
              <input
                type="text"
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                autoFocus
                maxLength={200}
                placeholder="breve descritor do conteúdo"
              />
            </label>

            <label className="form-field">
              <span>ORIGEM</span>
              <input
                type="text"
                value={origin}
                onChange={(e) => {
                  setOriginTouched(true);
                  setOrigin(e.target.value);
                }}
                placeholder={settingsLoading ? "carregando…" : "ex.: CCINT/ASINT/PMCE"}
                maxLength={120}
              />
            </label>

            <label className="form-field">
              <span>DIFUSÃO</span>
              <input
                type="text"
                value={diffusion}
                onChange={(e) => setDiffusion(e.target.value)}
                maxLength={200}
                placeholder="ex.: 1ªCIA/20ºBPM"
              />
            </label>

            <div className="form-field">
              <span>CONFIDENCIALIDADE</span>
              <Select
                value={confidentiality}
                onChange={(v) => setConfidentiality(v as ReportConfidentiality)}
                options={[
                  { value: "sigiloso", label: CONFIDENTIALITY_LABEL.sigiloso },
                  { value: "secreto", label: CONFIDENTIALITY_LABEL.secreto },
                  { value: "ultrassecreto", label: CONFIDENTIALITY_LABEL.ultrassecreto },
                ]}
              />
            </div>

            {err && <div className="banner banner-error">⚠ {err}</div>}
          </div>
          <div className="modal-ft">
            <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
              CANCELAR
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy}>
              {busy ? "CRIANDO…" : "CRIAR RASCUNHO"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
