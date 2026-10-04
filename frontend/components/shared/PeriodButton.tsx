"use client";

import { useState } from "react";
import { Calendar, ChevronDown } from "lucide-react";
import { periodLabel, type PeriodSelection, type PresetId } from "@/lib/period";
import PeriodPickerModal from "./PeriodPickerModal";

type Props = {
  value: PeriodSelection;
  onChange: (p: PeriodSelection) => void;
  /** Título do modal (default "PERÍODO"). */
  title?: string;
  presets?: PresetId[];
};

/**
 * Filtro temporal padrão do sistema: um botão compacto na barra da tela, com
 * o recorte atual, que abre o seletor de período (atalhos, mês, intervalo).
 */
export default function PeriodButton({ value, onChange, title, presets }: Props) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button
        type="button"
        className="btn period-btn"
        onClick={() => setOpen(true)}
        aria-haspopup="dialog"
        title="Alterar período"
      >
        <Calendar size={14} strokeWidth={1.8} />
        <span className="period-btn-lbl">{periodLabel(value)}</span>
        <ChevronDown size={14} strokeWidth={1.8} />
      </button>
      {open && (
        <PeriodPickerModal
          value={value}
          title={title}
          presets={presets}
          onApply={(p) => {
            onChange(p);
            setOpen(false);
          }}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  );
}
