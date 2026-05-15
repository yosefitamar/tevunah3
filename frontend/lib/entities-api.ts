import { api } from "./api";
import type {
  Entity,
  EntityClassification,
  EntityGraph,
  EntityKind,
  EntityLink,
  GalleryPhoto,
  OrganizationAttrs,
  PersonAddress,
  PersonAttrs,
  PlaceAttrs,
  RelationType,
  VehicleAttrs,
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
  vehicle?: VehicleAttrs;
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
  vehicle?: VehicleAttrs;
};

// ────── Vínculos ──────

export type ListLinksResult = { items: EntityLink[] };

export type NewLinkInput = {
  to_entity_id: string;
  relation_type: RelationType;
  valid_from?: string;
  valid_to?: string;
  note?: string;
};

export function listEntityLinks(entityID: string) {
  return api<ListLinksResult>(
    `/api/entities/${encodeURIComponent(entityID)}/links`,
  );
}

export function createEntityLink(entityID: string, input: NewLinkInput) {
  return api<{ link: EntityLink }>(
    `/api/entities/${encodeURIComponent(entityID)}/links`,
    {
      method: "POST",
      body: JSON.stringify(input),
    },
  );
}

export function deleteEntityLink(entityID: string, linkID: string) {
  return api<void>(
    `/api/entities/${encodeURIComponent(entityID)}/links/${encodeURIComponent(linkID)}`,
    { method: "DELETE" },
  );
}

// ────── Grafo (subrede multi-hop) ──────

export type GraphDepth = 1 | 2 | 3;

export function getEntityGraph(entityID: string, depth: GraphDepth) {
  return api<EntityGraph>(
    `/api/entities/${encodeURIComponent(entityID)}/graph?depth=${depth}`,
  );
}

// ────── Endereços de pessoa ──────

export type ListAddressesResult = { items: PersonAddress[] };

export type AddressPayload = {
  label?: string;
  cep?: string;
  street?: string;
  number?: string;
  complement?: string;
  neighborhood?: string;
  city?: string;
  state?: string;
};

export function listPersonAddresses(entityID: string) {
  return api<ListAddressesResult>(
    `/api/entities/${encodeURIComponent(entityID)}/addresses`,
  );
}

export function createPersonAddress(entityID: string, input: AddressPayload) {
  return api<{ address: PersonAddress }>(
    `/api/entities/${encodeURIComponent(entityID)}/addresses`,
    { method: "POST", body: JSON.stringify(input) },
  );
}

export function updatePersonAddress(
  entityID: string,
  addressID: string,
  input: AddressPayload,
) {
  return api<{ address: PersonAddress }>(
    `/api/entities/${encodeURIComponent(entityID)}/addresses/${encodeURIComponent(addressID)}`,
    { method: "PATCH", body: JSON.stringify(input) },
  );
}

export function deletePersonAddress(entityID: string, addressID: string) {
  return api<void>(
    `/api/entities/${encodeURIComponent(entityID)}/addresses/${encodeURIComponent(addressID)}`,
    { method: "DELETE" },
  );
}

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

// ─────────── Gallery (fotos adicionais) ───────────
//
// URL pra exibir uma foto da galeria. `v` força bust quando metadados mudam
// (caption/ord). O binário em si é imutável, então o updated_at do registro
// é suficiente como busting.
export function galleryPhotoURL(entityID: string, photoID: string, v?: string): string {
  const bust = v ? `?v=${encodeURIComponent(v)}` : "";
  return `/api/entities/${encodeURIComponent(entityID)}/photos/${encodeURIComponent(photoID)}${bust}`;
}

export async function uploadGalleryPhoto(
  entityID: string,
  file: File,
  caption: string,
  ord?: number,
): Promise<GalleryPhoto> {
  const fd = new FormData();
  fd.append("photo", file);
  fd.append("caption", caption);
  if (ord != null) fd.append("ord", String(ord));
  const res = await fetch(`/api/entities/${encodeURIComponent(entityID)}/photos`, {
    method: "POST",
    credentials: "include",
    body: fd,
  });
  let body: { success: boolean; data?: { photo: GalleryPhoto }; message?: string } = {
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
  return (body.data as { photo: GalleryPhoto }).photo;
}

export function updateGalleryPhoto(
  entityID: string,
  photoID: string,
  caption: string,
  ord?: number,
) {
  return api<{ photo: GalleryPhoto }>(
    `/api/entities/${encodeURIComponent(entityID)}/photos/${encodeURIComponent(photoID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ caption, ord }),
    },
  );
}

export function deleteGalleryPhoto(entityID: string, photoID: string) {
  return api<void>(
    `/api/entities/${encodeURIComponent(entityID)}/photos/${encodeURIComponent(photoID)}`,
    { method: "DELETE" },
  );
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
