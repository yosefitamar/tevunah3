import { api } from "./api";

// Registro de consulta: a pesquisa que o agente fez num sistema externo a
// pedido de alguém, guardada como justificativa para auditoria.
export type Consulta = {
  id: string;
  performed_at: string; // ISO 8601
  performed_on: string; // YYYY-MM-DD no fuso da agência
  performed_time: string; // HH:MM no fuso da agência
  system: string; // SISTEMA consultado
  target: string; // ALVO da pesquisa
  requester: string; // SOLICITANTE
  requester_org: string; // ÓRGÃO do solicitante
  reference_doc: string; // documento de referência
  reason: string; // MOTIVO
  result: string; // RESULTADO
  version: number;
  created_at: string;
  created_by: string;
  created_by_code: string;
  created_by_name: string;
  updated_at: string;
};

export type ConsultasList = {
  items: Consulta[];
  total: number;
  limit: number;
  offset: number;
};

export type ListConsultasOpts = {
  limit?: number;
  offset?: number;
  search?: string;
  date_from?: string;
  date_to?: string;
  sort_by?: string;
  sort_dir?: "asc" | "desc";
};

function qs(opts: ListConsultasOpts): string {
  const p = new URLSearchParams();
  if (opts.limit) p.set("limit", String(opts.limit));
  if (opts.offset) p.set("offset", String(opts.offset));
  if (opts.search) p.set("search", opts.search);
  if (opts.date_from) p.set("date_from", opts.date_from);
  if (opts.date_to) p.set("date_to", opts.date_to);
  if (opts.sort_by) p.set("sort_by", opts.sort_by);
  if (opts.sort_dir) p.set("sort_dir", opts.sort_dir);
  const s = p.toString();
  return s ? "?" + s : "";
}

export function listConsultas(opts: ListConsultasOpts = {}) {
  return api<ConsultasList>(`/api/consultas${qs(opts)}`);
}

// Sistemas já usados em registros anteriores — sugestões do campo SISTEMA.
export function listConsultaSystems() {
  return api<{ items: string[] }>(`/api/consultas/sistemas`);
}

// Criação e edição gravam o registro inteiro.
export type ConsultaInput = {
  performed_on: string;
  performed_time: string;
  system: string;
  target: string;
  requester: string;
  requester_org: string;
  reference_doc: string;
  reason: string;
  result: string;
};

export function createConsulta(input: ConsultaInput) {
  return api<{ consulta: Consulta }>(`/api/consultas`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateConsulta(id: string, input: ConsultaInput) {
  return api<{ consulta: Consulta }>(`/api/consultas/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteConsulta(id: string) {
  return api<void>(`/api/consultas/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}
