// SIPOM (sipom.pm.ce.gov.br): destino do envio das ocorrências do relatório
// operacional. Catálogo de naturezas, natureza fixada pelo analista e o
// de-para relatório → SIPOM (Admin).

import { api } from "./api";
import type { OpsOfficer, SipomIntelOfficer, SipomOccurrence, SipomPending, SipomRef } from "./ops-reports-api";

type SipomChange = { sipom: SipomOccurrence; officers: OpsOfficer[] };

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

// ─── Fase 2: procedimento e materiais ───

export type SipomDrogaRef = SipomRef & { unidade: string };

/** Listas dos selects de procedimento e materiais. */
export type SipomCatalogo = {
  procedimentos: SipomRef[];
  delegacias: SipomRef[];
  delegados: SipomRef[];
  arma_tipos: SipomRef[];
  arma_marcas: SipomRef[];
  arma_calibres: SipomRef[];
  drogas: SipomDrogaRef[];
  veiculo_tipos: SipomRef[];
  veiculo_cores: SipomRef[];
  celular_marcas: SipomRef[];
};

// As listas não mudam durante a sessão: uma busca serve para todas as fichas.
let catalogoCache: Promise<SipomCatalogo> | null = null;
export function getSipomCatalogo(): Promise<SipomCatalogo> {
  catalogoCache ??= api<SipomCatalogo>("/api/sipom/catalogo").catch((e) => {
    catalogoCache = null;
    throw e;
  });
  return catalogoCache;
}

/** Marca/modelo de veículo (tabela DENATRAN), por texto. */
export function searchSipomMarcasModelos(q: string) {
  return api<{ items: SipomRef[] }>(`/api/sipom/marcas-modelos?q=${encodeURIComponent(q)}`);
}

export type SipomProcedimentoInput =
  | { procedimento_id?: number; delegacia_id?: number; delegado_id?: number }
  | { reset: true };

/** Fixa o procedimento (tipo, delegacia, delegado) ou devolve ao automático. */
export function setOpsSipomProcedimento(occId: string, input: SipomProcedimentoInput) {
  return api<{ sipom: SipomOccurrence; officers: OpsOfficer[] }>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/procedimento`,
    { method: "PUT", body: JSON.stringify(input) },
  );
}

export type SipomMaterialInput =
  | { kind: "armas"; tipo_id: number; marca_id?: number; calibre_id?: number }
  | { kind: "drogas"; droga_id: number; quantidade: number }
  | { kind: "veiculos"; tipo_codigo: number; cor_codigo: number; marca_modelo_codigo: number; situacao: 1 | 2 }
  | { kind: "armas" | "drogas" | "veiculos"; reset: true };

/** Fixa a tradução de um material (pela posição na lista) ou devolve ao automático. */
export function setOpsSipomMaterial(occId: string, index: number, input: SipomMaterialInput) {
  const { kind, ...body } = input;
  return api<{ sipom: SipomOccurrence; officers: OpsOfficer[] }>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/materiais/${kind}/${index}`,
    { method: "PUT", body: JSON.stringify(body) },
  );
}

/**
 * Confiança da regra do de-para. "com_arma" e "com_droga" aplicam sozinhas
 * quando a ficha traz a apreensão correspondente (arma de fogo / droga); sem
 * ela, valem como "sugerida".
 */
export type SipomConfianca = "direta" | "sugerida" | "com_arma" | "com_droga";

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
  | { field: "bairro"; bairro_id: number | null }
  | { field: "opm"; opm_id: number | null }
  | { field: "endereco"; logradouro: string; numeral: string }
  | { field: "endereco"; reset: true }
  | { field: "composicao"; equipes: string[] }
  | { field: "composicao"; reset: true };

/**
 * Referência criada (ou trocada) pela escolha da área: daqui em diante as
 * ocorrências do mesmo lugar já entram com ela. `resolved` são as outras
 * ocorrências do acervo que deixaram de ter pendência de área.
 */
export type SipomAreaRule = {
  city: string;
  neighborhood: string;
  area_id: number;
  area: string;
  resolved: number;
};

/** Fixa um campo da tradução (ou devolve ao automático com null/reset). */
export function setOpsSipomField(occId: string, input: SipomFieldInput) {
  const { field, ...body } = input;
  return api<{ sipom: SipomOccurrence; officers: OpsOfficer[]; area_rule?: SipomAreaRule | null }>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/${field}`,
    { method: "PUT", body: JSON.stringify(body) },
  );
}

// ─── Composição da inteligência ───

/** Agentes que podem entrar na composição (marcados no cadastro) e as funções do policiamento. */
export function listSipomIntelAgentes() {
  return api<{ items: SipomIntelOfficer[]; funcoes: SipomRef[] }>("/api/sipom/inteligencia/agentes");
}

export type SipomIntelInput = { agentes: { user_id: string; funcao_id: number }[] } | { reset: true };

/** Grava quem da agência entra na composição, ou volta ao cadastro dos agentes. */
export function setSipomIntel(occId: string, input: SipomIntelInput) {
  return api<{ sipom: SipomOccurrence; officers: OpsOfficer[] }>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/inteligencia`,
    { method: "PUT", body: JSON.stringify(input) },
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

/** Confirma em lote o sexo sugerido dos envolvidos das ocorrências. */
export function confirmSipomSexos(ids: string[]) {
  return api<{ confirmed: number; occurrences: number }>("/api/ops-occurrences/sipom/sexo/confirm", {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

/**
 * Corpo do POST ao SIPOM (contrato em docs/sipom-integracao.md). Sem
 * `withPhotos` as fotos vêm só com o tipo, sem o base64 — a leitura do
 * conteúdo das fotos de dossiê é auditada, por isso é pedida à parte.
 */
export function getSipomPayload(occId: string, withPhotos = false) {
  return api<{ ready: boolean; pendencias: SipomPending[]; payload: unknown | null }>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/payload${withPhotos ? "?fotos=1" : ""}`,
  );
}

/**
 * Marca (ou desmarca) a ocorrência como lançada no SIPOM — só sem a API
 * configurada, quando o analista lança por lá.
 */
export function setSipomSent(occId: string, sent: boolean) {
  return api<{ sent_at: string | null }>(`/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/sent`, {
    method: "PUT",
    body: JSON.stringify({ sent }),
  });
}

// ─── API do SIPOM ───

/**
 * Envia a ocorrência ao SIPOM pela API. Reenviar é seguro: o SIPOM responde
 * "sem_alteracao" quando nada mudou e "atualizado" quando a ficha mudou.
 */
export function enviarSipom(occId: string) {
  return api<{ sipom: SipomOccurrence }>(`/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/enviar`, {
    method: "POST",
  });
}

/** Testa as credenciais da API do SIPOM (não grava nada lá). */
export function pingSipom() {
  return api<{ configurado: boolean; ok?: boolean; cliente?: string; erro?: string; url?: string; ms?: number }>(
    "/api/admin/sipom/ping",
  );
}

/** Bairros do catálogo do SIPOM na cidade. */
export function listSipomBairros(cidadeId: number) {
  return api<{ items: SipomRef[] }>(`/api/sipom/bairros?cidade_id=${cidadeId}`);
}

/** Sexo do envolvido para o SIPOM, quando o dossiê não diz (0 limpa). */
export function setSipomSexo(occId: string, personId: string, sexo: 0 | 1 | 2) {
  return api<SipomChange>(
    `/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/pessoas/${encodeURIComponent(personId)}/sexo`,
    { method: "PUT", body: JSON.stringify({ sexo }) },
  );
}

/** Ano de fabricação e do modelo do veículo (pela posição na lista). */
/**
 * Placa do veículo que o relatório omitiu ou escreveu errado; ou, com placa
 * vazia e `semPlaca`, a confirmação de que ela não pôde ser identificada.
 */
export function setSipomVehiclePlate(occId: string, index: number, placa: string, semPlaca = false) {
  return api<SipomChange>(`/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/veiculos/${index}/placa`, {
    method: "PUT",
    body: JSON.stringify({ placa, sem_placa: semPlaca }),
  });
}

export function setSipomVehicleYears(occId: string, index: number, anoFabricacao: number | null, anoModelo: number | null) {
  return api<SipomChange>(`/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/veiculos/${index}/ano`, {
    method: "PUT",
    body: JSON.stringify({ ano_fabricacao: anoFabricacao, ano_modelo: anoModelo }),
  });
}
