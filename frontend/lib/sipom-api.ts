// SIPOM (sipom.pm.ce.gov.br): destino do envio das ocorrências do relatório
// operacional. Catálogo de naturezas, natureza fixada pelo analista e o
// de-para relatório → SIPOM (Admin).

import { api } from "./api";
import type { OpsOfficer, SipomOccurrence, SipomPending, SipomRef } from "./ops-reports-api";

export type SipomNatureza = { id: number; nome: string; rotulo: string };

export function listSipomNaturezas() {
  return api<{ items: SipomNatureza[] }>("/api/sipom/naturezas");
}

/** Fixa a natureza SIPOM da ocorrência; null devolve a decisão ao de-para. */
export function setOpsSipomNatureza(occId: string, naturezaId: number | null) {
  return api<{ sipom: SipomOccurrence }>(`/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/natureza`, {
    method: "PUT",
    body: JSON.stringify({ natureza_id: naturezaId }),
  });
}

export type SipomConfianca = "direta" | "sugerida";

export type SipomRule = {
  id: string;
  source: string;
  natureza: SipomRef | null;
  confianca: SipomConfianca;
  prioridade: number;
  ocorrencias: number;
};

export type SipomUnmapped = { nature: string; ocorrencias: number };

export function getSipomNatureMap() {
  return api<{ rules: SipomRule[]; unmapped: SipomUnmapped[] }>("/api/admin/sipom/natureza-map");
}

export function upsertSipomNatureRule(input: {
  source: string;
  natureza_id: number | null;
  confianca: SipomConfianca;
  prioridade: number;
}) {
  return api<{ rule: SipomRule; ready: number; pending: number }>("/api/admin/sipom/natureza-map", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// ─── Correções do analista ───

export type SipomFieldInput =
  | { field: "area"; area_id: number | null }
  | { field: "opm"; opm_id: number | null }
  | { field: "endereco"; logradouro: string; numeral: string }
  | { field: "endereco"; reset: true }
  | { field: "composicao"; equipes: string[] }
  | { field: "composicao"; reset: true };

/** Fixa um campo da tradução (ou devolve ao automático com null/reset). */
export function setOpsSipomField(occId: string, input: SipomFieldInput) {
  const { field, ...body } = input;
  return api<{ sipom: SipomOccurrence; officers: OpsOfficer[] }>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/${field}`,
    { method: "PUT", body: JSON.stringify(body) },
  );
}

// ─── Fila de envio ───

export type SipomQueueRow = {
  id: string;
  ciops_record: string;
  occurred_on: string;
  start_time: string;
  natures: string[];
  natureza: SipomRef | null;
  place_city: string;
  place_neighborhood: string;
  area: SipomRef | null;
  opm: SipomRef | null;
  pendencias: SipomPending[];
  ready: boolean;
  people_count: number;
  unlinked_count: number;
  officer_count: number;
  intel_participation: boolean;
};

export type SipomQueue = {
  items: SipomQueueRow[];
  summary: { total: number; ready: number; pending: number; by_code: Record<string, number> };
  labels: Record<string, string>;
};

export function getSipomQueue(p: { date_from?: string; date_to?: string }) {
  const q = new URLSearchParams();
  if (p.date_from) q.set("date_from", p.date_from);
  if (p.date_to) q.set("date_to", p.date_to);
  return api<SipomQueue>(`/api/ops-occurrences/sipom/queue?${q}`);
}

/** Confirma em lote a natureza sugerida pelo de-para. */
export function confirmSipomNaturezas(ids: string[]) {
  return api<{ confirmed: number }>("/api/ops-occurrences/sipom/natureza/confirm", {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

/** Prévia do corpo do POST ao SIPOM (contrato em docs/sipom-integracao.md). */
export function getSipomPayload(occId: string) {
  return api<{ ready: boolean; pendencias: SipomPending[]; payload: unknown | null }>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/payload`,
  );
}
