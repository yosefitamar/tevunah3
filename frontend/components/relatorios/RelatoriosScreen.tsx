"use client";

import { useCallback, useEffect, useState } from "react";
import { FileText, Plus, Search, ShieldAlert } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  REPORT_STATUS_LABEL,
  REPORT_STATUS_PILL,
  listReports,
  type Report,
  type ReportStatus,
  type ReportsList,
} from "@/lib/reports-api";
import { canCreateReports, canReadReports } from "@/lib/permissions";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import Select from "../shared/Select";
import CreateRelatorioModal from "./CreateRelatorioModal";
import RelatorioDrawer from "./RelatorioDrawer";

const PAGE_SIZE = 25;

type Filters = {
  status: "" | ReportStatus;
  search: string;
};

const EMPTY_FILTERS: Filters = { status: "", search: "" };

export default function RelatoriosScreen() {
  const { user: me } = useAuth();
  const [data, setData] = useState<ReportsList | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "doc_date", dir: "desc" });
  const [showCreate, setShowCreate] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);

  const canRead = canReadReports(me);
  const canCreate = canCreateReports(me);

  const reload = useCallback(async () => {
    if (!canRead) return;
    setLoading(true);
    setError(null);
    try {
      const res = await listReports({
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        status: filters.status || undefined,
        search: filters.search.trim() || undefined,
      });
      setData(res);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [canRead, filters, page]);

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
    <div className="screen">
      <div className="toolbar">
        <div className="toolbar-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            value={filters.search}
            onChange={(e) => {
              setFilters({ ...filters, search: e.target.value });
              setPage(0);
            }}
            placeholder="buscar por assunto…"
          />
        </div>
        <Select
          value={filters.status}
          onChange={(v) => {
            setFilters({ ...filters, status: v as "" | ReportStatus });
            setPage(0);
          }}
          className="sel--toolbar"
          placeholder="STATUS · TODOS"
          options={[
            { value: "", label: "STATUS · TODOS" },
            { value: "criado", label: "STATUS · CRIADO" },
            { value: "difundido", label: "STATUS · DIFUNDIDO" },
            { value: "arquivado", label: "STATUS · ARQUIVADO" },
          ]}
        />
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

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="number" label="Nº / ANO" sort={sort} onChange={setSort} width={120} />
                <SortHeader field="status" label="STATUS" sort={sort} onChange={setSort} width={120} />
                <SortHeader field="doc_date" label="DATA" sort={sort} onChange={setSort} width={110} />
                <SortHeader field="subject" label="ASSUNTO" sort={sort} onChange={setSort} />
                <SortHeader field="diffusion" label="DIFUSÃO" sort={sort} onChange={setSort} width={200} />
                <SortHeader field="updated_at" label="ATUALIZADO" sort={sort} onChange={setSort} width={140} />
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={6} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // CARREGANDO…
                  </td>
                </tr>
              )}
              {!loading && items.length === 0 && (
                <tr>
                  <td colSpan={6} className="muted" style={{ textAlign: "center", padding: 32 }}>
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
      <td>{formatBR(report.doc_date)}</td>
      <td style={{ color: "var(--fg-0)" }}>
        {report.subject ? (
          report.subject.toUpperCase()
        ) : (
          <span className="muted">(sem assunto)</span>
        )}
      </td>
      <td className="muted">
        {report.diffusion ? report.diffusion.toUpperCase() : "—"}
      </td>
      <td className="muted">{formatBR(report.updated_at)}</td>
    </tr>
  );
}
