"use client";

import {
  INCIDENT_MEANS,
  INCIDENT_MEANS_LABEL,
  INCIDENT_TYPES,
  INCIDENT_TYPE_LABEL,
  type IncidentMeans,
  type IncidentType,
  type PlaceFacet,
} from "@/lib/incidents-api";
import { OCCURRENCE_SOURCE_LABEL, type OccurrenceSource } from "@/lib/occurrences-api";
import FiltersModal from "./FiltersModal";
import Select from "./Select";

// Recorte de ocorrências compartilhado pelo mapa do crime e pela listagem —
// as duas telas leem a mesma base e precisam oferecer os mesmos cortes. O
// período fica fora: tem o próprio botão na barra (PeriodButton).
export type IncidentFilters = {
  type: "" | IncidentType;
  means: IncidentMeans;
  city: string;
  neighborhood: string;
  /**
   * Origem — só na listagem de Ocorrências, que reúne o cadastro manual e o
   * relatório operacional. O mapa lê só o cadastro e não usa o campo.
   */
  source?: "" | OccurrenceSource;
};

/** Quantos recortes fogem do padrão da tela (badge do botão FILTROS). */
export function incidentFilterCount(f: IncidentFilters, defaults: IncidentFilters): number {
  let n = 0;
  if (f.type !== defaults.type) n++;
  if (f.means) n++;
  if (f.city) n++;
  if (f.neighborhood) n++;
  if (f.source) n++;
  return n;
}

/**
 * Resumo ao lado dos botões: o recorte segue visível sem abrir o modal.
 * `withType: false` para telas em que o tipo não é um recorte (a aba já o
 * define) — o resumo não anuncia "TODOS OS TIPOS".
 */
export function incidentFilterSummary(f: IncidentFilters, withType = true): string {
  const parts: string[] = [];
  if (f.type) parts.push(INCIDENT_TYPE_LABEL[f.type]);
  else if (withType) parts.push("TODOS OS TIPOS");
  if (f.means) parts.push(INCIDENT_MEANS_LABEL[f.means]);
  if (f.city) parts.push(f.city);
  if (f.neighborhood) parts.push(f.neighborhood);
  if (f.source) parts.push(OCCURRENCE_SOURCE_LABEL[f.source]);
  return parts.join(" · ");
}

type Props = {
  /** Título do modal — identifica a tela que abriu ("DO MAPA", "DAS OCORRÊNCIAS"). */
  title: string;
  value: IncidentFilters;
  defaults: IncidentFilters;
  cities: PlaceFacet[];
  neighborhoodsOf: (city: string) => string[];
  /** Mostra o filtro de origem (cadastro manual × relatório operacional). */
  withSource?: boolean;
  /** Tipos oferecidos; lista vazia esconde o campo. Padrão: todos. */
  types?: IncidentType[];
  /** Mostra o meio utilizado (campo de CVLI). Padrão: sim. */
  withMeans?: boolean;
  onApply: (f: IncidentFilters) => void;
  onClose: () => void;
};

export default function IncidentFiltersModal({
  title,
  value,
  defaults,
  cities,
  neighborhoodsOf,
  withSource = false,
  types = INCIDENT_TYPES,
  withMeans = true,
  onApply,
  onClose,
}: Props) {
  return (
    <FiltersModal title={title} value={value} empty={defaults} onApply={onApply} onClose={onClose} width={560}>
      {(d, set) => (
        <>
          {withSource && (
            <div className="form-field">
              <span>ORIGEM</span>
              <Select
                value={d.source ?? ""}
                onChange={(v) => set({ source: v as "" | OccurrenceSource })}
                options={[
                  { value: "", label: "TODAS" },
                  { value: "manual", label: OCCURRENCE_SOURCE_LABEL.manual },
                  { value: "operacional", label: OCCURRENCE_SOURCE_LABEL.operacional },
                ]}
              />
            </div>
          )}

          {(types.length > 0 || withMeans) && (
            <div className={types.length > 0 && withMeans ? "form-grid-2" : undefined}>
              {types.length > 0 && (
                <div className="form-field">
                  <span>TIPO DE OCORRÊNCIA</span>
                  <Select
                    value={d.type}
                    onChange={(v) => set({ type: v as "" | IncidentType })}
                    options={[
                      { value: "", label: "TODOS" },
                      ...types.map((t) => ({ value: t, label: INCIDENT_TYPE_LABEL[t] })),
                    ]}
                  />
                </div>
              )}
              {withMeans && (
                <div className="form-field">
                  <span>MEIO UTILIZADO</span>
                  <Select
                    value={d.means}
                    onChange={(v) => set({ means: v as IncidentMeans })}
                    options={[
                      { value: "", label: "TODOS" },
                      ...INCIDENT_MEANS.map((m) => ({ value: m, label: INCIDENT_MEANS_LABEL[m] })),
                    ]}
                  />
                </div>
              )}
            </div>
          )}

          <div className="form-grid-2">
            <div className="form-field">
              <span>MUNICÍPIO</span>
              <Select
                value={d.city}
                // Bairro pertence ao município: trocar de cidade zera o bairro.
                onChange={(v) => set({ city: v, neighborhood: "" })}
                options={[
                  { value: "", label: "TODOS" },
                  ...cities.map((c) => ({ value: c.city, label: `${c.city} (${c.count})` })),
                ]}
              />
            </div>
            <div className="form-field">
              <span>BAIRRO</span>
              <Select
                value={d.neighborhood}
                onChange={(v) => set({ neighborhood: v })}
                options={[
                  { value: "", label: "TODOS" },
                  ...neighborhoodsOf(d.city).map((n) => ({ value: n, label: n })),
                ]}
              />
            </div>
          </div>
        </>
      )}
    </FiltersModal>
  );
}
