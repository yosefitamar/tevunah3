import { api } from "./api";
import type {
  Entity,
  EntityClassification,
  EntityKind,
  OrganizationAttrs,
  PersonAttrs,
  PlaceAttrs,
} from "./entities-types";

export type EntitiesList = {
  items: Entity[];
  total: number;
  limit: number;
  offset: number;
};

export type ListEntitiesOpts = {
  limit?: number;
  offset?: number;
  search?: string;
  kind?: EntityKind | "";
  classification?: number; // 0 = todos
  tag?: string;
  sort_by?: "name" | "kind" | "classification" | "created_at" | "updated_at";
  sort_dir?: "asc" | "desc";
  deleted?: "only"; // ativa visão Lixeira (apenas soft-deletados)
};

export type NewEntityInput = {
  kind: EntityKind;
  name: string;
  description?: string;
  classification: EntityClassification;
  tags?: string[];
  person?: PersonAttrs;
  organization?: OrganizationAttrs;
  place?: PlaceAttrs;
};

export type UpdateEntityInput = {
  version: number;
  name?: string;
  description?: string;
  classification?: EntityClassification;
  tags?: string[];
  person?: PersonAttrs;
  organization?: OrganizationAttrs;
  place?: PlaceAttrs;
};

function qs(opts: ListEntitiesOpts): string {
  const p = new URLSearchParams();
  if (opts.limit) p.set("limit", String(opts.limit));
  if (opts.offset) p.set("offset", String(opts.offset));
  if (opts.search) p.set("search", opts.search);
  if (opts.kind) p.set("kind", opts.kind);
  if (opts.classification) p.set("classification", String(opts.classification));
  if (opts.tag) p.set("tag", opts.tag);
  if (opts.sort_by) p.set("sort_by", opts.sort_by);
  if (opts.sort_dir) p.set("sort_dir", opts.sort_dir);
  if (opts.deleted) p.set("deleted", opts.deleted);
  const s = p.toString();
  return s ? "?" + s : "";
}

export function listEntities(opts: ListEntitiesOpts = {}) {
  return api<EntitiesList>(`/api/entities${qs(opts)}`);
}

export function getEntity(id: string) {
  return api<{ entity: Entity }>(`/api/entities/${encodeURIComponent(id)}`);
}

export function createEntity(input: NewEntityInput) {
  return api<{ entity: Entity }>(`/api/entities`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateEntity(id: string, input: UpdateEntityInput) {
  return api<{ entity: Entity }>(`/api/entities/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deleteEntity(id: string, reason?: string) {
  return api<void>(`/api/entities/${encodeURIComponent(id)}`, {
    method: "DELETE",
    body: JSON.stringify({ reason: reason ?? "" }),
  });
}

export function restoreEntity(id: string, reason?: string) {
  return api<{ entity: Entity }>(`/api/entities/${encodeURIComponent(id)}/restore`, {
    method: "POST",
    body: JSON.stringify({ reason: reason ?? "" }),
  });
}

// URL pra exibir a foto. O backend serve com cache curto (5 min); o querystring
// `v` força bust quando o entity.version muda.
export function photoURL(id: string, version: number): string {
  return `/api/entities/${encodeURIComponent(id)}/photo?v=${version}`;
}

export async function uploadEntityPhoto(id: string, file: File): Promise<Entity> {
  const fd = new FormData();
  fd.append("photo", file);
  const res = await fetch(`/api/entities/${encodeURIComponent(id)}/photo`, {
    method: "POST",
    credentials: "include",
    body: fd,
  });
  let body: { success: boolean; data?: { entity: Entity }; message?: string } = {
    success: false,
  };
  try {
    body = await res.json();
  } catch {
    // ignore — talvez não tenha corpo JSON
  }
  if (!res.ok) {
    const err = new Error(body.message ?? `HTTP ${res.status}`) as Error & {
      status: number;
    };
    err.status = res.status;
    throw err;
  }
  return (body.data as { entity: Entity }).entity;
}

export function deleteEntityPhoto(id: string) {
  return api<void>(`/api/entities/${encodeURIComponent(id)}/photo`, {
    method: "DELETE",
  });
}

// ─────────── Duplicates (pessoas) ───────────

export type PersonDuplicate = {
  id: string;
  name: string;
  mother_name?: string;
  date_of_birth?: string;
  score: number; // 1..3
  matched_fields: string[]; // "name" | "mother_name" | "date_of_birth"
};

export type DuplicatesResult = {
  cpf_taken_by?: PersonDuplicate;
  matches: PersonDuplicate[];
};

export type DuplicatesQuery = {
  name?: string;
  mother_name?: string;
  date_of_birth?: string;
  cpf?: string;
  exclude_id?: string;
};

export function findPersonDuplicates(q: DuplicatesQuery) {
  const p = new URLSearchParams();
  if (q.name) p.set("name", q.name);
  if (q.mother_name) p.set("mother_name", q.mother_name);
  if (q.date_of_birth) p.set("date_of_birth", q.date_of_birth);
  if (q.cpf) p.set("cpf", q.cpf);
  if (q.exclude_id) p.set("exclude_id", q.exclude_id);
  return api<DuplicatesResult>(`/api/entities/persons/duplicates?${p.toString()}`);
}
