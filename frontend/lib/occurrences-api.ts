// Ocorrências — a listagem que reúne o cadastro manual (incidents) e o que
// veio do relatório operacional (ops-occurrences). O detalhe de cada linha
// continua nas APIs de cada fonte; aqui ficam a listagem e o vocabulário de
// duplicidade que as duas compartilham.

import { api, type ApiError } from "./api";
import type { SipomPending } from "./ops-reports-api";
import {
  INCIDENT_TYPE_LABEL,
  type IncidentMeans,
  type IncidentType,
  type PlaceFacet,
} from "./incidents-api";

/** De onde vem a ocorrência. */
export type OccurrenceSource = "manual" | "operacional";

export const OCCURRENCE_SOURCE_LABEL: Record<OccurrenceSource, string> = {
  manual: "CADASTRO MANUAL",
  operacional: "RELATÓRIO OPERACIONAL",
};

/**
 * Linha da listagem. Tem `incident_id`, `ops_id` ou os dois — os dois quando
 * o cadastro manual e o relatório operacional trazem a mesma ficha CIOPS: é
 * uma ocorrência só, e os campos do cadastro prevalecem.
 */
export type OccurrenceRow = {
  incident_id: string | null;
  ops_id: string | null;
  /** Tipo do cadastro manual; "" quando a linha é só do relatório. */
  type: IncidentType | "";
  /** Naturezas do relatório; vazio quando a linha é só do cadastro. */
  natures: string[];
  occurred_on: string;
  time: string;
  ciops_record: string;
  city: string;
  neighborhood: string;
  /** Descrição do cadastro ou, na falta dela, o histórico do relatório. */
  description: string;
  means: IncidentMeans;
  intel_participation: boolean;
  has_geo: boolean;
  people_count: number;
  updated_at: string;
  /**
   * O que falta na ocorrência do relatório operacional para o envio ao SIPOM.
   * `blocking` impede o envio; as demais são avisos. Vazio quando a linha é
   * só do cadastro manual.
   */
  pendencias: SipomPending[];
  /** Coordenada (vem na consulta do mapa). "bairro" = ponto aproximado. */
  latitude?: number;
  longitude?: number;
  geo_precision?: "" | "porta" | "rua" | "bairro";
};

/**
 * Abas da tela. HOMICÍDIOS é o cadastro de CVLI; PRODUTIVIDADE é o resto —
 * prisões e apreensões do cadastro e tudo o que veio do relatório operacional.
 */
export type OccurrenceCategory = "produtividade" | "homicidios";

export const OCCURRENCE_CATEGORIES: OccurrenceCategory[] = ["produtividade", "homicidios"];

export const OCCURRENCE_CATEGORY_LABEL: Record<OccurrenceCategory, string> = {
  produtividade: "PRODUTIVIDADE",
  homicidios: "HOMICÍDIOS",
};

export type OccurrencesList = {
  items: OccurrenceRow[];
  total: number;
  limit: number;
  offset: number;
  /** Total de cada aba no recorte comum às duas (busca, período, território). */
  counts: Record<OccurrenceCategory, number>;
};

export type ListOccurrencesOpts = {
  limit?: number;
  offset?: number;
  category?: OccurrenceCategory;
  source?: "" | OccurrenceSource;
  type?: "" | IncidentType;
  means?: IncidentMeans;
  city?: string;
  neighborhood?: string;
  search?: string;
  date_from?: string;
  date_to?: string;
  sort_by?: "occurred_on" | "type" | "updated_at";
  sort_dir?: "asc" | "desc";
};

export function listOccurrences(opts: ListOccurrencesOpts = {}) {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(opts)) {
    if (v !== undefined && v !== "" && v !== 0) q.set(k, String(v));
  }
  const s = q.toString();
  return api<OccurrencesList>(`/api/occurrences${s ? "?" + s : ""}`);
}

export type OccurrencesGeo = {
  items: OccurrenceRow[];
  /** O recorte inteiro, com ou sem coordenada. */
  total: number;
  /** true = o recorte estourou o teto de pontos do servidor. */
  truncated: boolean;
};

/**
 * Pontos georreferenciados do recorte, para o mapa (sem paginação — o período
 * é o limitador). A camada de produtividade pede `category: "produtividade"`.
 */
export function listOccurrencesGeo(opts: ListOccurrencesOpts = {}) {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(opts)) {
    if (v !== undefined && v !== "" && v !== 0) q.set(k, String(v));
  }
  const s = q.toString();
  return api<OccurrencesGeo>(`/api/occurrences/geo${s ? "?" + s : ""}`);
}

/** Municípios e bairros da listagem unificada (as duas fontes). */
export function listOccurrenceLocations() {
  return api<{ cities: PlaceFacet[]; neighborhoods: PlaceFacet[] }>(`/api/occurrences/locations`);
}

// ─── Duplicidade ──────────────────────────────────────────────────────
//
// Duas camadas. Ficha CIOPS igual à de outro cadastro: o servidor recusa
// (`ciops_duplicate`). Ficha diferente, mas mesma data, hora e lugar — ou
// ficha quase igual —, é sinal de ficha digitada errado: o servidor devolve
// as ocorrências parecidas (`possible_duplicates`) e só grava se o analista
// confirmar que é outra.

export type DuplicateReason = "similar_ciops" | "same_time" | "same_city" | "same_neighborhood";

export const DUPLICATE_REASON_LABEL: Record<DuplicateReason, string> = {
  similar_ciops: "FICHA PARECIDA",
  same_time: "MESMA HORA",
  same_city: "MESMO MUNICÍPIO",
  same_neighborhood: "MESMO BAIRRO",
};

/** Ocorrência já gravada, apontada como a mesma ou como possível repetição. */
export type DuplicateCandidate = {
  source: OccurrenceSource;
  id: string;
  type?: IncidentType;
  natures: string[];
  occurred_on: string;
  time: string;
  ciops_record: string;
  city: string;
  neighborhood: string;
  reasons: DuplicateReason[];
};

export type DuplicateConflict = {
  code: "ciops_duplicate" | "possible_duplicates";
  candidates: DuplicateCandidate[];
};

/** Extrai o conflito de duplicidade de um erro 409, se for o caso. */
export function duplicateConflictOf(e: unknown): DuplicateConflict | null {
  const err = e as ApiError;
  if (err?.status !== 409) return null;
  const d = err.details as Partial<DuplicateConflict> | undefined;
  if (d?.code !== "ciops_duplicate" && d?.code !== "possible_duplicates") return null;
  return { code: d.code, candidates: d.candidates ?? [] };
}

/** "HOMICÍDIO" para o cadastro; as naturezas para o relatório. */
export function occurrenceTitle(o: { type?: IncidentType | ""; natures: string[] }): string {
  if (o.type) return INCIDENT_TYPE_LABEL[o.type];
  return o.natures.join(" · ") || "OCORRÊNCIA";
}
