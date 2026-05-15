"use client";

// MotherField — input do nome da mãe com autocomplete contra pessoas já
// cadastradas. Quando uma pessoa é pareada, `linkedId` recebe o id; o caller
// (form de pessoa, no modal e no dossiê) usa esse id pra criar/atualizar o
// vínculo mother_of automaticamente.
//
// Editar o texto após o pareamento descarta o link (volta a ser nome livre).

import { useEffect, useState } from "react";
import { listEntities } from "@/lib/entities-api";
import type { Entity } from "@/lib/entities-types";

export default function MotherField({
  name,
  setName,
  linkedId,
  setLinkedId,
}: {
  name: string;
  setName: (v: string) => void;
  linkedId: string;
  setLinkedId: (v: string) => void;
}) {
  const [results, setResults] = useState<Entity[]>([]);
  const [searching, setSearching] = useState(false);
  const [focused, setFocused] = useState(false);

  useEffect(() => {
    const q = name.trim();
    if (linkedId || q.length < 2) {
      setResults([]);
      return;
    }
    let alive = true;
    setSearching(true);
    const handle = window.setTimeout(() => {
      listEntities({ kind: "person", search: q, limit: 6 })
        .then((r) => alive && setResults(r.items))
        .catch(() => alive && setResults([]))
        .finally(() => alive && setSearching(false));
    }, 250);
    return () => {
      alive = false;
      window.clearTimeout(handle);
    };
  }, [name, linkedId]);

  return (
    <div className="form-field" style={{ position: "relative" }}>
      <label htmlFor="mother-name-input" style={{ display: "contents" }}>
        <span>NOME DA MÃE</span>
      </label>
      <input
        id="mother-name-input"
        type="text"
        value={name}
        onChange={(e) => {
          setName(e.target.value);
          // Edição manual descarta o pareamento — evita salvar um link com
          // entidade que não corresponde mais ao texto digitado.
          if (linkedId) setLinkedId("");
        }}
        onFocus={() => setFocused(true)}
        onBlur={() => window.setTimeout(() => setFocused(false), 150)}
        maxLength={200}
        placeholder="digite o nome para buscar pessoas já cadastradas…"
      />
      {linkedId && (
        <div
          className="mother-picked"
          title="Vínculo mãe→filho será criado automaticamente"
        >
          <span className="mother-picked__dot" /> VINCULADO À ENTIDADE EXISTENTE
        </div>
      )}
      {focused && !linkedId && name.trim().length >= 2 && (
        <div className="link-search-results mother-search-results">
          {searching && (
            <div className="muted" style={{ fontSize: 11 }}>
              // buscando…
            </div>
          )}
          {!searching && results.length === 0 && (
            <div className="muted" style={{ fontSize: 11 }}>
              // sem correspondência — será salvo como texto livre
            </div>
          )}
          {!searching &&
            results.map((e) => (
              <button
                key={e.id}
                type="button"
                className="link-search-item"
                onMouseDown={(ev) => {
                  // mousedown em vez de click pra disparar antes do blur do
                  // input fechar o dropdown.
                  ev.preventDefault();
                  setName(e.name);
                  setLinkedId(e.id);
                  setFocused(false);
                }}
              >
                <span className="link-search-kind">PESSOA</span>
                <span className="link-search-name">{e.name.toUpperCase()}</span>
              </button>
            ))}
        </div>
      )}
    </div>
  );
}
