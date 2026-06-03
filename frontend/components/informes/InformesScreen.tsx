"use client";

import { useCallback, useEffect, useState } from "react";
import { Plus, Search, ShieldAlert } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  listInformes,
  type InformesList,
} from "@/lib/informes-api";
import { canCreateInformes, canReadInformes } from "@/lib/permissions";
import { clearanceLabel } from "@/lib/types";
import { formatBR, formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import CreateInformeModal from "./CreateInformeModal";
import InformeDrawer from "./InformeDrawer";

const PAGE_SIZE = 25;

export default function InformesScreen() {
  const { user: me } = useAuth();
  const [data, setData] = useState<InformesList | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "occurred_on", dir: "desc" });
  const [showCreate, setShowCreate] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);

  const canRead = canReadInformes(me);
  const canCreate = canCreateInformes(me);

  const reload = useCallback(async () => {
    if (!canRead) return;
    setLoading(true);
    setError(null);
    try {
      const res = await listInformes({
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        search: search.trim() || undefined,
        sort_by: sort?.field,
        sort_dir: sort?.dir,
      });
      setData(res);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [canRead, page, search, sort]);

  useEffect(() => {
    reload();
  }, [reload]);

  if (!canRead) {
    return (
      <div className="placeholder">
        <ShieldAlert size={36} strokeWidth={1.2} />
        <div className="ph-tag">// ACESSO RESTRITO</div>
        <div className="ph-ttl">SEM PERMISSÃO DE LEITURA DE INFORMES</div>
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
            placeholder="buscar por onde / como / descrição…"
          />
        </div>
        <div style={{ marginLeft: "auto" }} />
        {canCreate && (
          <button type="button" className="btn btn-primary" onClick={() => setShowCreate(true)}>
            <Plus size={14} strokeWidth={2} /> NOVO INFORME
          </button>
        )}
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <SortHeader field="occurred_on" label="QUANDO" sort={sort} onChange={setSort} width={120} />
                <SortHeader field="location" label="ONDE" sort={sort} onChange={setSort} width={180} />
                <th>DESCRIÇÃO</th>
                <SortHeader field="author" label="AGENTE" sort={sort} onChange={setSort} width={150} />
                <SortHeader field="required_clearance" label="ACESSO" sort={sort} onChange={setSort} width={95} />
                <SortHeader field="created_at" label="CRIADO" sort={sort} onChange={setSort} width={140} />
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
                    // NENHUM INFORME ENCONTRADO
                  </td>
                </tr>
              )}
              {!loading &&
                items.map((inf) => (
                  <tr
                    key={inf.id}
                    className="row-clickable"
                    onClick={() => setOpenId(inf.id)}
                  >
                    <td style={{ whiteSpace: "nowrap" }}>{formatBRDate(inf.occurred_on)}</td>
                    <td style={{ color: "var(--fg-0)" }}>
                      {inf.location ? inf.location.toUpperCase() : "—"}
                    </td>
                    <td className="muted" style={{ maxWidth: 360, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {inf.description || "—"}
                    </td>
                    <td className="muted">{inf.created_by_code}</td>
                    <td className="mono" style={{ fontSize: 11 }}>{clearanceLabel(inf.required_clearance)}</td>
                    <td className="muted">{formatBR(inf.created_at)}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>

        <div className="pagination">
          <span className="muted">{total} informe{total === 1 ? "" : "s"}</span>
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
        <CreateInformeModal
          onClose={() => setShowCreate(false)}
          onCreated={(id) => {
            setShowCreate(false);
            setOpenId(id);
            reload();
          }}
        />
      )}

      {openId && (
        <InformeDrawer
          informeId={openId}
          onClose={() => setOpenId(null)}
          onChanged={() => reload()}
        />
      )}
    </div>
  );
}
