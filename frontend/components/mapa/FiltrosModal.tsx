"use client";

import { useState, type FormEvent } from "react";
import { X } from "lucide-react";
import {
  INCIDENT_MEANS,
  INCIDENT_MEANS_LABEL,
  INCIDENT_TYPES,
  INCIDENT_TYPE_LABEL,
  type IncidentMeans,
  type IncidentType,
  type PlaceFacet,
} from "@/lib/incidents-api";
import { RANGE_IDS, RANGE_LABEL, type RangeId } from "@/lib/date-ranges";
import DateInput from "../shared/DateInput";
import Select from "../shared/Select";

export type MapFilters = {
  range: RangeId;
  from: string; // usados só quando range === "custom"
  to: string;
  type: "" | IncidentType;
  means: IncidentMeans;
  city: string;
  neighborhood: string;
};

type Props = {
  value: MapFilters;
  defaults: MapFilters;
  cities: PlaceFacet[];
  neighborhoodsOf: (city: string) => string[];
  onApply: (f: MapFilters) => void;
  onClose: () => void;
};

/**
 * Modal com todos os recortes do mapa. O estado é rascunho: só vai para a
 * tela em APLICAR, então mexer nos campos não dispara uma consulta a cada
 * clique — o recorte inteiro viaja de uma vez.
 */
export default function FiltrosModal({
  value,
  defaults,
  cities,
  neighborhoodsOf,
  onApply,
  onClose,
}: Props) {
  const [draft, setDraft] = useState<MapFilters>(value);

  function submit(e: FormEvent) {
    e.preventDefault();
    onApply(draft);
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 560 }}>
        <div className="modal-hd">
          <span>FILTROS DO MAPA</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <form onSubmit={submit}>
          {/* --no-clip: são poucos campos e o popover do select precisa poder
              transbordar o corpo do modal (município e bairro ficam colados
              no rodapé). */}
          <div className="modal-bd modal-bd--no-clip">
            <div className="form-field">
              <span>PERÍODO</span>
              <Select
                value={draft.range}
                onChange={(v) => setDraft({ ...draft, range: v as RangeId })}
                options={RANGE_IDS.map((r) => ({ value: r, label: RANGE_LABEL[r] }))}
              />
            </div>

            {draft.range === "custom" && (
              <div className="form-grid-2">
                <div className="form-field">
                  <span>DE</span>
                  <DateInput
                    value={draft.from}
                    onChange={(v) => setDraft({ ...draft, from: v })}
                  />
                </div>
                <div className="form-field">
                  <span>ATÉ</span>
                  <DateInput value={draft.to} onChange={(v) => setDraft({ ...draft, to: v })} />
                </div>
              </div>
            )}

            <div className="form-grid-2">
              <div className="form-field">
                <span>TIPO DE OCORRÊNCIA</span>
                <Select
                  value={draft.type}
                  onChange={(v) => setDraft({ ...draft, type: v as "" | IncidentType })}
                  options={[
                    { value: "", label: "TODOS" },
                    ...INCIDENT_TYPES.map((t) => ({
                      value: t,
                      label: INCIDENT_TYPE_LABEL[t],
                    })),
                  ]}
                />
              </div>
              <div className="form-field">
                <span>MEIO UTILIZADO</span>
                <Select
                  value={draft.means}
                  onChange={(v) => setDraft({ ...draft, means: v as IncidentMeans })}
                  options={[
                    { value: "", label: "TODOS" },
                    ...INCIDENT_MEANS.map((m) => ({ value: m, label: INCIDENT_MEANS_LABEL[m] })),
                  ]}
                />
              </div>
            </div>

            <div className="form-grid-2">
              <div className="form-field">
                <span>MUNICÍPIO</span>
                <Select
                  value={draft.city}
                  onChange={(v) =>
                    // Bairro pertence ao município: trocar de cidade zera o
                    // recorte de bairro.
                    setDraft({ ...draft, city: v, neighborhood: "" })
                  }
                  options={[
                    { value: "", label: "TODOS" },
                    ...cities.map((c) => ({
                      value: c.city,
                      label: `${c.city} (${c.count})`,
                    })),
                  ]}
                />
              </div>
              <div className="form-field">
                <span>BAIRRO</span>
                <Select
                  value={draft.neighborhood}
                  onChange={(v) => setDraft({ ...draft, neighborhood: v })}
                  options={[
                    { value: "", label: "TODOS" },
                    ...neighborhoodsOf(draft.city).map((n) => ({ value: n, label: n })),
                  ]}
                />
              </div>
            </div>
          </div>

          <div className="modal-ft">
            <button
              type="button"
              className="btn btn-ghost"
              onClick={() => setDraft(defaults)}
              style={{ marginRight: "auto" }}
            >
              LIMPAR
            </button>
            <button type="button" className="btn btn-ghost" onClick={onClose}>
              CANCELAR
            </button>
            <button type="submit" className="btn btn-primary">
              APLICAR
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
