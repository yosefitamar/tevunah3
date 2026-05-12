"use client";

import { useEffect, useRef, useState, type DragEvent } from "react";
import { Camera, Trash2 } from "lucide-react";

// PrimaryPhotoPicker — escolhe a foto primária (3:4) da entidade. Modo Create:
// guarda o File em memória; o caller faz upload após criar a entidade. Modo
// Edit: aceita uma URL já persistida + callbacks de upload/remover assíncronos.
export type PrimaryPhotoPickerProps = {
  // Modo Create: arquivo local, callback de escolha.
  file?: File | null;
  onFileChange?: (f: File | null) => void;
  // Modo Edit: URL atual (se houver) + callbacks de upload/remoção.
  currentURL?: string | null;
  onUpload?: (f: File) => Promise<void>;
  onRemove?: () => Promise<void>;
  busy?: boolean;
  label?: string; // default "FOTO PRINCIPAL"
};

export default function PrimaryPhotoPicker(props: PrimaryPhotoPickerProps) {
  const {
    file,
    onFileChange,
    currentURL,
    onUpload,
    onRemove,
    busy = false,
    label = "FOTO PRINCIPAL",
  } = props;

  const inputRef = useRef<HTMLInputElement>(null);
  const [previewURL, setPreviewURL] = useState<string | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!file) {
      setPreviewURL(null);
      return;
    }
    const url = URL.createObjectURL(file);
    setPreviewURL(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  function validate(f: File): string | null {
    if (!/^image\/(jpeg|png)$/.test(f.type)) return "Envie uma imagem JPEG ou PNG.";
    if (f.size > 5 * 1024 * 1024) return "A foto excede 5 MiB.";
    return null;
  }

  async function pick(f?: File) {
    if (!f) return;
    const v = validate(f);
    if (v) {
      setErr(v);
      return;
    }
    setErr(null);
    if (onUpload) {
      try {
        await onUpload(f);
      } catch (e) {
        setErr((e as Error).message || "Falha no upload");
      }
    } else if (onFileChange) {
      onFileChange(f);
    }
  }

  function clear() {
    if (onRemove) {
      onRemove().catch((e) => setErr((e as Error).message || "Falha ao remover"));
    } else if (onFileChange) {
      onFileChange(null);
    }
    if (inputRef.current) inputRef.current.value = "";
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
    pick(e.dataTransfer.files?.[0]);
  }

  const displayURL = previewURL ?? currentURL ?? null;
  const hasPhoto = !!displayURL;

  return (
    <div className="primary-photo">
      <span className="primary-photo-lbl">{label}</span>
      <div
        className={"photo-frame" + (dragOver ? " photo-frame--drop" : "")}
        onClick={() => !busy && inputRef.current?.click()}
        onDragEnter={onDragOver}
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
        role="button"
        aria-label={`${label} — clique ou arraste`}
      >
        {hasPhoto ? (
          <img src={displayURL!} alt={label.toLowerCase()} />
        ) : (
          <div className="photo-placeholder">
            <Camera size={22} strokeWidth={1.4} />
            <span>3X4</span>
            <span className="muted">CLIQUE OU ARRASTE</span>
          </div>
        )}
      </div>
      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png"
        style={{ display: "none" }}
        onChange={(e) => pick(e.target.files?.[0])}
      />
      {hasPhoto && (
        <div className="primary-photo-actions">
          <button
            type="button"
            className="btn btn-ghost photo-clear"
            onClick={() => inputRef.current?.click()}
            disabled={busy}
          >
            SUBSTITUIR
          </button>
          <button
            type="button"
            className="btn btn-ghost photo-clear"
            onClick={clear}
            disabled={busy}
          >
            <Trash2 size={11} /> REMOVER
          </button>
        </div>
      )}
      {err && (
        <div className="muted" style={{ fontSize: 10, color: "var(--crit)" }}>
          ⚠ {err}
        </div>
      )}
    </div>
  );
}
