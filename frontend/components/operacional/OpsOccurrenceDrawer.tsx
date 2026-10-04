"use client";

import { useCallback, useEffect, useState } from "react";
import { FileText, Link2, Pencil, Search, Unlink, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  getOpsOccurrence,
  linkOpsPerson,
  opsReportFileURL,
  setOpsIntel,
  unlinkOpsPerson,
  type OpsOccurrence,
  type OpsPerson,
} from "@/lib/ops-reports-api";
import { listEntities } from "@/lib/entities-api";
import type { Entity, PersonAttrs } from "@/lib/entities-types";
import { canLinkOpsPeople, canSetOpsIntel } from "@/lib/permissions";
import { formatBR, formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import EntidadeDrawer from "../entidades/EntidadeDrawer";
import EditOpsOccurrenceModal from "./EditOpsOccurrenceModal";
import OpsOccurrenceBody from "./OpsOccurrenceBody";
import { IntelPill, IntelSummary } from "./IntelStatus";

type Props = {
  occurrenceId: string;
  onClose: () => void;
  onChanged: () => void;
};

/**
 * Ocorrência importada do relatório operacional. Nasce como cópia do PDF; o
 * analista liga as pessoas aos dossiês (ou desfaz um vínculo automático
 * errado), corrige a marcação de participação da inteligência e conserta o
 * que o relatório trouxe errado (CORRIGIR) — data, hora, ficha, local,
 * histórico. Cada correção vai para a auditoria com o antes e o depois, e o
 * PDF original segue disponível para conferência.
 */
export default function OpsOccurrenceDrawer({ occurrenceId, onClose, onChanged }: Props) {
  const { user: me } = useAuth();
  const canLink = canLinkOpsPeople(me);
  const canIntel = canSetOpsIntel(me);
  const [busyIntel, setBusyIntel] = useState(false);
  const [data, setData] = useState<OpsOccurrence | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyPerson, setBusyPerson] = useState<string | null>(null);
  const [searchFor, setSearchFor] = useState<string | null>(null);
  const [entityOverlayId, setEntityOverlayId] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const r = await getOpsOccurrence(occurrenceId);
      setData(r.occurrence);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [occurrenceId]);

  useEffect(() => {
    reload();
  }, [reload]);

  async function link(p: OpsPerson, entityId: string) {
    if (!data?.id || !p.id) return;
    setBusyPerson(p.id);
    setError(null);
    try {
      await linkOpsPerson(data.id, p.id, entityId);
      setSearchFor(null);
      await reload();
      onChanged();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao vincular");
    } finally {
      setBusyPerson(null);
    }
  }

  async function unlink(p: OpsPerson) {
    if (!data?.id || !p.id) return;
    setBusyPerson(p.id);
    setError(null);
    try {
      await unlinkOpsPerson(data.id, p.id);
      await reload();
      onChanged();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao desvincular");
    } finally {
      setBusyPerson(null);
    }
  }

  // A decisão do analista vira modo manual: reaplicar os termos depois não
  // desfaz a correção.
  async function setIntel(value: boolean) {
    if (!data?.id || busyIntel || (value === data.intel_participation && data.intel_mode === "manual")) return;
    setBusyIntel(true);
    setError(null);
    try {
      const r = await setOpsIntel(data.id, value);
      setData({ ...data, ...r });
      onChanged();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gravar a participação da inteligência");
    } finally {
      setBusyIntel(false);
    }
  }

  function renderPerson(p: OpsPerson) {
    const busy = busyPerson === p.id;
    if (p.entity_id) {
      return (
        <div className="ops-person-link">
          <button
            type="button"
            className="ops-link ops-link--auto ops-link--btn"
            onClick={() => setEntityOverlayId(p.entity_id)}
            title="Abrir dossiê"
          >
            <Link2 size={12} /> {p.entity_name || "DOSSIÊ"}
          </button>
          {p.entity_deceased && <span className="pill deceased">ÓBITO</span>}
          <span className={"pill " + (p.link_mode === "auto" ? "info" : "cold")}>
            {p.link_mode === "auto" ? "AUTOMÁTICO" : "MANUAL"}
          </span>
          {canLink && (
            <button
              type="button"
              className="action-btn"
              aria-label="Desvincular"
              title="Desvincular"
              disabled={busy}
              onClick={() => unlink(p)}
            >
              <Unlink size={12} />
            </button>
          )}
        </div>
      );
    }
    return (
      <div className="ops-person-link ops-person-link--col">
        {p.link_warning && <span className="ops-link ops-link--warn">{p.link_warning}</span>}
        {canLink &&
          p.matches.map((m) => (
            <button
              key={m.id}
              type="button"
              className="btn btn-sm"
              disabled={busy}
              onClick={() => link(p, m.id)}
              title={m.matched_fields.includes("mother_name") ? "Nome e mãe iguais" : "Só o nome é igual"}
            >
              <Link2 size={12} /> {m.name}
              {m.mother_name && <span className="muted"> · MÃE {m.mother_name}</span>}
            </button>
          ))}
        {canLink &&
          (searchFor === p.id ? (
            <PersonSearch initial={p.name} disabled={busy} onPick={(id) => link(p, id)} onCancel={() => setSearchFor(null)} />
          ) : (
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => setSearchFor(p.id ?? null)}>
              <Search size={12} /> BUSCAR DOSSIÊ
            </button>
          ))}
        {!canLink && p.matches.length === 0 && <span className="ops-link muted">SEM DOSSIÊ</span>}
      </div>
    );
  }

  return (
    <>
      <div className="drawer-backdrop" onClick={onClose}>
        <aside className="drawer drawer--wide" onClick={(e) => e.stopPropagation()} aria-label="Ocorrência do relatório operacional">
          <div className="drawer-hd">
            <span>OCORRÊNCIA · RELATÓRIO OPERACIONAL</span>
            <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
              <X size={14} />
            </button>
          </div>

          <div className="drawer-bd">
            {loading && !data && <div className="muted">// CARREGANDO…</div>}
            {error && <div className="banner banner-error">⚠ {error}</div>}

            {data && (
              <>
                <div className="dossier-head dossier-head--actions">
                  <div className="ops-natures">
                    {data.intel_participation && <IntelPill />}
                    {data.natures.map((n) => (
                      <span key={n} className="pill hold">
                        {n}
                      </span>
                    ))}
                  </div>
                  <div style={{ display: "flex", gap: 6, flexShrink: 0 }}>
                    {canLink && data.id && (
                      <button type="button" className="btn ops-nowrap" onClick={() => setEditing(true)}>
                        <Pencil size={13} strokeWidth={1.8} /> CORRIGIR
                      </button>
                    )}
                    {data.report_id && (
                      <a className="btn ops-nowrap" href={opsReportFileURL(data.report_id)} target="_blank" rel="noreferrer">
                        <FileText size={13} strokeWidth={1.8} /> PDF ORIGINAL · PÁG. {data.page}
                      </a>
                    )}
                  </div>
                </div>

                <OpsOccurrenceBody
                  occ={data}
                  renderPerson={renderPerson}
                  onSipomChange={
                    canIntel
                      ? (sipom, officers) => {
                          setData({ ...data, sipom, officers: officers ?? data.officers });
                          onChanged();
                        }
                      : undefined
                  }
                  onGeoChange={
                    canLink
                      ? (occ) => {
                          setData(occ);
                          onChanged();
                        }
                      : undefined
                  }
                  intelSlot={
                    <div className="intel-control">
                      {canIntel && (
                        <div className="seg-row intel-seg" role="radiogroup" aria-label="Participação da inteligência">
                          {[true, false].map((v) => (
                            <button
                              key={String(v)}
                              type="button"
                              role="radio"
                              aria-checked={data.intel_participation === v}
                              className={"seg-btn" + (data.intel_participation === v ? " seg-btn--on" : "")}
                              disabled={busyIntel}
                              onClick={() => setIntel(v)}
                            >
                              {v ? "SIM" : "NÃO"}
                            </button>
                          ))}
                        </div>
                      )}
                      <IntelSummary
                        value={data.intel_participation}
                        mode={data.intel_mode}
                        matched={data.intel_matched}
                      />
                    </div>
                  }
                />

                <dl className="dossier-list" style={{ marginTop: 14 }}>
                  <div>
                    <dt>DATA DO FATO</dt>
                    <dd>{formatBRDate(data.occurred_on)}</dd>
                  </div>
                  {data.created_at && (
                    <div>
                      <dt>IMPORTADO EM</dt>
                      <dd>{formatBR(data.created_at)}</dd>
                    </div>
                  )}
                  {data.updated_at && (
                    <div>
                      <dt>CORRIGIDO EM</dt>
                      <dd>
                        {formatBR(data.updated_at)}
                        {data.updated_by_name && ` · ${data.updated_by_name.toUpperCase()}`}
                      </dd>
                    </div>
                  )}
                </dl>
              </>
            )}
          </div>
        </aside>
      </div>

      {entityOverlayId && (
        <EntidadeDrawer
          entityId={entityOverlayId}
          onClose={() => setEntityOverlayId(null)}
          onChanged={reload}
          onOpenEntity={(id) => setEntityOverlayId(id)}
        />
      )}
      {editing && data && (
        <EditOpsOccurrenceModal
          occ={data}
          onClose={() => setEditing(false)}
          onSaved={(occ) => {
            setEditing(false);
            setData(occ);
            onChanged();
          }}
        />
      )}
    </>
  );
}

// Busca de dossiê de pessoa para o vínculo manual — parte do nome como veio
// no relatório, que o analista ajusta se o dossiê estiver grafado diferente.
function PersonSearch({
  initial,
  disabled,
  onPick,
  onCancel,
}: {
  initial: string;
  disabled: boolean;
  onPick: (id: string) => void;
  onCancel: () => void;
}) {
  const [q, setQ] = useState(initial);
  const [results, setResults] = useState<Entity[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const term = q.trim();
    if (term.length < 3) {
      setResults([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    const h = window.setTimeout(async () => {
      try {
        const r = await listEntities({ search: term, kind: "person", limit: 8 });
        if (!cancelled) setResults(r.items || []);
      } catch {
        if (!cancelled) setResults([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(h);
    };
  }, [q]);

  return (
    <div className="ops-person-search">
      <div className="toolbar-search">
        <Search size={13} strokeWidth={1.6} />
        <input type="text" value={q} autoFocus onChange={(e) => setQ(e.target.value)} placeholder="nome, alcunha ou CPF…" />
        <button type="button" className="action-btn" onClick={onCancel} aria-label="Cancelar busca">
          <X size={12} />
        </button>
      </div>
      {loading && <div className="muted" style={{ fontSize: 12 }}>// BUSCANDO…</div>}
      {!loading && q.trim().length >= 3 && results.length === 0 && (
        <div className="muted" style={{ fontSize: 12 }}>// NENHUM DOSSIÊ ENCONTRADO</div>
      )}
      {results.map((e) => {
        const mother = (e.attrs as PersonAttrs | undefined)?.mother_name;
        return (
          <button key={e.id} type="button" className="ops-search-hit" disabled={disabled} onClick={() => onPick(e.id)}>
            <span>{e.name.toUpperCase()}</span>
            {mother && <span className="muted"> · MÃE {mother.toUpperCase()}</span>}
          </button>
        );
      })}
    </div>
  );
}
