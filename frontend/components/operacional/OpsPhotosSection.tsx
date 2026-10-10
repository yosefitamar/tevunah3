"use client";

import { useRef, useState, type DragEvent } from "react";
import { ImagePlus, Trash2 } from "lucide-react";
import {
  OPS_MAX_PHOTOS,
  addOpsPhoto,
  deleteOpsPhoto,
  opsPhotoURL,
  type OpsOccurrence,
  type OpsPhoto,
} from "@/lib/ops-reports-api";
import { shrinkImage } from "@/lib/image-resize";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";

type Props = {
  occ: OpsOccurrence;
  /** Ficha gravada + permissão de edição: o analista anexa e remove fotos. */
  editable?: boolean;
  onChange?: (photos: OpsPhoto[]) => void;
};

function formatSize(bytes: number) {
  return bytes >= 1 << 20 ? `${(bytes / (1 << 20)).toFixed(1).replace(".", ",")} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

/**
 * Fotos da ocorrência — apreensão, prisão, local. O relatório do CPRAIO não
 * traz imagem: o analista anexa aqui (clicando ou arrastando) e elas vão ao
 * SIPOM em base64, junto com a ocorrência — o arquivo original, sem
 * recompressão. Só a foto acima do limite do servidor (5 MiB) é reduzida no
 * navegador antes de subir.
 */
export default function OpsPhotosSection({ occ, editable = false, onChange }: Props) {
  const photos = occ.photos ?? [];
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [busy, setBusy] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<string | null>(null);
  const room = OPS_MAX_PHOTOS - photos.length;
  const canAdd = editable && room > 0 && !busy;

  async function add(files: File[]) {
    if (!occ.id || files.length === 0) return;
    const images = files.filter((f) => /^image\/(jpeg|png)$/.test(f.type));
    setError(null);
    if (images.length < files.length) setError("Só JPEG ou PNG — os demais arquivos foram ignorados.");
    if (images.length > room) setError(`Cabem só mais ${room} foto${room === 1 ? "" : "s"} (limite de ${OPS_MAX_PHOTOS}).`);
    setBusy(true);
    try {
      for (const f of images.slice(0, room)) {
        const r = await addOpsPhoto(occ.id, await shrinkImage(f));
        onChange?.(r.photos);
      }
    } catch (e) {
      setError((e as ApiError).message || "Erro ao anexar a foto");
    } finally {
      setBusy(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  async function remove(p: OpsPhoto) {
    if (!occ.id) return;
    setBusy(true);
    setError(null);
    try {
      await deleteOpsPhoto(occ.id, p.id);
      onChange?.(photos.filter((x) => x.id !== p.id));
    } catch (e) {
      setError((e as ApiError).message || "Erro ao remover a foto");
    } finally {
      setBusy(false);
      setConfirming(null);
    }
  }

  const over = (e: DragEvent) => {
    if (!canAdd) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
    setDragging(true);
  };

  return (
    <>
      {editable && (
        <div
          className={"ops-photo-drop" + (dragging ? " ops-photo-drop--over" : "")}
          onDragEnter={over}
          onDragOver={over}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => {
            e.preventDefault();
            setDragging(false);
            if (canAdd) add(Array.from(e.dataTransfer.files));
          }}
        >
          <button type="button" className="btn btn-primary btn-sm" disabled={!canAdd} onClick={() => inputRef.current?.click()}>
            <ImagePlus size={13} strokeWidth={1.8} /> {busy ? "ENVIANDO…" : "ADICIONAR FOTOS"}
          </button>
          <span className="muted">
            {room > 0
              ? `OU ARRASTE AS IMAGENS PARA CÁ · JPEG OU PNG · ${photos.length} DE ${OPS_MAX_PHOTOS}`
              : `LIMITE DE ${OPS_MAX_PHOTOS} FOTOS ATINGIDO — REMOVA UMA PARA ANEXAR OUTRA`}
          </span>
          <input
            ref={inputRef}
            type="file"
            accept="image/jpeg,image/png"
            multiple
            hidden
            onChange={(e) => add(Array.from(e.target.files ?? []))}
          />
        </div>
      )}
      {error && <div className="banner banner-error">⚠ {error}</div>}
      {photos.length === 0 ? (
        <div className="ops-empty">NENHUMA FOTO ANEXADA{editable ? "" : " A ESTA OCORRÊNCIA"}</div>
      ) : (
        <div className="ops-photos">
          {photos.map((p, i) => (
            <figure key={p.id} className="ops-photo">
              <a href={opsPhotoURL(occ.id!, p.id)} target="_blank" rel="noreferrer" title="Abrir em tamanho real">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={opsPhotoURL(occ.id!, p.id)} alt={`Foto ${i + 1} da ocorrência`} loading="lazy" />
              </a>
              <figcaption>
                <span title={`Anexada em ${formatBR(p.created_at)}`}>
                  FOTO {i + 1} · {formatSize(p.size)}
                </span>
                {editable &&
                  (confirming === p.id ? (
                    <span className="ops-photo-confirm">
                      <button type="button" className="ops-field-btn ops-field-btn--danger" disabled={busy} onClick={() => remove(p)}>
                        CONFIRMAR
                      </button>
                      <button type="button" className="ops-field-btn" disabled={busy} onClick={() => setConfirming(null)}>
                        CANCELAR
                      </button>
                    </span>
                  ) : (
                    <button type="button" className="ops-field-btn" disabled={busy} onClick={() => setConfirming(p.id)} title="Remover a foto">
                      <Trash2 size={11} /> REMOVER
                    </button>
                  ))}
              </figcaption>
            </figure>
          ))}
        </div>
      )}
    </>
  );
}
