"use client";

import { useState } from "react";
import { Crosshair, MapPin } from "lucide-react";
import { googleMapsURL } from "@/lib/incidents-api";

type Props = {
  lat: string;
  lng: string;
  onChange: (lat: string, lng: string) => void;
  disabled?: boolean;
};

/**
 * Campo de coordenadas: lat/long manuais + "usar minha localização"
 * (geolocalização do navegador, sem custo) + link "abrir no Google Maps".
 * Valores são strings pra permitir campo vazio; o consumidor converte.
 */
export default function GeoField({ lat, lng, onChange, disabled }: Props) {
  const [locating, setLocating] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const latN = Number(lat);
  const lngN = Number(lng);
  const valid =
    lat.trim() !== "" &&
    lng.trim() !== "" &&
    Number.isFinite(latN) &&
    Number.isFinite(lngN);

  function useMyLocation() {
    setErr(null);
    if (!navigator.geolocation) {
      setErr("Geolocalização indisponível neste navegador");
      return;
    }
    setLocating(true);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        onChange(pos.coords.latitude.toFixed(6), pos.coords.longitude.toFixed(6));
        setLocating(false);
      },
      (e) => {
        setErr(e.message || "Não foi possível obter a localização");
        setLocating(false);
      },
      { enableHighAccuracy: true, timeout: 10000 },
    );
  }

  return (
    <div className="form-field">
      <span>LOCALIZAÇÃO (LAT / LONG)</span>
      <div className="form-grid-2">
        <input
          type="text"
          inputMode="decimal"
          value={lat}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value, lng)}
          placeholder="-3.731000"
        />
        <input
          type="text"
          inputMode="decimal"
          value={lng}
          disabled={disabled}
          onChange={(e) => onChange(lat, e.target.value)}
          placeholder="-38.526000"
        />
      </div>
      <div style={{ display: "flex", gap: 8, marginTop: 6, flexWrap: "wrap" }}>
        {!disabled && (
          <button type="button" className="btn btn-ghost" onClick={useMyLocation} disabled={locating}>
            <Crosshair size={13} strokeWidth={1.8} />
            {locating ? " LOCALIZANDO…" : " USAR MINHA LOCALIZAÇÃO"}
          </button>
        )}
        {valid && (
          <a
            className="btn btn-ghost"
            href={googleMapsURL(latN, lngN)}
            target="_blank"
            rel="noopener noreferrer"
          >
            <MapPin size={13} strokeWidth={1.8} /> ABRIR NO GOOGLE MAPS
          </a>
        )}
      </div>
      {err && (
        <div className="muted" style={{ fontSize: 10, marginTop: 4, color: "var(--danger)" }}>
          ⚠ {err}
        </div>
      )}
    </div>
  );
}
