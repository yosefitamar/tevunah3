"use client";

import { useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { listEntityOccurrences, type EntityOccurrence } from "@/lib/entities-api";
import { INCIDENT_TYPE_LABEL } from "@/lib/incidents-api";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import OcorrenciaDrawer from "../ocorrencias/OcorrenciaDrawer";
import OpsOccurrenceDrawer from "../operacional/OpsOccurrenceDrawer";

function roleTone(role: string): string {
  switch (role.trim().toUpperCase()) {
    case "VÍTIMA":
    case "VITIMA":
      return "role-tone role-tone--victim";
    case "ACUSADO":
      return "role-tone role-tone--accused";
    default:
      return "role-tone";
  }
}

/**
 * Onde a entidade aparece: CVLI do cadastro manual e ocorrências do relatório
 * operacional, numa linha do tempo só. Cada fonte só vem se quem consulta lê
 * o módulo dela — o servidor recorta. Sem ocorrências, a seção não aparece.
 */
export default function OccurrencesSection({ entityID }: { entityID: string }) {
  const [items, setItems] = useState<EntityOccurrence[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState<EntityOccurrence | null>(null);

  const reload = useCallback(async () => {
    try {
      const r = await listEntityOccurrences(entityID);
      setItems(r.items);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar ocorrências");
    }
  }, [entityID]);

  useEffect(() => {
    reload();
  }, [reload]);

  if (error) {
    return (
      <div className="drawer-section">
        <div className="drawer-section-title">OCORRÊNCIAS</div>
        <div className="banner banner-error">⚠ {error}</div>
      </div>
    );
  }
  if (!items || items.length === 0) return null;

  return (
    <div className="drawer-section">
      <div className="drawer-section-title">OCORRÊNCIAS ({items.length})</div>
      {items.map((o) => {
        const title =
          o.source === "cvli"
            ? o.type
              ? INCIDENT_TYPE_LABEL[o.type]
              : "OCORRÊNCIA"
            : o.natures.join(" · ") || "OCORRÊNCIA";
        const place = [o.neighborhood, o.city].filter(Boolean).join(" · ");
        return (
          <div
            key={o.source + o.id}
            className="qual-row qual-row--clickable"
            role="button"
            tabIndex={0}
            onClick={() => setOpen(o)}
            onKeyDown={(ev) => {
              if (ev.key === "Enter" || ev.key === " ") {
                ev.preventDefault();
                setOpen(o);
              }
            }}
          >
            <div className="qual-row-info">
              <div className="qual-row-name">{title}</div>
              <div className="qual-row-meta">
                <span className={roleTone(o.role)}>{o.role || "SEM PAPEL"}</span>
                {" · "}
                {formatBRDate(o.occurred_on)}
                {o.time && ` ${o.time}`}
                {place && ` · ${place}`}
                {o.ciops_record && ` · ${o.ciops_record}`}
              </div>
            </div>
            <span className={"pill " + (o.source === "cvli" ? "crit" : "hold")}>
              {o.source === "cvli" ? "CVLI" : "REL. OPERACIONAL"}
            </span>
          </div>
        );
      })}

      {/* Portal: o drawer da ocorrência abre por cima do dossiê, fora da
          árvore dele (o aside do dossiê não pode ser o contêiner do fixed). */}
      {open &&
        createPortal(
          open.source === "cvli" ? (
            <OcorrenciaDrawer incidentId={open.id} onClose={() => setOpen(null)} onChanged={reload} />
          ) : (
            <OpsOccurrenceDrawer occurrenceId={open.id} onClose={() => setOpen(null)} onChanged={reload} />
          ),
          document.body,
        )}
    </div>
  );
}
