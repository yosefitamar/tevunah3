"use client";

type Props = {
  value: boolean;
  onChange: (v: boolean) => void;
  /** Termos de inteligência encontrados no relatório importado (sugestão). */
  matched?: string[];
  disabled?: boolean;
};

/**
 * Participação da inteligência (SAI) na ocorrência. No cadastro vindo do
 * relatório do grupo, os termos configurados pelo administrador já pré-marcam
 * o campo e aparecem como justificativa — o analista confirma ou corrige.
 */
export default function IntelField({ value, onChange, matched, disabled }: Props) {
  return (
    <div className="form-field">
      <span>PARTICIPAÇÃO DA INTELIGÊNCIA</span>
      <div className="intel-control">
        <div className="seg-row intel-seg" role="radiogroup" aria-label="Participação da inteligência">
          {[true, false].map((v) => (
            <button
              key={String(v)}
              type="button"
              role="radio"
              aria-checked={value === v}
              className={"seg-btn" + (value === v ? " seg-btn--on" : "")}
              disabled={disabled}
              onClick={() => value !== v && onChange(v)}
            >
              {v ? "SIM" : "NÃO"}
            </button>
          ))}
        </div>
        {matched && matched.length > 0 && (
          <span className="intel-summary">
            <span className="muted">MARCADO PELO RELATÓRIO:</span>
            <span className="intel-terms" style={{ marginLeft: 0 }}>
              {matched.map((t) => (
                <span key={t} className="intel-term">
                  {t}
                </span>
              ))}
            </span>
          </span>
        )}
      </div>
    </div>
  );
}
