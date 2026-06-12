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

// Rótulo de listagem de pessoa: "NOME (VULGO)" quando há vulgo, senão só o nome.
export function personListLabel(name: string, aliases?: string[]): string {
  const alias = aliases && aliases.length > 0 ? aliases[0].trim() : "";
  if (alias) return `${name} (${alias})`;
  return name;
}

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

export type VehicleCategory = "car" | "motorcycle";

export const VEHICLE_CATEGORIES: VehicleCategory[] = ["car", "motorcycle"];

export const VEHICLE_CATEGORY_LABEL: Record<VehicleCategory, string> = {
  car: "CARRO",
  motorcycle: "MOTO",
};

export type VehicleAttrs = {
  category?: VehicleCategory;
  plate?: string;
  brand?: string;
  model?: string;
  color?: string;
  year?: number;
  chassis?: string;
  renavam?: string;
  has_photo?: boolean;
};

export type EntityAttrs =
  | PersonAttrs
  | OrganizationAttrs
  | PlaceAttrs
  | VehicleAttrs;

// Rótulo prioritário de um veículo: placa quando existe, senão o nome livre.
// Usado em selects de "alvo do vínculo".
export function vehiclePrimaryLabel(name: string, plate?: string): string {
  if (plate && plate.trim()) {
    if (name && name !== plate) return `${plate} · ${name}`;
    return plate;
  }
  return name;
}

// Rótulo de listagem de veículo no formato "MARCA MODELO COR - PLACA".
// Partes vazias são omitidas; sem nenhum atributo, cai no nome livre.
// O "-" separa o bloco identificação do bloco placa pra dar respiro visual.
export function vehicleListLabel(attrs?: VehicleAttrs, fallbackName?: string): string {
  const plate = attrs?.plate?.trim();
  const brand = attrs?.brand?.trim();
  const model = attrs?.model?.trim();
  const color = attrs?.color?.trim();
  const head = [brand, model, color].filter(Boolean).join(" ");
  if (head && plate) return `${head} - ${plate}`;
  if (head) return head;
  if (plate) return plate;
  return fallbackName ?? "";
}

// ────── Vínculos entre entidades ──────
//
// Catálogo de relações é fechado: tipos e pares (from_kind × to_kind) são
// validados no domain. UI usa o catálogo pra (a) filtrar opções no seletor
// conforme o par anchor↔target, (b) renderizar rótulos bidirecionais coerentes.

export type RelationType =
  | "associated_with"
  | "owns"
  | "spouse"
  | "relative"
  | "friend"
  | "colleague"
  | "partner"
  | "member_of"
  | "leader_of"
  | "employee_of"
  | "drives"
  | "frequents"
  | "subsidiary_of"
  | "partnership"
  | "based_at"
  | "father_of"
  | "mother_of"
  | "sibling"
  | "half_sibling";

// Rótulo "out" — perspectiva da entidade FROM. Usado nas edges do grafo
// (sempre desenhadas from→to) e em listagens de links saindo.
export const RELATION_LABEL: Record<RelationType, string> = {
  associated_with: "ASSOCIADO",
  owns: "PROPRIETÁRIO",
  spouse: "CÔNJUGE",
  relative: "PARENTE",
  friend: "AMIGO",
  colleague: "COLEGA",
  partner: "SÓCIO",
  member_of: "MEMBRO DE",
  leader_of: "LÍDER DE",
  employee_of: "FUNCIONÁRIO DE",
  drives: "CONDUTOR",
  frequents: "FREQUENTA",
  subsidiary_of: "SUBSIDIÁRIA DE",
  partnership: "PARCERIA",
  based_at: "SEDE EM",
  father_of: "PAI DE",
  mother_of: "MÃE DE",
  sibling: "IRMÃO(Ã)",
  half_sibling: "MEIO-IRMÃO(Ã)",
};

// Rótulo "in" — perspectiva da entidade TO. Para relações simétricas é igual
// ao "out". Para direcionais, inverte o sentido semântico
// (ex.: out=MEMBRO DE / in=TEM COMO MEMBRO).
export const RELATION_LABEL_IN: Record<RelationType, string> = {
  associated_with: "ASSOCIADO",
  owns: "PROPRIEDADE DE",
  spouse: "CÔNJUGE",
  relative: "PARENTE",
  friend: "AMIGO",
  colleague: "COLEGA",
  partner: "SÓCIO",
  member_of: "TEM COMO MEMBRO",
  leader_of: "LIDERADO POR",
  employee_of: "EMPREGA",
  drives: "CONDUZIDO POR",
  frequents: "FREQUENTADO POR",
  subsidiary_of: "CONTROLA",
  partnership: "PARCERIA",
  based_at: "SEDE DE",
  // Os dois mapeiam pra "FILHO(A) DE" semanticamente, mas no seletor de
  // vínculo apareceriam duplicados. Disambiguamos com a especificação do
  // parente.
  father_of: "FILHO(A) DO PAI",
  mother_of: "FILHO(A) DA MÃE",
  sibling: "IRMÃO(Ã)",
  half_sibling: "MEIO-IRMÃO(Ã)",
};

export const RELATION_TYPES: RelationType[] = [
  "associated_with",
  "owns",
  "spouse",
  "relative",
  "friend",
  "colleague",
  "partner",
  "member_of",
  "leader_of",
  "employee_of",
  "drives",
  "frequents",
  "subsidiary_of",
  "partnership",
  "based_at",
  "father_of",
  "mother_of",
  "sibling",
  "half_sibling",
];

// RelationDef descreve os kinds aceitos nas pontas canônicas (from × to) e se
// a relação é simétrica. UI usa esses dados pra montar o seletor.
export type RelationDef = {
  key: RelationType;
  symmetric: boolean;
  fromKinds: EntityKind[];
  toKinds: EntityKind[];
};

const ALL_KINDS: EntityKind[] = ["person", "organization", "place", "vehicle"];

export const RELATIONS: RelationDef[] = [
  { key: "associated_with", symmetric: true, fromKinds: ALL_KINDS, toKinds: ALL_KINDS },
  { key: "owns",            symmetric: false, fromKinds: ["person", "organization"], toKinds: ["vehicle"] },
  { key: "spouse",          symmetric: true,  fromKinds: ["person"], toKinds: ["person"] },
  { key: "relative",        symmetric: true,  fromKinds: ["person"], toKinds: ["person"] },
  { key: "friend",          symmetric: true,  fromKinds: ["person"], toKinds: ["person"] },
  { key: "colleague",       symmetric: true,  fromKinds: ["person"], toKinds: ["person"] },
  { key: "partner",         symmetric: true,  fromKinds: ["person"], toKinds: ["person"] },
  { key: "member_of",       symmetric: false, fromKinds: ["person"], toKinds: ["organization"] },
  { key: "leader_of",       symmetric: false, fromKinds: ["person"], toKinds: ["organization"] },
  { key: "employee_of",     symmetric: false, fromKinds: ["person"], toKinds: ["organization"] },
  { key: "drives",          symmetric: false, fromKinds: ["person"], toKinds: ["vehicle"] },
  { key: "frequents",       symmetric: false, fromKinds: ["person"], toKinds: ["place"] },
  { key: "subsidiary_of",   symmetric: false, fromKinds: ["organization"], toKinds: ["organization"] },
  { key: "partnership",     symmetric: true,  fromKinds: ["organization"], toKinds: ["organization"] },
  { key: "based_at",        symmetric: false, fromKinds: ["organization"], toKinds: ["place"] },
  { key: "father_of",       symmetric: false, fromKinds: ["person"], toKinds: ["person"] },
  { key: "mother_of",       symmetric: false, fromKinds: ["person"], toKinds: ["person"] },
  { key: "sibling",         symmetric: true,  fromKinds: ["person"], toKinds: ["person"] },
  { key: "half_sibling",    symmetric: true,  fromKinds: ["person"], toKinds: ["person"] },
];

const RELATIONS_BY_KEY: Record<RelationType, RelationDef> = Object.fromEntries(
  RELATIONS.map((r) => [r.key, r]),
) as Record<RelationType, RelationDef>;

// Devolve a label da relação na perspectiva da entidade consultada.
// direction="out" quando ela é o from; "in" quando é o to.
export function relationLabelFor(
  key: RelationType,
  direction: LinkDirection,
): string {
  return direction === "out" ? RELATION_LABEL[key] : RELATION_LABEL_IN[key];
}

// Opção apresentada no seletor de relação dentro do modal de adicionar link.
//   key:        relation_type canônico (vai pro DB).
//   label:      texto exibido (já na perspectiva do anchor).
//   anchorAsFrom: se true, o anchor vira `from` no insert; senão, o `other` vira `from`
//   (caso em que a UI precisa fazer POST no endpoint do `other`).
export type RelationOption = {
  key: RelationType;
  label: string;
  anchorAsFrom: boolean;
};

// relationsForPair lista as opções aceitas pra um par (anchorKind, otherKind)
// já com o rótulo na perspectiva do anchor e o flag de orientação canônica.
// Para relações simétricas, anchorAsFrom = true por padrão (irrelevante na
// semântica, mas mantém a inserção previsível).
export function relationsForPair(
  anchorKind: EntityKind,
  otherKind: EntityKind,
): RelationOption[] {
  const out: RelationOption[] = [];
  for (const r of RELATIONS) {
    const asFrom =
      r.fromKinds.includes(anchorKind) && r.toKinds.includes(otherKind);
    const asTo =
      !r.symmetric &&
      r.toKinds.includes(anchorKind) &&
      r.fromKinds.includes(otherKind);
    if (asFrom) {
      out.push({
        key: r.key,
        label: r.symmetric ? RELATION_LABEL[r.key] : RELATION_LABEL[r.key],
        anchorAsFrom: true,
      });
    } else if (asTo) {
      out.push({
        key: r.key,
        label: RELATION_LABEL_IN[r.key],
        anchorAsFrom: false,
      });
    }
  }
  return out;
}

// Utilitário pra consultar a definição canônica de uma relação a partir do
// key (útil em raros pontos da UI que precisam saber se é simétrica).
export function relationDef(key: RelationType): RelationDef | undefined {
  return RELATIONS_BY_KEY[key];
}

export type LinkDirection = "out" | "in";

// ────── Catálogo de relações sociais/familiares (UI-friendly) ──────
//
// Mapa de rótulos cotidianos → (relation_type canônico + direção). Usado no
// seletor de vínculos do wizard de pessoa: o usuário pensa "FILHO(A) DESTA MÃE"
// e o sistema resolve pro link correto (mother_of de OTHER → NEW).
//
// `anchorAsFrom = true`  → cria o link com a entidade nova como `from`
// `anchorAsFrom = false` → cria com a entidade externa (other) como `from`

export type FamilyOption = {
  id: string;
  label: string;
  relation: RelationType;
  anchorAsFrom: boolean;
};

export const FAMILY_OPTIONS: FamilyOption[] = [
  { id: "spouse",         label: "ESPOSO(A) / CÔNJUGE",  relation: "spouse",         anchorAsFrom: true },
  { id: "is_father",      label: "É PAI DE",             relation: "father_of",      anchorAsFrom: true },
  { id: "is_mother",      label: "É MÃE DE",             relation: "mother_of",      anchorAsFrom: true },
  { id: "son_of_father",  label: "FILHO(A) DESTE PAI",   relation: "father_of",      anchorAsFrom: false },
  { id: "son_of_mother",  label: "FILHO(A) DESTA MÃE",   relation: "mother_of",      anchorAsFrom: false },
  { id: "sibling",        label: "IRMÃO(Ã)",             relation: "sibling",        anchorAsFrom: true },
  { id: "half_sibling",   label: "MEIO-IRMÃO(Ã)",        relation: "half_sibling",   anchorAsFrom: true },
  { id: "relative",       label: "PARENTE (TIO, PRIMO…)", relation: "relative",      anchorAsFrom: true },
  { id: "friend",         label: "AMIGO(A)",             relation: "friend",         anchorAsFrom: true },
  { id: "colleague",      label: "COLEGA",               relation: "colleague",      anchorAsFrom: true },
  { id: "partner",        label: "SÓCIO(A)",             relation: "partner",        anchorAsFrom: true },
  { id: "associated",     label: "ASSOCIADO",            relation: "associated_with", anchorAsFrom: true },
];

// Resumo dos attrs de veículo embutido no payload de link. Backend popula
// quando a ponta correspondente é kind=vehicle. Permite renderizar
// "MARCA MODELO COR - PLACA" sem fazer fetch extra por linha.
export type VehicleSummary = {
  plate?: string;
  brand?: string;
  model?: string;
  color?: string;
  category?: VehicleCategory;
};

export type EntityLink = {
  id: string;
  direction: LinkDirection;
  from_entity_id: string;
  from_kind: EntityKind;
  from_name: string;
  from_vehicle?: VehicleSummary;
  to_entity_id: string;
  to_kind: EntityKind;
  to_name: string;
  to_vehicle?: VehicleSummary;
  relation_type: RelationType;
  valid_from?: string;
  valid_to?: string;
  note?: string;
  created_at: string;
  created_by: string;
};

// Devolve { id, kind, name, vehicle } da entidade do "outro lado" do link
// na perspectiva da entidade consultada. `vehicle` só vem quando o outro
// lado é kind=vehicle e o backend incluiu o summary.
export function linkOtherSide(l: EntityLink): {
  id: string;
  kind: EntityKind;
  name: string;
  vehicle?: VehicleSummary;
} {
  if (l.direction === "out") {
    return {
      id: l.to_entity_id,
      kind: l.to_kind,
      name: l.to_name,
      vehicle: l.to_vehicle,
    };
  }
  return {
    id: l.from_entity_id,
    kind: l.from_kind,
    name: l.from_name,
    vehicle: l.from_vehicle,
  };
}

// ────── Grafo (subrede multi-hop) ──────
//
// Espelha o JSON de GET /api/entities/{id}/graph?depth=N. Inclui classification
// no nó porque a UI quer pintar a pill, e summary de veículo só quando a ponta
// é kind=vehicle. Edges não trazem datas de validade — o grafo é "agora".

export type GraphNode = {
  id: string;
  kind: EntityKind;
  name: string;
  classification: EntityClassification;
  version: number;
  has_photo: boolean;
  alias?: string;        // person: primeiro vulgo; org: primeira sigla
  orcrim_alias?: string; // person: alias da ORCRIM associada
  vehicle?: VehicleSummary;
};

export type GraphEdge = {
  id: string;
  from: string;
  to: string;
  relation_type: RelationType;
  note?: string;
};

export type EntityGraph = {
  center_id: string;
  depth: number;
  nodes: GraphNode[];
  edges: GraphEdge[];
  truncated: boolean;
};

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
