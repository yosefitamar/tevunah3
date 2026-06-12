import { api } from "./api";

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

// Papéis sugeridos pro vínculo de entidade (texto livre — a UI sugere estes).
export const INVOLVED_ROLE_SUGGESTIONS = [
  "AUTOR",
  "SUSPEITO",
  "PRESO",
  "VÍTIMA",
  "TESTEMUNHA",
  "ENVOLVIDO",
];

export type InvolvedEntity = {
  entity_id: string;
  name: string;
  kind: string;
  role: string;
  has_photo: boolean;
  version: number;
};

export type Incident = {
  id: string;
  type: IncidentType;
  occurred_on: string; // YYYY-MM-DD
  occurred_time?: string; // HH:MM
  ciops_record: string;
  intel_participation: boolean;
  has_photo: boolean;
  latitude?: number;
  longitude?: number;
  description: string;
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
  search?: string;
  intel?: boolean;
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
  if (opts.search) p.set("search", opts.search);
  if (opts.intel) p.set("intel", "1");
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

export function getIncident(id: string) {
  return api<{ incident: Incident }>(`/api/incidents/${encodeURIComponent(id)}`);
}

export type NewInvolvedInput = { entity_id: string; role?: string };

export type NewIncidentInput = {
  type: IncidentType;
  occurred_on: string;
  occurred_time?: string;
  ciops_record?: string;
  intel_participation?: boolean;
  latitude?: number | null;
  longitude?: number | null;
  description?: string;
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
  intel_participation?: boolean;
  latitude?: number | null;
  longitude?: number | null;
  description?: string;
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
