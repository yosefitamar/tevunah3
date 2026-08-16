"use client";

import {
  INCIDENT_MEANS,
  INCIDENT_MEANS_LABEL,
  type IncidentMeans,
} from "@/lib/incidents-api";
import Select from "../shared/Select";

type Props = {
  means: IncidentMeans;
  detail: string;
  onChangeMeans: (v: IncidentMeans) => void;
  onChangeDetail: (v: string) => void;
  /** Chamado ao sair do campo de detalhe — usado pelo drawer, que persiste no blur. */
  onDetailBlur?: (v: string) => void;
  disabled?: boolean;
};

/**
 * Meio utilizado (CVLI) + detalhamento livre. O detalhe só aparece em
 * "OUTROS" — nos demais o rótulo já basta e um campo aberto só convidaria
 * a duplicar a informação.
 */
export default function MeansField({
  means,
  detail,
  onChangeMeans,
  onChangeDetail,
  onDetailBlur,
  disabled,
}: Props) {
  return (
    <>
      <div className="form-field">
        <span>MEIO UTILIZADO</span>
        <Select
          value={means}
          disabled={disabled}
          onChange={(v) => onChangeMeans(v as IncidentMeans)}
          options={[
            { value: "", label: INCIDENT_MEANS_LABEL[""] },
            ...INCIDENT_MEANS.map((m) => ({ value: m, label: INCIDENT_MEANS_LABEL[m] })),
          ]}
        />
      </div>
      {means === "outros" && (
        <label className="form-field">
          <span>DETALHAR O MEIO</span>
          <input
            type="text"
            value={detail}
            disabled={disabled}
            onChange={(e) => onChangeDetail(e.target.value)}
            onBlur={(e) => onDetailBlur?.(e.target.value)}
            maxLength={120}
            placeholder="ex.: atropelamento, queda provocada…"
          />
        </label>
      )}
    </>
  );
}
