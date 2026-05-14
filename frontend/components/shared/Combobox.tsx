"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";

type ComboboxProps = {
  value: string;
  onChange: (v: string) => void;
  /** Lista curada de opções a sugerir. Usuário pode digitar livre fora dela. */
  options: string[];
  placeholder?: string;
  /** Quando true, força uppercase no que o usuário digita (placa, chassi, etc.). */
  uppercase?: boolean;
  /** Limita a quantidade de itens visíveis no popover. */
  maxItems?: number;
  /** Desabilita o input (não permite digitação nem abertura do popover). */
  disabled?: boolean;
  id?: string;
};

/**
 * Combobox = input texto + dropdown de sugestões filtráveis.
 * - Aceita valor livre: o usuário pode digitar qualquer coisa fora da lista.
 * - Filtra as opções por substring (case-insensitive) conforme digita.
 * - Setas ↓/↑ navegam, Enter seleciona, Escape fecha.
 * - Visualmente coerente com o restante do form-field (mesma borda/bg/foco).
 */
export default function Combobox({
  value,
  onChange,
  options,
  placeholder,
  uppercase = false,
  maxItems = 8,
  disabled = false,
  id,
}: ComboboxProps) {
  const [open, setOpen] = useState(false);
  const [hover, setHover] = useState<number>(-1);
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const q = value.trim().toUpperCase();
  const filtered = q
    ? options.filter((o) => o.toUpperCase().includes(q)).slice(0, maxItems)
    : options.slice(0, maxItems);

  // Click fora fecha.
  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (!wrapRef.current) return;
      if (!wrapRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, []);

  function commit(v: string) {
    onChange(uppercase ? v.toUpperCase() : v);
    setOpen(false);
    setHover(-1);
  }

  function onKey(e: React.KeyboardEvent<HTMLInputElement>) {
    if (!open && (e.key === "ArrowDown" || e.key === "Enter")) {
      setOpen(true);
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHover((h) => Math.min(filtered.length - 1, h + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHover((h) => Math.max(0, h - 1));
    } else if (e.key === "Enter") {
      if (hover >= 0 && hover < filtered.length) {
        e.preventDefault();
        commit(filtered[hover]);
      }
    } else if (e.key === "Escape") {
      setOpen(false);
      setHover(-1);
    }
  }

  return (
    <div className="combobox" ref={wrapRef}>
      <input
        id={id}
        ref={inputRef}
        type="text"
        autoComplete="off"
        value={value}
        disabled={disabled}
        placeholder={placeholder}
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          const v = uppercase ? e.target.value.toUpperCase() : e.target.value;
          onChange(v);
          setOpen(true);
          setHover(-1);
        }}
        onKeyDown={onKey}
        className="combobox-input"
      />
      <button
        type="button"
        className="combobox-chevron"
        tabIndex={-1}
        disabled={disabled}
        onClick={() => {
          setOpen((o) => !o);
          inputRef.current?.focus();
        }}
        aria-label="Abrir lista"
      >
        <ChevronDown size={13} strokeWidth={1.8} />
      </button>
      {open && !disabled && filtered.length > 0 && (
        <ul className="combobox-list" role="listbox">
          {filtered.map((o, i) => (
            <li
              key={o}
              role="option"
              aria-selected={i === hover}
              className={"combobox-item" + (i === hover ? " combobox-item--on" : "")}
              onMouseEnter={() => setHover(i)}
              onMouseDown={(e) => {
                e.preventDefault();
                commit(o);
              }}
            >
              {o}
            </li>
          ))}
        </ul>
      )}
      {open && !disabled && filtered.length === 0 && q && (
        <div className="combobox-empty">
          // sem sugestões — valor será gravado como digitado
        </div>
      )}
    </div>
  );
}
