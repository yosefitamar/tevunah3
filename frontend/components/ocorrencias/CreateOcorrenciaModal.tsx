"use client";

import { useState, type FormEvent } from "react";
import { X, Trash2 } from "lucide-react";
import {
  INCIDENT_TYPE_LABEL,
  INCIDENT_TYPES,
  createIncident,
  isVictimRole,
  type IncidentMeans,
  type IncidentType,
} from "@/lib/incidents-api";
import type { ApiError } from "@/lib/api";
import { useIncidentLocations } from "@/lib/useIncidentLocations";
import DateInput from "../shared/DateInput";
import Select from "../shared/Select";
import GeoField from "./GeoField";
import InvolvedPicker from "./InvolvedPicker";
import MeansField from "./MeansField";
import PlaceField from "./PlaceField";
import ConfirmVitimaModal, { type VictimCandidate } from "./ConfirmVitimaModal";

type Props = {
  onClose: () => void;
  onCreated: (id: string) => void;
};

type PendingInvolved = { entity_id: string; name: string; kind: string; role: string };

export default function CreateOcorrenciaModal({ onClose, onCreated }: Props) {
  const today = new Date().toISOString().slice(0, 10);
  const [type, setType] = useState<IncidentType>("homicidio");
  const [occurredOn, setOccurredOn] = useState(today);
  const [occurredTime, setOccurredTime] = useState("");
  const [ciops, setCiops] = useState("");
  const [lat, setLat] = useState("");
  const [lng, setLng] = useState("");
  const [city, setCity] = useState("");
  const [neighborhood, setNeighborhood] = useState("");
  const [description, setDescription] = useState("");
  const [means, setMeans] = useState<IncidentMeans>("");
  const [meansDetail, setMeansDetail] = useState("");
  const [involved, setInvolved] = useState<PendingInvolved[]>([]);
  const [pendingVictim, setPendingVictim] = useState<{
    candidate: VictimCandidate;
    role: string;
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const { neighborhoodsOf } = useIncidentLocations();

  function commitInvolved(e: { id: string; name: string; kind: string }, role: string) {
    setInvolved((cur) =>
      cur.some((x) => x.entity_id === e.id)
        ? cur
        : [...cur, { entity_id: e.id, name: e.name, kind: e.kind, role }],
    );
  }

  function addInvolved(e: VictimCandidate, role: string) {
    // Mesma confirmação do dossiê: aqui a marcação de óbito acontece no
    // POST, mas a decisão (é mesmo esta pessoa?) é a mesma.
    if (type === "homicidio" && isVictimRole(role) && e.kind === "person") {
      setPendingVictim({ candidate: e, role });
      return;
    }
    commitInvolved(e, role);
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (occurredTime.trim() && !/^\d{1,2}:\d{2}$/.test(occurredTime.trim())) {
      setErr("Hora inválida — use HH:MM");
      return;
    }
    setBusy(true);
    try {
      const r = await createIncident({
        type,
        occurred_on: occurredOn,
        occurred_time: occurredTime.trim() || undefined,
        ciops_record: ciops.trim(),
        latitude: lat.trim() ? Number(lat) : undefined,
        longitude: lng.trim() ? Number(lng) : undefined,
        city,
        neighborhood: neighborhood.trim(),
        description: description.trim(),
        // Meio utilizado é campo de CVLI — não vai junto nos demais tipos.
        means: type === "homicidio" ? means : "",
        means_detail: type === "homicidio" && means === "outros" ? meansDetail.trim() : "",
        involved: involved.map((i) => ({ entity_id: i.entity_id, role: i.role })),
      });
      onCreated(r.incident.id);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao registrar ocorrência");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 600 }}>
        <div className="modal-hd">
          <span>NOVA OCORRÊNCIA</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <form onSubmit={onSubmit}>
          <div className="modal-bd">
            <div className="form-grid-2">
              <div className="form-field">
                <span>TIPO</span>
                <Select
                  value={type}
                  onChange={(v) => setType(v as IncidentType)}
                  options={INCIDENT_TYPES.map((t) => ({ value: t, label: INCIDENT_TYPE_LABEL[t] }))}
                />
              </div>
              <div className="form-field">
                <span>FICHA CIOPS</span>
                <input
                  type="text"
                  value={ciops}
                  onChange={(e) => setCiops(e.target.value)}
                  maxLength={60}
                  placeholder="nº / referência"
                />
              </div>
            </div>

            <div className="form-grid-2">
              <div className="form-field">
                <span>DATA</span>
                <DateInput value={occurredOn} onChange={setOccurredOn} max={today} />
              </div>
              <label className="form-field">
                <span>HORA (OPCIONAL)</span>
                <input
                  type="text"
                  value={occurredTime}
                  onChange={(e) => setOccurredTime(e.target.value)}
                  placeholder="HH:MM"
                  maxLength={5}
                  inputMode="numeric"
                />
              </label>
            </div>

            {type === "homicidio" && (
              <MeansField
                means={means}
                detail={meansDetail}
                onChangeMeans={setMeans}
                onChangeDetail={setMeansDetail}
              />
            )}

            <PlaceField
              city={city}
              neighborhood={neighborhood}
              onChangeCity={(v) => {
                setCity(v);
                // Bairro pertence ao município: trocar de cidade invalida o
                // que estava digitado.
                setNeighborhood("");
              }}
              onChangeNeighborhood={setNeighborhood}
              neighborhoodOptions={neighborhoodsOf(city)}
            />

            <GeoField lat={lat} lng={lng} onChange={(la, lo) => { setLat(la); setLng(lo); }} />

            <label className="form-field">
              <span>DESCRIÇÃO</span>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={5}
                placeholder="relato da ocorrência…"
              />
            </label>

            <div className="form-field">
              <span>ENVOLVIDOS (OPCIONAL)</span>
              {involved.length > 0 && (
                <div className="tag-row" style={{ marginBottom: 8 }}>
                  {involved.map((i) => (
                    <span key={i.entity_id} className="tag-chip" style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                      {i.role ? `${i.role}: ` : ""}
                      {i.name.toUpperCase()}
                      <button
                        type="button"
                        className="action-btn"
                        aria-label="Remover"
                        onClick={() =>
                          setInvolved((cur) => cur.filter((x) => x.entity_id !== i.entity_id))
                        }
                      >
                        <Trash2 size={11} />
                      </button>
                    </span>
                  ))}
                </div>
              )}
              <InvolvedPicker
                exclude={involved.map((i) => i.entity_id)}
                onPick={addInvolved}
              />
            </div>

            {err && <div className="banner banner-error">⚠ {err}</div>}

            {pendingVictim && (
              <ConfirmVitimaModal
                candidate={pendingVictim.candidate}
                occurredOn={occurredOn}
                onCancel={() => setPendingVictim(null)}
                onConfirm={() => {
                  commitInvolved(pendingVictim.candidate, pendingVictim.role);
                  setPendingVictim(null);
                }}
              />
            )}
          </div>
          <div className="modal-ft">
            <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
              CANCELAR
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy}>
              {busy ? "REGISTRANDO…" : "REGISTRAR OCORRÊNCIA"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
