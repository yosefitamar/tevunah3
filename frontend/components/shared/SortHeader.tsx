"use client";

import type { CSSProperties } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";

export type SortDir = "asc" | "desc";
export type SortState = { field: string; dir: SortDir } | null;

type Props = {
  field: string;
  label: string;
  sort: SortState;
  onChange: (next: SortState) => void;
  width?: number | string;
  align?: "left" | "center" | "right";
  /** Se true, o header não é clicável (não é sortable). */
  disabled?: boolean;
};

/**
 * Header de coluna sortable. Click alterna asc → desc → unset.
 * Mantém a estética terminal/tactical do design system (uppercase, tracking,
 * cor muted; ícone destacado quando ativo).
 */
export default function SortHeader({
  field,
  label,
  sort,
  onChange,
  width,
  align = "left",
  disabled,
}: Props) {
  const active = sort?.field === field;
  const dir = active ? sort?.dir : null;

  const style: CSSProperties = {};
  if (width !== undefined) style.width = typeof width === "number" ? `${width}px` : width;
  if (align) style.textAlign = align;

  function toggle() {
    if (disabled) return;
    if (!active) onChange({ field, dir: "asc" });
    else if (dir === "asc") onChange({ field, dir: "desc" });
    else onChange(null); // remove sort
  }

  if (disabled) {
    return <th style={style}>{label}</th>;
  }

  return (
    <th style={style}>
      <button
        type="button"
        className={"sort-th" + (active ? " sort-th--on" : "")}
        onClick={toggle}
        aria-sort={
          active ? (dir === "asc" ? "ascending" : "descending") : "none"
        }
      >
        <span>{label}</span>
        {dir === "asc" && <ArrowUp size={11} strokeWidth={2} />}
        {dir === "desc" && <ArrowDown size={11} strokeWidth={2} />}
        {!active && <ArrowUpDown size={11} strokeWidth={1.6} />}
      </button>
    </th>
  );
}
