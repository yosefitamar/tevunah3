"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Car, Crosshair, FileUp, History, List, Pill, Search, Send, ShieldAlert, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  getOpsFacets,
  listOpsOccurrences,
  opsTimeRange,
  type OpsFacets,
  type OpsOccurrenceRow,
  type OpsReport,
} from "@/lib/ops-reports-api";
import { canImportOpsReports, canReadOpsReports } from "@/lib/permissions";
import { ALL_PERIOD, isAllPeriod, periodBounds, type PeriodSelection } from "@/lib/period";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import Select from "../shared/Select";
import PeriodButton from "../shared/PeriodButton";
import FiltersModal, { FiltersButton } from "../shared/FiltersModal";
import { IntelPill } from "./IntelStatus";
import ImportOpsReportModal from "./ImportOpsReportModal";
import OpsOccurrenceDrawer from "./OpsOccurrenceDrawer";
import OpsReportsHistoryModal from "./OpsReportsHistoryModal";
import SipomQueue from "./SipomQueue";

const PAGE_SIZE = 25;

// Recortes não temporais — vivem no modal de FILTROS.
type OpsFilters = { cia: string; nature: string; intel: "" | "1" | "0" };
const EMPTY_FILTERS: OpsFilters = { cia: "", nature: "", intel: "" };

function activeFilterCount(f: OpsFilters): number {
  return [f.cia, f.nature, f.intel].filter(Boolean).length;
}

export default function RelatorioOperacionalScreen() {
  const { user: me } = useAuth();
  const canRead = canReadOpsReports(me);
  const canImport = canImportOpsReports(me);

  const [data, setData] = useState<{ items: OpsOccurrenceRow[]; total: number } | null>(null);
  const [facets, setFacets] = useState<OpsFacets | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [period, setPeriod] = useState<PeriodSelection>(ALL_PERIOD);
  const [filters, setFilters] = useState<OpsFilters>(EMPTY_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [report, setReport] = useState<OpsReport | null>(null);
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "occurred_on", dir: "desc" });
  const [showImport, setShowImport] = useState(false);
  const [showHistory, setShowHistory] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  // Visão da tela: acervo de ocorrências ou fila de envio ao SIPOM.
  const [view, setView] = useState<"ocorrencias" | "sipom">("ocorrencias");

  useEffect(() => {
    const h = window.setTimeout(() => setDebouncedSearch(search.trim()), 350);
    return () => window.clearTimeout(h);
  }, [search]);

  const bounds = useMemo(() => periodBounds(period), [period]);

  const reload = useCallback(async () => {
    if (!canRead) return;
    setLoading(true);
    setError(null);
    try {
      const res = await listOpsOccurrences({
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        search: debouncedSearch,
        cia: filters.cia,
        nature: filters.nature,
        intel: filters.intel,
        report_id: report?.id,
        date_from: bounds.from,
        date_to: bounds.to,
        sort_by: (sort?.field as "occurred_on" | "ciops_record" | "place_city" | "cia") || undefined,
        sort_dir: sort?.dir,
      });
      setData(res);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [canRead, page, debouncedSearch, filters, report?.id, bounds.from, bounds.to, sort]);

  const reloadFacets = useCallback(() => {
    if (!canRead) return;
    getOpsFacets()
      .then(setFacets)
      .catch(() => setFacets(null));
  }, [canRead]);

  useEffect(() => {
    reload();
  }, [reload]);

  useEffect(() => {
    reloadFacets();
  }, [reloadFacets]);

  if (!canRead) {
    return (
      <div className="placeholder">
        <ShieldAlert size={36} strokeWidth={1.2} />
        <div className="ph-tag">// ACESSO RESTRITO</div>
        <div className="ph-ttl">SEM PERMISSÃO DE LEITURA DO RELATÓRIO OPERACIONAL</div>
        <div className="ph-sub">Contate o administrador.</div>
      </div>
    );
  }

  const viewTabs = (
    <div className="screen-tabs">
      <button
        type="button"
        className={"modal-tab" + (view === "ocorrencias" ? " modal-tab--on" : "")}
        onClick={() => setView("ocorrencias")}
      >
        <List size={12} strokeWidth={1.8} /> OCORRÊNCIAS
      </button>
      <button
        type="button"
        className={"modal-tab" + (view === "sipom" ? " modal-tab--on" : "")}
        onClick={() => setView("sipom")}
      >
        <Send size={12} strokeWidth={1.8} /> ENVIO AO SIPOM
      </button>
    </div>
  );

  if (view === "sipom") {
    return (
      <div className="screen-fill">
        {viewTabs}
        <SipomQueue />
      </div>
    );
  }

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const resetPage = () => setPage(0);

  return (
    <div className="screen-fill">
      {viewTabs}
      <div className="toolbar">
        <div className="toolbar-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              resetPage();
            }}
            placeholder="buscar por pessoa, placa, nº de série, bairro, ficha CIOPS, policial ou histórico…"
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
        />
        <FiltersButton count={activeFilterCount(filters)} onClick={() => setFiltersOpen(true)} />
        <div style={{ marginLeft: "auto" }} />
        <button type="button" className="btn ops-nowrap" onClick={() => setShowHistory(true)}>
          <History size={13} strokeWidth={1.8} /> IMPORTAÇÕES
        </button>
        {canImport && (
          <button type="button" className="btn btn-primary ops-nowrap" onClick={() => setShowImport(true)}>
            <FileUp size={14} strokeWidth={2} /> IMPORTAR PDF
          </button>
        )}
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
      {notice && (
        <div className="banner banner-info" onClick={() => setNotice(null)} role="status">
          ✓ {notice}
        </div>
      )}
      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="occurred_on" label="DATA / HORA" sort={sort} onChange={setSort} width={150} />
                <th>NATUREZA</th>
                <SortHeader field="cia" label="UNIDADE" sort={sort} onChange={setSort} width={110} />
                <SortHeader field="place_city" label="LOCAL" sort={sort} onChange={setSort} width={160} />
                <th style={{ width: 100 }}>EQUIPE</th>
                <SortHeader field="ciops_record" label="FICHA CIOPS" sort={sort} onChange={setSort} width={130} />
                <th style={{ width: 100 }}>APREENSÕES</th>
                <th style={{ width: 190 }}>ACUSADOS</th>
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={8} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // CARREGANDO…
                  </td>
                </tr>
              )}
              {!loading && items.length === 0 && (
                <tr>
                  <td colSpan={8} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    {total === 0 && !debouncedSearch && activeFilterCount(filters) === 0 && !report && isAllPeriod(period)
                      ? "// NENHUM RELATÓRIO IMPORTADO — USE “IMPORTAR PDF”"
                      : "// NENHUMA OCORRÊNCIA ENCONTRADA"}
                  </td>
                </tr>
              )}
              {!loading && items.map((it) => <Row key={it.id} row={it} onOpen={() => setOpenId(it.id)} />)}
            </tbody>
          </table>
        </div>

        <div className="pagination">
          <span className="muted">
            {total === 0 ? "—" : `${page * PAGE_SIZE + 1}–${Math.min((page + 1) * PAGE_SIZE, total)} de ${total}`}
          </span>
          <div className="pagination-controls">
            <button type="button" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
              ‹ ANTERIOR
            </button>
            <span>
              PÁGINA {page + 1} / {pages}
            </span>
            <button type="button" disabled={page >= pages - 1} onClick={() => setPage((p) => Math.min(pages - 1, p + 1))}>
              PRÓXIMA ›
            </button>
          </div>
        </div>
      </div>

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
            setPage(0);
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
            setPage(0);
          }}
        />
      )}

      {filtersOpen && (
        <FiltersModal
          title="FILTROS · RELATÓRIO OPERACIONAL"
          value={filters}
          empty={EMPTY_FILTERS}
          onClose={() => setFiltersOpen(false)}
          onApply={(f) => {
            setFilters(f);
            resetPage();
            setFiltersOpen(false);
          }}
        >
          {(d, set) => (
            <>
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
                <span>NATUREZA</span>
                <Select
                  value={d.nature}
                  onChange={(v) => set({ nature: v })}
                  options={[
                    { value: "", label: "TODAS AS NATUREZAS" },
                    ...(facets?.natures ?? []).map((n) => ({ value: n, label: n })),
                  ]}
                />
              </div>
              <div className="form-field">
                <span>PARTICIPAÇÃO DA INTELIGÊNCIA</span>
                <div className="seg-row">
                  {(
                    [
                      ["", "TODAS"],
                      ["1", "COM"],
                      ["0", "SEM"],
                    ] as const
                  ).map(([v, label]) => (
                    <button
                      key={v}
                      type="button"
                      className={"seg-btn" + (d.intel === v ? " seg-btn--on" : "")}
                      onClick={() => set({ intel: v })}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              </div>
            </>
          )}
        </FiltersModal>
      )}

      {openId && <OpsOccurrenceDrawer occurrenceId={openId} onClose={() => setOpenId(null)} onChanged={reload} />}
    </div>
  );
}

function Row({ row, onOpen }: { row: OpsOccurrenceRow; onOpen: () => void }) {
  const accused = row.accused_names;
  return (
    <tr onClick={onOpen} className="row-clickable">
      <td style={{ whiteSpace: "nowrap" }}>
        {formatBRDate(row.occurred_on)}
        {opsTimeRange(row) && <span className="muted"> · {opsTimeRange(row)}</span>}
      </td>
      <td style={{ color: "var(--fg-0)" }}>
        <div className="ops-natures-cell" title={row.natures.join("\n")}>
          {row.natures.map((n) => (
            <div key={n}>{n}</div>
          ))}
        </div>
        {row.intel_participation && <IntelPill />}
      </td>
      <td>
        <div style={{ color: "var(--fg-0)" }}>{[row.cia, row.pel].filter(Boolean).join(" · ") || "—"}</div>
        <div className="muted" style={{ fontSize: 11.5 }}>
          {row.base_city}
        </div>
      </td>
      <td>
        <div style={{ color: "var(--fg-0)" }}>{row.place_neighborhood || "—"}</div>
        <div className="muted" style={{ fontSize: 11.5 }}>
          {row.place_city}
        </div>
      </td>
      <td className="muted">{row.teams || "—"}</td>
      <td className="muted mono">{row.ciops_record || "—"}</td>
      <td className="muted">
        <span className="ops-seizures">
          {row.weapon_count > 0 && (
            <span title="Armas">
              <Crosshair size={12} strokeWidth={1.6} /> {row.weapon_count}
            </span>
          )}
          {row.drug_count > 0 && (
            <span title="Drogas">
              <Pill size={12} strokeWidth={1.6} /> {row.drug_count}
            </span>
          )}
          {row.vehicle_count > 0 && (
            <span title="Veículos">
              <Car size={12} strokeWidth={1.6} /> {row.vehicle_count}
            </span>
          )}
          {row.weapon_count + row.drug_count + row.vehicle_count === 0 && "—"}
        </span>
      </td>
      <td>
        {accused.length === 0 ? (
          <span className="muted">—</span>
        ) : (
          <div className="ops-accused-cell" title={accused.join("\n")}>
            <span style={{ color: "var(--fg-0)" }}>{accused[0]}</span>
            {accused.length > 1 && <span className="muted"> +{accused.length - 1}</span>}
          </div>
        )}
      </td>
    </tr>
  );
}
