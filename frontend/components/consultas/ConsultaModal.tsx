"use client";

import { useEffect, useRef, useState, type ClipboardEvent, type FormEvent } from "react";
import { ExternalLink, ImageUp, Trash2, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useModal } from "@/contexts/ModalContext";
import {
  consultaPrintURL,
  createConsulta,
  deleteConsulta,
  deleteConsultaPrint,
  updateConsulta,
  uploadConsultaPrint,
  type Consulta,
  type ConsultaInput,
} from "@/lib/consultas-api";
import { canDeleteConsultas, canEditConsultas, hasRole } from "@/lib/permissions";
import { formatBR } from "@/lib/format";
import { useFileDrop } from "@/lib/useFileDrop";
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
  /** Chamado quando o print de um registro existente muda (sem fechar). */
  onChanged?: () => void;
};

const PRINT_MAX_BYTES = 5 << 20; // mesmo teto do servidor

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

export default function ConsultaModal({ consulta, systems, onClose, onSaved, onChanged }: Props) {
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

  // Print do pedido. Em registro novo o arquivo fica guardado e sobe depois de
  // criar; em registro existente sobe (ou sai) na hora.
  const [hasPrint, setHasPrint] = useState(consulta?.has_print ?? false);
  const [printVer, setPrintVer] = useState(consulta?.version ?? 0);
  const [pending, setPending] = useState<File | null>(null);
  const [pendingURL, setPendingURL] = useState("");
  const fileRef = useRef<HTMLInputElement | null>(null);
  const { dragging, handlers } = useFileDrop((f) => onPrint(f), ro);

  useEffect(() => {
    return () => {
      if (pendingURL) URL.revokeObjectURL(pendingURL);
    };
  }, [pendingURL]);

  const printSrc = consulta
    ? hasPrint
      ? consultaPrintURL(consulta.id, printVer)
      : ""
    : pendingURL;

  async function onPrint(f: File | null) {
    if (!f || ro) return;
    if (!/^image\/(png|jpeg)$/.test(f.type)) {
      setErr("O print deve ser uma imagem PNG ou JPEG.");
      return;
    }
    if (f.size > PRINT_MAX_BYTES) {
      setErr("O print passa de 5 MiB.");
      return;
    }
    setErr(null);
    if (!consulta) {
      setPending(f);
      setPendingURL(URL.createObjectURL(f));
      return;
    }
    setBusy(true);
    try {
      const saved = await uploadConsultaPrint(consulta.id, f);
      setHasPrint(true);
      setPrintVer(saved.version);
      onChanged?.();
    } catch (e) {
      setErr((e as ApiError).message || "Erro ao enviar o print");
    } finally {
      setBusy(false);
    }
  }

  async function onPrintRemove() {
    if (ro) return;
    if (!consulta) {
      setPending(null);
      setPendingURL("");
      return;
    }
    const ok = await modal.confirm({
      variant: "warning",
      title: "REMOVER PRINT",
      message: "O print do pedido é apagado e a remoção fica na trilha de auditoria. Confirmar?",
      confirm: "REMOVER",
      cancel: "CANCELAR",
    });
    if (!ok) return;
    setBusy(true);
    try {
      await deleteConsultaPrint(consulta.id);
      setHasPrint(false);
      onChanged?.();
    } catch (e) {
      setErr((e as ApiError).message || "Erro ao remover o print");
    } finally {
      setBusy(false);
    }
  }

  // Ctrl+V em qualquer ponto do formulário: se a área de transferência traz
  // uma imagem (o print recém-tirado), ela vira o print do pedido.
  function onPaste(e: ClipboardEvent) {
    if (ro) return;
    for (const item of Array.from(e.clipboardData.items)) {
      if (item.kind === "file" && item.type.startsWith("image/")) {
        const f = item.getAsFile();
        if (f) {
          e.preventDefault();
          onPrint(f);
        }
        return;
      }
    }
  }

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
      if (consulta) {
        await updateConsulta(consulta.id, form);
      } else {
        const { consulta: created } = await createConsulta(form);
        if (pending) {
          try {
            await uploadConsultaPrint(created.id, pending);
          } catch (e) {
            // O registro já existe; só o print falhou. Avisa antes de fechar,
            // para o agente reabrir o registro e anexar de novo.
            await modal.alert({
              variant: "error",
              title: "PRINT NÃO ANEXADO",
              message:
                "A consulta foi registrada, mas o print falhou: " +
                ((e as ApiError).message || "erro") +
                ". Abra o registro e anexe o print de novo.",
            });
          }
        }
      }
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

        <form className="modal-bd" onSubmit={onSubmit} onPaste={onPaste} autoComplete="off">
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

          {(!ro || printSrc) && (
            <div className="form-field">
              <span>PRINT DO PEDIDO (OPCIONAL)</span>
              <div className="brasao-row">
                <div
                  className="brasao-preview"
                  role={ro ? undefined : "button"}
                  title={ro ? undefined : "Clique, arraste uma imagem ou cole com Ctrl+V"}
                  onClick={() => !ro && fileRef.current?.click()}
                  style={{
                    width: 200,
                    height: 130,
                    cursor: ro ? undefined : "pointer",
                    outline: dragging ? "2px dashed var(--accent)" : undefined,
                  }}
                  {...handlers}
                >
                  {printSrc ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={printSrc} alt="print do pedido" />
                  ) : (
                    <div className="brasao-empty">
                      <ImageUp size={24} strokeWidth={1.4} />
                      <span>SOLTE, CLIQUE OU COLE</span>
                    </div>
                  )}
                </div>
                <div className="brasao-actions">
                  <input
                    ref={fileRef}
                    type="file"
                    accept="image/png,image/jpeg"
                    style={{ display: "none" }}
                    onChange={(e) => {
                      onPrint(e.target.files?.[0] ?? null);
                      e.target.value = "";
                    }}
                  />
                  {!ro && (
                    <button
                      type="button"
                      className="btn btn-outline"
                      disabled={busy}
                      onClick={() => fileRef.current?.click()}
                    >
                      <ImageUp size={14} strokeWidth={1.6} /> {printSrc ? "TROCAR PRINT" : "ANEXAR PRINT"}
                    </button>
                  )}
                  {printSrc && (
                    <a
                      className="btn btn-outline"
                      href={printSrc}
                      target="_blank"
                      rel="noreferrer"
                      style={{ textDecoration: "none" }}
                    >
                      <ExternalLink size={14} strokeWidth={1.6} /> ABRIR EM TAMANHO REAL
                    </a>
                  )}
                  {!ro && printSrc && (
                    <button type="button" className="btn btn-ghost" disabled={busy} onClick={onPrintRemove}>
                      REMOVER
                    </button>
                  )}
                  {!ro && (
                    <span className="form-hint">
                      PNG ou JPEG, até 5 MiB. Cole com Ctrl+V o print do pedido recebido.
                    </span>
                  )}
                </div>
              </div>
            </div>
          )}

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
