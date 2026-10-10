"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { FileText, Plus, Search, ShieldAlert } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  CONFIDENTIALITY_LABEL,
  REPORT_STATUS_LABEL,
  REPORT_STATUS_PILL,
  listReports,
  listReportYears,
  type Report,
  type ReportStatus,
  type ReportsList,
} from "@/lib/reports-api";
import { canCreateReports, canReadReports } from "@/lib/permissions";
import { clearanceLabel } from "@/lib/types";
import { formatBR } from "@/lib/format";
import { ALL_PERIOD, periodBounds, type PeriodSelection } from "@/lib/period";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import PeriodButton from "../shared/PeriodButton";
import FiltersModal, { FiltersButton } from "../shared/FiltersModal";
import CreateRelatorioModal from "./CreateRelatorioModal";
import RelatorioDrawer from "./RelatorioDrawer";

const PAGE_SIZE = 25;

// Recortes não temporais — modal de FILTROS. O período (data do documento)
// tem o próprio botão; a busca fica na barra.
type Filters = {
  status: "" | ReportStatus;
};

const EMPTY_FILTERS: Filters = { status: "" };

const STATUS_OPTIONS: Array<{ value: "" | ReportStatus; label: string }> = [
  { value: "", label: "TODOS" },
  { value: "criado", label: "CRIADO" },
  { value: "difundido", label: "DIFUNDIDO" },
  { value: "arquivado", label: "ARQUIVADO" },
];

export default function RelatoriosScreen() {
  const { user: me } = useAuth();
  const [data, setData] = useState<ReportsList | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  // Período inicial: ANO ATUAL quando já há RIs no ano, senão TODO O PERÍODO
  // (decidido após listReportYears).
  const [period, setPeriod] = useState<PeriodSelection>(ALL_PERIOD);
  const [yearsLoaded, setYearsLoaded] = useState(false);
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "doc_date", dir: "desc" });
  const [showCreate, setShowCreate] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);

  const canRead = canReadReports(me);
  const canCreate = canCreateReports(me);

  // Decide o período inicial pelos anos com RI. Roda uma vez por montagem.
  useEffect(() => {
    if (!canRead) return;
    let alive = true;
    (async () => {
      try {
        const { years: ys } = await listReportYears();
        if (!alive) return;
        if (ys.includes(new Date().getFullYear())) {
          setPeriod({ kind: "preset", id: "ano_atual" });
        }
      } catch {
        // Sem fatal: fica em TODO O PERÍODO.
      } finally {
        if (alive) setYearsLoaded(true);
      }
    })();
    return () => {
      alive = false;
    };
  }, [canRead]);

  const bounds = useMemo(() => periodBounds(period), [period]);

  const reload = useCallback(async () => {
    if (!canRead) return;
    // Espera o período inicial ser decidido — senão faríamos uma listagem
    // sem recorte e outra logo em seguida com ele.
    if (!yearsLoaded) return;
    setLoading(true);
    setError(null);
    try {
      const res = await listReports({
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        status: filters.status || undefined,
        search: search.trim() || undefined,
        date_from: bounds.from || undefined,
        date_to: bounds.to || undefined,
        sort_by: sort?.field,
        sort_dir: sort?.dir,
      });
      setData(res);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [canRead, yearsLoaded, search, filters, bounds.from, bounds.to, page, sort]);

  useEffect(() => {
    reload();
  }, [reload]);

  if (!canRead) {
    return (
      <div className="placeholder">
        <ShieldAlert size={36} strokeWidth={1.2} />
        <div className="ph-tag">// ACESSO RESTRITO</div>
        <div className="ph-ttl">SEM PERMISSÃO DE LEITURA DE RELATÓRIOS</div>
        <div className="ph-sub">Contate o administrador.</div>
      </div>
    );
  }

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="screen-fill">
      <div className="toolbar">
        <div className="toolbar-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(0);
            }}
            placeholder="buscar por assunto…"
          />
        </div>
        <PeriodButton
          value={period}
          onChange={(p) => {
            setPeriod(p);
            setPage(0);
          }}
          title="PERÍODO · DATA DO DOCUMENTO"
        />
        <FiltersButton count={filters.status ? 1 : 0} onClick={() => setFiltersOpen(true)} />
        <div style={{ marginLeft: "auto" }} />
        {canCreate && (
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => setShowCreate(true)}
          >
            <Plus size={14} strokeWidth={2} /> NOVO RELATÓRIO
          </button>
        )}
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      {filtersOpen && (
        <FiltersModal
          title="FILTROS · RELATÓRIOS"
          value={filters}
          empty={EMPTY_FILTERS}
          width={420}
          onClose={() => setFiltersOpen(false)}
          onApply={(f) => {
            setFilters(f);
            setPage(0);
            setFiltersOpen(false);
          }}
        >
          {(d, set) => (
            <div className="form-field">
              <span>STATUS</span>
              <div className="seg-row">
                {STATUS_OPTIONS.map((o) => (
                  <button
                    key={o.value}
                    type="button"
                    className={"seg-btn" + (d.status === o.value ? " seg-btn--on" : "")}
                    onClick={() => set({ status: o.value })}
                  >
                    {o.label}
                  </button>
                ))}
              </div>
            </div>
          )}
        </FiltersModal>
      )}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="number" label="Nº / ANO" sort={sort} onChange={setSort} width={120} />
                <SortHeader field="status" label="STATUS" sort={sort} onChange={setSort} width={120} />
                <SortHeader field="doc_date" label="DATA" sort={sort} onChange={setSort} width={135} />
                <SortHeader field="subject" label="ASSUNTO" sort={sort} onChange={setSort} />
                <SortHeader field="confidentiality" label="CONFIDENCIALIDADE" sort={sort} onChange={setSort} width={160} />
                <th style={{ width: 95 }}>ACESSO</th>
                <SortHeader field="diffusion" label="DIFUSÃO" sort={sort} onChange={setSort} width={200} />
                <SortHeader field="updated_at" label="ATUALIZADO" sort={sort} onChange={setSort} width={140} />
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
                    // NENHUM RELATÓRIO ENCONTRADO
                  </td>
                </tr>
              )}
              {!loading &&
                items.map((it) => (
                  <Row key={it.id} report={it} onOpen={() => setOpenId(it.id)} />
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
            <button
              type="button"
              disabled={page === 0}
              onClick={() => setPage((p) => Math.max(0, p - 1))}
            >
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

      {showCreate && (
        <CreateRelatorioModal
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            setShowCreate(false);
            setOpenId(id);
            reload();
          }}
        />
      )}

      {openId && (
        <RelatorioDrawer
          reportId={openId}
          onClose={() => setOpenId(null)}
          onChanged={reload}
        />
      )}
    </div>
  );
}

function Row({ report, onOpen }: { report: Report; onOpen: () => void }) {
  return (
    <tr onClick={onOpen} className="row-clickable">
      <td className="muted">
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <FileText size={12} strokeWidth={1.6} />
          {report.number ? (
            <span style={{ color: "var(--fg-0)", fontWeight: 600 }}>
              {report.number}
            </span>
          ) : (
            "s/n"
          )}
        </div>
      </td>
      <td>
        <span className={"pill " + REPORT_STATUS_PILL[report.status]}>
          {REPORT_STATUS_LABEL[report.status]}
        </span>
      </td>
      <td style={{ whiteSpace: "nowrap" }}>{formatBR(report.doc_date)}</td>
      <td style={{ color: "var(--fg-0)" }}>
        {report.subject ? (
          report.subject.toUpperCase()
        ) : (
          <span className="muted">(sem assunto)</span>
        )}
      </td>
      <td>
        <span className={"pill conf-" + report.confidentiality}>
          {CONFIDENTIALITY_LABEL[report.confidentiality]}
        </span>
      </td>
      <td className="mono" style={{ fontSize: 13 }}>
        {clearanceLabel(report.required_clearance)}
      </td>
      <td className="muted">
        {report.diffusion ? report.diffusion.toUpperCase() : "—"}
      </td>
      <td className="muted">{formatBR(report.updated_at)}</td>
    </tr>
  );
}
