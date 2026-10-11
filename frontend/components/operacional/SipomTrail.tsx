"use client";

import { Box, FileText, MapPin, Send, Shield, Tag, Users, type LucideIcon } from "lucide-react";
import { formatBRDate } from "@/lib/format";
import type { SipomPending } from "@/lib/ops-reports-api";
import { STEP_OF_PENDING, type OpsStep } from "./OpsOccurrenceBody";

// Trilha de etapas: um ícone por etapa da ficha da ocorrência, sempre na
// mesma posição, para a coluna poder ser lida de cima a baixo. É como as
// listagens mostram a situação da ocorrência do relatório operacional.
export const TRAIL: [OpsStep, string, LucideIcon][] = [
  ["fato", "FATO", Tag],
  ["local", "LOCAL", MapPin],
  ["efetivo", "EFETIVO", Shield],
  ["pessoas", "PESSOAS", Users],
  ["procedimento", "PROCEDIMENTO", FileText],
  ["apreensoes", "APREENSÕES", Box],
];

/** Etapas em que uma pendência pode impedir o envio (PESSOAS só tem aviso). */
export const BLOCKING_STEPS = TRAIL.filter(([k]) => k !== "pessoas");

// Rótulo curto de cada pendência (o longo vem do servidor).
export const PENDING_SHORT: Record<string, string> = {
  natureza: "SEM NATUREZA",
  natureza_confirmar: "CONFIRMAR NATUREZA",
  data_hora: "SEM HORA",
  ficha: "SEM FICHA",
  logradouro: "SEM LOGRADOURO",
  cidade: "CIDADE FORA DO CATÁLOGO",
  bairro: "SEM BAIRRO",
  area: "SEM ÁREA",
  area_ambigua: "ÁREA AMBÍGUA",
  opm: "SEM OPM",
  composicao: "COMPOSIÇÃO",
  composicao_multi: "DUAS EQUIPES",
  pessoa_sem_dossie: "PESSOA SEM DOSSIÊ",
  coordenada: "SEM COORDENADA",
  procedimento: "SEM PROCEDIMENTO",
  procedimento_tipo: "TIPO DO PROCED.",
  procedimento_numero: "Nº DO PROCED.",
  delegacia: "SEM DELEGACIA",
  delegado: "SEM DELEGADO",
  arma: "ARMA",
  droga: "DROGA",
  veiculo: "VEÍCULO",
};

/**
 * A trilha de uma ocorrência: a etapa que impede o envio acende em âmbar, a
 * que só tem aviso fica tracejada, as resolvidas ficam apagadas; sem
 * pendência nenhuma a trilha inteira fica na cor de destaque.
 */
export function SipomTrail({ pendencias, sentAt }: { pendencias: SipomPending[]; sentAt?: string | null }) {
  // Enviada: a trilha dá lugar ao sinal de envio — não há mais o que resolver.
  if (sentAt) {
    return (
      <span className="sipom-sent" title={`Enviada ao SIPOM em ${formatBRDate(sentAt.slice(0, 10))}`}>
        <Send size={13} strokeWidth={1.9} /> ENVIADA
      </span>
    );
  }
  return (
    <div className={"sipom-trail" + (pendencias.length === 0 ? " sipom-trail--ok" : "")}>
      {TRAIL.map(([k, label, Icon]) => {
        const mine = pendencias.filter((p) => STEP_OF_PENDING[p.code] === k);
        const cls = mine.some((p) => p.blocking) ? " blk" : mine.length > 0 ? " note" : "";
        const tip = mine.length > 0 ? `${label}:\n${mine.map((p) => "• " + p.label).join("\n")}` : `${label}: SEM PENDÊNCIA`;
        return (
          <span key={k} className={"sipom-trail-step" + cls} title={tip}>
            <Icon size={14} strokeWidth={1.9} />
          </span>
        );
      })}
    </div>
  );
}

/** Legenda da trilha, para ficar acima da tabela. */
export function SipomTrailLegend({ sent = false, children }: { sent?: boolean; children?: React.ReactNode }) {
  return (
    <div className="sipom-legend">
      {TRAIL.map(([k, label, Icon]) => (
        <span key={k}>
          <Icon size={13} strokeWidth={1.9} /> {label}
        </span>
      ))}
      <span className="sipom-legend-sep" />
      <span>
        <i className="sipom-trail-step blk" /> IMPEDE O ENVIO AO SIPOM
      </span>
      <span>
        <i className="sipom-trail-step note" /> AVISO
      </span>
      {sent && (
        <span className="sipom-sent">
          <Send size={12} strokeWidth={1.9} /> JÁ ENVIADA AO SIPOM
        </span>
      )}
      {children}
    </div>
  );
}
