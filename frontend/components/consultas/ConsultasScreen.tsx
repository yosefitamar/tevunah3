"use client";

import { useCallback, useEffect, useState } from "react";
import { Plus, Search, ShieldAlert } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  listConsultaSystems,
  listConsultas,
  type Consulta,
  type ConsultasList,
} from "@/lib/consultas-api";
import {
  canCreateConsultas,
  canReadAllConsultas,
  canReadConsultas,
} from "@/lib/permissions";
import { DEFAULT_PERIOD, periodQuery, type PeriodSelection } from "@/lib/period";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import PeriodButton from "../shared/PeriodButton";
import SortHeader, { type SortState } from "../shared/SortHeader";
import ConsultaModal from "./ConsultaModal";

const PAGE_SIZE = 25;

const ELLIPSIS = {
  overflow: "hidden",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
} as const;

export default function ConsultasScreen() {
  const { user: me } = useAuth();
  const [data, setData] = useState<ConsultasList | null>(null);
  const [systems, setSystems] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [period, setPeriod] = useState<PeriodSelection>(DEFAULT_PERIOD);
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "performed_at", dir: "desc" });
  const [showCreate, setShowCreate] = useState(false);
  const [open, setOpen] = useState<Consulta | null>(null);

  const canRead = canReadConsultas(me);
  const canCreate = canCreateConsultas(me);
  // Quem vê os registros de todos ganha a coluna do agente que registrou.
  const seesAll = canReadAllConsultas(me);
  const cols = seesAll ? 6 : 5;

  const reload = useCallback(async () => {
    if (!canRead) return;
    setLoading(true);
    setError(null);
    try {
      const pq = periodQuery(period);
      const res = await listConsultas({
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        search: search.trim() || undefined,
        date_from: pq.date_from,
        date_to: pq.date_to,
        sort_by: sort?.field,
        sort_dir: sort?.dir,
      });
      setData(res);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [canRead, page, search, period, sort]);

  const reloadSystems = useCallback(async () => {
    if (!canRead) return;
    try {
      setSystems((await listConsultaSystems()).items);
    } catch {
      // Sem sugestões o campo segue aceitando texto livre.
    }
  }, [canRead]);

  useEffect(() => {
    reload();
  }, [reload]);

  useEffect(() => {
    reloadSystems();
  }, [reloadSystems]);

  if (!canRead) {
    return (
      <div className="placeholder">
        <ShieldAlert size={36} strokeWidth={1.2} />
        <div className="ph-tag">// ACESSO RESTRITO</div>
        <div className="ph-ttl">SEM PERMISSÃO DE LEITURA DO REGISTRO DE CONSULTAS</div>
        <div className="ph-sub">Contate o administrador.</div>
      </div>
    );
  }

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  function onSaved() {
    setShowCreate(false);
    setOpen(null);
    reload();
    reloadSystems();
  }

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
            placeholder={
              seesAll
                ? "buscar por sistema / alvo / solicitante / motivo / agente…"
                : "buscar por sistema / alvo / solicitante / motivo…"
            }
          />
        </div>
        <PeriodButton
          value={period}
          onChange={(p) => {
            setPeriod(p);
            setPage(0);
          }}
          title="PERÍODO · DATA DA CONSULTA"
        />
        <div style={{ marginLeft: "auto" }} />
        {canCreate && (
          <button type="button" className="btn btn-primary" onClick={() => setShowCreate(true)}>
            <Plus size={14} strokeWidth={2} /> REGISTRAR CONSULTA
          </button>
        )}
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="performed_at" label="DATA / HORA" sort={sort} onChange={setSort} width={140} />
                <SortHeader field="system" label="SISTEMA" sort={sort} onChange={setSort} width={170} />
                <SortHeader field="target" label="ALVO" sort={sort} onChange={setSort} width={220} />
                <SortHeader field="requester" label="SOLICITANTE" sort={sort} onChange={setSort} width={220} />
                <th>MOTIVO</th>
                {seesAll && (
                  <SortHeader field="author" label="AGENTE" sort={sort} onChange={setSort} width={130} />
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
                    // NENHUMA CONSULTA REGISTRADA NO PERÍODO
                  </td>
                </tr>
              )}
              {!loading &&
                items.map((c) => (
                  <tr key={c.id} className="row-clickable" onClick={() => setOpen(c)}>
                    <td style={{ whiteSpace: "nowrap" }}>{formatBR(c.performed_at)}</td>
                    <td style={{ color: "var(--fg-0)", maxWidth: 170, ...ELLIPSIS }}>{c.system}</td>
                    <td style={{ maxWidth: 220, ...ELLIPSIS }}>{c.target.toUpperCase()}</td>
                    <td style={{ maxWidth: 220, ...ELLIPSIS }}>
                      {c.requester.toUpperCase()}
                      {c.requester_org && <span className="muted"> · {c.requester_org.toUpperCase()}</span>}
                    </td>
                    <td className="muted" style={{ maxWidth: 360, ...ELLIPSIS }}>
                      {c.reason}
                    </td>
                    {seesAll && <td className="muted">{c.created_by_code}</td>}
                  </tr>
                ))}
            </tbody>
          </table>
        </div>

        <div className="pagination">
          <span className="muted">
            {total} consulta{total === 1 ? "" : "s"}
            {!seesAll && " · SÓ OS SEUS REGISTROS"}
          </span>
          <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
            <button className="btn" disabled={page <= 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
              ‹ ANTERIOR
            </button>
            <span className="muted">{page + 1} / {pages}</span>
            <button className="btn" disabled={page >= pages - 1} onClick={() => setPage((p) => p + 1)}>
              PRÓXIMA ›
            </button>
          </div>
        </div>
      </div>

      {showCreate && (
        <ConsultaModal systems={systems} onClose={() => setShowCreate(false)} onSaved={onSaved} />
      )}

      {open && (
        <ConsultaModal
          key={open.id}
          consulta={open}
          systems={systems}
          onClose={() => setOpen(null)}
          onSaved={onSaved}
          onChanged={reload}
        />
      )}
    </div>
  );
}
