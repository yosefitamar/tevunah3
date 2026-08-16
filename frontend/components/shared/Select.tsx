"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";

export type SelectOption = {
  value: string;
  label: string;
  disabled?: boolean;
};

type SelectProps = {
  value: string;
  onChange: (v: string) => void;
  options: SelectOption[];
  /** Texto exibido quando value === "" (default: "—"). */
  placeholder?: string;
  disabled?: boolean;
  id?: string;
  /** Classe extra no wrapper, p/ casos com layout customizado. */
  className?: string;
  /** Renderiza chevron interno (default true). */
  chevron?: boolean;
};

/**
 * Select custom no padrão Tevunah (terminal/tactical). Substitui o <select>
 * nativo, cuja lista de opções é controlada pelo SO e não pode ser estilizada.
 * - Mesma estética do .form-field (borda, bg, foco).
 * - Setas navegam, Enter seleciona, Escape fecha, clique fora fecha.
 */
export default function Select({
  value,
  onChange,
  options,
  placeholder = "—",
  disabled = false,
  id,
  className = "",
  chevron = true,
}: SelectProps) {
  const [open, setOpen] = useState(false);
  const [hover, setHover] = useState<number>(-1);
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const btnRef = useRef<HTMLButtonElement | null>(null);
  const hoveredRef = useRef<HTMLLIElement | null>(null);

  const current = options.find((o) => o.value === value);
  const label = current ? current.label : placeholder;

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (!wrapRef.current) return;
      if (!wrapRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, []);

  useEffect(() => {
    if (open) {
      const idx = options.findIndex((o) => o.value === value);
      setHover(idx >= 0 ? idx : 0);
    }
  }, [open, value, options]);

  // Mantém a opção destacada visível — seta e type-ahead podem levar o
  // destaque para fora da área rolável em listas longas.
  useEffect(() => {
    if (open) hoveredRef.current?.scrollIntoView({ block: "nearest" });
  }, [open, hover]);

  function commit(v: string) {
    onChange(v);
    setOpen(false);
    btnRef.current?.focus();
  }

  function onKey(e: React.KeyboardEvent<HTMLButtonElement>) {
    if (disabled) return;
    if (!open) {
      if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        setOpen(true);
      }
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHover((h) => {
        let n = h;
        for (let i = 0; i < options.length; i++) {
          n = (n + 1) % options.length;
          if (!options[n].disabled) return n;
        }
        return h;
      });
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHover((h) => {
        let n = h;
        for (let i = 0; i < options.length; i++) {
          n = (n - 1 + options.length) % options.length;
          if (!options[n].disabled) return n;
        }
        return h;
      });
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (hover >= 0 && hover < options.length && !options[hover].disabled) {
        commit(options[hover].value);
      }
    } else if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
    } else if (e.key === "Tab") {
      setOpen(false);
    } else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      // Type-ahead: digitar salta para a opção que começa com o que foi
      // teclado (como no <select> nativo). Sem isto, listas longas — os 184
      // municípios do Ceará, por exemplo — só se navegam rolando.
      e.preventDefault();
      typeAhead(e.key);
    }
  }

  // Buffer do type-ahead: teclas em sequência formam o prefixo; a pausa o
  // reinicia, então "F","O","R" busca "FOR" mas "F" … "F" cicla os "F".
  const typeBuf = useRef("");
  const typeTimer = useRef<number | null>(null);

  function typeAhead(key: string) {
    if (typeTimer.current) window.clearTimeout(typeTimer.current);
    typeBuf.current += key.toLowerCase();
    typeTimer.current = window.setTimeout(() => {
      typeBuf.current = "";
    }, 800);

    const buf = typeBuf.current;
    const match = (o: SelectOption) =>
      !o.disabled && o.label.toLowerCase().startsWith(buf);

    // Com um único caractere repetido, cicla entre as opções que começam com
    // ele em vez de travar sempre na primeira.
    const repeated = buf.length > 1 && buf.split("").every((c) => c === buf[0]);
    if (repeated) {
      const c = buf[0];
      const cyclic = (o: SelectOption) =>
        !o.disabled && o.label.toLowerCase().startsWith(c);
      const after = options.findIndex((o, i) => i > hover && cyclic(o));
      const idx = after >= 0 ? after : options.findIndex(cyclic);
      if (idx >= 0) setHover(idx);
      return;
    }
    const idx = options.findIndex(match);
    if (idx >= 0) setHover(idx);
  }

  return (
    <div
      className={"sel " + (className || "")}
      ref={wrapRef}
      data-open={open || undefined}
    >
      <button
        id={id}
        ref={btnRef}
        type="button"
        className={"sel-trigger" + (!current ? " sel-trigger--empty" : "")}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={onKey}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="sel-label">{label}</span>
        {chevron && (
          <ChevronDown
            size={12}
            strokeWidth={1.8}
            className="sel-chevron-icon"
          />
        )}
      </button>
      {open && !disabled && options.length > 0 && (
        <ul className="sel-list" role="listbox">
          {options.map((o, i) => (
            <li
              key={o.value + ":" + i}
              role="option"
              aria-selected={o.value === value}
              aria-disabled={o.disabled || undefined}
              className={
                "sel-item" +
                (i === hover ? " sel-item--on" : "") +
                (o.value === value ? " sel-item--sel" : "") +
                (o.disabled ? " sel-item--disabled" : "")
              }
              ref={i === hover ? hoveredRef : undefined}
              onMouseEnter={() => !o.disabled && setHover(i)}
              onMouseDown={(e) => {
                e.preventDefault();
                if (!o.disabled) commit(o.value);
              }}
            >
              {o.label}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
