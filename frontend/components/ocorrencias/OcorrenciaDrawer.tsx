"use client";

import { useCallback, useEffect, useState } from "react";
import { Camera, Eye, Pencil, Trash, Trash2, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useModal } from "@/contexts/ModalContext";
import {
  INCIDENT_TYPE_LABEL,
  INCIDENT_TYPE_PILL,
  INCIDENT_TYPES,
  addIncidentEntity,
  deleteIncident,
  deleteIncidentPhoto,
  getIncident,
  incidentPhotoURL,
  removeIncidentEntity,
  updateIncident,
  uploadIncidentPhoto,
  type Incident,
  type IncidentMeans,
  type IncidentType,
  type UpdateIncidentInput,
} from "@/lib/incidents-api";
import { canDeleteIncidents, canEditIncidents } from "@/lib/permissions";
import { photoURL } from "@/lib/entities-api";
import { formatBR, formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import DateInput from "../shared/DateInput";
import Select from "../shared/Select";
import { useIncidentLocations } from "@/lib/useIncidentLocations";
import GeoField from "./GeoField";
import InvolvedPicker from "./InvolvedPicker";
import MeansField from "./MeansField";
import PlaceField from "./PlaceField";
import ConfirmVitimaModal, { type VictimCandidate } from "./ConfirmVitimaModal";
import DeceasedPhoto from "../shared/DeceasedPhoto";
import { isVictimRole } from "@/lib/incidents-api";
import EntidadeDrawer from "../entidades/EntidadeDrawer";

// Cor do papel na lista de envolvidos: vítima puxa para o vermelho, acusado
// para o âmbar, testemunha fica neutra — leitura da linha sem precisar ler.
function roleToneClass(role: string): string {
  switch (role.trim().toUpperCase()) {
    case "VÍTIMA":
    case "VITIMA":
      return "role-tone role-tone--victim";
    case "ACUSADO":
      return "role-tone role-tone--accused";
    default:
      return "role-tone";
  }
}

type Props = {
  incidentId: string;
  onClose: () => void;
  onChanged: () => void;
};

// O dossiê abre SEMPRE em leitura: editar é ato deliberado, e o trilho de
// auditoria registra cada alteração (incident.update, com before/after).

export default function OcorrenciaDrawer({ incidentId, onClose, onChanged }: Props) {
  const { user: me } = useAuth();
  const modal = useModal();
  const [data, setData] = useState<Incident | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [geoLat, setGeoLat] = useState("");
  const [geoLng, setGeoLng] = useState("");
  // Detalhe do meio é editado localmente e persistido no blur (como os
  // demais campos de texto deste drawer).
  const [meansDetail, setMeansDetail] = useState("");
  const [neighborhood, setNeighborhood] = useState("");
  const { neighborhoodsOf, reload: reloadLocations } = useIncidentLocations();
  const [photoBust, setPhotoBust] = useState(0);
  const [entityOverlayId, setEntityOverlayId] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  // Vínculo de vítima aguardando confirmação (checagem de homônimo).
  const [pendingVictim, setPendingVictim] = useState<{
    candidate: VictimCandidate;
    role: string;
  } | null>(null);

  // canEdit = tem permissão; editing = ativou o modo edição. Os campos só
  // aceitam entrada quando as duas coisas valem.
  const hasEditPerm = canEditIncidents(me);
  const canEdit = hasEditPerm && editing;
  const canDelete = canDeleteIncidents(me);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const r = await getIncident(incidentId);
      setData(r.incident);
      setGeoLat(r.incident.latitude != null ? String(r.incident.latitude) : "");
      setGeoLng(r.incident.longitude != null ? String(r.incident.longitude) : "");
      setMeansDetail(r.incident.means_detail);
      setNeighborhood(r.incident.neighborhood);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [incidentId]);

  useEffect(() => {
    reload();
  }, [reload]);

  async function patch(input: UpdateIncidentInput) {
    if (!data) return;
    setError(null);
    try {
      const r = await updateIncident(data.id, input);
      setData(r.incident);
      setNeighborhood(r.incident.neighborhood);
      onChanged();
      // Um bairro novo passa a valer como sugestão para os próximos registros.
      if (input.city !== undefined || input.neighborhood !== undefined) {
        reloadLocations();
      }
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gravar");
    }
  }

  // Coordenadas: debounce 700ms; compara numericamente pra não re-disparar
  // por diferença de formatação após o patch.
  useEffect(() => {
    if (!data) return;
    const latChanged =
      geoLat.trim() === "" ? data.latitude != null : Number(geoLat) !== data.latitude;
    const lngChanged =
      geoLng.trim() === "" ? data.longitude != null : Number(geoLng) !== data.longitude;
    if (!latChanged && !lngChanged) return;
    if (geoLat.trim() !== "" && !Number.isFinite(Number(geoLat))) return;
    if (geoLng.trim() !== "" && !Number.isFinite(Number(geoLng))) return;
    const h = window.setTimeout(() => {
      patch({
        latitude: geoLat.trim() ? Number(geoLat) : null,
        longitude: geoLng.trim() ? Number(geoLng) : null,
      });
    }, 700);
    return () => window.clearTimeout(h);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [geoLat, geoLng, data?.id, data?.latitude, data?.longitude]);

  function pickPhoto() {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/jpeg,image/png";
    input.onchange = async () => {
      const f = input.files?.[0];
      if (!f || !data) return;
      if (f.size > 5 * 1024 * 1024) {
        setError("Foto deve ter no máximo 5 MiB");
        return;
      }
      try {
        const updated = await uploadIncidentPhoto(data.id, f);
        setData(updated);
        setPhotoBust((b) => b + 1);
        onChanged();
      } catch (e) {
        setError((e as ApiError).message || "Erro ao subir foto");
      }
    };
    input.click();
  }

  async function removePhoto() {
    if (!data) return;
    const ok = await modal.confirm({
      title: "REMOVER FOTO",
      message: "A foto da ocorrência será removida. Confirmar?",
      confirm: "REMOVER",
      cancel: "CANCELAR",
      variant: "warning",
      danger: true,
    });
    if (!ok) return;
    try {
      await deleteIncidentPhoto(data.id);
      await reload();
      onChanged();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao remover foto");
    }
  }

  async function linkInvolved(entityID: string, role: string) {
    if (!data) return;
    try {
      const r = await addIncidentEntity(data.id, entityID, role);
      setData(r.incident);
      onChanged();
    } catch (err) {
      setError((err as ApiError).message || "Erro ao vincular");
    }
  }

  async function addInvolved(e: VictimCandidate, role: string) {
    if (!data) return;
    // Vítima de homicídio marca óbito no acervo inteiro: passa pela
    // confirmação antes de gravar. Os demais papéis entram direto.
    if (data.type === "homicidio" && isVictimRole(role) && e.kind === "person") {
      setPendingVictim({ candidate: e, role });
      return;
    }
    await linkInvolved(e.id, role);
  }

  async function removeInvolved(entityID: string) {
    if (!data) return;
    try {
      await removeIncidentEntity(data.id, entityID);
      await reload();
      onChanged();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao desvincular");
    }
  }

  async function onDelete() {
    if (!data) return;
    const ok = await modal.confirm({
      title: "EXCLUIR OCORRÊNCIA",
      message:
        "A ocorrência será removida (exclusão lógica, auditada). Confirmar?",
      confirm: "EXCLUIR",
      cancel: "CANCELAR",
      variant: "warning",
      danger: true,
    });
    if (!ok) return;
    setActing(true);
    try {
      await deleteIncident(data.id);
      onChanged();
      onClose();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao excluir");
      setActing(false);
    }
  }

  return (
    <>
      <div className="drawer-backdrop" onClick={onClose}>
        <aside
          className="drawer drawer--wide"
          onClick={(e) => e.stopPropagation()}
          aria-label="Ocorrência"
        >
          <div className="drawer-hd">
            <span>OCORRÊNCIA</span>
            <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
              <X size={14} />
            </button>
          </div>

          <div className="drawer-bd">
            {loading && <div className="muted">// CARREGANDO…</div>}
            {error && <div className="banner banner-error">⚠ {error}</div>}

            {data && (
              <>
                <div className="dossier-head dossier-head--actions">
                  <div className="dossier-meta">
                    <span className={"pill " + INCIDENT_TYPE_PILL[data.type]}>
                      {INCIDENT_TYPE_LABEL[data.type]}
                    </span>
                    <span>
                      {formatBRDate(data.occurred_on)}
                      {data.occurred_time ? ` · ${data.occurred_time}` : ""}
                    </span>
                    {(data.city || data.neighborhood) && (
                      <span>
                        {[data.neighborhood, data.city].filter(Boolean).join(" · ")}
                      </span>
                    )}
                    <span className={"pill " + (editing ? "hold" : "cold")}>
                      {editing ? "EDITANDO" : "VISUALIZAÇÃO"}
                    </span>
                  </div>
                  {hasEditPerm && (
                    <button
                      type="button"
                      className={"btn" + (editing ? "" : " btn-primary")}
                      onClick={() => setEditing((v) => !v)}
                    >
                      {editing ? (
                        <>
                          <Eye size={13} strokeWidth={1.8} /> CONCLUIR EDIÇÃO
                        </>
                      ) : (
                        <>
                          <Pencil size={13} strokeWidth={1.8} /> EDITAR
                        </>
                      )}
                    </button>
                  )}
                </div>

                <div className="entity-form" style={{ marginTop: 12 }}>
                  <fieldset className="form-fieldset" disabled={!canEdit}>
                    <legend>DADOS</legend>
                    <div className="form-grid-2">
                      <div className="form-field">
                        <span>TIPO</span>
                        <Select
                          value={data.type}
                          disabled={!canEdit}
                          onChange={(v) => {
                            if (v === data.type) return;
                            // Meio utilizado só vale para homicídio: sair do
                            // tipo limpa o campo em vez de deixá-lo órfão.
                            patch(
                              v === "homicidio"
                                ? { type: v as IncidentType }
                                : { type: v as IncidentType, means: "", means_detail: "" },
                            );
                          }}
                          options={INCIDENT_TYPES.map((t) => ({
                            value: t,
                            label: INCIDENT_TYPE_LABEL[t],
                          }))}
                        />
                      </div>
                      <label className="form-field">
                        <span>FICHA CIOPS</span>
                        <input
                          type="text"
                          defaultValue={data.ciops_record}
                          onBlur={(e) =>
                            e.target.value !== data.ciops_record &&
                            patch({ ciops_record: e.target.value })
                          }
                        />
                      </label>
                    </div>
                    <div className="form-grid-2">
                      <div className="form-field">
                        <span>DATA</span>
                        <DateInput
                          value={data.occurred_on}
                          disabled={!canEdit}
                          onChange={(v) => v !== data.occurred_on && patch({ occurred_on: v })}
                        />
                      </div>
                      <label className="form-field">
                        <span>HORA</span>
                        <input
                          type="text"
                          defaultValue={data.occurred_time ?? ""}
                          placeholder="HH:MM"
                          maxLength={5}
                          onBlur={(e) => {
                            const v = e.target.value.trim();
                            if (v && !/^\d{1,2}:\d{2}$/.test(v)) {
                              setError("Hora inválida — use HH:MM");
                              return;
                            }
                            if (v !== (data.occurred_time ?? "")) {
                              patch({ occurred_time: v || null });
                            }
                          }}
                        />
                      </label>
                    </div>

                    {(data.type === "homicidio" || data.means !== "") && (
                      <MeansField
                        means={data.means}
                        detail={meansDetail}
                        disabled={!canEdit}
                        onChangeMeans={(v) => {
                          if (v === data.means) return;
                          if (v !== "outros") setMeansDetail("");
                          patch(
                            v === "outros"
                              ? { means: v }
                              : { means: v, means_detail: "" },
                          );
                        }}
                        onChangeDetail={setMeansDetail}
                        onDetailBlur={(v) =>
                          v !== data.means_detail && patch({ means_detail: v })
                        }
                      />
                    )}

                    <PlaceField
                      city={data.city}
                      neighborhood={neighborhood}
                      disabled={!canEdit}
                      onChangeCity={(v) => {
                        if (v === data.city) return;
                        // Bairro pertence ao município: trocar de cidade
                        // invalida o bairro atual.
                        setNeighborhood("");
                        patch({ city: v, neighborhood: "" });
                      }}
                      onChangeNeighborhood={setNeighborhood}
                      onNeighborhoodBlur={(v) => {
                        if (v.trim().toUpperCase() === data.neighborhood) return;
                        patch({ neighborhood: v });
                      }}
                      neighborhoodOptions={neighborhoodsOf(data.city)}
                    />

                    <GeoField
                      lat={geoLat}
                      lng={geoLng}
                      disabled={!canEdit}
                      onChange={(la, lo) => {
                        setGeoLat(la);
                        setGeoLng(lo);
                      }}
                    />

                    <label className="form-field">
                      <span>DESCRIÇÃO</span>
                      <textarea
                        defaultValue={data.description}
                        rows={5}
                        onBlur={(e) =>
                          e.target.value !== data.description &&
                          patch({ description: e.target.value })
                        }
                      />
                    </label>
                  </fieldset>

                  {/* Foto */}
                  <fieldset className="form-fieldset">
                    <legend>FOTO</legend>
                    <div className="qual-photo-row">
                      <div className="qual-photo-preview">
                        {data.has_photo ? (
                          <img
                            src={incidentPhotoURL(data.id, `${data.updated_at}-${photoBust}`)}
                            alt="foto da ocorrência"
                            onError={(ev) => {
                              (ev.currentTarget as HTMLImageElement).style.display = "none";
                            }}
                          />
                        ) : (
                          <span className="muted" style={{ fontSize: 10 }}>// SEM FOTO</span>
                        )}
                      </div>
                      {canEdit && (
                        <div className="qual-photo-actions">
                          <button type="button" className="btn" onClick={pickPhoto}>
                            <Camera size={13} /> {data.has_photo ? "TROCAR FOTO" : "ADICIONAR FOTO"}
                          </button>
                          {data.has_photo && (
                            <button type="button" className="btn btn-ghost" onClick={removePhoto}>
                              REMOVER
                            </button>
                          )}
                          <div className="muted" style={{ fontSize: 9.5, marginTop: 4 }}>
                            JPEG/PNG · MÁX 5 MIB
                          </div>
                        </div>
                      )}
                    </div>
                  </fieldset>

                  {/* Envolvidos */}
                  <fieldset className="form-fieldset">
                    <legend>ENVOLVIDOS ({data.involved.length})</legend>
                    {data.involved.length === 0 && (
                      <div className="muted" style={{ fontSize: 11 }}>
                        // NENHUMA ENTIDADE VINCULADA
                      </div>
                    )}
                    {data.involved.map((e) => (
                      <div
                        key={e.entity_id}
                        className="qual-row qual-row--clickable"
                        role="button"
                        tabIndex={0}
                        onClick={() => setEntityOverlayId(e.entity_id)}
                        onKeyDown={(ev) => {
                          if (ev.key === "Enter" || ev.key === " ") {
                            ev.preventDefault();
                            setEntityOverlayId(e.entity_id);
                          }
                        }}
                      >
                        <div className="qual-row-info">
                          <div className="qual-row-name">{e.name.toUpperCase()}</div>
                          {/* O envolvimento é o que importa aqui; o tipo da
                              entidade ("PERSON") não diz nada ao analista. */}
                          <div className={"qual-row-meta " + roleToneClass(e.role)}>
                            {e.role ? e.role.toUpperCase() : "SEM PAPEL"}
                          </div>
                        </div>
                        {e.has_photo && (
                          <DeceasedPhoto
                            src={photoURL(e.entity_id, e.version)}
                            deceased={e.deceased}
                            className="qual-thumb"
                          />
                        )}
                        {canEdit && (
                          <button
                            type="button"
                            className="action-btn"
                            aria-label="Desvincular"
                            onClick={(ev) => {
                              ev.stopPropagation();
                              removeInvolved(e.entity_id);
                            }}
                          >
                            <Trash2 size={12} />
                          </button>
                        )}
                      </div>
                    ))}
                    {canEdit && (
                      <div style={{ marginTop: 12 }}>
                        <InvolvedPicker
                          exclude={data.involved.map((e) => e.entity_id)}
                          onPick={addInvolved}
                        />
                      </div>
                    )}
                  </fieldset>
                </div>

                {canDelete && (
                  <div className="drawer-section">
                    <div className="drawer-section-title">AÇÕES</div>
                    <div className="action-list action-list--inline">
                      <button
                        type="button"
                        className="action-row action-row--danger"
                        onClick={onDelete}
                        disabled={acting}
                      >
                        <span className="action-row-icon">
                          <Trash size={14} />
                        </span>
                        <span className="action-row-label">EXCLUIR OCORRÊNCIA</span>
                        <span className="action-row-hint">
                          EXCLUSÃO LÓGICA · AUDITADA
                        </span>
                      </button>
                    </div>
                  </div>
                )}

                <dl className="dossier-list">
                  <div>
                    <dt>REGISTRADO EM</dt>
                    <dd>{formatBR(data.created_at)}</dd>
                  </div>
                  <div>
                    <dt>ATUALIZADO EM</dt>
                    <dd>{formatBR(data.updated_at)}</dd>
                  </div>
                </dl>
              </>
            )}
          </div>
        </aside>
      </div>

      {pendingVictim && (
        <ConfirmVitimaModal
          candidate={pendingVictim.candidate}
          occurredOn={data?.occurred_on ?? ""}
          onCancel={() => setPendingVictim(null)}
          onConfirm={async () => {
            const { candidate, role } = pendingVictim;
            setPendingVictim(null);
            await linkInvolved(candidate.id, role);
          }}
        />
      )}

      {entityOverlayId && (
        <EntidadeDrawer
          entityId={entityOverlayId}
          onClose={() => setEntityOverlayId(null)}
          onChanged={reload}
          onOpenEntity={(id) => setEntityOverlayId(id)}
        />
      )}
    </>
  );
}
