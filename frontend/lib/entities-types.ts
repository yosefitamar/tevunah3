// Types do módulo Entidades. Espelham o envelope JSON enviado pelo backend
// (cmd/server/entities.go) e os enums do domain (internal/entities).
//
// Polimorfismo: o campo `attrs` carrega o bloco específico do kind. No request
// (create/update), envia-se o bloco com a chave do kind (`person`, `organization`,
// `place`). Na resposta, sempre vem em `attrs`.

export type EntityKind = "person" | "organization" | "place";

export const ENTITY_KIND_LABEL: Record<EntityKind, string> = {
  person: "PESSOA",
  organization: "ORGANIZAÇÃO",
  place: "LUGAR",
};

export const ENTITY_KINDS: EntityKind[] = ["person", "organization", "place"];

// Classificação 1..4 (alinhada com clearance_level 1..5 do usuário).
export type EntityClassification = 1 | 2 | 3 | 4;

export const CLASSIFICATION_LABEL: Record<EntityClassification, string> = {
  1: "PÚBLICO",
  2: "RESTRITO",
  3: "CONFIDENCIAL",
  4: "SECRETO",
};

// Classe visual da pill (reaproveita o sistema do users):
//   - active (verde)  = público
//   - hold   (âmbar)  = restrito
//   - warn   (laranja)= confidencial
//   - cold   (cinza)  = secreto (deliberadamente discreto)
export const CLASSIFICATION_PILL: Record<EntityClassification, string> = {
  1: "active",
  2: "hold",
  3: "warn",
  4: "cold",
};

export const CLASSIFICATIONS: EntityClassification[] = [1, 2, 3, 4];

// Gênero — opções fechadas no MVP. Pode ser expandido depois.
export type Gender = "M" | "F";
export const GENDER_LABEL: Record<Gender, string> = {
  M: "MASCULINO",
  F: "FEMININO",
};
export const GENDERS: Gender[] = ["M", "F"];

export type PersonAttrs = {
  aliases?: string[];
  gender?: Gender | string; // string permite tolerar valores antigos
  date_of_birth?: string;   // YYYY-MM-DD
  mother_name?: string;
  cpf?: string;
  has_photo?: boolean;
  orcrim_id?: string;
  orcrim_name?: string;
  orcrim_alias?: string;
};

export type OrganizationAttrs = {
  aliases?: string[]; // siglas; primeiro elemento é a sigla primária
  legal_name?: string;
  tax_id?: string;
  founded_at?: string; // YYYY-MM-DD
};

// Rótulo prioritário de uma organização: sigla primária se existir, senão nome.
export function orgPrimaryLabel(name: string, aliases?: string[]): string {
  const primary = aliases && aliases.length > 0 ? aliases[0].trim() : "";
  if (primary && primary !== name) return `${primary} · ${name}`;
  return name;
}

export type PlaceAttrs = {
  address?: string;
  country?: string;
  region?: string;
  latitude?: number;
  longitude?: number;
  has_photo?: boolean;
};

export type EntityAttrs = PersonAttrs | OrganizationAttrs | PlaceAttrs;

// GalleryPhoto = foto adicional anexada à entidade (distinta da foto primária).
// O binário é servido em /api/entities/{id}/photos/{photo.id}.
export type GalleryPhoto = {
  id: string;
  caption: string;
  mime: string;
  ord: number;
  created_at: string;
  created_by: string;
  updated_at: string;
  updated_by: string;
};

export type Entity = {
  id: string;
  kind: EntityKind;
  name: string;
  description?: string;
  classification: EntityClassification;
  version: number;
  tags: string[];
  created_at: string;
  created_by: string;
  updated_at: string;
  updated_by: string;
  deleted_at?: string | null;
  attrs?: EntityAttrs;
  photos?: GalleryPhoto[];
};

export function isPerson(e: Entity): e is Entity & { attrs?: PersonAttrs } {
  return e.kind === "person";
}
export function isOrganization(
  e: Entity,
): e is Entity & { attrs?: OrganizationAttrs } {
  return e.kind === "organization";
}
export function isPlace(e: Entity): e is Entity & { attrs?: PlaceAttrs } {
  return e.kind === "place";
}
