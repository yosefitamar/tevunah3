"use client";

import { useState, type FormEvent } from "react";
import { X } from "lucide-react";
import { createReport } from "@/lib/reports-api";
import type { ApiError } from "@/lib/api";
import DateInput from "../shared/DateInput";

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
  const [docDate, setDocDate] = useState(today);
  const [subject, setSubject] = useState("");
  const [origin, setOrigin] = useState("CCINT/ASINT/PMCE");
  const [diffusion, setDiffusion] = useState("");
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
                onChange={(e) => setOrigin(e.target.value)}
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
