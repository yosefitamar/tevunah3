"use client";

import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { X } from "lucide-react";
import { listReports, type Report } from "@/lib/reports-api";

type Props = {
  /** String serializada com itens separados por "; ". */
  value: string;
  onChange: (next: string) => void;
  disabled?: boolean;
};

const SEP = "; ";

function parse(v: string): string[] {
  if (!v) return [];
  return v
    .split(/;\s*/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function serialize(items: string[]): string {
  return items.join(SEP);
}

type Suggestion = { label: string; hint: string };

/**
 * Multi-select para o campo REF. Sugere RIs difundidos (Nº · ASSUNTO) e aceita
 * texto livre para referências externas. Internamente trabalha com array; persiste
 * como string única separada por "; " (formato do campo `reference` no backend).
 */
export default function RefMultiSelect({ value, onChange, disabled }: Props) {
  const items = parse(value);
  const [typing, setTyping] = useState("");
  const [open, setOpen] = useState(false);
  const [hover, setHover] = useState(-1);
  const [options, setOptions] = useState<Suggestion[]>([]);
  const [loaded, setLoaded] = useState(false);
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // Lazy-load: carrega RIs difundidos no primeiro foco.
  useEffect(() => {
    if (!loaded && open) {
      let cancelled = false;
      listReports({ status: "difundido", limit: 200 })
        .then((r) => {
          if (cancelled) return;
          const opts: Suggestion[] = (r.items || [])
            .filter((it: Report) => !!it.number)
            .map((it: Report) => ({
              label: `RI Nº ${it.number}`,
              hint: it.subject || "",
            }));
          setOptions(opts);
          setLoaded(true);
        })
        .catch(() => setLoaded(true));
      return () => {
        cancelled = true;
      };
    }
  }, [open, loaded]);

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (!wrapRef.current) return;
      if (!wrapRef.current.contains(e.target as Node)) {
        commitTyping();
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [typing, value]);

  function emit(next: string[]) {
    onChange(serialize(next));
  }

  function addItem(s: string) {
    const t = s.trim();
    if (!t) return;
    if (items.includes(t)) return;
    emit([...items, t]);
  }

  function removeAt(i: number) {
    const next = items.slice();
    next.splice(i, 1);
    emit(next);
  }

  function commitTyping() {
    if (!typing.trim()) return;
    addItem(typing);
    setTyping("");
  }

  const q = typing.trim().toUpperCase();
  const filtered = options
    .filter((o) => !items.includes(o.label))
    .filter((o) =>
      q
        ? o.label.toUpperCase().includes(q) ||
          o.hint.toUpperCase().includes(q)
        : true,
    )
    .slice(0, 8);

  function onKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setHover((h) => Math.min(filtered.length - 1, h + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHover((h) => Math.max(0, h - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (hover >= 0 && hover < filtered.length) {
        addItem(filtered[hover].label);
        setTyping("");
        setHover(-1);
      } else {
        commitTyping();
      }
    } else if (e.key === ",") {
      e.preventDefault();
      commitTyping();
    } else if (e.key === "Backspace" && typing === "" && items.length > 0) {
      e.preventDefault();
      removeAt(items.length - 1);
    } else if (e.key === "Escape") {
      setOpen(false);
      setHover(-1);
    }
  }

  return (
    <div
      className="tag-input"
      ref={wrapRef}
      onClick={() => inputRef.current?.focus()}
      style={{ position: "relative" }}
    >
      {items.map((t, i) => (
        <span className="tag-input-chip" key={t + i}>
          <span className="tag-input-chip-lbl">{t}</span>
          {!disabled && (
            <button
              type="button"
              className="tag-input-chip-x"
              aria-label={`remover ${t}`}
              onClick={(e) => {
                e.stopPropagation();
                removeAt(i);
              }}
            >
              <X size={11} strokeWidth={2} />
            </button>
          )}
        </span>
      ))}
      <input
        ref={inputRef}
        type="text"
        value={typing}
        disabled={disabled}
        autoComplete="off"
        placeholder={items.length === 0 ? "RI Nº … ou texto livre" : ""}
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setTyping(e.target.value);
          setOpen(true);
          setHover(-1);
        }}
        onKeyDown={onKey}
        onBlur={() => {
          /* commit handled by outside-click */
        }}
        className="tag-input-text"
      />
      {open && !disabled && filtered.length > 0 && (
        <ul
          className="combobox-list"
          role="listbox"
          style={{ top: "100%", left: 0, right: 0 }}
        >
          {filtered.map((o, i) => (
            <li
              key={o.label}
              role="option"
              aria-selected={i === hover}
              className={
                "combobox-item" + (i === hover ? " combobox-item--on" : "")
              }
              onMouseEnter={() => setHover(i)}
              onMouseDown={(e) => {
                e.preventDefault();
                addItem(o.label);
                setTyping("");
                setHover(-1);
                inputRef.current?.focus();
              }}
            >
              <div>{o.label}</div>
              {o.hint && (
                <div
                  style={{
                    fontSize: 10,
                    color: "var(--fg-3)",
                    letterSpacing: "0.06em",
                    textTransform: "uppercase",
                    marginTop: 2,
                  }}
                >
                  {o.hint}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
      {open && !disabled && filtered.length === 0 && typing.trim() && (
        <div
          className="combobox-empty"
          style={{ top: "100%", left: 0, right: 0 }}
        >
          // sem RIs correspondentes — ENTER grava como referência externa
        </div>
      )}
    </div>
  );
}
