"use client";

import {
  DUPLICATE_REASON_LABEL,
  occurrenceTitle,
  type DuplicateCandidate,
} from "@/lib/occurrences-api";
import { formatBRDate } from "@/lib/format";

type Props = {
  candidates: DuplicateCandidate[];
  /** Abre a ocorrência apontada, para o analista conferir antes de decidir. */
  onOpen?: (c: DuplicateCandidate) => void;
};

/**
 * Ocorrências já gravadas apontadas pelo servidor como a mesma (ficha CIOPS
 * igual) ou como possível repetição — com o motivo de cada suspeita. Usada no
 * cadastro manual e na prévia da importação do relatório operacional.
 */
export default function DuplicateCandidates({ candidates, onOpen }: Props) {
  return (
    <>
      {candidates.map((c) => {
        const place = [c.neighborhood, c.city].filter(Boolean).join(" · ");
        const clickable = !!onOpen;
        return (
          <div
            key={c.source + c.id}
            className={"qual-row" + (clickable ? " qual-row--clickable" : "")}
            role={clickable ? "button" : undefined}
            tabIndex={clickable ? 0 : undefined}
            title={clickable ? "Abrir a ocorrência" : undefined}
            onClick={clickable ? () => onOpen(c) : undefined}
            onKeyDown={
              clickable
                ? (ev) => {
                    if (ev.key === "Enter" || ev.key === " ") {
                      ev.preventDefault();
                      onOpen(c);
                    }
                  }
                : undefined
            }
          >
            <div className="qual-row-info">
              <div className="qual-row-name">{occurrenceTitle(c)}</div>
              <div className="qual-row-meta">
                {formatBRDate(c.occurred_on)}
                {c.time && ` ${c.time}`}
                {place && ` · ${place}`}
                {" · "}
                <span className="mono">{c.ciops_record || "SEM FICHA"}</span>
              </div>
              {c.reasons.length > 0 && (
                <div className="qual-row-meta" style={{ color: "var(--warn)" }}>
                  {c.reasons.map((r) => DUPLICATE_REASON_LABEL[r] ?? r).join(" + ")}
                </div>
              )}
            </div>
            <span className={"pill " + (c.source === "manual" ? "cold" : "hold")}>
              {c.source === "manual" ? "CADASTRO" : "REL. OPERACIONAL"}
            </span>
          </div>
        );
      })}
    </>
  );
}
