// Orientações dos analistas para a IA nas pendências do SIPOM. Pendente vale
// só para quem escreveu; aprovada pelo administrador vale para todos.

import { api } from "./api";

export type GuidanceTopic = "natureza" | "bairro" | "sexo" | "apreensoes" | "recusa" | "geral";
export type GuidanceStatus = "pendente" | "aprovada" | "recusada" | "desativada";

export type Guidance = {
  id: string;
  topic: GuidanceTopic;
  cidade_id: number | null;
  cidade: string;
  body: string;
  status: GuidanceStatus;
  source_occurrence_id: string | null;
  created_by: string;
  author_name: string;
  created_at: string;
  decider_name: string;
  decided_at: string | null;
  decision_note: string;
  uses: number;
  last_used_at: string | null;
  mine: boolean;
};

export const GUIDANCE_TOPIC_LABEL: Record<GuidanceTopic, string> = {
  natureza: "NATUREZA",
  bairro: "BAIRRO",
  sexo: "SEXO DOS ENVOLVIDOS",
  apreensoes: "APREENSÕES",
  recusa: "RECUSA DO SIPOM",
  geral: "GERAL",
};

export const GUIDANCE_STATUS_LABEL: Record<GuidanceStatus, string> = {
  pendente: "SÓ PARA VOCÊ · AGUARDA APROVAÇÃO",
  aprovada: "APROVADA · VALE PARA TODOS",
  recusada: "RECUSADA",
  desativada: "DESATIVADA",
};

/** Tema da orientação a partir do tipo da sugestão da IA. */
export const TOPIC_OF_KIND: Record<string, GuidanceTopic> = {
  natureza: "natureza",
  bairro: "bairro",
  sexo: "sexo",
  arma: "apreensoes",
  droga: "apreensoes",
  veiculo: "apreensoes",
  veiculo_placa: "apreensoes",
  veiculo_outros: "apreensoes",
  remover: "apreensoes",
  adicionar: "apreensoes",
  veiculo_ano: "recusa",
  veiculo_como_outros: "recusa",
  recusa_info: "recusa",
  campo: "geral",
  pessoa_dados: "geral",
  pessoa_remover: "geral",
  delegacia: "geral",
};

export type GuidanceInput = {
  topic: GuidanceTopic;
  cidade_id: number | null;
  body: string;
  occurrence_id?: string;
};

export function listGuidance() {
  return api<{ items: Guidance[]; can_approve: boolean; pending: number }>("/api/ai/orientacoes");
}

export function createGuidance(input: GuidanceInput) {
  return api<{ item: Guidance }>("/api/ai/orientacoes", { method: "POST", body: JSON.stringify(input) });
}

export function updateGuidance(id: string, input: GuidanceInput) {
  return api<{ item: Guidance }>(`/api/ai/orientacoes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function decideGuidance(id: string, decisao: "aprovar" | "recusar" | "desativar", nota = "") {
  return api<{ item: Guidance }>(`/api/ai/orientacoes/${encodeURIComponent(id)}/decisao`, {
    method: "POST",
    body: JSON.stringify({ decisao, nota }),
  });
}
