"use client";

import { useEffect, useState } from "react";
import { Check, GraduationCap, Pencil, Power, X } from "lucide-react";
import { useModal } from "@/contexts/ModalContext";
import {
  GUIDANCE_STATUS_LABEL,
  GUIDANCE_TOPIC_LABEL,
  decideGuidance,
  listGuidance,
  type Guidance,
  type GuidanceStatus,
} from "@/lib/ai-guidance-api";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import GuidanceModal from "./GuidanceModal";

const STATUS_PILL: Record<GuidanceStatus, string> = {
  pendente: "hold",
  aprovada: "active",
  recusada: "crit",
  desativada: "cold",
};

type Filter = "ativas" | "pendentes" | "todas";

/**
 * Orientações para a IA nas pendências do SIPOM. O analista vê as aprovadas e
 * as suas; o administrador vê todas e aprova ou recusa as pendentes.
 */
export default function GuidancePanel() {
  const modal = useModal();
  const [items, setItems] = useState<Guidance[]>([]);
  const [canApprove, setCanApprove] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [editing, setEditing] = useState<Guidance | null>(null);
  const [filter, setFilter] = useState<Filter>("ativas");

  async function load() {
    try {
      const r = await listGuidance();
      setItems(r.items);
      setCanApprove(r.can_approve);
      if (r.can_approve && r.pending > 0) setFilter((f) => (f === "ativas" ? "pendentes" : f));
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  const replace = (g: Guidance) => setItems((cur) => cur.map((x) => (x.id === g.id ? g : x)));

  async function decide(g: Guidance, decisao: "aprovar" | "recusar" | "desativar") {
    let nota = "";
    if (decisao === "recusar") {
      const r = await modal.prompt({
        title: "RECUSAR ORIENTAÇÃO",
        message: "A orientação deixa de valer também para o autor. Diga o motivo — ele verá na lista.",
        label: "MOTIVO",
        type: "textarea",
        confirm: "RECUSAR",
        validate: (v) => (v.trim().length < 3 ? "Escreva o motivo" : null),
      });
      if (r == null) return;
      nota = r;
    } else if (decisao === "desativar") {
      const ok = await modal.confirm({
        variant: "warning",
        title: "DESATIVAR ORIENTAÇÃO",
        message: "A IA deixa de seguir esta orientação. Ela continua no histórico.",
        confirm: "DESATIVAR",
        cancel: "CANCELAR",
      });
      if (!ok) return;
    }
    setBusy(g.id);
    setError(null);
    try {
      replace((await decideGuidance(g.id, decisao, nota)).item);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gravar");
    } finally {
      setBusy(null);
    }
  }

  const pending = items.filter((g) => g.status === "pendente").length;
  const shown = items.filter((g) =>
    filter === "todas" ? true : filter === "pendentes" ? g.status === "pendente" : g.status === "pendente" || g.status === "aprovada",
  );

  return (
    <div className="panel">
      <div className="panel-hd">
        <span className="ttl">
          <GraduationCap size={13} style={{ verticalAlign: -2 }} /> ORIENTAÇÕES PARA A IA
        </span>
        <span className="meta">
          {canApprove
            ? `${pending} AGUARDANDO SUA APROVAÇÃO`
            : "CRIE PELO BOTÃO ENSINAR NAS SUGESTÕES DA IA · VALEM PARA VOCÊ ATÉ O ADMINISTRADOR APROVAR"}
        </span>
      </div>
      <div className="panel-bd dense">
        <div className="guid-filter">
          {(["ativas", "pendentes", "todas"] as Filter[]).map((f) => (
            <button
              key={f}
              type="button"
              className={"btn btn-sm" + (filter === f ? " btn-primary" : " btn-ghost")}
              onClick={() => setFilter(f)}
            >
              {f === "ativas" ? "EM VIGOR" : f === "pendentes" ? `PENDENTES (${pending})` : "TODAS"}
            </button>
          ))}
        </div>
        {error && <div className="banner banner-error">⚠ {error}</div>}
        {loading ? (
          <div className="ia-center">
            <span className="spinner" aria-label="Carregando" />
          </div>
        ) : shown.length === 0 ? (
          <div className="muted guid-empty">// NENHUMA ORIENTAÇÃO {filter === "pendentes" ? "PENDENTE" : "AQUI"}</div>
        ) : (
          <ul className="guid-list">
            {shown.map((g) => (
              <li key={g.id} className="guid-item">
                <div className="guid-meta">
                  <span className="guid-topic">{GUIDANCE_TOPIC_LABEL[g.topic]}</span>
                  <span className="muted">· {g.cidade ? g.cidade.toUpperCase() : "TODAS AS CIDADES"}</span>
                  <span className={"pill " + STATUS_PILL[g.status]}>
                    {g.mine || g.status !== "pendente" ? GUIDANCE_STATUS_LABEL[g.status] : "PENDENTE"}
                  </span>
                </div>
                <div className="guid-body">{g.body}</div>
                <div className="muted guid-foot">
                  {g.mine ? "VOCÊ" : g.author_name.toUpperCase()} · {formatBR(g.created_at)}
                  {g.uses > 0 && ` · USADA ${g.uses}×`}
                  {g.decided_at && g.status !== "pendente" && ` · ${g.status.toUpperCase()} POR ${g.decider_name.toUpperCase()} EM ${formatBR(g.decided_at)}`}
                  {g.decision_note && ` · “${g.decision_note}”`}
                </div>
                <div className="guid-actions">
                  {canApprove && g.status === "pendente" && (
                    <>
                      <button type="button" className="btn btn-sm btn-primary" disabled={!!busy} onClick={() => decide(g, "aprovar")}>
                        <Check size={12} /> APROVAR
                      </button>
                      <button type="button" className="btn btn-sm btn-danger" disabled={!!busy} onClick={() => decide(g, "recusar")}>
                        <X size={12} /> RECUSAR
                      </button>
                    </>
                  )}
                  {g.mine && g.status === "pendente" && (
                    <button type="button" className="btn btn-sm btn-ghost" disabled={!!busy} onClick={() => setEditing(g)}>
                      <Pencil size={12} /> EDITAR
                    </button>
                  )}
                  {(g.mine || canApprove) && (g.status === "pendente" || g.status === "aprovada") && (
                    <button type="button" className="btn btn-sm btn-ghost" disabled={!!busy} onClick={() => decide(g, "desativar")}>
                      <Power size={12} /> DESATIVAR
                    </button>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
      {editing && (
        <GuidanceModal
          editing={editing}
          topic={editing.topic}
          onClose={() => setEditing(null)}
          onSaved={(g) => {
            replace(g);
            setEditing(null);
          }}
        />
      )}
    </div>
  );
}
