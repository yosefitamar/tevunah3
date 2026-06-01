"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ImageUp, Trash2, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useModal } from "@/contexts/ModalContext";
import {
  deleteInforme,
  deleteInformePhoto,
  getInforme,
  informePhotoURL,
  updateInforme,
  uploadInformePhoto,
  type Informe,
} from "@/lib/informes-api";
import { canDeleteInformes, hasRole } from "@/lib/permissions";
import { clearanceLabel } from "@/lib/types";
import { useFileDrop } from "@/lib/useFileDrop";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import DateInput from "../shared/DateInput";
import Select from "../shared/Select";

type Props = {
  informeId: string;
  onClose: () => void;
  onChanged: () => void;
};

export default function InformeDrawer({ informeId, onClose, onChanged }: Props) {
  const { user: me } = useAuth();
  const modal = useModal();
  const [data, setData] = useState<Informe | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);

  // Buffers de edição.
  const [occurredOn, setOccurredOn] = useState("");
  const [location, setLocation] = useState("");
  const [how, setHow] = useState("");
  const [description, setDescription] = useState("");
  const [clearance, setClearance] = useState(1);
  const [dirty, setDirty] = useState(false);
  const [photoVer, setPhotoVer] = useState(0);
  const fileRef = useRef<HTMLInputElement | null>(null);

  const canManage =
    !!data && (me?.id === data.created_by || hasRole(me, "gestor", "administrador"));
  const canRemove = canManage && canDeleteInformes(me);
  const { dragging, handlers } = useFileDrop((f) => onPhoto(f), !canManage);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const { informe } = await getInforme(informeId);
      setData(informe);
      setOccurredOn(informe.occurred_on);
      setLocation(informe.location);
      setHow(informe.how);
      setDescription(informe.description);
      setClearance(informe.required_clearance);
      setDirty(false);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [informeId]);

  useEffect(() => {
    reload();
  }, [reload]);

  async function onSave() {
    if (!canManage) return;
    setActing(true);
    setError(null);
    try {
      const { informe } = await updateInforme(informeId, {
        occurred_on: occurredOn,
        location: location.trim(),
        how: how.trim(),
        description: description.trim(),
        required_clearance: clearance,
      });
      setData(informe);
      setDirty(false);
      onChanged();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao salvar");
    } finally {
      setActing(false);
    }
  }

  async function onDelete() {
    if (!canRemove) return;
    const ok = await modal.confirm({
      variant: "warning",
      title: "EXCLUIR INFORME",
      message: "Esta ação remove o informe da listagem. Confirmar?",
      confirm: "EXCLUIR",
      cancel: "CANCELAR",
    });
    if (!ok) return;
    setActing(true);
    try {
      await deleteInforme(informeId);
      onChanged();
      onClose();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao excluir");
      setActing(false);
    }
  }

  async function onPhoto(f: File | null) {
    if (!f || !canManage) return;
    if (!/^image\/(png|jpeg)$/.test(f.type)) {
      await modal.alert({ variant: "error", title: "FORMATO", message: "Envie PNG ou JPEG." });
      return;
    }
    setActing(true);
    try {
      const informe = await uploadInformePhoto(informeId, f);
      setData(informe);
      setPhotoVer((v) => v + 1);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao enviar foto");
    } finally {
      setActing(false);
    }
  }

  async function onPhotoDelete() {
    if (!canManage || !data?.has_photo) return;
    setActing(true);
    try {
      await deleteInformePhoto(informeId);
      await reload();
      setPhotoVer((v) => v + 1);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao remover foto");
    } finally {
      setActing(false);
    }
  }

  const ro = !canManage;

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <aside className="drawer" onClick={(e) => e.stopPropagation()}>
        <div className="drawer-hd">
          <span>INFORME</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>

        <div className="drawer-bd">
          {loading && <div className="muted">// CARREGANDO…</div>}
          {error && <div className="banner banner-error">⚠ {error}</div>}

          {!loading && data && (
            <>
              <div className="muted" style={{ fontSize: 11, marginBottom: 10 }}>
                Por {data.created_by_code} · {data.created_by_name?.toUpperCase()} ·{" "}
                criado {formatBR(data.created_at)}
              </div>

              <div className="form-grid-2">
                <label className="form-field">
                  <span>QUANDO</span>
                  <DateInput
                    value={occurredOn}
                    disabled={ro}
                    onChange={(v) => {
                      setOccurredOn(v);
                      setDirty(true);
                    }}
                  />
                </label>
                <div className="form-field">
                  <span>NÍVEL DE ACESSO</span>
                  <Select
                    value={String(clearance)}
                    disabled={ro}
                    onChange={(v) => {
                      setClearance(Number(v));
                      setDirty(true);
                    }}
                    options={[1, 2, 3, 4, 5].map((n) => ({ value: String(n), label: clearanceLabel(n) }))}
                  />
                </div>
              </div>

              <label className="form-field">
                <span>ONDE</span>
                <input
                  type="text"
                  value={location}
                  disabled={ro}
                  onChange={(e) => {
                    setLocation(e.target.value);
                    setDirty(true);
                  }}
                />
              </label>

              <label className="form-field">
                <span>COMO</span>
                <input
                  type="text"
                  value={how}
                  disabled={ro}
                  onChange={(e) => {
                    setHow(e.target.value);
                    setDirty(true);
                  }}
                />
              </label>

              <label className="form-field">
                <span>DESCRIÇÃO</span>
                <textarea
                  value={description}
                  disabled={ro}
                  rows={6}
                  onChange={(e) => {
                    setDescription(e.target.value);
                    setDirty(true);
                  }}
                />
              </label>

              <div className="form-field">
                <span>FOTO</span>
                <div className="brasao-row">
                  <div
                    className="brasao-preview"
                    role={canManage ? "button" : undefined}
                    title={canManage ? "Clique ou arraste uma imagem" : undefined}
                    onClick={() => canManage && fileRef.current?.click()}
                    style={{
                      cursor: canManage ? "pointer" : undefined,
                      outline: dragging ? "2px dashed var(--accent)" : undefined,
                    }}
                    {...handlers}
                  >
                    {data.has_photo ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={informePhotoURL(informeId, photoVer)} alt="foto do informe" />
                    ) : (
                      <div className="brasao-empty">
                        <ImageUp size={24} strokeWidth={1.4} />
                        <span>{canManage ? "SOLTE OU CLIQUE" : "SEM FOTO"}</span>
                      </div>
                    )}
                  </div>
                  {canManage && (
                    <div className="brasao-actions">
                      <input
                        ref={fileRef}
                        type="file"
                        accept="image/png,image/jpeg"
                        style={{ display: "none" }}
                        onChange={(e) => onPhoto(e.target.files?.[0] ?? null)}
                      />
                      <button type="button" className="btn btn-outline" disabled={acting} onClick={() => fileRef.current?.click()}>
                        <ImageUp size={14} strokeWidth={1.6} /> {data.has_photo ? "TROCAR" : "ANEXAR"}
                      </button>
                      {data.has_photo && (
                        <button type="button" className="btn btn-ghost" disabled={acting} onClick={onPhotoDelete}>
                          REMOVER
                        </button>
                      )}
                    </div>
                  )}
                </div>
              </div>

              {canManage && (
                <div className="drawer-section" style={{ marginTop: 8 }}>
                  <div style={{ display: "flex", gap: 8, justifyContent: "space-between" }}>
                    <button type="button" className="btn btn-primary" disabled={!dirty || acting} onClick={onSave}>
                      {acting ? "SALVANDO…" : "SALVAR"}
                    </button>
                    {canRemove && (
                      <button type="button" className="btn btn-danger" disabled={acting} onClick={onDelete}>
                        <Trash2 size={14} strokeWidth={1.8} /> EXCLUIR
                      </button>
                    )}
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </aside>
    </div>
  );
}
