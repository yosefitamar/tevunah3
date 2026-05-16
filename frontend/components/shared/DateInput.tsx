"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Calendar, ChevronLeft, ChevronRight } from "lucide-react";

type DateInputProps = {
  /** Valor em ISO YYYY-MM-DD (compatível com input[type=date]). String vazia = sem data. */
  value: string;
  onChange: (v: string) => void;
  /** Texto exibido quando vazio (default DD/MM/AAAA). */
  placeholder?: string;
  disabled?: boolean;
  id?: string;
  /** Limita o intervalo navegável. Formato ISO. */
  min?: string;
  max?: string;
};

const WEEKDAYS = ["D", "S", "T", "Q", "Q", "S", "S"];
const MONTHS = [
  "janeiro",
  "fevereiro",
  "março",
  "abril",
  "maio",
  "junho",
  "julho",
  "agosto",
  "setembro",
  "outubro",
  "novembro",
  "dezembro",
];

function pad(n: number): string {
  return String(n).padStart(2, "0");
}

function toIso(y: number, m: number, d: number): string {
  return `${y}-${pad(m + 1)}-${pad(d)}`;
}

function parseIso(v: string): { y: number; m: number; d: number } | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(v);
  if (!match) return null;
  const y = Number(match[1]);
  const m = Number(match[2]) - 1;
  const d = Number(match[3]);
  if (m < 0 || m > 11 || d < 1 || d > 31) return null;
  return { y, m, d };
}

function toBR(v: string): string {
  const p = parseIso(v);
  if (!p) return "";
  return `${pad(p.d)}/${pad(p.m + 1)}/${p.y}`;
}

function parseBR(v: string): string | null {
  const match = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(v);
  if (!match) return null;
  const d = Number(match[1]);
  const m = Number(match[2]);
  const y = Number(match[3]);
  if (m < 1 || m > 12 || d < 1 || d > 31) return null;
  // Valida data (rejeita 31/02 etc).
  const dt = new Date(y, m - 1, d);
  if (dt.getFullYear() !== y || dt.getMonth() !== m - 1 || dt.getDate() !== d) {
    return null;
  }
  return `${y}-${pad(m)}-${pad(d)}`;
}

function daysInMonth(y: number, m: number): number {
  return new Date(y, m + 1, 0).getDate();
}

/**
 * DateInput custom no padrão Tevunah. Substitui <input type="date">, cujo
 * popover de calendário é controlado pelo navegador e não estilizável.
 * - Valor sempre em ISO (compatível com APIs e payloads atuais).
 * - Display em DD/MM/AAAA com digitação livre + popover de calendário.
 * - Setas no header navegam mês a mês; clique no rótulo do mês mostra anos.
 */
export default function DateInput({
  value,
  onChange,
  placeholder = "DD/MM/AAAA",
  disabled = false,
  id,
  min,
  max,
}: DateInputProps) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState(toBR(value));
  const [viewYM, setViewYM] = useState<{ y: number; m: number }>(() => {
    const p = parseIso(value) ?? null;
    const now = new Date();
    return p ? { y: p.y, m: p.m } : { y: now.getFullYear(), m: now.getMonth() };
  });
  const [yearPicker, setYearPicker] = useState(false);
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    setText(toBR(value));
    const p = parseIso(value);
    if (p) setViewYM({ y: p.y, m: p.m });
  }, [value]);

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (!wrapRef.current) return;
      if (!wrapRef.current.contains(e.target as Node)) {
        setOpen(false);
        setYearPicker(false);
        // Reverte texto se inválido.
        setText(toBR(value));
      }
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [value]);

  const parsedMin = min ? parseIso(min) : null;
  const parsedMax = max ? parseIso(max) : null;

  function inRange(iso: string): boolean {
    if (parsedMin && iso < min!) return false;
    if (parsedMax && iso > max!) return false;
    return true;
  }

  function commit(iso: string) {
    if (!inRange(iso)) return;
    onChange(iso);
    setText(toBR(iso));
    setOpen(false);
    setYearPicker(false);
  }

  function clear() {
    onChange("");
    setText("");
    setOpen(false);
    setYearPicker(false);
  }

  function today() {
    const now = new Date();
    commit(toIso(now.getFullYear(), now.getMonth(), now.getDate()));
  }

  // Auto-formatador (insere "/" enquanto digita).
  function onTextChange(e: React.ChangeEvent<HTMLInputElement>) {
    const raw = e.target.value;
    // Permite que o usuário apague livremente
    let digits = raw.replace(/\D/g, "").slice(0, 8);
    let next = digits;
    if (digits.length > 4) {
      next = `${digits.slice(0, 2)}/${digits.slice(2, 4)}/${digits.slice(4)}`;
    } else if (digits.length > 2) {
      next = `${digits.slice(0, 2)}/${digits.slice(2)}`;
    }
    setText(next);
    if (digits.length === 8) {
      const iso = parseBR(next);
      if (iso && inRange(iso)) {
        onChange(iso);
        const p = parseIso(iso)!;
        setViewYM({ y: p.y, m: p.m });
      }
    } else if (digits.length === 0) {
      onChange("");
    }
  }

  function onTextBlur() {
    // Se inválido, reverte.
    const iso = parseBR(text);
    if (iso && inRange(iso)) {
      onChange(iso);
    } else if (text === "") {
      onChange("");
    } else {
      setText(toBR(value));
    }
  }

  function shiftMonth(delta: number) {
    setViewYM((cur) => {
      const nm = cur.m + delta;
      const ny = cur.y + Math.floor(nm / 12);
      const m = ((nm % 12) + 12) % 12;
      return { y: ny, m };
    });
  }

  // Constrói o grid 6×7 do mês visível (preenche com dias do mês anterior/próximo).
  const grid = useMemo(() => {
    const { y, m } = viewYM;
    const first = new Date(y, m, 1);
    const firstDow = first.getDay(); // 0 = domingo
    const dim = daysInMonth(y, m);
    const prevDim = daysInMonth(y, m - 1);
    const cells: Array<{
      y: number;
      m: number;
      d: number;
      iso: string;
      cur: boolean;
    }> = [];
    for (let i = 0; i < 42; i++) {
      const off = i - firstDow;
      let cy = y;
      let cm = m;
      let cd = off + 1;
      let cur = true;
      if (off < 0) {
        cd = prevDim + off + 1;
        cm = m - 1;
        if (cm < 0) {
          cm = 11;
          cy = y - 1;
        }
        cur = false;
      } else if (off >= dim) {
        cd = off - dim + 1;
        cm = m + 1;
        if (cm > 11) {
          cm = 0;
          cy = y + 1;
        }
        cur = false;
      }
      cells.push({ y: cy, m: cm, d: cd, iso: toIso(cy, cm, cd), cur });
    }
    return cells;
  }, [viewYM]);

  const todayIso = useMemo(() => {
    const t = new Date();
    return toIso(t.getFullYear(), t.getMonth(), t.getDate());
  }, []);

  const selected = parseIso(value);
  const yearRange = useMemo(() => {
    const start = Math.floor(viewYM.y / 12) * 12;
    return Array.from({ length: 12 }, (_, i) => start + i);
  }, [viewYM.y]);

  return (
    <div className="dpk" ref={wrapRef} data-open={open || undefined}>
      <div className={"dpk-trigger" + (disabled ? " dpk-trigger--disabled" : "")}>
        <input
          id={id}
          ref={inputRef}
          type="text"
          inputMode="numeric"
          autoComplete="off"
          className="dpk-input"
          placeholder={placeholder}
          value={text}
          disabled={disabled}
          onChange={onTextChange}
          onBlur={onTextBlur}
          onFocus={() => setOpen(true)}
        />
        <button
          type="button"
          className="dpk-icon"
          tabIndex={-1}
          disabled={disabled}
          onClick={() => {
            setOpen((o) => !o);
            inputRef.current?.focus();
          }}
          aria-label="Abrir calendário"
        >
          <Calendar size={13} strokeWidth={1.8} />
        </button>
      </div>

      {open && !disabled && (
        <div className="dpk-pop" role="dialog">
          <div className="dpk-head">
            <button
              type="button"
              className="dpk-head-label"
              onClick={() => setYearPicker((y) => !y)}
            >
              {MONTHS[viewYM.m]} de {viewYM.y}
              <span className="dpk-head-caret">▾</span>
            </button>
            <div className="dpk-nav">
              <button
                type="button"
                onClick={() => shiftMonth(-1)}
                aria-label="Mês anterior"
              >
                <ChevronLeft size={14} strokeWidth={1.8} />
              </button>
              <button
                type="button"
                onClick={() => shiftMonth(1)}
                aria-label="Próximo mês"
              >
                <ChevronRight size={14} strokeWidth={1.8} />
              </button>
            </div>
          </div>

          {yearPicker ? (
            <div className="dpk-years">
              {yearRange.map((y) => (
                <button
                  key={y}
                  type="button"
                  className={
                    "dpk-year" + (y === viewYM.y ? " dpk-year--on" : "")
                  }
                  onClick={() => {
                    setViewYM((cur) => ({ ...cur, y }));
                    setYearPicker(false);
                  }}
                >
                  {y}
                </button>
              ))}
            </div>
          ) : (
            <>
              <div className="dpk-dow">
                {WEEKDAYS.map((w, i) => (
                  <span key={i}>{w}</span>
                ))}
              </div>
              <div className="dpk-grid">
                {grid.map((c, i) => {
                  const isSel = selected
                    ? c.y === selected.y &&
                      c.m === selected.m &&
                      c.d === selected.d
                    : false;
                  const isToday = c.iso === todayIso;
                  const ok = inRange(c.iso);
                  return (
                    <button
                      key={i}
                      type="button"
                      disabled={!ok}
                      className={
                        "dpk-day" +
                        (c.cur ? "" : " dpk-day--out") +
                        (isSel ? " dpk-day--sel" : "") +
                        (isToday ? " dpk-day--today" : "")
                      }
                      onClick={() => commit(c.iso)}
                    >
                      {c.d}
                    </button>
                  );
                })}
              </div>
            </>
          )}

          <div className="dpk-ft">
            <button type="button" className="dpk-ft-btn" onClick={clear}>
              LIMPAR
            </button>
            <button type="button" className="dpk-ft-btn" onClick={today}>
              HOJE
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
