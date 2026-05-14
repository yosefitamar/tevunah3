// Types do módulo Entidades. Espelham o envelope JSON enviado pelo backend
// (cmd/server/entities.go) e os enums do domain (internal/entities).
//
// Polimorfismo: o campo `attrs` carrega o bloco específico do kind. No request
// (create/update), envia-se o bloco com a chave do kind (`person`, `organization`,
// `place`). Na resposta, sempre vem em `attrs`.

export type EntityKind = "person" | "organization" | "place" | "vehicle";

export const ENTITY_KIND_LABEL: Record<EntityKind, string> = {
  person: "PESSOA",
  organization: "ORGANIZAÇÃO",
  place: "LUGAR",
  vehicle: "VEÍCULO",
};

export const ENTITY_KINDS: EntityKind[] = [
  "person",
  "organization",
  "place",
  "vehicle",
];

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

export type PersonAddress = {
  id: string;
  label?: string;
  cep?: string;
  street?: string;
  number?: string;
  complement?: string;
  neighborhood?: string;
  city?: string;
  state?: string;
  created_at: string;
  created_by: string;
  updated_at: string;
  updated_by: string;
};

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
  addresses?: PersonAddress[];
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

export type VehicleAttrs = {
  plate?: string;
  brand?: string;
  model?: string;
  color?: string;
  year?: number;
  chassis?: string;
  renavam?: string;
};

export type EntityAttrs =
  | PersonAttrs
  | OrganizationAttrs
  | PlaceAttrs
  | VehicleAttrs;

// Rótulo prioritário de um veículo: placa quando existe, senão o nome livre.
// Usado em listagens e em selects de "alvo do vínculo".
export function vehiclePrimaryLabel(name: string, plate?: string): string {
  if (plate && plate.trim()) {
    if (name && name !== plate) return `${plate} · ${name}`;
    return plate;
  }
  return name;
}

// ────── Vínculos entre entidades ──────

export type RelationType = "owns" | "associated_with";

export const RELATION_LABEL: Record<RelationType, string> = {
  owns: "PROPRIETÁRIO",
  associated_with: "ASSOCIADO",
};

export const RELATION_TYPES: RelationType[] = ["owns", "associated_with"];

export type LinkDirection = "out" | "in";

export type EntityLink = {
  id: string;
  direction: LinkDirection;
  from_entity_id: string;
  from_kind: EntityKind;
  from_name: string;
  to_entity_id: string;
  to_kind: EntityKind;
  to_name: string;
  relation_type: RelationType;
  valid_from?: string;
  valid_to?: string;
  note?: string;
  created_at: string;
  created_by: string;
};

// Devolve { id, kind, name } da entidade do "outro lado" do link na
// perspectiva da entidade consultada.
export function linkOtherSide(l: EntityLink): {
  id: string;
  kind: EntityKind;
  name: string;
} {
  if (l.direction === "out") {
    return { id: l.to_entity_id, kind: l.to_kind, name: l.to_name };
  }
  return { id: l.from_entity_id, kind: l.from_kind, name: l.from_name };
}

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
export function isVehicle(e: Entity): e is Entity & { attrs?: VehicleAttrs } {
  return e.kind === "vehicle";
}
