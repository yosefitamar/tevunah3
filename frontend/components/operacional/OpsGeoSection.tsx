"use client";

import { useEffect, useState } from "react";
import { Crosshair, MapPin, Save } from "lucide-react";
import { setOpsGeo, type OpsOccurrence } from "@/lib/ops-reports-api";
import { googleMapsURL } from "@/lib/incidents-api";
import type { ApiError } from "@/lib/api";
import { OpsActions, OpsField, OpsFields } from "./OpsField";

type Props = {
  occ: OpsOccurrence;
  /** Ficha gravada + permissão de edição: o analista pode informar o ponto. */
  editable?: boolean;
  /** Chamado com a ficha já regravada e recalculada. */
  onChange?: (occ: OpsOccurrence) => void;
};

// Como o ponto foi obtido — e o quanto confiar nele.
function precisionPill(occ: OpsOccurrence): { cls: string; label: string; title: string } {
  if (occ.geo_source === "manual") {
    return { cls: "cold", label: "INFORMADA PELO ANALISTA", title: "Ponto informado na ficha" };
  }
  switch (occ.geo_precision) {
    case "porta":
      return { cls: "active", label: "ENDEREÇO LOCALIZADO", title: "Rua e número encontrados no mapa" };
    case "rua":
      return { cls: "info", label: "RUA LOCALIZADA", title: "A rua foi encontrada; o número, não" };
    default:
      return {
        cls: "hold",
        label: "APROXIMADA · BAIRRO",
        title: "Só o bairro foi encontrado: o ponto é o centro dele, não o local do fato",
      };
  }
}

/**
 * Coordenada da ocorrência importada. Vem do geocodificador da agência, a
 * partir do endereço do relatório — exata, só da rua ou aproximada (centro do
 * bairro). Sem coordenada a ocorrência fica pendente; o analista resolve
 * informando o ponto aqui, ou corrigindo o endereço e mandando localizar de
 * novo. O ponto informado por ele vale sobre o automático.
 */
export default function OpsGeoSection({ occ, editable = false, onChange }: Props) {
  const located = occ.latitude != null && occ.longitude != null;
  const [lat, setLat] = useState("");
  const [lng, setLng] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // A ficha mudou (gravou, localizou, corrigiu o endereço): o campo acompanha.
  useEffect(() => {
    setLat(occ.latitude != null ? occ.latitude.toFixed(6) : "");
    setLng(occ.longitude != null ? occ.longitude.toFixed(6) : "");
  }, [occ.latitude, occ.longitude]);

  const latN = Number(lat.replace(",", "."));
  const lngN = Number(lng.replace(",", "."));
  const valid =
    lat.trim() !== "" &&
    lng.trim() !== "" &&
    Number.isFinite(latN) &&
    Number.isFinite(lngN) &&
    Math.abs(latN) <= 90 &&
    Math.abs(lngN) <= 180;
  const dirty = !located || latN.toFixed(6) !== occ.latitude?.toFixed(6) || lngN.toFixed(6) !== occ.longitude?.toFixed(6);

  async function run(input: Parameters<typeof setOpsGeo>[1]) {
    if (!occ.id) return;
    setBusy(true);
    setError(null);
    try {
      const r = await setOpsGeo(occ.id, input);
      onChange?.(r.occurrence);
      if (r.occurrence.latitude == null) {
        setError("O endereço não foi localizado no mapa — informe o ponto ou corrija o endereço.");
      }
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gravar a coordenada");
    } finally {
      setBusy(false);
    }
  }

  const pill = located ? precisionPill(occ) : null;

  return (
    <>
      {!located && (
        <div className="banner banner-warn">
          ⚠ SEM COORDENADA — O ENDEREÇO NÃO FOI LOCALIZADO NO MAPA.
          {editable && " INFORME O PONTO ABAIXO, OU CORRIJA O ENDEREÇO E MANDE LOCALIZAR DE NOVO."}
        </div>
      )}
      <OpsFields edit={editable}>
        <OpsField label="LATITUDE" mono missing={!located} value={located ? occ.latitude!.toFixed(6) : undefined}>
          {editable && (
            <input
              className="ops-input mono"
              type="text"
              inputMode="decimal"
              value={lat}
              onChange={(e) => setLat(e.target.value)}
              placeholder="-3.731000"
              aria-label="Latitude"
              disabled={busy}
            />
          )}
        </OpsField>
        <OpsField label="LONGITUDE" mono missing={!located} value={located ? occ.longitude!.toFixed(6) : undefined}>
          {editable && (
            <input
              className="ops-input mono"
              type="text"
              inputMode="decimal"
              value={lng}
              onChange={(e) => setLng(e.target.value)}
              placeholder="-38.526000"
              aria-label="Longitude"
              disabled={busy}
            />
          )}
        </OpsField>
        <OpsField
          label="PRECISÃO DO PONTO"
          span={2}
          value={
            pill && (
              <span className={"pill " + pill.cls} title={pill.title}>
                {pill.label}
              </span>
            )
          }
        />
      </OpsFields>
      {(editable || located) && (
        <OpsActions>
          {editable && (
            <>
              <button
                type="button"
                className="btn btn-primary btn-sm"
                disabled={busy || !valid || !dirty}
                onClick={() => run({ latitude: latN, longitude: lngN })}
                title="Grava o ponto informado; ele passa a valer sobre o automático"
              >
                <Save size={12} strokeWidth={2} /> GRAVAR PONTO
              </button>
              <button
                type="button"
                className="btn btn-sm"
                disabled={busy}
                onClick={() => run({ reset: true })}
                title="Procura o endereço da ficha no mapa da agência e substitui o ponto atual"
              >
                <Crosshair size={12} strokeWidth={2} /> {busy ? "LOCALIZANDO…" : "LOCALIZAR PELO ENDEREÇO"}
              </button>
            </>
          )}
          {located && (
            <a
              className="btn btn-ghost btn-sm"
              href={googleMapsURL(occ.latitude!, occ.longitude!)}
              target="_blank"
              rel="noopener noreferrer"
            >
              <MapPin size={12} strokeWidth={1.8} /> ABRIR NO GOOGLE MAPS
            </a>
          )}
        </OpsActions>
      )}
      {error && <div className="banner banner-error">⚠ {error}</div>}
    </>
  );
}
