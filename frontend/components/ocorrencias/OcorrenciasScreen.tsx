"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { CheckCheck, FileUp, History, MapPinned, Plus, Search, Send, ShieldAlert, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  INCIDENT_MEANS,
  INCIDENT_MEANS_LABEL,
  INCIDENT_MEANS_SHORT,
  INCIDENT_TYPE_LABEL,
  type IncidentMeans,
  type IncidentType,
} from "@/lib/incidents-api";
import {
  OCCURRENCE_CATEGORY_LABEL,
  OCCURRENCE_TABS,
  OCCURRENCE_SOURCE_LABEL,
  listOccurrenceLocations,
  listOccurrences,
  type ListOccurrencesOpts,
  type OccurrenceRow,
  type OccurrenceSource,
  type OccurrenceStatus,
  type OccurrenceTab,
  type OccurrencesList,
} from "@/lib/occurrences-api";
import { getOpsFacets, type OpsFacets, type OpsReport } from "@/lib/ops-reports-api";
import { confirmSipomNaturezas, confirmSipomSexos } from "@/lib/sipom-api";
import {
  canCreateIncidents,
  canImportOpsReports,
  canReadIncidents,
  canReadOpsReports,
  canSetOpsIntel,
} from "@/lib/permissions";
import { useIncidentLocations } from "@/lib/useIncidentLocations";
import { ALL_PERIOD, periodBounds, type PeriodSelection } from "@/lib/period";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import Select from "../shared/Select";
import PeriodButton from "../shared/PeriodButton";
import FiltersModal, { FiltersButton } from "../shared/FiltersModal";
import OpsOccurrenceDrawer from "../operacional/OpsOccurrenceDrawer";
import ImportOpsReportModal from "../operacional/ImportOpsReportModal";
import OpsReportsHistoryModal from "../operacional/OpsReportsHistoryModal";
import { BLOCKING_STEPS, PENDING_SHORT, SipomTrail, SipomTrailLegend } from "../operacional/SipomTrail";
import CreateOcorrenciaModal from "./CreateOcorrenciaModal";
import OcorrenciaDrawer from "./OcorrenciaDrawer";
import SipomBatchSendModal, { type BatchItem } from "./SipomBatchSendModal";
import SipomAreaRefsModal from "./SipomAreaRefsModal";

const PAGE_SIZE = 25;

// Recortes que vivem no modal de FILTROS. Território vale para as duas abas;
// os demais pertencem a uma delas e zeram na troca.
type Filters = {
  city: string;
  neighborhood: string;
  // Homicídios.
  means: IncidentMeans;
  // Produtividade.
  source: "" | OccurrenceSource;
  type: "" | IncidentType;
  step: string;
  pendencia: string;
  notes: "" | "1" | "0";
  cia: string;
  nature: string;
  intel: "" | "1" | "0";
};

const EMPTY_FILTERS: Filters = {
  city: "",
  neighborhood: "",
  means: "",
  source: "",
  type: "",
  step: "",
  pendencia: "",
  notes: "",
  cia: "",
  nature: "",
  intel: "",
};

// Tipos do cadastro manual que caem na aba PRODUTIVIDADE (homicídio tem a
// própria aba). O primeiro é o tipo com que NOVA OCORRÊNCIA abre nela.
const PRODUCTIVITY_TYPES: IncidentType[] = ["apreensao", "prisao"];

// O que está aberto: o cadastro manual ou a ocorrência do relatório
// operacional. Linha com as duas fontes abre o cadastro, que leva ao relatório.
type OpenTarget = { kind: "incident" | "ops"; id: string };

const SEG3 = [
  ["", "TODAS"],
  ["1", "COM"],
  ["0", "SEM"],
] as const;

/**
 * Ocorrências: a única listagem do que a agência registra. Três abas:
 *
 *  - PRODUTIVIDADE: tudo o que veio do relatório operacional (importado aqui
 *    mesmo, em PDF) e as prisões e apreensões do cadastro manual. A trilha de
 *    etapas mostra o que falta em cada ocorrência, e a que já foi enviada ao
 *    SIPOM vem sinalizada.
 *  - FILA DO SIPOM: o trabalho a fazer — as ocorrências do relatório que
 *    ainda não foram enviadas, pendentes ou prontas.
 *  - HOMICÍDIOS: o cadastro de CVLI.
 *
 * A ficha CIOPS é a identidade — cadastro e relatório com a mesma ficha são
 * uma linha. Busca, período e território valem para as duas abas.
 */
export default function OcorrenciasScreen() {
  const { user: me } = useAuth();
  const [tab, setTab] = useState<OccurrenceTab>("produtividade");
  const [data, setData] = useState<OccurrencesList | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [period, setPeriod] = useState<PeriodSelection>(ALL_PERIOD);
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [status, setStatus] = useState<OccurrenceStatus>("");
  const [report, setReport] = useState<OpsReport | null>(null);
  const [showFilters, setShowFilters] = useState(false);
  // Busca livre fica fora do modal: é o gesto mais frequente ("cadê a
  // ocorrência do fulano?"), não um recorte que se configura uma vez.
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "occurred_on", dir: "desc" });
  const [showCreate, setShowCreate] = useState(false);
  const [showImport, setShowImport] = useState(false);
  const [showHistory, setShowHistory] = useState(false);
  const [open, setOpen] = useState<OpenTarget | null>(null);
  const [facets, setFacets] = useState<OpsFacets | null>(null);
  const [busy, setBusy] = useState(false);
  // Fila do SIPOM: as prontas escolhidas para o envio em lote (vale entre
  // páginas; troca de aba limpa).
  const [selected, setSelected] = useState<Map<string, BatchItem>>(new Map());
  const [batch, setBatch] = useState<BatchItem[] | null>(null);
  const [areaRefs, setAreaRefs] = useState(false);

  // Cada fonte tem a sua permissão; o servidor recorta a listagem pelo que o
  // usuário lê.
  const canOps = canReadOpsReports(me);
  const canRead = canReadIncidents(me) || canOps;
  const canCreate = canCreateIncidents(me);
  const canImport = canImportOpsReports(me);
  const canEditOps = canSetOpsIntel(me);
  const { cities, neighborhoodsOf } = useIncidentLocations(listOccurrenceLocations);
  const homicides = tab === "homicidios";

  useEffect(() => {
    const h = window.setTimeout(() => setDebouncedSearch(search.trim()), 350);
    return () => window.clearTimeout(h);
  }, [search]);

  const reloadFacets = useCallback(() => {
    if (!canOps) return;
    getOpsFacets()
      .then(setFacets)
      .catch(() => setFacets(null));
  }, [canOps]);

  useEffect(() => {
    reloadFacets();
  }, [reloadFacets]);

  const bounds = useMemo(() => periodBounds(period), [period]);

  // O recorte da listagem (abas, filtros, busca), sem a paginação.
  const listOpts = useCallback((): ListOccurrencesOpts => {
    const opts: ListOccurrencesOpts = {
      category: tab,
      city: filters.city,
      neighborhood: filters.neighborhood,
      date_from: bounds.from,
      date_to: bounds.to,
      search: debouncedSearch,
      sort_by: sort?.field as ListOccurrencesOpts["sort_by"],
      sort_dir: sort?.dir,
    };
    if (tab === "homicidios") {
      opts.means = filters.means;
    } else {
      Object.assign(opts, {
        source: filters.source,
        type: filters.type,
        status,
        step: filters.step,
        pendencia: filters.pendencia,
        notes: filters.notes,
        cia: filters.cia,
        nature: filters.nature,
        intel: filters.intel,
        report_id: report?.id,
      } satisfies ListOccurrencesOpts);
    }
    return opts;
  }, [tab, filters, status, report?.id, bounds.from, bounds.to, debouncedSearch, sort]);

  const reload = useCallback(async () => {
    if (!canRead) return;
    setLoading(true);
    setError(null);
    try {
      setData(await listOccurrences({ ...listOpts(), limit: PAGE_SIZE, offset: page * PAGE_SIZE }));
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [canRead, listOpts, page]);

  useEffect(() => {
    reload();
  }, [reload]);

  function switchTab(next: OccurrenceTab) {
    if (next === tab) return;
    setTab(next);
    setPage(0);
    // Só o território vale para as duas abas.
    setFilters((f) => ({ ...EMPTY_FILTERS, city: f.city, neighborhood: f.neighborhood }));
    setStatus("");
    setReport(null);
    setSort({ field: "occurred_on", dir: "desc" });
    setSelected(new Map());
    // Some com as linhas da outra aba já na troca — as colunas mudam.
    setData((d) => (d ? { ...d, items: [], total: 0 } : d));
    setLoading(true);
  }

  const items = data?.items ?? [];
  // Naturezas que o de-para só sugeriu, entre as linhas à vista.
  const toConfirm = items.filter((x) => x.ops_id && x.pendencias.some((p) => p.code === "natureza_confirmar"));

  // Sexos que só foram sugeridos (pelo nome, escolha anterior ou histórico).
  const toConfirmSexo = items.filter((x) => x.ops_id && x.pendencias.some((p) => p.code === "pessoa_sexo_confirmar"));

  async function confirmAll() {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const r = await confirmSipomNaturezas(toConfirm.map((x) => x.ops_id as string));
      setNotice(`${r.confirmed} natureza(s) confirmada(s).`);
      await reload();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao confirmar");
    } finally {
      setBusy(false);
    }
  }

  async function confirmAllSexos() {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const r = await confirmSipomSexos(toConfirmSexo.map((x) => x.ops_id as string));
      setNotice(`Sexo confirmado de ${r.confirmed} envolvido(s) em ${r.occurrences} ocorrência(s).`);
      await reload();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao confirmar");
    } finally {
      setBusy(false);
    }
  }

  // Envio em lote: só na fila, para quem pode enviar, e só o que está pronto.
  const selectable = tab === "fila" && canEditOps;
  const pageReady = items.filter(sendable);
  const pageAllSelected = pageReady.length > 0 && pageReady.every((x) => selected.has(x.ops_id as string));

  function toggle(rows: OccurrenceRow[], on: boolean) {
    setSelected((cur) => {
      const next = new Map(cur);
      for (const r of rows) {
        if (on) next.set(r.ops_id as string, batchItem(r));
        else next.delete(r.ops_id as string);
      }
      return next;
    });
  }

  // Todas as prontas do recorte atual, de todas as páginas (o servidor
  // devolve no máximo 100 por vez).
  async function selectAllReady() {
    setBusy(true);
    setError(null);
    try {
      const all: OccurrenceRow[] = [];
      for (let offset = 0; ; offset += 100) {
        const r = await listOccurrences({ ...listOpts(), status: "ready", limit: 100, offset });
        all.push(...r.items.filter(sendable));
        if (r.items.length < 100 || offset + 100 >= r.total) break;
      }
      toggle(all, true);
      setNotice(`${all.length} ocorrência(s) pronta(s) selecionada(s).`);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao selecionar");
    } finally {
      setBusy(false);
    }
  }

  function openBatch() {
    // Em ordem do fato: a mais antiga primeiro.
    const list = [...selected.values()].sort((a, b) => a.sort.localeCompare(b.sort));
    setBatch(list);
  }

  if (!canRead) {
    return (
      <div className="placeholder">
        <ShieldAlert size={36} strokeWidth={1.2} />
        <div className="ph-tag">// ACESSO RESTRITO</div>
        <div className="ph-ttl">SEM PERMISSÃO DE LEITURA DE OCORRÊNCIAS</div>
        <div className="ph-sub">Contate o administrador.</div>
      </div>
    );
  }

  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const sipom = data?.sipom;
  const codes = Object.entries(sipom?.by_code ?? {}).sort((a, b) => b[1] - a[1]);
  const activeCount = (
    homicides
      ? [filters.city, filters.neighborhood, filters.means]
      : [
          filters.city,
          filters.neighborhood,
          filters.source,
          filters.type,
          filters.step,
          filters.pendencia,
          filters.notes,
          filters.cia,
          filters.nature,
          filters.intel,
        ]
  ).filter(Boolean).length;
  const cols = selectable ? 6 : 5;
  const resetPage = () => setPage(0);

  return (
    <div className="screen-fill">
      <div className="toolbar">
        <div className="tabs">
          {OCCURRENCE_TABS.filter((c) => c !== "fila" || canOps).map((c) => (
            <button
              key={c}
              type="button"
              className={"tab" + (tab === c ? " tab-active" : "")}
              onClick={() => switchTab(c)}
            >
              {OCCURRENCE_CATEGORY_LABEL[c]}
              {data?.counts && <span className="tab-count">{data.counts[c]}</span>}
            </button>
          ))}
        </div>
        <div style={{ marginLeft: "auto" }} />
        {!homicides && canOps && (
          <button type="button" className="btn ops-nowrap" onClick={() => setShowHistory(true)} title="Relatórios operacionais já importados">
            <History size={13} strokeWidth={1.8} /> IMPORTAÇÕES
          </button>
        )}
        {!homicides && canImport && (
          <button type="button" className="btn ops-nowrap" onClick={() => setShowImport(true)}>
            <FileUp size={14} strokeWidth={2} /> IMPORTAR PDF
          </button>
        )}
        {canCreate && (
          <button type="button" className="btn btn-primary ops-nowrap" onClick={() => setShowCreate(true)}>
            <Plus size={14} strokeWidth={2} /> NOVA OCORRÊNCIA
          </button>
        )}
      </div>

      {/* Segunda linha: o que recorta a lista. */}
      <div className="toolbar">
        <div className="toolbar-search toolbar-search--compact">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            value={search}
            onChange={(e) => {
              // A página 3 do recorte anterior não existe mais depois que o
              // termo muda.
              setSearch(e.target.value);
              resetPage();
            }}
            placeholder="buscar…"
            title="Nome, CPF, natureza, placa, nº de série, histórico ou ficha CIOPS"
          />
          {search && (
            <button
              type="button"
              className="action-btn"
              onClick={() => {
                setSearch("");
                resetPage();
              }}
              aria-label="Limpar busca"
            >
              <X size={12} />
            </button>
          )}
        </div>
        <PeriodButton
          value={period}
          onChange={(p) => {
            setPeriod(p);
            resetPage();
          }}
          title="PERÍODO · DATA DO FATO"
        />
        {/* Situação do envio ao SIPOM: só a ocorrência do relatório tem. */}
        {!homicides && canOps && (
          <div className="status-select">
            <Select
              value={status}
              onChange={(v) => {
                setStatus(v as OccurrenceStatus);
                resetPage();
              }}
              options={[
                { value: "", label: "TODAS AS SITUAÇÕES" },
                { value: "pending", label: sipom ? `PENDENTES · ${sipom.pending}` : "PENDENTES" },
                { value: "ready", label: sipom ? `PRONTAS · ${sipom.ready}` : "PRONTAS" },
                // Na fila só há o que não foi enviado.
                ...(tab === "fila"
                  ? []
                  : [{ value: "sent", label: sipom ? `ENVIADAS · ${sipom.sent}` : "ENVIADAS" }]),
              ]}
              placeholder="TODAS AS SITUAÇÕES"
            />
          </div>
        )}
        <FiltersButton count={activeCount} onClick={() => setShowFilters(true)} />
        <div style={{ marginLeft: "auto" }} />
      </div>

      {report && (
        <div className="ops-filter-row">
          <span className="muted">SÓ AS OCORRÊNCIAS DO</span>
          <span className="chip on ops-report-chip" title={report.file_name}>
            RELATÓRIO DE {report.report_date ? formatBRDate(report.report_date) : "—"}
            <button
              type="button"
              className="action-btn"
              onClick={() => {
                setReport(null);
                resetPage();
              }}
              aria-label="Remover filtro de relatório"
            >
              <X size={11} />
            </button>
          </span>
        </div>
      )}

      {!homicides && canOps && (
        <SipomTrailLegend sent={tab === "produtividade"}>
          {canEditOps && toConfirm.length > 0 && (
            <button
              type="button"
              className="btn btn-sm"
              style={{ marginLeft: "auto" }}
              disabled={busy}
              onClick={confirmAll}
              title="Fixa a natureza sugerida pelo de-para nas ocorrências desta página"
            >
              <CheckCheck size={13} strokeWidth={1.8} /> CONFIRMAR NATUREZAS ({toConfirm.length})
            </button>
          )}
          {selectable && (
            <>
              <button
                type="button"
                className="btn btn-sm"
                style={{ marginLeft: "auto" }}
                disabled={busy}
                onClick={() => setAreaRefs(true)}
                title="Referências de área da unidade militar: resolver os lugares sem área e manter as referências"
              >
                <MapPinned size={13} strokeWidth={1.8} /> REFERÊNCIAS DE ÁREA
              </button>
              <button
                type="button"
                className="btn btn-sm"
                disabled={busy || (sipom?.ready ?? 0) === 0}
                onClick={selectAllReady}
                title="Seleciona todas as ocorrências prontas do recorte atual, em todas as páginas"
              >
                <CheckCheck size={13} strokeWidth={1.8} /> SELECIONAR TODAS AS PRONTAS
              </button>
              {selected.size > 0 && (
                <>
                  <button type="button" className="btn btn-ghost btn-sm" disabled={busy} onClick={() => setSelected(new Map())}>
                    LIMPAR SELEÇÃO
                  </button>
                  <button type="button" className="btn btn-primary btn-sm" disabled={busy} onClick={openBatch}>
                    <Send size={13} strokeWidth={1.8} /> ENVIAR SELECIONADAS ({selected.size})
                  </button>
                </>
              )}
            </>
          )}
          {canEditOps && toConfirmSexo.length > 0 && (
            <button
              type="button"
              className="btn btn-sm"
              style={toConfirm.length > 0 || selectable ? undefined : { marginLeft: "auto" }}
              disabled={busy}
              onClick={confirmAllSexos}
              title="Fixa o sexo sugerido (pelo nome, escolha anterior ou histórico) dos envolvidos das ocorrências desta página"
            >
              <CheckCheck size={13} strokeWidth={1.8} /> CONFIRMAR SEXOS ({toConfirmSexo.length})
            </button>
          )}
        </SipomTrailLegend>
      )}

      {notice && (
        <div className="banner banner-info" onClick={() => setNotice(null)} role="status">
          ✓ {notice}
        </div>
      )}
      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl sipom-queue">
            <thead>
              <tr>
                {/* Só o essencial para achar a ocorrência e saber o que falta
                    nela; o resto está na ficha, a um clique. Em cada aba uma
                    coluna fica sem largura fixa e absorve a sobra. */}
                {homicides ? (
                  <>
                    <SortHeader field="occurred_on" label="DATA" sort={sort} onChange={setSort} width={150} />
                    <SortHeader field="ciops_record" label="FICHA" sort={sort} onChange={setSort} width={150} />
                    <th style={{ width: 200 }}>MEIO</th>
                    <SortHeader field="place" label="LOCAL" sort={sort} onChange={setSort} width={260} />
                    <th>DESCRIÇÃO</th>
                  </>
                ) : (
                  <>
                    {selectable && (
                      <th style={{ width: 34 }} className="tbl-check">
                        <input
                          type="checkbox"
                          checked={pageAllSelected}
                          disabled={pageReady.length === 0}
                          onChange={(e) => toggle(pageReady, e.target.checked)}
                          aria-label="Selecionar as prontas desta página"
                          title="Selecionar as prontas desta página"
                        />
                      </th>
                    )}
                    <SortHeader field="pending" label="ETAPAS" sort={sort} onChange={setSort} width={172} />
                    <SortHeader field="occurred_on" label="DATA" sort={sort} onChange={setSort} width={150} />
                    <SortHeader field="ciops_record" label="FICHA" sort={sort} onChange={setSort} width={150} />
                    <SortHeader field="nature" label="NATUREZA" sort={sort} onChange={setSort} />
                    <SortHeader field="place" label="LOCAL" sort={sort} onChange={setSort} width={240} />
                  </>
                )}
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={cols} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // CARREGANDO…
                  </td>
                </tr>
              )}
              {!loading && items.length === 0 && (
                <tr>
                  <td colSpan={cols} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // NENHUMA OCORRÊNCIA ENCONTRADA
                  </td>
                </tr>
              )}
              {!loading &&
                items.map((it) => (
                  <Row
                    key={(it.incident_id ?? "") + ":" + (it.ops_id ?? "")}
                    row={it}
                    homicides={homicides}
                    selection={
                      selectable
                        ? { checked: selected.has(it.ops_id ?? ""), onToggle: (on) => toggle([it], on) }
                        : undefined
                    }
                    onOpen={() =>
                      setOpen(
                        it.incident_id
                          ? { kind: "incident", id: it.incident_id }
                          : { kind: "ops", id: it.ops_id as string },
                      )
                    }
                  />
                ))}
            </tbody>
          </table>
        </div>

        <div className="pagination">
          <span className="muted">
            {total === 0
              ? "—"
              : `${page * PAGE_SIZE + 1}–${Math.min((page + 1) * PAGE_SIZE, total)} de ${total}`}
          </span>
          <div className="pagination-controls">
            <button type="button" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
              ‹ ANTERIOR
            </button>
            <span>
              PÁGINA {page + 1} / {pages}
            </span>
            <button
              type="button"
              disabled={page >= pages - 1}
              onClick={() => setPage((p) => Math.min(pages - 1, p + 1))}
            >
              PRÓXIMA ›
            </button>
          </div>
        </div>
      </div>

      {areaRefs && (
        <SipomAreaRefsModal
          onClose={() => setAreaRefs(false)}
          onChanged={() => {
            reload();
            reloadFacets();
          }}
        />
      )}

      {batch && (
        <SipomBatchSendModal
          items={batch}
          onClose={() => setBatch(null)}
          onDone={() => {
            setSelected(new Map());
            reload();
            reloadFacets();
          }}
        />
      )}

      {showFilters && (
        <FiltersModal
          title={"FILTROS · " + OCCURRENCE_CATEGORY_LABEL[tab]}
          value={filters}
          empty={EMPTY_FILTERS}
          width={560}
          onClose={() => setShowFilters(false)}
          onApply={(f) => {
            setFilters(f);
            resetPage();
            setShowFilters(false);
          }}
        >
          {(d, set) => (
            <>
              <div className="form-grid-2">
                <div className="form-field">
                  <span>MUNICÍPIO</span>
                  <Select
                    value={d.city}
                    // Bairro pertence ao município: trocar de cidade zera o bairro.
                    onChange={(v) => set({ city: v, neighborhood: "" })}
                    options={[
                      { value: "", label: "TODOS" },
                      ...cities.map((c) => ({ value: c.city, label: `${c.city} (${c.count})` })),
                    ]}
                    searchable
                  />
                </div>
                <div className="form-field">
                  <span>BAIRRO</span>
                  <Select
                    value={d.neighborhood}
                    onChange={(v) => set({ neighborhood: v })}
                    options={[
                      { value: "", label: "TODOS" },
                      ...neighborhoodsOf(d.city).map((n) => ({ value: n, label: n })),
                    ]}
                    searchable
                  />
                </div>
              </div>

              {homicides ? (
                <div className="form-field">
                  <span>MEIO UTILIZADO</span>
                  <Select
                    value={d.means}
                    onChange={(v) => set({ means: v as IncidentMeans })}
                    options={[
                      { value: "", label: "TODOS" },
                      ...INCIDENT_MEANS.map((m) => ({ value: m, label: INCIDENT_MEANS_LABEL[m] })),
                    ]}
                  />
                </div>
              ) : (
                <>
                  <div className="form-grid-2">
                    <div className="form-field">
                      <span>ETAPA QUE IMPEDE O ENVIO AO SIPOM</span>
                      <Select
                        value={d.step}
                        onChange={(v) => set({ step: v })}
                        options={[
                          { value: "", label: "TODAS AS ETAPAS" },
                          ...BLOCKING_STEPS.map(([k, label]) => ({ value: k as string, label })),
                        ]}
                      />
                    </div>
                    <div className="form-field">
                      <span>PENDÊNCIA</span>
                      <Select
                        value={d.pendencia}
                        onChange={(v) => set({ pendencia: v })}
                        options={[
                          { value: "", label: "TODAS" },
                          ...codes.map(([c, n]) => ({ value: c, label: `${PENDING_SHORT[c] ?? c} · ${n}` })),
                        ]}
                      />
                    </div>
                  </div>
                  <div className="form-grid-2">
                    <div className="form-field">
                      <span>AVISOS (NÃO IMPEDEM O ENVIO)</span>
                      <Seg value={d.notes} onChange={(v) => set({ notes: v })} />
                    </div>
                    <div className="form-field">
                      <span>PARTICIPAÇÃO DA INTELIGÊNCIA</span>
                      <Seg value={d.intel} onChange={(v) => set({ intel: v })} />
                    </div>
                  </div>
                  <div className="form-grid-2">
                    <div className="form-field">
                      <span>COMPANHIA</span>
                      <Select
                        value={d.cia}
                        onChange={(v) => set({ cia: v })}
                        options={[
                          { value: "", label: "TODAS AS CIAS" },
                          ...(facets?.cias ?? []).map((c) => ({ value: c, label: c })),
                        ]}
                      />
                    </div>
                    <div className="form-field">
                      <span>NATUREZA NO RELATÓRIO</span>
                      <Select
                        value={d.nature}
                        onChange={(v) => set({ nature: v })}
                        options={[
                          { value: "", label: "TODAS AS NATUREZAS" },
                          ...(facets?.natures ?? []).map((n) => ({ value: n, label: n })),
                        ]}
                        searchable
                      />
                    </div>
                  </div>
                  <div className="form-grid-2">
                    <div className="form-field">
                      <span>ORIGEM</span>
                      <Select
                        value={d.source}
                        onChange={(v) => set({ source: v as "" | OccurrenceSource })}
                        options={[
                          { value: "", label: "TODAS" },
                          { value: "manual", label: OCCURRENCE_SOURCE_LABEL.manual },
                          { value: "operacional", label: OCCURRENCE_SOURCE_LABEL.operacional },
                        ]}
                      />
                    </div>
                    <div className="form-field">
                      <span>TIPO DO CADASTRO MANUAL</span>
                      <Select
                        value={d.type}
                        onChange={(v) => set({ type: v as "" | IncidentType })}
                        options={[
                          { value: "", label: "TODOS" },
                          ...PRODUCTIVITY_TYPES.map((t) => ({ value: t, label: INCIDENT_TYPE_LABEL[t] })),
                        ]}
                      />
                    </div>
                  </div>
                </>
              )}
            </>
          )}
        </FiltersModal>
      )}

      {showCreate && (
        <CreateOcorrenciaModal
          initialType={homicides ? "homicidio" : PRODUCTIVITY_TYPES[0]}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            setShowCreate(false);
            setOpen({ kind: "incident", id });
            reload();
          }}
        />
      )}

      {showImport && (
        <ImportOpsReportModal
          onClose={() => setShowImport(false)}
          onImported={(r) => {
            setShowImport(false);
            const parts = [`${r.report.imported_occurrences} ocorrência(s) gravada(s)`];
            if (r.skipped_ciops.length) parts.push(`${r.skipped_ciops.length} já existiam`);
            if (r.auto_linked) parts.push(`${r.auto_linked} pessoa(s) vinculada(s) a dossiês`);
            setNotice(`Relatório de ${r.report.report_date ? formatBRDate(r.report.report_date) : "—"}: ${parts.join(" · ")}.`);
            setReport(r.report);
            resetPage();
            reloadFacets();
          }}
        />
      )}

      {showHistory && (
        <OpsReportsHistoryModal
          onClose={() => setShowHistory(false)}
          onPick={(r) => {
            setShowHistory(false);
            setReport(r);
            setPeriod(ALL_PERIOD);
            resetPage();
          }}
        />
      )}

      {open?.kind === "incident" && (
        <OcorrenciaDrawer incidentId={open.id} onClose={() => setOpen(null)} onChanged={reload} />
      )}
      {open?.kind === "ops" && (
        <OpsOccurrenceDrawer occurrenceId={open.id} onClose={() => setOpen(null)} onChanged={reload} />
      )}
    </div>
  );
}

function Seg({ value, onChange }: { value: "" | "1" | "0"; onChange: (v: "" | "1" | "0") => void }) {
  return (
    <div className="seg-row">
      {SEG3.map(([v, label]) => (
        <button
          key={v}
          type="button"
          className={"seg-btn" + (value === v ? " seg-btn--on" : "")}
          onClick={() => onChange(v)}
        >
          {label}
        </button>
      ))}
    </div>
  );
}

/** Pronta para o envio em lote: do relatório, não enviada, sem pendência que impeça. */
function sendable(r: OccurrenceRow) {
  return !!r.ops_id && !r.sipom_sent_at && !r.pendencias.some((p) => p.blocking);
}

function batchItem(r: OccurrenceRow): BatchItem {
  return {
    id: r.ops_id as string,
    ficha: r.ciops_record,
    date: formatBRDate(r.occurred_on) + (r.time ? " " + r.time : ""),
    sort: r.occurred_on + " " + (r.time ?? "") + " " + r.ciops_record,
  };
}

function Row({
  row,
  homicides,
  onOpen,
  selection,
}: {
  row: OccurrenceRow;
  homicides: boolean;
  onOpen: () => void;
  /** Fila do SIPOM: caixa de seleção para o envio em lote. */
  selection?: { checked: boolean; onToggle: (on: boolean) => void };
}) {
  const date = (
    <td style={{ whiteSpace: "nowrap" }}>
      {formatBRDate(row.occurred_on)}
      {row.time ? <span className="muted"> · {row.time}</span> : null}
    </td>
  );
  const ficha = <td style={{ color: "var(--fg-0)", whiteSpace: "nowrap" }}>{row.ciops_record || "—"}</td>;
  const place = (
    <td>
      {row.neighborhood || "—"}
      <div className="muted" style={{ fontSize: 11.5 }}>
        {row.city}
      </div>
    </td>
  );

  if (homicides) {
    return (
      <tr onClick={onOpen} className="row-clickable">
        {date}
        {ficha}
        <td className={row.means ? undefined : "muted"} title={row.means ? INCIDENT_MEANS_LABEL[row.means] : undefined}>
          {row.means ? INCIDENT_MEANS_SHORT[row.means] : "—"}
        </td>
        {place}
        <td>
          <div className="tbl-nature" title={row.description}>
            {row.description || "—"}
          </div>
        </td>
      </tr>
    );
  }

  // Natureza: a do SIPOM, já resolvida; sem ela, o que o relatório escreveu.
  // Linha do cadastro manual mostra o tipo.
  const natures = row.natures.join(" · ");
  const nature = row.sipom_natureza || natures || (row.type ? INCIDENT_TYPE_LABEL[row.type] : "—");
  return (
    <tr onClick={onOpen} className="row-clickable">
      {selection && (
        <td className="tbl-check" onClick={(e) => e.stopPropagation()}>
          <input
            type="checkbox"
            checked={selection.checked}
            disabled={!sendable(row)}
            onChange={(e) => selection.onToggle(e.target.checked)}
            aria-label="Selecionar para envio"
            title={sendable(row) ? "Selecionar para o envio em lote" : "Tem pendência que impede o envio"}
          />
        </td>
      )}
      <td>
        {row.ops_id ? (
          <SipomTrail pendencias={row.pendencias} sentAt={row.sipom_sent_at} erro={row.sipom_erro} />
        ) : (
          <span className="muted" style={{ fontSize: 11.5 }} title="Ocorrência do cadastro manual: não vai ao SIPOM por aqui">
            CADASTRO MANUAL
          </span>
        )}
      </td>
      {date}
      {ficha}
      <td title={natures ? "NO RELATÓRIO:\n" + row.natures.join("\n") : undefined}>
        {nature}
        {row.type && row.ops_id && <span className="muted"> · {INCIDENT_TYPE_LABEL[row.type]}</span>}
      </td>
      {place}
    </tr>
  );
}
