"use client";

import { useState, type FormEvent } from "react";
import { X } from "lucide-react";
import { updateOpsPerson, type OpsOccurrence, type OpsPerson, type OpsPersonEdit } from "@/lib/ops-reports-api";
import type { ApiError } from "@/lib/api";

type Props = {
  occId: string;
  person: OpsPerson;
  onClose: () => void;
  /** Chamado com a ficha já corrigida e recalculada. */
  onSaved: (occ: OpsOccurrence) => void;
};

const norm = (s: string) => s.trim().replace(/\s+/g, " ").toUpperCase();

/**
 * Correção do envolvido como o relatório o trouxe — o PDF erra letra de nome
 * ("OÃO" por "JOÃO"). Só o que mudou é enviado; o antes e o depois vão para a
 * auditoria e a tradução para o SIPOM é refeita.
 */
export default function EditOpsPersonModal({ occId, person, onClose, onSaved }: Props) {
  const [name, setName] = useState(person.name);
  const [mother, setMother] = useState(person.mother_name);
  const [age, setAge] = useState(person.age != null ? String(person.age) : "");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  function changes(): OpsPersonEdit {
    const out: OpsPersonEdit = {};
    if (norm(name) !== norm(person.name)) out.name = norm(name);
    if (norm(mother) !== norm(person.mother_name)) out.mother_name = norm(mother);
    const a = age.trim() === "" ? null : Number(age);
    if (a !== person.age) out.age = a;
    return out;
  }
  const patch = changes();
  const total = Object.keys(patch).length;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (!norm(name)) {
      setErr("Informe o nome");
      return;
    }
    if (age.trim() && !(Number(age) >= 0 && Number(age) <= 120)) {
      setErr("Idade entre 0 e 120");
      return;
    }
    if (total === 0) {
      onClose();
      return;
    }
    setBusy(true);
    try {
      const r = await updateOpsPerson(occId, person.id as string, patch);
      onSaved(r.occurrence);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao gravar a correção");
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={busy ? undefined : onClose}>
      <div className="modal ops-edit" onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span>CORRIGIR ENVOLVIDO · {person.role}</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar" disabled={busy}>
            <X size={14} />
          </button>
        </div>
        <form className="modal-form" onSubmit={onSubmit}>
          <div className="modal-bd ops-edit-bd">
            <label className="form-field">
              <span>NOME</span>
              <input type="text" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
            </label>
            <label className="form-field">
              <span>NOME DA MÃE</span>
              <input type="text" value={mother} onChange={(e) => setMother(e.target.value)} />
            </label>
            <label className="form-field">
              <span>IDADE</span>
              <input
                type="text"
                inputMode="numeric"
                maxLength={3}
                value={age}
                onChange={(e) => setAge(e.target.value.replace(/\D/g, ""))}
              />
            </label>
            {err && <div className="banner banner-error">⚠ {err}</div>}
          </div>
          <div className="modal-ft ops-edit-ft">
            <span className="muted ops-edit-note">
              {total === 0 ? "NENHUMA CORREÇÃO" : `${total} CAMPO${total > 1 ? "S" : ""} CORRIGIDO${total > 1 ? "S" : ""}`}
              {" · A CORREÇÃO VAI PARA A AUDITORIA; O PDF ORIGINAL NÃO É ALTERADO"}
            </span>
            <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
              CANCELAR
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy || total === 0}>
              {busy ? "GRAVANDO…" : "GRAVAR CORREÇÃO"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
