"use client";

import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent, type ReactNode } from "react";
import { SlidersHorizontal, X } from "lucide-react";

/** Botão FILTROS da barra: abre o modal e mostra quantos filtros estão ativos. */
export function FiltersButton({ count, onClick }: { count: number; onClick: () => void }) {
  return (
    <button
      type="button"
      className={"btn" + (count > 0 ? " btn-active" : "")}
      onClick={onClick}
      aria-haspopup="dialog"
      title={count > 0 ? `${count} filtro(s) ativo(s)` : "Filtrar"}
    >
      <SlidersHorizontal size={14} strokeWidth={1.8} /> FILTROS
      {count > 0 && <span className="btn-badge">{count}</span>}
    </button>
  );
}

type Props<T> = {
  title?: string;
  value: T;
  /** Estado de "sem filtro" — o LIMPAR volta o rascunho para ele. */
  empty: T;
  onApply: (v: T) => void;
  onClose: () => void;
  /** Campos do filtro, sobre o rascunho. */
  children: (draft: T, set: (patch: Partial<T>) => void) => ReactNode;
  width?: number;
};

/**
 * Modal de filtros — padrão do sistema para os recortes não temporais (o
 * período tem o próprio botão). A barra da tela fica só com busca, período,
 * FILTROS e ações; os campos vivem aqui, num rascunho que só vale em
 * APLICAR. Esc fecha; Enter aplica.
 */
export default function FiltersModal<T extends object>({
  title = "FILTROS",
  value,
  empty,
  onApply,
  onClose,
  children,
  width = 520,
}: Props<T>) {
  const [draft, setDraft] = useState<T>(value);
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    ref.current?.focus();
  }, []);

  function submit(e: FormEvent) {
    e.preventDefault();
    onApply(draft);
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.stopPropagation();
      onClose();
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        ref={ref}
        className="modal filters-modal"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
        style={{ maxWidth: width }}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={onKeyDown}
      >
        <div className="modal-hd">
          <span>{title}</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <form className="modal-form" onSubmit={submit}>
          {/* --no-clip: o popover dos selects precisa transbordar o corpo. */}
          <div className="modal-bd modal-bd--no-clip">
            {children(draft, (patch) => setDraft((d) => ({ ...d, ...patch })))}
          </div>
          <div className="filters-modal-ft">
            <button type="button" className="btn btn-ghost" onClick={() => setDraft(empty)}>
              LIMPAR
            </button>
            <div style={{ marginLeft: "auto" }} />
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
