"use client";

import { useState, type ReactNode } from "react";
import { ChevronLeft, ChevronRight, Pencil } from "lucide-react";
import {
  OPS_ROLE_TONE,
  formatGrams,
  opsTimeRange,
  opsUnitLabel,
  type OpsOccurrence,
  type OpsPerson,
  type OpsPhoto,
  type OpsOfficer,
  type SipomOccurrence,
} from "@/lib/ops-reports-api";
import { formatBR, formatBRDate } from "@/lib/format";
import { IntelSummary } from "./IntelStatus";
import { OpsBlock, OpsField, OpsFields } from "./OpsField";
import OpsGeoSection from "./OpsGeoSection";
import OpsPhotosSection from "./OpsPhotosSection";
import { SipomMateriais, SipomProcedimento } from "./SipomPhase2";
import {
  OfficersTable,
  SipomEfetivo,
  SipomEnvio,
  SipomEnvolvidos,
  SipomFato,
  SipomLocal,
  useSipomEdit,
} from "./SipomSection";

/** Etapas da ficha, na ordem em que o analista confere a ocorrência. */
export type OpsStep =
  | "fato"
  | "local"
  | "efetivo"
  | "pessoas"
  | "procedimento"
  | "apreensoes"
  | "fotos"
  | "envio";
/** Etapas cujo texto do relatório pode ser corrigido (abas do CORRIGIR). */
export type OpsEditTab = Exclude<OpsStep, "pessoas" | "fotos" | "envio">;

// Em que etapa cada pendência do SIPOM se resolve.
const STEP_OF_PENDING: Record<string, OpsStep> = {
  natureza: "fato",
  natureza_confirmar: "fato",
  data_hora: "fato",
  ficha: "fato",
  logradouro: "local",
  cidade: "local",
  bairro: "local",
  area: "local",
  area_ambigua: "local",
  coordenada: "local",
  opm: "efetivo",
  composicao: "efetivo",
  composicao_multi: "efetivo",
  pessoa_sem_dossie: "pessoas",
  procedimento: "procedimento",
  procedimento_tipo: "procedimento",
  procedimento_numero: "procedimento",
  delegacia: "procedimento",
  delegado: "procedimento",
  arma: "apreensoes",
  droga: "apreensoes",
  veiculo: "apreensoes",
};

type Props = {
  occ: OpsOccurrence;
  /**
   * Ficha em etapas (drawer): uma por vez, com navegação. Sem isto, as etapas
   * vêm empilhadas, uma seção por etapa (prévia da importação).
   */
  steps?: boolean;
  /** Ações/estado de vínculo ao lado de cada pessoa (prévia e drawer diferem). */
  renderPerson?: (p: OpsPerson, index: number) => ReactNode;
  /** Substitui o resumo da participação da inteligência (o drawer põe o controle). */
  intelSlot?: ReactNode;
  /** Ficha gravada com permissão de edição: natureza SIPOM escolhível. */
  onSipomChange?: (s: SipomOccurrence, officers?: OpsOfficer[]) => void;
  /** Ficha gravada com permissão de edição: o analista informa a coordenada. */
  onGeoChange?: (occ: OpsOccurrence) => void;
  /** Ficha gravada com permissão de edição: o analista anexa e remove fotos. */
  onPhotosChange?: (photos: OpsPhoto[]) => void;
  /** Abre a correção do texto do relatório, na aba da etapa. */
  onEdit?: (tab: OpsEditTab) => void;
};

/**
 * Corpo da ocorrência do relatório operacional, organizado por tema: em cada
 * etapa, primeiro o que o PDF trouxe (RELATÓRIO) e depois como aquilo vai ao
 * SIPOM, com as correções do analista. Usado no drawer (em etapas) e na
 * prévia da importação (empilhado) — o que muda entre os dois é o vínculo das
 * pessoas, injetado por renderPerson, e o que é editável.
 */
export default function OpsOccurrenceBody({
  occ,
  steps = false,
  renderPerson,
  intelSlot,
  onSipomChange,
  onGeoChange,
  onPhotosChange,
  onEdit,
}: Props) {
  const [step, setStep] = useState<OpsStep>("fato");
  const ed = useSipomEdit(occ, !!onSipomChange && !!occ.id, onSipomChange);
  const s = occ.sipom;
  const seizures = occ.weapons.length + occ.drugs.length + occ.vehicles.length;
  // Na ficha gravada a coordenada aparece sempre (é onde o analista informa
  // o ponto). Na prévia, só quando há o que dizer: o ponto encontrado, ou a
  // pendência de coordenada (geocodificador ligado e endereço não localizado).
  const showGeo = !!occ.id || occ.latitude != null || ed.pending.has("coordenada");

  const blockingBy = new Map<OpsStep, number>();
  for (const p of s?.pendencias ?? []) {
    const k = STEP_OF_PENDING[p.code];
    if (k && p.blocking) blockingBy.set(k, (blockingBy.get(k) ?? 0) + 1);
  }

  const editBtn = (tab: OpsEditTab) =>
    onEdit && (
      <button type="button" className="ops-field-btn" onClick={() => onEdit(tab)} title="Corrigir o que o relatório trouxe">
        <Pencil size={11} /> CORRIGIR
      </button>
    );

  const list: { key: OpsStep; label: string; count?: number; body: ReactNode }[] = [
    {
      key: "fato",
      label: "FATO",
      body: (
        <>
          <OpsBlock title="RELATÓRIO" aside={editBtn("fato")}>
            <OpsFields>
              <OpsField label="DATA DO FATO" value={formatBRDate(occ.occurred_on)} />
              <OpsField label="HORÁRIO" value={opsTimeRange(occ)} />
              <OpsField label="FICHA CIOPS" mono value={occ.ciops_record} />
              <OpsField label="PÁGINA DO PDF" value={occ.page ? String(occ.page) : ""} />
              <OpsField label="NATUREZAS" span={4} value={occ.natures.join(" · ")} />
            </OpsFields>
          </OpsBlock>
          {s && (
            <OpsBlock title="SIPOM">
              <SipomFato occ={occ} ed={ed} />
            </OpsBlock>
          )}
          <OpsBlock title="PARTICIPAÇÃO DA INTELIGÊNCIA">
            {intelSlot ?? (
              <IntelSummary value={occ.intel_participation} mode={occ.intel_mode} matched={occ.intel_matched} />
            )}
          </OpsBlock>
          <OpsBlock title="HISTÓRICO" aside={editBtn("fato")}>
            <p className="ops-text ops-text--narrative">{occ.narrative || "—"}</p>
          </OpsBlock>
        </>
      ),
    },
    {
      key: "local",
      label: "LOCAL",
      body: (
        <>
          <OpsBlock title="RELATÓRIO" aside={editBtn("local")}>
            <OpsFields>
              <OpsField label="LOCAL DA OCORRÊNCIA" span={2} value={occ.place_address} />
              <OpsField label="BAIRRO" value={occ.place_neighborhood} />
              <OpsField label="CIDADE" value={occ.place_city} />
              {(occ.approach_address || occ.approach_neighborhood || occ.approach_city) && (
                <>
                  <OpsField label="LOCAL DA ABORDAGEM" span={2} value={occ.approach_address} />
                  <OpsField label="BAIRRO" value={occ.approach_neighborhood} />
                  <OpsField label="CIDADE" value={occ.approach_city} />
                </>
              )}
            </OpsFields>
          </OpsBlock>
          {s && (
            <OpsBlock title="SIPOM">
              <SipomLocal occ={occ} ed={ed} />
            </OpsBlock>
          )}
          {showGeo && (
            <OpsBlock title="COORDENADA">
              <OpsGeoSection occ={occ} editable={!!onGeoChange && !!occ.id} onChange={onGeoChange} />
            </OpsBlock>
          )}
        </>
      ),
    },
    {
      key: "efetivo",
      label: "EFETIVO",
      body: (
        <>
          <OpsBlock title="RELATÓRIO" aside={editBtn("efetivo")}>
            <OpsFields>
              <OpsField label="UNIDADE" span={2} value={opsUnitLabel(occ)} />
              <OpsField label="EQUIPES" span={2} value={occ.teams} />
            </OpsFields>
          </OpsBlock>
          <OpsBlock title={`${s ? "SIPOM · " : ""}COMPOSIÇÃO (${occ.officers.length})`}>
            {s ? <SipomEfetivo occ={occ} ed={ed} /> : <OfficersTable officers={occ.officers} />}
            {occ.officers.length === 0 && <div className="ops-empty">O RELATÓRIO NÃO TRAZ A COMPOSIÇÃO</div>}
          </OpsBlock>
        </>
      ),
    },
    {
      key: "pessoas",
      label: "PESSOAS",
      count: occ.people.length,
      body: (
        <>
          <OpsBlock title={`RELATÓRIO (${occ.people.length})`}>
            {occ.people.length === 0 && <div className="ops-empty">NENHUMA PESSOA NO RELATÓRIO</div>}
            {occ.people.map((p, i) => (
              <div key={p.id ?? i} className="ops-person">
                <div className="ops-person-info">
                  <div className="ops-person-name">
                    {p.name}
                    {p.note && <span className="pill crit ops-person-note">{p.note}</span>}
                  </div>
                  <div className="ops-person-meta">
                    <span className={OPS_ROLE_TONE[p.role]}>{p.role}</span>
                    {p.age != null && <span>{p.age} ANOS</span>}
                    {p.mother_name && <span>MÃE: {p.mother_name}</span>}
                    {p.address && <span>{p.address}</span>}
                  </div>
                </div>
                {renderPerson?.(p, i)}
              </div>
            ))}
          </OpsBlock>
          {s && occ.people.length > 0 && (
            <OpsBlock title={`SIPOM · ENVOLVIDOS (${s.pessoas.length})`}>
              <SipomEnvolvidos occ={occ} />
            </OpsBlock>
          )}
        </>
      ),
    },
    {
      key: "procedimento",
      label: "PROCEDIMENTO",
      body: (
        <>
          <OpsBlock title="RELATÓRIO" aside={editBtn("procedimento")}>
            <OpsFields>
              <OpsField label="PROCEDIMENTO" span={2} value={occ.procedure_type} />
              <OpsField label="Nº DO PROCEDIMENTO" span={2} mono value={occ.procedure_number} />
              <OpsField label="DELEGACIA" span={2} value={occ.police_station} />
              <OpsField label="DELEGADO(A)" span={2} value={occ.delegate} />
            </OpsFields>
          </OpsBlock>
          {s && (
            <OpsBlock title="SIPOM">
              <SipomProcedimento occ={occ} ed={ed} />
            </OpsBlock>
          )}
        </>
      ),
    },
    {
      key: "apreensoes",
      label: "APREENSÕES",
      count: seizures,
      body: (
        <>
          {seizures === 0 && !occ.seized_objects && <div className="ops-empty">NENHUMA APREENSÃO NO RELATÓRIO</div>}
          {seizures > 0 &&
            (s ? (
              <OpsBlock title="ARMAS, DROGAS E VEÍCULOS · RELATÓRIO E SIPOM">
                <SipomMateriais occ={occ} ed={ed} />
              </OpsBlock>
            ) : (
              <OpsBlock title="ARMAS, DROGAS E VEÍCULOS">
                <SeizureTables occ={occ} />
              </OpsBlock>
            ))}
          {(occ.seized_objects || onEdit) && (
            <OpsBlock title="OBJETOS APREENDIDOS" aside={editBtn("apreensoes")}>
              <p className="ops-text">{occ.seized_objects || "—"}</p>
            </OpsBlock>
          )}
        </>
      ),
    },
    {
      key: "fotos",
      label: "FOTOS",
      count: occ.photos?.length ?? 0,
      body: (
        <OpsBlock title="FOTOS DA OCORRÊNCIA · APREENSÃO, PRISÃO, LOCAL">
          <OpsPhotosSection occ={occ} editable={!!onPhotosChange && !!occ.id} onChange={onPhotosChange} />
        </OpsBlock>
      ),
    },
    {
      key: "envio",
      label: "ENVIO",
      body: (
        <>
          {s && (
            <OpsBlock title="ENVIO AO SIPOM">
              <SipomEnvio
                occ={occ}
                stepOf={(code) => {
                  const k = STEP_OF_PENDING[code];
                  return k ? { key: k, label: list.find((x) => x.key === k)!.label } : null;
                }}
                onGoTo={steps ? (k) => setStep(k as OpsStep) : undefined}
              />
            </OpsBlock>
          )}
          {occ.warnings.length > 0 && (
            <OpsBlock title="AVISOS DA LEITURA DO PDF">
              <div className="banner banner-warn">
                {occ.warnings.map((w, i) => (
                  <div key={i}>⚠ {w}</div>
                ))}
              </div>
            </OpsBlock>
          )}
          {occ.created_at && (
            <OpsBlock title="REGISTRO">
              <OpsFields>
                <OpsField label="IMPORTADO EM" span={2} value={formatBR(occ.created_at)} />
                <OpsField
                  label="CORRIGIDO EM"
                  span={2}
                  value={
                    occ.updated_at
                      ? formatBR(occ.updated_at) + (occ.updated_by_name ? ` · ${occ.updated_by_name}` : "")
                      : "SEM CORREÇÃO — CÓPIA DO PDF"
                  }
                />
              </OpsFields>
            </OpsBlock>
          )}
        </>
      ),
    },
  ];

  const error = ed.error && <div className="banner banner-error">⚠ {ed.error}</div>;

  if (!steps) {
    // Na prévia a etapa ENVIO só existe se houver o que conferir.
    // Fotos só existem na ficha gravada.
    const shown = list.filter(
      (x) => (x.key !== "envio" || s || occ.warnings.length > 0) && (x.key !== "fotos" || !!occ.id),
    );
    return (
      <div className="ops-body">
        {error}
        {shown.map((x) => (
          <section key={x.key} className="ops-section">
            <div className="ops-section-title">
              {x.label}
              {x.count != null && ` (${x.count})`}
            </div>
            {x.body}
          </section>
        ))}
      </div>
    );
  }

  const index = list.findIndex((x) => x.key === step);
  return (
    <div className="ops-body ops-body--steps">
      <nav className="ops-steps" aria-label="Etapas da ficha">
        {list.map((x, i) => {
          const blocking = x.key === "envio" ? (s && !s.ready ? 1 : 0) : (blockingBy.get(x.key) ?? 0);
          return (
            <button
              key={x.key}
              type="button"
              className={"ops-step" + (x.key === step ? " ops-step--on" : "") + (blocking ? " ops-step--pending" : "")}
              aria-current={x.key === step ? "step" : undefined}
              onClick={() => setStep(x.key)}
              title={blocking ? "Há pendência para o SIPOM nesta etapa" : undefined}
            >
              <span className="ops-step-num">{i + 1}</span>
              <span className="ops-step-lbl">
                {x.label}
                {x.count != null && ` (${x.count})`}
              </span>
            </button>
          );
        })}
      </nav>

      {error}
      {/* Todas as etapas ficam montadas (só a ativa aparece): o que o analista
          escolheu e ainda não gravou numa etapa não se perde ao passar por outra. */}
      {list.map((x) => (
        <div key={x.key} className="ops-step-body" hidden={x.key !== step}>
          {x.body}
        </div>
      ))}

      <div className="ops-step-ft">
        <button type="button" className="btn btn-ghost" disabled={index === 0} onClick={() => setStep(list[index - 1].key)}>
          <ChevronLeft size={14} /> ANTERIOR
        </button>
        <span className="muted">
          ETAPA {index + 1} DE {list.length}
        </span>
        <button
          type="button"
          className="btn"
          disabled={index === list.length - 1}
          onClick={() => setStep(list[index + 1].key)}
        >
          PRÓXIMA <ChevronRight size={14} />
        </button>
      </div>
    </div>
  );
}

// Apreensões como o relatório as trouxe — usadas quando o catálogo do SIPOM
// não está carregado e não há tradução para mostrar ao lado.
function SeizureTables({ occ }: { occ: OpsOccurrence }) {
  return (
    <>
      {occ.weapons.length > 0 && (
        <table className="tbl ops-subtbl">
          <thead>
            <tr>
              <th>ARMA</th>
              <th>MARCA</th>
              <th>MODELO</th>
              <th>CALIBRE</th>
              <th>Nº SÉRIE</th>
            </tr>
          </thead>
          <tbody>
            {occ.weapons.map((w, i) => (
              <tr key={i}>
                <td>{w.kind || "—"}</td>
                <td>{w.brand || "—"}</td>
                <td>{w.model || "—"}</td>
                <td>{w.caliber || "—"}</td>
                <td className="mono">{w.serial || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {occ.drugs.length > 0 && (
        <table className="tbl ops-subtbl">
          <thead>
            <tr>
              <th>DROGA</th>
              <th style={{ textAlign: "right" }}>QUANTIDADE</th>
              <th style={{ textAlign: "right" }}>EMBALAGENS</th>
            </tr>
          </thead>
          <tbody>
            {occ.drugs.map((d, i) => (
              <tr key={i}>
                <td>{d.description || "—"}</td>
                <td style={{ textAlign: "right" }} className="mono">
                  {formatGrams(d.grams)}
                </td>
                <td style={{ textAlign: "right" }} className="mono">
                  {d.packages ? d.packages : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {occ.vehicles.length > 0 && (
        <table className="tbl ops-subtbl">
          <thead>
            <tr>
              <th>VEÍCULO</th>
              <th>MARCA / MODELO</th>
              <th>PLACA / CHASSI</th>
              <th>COR</th>
            </tr>
          </thead>
          <tbody>
            {occ.vehicles.map((v, i) => (
              <tr key={i}>
                <td>{v.kind || "—"}</td>
                <td>{[v.brand, v.model].filter(Boolean).join(" ") || "—"}</td>
                <td className="mono">{v.plate || "—"}</td>
                <td>{v.color || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}
