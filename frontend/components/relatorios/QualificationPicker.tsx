"use client";

import { useEffect, useRef, useState } from "react";
import { Search, User, Shield, X, Plus } from "lucide-react";
import { listEntities, photoURL } from "@/lib/entities-api";
import { useFileDrop } from "@/lib/useFileDrop";
import type { Entity, PersonAttrs } from "@/lib/entities-types";
import {
  addQualification,
  uploadQualificationPhoto,
  type NewQualificationInput,
} from "@/lib/reports-api";
import type { ApiError } from "@/lib/api";

type Props = {
  reportId: string;
  disabled?: boolean;
  onAdded: () => void;
};

type Mode = "idle" | "civil" | "militar";

/**
 * Picker para adicionar qualificações ao relatório.
 * - CIVIL: busca uma pessoa no banco de entidades e grava um snapshot dos
 *   atributos no momento da adição (campo `data`).
 * - MILITAR: form livre — não existe banco de militares (ainda), então o
 *   usuário preenche os campos do template e o backend grava em `data`.
 */
export default function QualificationPicker({ reportId, disabled, onAdded }: Props) {
  const [mode, setMode] = useState<Mode>("idle");
  const [error, setError] = useState<string | null>(null);

  if (disabled) return null;

  return (
    <div className="qual-picker">
      {error && (
        <div className="banner banner-error" style={{ marginBottom: 8 }}>
          ⚠ {error}
        </div>
      )}
      {mode === "idle" && (
        <div className="qual-picker-actions">
          <button
            type="button"
            className="action-row"
            onClick={() => {
              setError(null);
              setMode("civil");
            }}
          >
            <span className="action-row-icon">
              <User size={14} />
            </span>
            <span className="action-row-label">+ CIVIL</span>
            <span className="action-row-hint">BUSCAR NO BANCO DE ENTIDADES</span>
          </button>
          <button
            type="button"
            className="action-row"
            onClick={() => {
              setError(null);
              setMode("militar");
            }}
          >
            <span className="action-row-icon">
              <Shield size={14} />
            </span>
            <span className="action-row-label">+ MILITAR</span>
            <span className="action-row-hint">PREENCHER DADOS DA QUALIFICAÇÃO</span>
          </button>
        </div>
      )}
      {mode === "civil" && (
        <CivilPicker
          reportId={reportId}
          onCancel={() => setMode("idle")}
          onAdded={() => {
            setMode("idle");
            onAdded();
          }}
          onError={setError}
        />
      )}
      {mode === "militar" && (
        <MilitarForm
          reportId={reportId}
          onCancel={() => setMode("idle")}
          onAdded={() => {
            setMode("idle");
            onAdded();
          }}
          onError={setError}
        />
      )}
    </div>
  );
}

/* ─────────── CIVIL ─────────── */

function CivilPicker({
  reportId,
  onCancel,
  onAdded,
  onError,
}: {
  reportId: string;
  onCancel: () => void;
  onAdded: () => void;
  onError: (msg: string | null) => void;
}) {
  const [q, setQ] = useState("");
  const [results, setResults] = useState<Entity[]>([]);
  const [loading, setLoading] = useState(false);
  const [adding, setAdding] = useState<string | null>(null);
  const [source, setSource] = useState("");
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    if (!q.trim()) {
      setResults([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    const h = window.setTimeout(async () => {
      try {
        const r = await listEntities({
          kind: "person",
          search: q.trim(),
          limit: 20,
        });
        if (!cancelled) setResults(r.items || []);
      } catch (e) {
        if (!cancelled) onError((e as ApiError).message || "Erro na busca");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(h);
    };
  }, [q, onError]);

  async function pick(ent: Entity) {
    const attrs = (ent.attrs ?? {}) as PersonAttrs;
    const snapshot: Record<string, unknown> = {
      nome: ent.name,
      aliases: attrs.aliases ?? [],
      genero: attrs.gender ?? "",
      data_nascimento: attrs.date_of_birth ?? "",
      nome_mae: attrs.mother_name ?? "",
      cpf: attrs.cpf ?? "",
    };
    const input: NewQualificationInput = {
      kind: "civil",
      entity_id: ent.id,
      data: snapshot,
      source: source.trim(),
    };
    setAdding(ent.id);
    onError(null);
    try {
      await addQualification(reportId, input);
      onAdded();
    } catch (e) {
      onError((e as ApiError).message || "Erro ao adicionar");
    } finally {
      setAdding(null);
    }
  }

  return (
    <div className="qual-picker-panel">
      <div className="qual-picker-head">
        <span>BUSCAR PESSOA · BANCO DE ENTIDADES</span>
        <button type="button" className="action-btn" onClick={onCancel}>
          <X size={13} />
        </button>
      </div>
      <div className="qual-picker-search">
        <Search size={13} />
        <input
          ref={inputRef}
          type="text"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Nome, CPF, alcunha…"
          autoComplete="off"
        />
      </div>
      <label className="form-field" style={{ marginTop: 8 }}>
        <span>FONTE (OPCIONAL)</span>
        <input
          type="text"
          value={source}
          onChange={(e) => setSource(e.target.value)}
          placeholder="Ex.: BNMP, SINESP, fonte humana…"
        />
      </label>
      <div className="qual-picker-results">
        {loading && <div className="muted" style={{ fontSize: 11 }}>// BUSCANDO…</div>}
        {!loading && q.trim() && results.length === 0 && (
          <div className="muted" style={{ fontSize: 11 }}>
            // NENHUM RESULTADO
          </div>
        )}
        {results.map((e) => {
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
              disabled={adding !== null}
              onClick={() => pick(e)}
            >
              <span className="qual-picker-row-name">{display}</span>
              {meta && <span className="qual-picker-row-meta">{meta}</span>}
              <span className="qual-picker-row-add">
                {adding === e.id ? "ADICIONANDO…" : <Plus size={12} />}
              </span>
              {a.has_photo && (
                <img
                  className="qual-thumb"
                  src={photoURL(e.id, e.version)}
                  alt=""
                  aria-hidden
                />
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}

/* ─────────── MILITAR ─────────── */

const MIL_FIELDS: Array<{ key: string; label: string; full?: boolean }> = [
  { key: "nome", label: "NOME COMPLETO", full: true },
  { key: "nome_guerra", label: "NOME DE GUERRA" },
  { key: "posto", label: "POSTO/GRADUAÇÃO" },
  { key: "om", label: "ORGANIZAÇÃO MILITAR (OM)", full: true },
  { key: "identidade", label: "IDENTIDADE MILITAR" },
  { key: "cpf", label: "CPF" },
];

function MilitarForm({
  reportId,
  onCancel,
  onAdded,
  onError,
}: {
  reportId: string;
  onCancel: () => void;
  onAdded: () => void;
  onError: (msg: string | null) => void;
}) {
  const [data, setData] = useState<Record<string, string>>({});
  const [source, setSource] = useState("");
  const [photo, setPhoto] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string>("");
  const [saving, setSaving] = useState(false);
  const fileRef = useRef<HTMLInputElement | null>(null);
  const { dragging, handlers } = useFileDrop((f) => pickPhoto(f));

  function pickPhoto(f: File | null) {
    if (photoPreview) URL.revokeObjectURL(photoPreview);
    if (!f) {
      setPhoto(null);
      setPhotoPreview("");
      return;
    }
    if (!/^image\/(jpeg|png)$/.test(f.type)) {
      onError("Foto deve ser JPEG ou PNG");
      return;
    }
    if (f.size > 5 * 1024 * 1024) {
      onError("Foto deve ter no máximo 5 MiB");
      return;
    }
    setPhoto(f);
    setPhotoPreview(URL.createObjectURL(f));
  }

  useEffect(() => {
    return () => {
      if (photoPreview) URL.revokeObjectURL(photoPreview);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function save() {
    if (!(data.nome ?? "").trim()) {
      onError("Nome completo é obrigatório");
      return;
    }
    setSaving(true);
    onError(null);
    try {
      const clean: Record<string, unknown> = {};
      for (const f of MIL_FIELDS) {
        const v = (data[f.key] ?? "").trim();
        if (v) clean[f.key] = v;
      }
      const created = await addQualification(reportId, {
        kind: "militar",
        data: clean,
        source: source.trim(),
      });
      if (photo) {
        try {
          await uploadQualificationPhoto(reportId, created.qualification.id, photo);
        } catch (e) {
          onError(
            "Qualificação criada, mas falhou ao subir foto: " +
              ((e as ApiError).message || "erro"),
          );
        }
      }
      onAdded();
    } catch (e) {
      onError((e as ApiError).message || "Erro ao adicionar");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="qual-picker-panel">
      <div className="qual-picker-head">
        <span>QUALIFICAÇÃO MILITAR</span>
        <button type="button" className="action-btn" onClick={onCancel}>
          <X size={13} />
        </button>
      </div>
      <div className="form-grid-2" style={{ marginTop: 8 }}>
        {MIL_FIELDS.map((f) => (
          <label
            key={f.key}
            className="form-field"
            style={f.full ? { gridColumn: "1 / -1" } : undefined}
          >
            <span>{f.label}</span>
            <input
              type="text"
              value={data[f.key] ?? ""}
              onChange={(e) => setData({ ...data, [f.key]: e.target.value })}
            />
          </label>
        ))}
      </div>
      <label className="form-field" style={{ marginTop: 8 }}>
        <span>FONTE (OPCIONAL)</span>
        <input
          type="text"
          value={source}
          onChange={(e) => setSource(e.target.value)}
        />
      </label>
      <div className="qual-photo-row">
        <div
          className="qual-photo-preview"
          role="button"
          title="Clique ou arraste uma imagem"
          onClick={() => fileRef.current?.click()}
          style={{ cursor: "pointer", outline: dragging ? "2px dashed var(--accent)" : undefined }}
          {...handlers}
        >
          {photoPreview ? (
            <img src={photoPreview} alt="prévia" />
          ) : (
            <span className="muted" style={{ fontSize: 10 }}>// SOLTE OU CLIQUE</span>
          )}
        </div>
        <div className="qual-photo-actions">
          <input
            ref={fileRef}
            type="file"
            accept="image/jpeg,image/png"
            style={{ display: "none" }}
            onChange={(e) => pickPhoto(e.target.files?.[0] ?? null)}
          />
          <button
            type="button"
            className="btn"
            onClick={() => fileRef.current?.click()}
            disabled={saving}
          >
            {photo ? "TROCAR FOTO" : "SELECIONAR FOTO"}
          </button>
          {photo && (
            <button
              type="button"
              className="btn btn-ghost"
              onClick={() => pickPhoto(null)}
              disabled={saving}
            >
              REMOVER
            </button>
          )}
          <div className="muted" style={{ fontSize: 9.5, marginTop: 4 }}>
            JPEG/PNG · MÁX 5 MIB
          </div>
        </div>
      </div>
      <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 10 }}>
        <button type="button" className="btn btn-ghost" onClick={onCancel} disabled={saving}>
          CANCELAR
        </button>
        <button type="button" className="btn btn-primary" onClick={save} disabled={saving}>
          {saving ? "GRAVANDO…" : "ADICIONAR"}
        </button>
      </div>
    </div>
  );
}
