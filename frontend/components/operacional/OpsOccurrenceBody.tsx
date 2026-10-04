"use client";

import type { ReactNode } from "react";
import {
  OPS_ROLE_TONE,
  formatGrams,
  opsTimeRange,
  opsUnitLabel,
  type OpsOccurrence,
  type OpsPerson,
  type OpsOfficer,
  type SipomOccurrence,
} from "@/lib/ops-reports-api";
import { formatBRDate } from "@/lib/format";
import { IntelSummary } from "./IntelStatus";
import OpsGeoSection from "./OpsGeoSection";
import SipomSection from "./SipomSection";

type Props = {
  occ: OpsOccurrence;
  /** Ações/estado de vínculo ao lado de cada pessoa (prévia e drawer diferem). */
  renderPerson?: (p: OpsPerson, index: number) => ReactNode;
  /** Substitui o resumo da participação da inteligência (o drawer põe o controle). */
  intelSlot?: ReactNode;
  /** Ficha gravada com permissão de edição: natureza SIPOM escolhível. */
  onSipomChange?: (s: SipomOccurrence, officers?: OpsOfficer[]) => void;
  /** Ficha gravada com permissão de edição: o analista informa a coordenada. */
  onGeoChange?: (occ: OpsOccurrence) => void;
};

/**
 * Corpo da ocorrência do relatório operacional: tudo o que o PDF traz, na
 * ordem do documento. Usado na prévia da importação e no drawer — o que
 * muda entre os dois é só o vínculo das pessoas, injetado por renderPerson.
 */
export default function OpsOccurrenceBody({ occ, renderPerson, intelSlot, onSipomChange, onGeoChange }: Props) {
  const place = [occ.place_address, occ.place_neighborhood, occ.place_city].filter(Boolean).join(" · ");
  const approach = [occ.approach_address, occ.approach_neighborhood, occ.approach_city]
    .filter(Boolean)
    .join(" · ");
  const hasSeizures = occ.weapons.length + occ.drugs.length + occ.vehicles.length > 0;
  // Na ficha gravada a localização aparece sempre (é onde o analista informa
  // o ponto). Na prévia, só quando há o que dizer: o ponto encontrado, ou a
  // pendência de coordenada (geocodificador ligado e endereço não localizado).
  const showGeo =
    !!occ.id || occ.latitude != null || !!occ.sipom?.pendencias.some((p) => p.code === "coordenada");

  return (
    <div className="ops-body">
      <dl className="ops-kv">
        <div>
          <dt>DATA</dt>
          <dd>
            {formatBRDate(occ.occurred_on)}
            {opsTimeRange(occ) && <span className="muted"> · {opsTimeRange(occ)}</span>}
          </dd>
        </div>
        <div>
          <dt>UNIDADE</dt>
          <dd>{opsUnitLabel(occ) || "—"}</dd>
        </div>
        <div>
          <dt>EQUIPE</dt>
          <dd>{occ.teams || "—"}</dd>
        </div>
        <div>
          <dt>FICHA CIOPS</dt>
          <dd className="mono">{occ.ciops_record || "—"}</dd>
        </div>
        <div className="ops-kv-wide">
          <dt>LOCAL</dt>
          <dd>{place || "—"}</dd>
        </div>
        {approach && (
          <div className="ops-kv-wide">
            <dt>ABORDAGEM</dt>
            <dd>{approach}</dd>
          </div>
        )}
        <div>
          <dt>DELEGACIA</dt>
          <dd>{occ.police_station || "—"}</dd>
        </div>
        <div>
          <dt>DELEGADO(A)</dt>
          <dd>{occ.delegate || "—"}</dd>
        </div>
        <div>
          <dt>PROCEDIMENTO</dt>
          <dd>
            {[occ.procedure_type, occ.procedure_number].filter(Boolean).join(" · ") || "—"}
          </dd>
        </div>
        <div>
          <dt>PÁGINA DO PDF</dt>
          <dd>{occ.page || "—"}</dd>
        </div>
        <div className="ops-kv-wide">
          <dt>PARTICIPAÇÃO DA INTELIGÊNCIA</dt>
          <dd>
            {intelSlot ?? (
              <IntelSummary
                value={occ.intel_participation}
                mode={occ.intel_mode}
                matched={occ.intel_matched}
              />
            )}
          </dd>
        </div>
      </dl>

      <SipomSection occ={occ} editable={!!onSipomChange && !!occ.id} onChange={onSipomChange} />

      {showGeo && <OpsGeoSection occ={occ} editable={!!onGeoChange && !!occ.id} onChange={onGeoChange} />}

      {occ.people.length > 0 && (
        <section className="ops-section">
          <div className="ops-section-title">PESSOAS ({occ.people.length})</div>
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
        </section>
      )}

      {hasSeizures && (
        <section className="ops-section">
          <div className="ops-section-title">APREENSÕES</div>
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
        </section>
      )}

      {occ.seized_objects && (
        <section className="ops-section">
          <div className="ops-section-title">OBJETOS APREENDIDOS</div>
          <p className="ops-text">{occ.seized_objects}</p>
        </section>
      )}

      <section className="ops-section">
        <div className="ops-section-title">HISTÓRICO</div>
        <p className="ops-text ops-text--narrative">{occ.narrative || "—"}</p>
      </section>

      {occ.officers.length > 0 && (
        <section className="ops-section">
          <div className="ops-section-title">
            COMPOSIÇÃO ({occ.officers.length})
            {occ.officers[0]?.sipom_policiamento && (
              <span className="muted"> · {occ.officers[0].sipom_policiamento.toUpperCase()}</span>
            )}
          </div>
          <div className="ops-officers">
            {occ.officers.map((f, i) => (
              <span key={i} className="ops-officer" title={f.registration ? `Matrícula ${f.registration}` : undefined}>
                <span className="muted">{f.rank}</span> {f.war_name}
                {f.number && <span className="muted"> · {f.number}</span>}
                {f.sipom_funcao && <span className="ops-officer-fn">{f.sipom_funcao.toUpperCase()}</span>}
              </span>
            ))}
          </div>
        </section>
      )}

      {occ.warnings.length > 0 && (
        <div className="banner banner-warn" style={{ marginTop: 10 }}>
          {occ.warnings.map((w, i) => (
            <div key={i}>⚠ {w}</div>
          ))}
        </div>
      )}
    </div>
  );
}
