"use client";

import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";
import { RANGE_LABEL } from "@/lib/date-ranges";
import { formatBRDate } from "@/lib/format";
import {
  MONTH_NAMES,
  PERIOD_PRESETS,
  WEEKDAYS,
  daysBetween,
  isPeriodComplete,
  pad,
  periodBounds,
  toIso,
  todayIso,
  type PeriodSelection,
  type PresetId,
} from "@/lib/period";
import DateInput from "./DateInput";

type Tab = "month" | "range";

type Props = {
  value: PeriodSelection;
  onApply: (p: PeriodSelection) => void;
  onClose: () => void;
  /** Título do modal (default "PERÍODO"). */
  title?: string;
  /** Atalhos oferecidos; default PERIOD_PRESETS. */
  presets?: PresetId[];
};

/**
 * Seletor de período — padrão de filtro temporal do sistema, no padrão dos date range pickers de
 * analytics: atalhos à esquerda, escolha fina à direita (mês fechado ou
 * intervalo em dois calendários), resumo do recorte no rodapé. O estado é
 * rascunho até APLICAR; duplo clique num atalho ou mês aplica direto.
 * Teclado: Esc fecha, Enter aplica.
 */
export default function PeriodPickerModal({
  value,
  onApply,
  onClose,
  title = "PERÍODO",
  presets = PERIOD_PRESETS,
}: Props) {
  const [draft, setDraft] = useState<PeriodSelection>(value);
  const [tab, setTab] = useState<Tab>(value.kind === "range" ? "range" : "month");
  const dialogRef = useRef<HTMLDivElement | null>(null);

  const today = useMemo(todayIso, []);
  const nowY = Number(today.slice(0, 4));
  const nowM = Number(today.slice(5, 7)) - 1;

  // Ano exibido na grade de meses.
  const [year, setYear] = useState<number>(() => {
    if (value.kind === "month") return value.year;
    const b = periodBounds(value);
    return b.to ? Math.min(Number(b.to.slice(0, 4)), nowY) : nowY;
  });

  useEffect(() => {
    dialogRef.current?.focus();
  }, []);

  const bounds = periodBounds(draft);
  const complete = isPeriodComplete(draft);

  function apply(p: PeriodSelection = draft) {
    if (isPeriodComplete(p)) onApply(p);
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.stopPropagation();
      onClose();
    } else if (e.key === "Enter" && !(e.target instanceof HTMLInputElement)) {
      e.preventDefault();
      apply();
    }
  }

  let summary: string;
  if (draft.kind === "preset" && draft.id === "tudo") {
    summary = "TODOS OS REGISTROS, SEM RECORTE DE DATA";
  } else if (!bounds.from) {
    summary = "ESCOLHA A DATA INICIAL";
  } else if (!bounds.to) {
    summary = `${formatBRDate(bounds.from)} → ESCOLHA A DATA FINAL`;
  } else if (bounds.from > bounds.to) {
    summary = "A DATA FINAL É ANTERIOR À INICIAL";
  } else {
    const n = daysBetween(bounds.from, bounds.to);
    summary = `${formatBRDate(bounds.from)} → ${formatBRDate(bounds.to)} · ${n} ${n === 1 ? "DIA" : "DIAS"}`;
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        ref={dialogRef}
        className="modal prd"
        role="dialog"
        aria-modal="true"
        aria-labelledby="prd-title"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={onKeyDown}
      >
        <div className="modal-hd">
          <span id="prd-title">{title}</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>

        <div className="prd-body">
          <nav className="prd-presets" aria-label="Atalhos de período">
            <div className="prd-section-lbl">ATALHOS</div>
            {presets.map((id) => {
              const on = draft.kind === "preset" && draft.id === id;
              return (
                <button
                  key={id}
                  type="button"
                  className={"prd-preset" + (on ? " prd-preset--on" : "")}
                  aria-pressed={on}
                  onClick={() => setDraft({ kind: "preset", id })}
                  onDoubleClick={() => apply({ kind: "preset", id })}
                >
                  {RANGE_LABEL[id]}
                </button>
              );
            })}
          </nav>

          <div className="prd-main">
            <div className="modal-tabs prd-tabs" role="tablist">
              <button
                type="button"
                role="tab"
                aria-selected={tab === "month"}
                className={"modal-tab" + (tab === "month" ? " modal-tab--on" : "")}
                onClick={() => setTab("month")}
              >
                MÊS ESPECÍFICO
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={tab === "range"}
                className={"modal-tab" + (tab === "range" ? " modal-tab--on" : "")}
                onClick={() => setTab("range")}
              >
                INTERVALO
              </button>
            </div>

            {tab === "month" ? (
              <MonthGrid
                year={year}
                onYear={setYear}
                nowY={nowY}
                nowM={nowM}
                selected={draft.kind === "month" ? draft : null}
                onPick={(m) => setDraft({ kind: "month", year, month: m })}
                onPickApply={(m) => apply({ kind: "month", year, month: m })}
              />
            ) : (
              <RangePicker
                from={bounds.from}
                to={bounds.to}
                today={today}
                onChange={(from, to) => setDraft({ kind: "range", from, to })}
              />
            )}
          </div>
        </div>

        <div className="prd-ft">
          <span className={"prd-summary" + (complete ? "" : " prd-summary--warn")} aria-live="polite">
            {summary}
          </span>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            CANCELAR
          </button>
          <button type="button" className="btn btn-primary" onClick={() => apply()} disabled={!complete}>
            APLICAR
          </button>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────── Mês específico ───────────────────────────

function MonthGrid({
  year,
  onYear,
  nowY,
  nowM,
  selected,
  onPick,
  onPickApply,
}: {
  year: number;
  onYear: (y: number) => void;
  nowY: number;
  nowM: number;
  selected: { year: number; month: number } | null;
  onPick: (m: number) => void;
  onPickApply: (m: number) => void;
}) {
  return (
    <div className="prd-pane">
      <div className="prd-cal-head">
        <button type="button" className="prd-nav" onClick={() => onYear(year - 1)} aria-label="Ano anterior">
          <ChevronLeft size={14} strokeWidth={1.8} />
        </button>
        <span className="prd-cal-title">{year}</span>
        <button
          type="button"
          className="prd-nav"
          onClick={() => onYear(year + 1)}
          disabled={year >= nowY}
          aria-label="Próximo ano"
        >
          <ChevronRight size={14} strokeWidth={1.8} />
        </button>
      </div>
      <div className="prd-months">
        {MONTH_NAMES.map((name, m) => {
          const future = year > nowY || (year === nowY && m > nowM);
          const on = selected?.year === year && selected.month === m;
          const current = year === nowY && m === nowM;
          return (
            <button
              key={m}
              type="button"
              disabled={future}
              aria-pressed={on}
              className={
                "prd-month" + (on ? " prd-month--on" : "") + (current ? " prd-month--now" : "")
              }
              onClick={() => onPick(m)}
              onDoubleClick={() => onPickApply(m)}
            >
              {name}
            </button>
          );
        })}
      </div>
      <p className="prd-hint">Duplo clique aplica direto.</p>
    </div>
  );
}

// ───────────────────────────── Intervalo ──────────────────────────────

function RangePicker({
  from,
  to,
  today,
  onChange,
}: {
  from: string;
  to: string;
  today: string;
  onChange: (from: string, to: string) => void;
}) {
  // Mês do calendário da esquerda; o da direita é o seguinte. Abre com o
  // fim do recorte (ou o mês corrente) no calendário da direita.
  const [view, setView] = useState<{ y: number; m: number }>(() => {
    const anchor = to || from || today;
    const y = Number(anchor.slice(0, 4));
    const m = Number(anchor.slice(5, 7)) - 1;
    return m === 0 ? { y: y - 1, m: 11 } : { y, m: m - 1 };
  });
  const [hover, setHover] = useState<string | null>(null);

  // Primeiro clique fixa o início; o segundo fecha o intervalo (invertendo
  // se vier antes do início). Um terceiro clique recomeça.
  function pick(iso: string) {
    if (!from || to) {
      onChange(iso, "");
    } else if (iso < from) {
      onChange(iso, from);
    } else {
      onChange(from, iso);
    }
    setHover(null);
  }

  // Durante a seleção, o hover mostra como ficaria o intervalo.
  let lo = from;
  let hi = to;
  if (from && !to && hover) {
    [lo, hi] = hover < from ? [hover, from] : [from, hover];
  }

  const right = view.m === 11 ? { y: view.y + 1, m: 0 } : { y: view.y, m: view.m + 1 };
  const nowYM = today.slice(0, 7);
  const canNext = `${right.y}-${pad(right.m + 1)}` < nowYM;

  function shift(delta: number) {
    setView((v) => {
      const n = v.m + delta;
      return { y: v.y + Math.floor(n / 12), m: ((n % 12) + 12) % 12 };
    });
  }

  return (
    <div className="prd-pane">
      <div className="form-grid-2">
        <div className="form-field">
          <span>DE</span>
          <DateInput value={from} max={to || today} onChange={(v) => onChange(v, to)} />
        </div>
        <div className="form-field">
          <span>ATÉ</span>
          <DateInput value={to} min={from || undefined} max={today} onChange={(v) => onChange(from, v)} />
        </div>
      </div>

      <div className="prd-cals" onMouseLeave={() => setHover(null)}>
        <MonthCalendar
          y={view.y}
          m={view.m}
          lo={lo}
          hi={hi}
          today={today}
          onPick={pick}
          onHover={setHover}
          nav={
            <button type="button" className="prd-nav" onClick={() => shift(-1)} aria-label="Mês anterior">
              <ChevronLeft size={14} strokeWidth={1.8} />
            </button>
          }
          navSide="left"
        />
        <MonthCalendar
          y={right.y}
          m={right.m}
          lo={lo}
          hi={hi}
          today={today}
          onPick={pick}
          onHover={setHover}
          nav={
            <button
              type="button"
              className="prd-nav"
              onClick={() => shift(1)}
              disabled={!canNext}
              aria-label="Próximo mês"
            >
              <ChevronRight size={14} strokeWidth={1.8} />
            </button>
          }
          navSide="right"
        />
      </div>
      <p className="prd-hint">Clique no dia inicial e depois no final, ou digite as datas.</p>
    </div>
  );
}

function MonthCalendar({
  y,
  m,
  lo,
  hi,
  today,
  onPick,
  onHover,
  nav,
  navSide,
}: {
  y: number;
  m: number;
  lo: string;
  hi: string;
  today: string;
  onPick: (iso: string) => void;
  onHover: (iso: string) => void;
  nav: React.ReactNode;
  navSide: "left" | "right";
}) {
  const firstDow = new Date(y, m, 1).getDay();
  const dim = new Date(y, m + 1, 0).getDate();
  const cells: Array<string | null> = [
    ...Array.from({ length: firstDow }, () => null),
    ...Array.from({ length: dim }, (_, i) => toIso(y, m, i + 1)),
  ];

  return (
    <div className="prd-cal">
      <div className="prd-cal-head">
        {navSide === "left" ? nav : <span className="prd-nav-spacer" />}
        <span className="prd-cal-title">
          {MONTH_NAMES[m]} {y}
        </span>
        {navSide === "right" ? nav : <span className="prd-nav-spacer" />}
      </div>
      <div className="prd-dow">
        {WEEKDAYS.map((w, i) => (
          <span key={i}>{w}</span>
        ))}
      </div>
      <div className="prd-days">
        {cells.map((iso, i) => {
          if (!iso) return <span key={i} />;
          const edge = iso === lo || iso === hi;
          const inside = lo && hi && iso > lo && iso < hi;
          return (
            <button
              key={i}
              type="button"
              disabled={iso > today}
              className={
                "prd-day" +
                (edge ? " prd-day--edge" : "") +
                (inside ? " prd-day--in" : "") +
                (iso === today ? " prd-day--today" : "")
              }
              onClick={() => onPick(iso)}
              onMouseEnter={() => onHover(iso)}
            >
              {Number(iso.slice(8))}
            </button>
          );
        })}
      </div>
    </div>
  );
}
