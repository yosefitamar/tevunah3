"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { CheckCheck, CheckCircle2, RefreshCw } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { confirmSipomNaturezas, getSipomQueue, type SipomQueue as Queue } from "@/lib/sipom-api";
import { canSetOpsIntel } from "@/lib/permissions";
import { ALL_PERIOD, periodBounds, type PeriodSelection } from "@/lib/period";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import PeriodButton from "../shared/PeriodButton";
import OpsOccurrenceDrawer from "./OpsOccurrenceDrawer";
import { IntelPill } from "./IntelStatus";

// Rótulo curto de cada pendência para a tabela (o longo fica no title).
const SHORT: Record<string, string> = {
  natureza: "SEM NATUREZA",
  natureza_confirmar: "CONFIRMAR NATUREZA",
  data_hora: "SEM HORA",
  ficha: "SEM FICHA",
  logradouro: "SEM LOGRADOURO",
  cidade: "CIDADE FORA DO CATÁLOGO",
  bairro: "SEM BAIRRO",
  area: "SEM ÁREA",
  area_ambigua: "ÁREA AMBÍGUA",
  opm: "SEM OPM",
  composicao: "COMPOSIÇÃO",
  composicao_multi: "DUAS EQUIPES",
  pessoa_sem_dossie: "PESSOA SEM DOSSIÊ",
  coordenada: "SEM COORDENADA",
  procedimento: "SEM PROCEDIMENTO",
  procedimento_tipo: "TIPO DO PROCED.",
  procedimento_numero: "Nº DO PROCED.",
  delegacia: "SEM DELEGACIA",
  delegado: "SEM DELEGADO",
  arma: "ARMA",
  droga: "DROGA",
  veiculo: "VEÍCULO",
};

type Status = "all" | "pending" | "ready";

/**
 * Fila de envio ao SIPOM: cada ocorrência do período com a situação no
 * destino — pronta, ou o que falta. Clicar abre a ficha, onde as correções
 * são feitas; as naturezas sugeridas pelo de-para podem ser confirmadas em
 * lote.
 */
export default function SipomQueue() {
  const { user: me } = useAuth();
  const canEdit = canSetOpsIntel(me);
  const [data, setData] = useState<Queue | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [period, setPeriod] = useState<PeriodSelection>(ALL_PERIOD);
  const [status, setStatus] = useState<Status>("pending");
  const [code, setCode] = useState<string>("");
  const [openId, setOpenId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const bounds = useMemo(() => periodBounds(period), [period]);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await getSipomQueue({ date_from: bounds.from || undefined, date_to: bounds.to || undefined }));
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar a fila");
    } finally {
      setLoading(false);
    }
  }, [bounds.from, bounds.to]);

  useEffect(() => {
    reload();
  }, [reload]);

  const items = useMemo(() => {
    let xs = data?.items ?? [];
    if (status === "ready") xs = xs.filter((x) => x.ready);
    if (status === "pending") xs = xs.filter((x) => !x.ready);
    if (code) xs = xs.filter((x) => x.pendencias.some((p) => p.code === code));
    return xs;
  }, [data, status, code]);

  const toConfirm = items.filter((x) => x.pendencias.some((p) => p.code === "natureza_confirmar"));

  async function confirmAll() {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const r = await confirmSipomNaturezas(toConfirm.map((x) => x.id));
      setNotice(`${r.confirmed} natureza(s) confirmada(s).`);
      await reload();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao confirmar");
    } finally {
      setBusy(false);
    }
  }

  const summary = data?.summary;
  const codes = Object.entries(summary?.by_code ?? {}).sort((a, b) => b[1] - a[1]);

  return (
    <>
      <div className="toolbar">
        <PeriodButton value={period} onChange={setPeriod} title="PERÍODO · DATA DO FATO" />
        <div className="tabs">
          {(
            [
              ["pending", "PENDENTES"],
              ["ready", "PRONTAS"],
              ["all", "TODAS"],
            ] as const
          ).map(([v, label]) => (
            <button
              key={v}
              type="button"
              className={"tab" + (status === v ? " tab-active" : "")}
              onClick={() => setStatus(v)}
            >
              {label}
              {summary && (
                <span className="muted">
                  {" "}
                  {v === "pending" ? summary.pending : v === "ready" ? summary.ready : summary.total}
                </span>
              )}
            </button>
          ))}
        </div>
        <div style={{ marginLeft: "auto" }} />
        {canEdit && toConfirm.length > 0 && (
          <button type="button" className="btn" disabled={busy} onClick={confirmAll} title="Fixa a natureza sugerida pelo de-para nas ocorrências listadas">
            <CheckCheck size={14} strokeWidth={1.8} /> CONFIRMAR NATUREZAS SUGERIDAS ({toConfirm.length})
          </button>
        )}
        <button type="button" className="btn btn-ghost" onClick={reload} disabled={loading}>
          <RefreshCw size={13} strokeWidth={1.8} /> {loading ? "CARREGANDO…" : "ATUALIZAR"}
        </button>
      </div>

      {codes.length > 0 && (
        <div className="sipom-codes">
          <span className="muted">MOTIVOS:</span>
          {codes.map(([c, n]) => (
            <button
              key={c}
              type="button"
              className={"chip" + (code === c ? " on" : "")}
              title={data?.labels[c]}
              onClick={() => setCode(code === c ? "" : c)}
            >
              {SHORT[c] ?? c} · {n}
            </button>
          ))}
        </div>
      )}

      {error && <div className="banner banner-error">⚠ {error}</div>}
      {notice && <div className="banner banner-info">{notice}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <th style={{ width: 110 }}>SITUAÇÃO</th>
                <th style={{ width: 130 }}>DATA</th>
                <th style={{ width: 130 }}>FICHA</th>
                <th>NATUREZA NO SIPOM</th>
                <th style={{ width: 200 }}>LOCAL</th>
                <th style={{ width: 130 }}>ÁREA</th>
                <th style={{ width: 130 }}>OPM</th>
                <th style={{ width: 90 }}>ENVOLV.</th>
                <th style={{ width: 260 }}>PENDÊNCIAS</th>
              </tr>
            </thead>
            <tbody>
              {loading && !data && (
                <tr>
                  <td colSpan={9} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // CARREGANDO…
                  </td>
                </tr>
              )}
              {!loading && items.length === 0 && (
                <tr>
                  <td colSpan={9} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    {status === "pending" && !code ? "// NENHUMA PENDÊNCIA NO PERÍODO" : "// NENHUMA OCORRÊNCIA"}
                  </td>
                </tr>
              )}
              {items.map((x) => (
                <tr key={x.id} className="row-clickable" onClick={() => setOpenId(x.id)}>
                  <td>
                    {x.ready ? (
                      <span className="pill active">
                        <CheckCircle2 size={10} strokeWidth={2} /> PRONTA
                      </span>
                    ) : (
                      <span className="pill hold">PENDENTE</span>
                    )}
                  </td>
                  <td style={{ whiteSpace: "nowrap" }}>
                    {formatBRDate(x.occurred_on)}
                    {x.start_time && <span className="muted"> · {x.start_time}</span>}
                  </td>
                  <td className="mono">{x.ciops_record || "—"}</td>
                  <td>
                    <div style={{ color: "var(--fg-0)" }}>{x.natureza?.nome ?? "—"}</div>
                    <div className="muted" style={{ fontSize: 11.5 }} title={x.natures.join("\n")}>
                      {x.natures.join(" · ")}
                    </div>
                    {x.intel_participation && <IntelPill />}
                  </td>
                  <td>
                    <div style={{ color: "var(--fg-0)" }}>{x.place_neighborhood || "—"}</div>
                    <div className="muted" style={{ fontSize: 11.5 }}>
                      {x.place_city}
                    </div>
                  </td>
                  <td>{x.area?.nome ?? "—"}</td>
                  <td>{x.opm?.nome ?? "—"}</td>
                  <td className="mono">
                    {x.people_count}
                    {x.unlinked_count > 0 && <span className="muted"> ({x.unlinked_count} s/ dossiê)</span>}
                  </td>
                  <td>
                    <div className="sipom-badges">
                      {x.pendencias.map((p) => (
                        <span key={p.code} className={"pill " + (p.blocking ? "hold" : "cold")} title={p.label}>
                          {SHORT[p.code] ?? p.code}
                        </span>
                      ))}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {openId && <OpsOccurrenceDrawer occurrenceId={openId} onClose={() => setOpenId(null)} onChanged={reload} />}
    </>
  );
}
