"use client";

import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Plus, Search, UserPlus } from "lucide-react";
import { getEntity, listEntities, photoURL } from "@/lib/entities-api";
import CreateEntidadeModal from "../entidades/CreateEntidadeModal";
import type { Entity, PersonAttrs } from "@/lib/entities-types";
import { INVOLVED_ROLES } from "@/lib/incidents-api";
import type { ApiError } from "@/lib/api";
import Select from "../shared/Select";

// A confirmação de vítima precisa dos dados de identificação (CPF, mãe,
// nascimento, foto) para o agente descartar homônimo — daí os attrs virem
// junto da escolha, e não só o id.
type Picked = {
  id: string;
  name: string;
  kind: string;
  version: number;
  attrs?: PersonAttrs;
};

type Props = {
  /** IDs já vinculados — filtrados dos resultados. */
  exclude: string[];
  /** Chamado quando o usuário escolhe uma entidade + papel. */
  onPick: (entity: Picked, role: string) => void;
  disabled?: boolean;
};

/** Dados que o relatório já trouxe, para abrir o cadastro preenchido. */
export type NewPersonSeed = {
  name?: string;
  motherName?: string;
  dateOfBirth?: string;
  alias?: string;
  description?: string;
};

/**
 * Busca no banco de entidades e adiciona um envolvido à ocorrência.
 * O papel (AUTOR/VÍTIMA/etc.) é texto livre com sugestões. Reusado no modal
 * de criação (lista pendente em memória) e no drawer (vínculo imediato).
 */
export default function InvolvedPicker({ exclude, onPick, disabled }: Props) {
  const [q, setQ] = useState("");
  const [role, setRole] = useState("");
  const [results, setResults] = useState<Entity[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Cadastro inline: a vítima de um CVLI quase nunca tem dossiê prévio, e
  // mandar o analista para outra tela perderia o formulário preenchido.
  const [newOpen, setNewOpen] = useState(false);
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

  /** Vincula a pessoa recém-cadastrada, já com o papel escolhido. */
  async function pickCreated(id: string) {
    setNewOpen(false);
    try {
      const r = await getEntity(id);
      onPick(
        {
          id: r.entity.id,
          name: r.entity.name,
          kind: r.entity.kind,
          version: r.entity.version,
          attrs: (r.entity.attrs ?? undefined) as PersonAttrs | undefined,
        },
        role.trim(),
      );
      setQ("");
      setResults([]);
    } catch (e) {
      setError((e as ApiError).message || "Cadastro feito, mas o vínculo falhou");
    }
  }

  function pick(e: Entity) {
    if (!role) return;
    onPick(
      {
        id: e.id,
        name: e.name,
        kind: e.kind,
        version: e.version,
        attrs: (e.attrs ?? undefined) as PersonAttrs | undefined,
      },
      role.trim(),
    );
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
          <Select
            value={role}
            onChange={setRole}
            placeholder="SELECIONE…"
            options={[
              { value: "", label: "SELECIONE…" },
              ...INVOLVED_ROLES.map((r) => ({ value: r, label: r })),
            ]}
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
        {!role && q.trim() && (
          <div className="muted" style={{ fontSize: 11 }}>
            // ESCOLHA O PAPEL ANTES DE VINCULAR
          </div>
        )}
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
        {!loading && q.trim() && visible.length === 0 && (
          <button
            type="button"
            className="btn btn-ghost"
            style={{ alignSelf: "flex-start", marginTop: 4 }}
            disabled={!role}
            title={role ? undefined : "Escolha o papel antes de cadastrar"}
            onClick={() => setNewOpen(true)}
          >
            <UserPlus size={12} />
            CADASTRAR “{q.trim().toUpperCase()}” E VINCULAR
          </button>
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
              disabled={!role}
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

      {/* Portal: o wizard de entidade tem <form> próprio e o picker vive
          dentro do <form> da ocorrência. */}
      {newOpen &&
        typeof document !== "undefined" &&
        createPortal(
          <CreateEntidadeModal
            initialKind="person"
            initialPerson={{ name: q.trim() }}
            onClose={() => setNewOpen(false)}
            onCreated={pickCreated}
          />,
          document.body,
        )}
    </div>
  );
}
