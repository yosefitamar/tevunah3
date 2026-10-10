"use client";

import { Radar } from "lucide-react";
import type { OpsIntelMode } from "@/lib/ops-reports-api";

/** Selo compacto para listas e cabeçalhos. */
export function IntelPill({ title }: { title?: string }) {
  return (
    <span className="pill intel" title={title ?? "Participação da inteligência"}>
      <Radar size={10} strokeWidth={2} /> INTELIGÊNCIA
    </span>
  );
}

/**
 * Situação da participação da inteligência na ficha: sim/não, quem decidiu
 * (termos ou analista) e os termos que a regra encontrou no texto.
 */
export function IntelSummary({
  value,
  mode,
  matched,
}: {
  value: boolean;
  mode: OpsIntelMode;
  matched: string[];
}) {
  return (
    <span className="intel-summary">
      <span className={value ? "intel-yes" : "muted"}>{value ? "SIM" : "NÃO"}</span>
      <span className="muted">
        {" · "}
        {mode === "manual" ? "MARCADO PELO ANALISTA" : "PELOS TERMOS CONFIGURADOS"}
      </span>
      {matched.length > 0 && (
        <span className="intel-terms">
          {matched.map((t) => (
            <span key={t} className="intel-term">
              {t}
            </span>
          ))}
        </span>
      )}
    </span>
  );
}
