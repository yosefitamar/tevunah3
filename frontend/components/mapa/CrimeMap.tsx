"use client";

import { useEffect, useMemo, useState } from "react";
import { MapContainer, TileLayer, CircleMarker, Popup, useMap } from "react-leaflet";
import type { LatLngBoundsExpression } from "leaflet";
import "leaflet/dist/leaflet.css";
import {
  INCIDENT_MEANS_LABEL,
  INCIDENT_TYPE_LABEL,
  incidentPhotoURL,
  type Incident,
  type IncidentMeans,
} from "@/lib/incidents-api";
import { occurrenceTitle, type OccurrenceRow } from "@/lib/occurrences-api";
import { photoURL } from "@/lib/entities-api";
import DeceasedPhoto from "../shared/DeceasedPhoto";
import { formatBRDate } from "@/lib/format";

// Centro de fallback quando o recorte não tem nenhum ponto: Fortaleza/CE.
const FALLBACK_CENTER: [number, number] = [-3.7319, -38.5267];
const FALLBACK_ZOOM = 11;

// Agrupamento dos envolvidos no popup, na ordem em que o analista lê o caso.
// Além dos três papéis atuais, cada grupo aceita os equivalentes anteriores à
// lista fechada, para que registros antigos não caiam todos em "OUTROS".
const INVOLVED_GROUPS: { title: string; roles: string[]; tone: string }[] = [
  { title: "VÍTIMAS", roles: ["VÍTIMA", "VITIMA"], tone: "crit" },
  {
    title: "ACUSADOS",
    roles: ["ACUSADO", "SUSPEITO", "AUTOR", "PRESO", "POSSÍVEL SUSPEITO", "POSSIVEL SUSPEITO"],
    tone: "warn",
  },
  { title: "TESTEMUNHAS", roles: ["TESTEMUNHA"], tone: "info" },
];

type Props = {
  items: Incident[];
  // Cor resolvida (hex/rgb) por meio utilizado — vem da paleta ativa.
  colorFor: (means: IncidentMeans) => string;
  /** Escurece as tiles do OSM para casar com o tema do app. */
  dark: boolean;
  /** Ocorrência a centralizar (chegada pelo link da ficha de uma vítima). */
  focusId?: string | null;
  onOpen: (id: string) => void;
  /**
   * Camada de produtividade: as ocorrências que não são homicídio (prisões,
   * apreensões e o que veio do relatório operacional), já com coordenada.
   */
  productivity?: OccurrenceRow[];
  /** Cor resolvida dos pontos da camada de produtividade. */
  productivityColor?: string;
  onOpenProductivity?: (row: OccurrenceRow) => void;
};

// Ponto plotável, de qualquer camada — o que o enquadramento precisa saber.
type Plotted = { id: string; lat: number; lng: number };

const rowKey = (r: OccurrenceRow) => (r.incident_id ?? "") + ":" + (r.ops_id ?? "");

/**
 * Mapa do crime — pontos das ocorrências georreferenciadas do recorte.
 *
 * Usa CircleMarker (SVG) em vez de Marker com ícone de imagem: colore por
 * meio utilizado sem assets e não esbarra no problema clássico de ícone
 * quebrado do Leaflet sob bundler.
 *
 * Este componente toca `window` no import do Leaflet — só é carregado via
 * dynamic(ssr:false) pelo MapaScreen.
 */
export default function CrimeMap({
  items,
  colorFor,
  dark,
  focusId,
  onOpen,
  productivity,
  productivityColor = "#888888",
  onOpenProductivity,
}: Props) {
  const extra = useMemo(
    () =>
      (productivity ?? []).filter(
        (r) =>
          typeof r.latitude === "number" &&
          typeof r.longitude === "number" &&
          Number.isFinite(r.latitude) &&
          Number.isFinite(r.longitude),
      ),
    [productivity],
  );
  const points = useMemo(
    () =>
      items.filter(
        (i) =>
          typeof i.latitude === "number" &&
          typeof i.longitude === "number" &&
          Number.isFinite(i.latitude) &&
          Number.isFinite(i.longitude),
      ),
    [items],
  );

  return (
    // O modo escuro vive no wrapper, não no MapContainer: o react-leaflet só
    // lê className na montagem e ignora mudanças depois — no container o
    // toggle não teria efeito nenhum.
    <div className={"crime-map-wrap" + (dark ? " crime-map-wrap--dark" : "")}>
      <MapContainer
        center={FALLBACK_CENTER}
        zoom={FALLBACK_ZOOM}
        scrollWheelZoom
        className="crime-map"
        preferCanvas={false}
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          maxZoom={19}
        />
        <FitToPoints
          points={[
            ...points.map((p) => ({ id: p.id, lat: p.latitude as number, lng: p.longitude as number })),
            // Com cadastro manual o id é o dele: é por ele que o foco chega.
            ...extra.map((r) => ({
              id: r.incident_id ?? rowKey(r),
              lat: r.latitude as number,
              lng: r.longitude as number,
            })),
          ]}
          focusId={focusId}
        />
        <KeepSized />
        {/* Produtividade por baixo: desenhada antes, o homicídio fica por cima
            quando os dois caem no mesmo lugar. Ponto aproximado (só o bairro
            foi localizado) vem tracejado e mais apagado. */}
        {extra.map((r) => {
          const approx = r.geo_precision === "bairro";
          return (
            <CircleMarker
              key={rowKey(r)}
              center={[r.latitude as number, r.longitude as number]}
              radius={5}
              pathOptions={{
                color: productivityColor,
                fillColor: productivityColor,
                fillOpacity: approx ? 0.15 : 0.45,
                weight: 1.5,
                dashArray: approx ? "3 3" : undefined,
              }}
            >
              <Popup maxWidth={320} minWidth={260}>
                <ProductivityPopup row={r} onOpen={onOpenProductivity} />
              </Popup>
            </CircleMarker>
          );
        })}
        {points.map((inc) => (
          <CircleMarker
            key={inc.id}
            center={[inc.latitude as number, inc.longitude as number]}
            radius={inc.id === focusId ? 11 : 7}
            pathOptions={{
              color: colorFor(inc.means),
              fillColor: colorFor(inc.means),
              fillOpacity: inc.id === focusId ? 0.85 : 0.55,
              weight: inc.id === focusId ? 4 : 2,
            }}
          >
            <Popup maxWidth={340} minWidth={280}>
              <IncidentPopup incident={inc} onOpen={onOpen} />
            </Popup>
          </CircleMarker>
        ))}
      </MapContainer>
    </div>
  );
}

/**
 * Ajusta o enquadramento aos pontos do recorte. Roda a cada troca de
 * conjunto (mudou filtro/período), mas não a cada pan/zoom do usuário.
 */
function FitToPoints({
  points,
  focusId,
}: {
  points: Plotted[];
  focusId?: string | null;
}) {
  const map = useMap();
  // Assinatura estável do conjunto: só refaz o fit quando os pontos mudam.
  const signature = points.map((p) => p.id).join(",");

  useEffect(() => {
    // Com um ponto em foco, o enquadramento é nele — não no conjunto: quem
    // veio da ficha da vítima quer ver aquele fato, não a distribuição.
    const focus = focusId ? points.find((p) => p.id === focusId) : undefined;
    if (focus) {
      map.setView([focus.lat, focus.lng], 17);
      return;
    }
    if (points.length === 0) {
      map.setView(FALLBACK_CENTER, FALLBACK_ZOOM);
      return;
    }
    const bounds: LatLngBoundsExpression = points.map((p) => [p.lat, p.lng] as [number, number]);
    map.fitBounds(bounds, { padding: [40, 40], maxZoom: 16 });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signature, map, focusId]);

  return null;
}

/**
 * O Leaflet mede o container uma vez, na inicialização, e passa a desenhar
 * tiles para aquele tamanho — se o painel muda depois (sidebar colapsando,
 * janela redimensionada, o próprio mapa montando antes do layout assentar),
 * sobram áreas cinzas sem tile. O ResizeObserver devolve a medida certa.
 */
function KeepSized() {
  const map = useMap();
  useEffect(() => {
    const el = map.getContainer();
    const ro = new ResizeObserver(() => map.invalidateSize());
    ro.observe(el);
    // Primeira medida após o layout assentar.
    const t = window.setTimeout(() => map.invalidateSize(), 0);
    return () => {
      ro.disconnect();
      window.clearTimeout(t);
    };
  }, [map]);
  return null;
}

// ─── Popup ────────────────────────────────────────────────────────────

/** Popup da camada de produtividade: o resumo da linha e o atalho à ficha. */
function ProductivityPopup({ row, onOpen }: { row: OccurrenceRow; onOpen?: (row: OccurrenceRow) => void }) {
  const desc = row.description.trim();
  return (
    <div className="map-popup">
      <div className="map-popup-hd">
        <span className="map-popup-type">{occurrenceTitle(row)}</span>
        <span className="map-popup-when">
          {formatBRDate(row.occurred_on)}
          {row.time ? ` · ${row.time}` : ""}
        </span>
      </div>
      <dl className="map-popup-dl">
        <div>
          <dt>FICHA CIOPS</dt>
          <dd>{row.ciops_record || "—"}</dd>
        </div>
        <div>
          <dt>MUNICÍPIO</dt>
          <dd>{row.city || "—"}</dd>
        </div>
        <div>
          <dt>BAIRRO</dt>
          <dd>{row.neighborhood || "—"}</dd>
        </div>
        <div>
          <dt>COORDENADAS</dt>
          <dd>
            {row.latitude?.toFixed(6)}, {row.longitude?.toFixed(6)}
            {row.geo_precision === "bairro" && " · APROXIMADA (CENTRO DO BAIRRO)"}
            {row.geo_precision === "rua" && " · RUA, SEM O NÚMERO"}
          </dd>
        </div>
      </dl>
      {desc && <div className="map-popup-desc">{desc.length > 280 ? desc.slice(0, 280) + "…" : desc}</div>}
      {onOpen && (
        <button type="button" className="btn btn-primary map-popup-btn" onClick={() => onOpen(row)}>
          ABRIR OCORRÊNCIA
        </button>
      )}
    </div>
  );
}

function IncidentPopup({
  incident,
  onOpen,
}: {
  incident: Incident;
  onOpen: (id: string) => void;
}) {
  const [photoFailed, setPhotoFailed] = useState(false);

  // Agrupa por papel; o que não casa com nenhum grupo (papéis legados) cai
  // em OUTROS, para nunca sumir da tela.
  const groups = useMemo(() => {
    const norm = (r: string) => r.trim().toUpperCase();
    const used = new Set<string>();
    const out = INVOLVED_GROUPS.map((g) => {
      const items = incident.involved.filter((e) => g.roles.includes(norm(e.role)));
      items.forEach((e) => used.add(e.entity_id));
      return { ...g, items };
    }).filter((g) => g.items.length > 0);

    const rest = incident.involved.filter((e) => !used.has(e.entity_id));
    if (rest.length > 0) out.push({ title: "OUTROS", roles: [], tone: "cold", items: rest });
    return out;
  }, [incident.involved]);

  return (
    <div className="map-popup">
      <div className="map-popup-hd">
        <span className="map-popup-type">{INCIDENT_TYPE_LABEL[incident.type]}</span>
        <span className="map-popup-when">
          {formatBRDate(incident.occurred_on)}
          {incident.occurred_time ? ` · ${incident.occurred_time}` : ""}
        </span>
      </div>

      {incident.has_photo && !photoFailed && (
        <img
          className="map-popup-photo"
          src={incidentPhotoURL(incident.id, incident.updated_at)}
          alt="foto da ocorrência"
          onError={() => setPhotoFailed(true)}
        />
      )}

      <dl className="map-popup-dl">
        <div>
          <dt>MEIO</dt>
          <dd>
            {INCIDENT_MEANS_LABEL[incident.means]}
            {incident.means_detail ? ` — ${incident.means_detail}` : ""}
          </dd>
        </div>
        <div>
          <dt>FICHA CIOPS</dt>
          <dd>{incident.ciops_record || "—"}</dd>
        </div>
        <div>
          <dt>MUNICÍPIO</dt>
          <dd>{incident.city || "—"}</dd>
        </div>
        <div>
          <dt>BAIRRO</dt>
          <dd>{incident.neighborhood || "—"}</dd>
        </div>
        <div>
          <dt>COORDENADAS</dt>
          <dd>
            {incident.latitude?.toFixed(6)}, {incident.longitude?.toFixed(6)}
          </dd>
        </div>
      </dl>

      {incident.description.trim() && (
        <div className="map-popup-desc">{incident.description}</div>
      )}

      {groups.map((g) => (
        <div className="map-popup-block" key={g.title}>
          <div className="map-popup-block-ttl">
            {g.title} ({g.items.length})
          </div>
          {g.items.map((e) => (
            <InvolvedLine key={e.entity_id} entity={e} tone={g.tone} />
          ))}
        </div>
      ))}

      <button
        type="button"
        className="btn btn-primary map-popup-btn"
        onClick={() => onOpen(incident.id)}
      >
        ABRIR DOSSIÊ COMPLETO
      </button>
    </div>
  );
}

function InvolvedLine({
  entity,
  tone,
}: {
  entity: Incident["involved"][number];
  tone: string;
}) {
  const [failed, setFailed] = useState(false);
  return (
    <div className="map-popup-inv">
      {entity.has_photo && !failed ? (
        entity.deceased ? (
          <DeceasedPhoto
            src={photoURL(entity.entity_id, entity.version)}
            deceased
            compact
            className="map-popup-inv-thumb"
          />
        ) : (
          <img
            className="map-popup-inv-thumb"
            src={photoURL(entity.entity_id, entity.version)}
            alt=""
            aria-hidden
            onError={() => setFailed(true)}
          />
        )
      ) : (
        <span className="map-popup-inv-thumb map-popup-inv-thumb--empty" aria-hidden />
      )}
      <span className="map-popup-inv-name">{entity.name.toUpperCase()}</span>
      {entity.role && (
        <span className={"pill " + tone} style={{ fontSize: 11.5 }}>
          {entity.role.toUpperCase()}
        </span>
      )}
    </div>
  );
}
