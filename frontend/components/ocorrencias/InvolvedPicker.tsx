"use client";

import { useEffect, useRef, useState } from "react";
import { Plus, Search } from "lucide-react";
import { listEntities, photoURL } from "@/lib/entities-api";
import type { Entity, PersonAttrs } from "@/lib/entities-types";
import { INVOLVED_ROLE_SUGGESTIONS } from "@/lib/incidents-api";
import type { ApiError } from "@/lib/api";
import Combobox from "../shared/Combobox";

type Picked = { id: string; name: string; kind: string; version: number };

type Props = {
  /** IDs já vinculados — filtrados dos resultados. */
  exclude: string[];
  /** Chamado quando o usuário escolhe uma entidade + papel. */
  onPick: (entity: Picked, role: string) => void;
  disabled?: boolean;
};

/**
 * Busca no banco de entidades e adiciona um envolvido à ocorrência.
 * O papel (AUTOR/VÍTIMA/etc.) é texto livre com sugestões. Reusado no modal
 * de criação (lista pendente em memória) e no drawer (vínculo imediato).
 */
export default function InvolvedPicker({ exclude, onPick, disabled }: Props) {
  const [q, setQ] = useState("");
  const [role, setRole] = useState("ENVOLVIDO");
  const [results, setResults] = useState<Entity[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!q.trim()) {
      setResults([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    const h = window.setTimeout(async () => {
      try {
        const r = await listEntities({ search: q.trim(), limit: 20 });
        if (!cancelled) setResults(r.items || []);
      } catch (e) {
        if (!cancelled) setError((e as ApiError).message || "Erro na busca");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(h);
    };
  }, [q]);

  const excludeSet = new Set(exclude);
  const visible = results.filter((e) => !excludeSet.has(e.id));

  function pick(e: Entity) {
    onPick({ id: e.id, name: e.name, kind: e.kind, version: e.version }, role.trim());
    setQ("");
    setResults([]);
    inputRef.current?.focus();
  }

  if (disabled) return null;

  return (
    <div className="qual-picker-panel">
      <div className="form-grid-2">
        <div className="form-field">
          <span>PAPEL</span>
          <Combobox
            value={role}
            onChange={setRole}
            options={INVOLVED_ROLE_SUGGESTIONS}
            uppercase
            placeholder="ex.: AUTOR, VÍTIMA…"
          />
        </div>
        <div className="form-field">
          <span>BUSCAR ENTIDADE</span>
          <div className="qual-picker-search">
            <Search size={13} />
            <input
              ref={inputRef}
              type="text"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Nome, CPF, alcunha, placa…"
              autoComplete="off"
            />
          </div>
        </div>
      </div>

      {error && (
        <div className="banner banner-error" style={{ marginTop: 6 }}>
          ⚠ {error}
        </div>
      )}

      <div className="qual-picker-results">
        {loading && (
          <div className="muted" style={{ fontSize: 11 }}>
            // BUSCANDO…
          </div>
        )}
        {!loading && q.trim() && visible.length === 0 && (
          <div className="muted" style={{ fontSize: 11 }}>
            // NENHUM RESULTADO
          </div>
        )}
        {visible.map((e) => {
          const a = (e.attrs ?? {}) as PersonAttrs;
          const apelido = a.aliases && a.aliases.length > 0 ? a.aliases[0] : "";
          const display = apelido
            ? `${e.name.toUpperCase()} (${apelido.toUpperCase()})`
            : e.name.toUpperCase();
          const meta = [a.cpf, a.date_of_birth, a.mother_name].filter(Boolean).join(" · ");
          return (
            <button
              key={e.id}
              type="button"
              className="qual-picker-row"
              onClick={() => pick(e)}
            >
              <span className="qual-picker-row-name">{display}</span>
              {meta && <span className="qual-picker-row-meta">{meta}</span>}
              <span className="qual-picker-row-add">
                <Plus size={12} />
              </span>
              {a.has_photo && (
                <img className="qual-thumb" src={photoURL(e.id, e.version)} alt="" aria-hidden />
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}
