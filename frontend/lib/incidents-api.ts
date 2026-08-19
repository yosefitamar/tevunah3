import { api } from "./api";
import { terminalHeaders } from "./device-id";

export type IncidentType = "homicidio" | "apreensao" | "prisao";

export const INCIDENT_TYPE_LABEL: Record<IncidentType, string> = {
  homicidio: "HOMICÍDIO",
  apreensao: "APREENSÃO",
  prisao: "PRISÃO",
};

// Reuso das variantes da CSS .pill (crit/hold/warn/active/cold/info).
export const INCIDENT_TYPE_PILL: Record<IncidentType, string> = {
  homicidio: "crit",
  apreensao: "hold",
  prisao: "warn",
};

export const INCIDENT_TYPES: IncidentType[] = ["homicidio", "apreensao", "prisao"];

// ─── Meio utilizado (CVLI) ────────────────────────────────────────────
// Aplicável a homicídio. "" = não informado.
export type IncidentMeans =
  | ""
  | "paf"
  | "arma_branca"
  | "asfixia"
  | "contundente"
  | "outros";

export const INCIDENT_MEANS: Exclude<IncidentMeans, "">[] = [
  "paf",
  "arma_branca",
  "asfixia",
  "contundente",
  "outros",
];

export const INCIDENT_MEANS_LABEL: Record<IncidentMeans, string> = {
  "": "NÃO INFORMADO",
  paf: "PAF (ARMA DE FOGO)",
  arma_branca: "ARMA BRANCA",
  asfixia: "ASFIXIA / ESTRANGULAMENTO",
  contundente: "OBJETO CONTUNDENTE",
  outros: "OUTROS",
};

// Rótulo curto pra legenda do mapa e células de tabela.
export const INCIDENT_MEANS_SHORT: Record<IncidentMeans, string> = {
  "": "N/I",
  paf: "PAF",
  arma_branca: "BRANCA",
  asfixia: "ASFIXIA",
  contundente: "CONTUNDENTE",
  outros: "OUTROS",
};

// Cor do ponto no mapa por meio utilizado (usa os tokens da paleta ativa).
export const INCIDENT_MEANS_COLOR: Record<IncidentMeans, string> = {
  "": "var(--fg-3)",
  paf: "var(--crit)",
  arma_branca: "var(--warn)",
  asfixia: "var(--info)",
  contundente: "var(--accent)",
  outros: "var(--fg-2)",
};

// isVictimRole reconhece o papel que, num homicídio, implica óbito. Espelha
// a checagem do backend (o papel é texto livre e pode vir sem acento).
export function isVictimRole(role: string): boolean {
  const r = role.trim().toUpperCase();
  return r === "VÍTIMA" || r === "VITIMA";
}

// Papéis do envolvido na ocorrência. Lista fechada: o papel é chave de
// leitura do caso (e VÍTIMA dispara o óbito), então não pode variar por
// grafia. Uma ocorrência aceita quantas pessoas forem necessárias em cada
// papel — várias vítimas, vários acusados, várias testemunhas.
//
// Registros anteriores a esta versão podem ter papéis fora da lista (AUTOR,
// SUSPEITO, PRESO, ENVOLVIDO); eles continuam sendo exibidos como estão.
export const INVOLVED_ROLES = ["VÍTIMA", "ACUSADO", "TESTEMUNHA"];

export type InvolvedEntity = {
  entity_id: string;
  name: string;
  kind: string;
  role: string;
  has_photo: boolean;
  version: number;
  // Óbito da pessoa — a lista de envolvidos aplica a tarja sem buscar a
  // entidade inteira.
  deceased: boolean;
};

export type Incident = {
  id: string;
  type: IncidentType;
  occurred_on: string; // YYYY-MM-DD
  occurred_time?: string; // HH:MM
  ciops_record: string;
  has_photo: boolean;
  latitude?: number;
  longitude?: number;
  city: string;
  neighborhood: string;
  description: string;
  means: IncidentMeans;
  means_detail: string;
  involved: InvolvedEntity[];
  created_at: string;
  created_by: string;
  updated_at: string;
  updated_by?: string;
};

export type IncidentsList = {
  items: Incident[];
  total: number;
  limit: number;
  offset: number;
};

export type ListIncidentsOpts = {
  limit?: number;
  offset?: number;
  type?: "" | IncidentType;
  means?: IncidentMeans;
  city?: string;
  neighborhood?: string;
  search?: string;
  date_from?: string;
  date_to?: string;
  sort_by?: "occurred_on" | "type" | "created_at" | "updated_at";
  sort_dir?: "asc" | "desc";
};

function qs(opts: ListIncidentsOpts): string {
  const p = new URLSearchParams();
  if (opts.limit) p.set("limit", String(opts.limit));
  if (opts.offset) p.set("offset", String(opts.offset));
  if (opts.type) p.set("type", opts.type);
  if (opts.means) p.set("means", opts.means);
  if (opts.city) p.set("city", opts.city);
  if (opts.neighborhood) p.set("neighborhood", opts.neighborhood);
  if (opts.search) p.set("search", opts.search);
  if (opts.date_from) p.set("date_from", opts.date_from);
  if (opts.date_to) p.set("date_to", opts.date_to);
  if (opts.sort_by) p.set("sort_by", opts.sort_by);
  if (opts.sort_dir) p.set("sort_dir", opts.sort_dir);
  const s = p.toString();
  return s ? "?" + s : "";
}

export function listIncidents(opts: ListIncidentsOpts = {}) {
  return api<IncidentsList>(`/api/incidents${qs(opts)}`);
}

// ─── Mapa do crime ────────────────────────────────────────────────────

export type GeoIncidentsOpts = {
  type?: "" | IncidentType;
  means?: IncidentMeans;
  city?: string;
  neighborhood?: string;
  date_from?: string;
  date_to?: string;
};

export type GeoIncidents = {
  items: Incident[];
  total: number;
  // true = o recorte estourou o teto do servidor e a resposta foi cortada.
  truncated: boolean;
};

// Pontos georreferenciados do recorte (sem paginação — o recorte temporal é
// o limitador). Traz a ocorrência inteira pra o popup do marcador não
// precisar de outra requisição.
export function listIncidentsGeo(opts: GeoIncidentsOpts = {}) {
  return api<GeoIncidents>(`/api/incidents/geo${qs(opts)}`);
}

// ─── Recorte territorial ──────────────────────────────────────────────

export type PlaceFacet = {
  city: string;
  neighborhood?: string;
  count: number;
};

export type IncidentLocations = {
  cities: PlaceFacet[];
  neighborhoods: PlaceFacet[];
};

// Municípios e bairros que existem no acervo (com contagem). Alimenta os
// filtros do mapa e o autocompletar de bairro no cadastro — filtrar por um
// lugar sem ocorrência não teria serventia.
export function listIncidentLocations() {
  return api<IncidentLocations>(`/api/incidents/locations`);
}

export function getIncident(id: string) {
  return api<{ incident: Incident }>(`/api/incidents/${encodeURIComponent(id)}`);
}

export type NewInvolvedInput = { entity_id: string; role?: string };

export type NewIncidentInput = {
  type: IncidentType;
  occurred_on: string;
  occurred_time?: string;
  ciops_record?: string;
  latitude?: number | null;
  longitude?: number | null;
  city?: string;
  neighborhood?: string;
  description?: string;
  means?: IncidentMeans;
  means_detail?: string;
  involved?: NewInvolvedInput[];
};

export function createIncident(input: NewIncidentInput) {
  return api<{ incident: Incident }>(`/api/incidents`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export type UpdateIncidentInput = {
  type?: IncidentType;
  occurred_on?: string;
  occurred_time?: string | null;
  ciops_record?: string;
  latitude?: number | null;
  longitude?: number | null;
  city?: string;
  neighborhood?: string;
  description?: string;
  means?: IncidentMeans;
  means_detail?: string;
};

export function updateIncident(id: string, input: UpdateIncidentInput) {
  return api<{ incident: Incident }>(`/api/incidents/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deleteIncident(id: string) {
  return api<void>(`/api/incidents/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function addIncidentEntity(id: string, entityID: string, role: string) {
  return api<{ incident: Incident }>(
    `/api/incidents/${encodeURIComponent(id)}/entities`,
    { method: "POST", body: JSON.stringify({ entity_id: entityID, role }) },
  );
}

export function removeIncidentEntity(id: string, entityID: string) {
  return api<void>(
    `/api/incidents/${encodeURIComponent(id)}/entities/${encodeURIComponent(entityID)}`,
    { method: "DELETE" },
  );
}

// URL pra servir a foto da ocorrência. `v` força bust quando muda.
export function incidentPhotoURL(id: string, v?: string | number): string {
  const bust = v != null ? `?v=${encodeURIComponent(String(v))}` : "";
  return `/api/incidents/${encodeURIComponent(id)}/photo${bust}`;
}

export async function uploadIncidentPhoto(id: string, file: File): Promise<Incident> {
  const fd = new FormData();
  fd.append("photo", file);
  const res = await fetch(`/api/incidents/${encodeURIComponent(id)}/photo`, {
    method: "POST",
    credentials: "include",
    headers: terminalHeaders(),
    body: fd,
  });
  let body: { success: boolean; data?: { incident: Incident }; message?: string } = {
    success: false,
  };
  try {
    body = await res.json();
  } catch {
    // sem corpo
  }
  if (!res.ok) {
    const err = new Error(body.message ?? `HTTP ${res.status}`) as Error & {
      status: number;
    };
    err.status = res.status;
    throw err;
  }
  return (body.data as { incident: Incident }).incident;
}

export function deleteIncidentPhoto(id: string) {
  return api<void>(`/api/incidents/${encodeURIComponent(id)}/photo`, {
    method: "DELETE",
  });
}

// Link externo pro Google Maps (abre o ponto em nova aba). Sem chave/custo.
export function googleMapsURL(lat: number, lng: number): string {
  return `https://www.google.com/maps?q=${lat},${lng}`;
}
