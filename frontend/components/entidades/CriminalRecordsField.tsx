"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, Plus, Trash2 } from "lucide-react";
import Select from "../shared/Select";
import DateInput from "../shared/DateInput";
import {
  CRIMINAL_CATEGORY_LABEL,
  emptyCriminalRecord,
  type CriminalRecord,
} from "@/lib/entities-types";
import { formatBRDate } from "@/lib/format";

const ROLE_SUGGESTIONS = ["INFRATOR", "TESTEMUNHA", "VÍTIMA", "AUTOR", "ENVOLVIDO"];

/** Resumo de uma linha do registro (cabeçalho do item e listagem). */
export function criminalRecordSummary(r: CriminalRecord): string {
  return [
    r.title || "(SEM NATUREZA/PEÇA)",
    r.detail,
    r.number ? `Nº ${r.number}` : "",
    r.agency,
  ]
    .filter(Boolean)
    .join(" · ");
}

/**
 * Repeater do histórico criminal da pessoa. Cada item fica recolhido numa
 * linha-resumo; clicar abre os campos. Itens vindos do PDF chegam recolhidos,
 * o item criado à mão chega aberto.
 */
export default function CriminalRecordsField({
  value,
  onChange,
  disabled,
}: {
  value: CriminalRecord[];
  onChange: (v: CriminalRecord[]) => void;
  disabled?: boolean;
}) {
  // Índices abertos. Guardado por posição: remover um item fecha os demais
  // para não deslocar o estado para o vizinho.
  const [open, setOpen] = useState<Set<number>>(new Set());

  const set = (i: number, patch: Partial<CriminalRecord>) =>
    onChange(value.map((r, k) => (k === i ? { ...r, ...patch } : r)));

  const toggle = (i: number) => {
    const n = new Set(open);
    if (n.has(i)) n.delete(i);
    else n.add(i);
    setOpen(n);
  };

  function add() {
    onChange([...value, emptyCriminalRecord()]);
    setOpen(new Set([...open, value.length]));
  }

  function remove(i: number) {
    onChange(value.filter((_, k) => k !== i));
    setOpen(new Set());
  }

  return (
    <fieldset className="form-fieldset">
      <legend>HISTÓRICO CRIMINAL ({value.length})</legend>
      {value.length === 0 && (
        <div className="muted" style={{ fontSize: 12 }}>
          // NENHUM REGISTRO — ANEXE UM PDF DE CONSULTA OU ADICIONE À MÃO
        </div>
      )}
      <div className="crim-list">
        {value.map((r, i) => {
          const isOpen = open.has(i);
          return (
            <div key={i} className={"crim-item" + (isOpen ? " crim-item--open" : "")}>
              <div
                className="crim-item-hd"
                role="button"
                tabIndex={0}
                onClick={() => toggle(i)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    toggle(i);
                  }
                }}
              >
                {isOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
                <span className="crim-date">{r.date ? formatBRDate(r.date) : "—"}</span>
                {r.category && (
                  <span className={"crim-tag crim-tag--" + r.category}>
                    {CRIMINAL_CATEGORY_LABEL[r.category]}
                  </span>
                )}
                {r.role && <span className="crim-tag">{r.role}</span>}
                <span className="crim-summary">{criminalRecordSummary(r)}</span>
                {!disabled && (
                  <button
                    type="button"
                    className="action-btn"
                    onClick={(e) => {
                      e.stopPropagation();
                      remove(i);
                    }}
                    aria-label="Remover registro"
                    title="Remover registro"
                  >
                    <Trash2 size={13} />
                  </button>
                )}
              </div>
              {isOpen && (
                <div className="crim-item-bd">
                  <div className="form-grid-3">
                    <div className="form-field">
                      <span>CATEGORIA</span>
                      <Select
                        value={r.category}
                        onChange={(v) => set(i, { category: v as CriminalRecord["category"] })}
                        disabled={disabled}
                        options={[
                          { value: "", label: "—" },
                          { value: "policial", label: "POLICIAL (OCORRÊNCIA)" },
                          { value: "judicial", label: "JUDICIAL (PEÇA)" },
                        ]}
                      />
                    </div>
                    <label className="form-field">
                      <span>PAPEL</span>
                      <input
                        type="text"
                        list="crim-roles"
                        value={r.role}
                        onChange={(e) => set(i, { role: e.target.value })}
                        disabled={disabled}
                      />
                    </label>
                    <div className="form-field">
                      <span>{r.category === "judicial" ? "EXPEDIÇÃO" : "DATA DO FATO"}</span>
                      <DateInput value={r.date} onChange={(v) => set(i, { date: v })} disabled={disabled} />
                    </div>
                  </div>
                  <label className="form-field">
                    <span>{r.category === "judicial" ? "TIPO DA PEÇA" : "NATUREZA DO FATO"}</span>
                    <input
                      type="text"
                      value={r.title}
                      onChange={(e) => set(i, { title: e.target.value })}
                      disabled={disabled}
                    />
                  </label>
                  <div className="form-grid-3">
                    <label className="form-field">
                      <span>{r.category === "judicial" ? "Nº DO PROCESSO" : "Nº DO PROCEDIMENTO"}</span>
                      <input
                        type="text"
                        value={r.number}
                        onChange={(e) => set(i, { number: e.target.value })}
                        disabled={disabled}
                      />
                    </label>
                    <label className="form-field">
                      <span>TIPO DE PROCEDIMENTO</span>
                      <input
                        type="text"
                        value={r.procedure_type}
                        onChange={(e) => set(i, { procedure_type: e.target.value })}
                        placeholder="INQUÉRITO, TCO, BO…"
                        disabled={disabled}
                      />
                    </label>
                    <div className="form-field">
                      <span>INSTAURAÇÃO</span>
                      <DateInput value={r.filed_on} onChange={(v) => set(i, { filed_on: v })} disabled={disabled} />
                    </div>
                  </div>
                  <label className="form-field">
                    <span>{r.category === "judicial" ? "ÓRGÃO EXPEDIDOR" : "DELEGACIA / ÓRGÃO"}</span>
                    <input
                      type="text"
                      value={r.agency}
                      onChange={(e) => set(i, { agency: e.target.value })}
                      disabled={disabled}
                    />
                  </label>
                  <div className="form-grid-3">
                    <label className="form-field">
                      <span>TRIBUNAL</span>
                      <input
                        type="text"
                        value={r.court}
                        onChange={(e) => set(i, { court: e.target.value })}
                        disabled={disabled}
                      />
                    </label>
                    <label className="form-field">
                      <span>COMARCA</span>
                      <input
                        type="text"
                        value={r.district}
                        onChange={(e) => set(i, { district: e.target.value })}
                        disabled={disabled}
                      />
                    </label>
                    <label className="form-field">
                      <span>Nº DA PEÇA</span>
                      <input
                        type="text"
                        value={r.piece}
                        onChange={(e) => set(i, { piece: e.target.value })}
                        disabled={disabled}
                      />
                    </label>
                  </div>
                  <div className="form-grid-3">
                    <label className="form-field">
                      <span>ESPÉCIE / DETALHE</span>
                      <input
                        type="text"
                        value={r.detail}
                        onChange={(e) => set(i, { detail: e.target.value })}
                        disabled={disabled}
                      />
                    </label>
                    <label className="form-field">
                      <span>MOTIVO</span>
                      <input
                        type="text"
                        value={r.reason}
                        onChange={(e) => set(i, { reason: e.target.value })}
                        disabled={disabled}
                      />
                    </label>
                    <label className="form-field">
                      <span>FONTE</span>
                      <input
                        type="text"
                        value={r.source}
                        onChange={(e) => set(i, { source: e.target.value })}
                        disabled={disabled}
                      />
                    </label>
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
      <datalist id="crim-roles">
        {ROLE_SUGGESTIONS.map((r) => (
          <option key={r} value={r} />
        ))}
      </datalist>
      {!disabled && (
        <button type="button" className="btn btn-ghost" onClick={add} style={{ marginTop: 6 }}>
          <Plus size={13} /> REGISTRO
        </button>
      )}
    </fieldset>
  );
}

/** Listagem só-leitura do histórico (dossiê da pessoa). */
export function CriminalRecordsList({ records }: { records?: CriminalRecord[] }) {
  if (!records || records.length === 0) {
    return <div className="muted" style={{ fontSize: 12 }}>// SEM REGISTROS</div>;
  }
  return (
    <div className="crim-list">
      {records.map((r, i) => (
        <div key={i} className="crim-item">
          <div className="crim-item-hd crim-item-hd--static">
            <span className="crim-date">{r.date ? formatBRDate(r.date) : "—"}</span>
            {r.category && (
              <span className={"crim-tag crim-tag--" + r.category}>
                {CRIMINAL_CATEGORY_LABEL[r.category]}
              </span>
            )}
            {r.role && <span className="crim-tag">{r.role}</span>}
            <span className="crim-summary">
              {criminalRecordSummary(r)}
              {[r.procedure_type, [r.court, r.district].filter(Boolean).join("/"), r.reason]
                .filter(Boolean)
                .map((x) => ` · ${x}`)
                .join("")}
              {r.source && <span className="muted"> · {r.source}</span>}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}
