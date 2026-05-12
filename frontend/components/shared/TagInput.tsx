"use client";

import { useRef, useState, type KeyboardEvent } from "react";
import { X } from "lucide-react";

type Props = {
  value: string[];
  onChange: (next: string[]) => void;
  /** Normaliza o texto antes de comitar (default: nenhuma). */
  normalize?: "lower" | "none";
  /** Limite máximo de itens. 0 = sem limite. */
  maxItems?: number;
  /** Permite ID/ref externos. */
  inputId?: string;
};

/**
 * Input de tags estilo "chip". Cada valor confirmado vira uma badge dentro do
 * próprio campo, com botão × para remover. Atalhos:
 *   Enter, vírgula  → confirma o que está sendo digitado
 *   Backspace vazio → remove o último chip
 * Blur também confirma o conteúdo pendente.
 */
export default function TagInput({
  value,
  onChange,
  normalize = "none",
  maxItems = 0,
  inputId,
}: Props) {
  const [typing, setTyping] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  function normalizeTag(s: string): string {
    const t = s.trim();
    if (!t) return "";
    return normalize === "lower" ? t.toLowerCase() : t;
  }

  function commit(raw?: string) {
    const t = normalizeTag(raw ?? typing);
    if (!t) {
      setTyping("");
      return;
    }
    if (value.includes(t)) {
      setTyping("");
      return;
    }
    if (maxItems > 0 && value.length >= maxItems) {
      setTyping("");
      return;
    }
    onChange([...value, t]);
    setTyping("");
  }

  function removeAt(i: number) {
    const next = value.slice();
    next.splice(i, 1);
    onChange(next);
  }

  function onKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commit();
    } else if (e.key === "Backspace" && typing === "" && value.length > 0) {
      e.preventDefault();
      removeAt(value.length - 1);
    }
  }

  function onChangeText(s: string) {
    // Aceita colar "a, b, c" e confirma tudo de uma vez.
    if (s.includes(",")) {
      const parts = s.split(",");
      const last = parts.pop() ?? "";
      parts.forEach((p) => commit(p));
      setTyping(last);
      return;
    }
    setTyping(s);
  }

  const atCapacity = maxItems > 0 && value.length >= maxItems;

  return (
    <div
      className="tag-input"
      onClick={() => inputRef.current?.focus()}
      role="group"
    >
      {value.map((t, i) => (
        <span className="tag-input-chip" key={t + i}>
          <span className="tag-input-chip-lbl">{t}</span>
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
        </span>
      ))}
      <input
        ref={inputRef}
        id={inputId}
        type="text"
        value={typing}
        onChange={(e) => onChangeText(e.target.value)}
        onKeyDown={onKey}
        onBlur={() => commit()}
        className="tag-input-text"
        disabled={atCapacity}
      />
    </div>
  );
}
