"use client";

import { useCallback, useEffect, useState } from "react";
import { Check, MapPinned, Trash2, X } from "lucide-react";
import {
  deleteSipomAreaRef,
  getSipomAreaRefCity,
  listSipomAreaRefCities,
  listSipomAreaRefs,
  setSipomAreaRef,
  type SipomAreaPending,
  type SipomAreaRef,
} from "@/lib/sipom-api";
import type { SipomRef } from "@/lib/ops-reports-api";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import Select from "../shared/Select";

type Props = {
  onClose: () => void;
  /** Alguma referência mudou: a fila precisa recarregar. */
  onChanged: () => void;
};

const placeLabel = (city: string, neighborhood: string, cityWide: boolean) =>
  cityWide ? `${city} · CIDADE INTEIRA` : `${city} · ${neighborhood || "SEM BAIRRO"}`;

/**
 * Referências de área da unidade militar: cidade (+ bairro) → companhia, que
 * a tradução usa antes do catálogo do SIPOM. Onde o catálogo distingue a área
 * por bairro (Fortaleza, Caucaia) a referência é do bairro; nas demais
 * cidades vale para a cidade inteira. O analista resolve de uma vez os
 * lugares que ainda pedem área na fila, insere referências novas e apaga as
 * erradas — toda mudança recalcula a fila.
 */
export default function SipomAreaRefsModal({ onClose, onChanged }: Props) {
  const [items, setItems] = useState<SipomAreaRef[] | null>(null);
  const [pending, setPending] = useState<SipomAreaPending[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [changed, setChanged] = useState(false);

  const reload = useCallback(async () => {
    try {
      const r = await listSipomAreaRefs();
      setItems(r.items);
      setPending(r.pending);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar as referências");
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  async function save(city: string, neighborhood: string, areaId: number) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const r = await setSipomAreaRef(city, neighborhood, areaId);
      setNotice(
        `REFERÊNCIA GRAVADA: ${placeLabel(city, neighborhood, r.city_wide)}` +
          (r.resolved > 0 ? ` · ${r.resolved} OCORRÊNCIA${r.resolved > 1 ? "S RESOLVIDAS" : " RESOLVIDA"}` : ""),
      );
      setChanged(true);
      await reload();
      return true;
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gravar a referência");
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function remove(ref: SipomAreaRef) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await deleteSipomAreaRef(ref.city, ref.neighborhood);
      setNotice(`REFERÊNCIA APAGADA: ${placeLabel(ref.city, ref.neighborhood, ref.city_wide)}`);
      setChanged(true);
      await reload();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao apagar a referência");
    } finally {
      setBusy(false);
    }
  }

  const close = () => {
    if (busy) return;
    if (changed) onChanged();
    onClose();
  };

  return (
    <div className="modal-backdrop" onClick={close}>
      <div className="modal sipom-areas" onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span>
            <MapPinned size={13} strokeWidth={1.8} /> REFERÊNCIAS DE ÁREA DA UNIDADE MILITAR
          </span>
          <button type="button" className="action-btn" onClick={close} aria-label="Fechar" disabled={busy}>
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd">
          <div className="muted">
            ONDE O CATÁLOGO DO SIPOM DISTINGUE A ÁREA POR BAIRRO (FORTALEZA, CAUCAIA), A REFERÊNCIA É DO BAIRRO; NAS DEMAIS
            CIDADES, VALE PARA A CIDADE INTEIRA. TODA MUDANÇA RECALCULA A FILA.
          </div>
          {error && <div className="banner banner-error">⚠ {error}</div>}
          {notice && <div className="banner banner-info">✓ {notice}</div>}

          <div className="ops-sub">LUGARES DA FILA SEM ÁREA ({pending.length})</div>
          {pending.length === 0 ? (
            <div className="ops-empty">NENHUMA OCORRÊNCIA DA FILA PEDE ÁREA</div>
          ) : (
            <table className="tbl ops-subtbl">
              <thead>
                <tr>
                  <th>LUGAR</th>
                  <th style={{ width: 90 }}>FICHAS</th>
                  <th style={{ width: 300 }}>ÁREA</th>
                </tr>
              </thead>
              <tbody>
                {pending.map((p) => (
                  <PendingRow key={p.city + "\u0000" + p.neighborhood} p={p} busy={busy} onSave={save} />
                ))}
              </tbody>
            </table>
          )}

          <div className="ops-sub">NOVA REFERÊNCIA</div>
          <NewRef busy={busy} onSave={save} />

          <div className="ops-sub">REFERÊNCIAS GRAVADAS ({items?.length ?? 0})</div>
          {items === null ? (
            <div className="muted">// CARREGANDO…</div>
          ) : items.length === 0 ? (
            <div className="ops-empty">NENHUMA REFERÊNCIA GRAVADA</div>
          ) : (
            <table className="tbl ops-subtbl">
              <thead>
                <tr>
                  <th>LUGAR</th>
                  <th>ÁREA</th>
                  <th>ÚLTIMA ALTERAÇÃO</th>
                  <th style={{ width: 40 }} />
                </tr>
              </thead>
              <tbody>
                {items.map((r) => (
                  <RefRow key={r.city + "\u0000" + r.neighborhood} r={r} busy={busy} onRemove={remove} />
                ))}
              </tbody>
            </table>
          )}
        </div>
        <div className="modal-ft">
          <button type="button" className="btn btn-primary" onClick={close} disabled={busy}>
            FECHAR
          </button>
        </div>
      </div>
    </div>
  );
}

function PendingRow({
  p,
  busy,
  onSave,
}: {
  p: SipomAreaPending;
  busy: boolean;
  onSave: (city: string, neighborhood: string, areaId: number) => Promise<boolean>;
}) {
  const [area, setArea] = useState("");
  return (
    <tr>
      <td className="ops-strong">{placeLabel(p.city, p.neighborhood, p.city_wide)}</td>
      <td className="mono" title={p.fichas.join("\n")}>
        {p.fichas.length}
      </td>
      <td>
        {p.cidade_id === null ? (
          <span className="muted">CIDADE FORA DO CATÁLOGO</span>
        ) : (
          <div className="ops-control-row">
            <Select
              value={area}
              placeholder={p.areas.length ? "ESCOLHA A ÁREA" : "SEM ÁREA NO CATÁLOGO"}
              disabled={busy || p.areas.length === 0}
              onChange={setArea}
              options={p.areas.map((a) => ({ value: String(a.id), label: a.nome }))}
            />
            <button
              type="button"
              className="btn btn-primary btn-sm ops-nowrap"
              disabled={busy || !area}
              onClick={() => onSave(p.city, p.neighborhood, Number(area))}
            >
              <Check size={12} strokeWidth={2} /> GRAVAR
            </button>
          </div>
        )}
      </td>
    </tr>
  );
}

function RefRow({ r, busy, onRemove }: { r: SipomAreaRef; busy: boolean; onRemove: (r: SipomAreaRef) => void }) {
  const [confirm, setConfirm] = useState(false);
  return (
    <tr>
      <td className="ops-strong">{placeLabel(r.city, r.neighborhood, r.city_wide)}</td>
      <td>{r.area?.nome ?? "—"}</td>
      <td className="muted">
        {r.updated_by_name || "—"} · {formatBRDate(r.updated_at.slice(0, 10))}
      </td>
      <td>
        {confirm ? (
          <div className="ops-control-row">
            <button type="button" className="btn btn-danger btn-sm ops-nowrap" disabled={busy} onClick={() => onRemove(r)}>
              APAGAR
            </button>
            <button type="button" className="btn btn-ghost btn-sm" disabled={busy} onClick={() => setConfirm(false)}>
              NÃO
            </button>
          </div>
        ) : (
          <button
            type="button"
            className="action-btn"
            disabled={busy}
            title="Apagar a referência"
            aria-label={`Apagar a referência de ${placeLabel(r.city, r.neighborhood, r.city_wide)}`}
            onClick={() => setConfirm(true)}
          >
            <Trash2 size={13} />
          </button>
        )}
      </td>
    </tr>
  );
}

/** Referência nova: cidade do catálogo, bairro (onde a área é por bairro) e área. */
function NewRef({
  busy,
  onSave,
}: {
  busy: boolean;
  onSave: (city: string, neighborhood: string, areaId: number) => Promise<boolean>;
}) {
  const [cities, setCities] = useState<SipomRef[]>([]);
  const [cityID, setCityID] = useState("");
  const [info, setInfo] = useState<{ city_wide: boolean; bairros: SipomRef[]; areas: SipomRef[] } | null>(null);
  const [bairro, setBairro] = useState("");
  const [area, setArea] = useState("");

  useEffect(() => {
    listSipomAreaRefCities()
      .then((r) => setCities(r.items))
      .catch(() => setCities([]));
  }, []);

  useEffect(() => {
    setInfo(null);
    setBairro("");
    setArea("");
    if (!cityID) return;
    getSipomAreaRefCity(Number(cityID))
      .then(setInfo)
      .catch(() => setInfo(null));
  }, [cityID]);

  const city = cities.find((c) => String(c.id) === cityID);
  const needsBairro = !!info && !info.city_wide;
  const bairroName = info?.bairros.find((b) => String(b.id) === bairro)?.nome ?? "";
  const ready = !!city && !!area && (!needsBairro || !!bairro);

  return (
    <div className="ops-control-row sipom-areas-new">
      <Select
        searchable
        value={cityID}
        placeholder="CIDADE"
        disabled={busy || cities.length === 0}
        onChange={setCityID}
        options={cities.map((c) => ({ value: String(c.id), label: c.nome.toUpperCase() }))}
      />
      {info &&
        (needsBairro ? (
          <Select
            searchable
            value={bairro}
            placeholder="BAIRRO"
            disabled={busy}
            onChange={setBairro}
            options={info.bairros.map((b) => ({ value: String(b.id), label: b.nome.toUpperCase() }))}
          />
        ) : (
          <span className="pill cold ops-nowrap">CIDADE INTEIRA</span>
        ))}
      {info && (
        <Select
          value={area}
          placeholder={info.areas.length ? "ÁREA" : "SEM ÁREA NO CATÁLOGO"}
          disabled={busy || info.areas.length === 0}
          onChange={setArea}
          options={info.areas.map((a) => ({ value: String(a.id), label: a.nome }))}
        />
      )}
      <button
        type="button"
        className="btn btn-primary btn-sm ops-nowrap"
        disabled={busy || !ready}
        onClick={async () => {
          if (city && (await onSave(city.nome, needsBairro ? bairroName : "", Number(area)))) {
            setBairro("");
            setArea("");
          }
        }}
      >
        <Check size={12} strokeWidth={2} /> GRAVAR
      </button>
    </div>
  );
}
