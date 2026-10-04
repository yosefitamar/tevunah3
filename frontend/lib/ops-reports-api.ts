// Relatório Operacional — o PDF diário do CPRAIO. O servidor lê o PDF, recorta
// as ocorrências do batalhão da agência e grava; aqui só a camada HTTP.

import { api, type ApiError } from "./api";
import { terminalHeaders } from "./device-id";
import type { DuplicateCandidate } from "./occurrences-api";

export type OpsPersonRole = "ACUSADO" | "VÍTIMA" | "TESTEMUNHA";

export type OpsMatch = {
  id: string;
  name: string;
  mother_name?: string;
  date_of_birth?: string;
  score: number;
  matched_fields: string[];
};

export type OpsPerson = {
  id?: string;
  role: OpsPersonRole;
  name: string;
  mother_name: string;
  age: number | null;
  address: string;
  note: string;
  entity_id: string | null;
  entity_name?: string;
  entity_deceased?: boolean;
  link_mode: "" | "auto" | "manual";
  matches: OpsMatch[];
  link_warning?: string;
};

export type OpsWeapon = { kind: string; model: string; brand: string; caliber: string; serial: string };
export type OpsDrug = { description: string; grams: number | null; packages: number | null };
export type OpsVehicle = { kind: string; brand: string; model: string; plate: string; color: string };
export type OpsOfficer = {
  registration: string;
  rank: string;
  number: string;
  war_name: string;
  // Composição no SIPOM (tipo de policiamento e função), derivada da equipe.
  sipom_equipe: string;
  sipom_policiamento: string;
  sipom_funcao: string;
};

// ─── SIPOM (destino do envio) ───

export type SipomRef = { id: number; nome: string };

export type SipomPending = { code: string; label: string; blocking: boolean };

/** Envolvido como vai ao SIPOM (modal "Adicionar Pessoa à Ocorrência"). */
export type SipomPerson = {
  vinculo: string;
  /** Texto da opção no SIPOM, na grafia deles. */
  vinculo_label: string;
  nome: string;
  cpf: string;
  /** 1 = Masculino, 2 = Feminino, 0 = não informado. */
  sexo: number;
  nascimento: string | null;
  mae: string;
  morte: boolean;
  foto: boolean;
  /** "dossie" = qualificação do dossiê vinculado; "relatorio" = só o PDF. */
  fonte: "dossie" | "relatorio";
};

/** A ocorrência traduzida para os códigos do SIPOM. */
export type SipomOccurrence = {
  natureza: SipomRef | null;
  logradouro: string;
  numeral: string;
  cidade: SipomRef | null;
  bairro: SipomRef | null;
  area: SipomRef | null;
  /**
   * A área veio da referência aprendida das escolhas dos analistas (cidade +
   * bairro → área), e não do catálogo nem de escolha feita nesta ficha.
   */
  area_learned: boolean;
  area_options: SipomRef[];
  opm: SipomRef | null;
  pendencias: SipomPending[];
  /** Sem pendência bloqueante: pode ser enviada. */
  ready: boolean;
  manual: string[];
  pessoas: SipomPerson[];
  /** Escolhas para as correções: áreas que atuam na cidade, companhias do
   *  batalhão e equipes da ficha. */
  area_candidates: SipomRef[];
  opm_candidates: SipomRef[];
  equipes: string[];
};

export type OpsIntelMode = "auto" | "manual";

export type OpsOccurrence = {
  id?: string;
  report_id?: string;
  page: number;
  natures: string[];
  base_raw: string;
  base_city: string;
  cia: string;
  pel: string;
  bpm: string;
  occurred_on: string;
  start_time: string;
  end_time: string;
  teams: string;
  ciops_record: string;
  place_address: string;
  place_neighborhood: string;
  place_city: string;
  approach_address: string;
  approach_neighborhood: string;
  approach_city: string;
  police_station: string;
  delegate: string;
  procedure_type: string;
  procedure_number: string;
  seized_objects: string;
  narrative: string;
  /**
   * Coordenada do local do fato; null = não localizada. `geo_precision` vale
   * para o ponto do geocodificador: "porta" (rua e número), "rua" ou "bairro"
   * (aproximada — centro do bairro). `geo_source`: "auto" ou "manual" (o
   * analista informou).
   */
  latitude: number | null;
  longitude: number | null;
  geo_precision: "" | "porta" | "rua" | "bairro";
  geo_source: "" | "auto" | "manual";
  people: OpsPerson[];
  weapons: OpsWeapon[];
  drugs: OpsDrug[];
  vehicles: OpsVehicle[];
  officers: OpsOfficer[];
  // Participação da inteligência: "auto" = decidida pelos termos
  // configurados; "manual" = o analista marcou/desmarcou na ficha.
  intel_participation: boolean;
  intel_mode: OpsIntelMode;
  intel_matched: string[];
  /** null quando o catálogo do SIPOM não está carregado no servidor. */
  sipom: SipomOccurrence | null;
  status?: "new" | "duplicate";
  existing_id?: string;
  warnings: string[];
  /**
   * Só na prévia, contra o cadastro manual de Ocorrências. `linked_incident_id`:
   * cadastro com a mesma ficha — é a mesma ocorrência, e a listagem as une.
   * `possible_duplicates`: cadastros de ficha DIFERENTE com a mesma data, hora
   * e lugar — sinal de ficha digitada errado no cadastro.
   */
  linked_incident_id?: string;
  possible_duplicates?: DuplicateCandidate[];
  created_at?: string;
  /** Última correção do analista; ausentes = a ficha é a cópia do PDF. */
  updated_at?: string;
  updated_by_name?: string;
};

/**
 * Correção da ocorrência importada: só os campos presentes são alterados.
 * Hora "" limpa. Tudo vai para a auditoria com o antes e o depois.
 */
export type OpsOccurrenceEdit = Partial<
  Pick<
    OpsOccurrence,
    | "natures"
    | "occurred_on"
    | "start_time"
    | "end_time"
    | "teams"
    | "ciops_record"
    | "base_city"
    | "cia"
    | "pel"
    | "place_address"
    | "place_neighborhood"
    | "place_city"
    | "approach_address"
    | "approach_neighborhood"
    | "approach_city"
    | "police_station"
    | "delegate"
    | "procedure_type"
    | "procedure_number"
    | "seized_objects"
    | "narrative"
  >
>;

export type OpsOccurrenceRow = {
  id: string;
  report_id: string;
  natures: string[];
  base_city: string;
  cia: string;
  pel: string;
  occurred_on: string;
  start_time: string;
  end_time: string;
  teams: string;
  ciops_record: string;
  place_neighborhood: string;
  place_city: string;
  procedure_type: string;
  accused_names: string[];
  people_count: number;
  weapon_count: number;
  drug_count: number;
  vehicle_count: number;
  intel_participation: boolean;
};

export type OpsReport = {
  id: string;
  report_date: string | null;
  file_name: string;
  file_size: number;
  total_occurrences: number;
  unit_occurrences: number;
  imported_occurrences: number;
  skipped_occurrences: number;
  created_at: string;
  created_by: string;
  created_by_name: string;
};

export type OpsPreview = {
  unit: string;
  file_name: string;
  report_date: string | null;
  total: number;
  unit_total: number;
  new_total: number;
  by_unit: Record<string, number>;
  occurrences: OpsOccurrence[];
  warnings: string[];
  already_imported: OpsReport | null;
};

export type OpsImportResult = {
  report: OpsReport;
  occurrence_ids: string[];
  skipped_ciops: string[];
  auto_linked: number;
};

export type OpsFacets = { cias: string[]; pels: string[]; cities: string[]; natures: string[] };

// multipart não passa pelo api(): o Content-Type precisa ser o do FormData.
async function upload<T>(path: string, file: File): Promise<T> {
  const fd = new FormData();
  fd.append("file", file);
  const res = await fetch(path, {
    method: "POST",
    credentials: "include",
    headers: terminalHeaders(),
    body: fd,
  });
  let body: { success: boolean; data?: T; message?: string } = { success: false };
  try {
    body = await res.json();
  } catch {
    // sem corpo
  }
  if (!res.ok) {
    const err = new Error(body.message ?? `HTTP ${res.status}`) as ApiError;
    err.status = res.status;
    throw err;
  }
  return body.data as T;
}

export function previewOpsReport(file: File) {
  return upload<OpsPreview>("/api/ops-reports/preview", file);
}

export function importOpsReport(file: File) {
  return upload<OpsImportResult>("/api/ops-reports", file);
}

export function listOpsReports(limit = 50, offset = 0) {
  return api<{ items: OpsReport[]; total: number }>(
    `/api/ops-reports?limit=${limit}&offset=${offset}`,
  );
}

export function opsReportFileURL(id: string) {
  return `/api/ops-reports/${encodeURIComponent(id)}/file`;
}

export type OpsListParams = {
  limit?: number;
  offset?: number;
  search?: string;
  cia?: string;
  pel?: string;
  nature?: string;
  city?: string;
  report_id?: string;
  date_from?: string;
  date_to?: string;
  intel?: "" | "1" | "0";
  sort_by?: "occurred_on" | "ciops_record" | "place_city" | "cia";
  sort_dir?: "asc" | "desc";
};

export function listOpsOccurrences(p: OpsListParams) {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(p)) {
    if (v !== undefined && v !== "") q.set(k, String(v));
  }
  return api<{ items: OpsOccurrenceRow[]; total: number }>(`/api/ops-occurrences?${q}`);
}

export function getOpsFacets() {
  return api<OpsFacets>("/api/ops-occurrences/facets");
}

export function getOpsOccurrence(id: string) {
  return api<{ occurrence: OpsOccurrence }>(`/api/ops-occurrences/${encodeURIComponent(id)}`);
}

export function updateOpsOccurrence(id: string, input: OpsOccurrenceEdit) {
  return api<{ occurrence: OpsOccurrence }>(`/api/ops-occurrences/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

/**
 * Coordenada da ocorrência: o ponto informado pelo analista, ou `reset` para
 * mandar o geocodificador localizar o endereço de novo.
 */
export function setOpsGeo(id: string, input: { latitude: number; longitude: number } | { reset: true }) {
  return api<{ occurrence: OpsOccurrence }>(`/api/ops-occurrences/${encodeURIComponent(id)}/geo`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function linkOpsPerson(occId: string, personId: string, entityId: string) {
  return api<{ entity_id: string; entity_name: string; link_mode: string }>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/people/${encodeURIComponent(personId)}/entity`,
    { method: "PUT", body: JSON.stringify({ entity_id: entityId }) },
  );
}

export function unlinkOpsPerson(occId: string, personId: string) {
  return api<void>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/people/${encodeURIComponent(personId)}/entity`,
    { method: "DELETE" },
  );
}

export function setOpsIntel(occId: string, value: boolean) {
  return api<{ intel_participation: boolean; intel_mode: OpsIntelMode; intel_matched: string[] }>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/intel`,
    { method: "PUT", body: JSON.stringify({ value }) },
  );
}

// ─── Apresentação ───

export const OPS_ROLE_TONE: Record<OpsPersonRole, string> = {
  ACUSADO: "role-tone role-tone--accused",
  VÍTIMA: "role-tone role-tone--victim",
  TESTEMUNHA: "role-tone",
};

/** "12:15–14:30", ou só o que houver. */
export function opsTimeRange(o: { start_time: string; end_time: string }) {
  if (o.start_time && o.end_time) return `${o.start_time}–${o.end_time}`;
  return o.start_time || o.end_time || "";
}

/** "1ª CIA · 2º PEL · CAUCAIA" — a unidade abaixo do batalhão. */
export function opsUnitLabel(o: { cia: string; pel: string; base_city: string }) {
  return [o.cia, o.pel, o.base_city].filter(Boolean).join(" · ");
}

export function formatGrams(g: number | null) {
  if (g == null) return "—";
  return `${g.toLocaleString("pt-BR", { maximumFractionDigits: 3 })} g`;
}
