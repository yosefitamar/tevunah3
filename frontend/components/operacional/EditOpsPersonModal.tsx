"use client";

import { useState, type FormEvent } from "react";
import { Trash2, X } from "lucide-react";
import {
  setOpsPersonRemoved,
  updateOpsPerson,
  type OpsOccurrence,
  type OpsPerson,
  type OpsPersonEdit,
} from "@/lib/ops-reports-api";
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
 * auditoria e a tradução para o SIPOM é refeita. Daqui também se remove o
 * envolvido que a importação leu a mais (linha da tabela quebrada em duas).
 */
export default function EditOpsPersonModal({ occId, person, onClose, onSaved }: Props) {
  const [name, setName] = useState(person.name);
  const [mother, setMother] = useState(person.mother_name);
  const [age, setAge] = useState(person.age != null ? String(person.age) : "");
  const [address, setAddress] = useState(person.address);
  const [removing, setRemoving] = useState(false);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  function changes(): OpsPersonEdit {
    const out: OpsPersonEdit = {};
    if (norm(name) !== norm(person.name)) out.name = norm(name);
    if (norm(mother) !== norm(person.mother_name)) out.mother_name = norm(mother);
    const a = age.trim() === "" ? null : Number(age);
    if (a !== person.age) out.age = a;
    if (norm(address) !== norm(person.address)) out.address = norm(address);
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

  async function remove() {
    setErr(null);
    if (reason.trim().length < 3) {
      setErr("Informe o motivo da remoção");
      return;
    }
    setBusy(true);
    try {
      const r = await setOpsPersonRemoved(occId, person.id as string, true, reason.trim());
      onSaved(r.occurrence);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao remover o envolvido");
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
            <label className="form-field">
              <span>ENDEREÇO</span>
              <input type="text" value={address} onChange={(e) => setAddress(e.target.value)} />
            </label>
            <div className="ops-person-remove">
              {removing ? (
                <>
                  <label className="form-field">
                    <span>MOTIVO DA REMOÇÃO</span>
                    <input
                      type="text"
                      value={reason}
                      maxLength={500}
                      placeholder="Ex.: linha da tabela quebrada — é a continuação do nome da vítima acima"
                      onChange={(e) => setReason(e.target.value)}
                      autoFocus
                    />
                  </label>
                  <div className="ops-person-remove-act">
                    <span className="muted">
                      SAI DA FICHA, DO ENVIO AO SIPOM E DOS DOSSIÊS. NÃO É APAGADO: DÁ PARA RESTAURAR.
                    </span>
                    <button type="button" className="btn btn-ghost btn-sm" onClick={() => setRemoving(false)} disabled={busy}>
                      VOLTAR
                    </button>
                    <button type="button" className="btn btn-danger btn-sm" onClick={remove} disabled={busy || reason.trim().length < 3}>
                      <Trash2 size={12} /> {busy ? "REMOVENDO…" : "CONFIRMAR REMOÇÃO"}
                    </button>
                  </div>
                </>
              ) : (
                <button type="button" className="btn btn-ghost btn-sm" onClick={() => setRemoving(true)} disabled={busy}>
                  <Trash2 size={12} /> REMOVER DA FICHA (LIDO A MAIS NA IMPORTAÇÃO)
                </button>
              )}
            </div>
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
