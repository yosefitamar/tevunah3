"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { getEntity, type PersonDuplicate, type UpdateEntityInput } from "@/lib/entities-api";
import type { Entity } from "@/lib/entities-types";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";

/**
 * O que atualizar o cadastro existente vai fazer. Regra: só preenche o que
 * está EM BRANCO no cadastro; listas (apelidos, telefones, endereços,
 * histórico, tags) só acrescentam o que falta. O que já está preenchido e
 * diverge do formulário é mantido e listado em `conflicts`, para o analista
 * saber o que não entrou.
 */
export type MergePlan = {
  /** PATCH da entidade (version + só os campos que mudam). */
  update: UpdateEntityInput;
  fills: { label: string; value: string }[];
  adds: string[];
  conflicts: { label: string; current: string; incoming: string }[];
  /** Quantos endereços do formulário ainda não estão no cadastro. */
  newAddresses: number;
  /** Sem nada a gravar no PATCH nem fora dele. */
  empty: boolean;
};

type Props = {
  title: string;
  variant: "warning" | "error";
  intro: string;
  candidates: PersonDuplicate[];
  /** false quando o CPF já é do candidato: criar outro é recusado. */
  allowCreate: boolean;
  /** Sem permissão de editar entidades, só resta revisar ou criar. */
  canUpdate: boolean;
  planFor: (existing: Entity) => MergePlan;
  busy: boolean;
  onReview: () => void;
  onCreate: () => void;
  onUpdate: (existing: Entity, plan: MergePlan) => void;
};

const FIELD_LABEL: Record<string, string> = {
  name: "nome",
  mother_name: "mãe",
  date_of_birth: "nascimento",
};

/**
 * Diálogo de duplicata provável no cadastro de pessoa: o analista escolhe o
 * cadastro existente, vê o que será preenchido/acrescentado e decide entre
 * ATUALIZAR esse cadastro, CRIAR uma pessoa nova assim mesmo ou REVISAR.
 */
export default function DuplicateResolver({
  title,
  variant,
  intro,
  candidates,
  allowCreate,
  canUpdate,
  planFor,
  busy,
  onReview,
  onCreate,
  onUpdate,
}: Props) {
  const [selected, setSelected] = useState(candidates[0]?.id ?? "");
  const [entity, setEntity] = useState<Entity | null>(null);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!selected) return;
    let alive = true;
    setLoading(true);
    setErr(null);
    setEntity(null);
    getEntity(selected)
      .then((r) => alive && setEntity(r.entity))
      .catch((e) => alive && setErr((e as ApiError).message || "Erro ao carregar o cadastro"))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [selected]);

  const plan = entity ? planFor(entity) : null;

  const node = (
    <div className="modal-backdrop dup-resolver-backdrop" onClick={(e) => e.stopPropagation()}>
      <div className="modal modal--wide" onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span className={variant === "error" ? "dup-title-error" : "dup-title-warn"}>{title}</span>
          <button type="button" className="action-btn" onClick={onReview} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd">
          <div style={{ fontSize: 13, marginBottom: 8 }}>{intro}</div>

          <div className="dup-candidates">
            {candidates.map((c) => (
              <label key={c.id} className="check dup-candidate">
                <input
                  type="radio"
                  name="dup-candidate"
                  checked={selected === c.id}
                  onChange={() => setSelected(c.id)}
                  disabled={busy}
                />
                <span>
                  <b>{c.name.toUpperCase()}</b>
                  {c.mother_name ? ` · MÃE: ${c.mother_name}` : ""}
                  {c.date_of_birth ? ` · NASC.: ${formatBRDate(c.date_of_birth)}` : ""}
                  {c.matched_fields.length > 0 && (
                    <span className="muted">
                      {" "}
                      · COINCIDE: {c.matched_fields.map((f) => FIELD_LABEL[f] ?? f).join(", ")}
                      {c.approx ? " (grafia aproximada)" : ""}
                    </span>
                  )}
                </span>
              </label>
            ))}
          </div>

          {loading && (
            <div className="dup-loading">
              <span className="spinner" aria-hidden /> CARREGANDO O CADASTRO…
            </div>
          )}
          {err && <div className="banner banner-error">⚠ {err}</div>}

          {plan && (
            <div className="dup-plan">
              <div className="dup-plan-title">SE ATUALIZAR ESTE CADASTRO</div>
              {plan.empty ? (
                <div className="muted" style={{ fontSize: 12.5 }}>
                  // NADA A ACRESCENTAR — O CADASTRO JÁ TEM TODOS ESTES DADOS
                </div>
              ) : (
                <>
                  {plan.fills.length > 0 && (
                    <>
                      <div className="dup-plan-sub">CAMPOS EM BRANCO QUE SERÃO PREENCHIDOS</div>
                      <dl className="dossier-list">
                        {plan.fills.map((f) => (
                          <div key={f.label} className="dossier-row">
                            <dt>{f.label}</dt>
                            <dd>{f.value}</dd>
                          </div>
                        ))}
                      </dl>
                    </>
                  )}
                  {plan.adds.length > 0 && (
                    <>
                      <div className="dup-plan-sub">SERÃO ACRESCENTADOS</div>
                      <ul className="dup-plan-list">
                        {plan.adds.map((a) => (
                          <li key={a}>{a}</li>
                        ))}
                      </ul>
                    </>
                  )}
                </>
              )}
              {plan.conflicts.length > 0 && (
                <>
                  <div className="dup-plan-sub">DIVERGENTES — FICA O QUE JÁ ESTÁ NO CADASTRO</div>
                  <ul className="dup-plan-list">
                    {plan.conflicts.map((c) => (
                      <li key={c.label}>
                        <span className="muted">{c.label}:</span> {c.current}{" "}
                        <span className="muted">(formulário: {c.incoming})</span>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </div>
          )}
          {!canUpdate && (
            <div className="banner banner-warn" style={{ marginTop: 8 }}>
              ⚠ VOCÊ NÃO TEM PERMISSÃO PARA EDITAR ENTIDADES — PEÇA A UM GESTOR PARA ATUALIZAR O
              CADASTRO.
            </div>
          )}
        </div>
        <div className="modal-ft">
          <button type="button" className="btn btn-ghost" onClick={onReview} disabled={busy}>
            REVISAR
          </button>
          {allowCreate && (
            <button type="button" className="btn btn-danger" onClick={onCreate} disabled={busy}>
              CRIAR NOVO ASSIM MESMO
            </button>
          )}
          {canUpdate && (
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => entity && plan && onUpdate(entity, plan)}
              disabled={busy || !entity || !plan || plan.empty}
            >
              {busy ? "GRAVANDO…" : "ATUALIZAR CADASTRO"}
            </button>
          )}
        </div>
      </div>
    </div>
  );

  return typeof document === "undefined" ? node : createPortal(node, document.body);
}
