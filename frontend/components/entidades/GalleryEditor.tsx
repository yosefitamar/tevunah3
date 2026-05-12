"use client";

import {
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type DragEvent,
} from "react";
import { ImagePlus, Trash2 } from "lucide-react";
import { useModal } from "@/contexts/ModalContext";
import {
  deleteGalleryPhoto,
  galleryPhotoURL,
  updateGalleryPhoto,
  uploadGalleryPhoto,
} from "@/lib/entities-api";
import type { GalleryPhoto } from "@/lib/entities-types";
import type { ApiError } from "@/lib/api";

// ─────────────────────────── Pendentes (Create) ────────────────────────────
//
// Em Create a entidade ainda não existe, então mantemos as fotos em memória
// (File + caption local). O caller faz upload após createEntity.

export type PendingPhoto = {
  // Chave estável só pra React; gerada no add.
  key: string;
  file: File;
  caption: string;
};

export function PendingGalleryEditor({
  photos,
  onChange,
}: {
  photos: PendingPhoto[];
  onChange: (next: PendingPhoto[]) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragOver, setDragOver] = useState(false);
  const [previews, setPreviews] = useState<Record<string, string>>({});
  const [err, setErr] = useState<string | null>(null);

  // Cria URLs de prévia por arquivo. Limpa os object URLs ao desmontar/trocar.
  useEffect(() => {
    const map: Record<string, string> = {};
    for (const p of photos) {
      map[p.key] = URL.createObjectURL(p.file);
    }
    setPreviews(map);
    return () => {
      for (const k of Object.keys(map)) URL.revokeObjectURL(map[k]);
    };
  }, [photos]);

  function validate(f: File): string | null {
    if (!/^image\/(jpeg|png)$/.test(f.type)) return "Envie JPEG ou PNG.";
    if (f.size > 5 * 1024 * 1024) return "Foto excede 5 MiB.";
    return null;
  }

  function add(files: FileList | null | undefined) {
    if (!files) return;
    const additions: PendingPhoto[] = [];
    for (let i = 0; i < files.length; i++) {
      const f = files[i];
      const v = validate(f);
      if (v) {
        setErr(v);
        return;
      }
      additions.push({
        key: `${Date.now()}-${i}-${Math.random().toString(36).slice(2, 8)}`,
        file: f,
        caption: "",
      });
    }
    setErr(null);
    onChange([...photos, ...additions]);
    if (inputRef.current) inputRef.current.value = "";
  }

  function updateCaption(key: string, caption: string) {
    onChange(photos.map((p) => (p.key === key ? { ...p, caption } : p)));
  }
  function remove(key: string) {
    onChange(photos.filter((p) => p.key !== key));
  }

  function onDragOver(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
    if (!dragOver) setDragOver(true);
  }
  function onDragLeave(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    setDragOver(false);
  }
  function onDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    setDragOver(false);
    add(e.dataTransfer.files);
  }

  return (
    <fieldset className="form-fieldset">
      <legend>OUTRAS FOTOS</legend>
      <div className="gallery-grid">
        {photos.map((p) => (
          <div key={p.key} className="gallery-card">
            <div
              className="gallery-thumb"
              onClick={() => inputRef.current?.click()}
              role="img"
              aria-label="prévia"
            >
              {previews[p.key] && <img src={previews[p.key]} alt="" />}
            </div>
            <textarea
              className="gallery-caption"
              value={p.caption}
              onChange={(e) => updateCaption(p.key, e.target.value)}
              placeholder="LEGENDA…"
              rows={2}
              maxLength={500}
            />
            <div className="gallery-card-actions">
              <span className="muted" style={{ fontSize: 9.5 }}>
                {fmtBytes(p.file.size)}
              </span>
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() => remove(p.key)}
                aria-label="Remover"
              >
                <Trash2 size={11} /> REMOVER
              </button>
            </div>
          </div>
        ))}
        <div
          className={"gallery-add" + (dragOver ? " gallery-add--drop" : "")}
          onClick={() => inputRef.current?.click()}
          onDragEnter={onDragOver}
          onDragOver={onDragOver}
          onDragLeave={onDragLeave}
          onDrop={onDrop}
          role="button"
          aria-label="Adicionar fotos"
        >
          <ImagePlus size={22} strokeWidth={1.4} />
          <span>ADICIONAR</span>
          <span className="muted">CLIQUE OU ARRASTE</span>
        </div>
      </div>
      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png"
        multiple
        style={{ display: "none" }}
        onChange={(e: ChangeEvent<HTMLInputElement>) => add(e.target.files)}
      />
      {err && (
        <div className="muted" style={{ fontSize: 10, color: "var(--crit)" }}>
          ⚠ {err}
        </div>
      )}
    </fieldset>
  );
}

// fmtBytes: rótulo curto para a barra inferior do cartão.
function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

// ─────────────────────────── Persistidas (Drawer) ──────────────────────────
//
// No Drawer (modo edição) as fotos já existem no servidor. Editar caption faz
// PATCH otimista; remover faz DELETE com confirmação; adicionar dispara POST
// e chama onChanged pra re-fetchar a entidade no parent.

export function PersistedGalleryEditor({
  entityID,
  photos,
  onChanged,
}: {
  entityID: string;
  photos: GalleryPhoto[];
  onChanged: () => void;
}) {
  const modal = useModal();
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const [drafts, setDrafts] = useState<Record<string, string>>({});

  function captionOf(p: GalleryPhoto): string {
    return drafts[p.id] ?? p.caption;
  }

  function validate(f: File): string | null {
    if (!/^image\/(jpeg|png)$/.test(f.type)) return "Envie JPEG ou PNG.";
    if (f.size > 5 * 1024 * 1024) return "Foto excede 5 MiB.";
    return null;
  }

  async function add(files: FileList | null | undefined) {
    if (!files || files.length === 0) return;
    setBusy(true);
    try {
      for (let i = 0; i < files.length; i++) {
        const f = files[i];
        const v = validate(f);
        if (v) {
          modal.alert({ variant: "error", message: v });
          continue;
        }
        await uploadGalleryPhoto(entityID, f, "");
      }
      onChanged();
    } catch (e) {
      modal.alert({
        variant: "error",
        message: (e as ApiError).message || "Falha no upload",
      });
    } finally {
      setBusy(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  async function commitCaption(p: GalleryPhoto) {
    const next = (drafts[p.id] ?? "").trim();
    if (next === p.caption) {
      setDrafts((d) => {
        const { [p.id]: _, ...rest } = d;
        return rest;
      });
      return;
    }
    setBusy(true);
    try {
      await updateGalleryPhoto(entityID, p.id, next);
      setDrafts((d) => {
        const { [p.id]: _, ...rest } = d;
        return rest;
      });
      onChanged();
    } catch (e) {
      modal.alert({
        variant: "error",
        message: (e as ApiError).message || "Falha ao salvar legenda",
      });
    } finally {
      setBusy(false);
    }
  }

  async function remove(p: GalleryPhoto) {
    const ok = await modal.confirm({
      variant: "warning",
      title: "REMOVER FOTO",
      message: "A foto será removida permanentemente do storage.",
      confirm: "REMOVER",
      danger: true,
    });
    if (!ok) return;
    setBusy(true);
    try {
      await deleteGalleryPhoto(entityID, p.id);
      onChanged();
    } catch (e) {
      modal.alert({
        variant: "error",
        message: (e as ApiError).message || "Falha ao remover",
      });
    } finally {
      setBusy(false);
    }
  }

  function onDragOver(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
    if (!dragOver) setDragOver(true);
  }
  function onDragLeave(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    setDragOver(false);
  }
  function onDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    setDragOver(false);
    add(e.dataTransfer.files);
  }

  return (
    <fieldset className="form-fieldset">
      <legend>OUTRAS FOTOS</legend>
      <div className="gallery-grid">
        {photos.map((p) => (
          <div key={p.id} className="gallery-card">
            <a
              className="gallery-thumb"
              href={galleryPhotoURL(entityID, p.id, p.updated_at)}
              target="_blank"
              rel="noreferrer"
              aria-label="abrir foto em tamanho real"
            >
              <img
                src={galleryPhotoURL(entityID, p.id, p.updated_at)}
                alt={p.caption || "foto"}
              />
            </a>
            <textarea
              className="gallery-caption"
              value={captionOf(p)}
              onChange={(e) =>
                setDrafts((d) => ({ ...d, [p.id]: e.target.value }))
              }
              onBlur={() => commitCaption(p)}
              placeholder="LEGENDA…"
              rows={2}
              maxLength={500}
              disabled={busy}
            />
            <div className="gallery-card-actions">
              <span className="muted" style={{ fontSize: 9.5 }}>
                {drafts[p.id] !== undefined && drafts[p.id] !== p.caption
                  ? "ALTERADA · TAB OU CLIQUE FORA P/ SALVAR"
                  : ""}
              </span>
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() => remove(p)}
                disabled={busy}
                aria-label="Remover"
              >
                <Trash2 size={11} /> REMOVER
              </button>
            </div>
          </div>
        ))}
        <div
          className={"gallery-add" + (dragOver ? " gallery-add--drop" : "")}
          onClick={() => !busy && inputRef.current?.click()}
          onDragEnter={onDragOver}
          onDragOver={onDragOver}
          onDragLeave={onDragLeave}
          onDrop={onDrop}
          role="button"
          aria-label="Adicionar fotos"
        >
          <ImagePlus size={22} strokeWidth={1.4} />
          <span>{busy ? "ENVIANDO…" : "ADICIONAR"}</span>
          <span className="muted">CLIQUE OU ARRASTE</span>
        </div>
      </div>
      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png"
        multiple
        style={{ display: "none" }}
        onChange={(e: ChangeEvent<HTMLInputElement>) => add(e.target.files)}
      />
    </fieldset>
  );
}
