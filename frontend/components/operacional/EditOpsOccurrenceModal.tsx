"use client";

import { useState, type FormEvent } from "react";
import { X } from "lucide-react";
import {
  updateOpsOccurrence,
  type OpsOccurrence,
  type OpsOccurrenceEdit,
} from "@/lib/ops-reports-api";
import { duplicateConflictOf } from "@/lib/occurrences-api";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import DateInput from "../shared/DateInput";

type Props = {
  occ: OpsOccurrence;
  onClose: () => void;
  /** Chamado com a ficha já corrigida e recalculada. */
  onSaved: (occ: OpsOccurrence) => void;
};

// Campos de texto da ficha, na ordem do formulário. As naturezas ficam à
// parte: na ficha são lista, aqui uma linha separada por ";".
type TextField = Exclude<keyof OpsOccurrenceEdit, "natures">;

const TEXT_FIELDS: TextField[] = [
  "occurred_on",
  "start_time",
  "end_time",
  "ciops_record",
  "teams",
  "base_city",
  "cia",
  "pel",
  "place_address",
  "place_neighborhood",
  "place_city",
  "approach_address",
  "approach_neighborhood",
  "approach_city",
  "police_station",
  "delegate",
  "procedure_type",
  "procedure_number",
  "seized_objects",
  "narrative",
];

const CLOCK = /^\d{1,2}:\d{2}$/;

function splitNatures(s: string): string[] {
  return s
    .split(";")
    .map((n) => n.trim().replace(/\s+/g, " ").toUpperCase())
    .filter(Boolean);
}

/**
 * Correção da ocorrência importada do relatório operacional. O PDF erra —
 * data, hora, ficha, bairro — e o analista conserta aqui. Só o que mudou é
 * enviado; o servidor grava o antes e o depois de cada campo na auditoria e
 * refaz a tradução para o SIPOM. O PDF original continua disponível na ficha.
 */
export default function EditOpsOccurrenceModal({ occ, onClose, onSaved }: Props) {
  const today = new Date().toISOString().slice(0, 10);
  const [form, setForm] = useState<Record<TextField, string>>(() => {
    const init = {} as Record<TextField, string>;
    for (const f of TEXT_FIELDS) init[f] = occ[f] ?? "";
    return init;
  });
  const [natures, setNatures] = useState(occ.natures.join("; "));
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const set = (f: TextField) => (v: string) => setForm((cur) => ({ ...cur, [f]: v }));

  function changes(): OpsOccurrenceEdit {
    const out: OpsOccurrenceEdit = {};
    for (const f of TEXT_FIELDS) {
      if (form[f].trim() !== (occ[f] ?? "").trim()) out[f] = form[f].trim();
    }
    const list = splitNatures(natures);
    if (list.join(";") !== occ.natures.join(";")) out.natures = list;
    return out;
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    for (const [f, label] of [
      ["start_time", "Hora inicial"],
      ["end_time", "Hora final"],
    ] as const) {
      if (form[f].trim() && !CLOCK.test(form[f].trim())) {
        setErr(`${label} inválida — use HH:MM`);
        return;
      }
    }
    if (!form.occurred_on) {
      setErr("Informe a data do fato");
      return;
    }
    if (splitNatures(natures).length === 0) {
      setErr("Informe ao menos uma natureza");
      return;
    }
    const patch = changes();
    if (Object.keys(patch).length === 0) {
      onClose();
      return;
    }
    setBusy(true);
    try {
      const r = await updateOpsOccurrence(occ.id as string, patch);
      onSaved(r.occurrence);
    } catch (e) {
      const dup = duplicateConflictOf(e);
      if (dup?.code === "ciops_duplicate") {
        const other = dup.candidates[0];
        setErr(
          "Ficha CIOPS já pertence a outra ocorrência" +
            (other ? ` (${formatBRDate(other.occurred_on)}${other.time ? " " + other.time : ""})` : "") +
            " — nada foi alterado.",
        );
      } else {
        setErr((e as ApiError).message || "Falha ao gravar a correção");
      }
      setBusy(false);
    }
  }

  const text = (f: TextField, label: string, placeholder?: string) => (
    <label className="form-field">
      <span>{label}</span>
      <input type="text" value={form[f]} onChange={(e) => set(f)(e.target.value)} placeholder={placeholder} />
    </label>
  );

  return (
    <div className="modal-backdrop" onClick={busy ? undefined : onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 720, width: "100%" }}>
        <div className="modal-hd">
          <span>CORRIGIR OCORRÊNCIA · {occ.ciops_record || "SEM FICHA"}</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar" disabled={busy}>
            <X size={14} />
          </button>
        </div>
        <form className="modal-form" onSubmit={onSubmit}>
          <div className="modal-bd">
            <div className="banner banner-info">
              A CORREÇÃO FICA REGISTRADA NA AUDITORIA, COM O VALOR ANTERIOR E O NOVO. O PDF ORIGINAL NÃO É ALTERADO.
            </div>

            <label className="form-field">
              <span>NATUREZAS (SEPARE COM ;)</span>
              <input type="text" value={natures} onChange={(e) => setNatures(e.target.value)} />
            </label>

            <div className="form-grid-2">
              <div className="form-field">
                <span>DATA DO FATO</span>
                <DateInput value={form.occurred_on} onChange={set("occurred_on")} max={today} />
              </div>
              {text("ciops_record", "FICHA CIOPS")}
            </div>
            <div className="form-grid-2">
              {text("start_time", "HORA INICIAL", "HH:MM")}
              {text("end_time", "HORA FINAL", "HH:MM")}
            </div>

            <div className="form-grid-2">
              {text("teams", "EQUIPES", "RAIO 01; RAIO 02")}
              {text("base_city", "CIDADE-BASE")}
            </div>
            <div className="form-grid-2">
              {text("cia", "COMPANHIA", "1ª CIA")}
              {text("pel", "PELOTÃO", "2º PEL")}
            </div>

            {text("place_address", "LOCAL DA OCORRÊNCIA · ENDEREÇO")}
            <div className="form-grid-2">
              {text("place_neighborhood", "LOCAL DA OCORRÊNCIA · BAIRRO")}
              {text("place_city", "LOCAL DA OCORRÊNCIA · CIDADE")}
            </div>

            {text("approach_address", "LOCAL DA ABORDAGEM · ENDEREÇO")}
            <div className="form-grid-2">
              {text("approach_neighborhood", "LOCAL DA ABORDAGEM · BAIRRO")}
              {text("approach_city", "LOCAL DA ABORDAGEM · CIDADE")}
            </div>

            <div className="form-grid-2">
              {text("police_station", "DELEGACIA")}
              {text("delegate", "DELEGADO")}
            </div>
            <div className="form-grid-2">
              {text("procedure_type", "PROCEDIMENTO")}
              {text("procedure_number", "Nº DO PROCEDIMENTO")}
            </div>

            <label className="form-field">
              <span>OBJETOS APREENDIDOS</span>
              <textarea value={form.seized_objects} onChange={(e) => set("seized_objects")(e.target.value)} rows={3} />
            </label>
            <label className="form-field">
              <span>HISTÓRICO</span>
              <textarea value={form.narrative} onChange={(e) => set("narrative")(e.target.value)} rows={8} />
            </label>

            {err && <div className="banner banner-error">⚠ {err}</div>}
          </div>
          <div className="modal-ft">
            <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
              CANCELAR
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy}>
              {busy ? "GRAVANDO…" : "GRAVAR CORREÇÃO"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
