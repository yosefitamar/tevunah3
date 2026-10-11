"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Box, CheckCheck, FileText, MapPin, RefreshCw, Shield, Tag, Users, type LucideIcon } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { confirmSipomNaturezas, getSipomQueue, type SipomQueue as Queue } from "@/lib/sipom-api";
import { canSetOpsIntel } from "@/lib/permissions";
import { ALL_PERIOD, periodBounds, type PeriodSelection } from "@/lib/period";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import PeriodButton from "../shared/PeriodButton";
import Select from "../shared/Select";
import SortHeader, { type SortState } from "../shared/SortHeader";
import FiltersModal, { FiltersButton } from "../shared/FiltersModal";
import OpsOccurrenceDrawer from "./OpsOccurrenceDrawer";
import { STEP_OF_PENDING, type OpsStep } from "./OpsOccurrenceBody";

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

// Etapas da ficha em que uma pendência pode travar o envio, na ordem da ficha.
const STEPS: [OpsStep, string][] = [
  ["fato", "FATO"],
  ["local", "LOCAL"],
  ["efetivo", "EFETIVO"],
  ["procedimento", "PROCEDIMENTO"],
  ["apreensoes", "APREENSÕES"],
];

// Trilha de etapas da tabela: um ícone por etapa da ficha, sempre na mesma
// posição, para a coluna poder ser lida de cima a baixo.
const TRAIL: [OpsStep, string, LucideIcon][] = [
  ["fato", "FATO", Tag],
  ["local", "LOCAL", MapPin],
  ["efetivo", "EFETIVO", Shield],
  ["pessoas", "PESSOAS", Users],
  ["procedimento", "PROCEDIMENTO", FileText],
  ["apreensoes", "APREENSÕES", Box],
];

/** A ocorrência tem pendência que impede o envio nesta etapa da ficha. */
function blockedAt(x: Queue["items"][number], step: OpsStep) {
  return x.pendencias.some((p) => p.blocking && STEP_OF_PENDING[p.code] === step);
}

// Recortes por pendência — vivem no modal de FILTROS.
type QueueFilters = {
  step: OpsStep | "";
  code: string;
  /** Avisos (pendência que não impede o envio): "1" só com, "0" só sem. */
  notes: "" | "1" | "0";
  natureza: string;
  city: string;
};
const EMPTY_FILTERS: QueueFilters = { step: "", code: "", notes: "", natureza: "", city: "" };

type Row = Queue["items"][number];

const blockingCount = (x: Row) => x.pendencias.filter((p) => p.blocking).length;
const hasNotes = (x: Row) => x.pendencias.some((p) => !p.blocking);

// Chave de ordenação de cada coluna. Sem ordenação escolhida vale a ordem do
// servidor: do fato mais recente para o mais antigo.
const SORT_KEY: Record<string, (x: Row) => string | number> = {
  etapas: blockingCount,
  occurred_on: (x) => x.occurred_on + " " + (x.start_time || ""),
  ciops_record: (x) => x.ciops_record,
  natureza: (x) => x.natureza?.nome ?? "",
  local: (x) => x.place_city + " " + x.place_neighborhood,
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
  const [filters, setFilters] = useState<QueueFilters>(EMPTY_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const { step, code, notes, natureza, city } = filters;
  const [sort, setSort] = useState<SortState>(null);
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
    if (step) xs = xs.filter((x) => blockedAt(x, step));
    if (code) xs = xs.filter((x) => x.pendencias.some((p) => p.code === code));
    if (notes) xs = xs.filter((x) => hasNotes(x) === (notes === "1"));
    if (natureza) xs = xs.filter((x) => (x.natureza?.nome ?? "") === natureza);
    if (city) xs = xs.filter((x) => x.place_city.toUpperCase() === city);
    const key = sort && SORT_KEY[sort.field];
    if (sort && key) {
      const sign = sort.dir === "asc" ? 1 : -1;
      xs = [...xs].sort((a, b) => {
        const ka = key(a);
        const kb = key(b);
        if (typeof ka === "number" && typeof kb === "number") return (ka - kb) * sign;
        return String(ka).localeCompare(String(kb), "pt-BR", { numeric: true, sensitivity: "base" }) * sign;
      });
    }
    return xs;
  }, [data, status, step, code, notes, natureza, city, sort]);

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

  const naturezas = useMemo(
    () => [...new Set((data?.items ?? []).map((x) => x.natureza?.nome ?? "").filter(Boolean))].sort((a, b) => a.localeCompare(b, "pt-BR")),
    [data],
  );
  const cities = useMemo(
    () => [...new Set((data?.items ?? []).map((x) => x.place_city.toUpperCase()).filter(Boolean))].sort((a, b) => a.localeCompare(b, "pt-BR")),
    [data],
  );

  const summary = data?.summary;
  const codes = Object.entries(summary?.by_code ?? {}).sort((a, b) => b[1] - a[1]);
  const steps = STEPS.map(([k, label]) => [k, label, (data?.items ?? []).filter((x) => blockedAt(x, k)).length] as const).filter(
    ([, , n]) => n > 0,
  );

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
        <FiltersButton count={[step, code, notes, natureza, city].filter(Boolean).length} onClick={() => setFiltersOpen(true)} />
        <div style={{ marginLeft: "auto" }} />
        {data && (
          <span className="muted" style={{ fontSize: 11.5, letterSpacing: "0.06em" }}>
            {items.length} DE {data.items.length}
          </span>
        )}
        {canEdit && toConfirm.length > 0 && (
          <button type="button" className="btn" disabled={busy} onClick={confirmAll} title="Fixa a natureza sugerida pelo de-para nas ocorrências listadas">
            <CheckCheck size={14} strokeWidth={1.8} /> CONFIRMAR NATUREZAS SUGERIDAS ({toConfirm.length})
          </button>
        )}
        <button type="button" className="btn btn-ghost" onClick={reload} disabled={loading}>
          <RefreshCw size={13} strokeWidth={1.8} /> {loading ? "CARREGANDO…" : "ATUALIZAR"}
        </button>
      </div>

      <div className="sipom-legend">
        {TRAIL.map(([k, label, Icon]) => (
          <span key={k}>
            <Icon size={13} strokeWidth={1.9} /> {label}
          </span>
        ))}
        <span className="sipom-legend-sep" />
        <span>
          <i className="sipom-trail-step blk" /> IMPEDE O ENVIO
        </span>
        <span>
          <i className="sipom-trail-step note" /> AVISO
        </span>
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}
      {notice && <div className="banner banner-info">{notice}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl sipom-queue">
            <thead>
              <tr>
                <SortHeader field="etapas" label="ETAPAS" sort={sort} onChange={setSort} width={172} />
                <SortHeader field="occurred_on" label="DATA" sort={sort} onChange={setSort} width={150} />
                <SortHeader field="ciops_record" label="FICHA" sort={sort} onChange={setSort} width={140} />
                <SortHeader field="natureza" label="NATUREZA NO SIPOM" sort={sort} onChange={setSort} />
                <SortHeader field="local" label="LOCAL" sort={sort} onChange={setSort} width={220} />
              </tr>
            </thead>
            <tbody>
              {loading && !data && (
                <tr>
                  <td colSpan={5} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // CARREGANDO…
                  </td>
                </tr>
              )}
              {!loading && items.length === 0 && (
                <tr>
                  <td colSpan={5} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    {status === "pending" && !Object.values(filters).some(Boolean) ? "// NENHUMA PENDÊNCIA NO PERÍODO" : "// NENHUMA OCORRÊNCIA"}
                  </td>
                </tr>
              )}
              {items.map((x) => (
                <tr key={x.id} className="row-clickable" onClick={() => setOpenId(x.id)}>
                  <td>
                    <div className={"sipom-trail" + (x.pendencias.length === 0 ? " sipom-trail--ok" : "")}>
                      {TRAIL.map(([k, label, Icon]) => {
                        const mine = x.pendencias.filter((p) => STEP_OF_PENDING[p.code] === k);
                        const cls = mine.some((p) => p.blocking) ? " blk" : mine.length > 0 ? " note" : "";
                        const tip = mine.length > 0 ? `${label}:\n${mine.map((p) => "• " + p.label).join("\n")}` : `${label}: SEM PENDÊNCIA`;
                        return (
                          <span key={k} className={"sipom-trail-step" + cls} title={tip}>
                            <Icon size={14} strokeWidth={1.9} />
                          </span>
                        );
                      })}
                    </div>
                  </td>
                  <td style={{ whiteSpace: "nowrap" }}>
                    {formatBRDate(x.occurred_on)}
                    {x.start_time && <span className="muted"> · {x.start_time}</span>}
                  </td>
                  <td style={{ color: "var(--fg-0)", whiteSpace: "nowrap" }}>{x.ciops_record || "—"}</td>
                  {/* As naturezas do relatório, a área, a OPM e os envolvidos ficam na
                      ficha: aqui a linha diz só o que é e o que falta. */}
                  <td title={"NO RELATÓRIO:\n" + x.natures.join("\n")}>{x.natureza?.nome ?? "—"}</td>
                  <td>
                    {x.place_neighborhood || "—"}
                    <div className="muted" style={{ fontSize: 11.5 }}>
                      {x.place_city}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {filtersOpen && (
        <FiltersModal
          title="FILTROS · FILA DO SIPOM"
          value={filters}
          empty={EMPTY_FILTERS}
          onClose={() => setFiltersOpen(false)}
          onApply={(f) => {
            setFilters(f);
            setFiltersOpen(false);
          }}
        >
          {(d, set) => (
            <>
              <div className="form-field">
                <span>ETAPA COM PENDÊNCIA QUE IMPEDE O ENVIO</span>
                <Select
                  value={d.step}
                  onChange={(v) => set({ step: v as OpsStep | "" })}
                  options={[
                    { value: "", label: "TODAS AS ETAPAS" },
                    ...steps.map(([k, label, n]) => ({ value: k as string, label: `${label} · ${n}` })),
                  ]}
                />
              </div>
              <div className="form-field">
                <span>AVISOS (NÃO IMPEDEM O ENVIO)</span>
                <div className="seg-row">
                  {(
                    [
                      ["", "TODAS"],
                      ["1", "SÓ COM AVISO"],
                      ["0", "SÓ SEM AVISO"],
                    ] as const
                  ).map(([v, label]) => (
                    <button
                      key={v}
                      type="button"
                      className={"seg-btn" + (d.notes === v ? " seg-btn--on" : "")}
                      onClick={() => set({ notes: v })}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              </div>
              <div className="form-field">
                <span>NATUREZA NO SIPOM</span>
                <Select
                  value={d.natureza}
                  onChange={(v) => set({ natureza: v })}
                  options={[{ value: "", label: "TODAS AS NATUREZAS" }, ...naturezas.map((n) => ({ value: n, label: n }))]}
                />
              </div>
              <div className="form-field">
                <span>CIDADE</span>
                <Select
                  value={d.city}
                  onChange={(v) => set({ city: v })}
                  options={[{ value: "", label: "TODAS AS CIDADES" }, ...cities.map((c) => ({ value: c, label: c }))]}
                />
              </div>
              <div className="form-field">
                <span>MOTIVO</span>
                <Select
                  value={d.code}
                  onChange={(v) => set({ code: v })}
                  options={[
                    { value: "", label: "TODOS OS MOTIVOS" },
                    ...codes.map(([c, n]) => ({ value: c, label: `${SHORT[c] ?? c} · ${n}` })),
                  ]}
                />
              </div>
            </>
          )}
        </FiltersModal>
      )}

      {openId && <OpsOccurrenceDrawer occurrenceId={openId} onClose={() => setOpenId(null)} onChanged={reload} />}
    </>
  );
}
