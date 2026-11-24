// Sugestões da IA para as pendências que travam o envio ao SIPOM. A IA só
// sugere: cada sugestão confirmada pelo analista passa pelo endpoint de
// correção de sempre (mesma validação, auditoria e termos aprendidos).

import { api } from "./api";
import type { OpsOfficer, SipomChangeExtra, SipomOccurrence } from "./ops-reports-api";
import {
  addSipomMaterial,
  type SipomNewMaterial,
  setOpsSipomField,
  setOpsSipomMaterial,
  setOpsSipomNatureza,
  setSipomMaterialRemoved,
  setSipomSexo,
  setSipomVehiclePlate,
} from "./sipom-api";

export type SipomAIKind = "natureza" | "bairro" | "sexo" | "arma" | "droga" | "veiculo" | "veiculo_placa" | "veiculo_outros" | "remover" | "adicionar";

export type SipomAISuggestion = {
  kind: SipomAIKind;
  /** Pendência que a sugestão resolve. */
  code: string;
  target: string;
  person_id?: string;
  index?: number;
  value: string;
  /** O que a ficha tem hoje (sugestão do de-para/censo). */
  current?: string;
  /** A IA concorda com a sugestão automática que já estava na ficha. */
  agrees?: boolean;
  confianca: "alta" | "media" | "baixa";
  motivo: string;
  applicable: boolean;
  apply?: Record<string, number | string>;
  /** Orientação de analista que a IA diz ter seguido. */
  guidance?: { id: string; body: string; author: string; date: string; pending: boolean };
};

export type SipomAIResult = {
  suggestions: SipomAISuggestion[];
  sections: string[];
  model?: string;
  cost_usd?: number;
  /** Cidade da ocorrência no catálogo do SIPOM (orientação só da cidade). */
  cidade?: { id: number; nome: string } | null;
};

/** Pendências com que a IA ajuda (as mesmas de internal/sipomai.Codes). */
export const SIPOM_AI_CODES = new Set([
  "natureza",
  "natureza_confirmar",
  "bairro_catalogo",
  "pessoa_sexo",
  "pessoa_sexo_confirmar",
  "arma",
  "droga",
  "veiculo",
  "veiculo_placa",
]);

export function sipomAIHandles(codes: string[]) {
  return codes.some((c) => SIPOM_AI_CODES.has(c));
}

export function suggestSipomAI(occId: string) {
  return api<SipomAIResult>(`/api/ops-occurrences/${encodeURIComponent(occId)}/sipom/ia`, { method: "POST" });
}

export const CONFIANCA_LABEL: Record<SipomAISuggestion["confianca"], string> = {
  alta: "ALTA",
  media: "MÉDIA",
  baixa: "BAIXA",
};

/** Marcada por padrão: aplicável e com confiança alta (ou concordando com a sugestão que já havia). */
export function preselect(s: SipomAISuggestion) {
  return s.applicable && (s.confianca === "alta" || !!s.agrees);
}

type Change = { sipom: SipomOccurrence; officers?: OpsOfficer[] } & SipomChangeExtra;

/** Aplica a sugestão pelo endpoint de correção correspondente. */
export function applySipomAISuggestion(occId: string, s: SipomAISuggestion): Promise<Change> {
  const a = s.apply ?? {};
  const num = (k: string) => Number(a[k]);
  const opt = (k: string) => (a[k] == null ? undefined : Number(a[k]));
  switch (s.kind) {
    case "natureza":
      return setOpsSipomNatureza(occId, num("natureza_id"));
    case "bairro":
      return setOpsSipomField(occId, { field: "bairro", bairro_id: num("bairro_id") });
    case "sexo":
      return setSipomSexo(occId, s.person_id!, num("sexo") as 1 | 2);
    case "arma":
      return setOpsSipomMaterial(occId, s.index!, {
        kind: "armas",
        tipo_id: num("tipo_id"),
        marca_id: opt("marca_id"),
        calibre_id: opt("calibre_id"),
      });
    case "droga":
      return setOpsSipomMaterial(occId, s.index!, { kind: "drogas", droga_id: num("droga_id"), quantidade: num("quantidade") });
    case "veiculo":
      return setOpsSipomMaterial(occId, s.index!, {
        kind: "veiculos",
        tipo_codigo: num("tipo_codigo"),
        cor_codigo: num("cor_codigo"),
        marca_modelo_codigo: num("marca_modelo_codigo"),
        situacao: num("situacao") as 1 | 2,
      });
    case "veiculo_placa":
      return setSipomVehiclePlate(occId, s.index!, String(a.placa));
    case "adicionar": {
      const { material, ...rest } = a;
      return addSipomMaterial(occId, { kind: material, ...rest } as unknown as SipomNewMaterial);
    }
    case "remover":
      return setSipomMaterialRemoved(
        occId,
        String(a.material) as "armas" | "drogas" | "veiculos",
        s.index!,
        true,
        String(a.motivo ?? ""),
      );
    case "veiculo_outros":
      return setSipomVehiclePlate(occId, s.index!, "", true, String(a.descricao ?? ""));
  }
}

/**
 * Aplica as sugestões de uma ocorrência em sequência (cada gravação recalcula
 * a ficha). Devolve o último estado e as que falharam, com o motivo.
 */
export async function applySipomAISuggestions(occId: string, list: SipomAISuggestion[]) {
  let last: Change | null = null;
  const failed: { s: SipomAISuggestion; error: string }[] = [];
  for (const s of list) {
    try {
      last = await applySipomAISuggestion(occId, s);
    } catch (e) {
      failed.push({ s, error: (e as Error).message || "erro ao gravar" });
    }
  }
  return { last, failed };
}
