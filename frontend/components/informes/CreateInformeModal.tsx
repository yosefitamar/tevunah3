"use client";

import { useRef, useState, type FormEvent } from "react";
import { ImageUp, X } from "lucide-react";
import { createInforme, uploadInformePhoto } from "@/lib/informes-api";
import { clearanceLabel } from "@/lib/types";
import { useFileDrop } from "@/lib/useFileDrop";
import type { ApiError } from "@/lib/api";
import DateInput from "../shared/DateInput";
import Select from "../shared/Select";

type Props = {
  onClose: () => void;
  onCreated: (id: string) => void;
};

export default function CreateInformeModal({ onClose, onCreated }: Props) {
  const today = new Date().toISOString().slice(0, 10);
  const [occurredOn, setOccurredOn] = useState(today);
  const [location, setLocation] = useState("");
  const [how, setHow] = useState("");
  const [description, setDescription] = useState("");
  const [clearance, setClearance] = useState(1);
  const [photo, setPhoto] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement | null>(null);
  const { dragging, handlers } = useFileDrop((f) => pickPhoto(f));

  function pickPhoto(f: File | null) {
    if (photoPreview) URL.revokeObjectURL(photoPreview);
    if (!f) {
      setPhoto(null);
      setPhotoPreview("");
      return;
    }
    if (!/^image\/(png|jpeg)$/.test(f.type)) {
      setErr("Foto deve ser JPEG ou PNG.");
      return;
    }
    setErr(null);
    setPhoto(f);
    setPhotoPreview(URL.createObjectURL(f));
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!description.trim()) {
      setErr("A descrição é obrigatória.");
      return;
    }
    setErr(null);
    setBusy(true);
    try {
      const { informe } = await createInforme({
        occurred_on: occurredOn,
        location: location.trim(),
        how: how.trim(),
        description: description.trim(),
        required_clearance: clearance,
      });
      if (photo) {
        try {
          await uploadInformePhoto(informe.id, photo);
        } catch (e) {
          // Informe criado; só a foto falhou — não bloqueia o fluxo.
          setErr("Informe criado, mas a foto falhou: " + ((e as ApiError).message || "erro"));
        }
      }
      onCreated(informe.id);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao criar informe");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 560 }}>
        <div className="modal-hd">
          <span>NOVO INFORME</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>

        <form className="modal-bd" onSubmit={onSubmit} autoComplete="off">
          <div className="form-grid-2">
            <label className="form-field">
              <span>QUANDO</span>
              <DateInput value={occurredOn} onChange={setOccurredOn} />
            </label>
            <div className="form-field">
              <span>NÍVEL DE ACESSO</span>
              <Select
                value={String(clearance)}
                onChange={(v) => setClearance(Number(v))}
                options={[1, 2, 3, 4, 5].map((n) => ({ value: String(n), label: clearanceLabel(n) }))}
              />
            </div>
          </div>

          <label className="form-field">
            <span>ONDE</span>
            <input
              type="text"
              value={location}
              onChange={(e) => setLocation(e.target.value)}
              placeholder="Local do fato"
              autoFocus
            />
          </label>

          <label className="form-field">
            <span>COMO</span>
            <input
              type="text"
              value={how}
              onChange={(e) => setHow(e.target.value)}
              placeholder="Como tomou ciência / modo"
            />
          </label>

          <label className="form-field">
            <span>DESCRIÇÃO</span>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="O que você tomou ciência…"
              rows={5}
              required
            />
          </label>

          <div className="form-field">
            <span>FOTO (OPCIONAL)</span>
            <div className="brasao-row">
              <div
                className="brasao-preview"
                role="button"
                title="Clique ou arraste uma imagem"
                onClick={() => fileRef.current?.click()}
                style={{ cursor: "pointer", outline: dragging ? "2px dashed var(--accent)" : undefined }}
                {...handlers}
              >
                {photoPreview ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={photoPreview} alt="prévia" />
                ) : (
                  <div className="brasao-empty">
                    <ImageUp size={24} strokeWidth={1.4} />
                    <span>SOLTE OU CLIQUE</span>
                  </div>
                )}
              </div>
              <div className="brasao-actions">
                <input
                  ref={fileRef}
                  type="file"
                  accept="image/png,image/jpeg"
                  style={{ display: "none" }}
                  onChange={(e) => pickPhoto(e.target.files?.[0] ?? null)}
                />
                <button type="button" className="btn btn-outline" onClick={() => fileRef.current?.click()}>
                  <ImageUp size={14} strokeWidth={1.6} /> {photo ? "TROCAR FOTO" : "ANEXAR FOTO"}
                </button>
                {photo && (
                  <button type="button" className="btn btn-ghost" onClick={() => pickPhoto(null)}>
                    REMOVER
                  </button>
                )}
                <span className="form-hint">PNG ou JPEG, até 5 MiB.</span>
              </div>
            </div>
          </div>

          {err && <div className="banner banner-error">⚠ {err}</div>}

          <div className="modal-ft">
            <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
              CANCELAR
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy}>
              {busy ? "CRIANDO…" : "CRIAR"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
