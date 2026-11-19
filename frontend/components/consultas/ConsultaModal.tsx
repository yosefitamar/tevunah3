"use client";

import { useState, type FormEvent } from "react";
import { Trash2, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useModal } from "@/contexts/ModalContext";
import {
  createConsulta,
  deleteConsulta,
  updateConsulta,
  type Consulta,
  type ConsultaInput,
} from "@/lib/consultas-api";
import { canDeleteConsultas, canEditConsultas, hasRole } from "@/lib/permissions";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import Combobox from "../shared/Combobox";
import DateInput from "../shared/DateInput";

type Props = {
  /** Registro aberto; ausente = novo registro. */
  consulta?: Consulta;
  /** Sistemas já usados, sugeridos no campo SISTEMA. */
  systems: string[];
  onClose: () => void;
  /** Chamado depois de criar, salvar ou excluir. */
  onSaved: () => void;
};

// Data e hora de agora no fuso da agência (o mesmo em que o servidor lê o
// formulário), independentemente do fuso do navegador.
function nowInAgency(): { date: string; time: string } {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Fortaleza",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date());
  const get = (t: string) => parts.find((p) => p.type === t)?.value ?? "";
  return { date: `${get("year")}-${get("month")}-${get("day")}`, time: `${get("hour")}:${get("minute")}` };
}

export default function ConsultaModal({ consulta, systems, onClose, onSaved }: Props) {
  const { user: me } = useAuth();
  const modal = useModal();
  const isNew = !consulta;

  const [form, setForm] = useState<ConsultaInput>(() => {
    if (consulta) {
      return {
        performed_on: consulta.performed_on,
        performed_time: consulta.performed_time,
        system: consulta.system,
        target: consulta.target,
        requester: consulta.requester,
        requester_org: consulta.requester_org,
        reference_doc: consulta.reference_doc,
        reason: consulta.reason,
        result: consulta.result,
      };
    }
    const now = nowInAgency();
    return {
      performed_on: now.date,
      performed_time: now.time,
      system: "",
      target: "",
      requester: "",
      requester_org: "",
      reference_doc: "",
      reason: "",
      result: "",
    };
  });
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const isOwnerOrChief =
    !!consulta && (me?.id === consulta.created_by || hasRole(me, "gestor", "administrador"));
  const canEdit = isNew || (isOwnerOrChief && canEditConsultas(me));
  const canRemove = !isNew && isOwnerOrChief && canDeleteConsultas(me);
  const ro = !canEdit;

  function set<K extends keyof ConsultaInput>(key: K, value: ConsultaInput[K]) {
    setForm((f) => ({ ...f, [key]: value }));
    setDirty(true);
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!canEdit) return;
    const required: [keyof ConsultaInput, string][] = [
      ["performed_on", "Informe a data da consulta."],
      ["performed_time", "Informe a hora da consulta."],
      ["system", "Informe o sistema consultado."],
      ["target", "Informe o alvo da pesquisa."],
      ["requester", "Informe o solicitante."],
      ["reason", "Informe o motivo."],
      ["result", "Informe o resultado."],
    ];
    for (const [key, msg] of required) {
      if (!form[key].trim()) {
        setErr(msg);
        return;
      }
    }
    setErr(null);
    setBusy(true);
    try {
      if (consulta) await updateConsulta(consulta.id, form);
      else await createConsulta(form);
      onSaved();
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao salvar o registro");
      setBusy(false);
    }
  }

  async function onDelete() {
    if (!consulta || !canRemove) return;
    const ok = await modal.confirm({
      variant: "warning",
      title: "EXCLUIR REGISTRO DE CONSULTA",
      message: "O registro sai da listagem e a exclusão fica na trilha de auditoria. Confirmar?",
      confirm: "EXCLUIR",
      cancel: "CANCELAR",
    });
    if (!ok) return;
    setBusy(true);
    try {
      await deleteConsulta(consulta.id);
      onSaved();
    } catch (e) {
      setErr((e as ApiError).message || "Erro ao excluir");
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 680 }}>
        <div className="modal-hd">
          <span>{isNew ? "REGISTRAR CONSULTA" : "REGISTRO DE CONSULTA"}</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>

        <form className="modal-bd" onSubmit={onSubmit} autoComplete="off">
          {consulta && (
            <div className="muted" style={{ fontSize: 13 }}>
              REGISTRADO POR {consulta.created_by_code} · {consulta.created_by_name?.toUpperCase()} EM{" "}
              {formatBR(consulta.created_at)}
              {consulta.version > 1 && ` · ALTERADO EM ${formatBR(consulta.updated_at)}`}
            </div>
          )}

          <div className="form-grid-2">
            <label className="form-field">
              <span>DATA DA CONSULTA</span>
              <DateInput
                value={form.performed_on}
                disabled={ro}
                max={nowInAgency().date}
                onChange={(v) => set("performed_on", v)}
              />
            </label>
            <label className="form-field">
              <span>HORA</span>
              <input
                type="time"
                value={form.performed_time}
                disabled={ro}
                onChange={(e) => set("performed_time", e.target.value)}
                style={{ colorScheme: "dark" }}
              />
            </label>
          </div>

          <div className="form-grid-2">
            <div className="form-field">
              <span>SISTEMA CONSULTADO</span>
              <Combobox
                value={form.system}
                onChange={(v) => set("system", v)}
                options={systems}
                uppercase
                disabled={ro}
                placeholder="EX.: INFOSEG, SINESP, DETRAN…"
              />
            </div>
            <label className="form-field">
              <span>ALVO DA PESQUISA</span>
              <input
                type="text"
                value={form.target}
                disabled={ro}
                maxLength={500}
                onChange={(e) => set("target", e.target.value)}
                placeholder="Nome, CPF, placa, telefone…"
              />
            </label>
          </div>

          <div className="form-grid-2">
            <label className="form-field">
              <span>SOLICITANTE</span>
              <input
                type="text"
                value={form.requester}
                disabled={ro}
                maxLength={200}
                onChange={(e) => set("requester", e.target.value)}
                placeholder="Quem pediu a consulta"
              />
            </label>
            <label className="form-field">
              <span>ÓRGÃO DO SOLICITANTE (OPCIONAL)</span>
              <input
                type="text"
                value={form.requester_org}
                disabled={ro}
                maxLength={200}
                onChange={(e) => set("requester_org", e.target.value)}
                placeholder="Unidade, delegacia, órgão…"
              />
            </label>
          </div>

          <label className="form-field">
            <span>DOCUMENTO DE REFERÊNCIA (OPCIONAL)</span>
            <input
              type="text"
              value={form.reference_doc}
              disabled={ro}
              maxLength={200}
              onChange={(e) => set("reference_doc", e.target.value)}
              placeholder="Ofício, protocolo, BO, procedimento…"
            />
          </label>

          <label className="form-field">
            <span>MOTIVO</span>
            <textarea
              value={form.reason}
              disabled={ro}
              rows={3}
              maxLength={4000}
              onChange={(e) => set("reason", e.target.value)}
              placeholder="Por que a consulta foi solicitada"
            />
          </label>

          <label className="form-field">
            <span>RESULTADO</span>
            <textarea
              value={form.result}
              disabled={ro}
              rows={5}
              maxLength={8000}
              onChange={(e) => set("result", e.target.value)}
              placeholder="O que a consulta retornou (ou NADA CONSTA)"
            />
          </label>

          {err && <div className="banner banner-error">⚠ {err}</div>}

          <div className="modal-ft">
            {canRemove && (
              <button
                type="button"
                className="btn btn-danger"
                disabled={busy}
                onClick={onDelete}
                style={{ marginRight: "auto" }}
              >
                <Trash2 size={14} strokeWidth={1.8} /> EXCLUIR
              </button>
            )}
            <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
              {ro ? "FECHAR" : "CANCELAR"}
            </button>
            {canEdit && (
              <button type="submit" className="btn btn-primary" disabled={busy || (!isNew && !dirty)}>
                {busy ? "SALVANDO…" : isNew ? "REGISTRAR" : "SALVAR"}
              </button>
            )}
          </div>
        </form>
      </div>
    </div>
  );
}
