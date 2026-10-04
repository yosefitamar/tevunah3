"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Plus, Search, ShieldAlert, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import {
  INCIDENT_MEANS_LABEL,
  INCIDENT_MEANS_SHORT,
  INCIDENT_TYPE_LABEL,
  INCIDENT_TYPE_PILL,
  type IncidentType,
} from "@/lib/incidents-api";
import {
  OCCURRENCE_CATEGORIES,
  OCCURRENCE_CATEGORY_LABEL,
  listOccurrenceLocations,
  listOccurrences,
  type OccurrenceCategory,
  type OccurrenceRow,
  type OccurrencesList,
} from "@/lib/occurrences-api";
import { canCreateIncidents, canReadIncidents, canReadOpsReports } from "@/lib/permissions";
import { useIncidentLocations } from "@/lib/useIncidentLocations";
import { ALL_PERIOD, periodBounds, type PeriodSelection } from "@/lib/period";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import SortHeader, { type SortState } from "../shared/SortHeader";
import IncidentFiltersModal, {
  incidentFilterCount,
  incidentFilterSummary,
  type IncidentFilters,
} from "../shared/IncidentFiltersModal";
import PeriodButton from "../shared/PeriodButton";
import { FiltersButton } from "../shared/FiltersModal";
import OpsOccurrenceDrawer from "../operacional/OpsOccurrenceDrawer";
import CreateOcorrenciaModal from "./CreateOcorrenciaModal";
import OcorrenciaDrawer from "./OcorrenciaDrawer";

const PAGE_SIZE = 25;

// A listagem é o acervo inteiro: nasce sem recorte nenhum. (O mapa parte de
// CVLI no mês atual porque é uma leitura territorial, não um índice.)
const DEFAULT_FILTERS: IncidentFilters = {
  type: "",
  means: "",
  city: "",
  neighborhood: "",
  source: "",
};

// Tipos do cadastro manual que caem na aba PRODUTIVIDADE (homicídio tem a
// própria aba). O primeiro é o tipo com que NOVA OCORRÊNCIA abre nela.
const PRODUCTIVITY_TYPES: IncidentType[] = ["apreensao", "prisao"];

// O que está aberto: o cadastro manual ou a ocorrência do relatório
// operacional. Linha com as duas fontes abre o cadastro, que leva ao relatório.
type OpenTarget = { kind: "incident" | "ops"; id: string };

/**
 * Ocorrências: o cadastro manual (CVLI e o que o analista registra) e o que
 * foi importado do relatório operacional. A ficha CIOPS é a identidade —
 * cadastro e relatório com a mesma ficha são uma linha.
 *
 * Duas abas, porque são duas leituras: PRODUTIVIDADE (prisões, apreensões e
 * tudo o que a tropa registrou no relatório operacional) e HOMICÍDIOS (o
 * cadastro de CVLI). Busca, período e território valem para as duas; tipo,
 * origem e meio utilizado são recortes da aba e zeram na troca.
 */
export default function OcorrenciasScreen() {
  const { user: me } = useAuth();
  const [tab, setTab] = useState<OccurrenceCategory>("produtividade");
  const [data, setData] = useState<OccurrencesList | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [period, setPeriod] = useState<PeriodSelection>(ALL_PERIOD);
  const [filters, setFilters] = useState<IncidentFilters>(DEFAULT_FILTERS);
  const [showFilters, setShowFilters] = useState(false);
  // Busca livre fica fora do modal: é o gesto mais frequente ("cadê a
  // ocorrência do fulano?"), não um recorte que se configura uma vez.
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<SortState>({ field: "occurred_on", dir: "desc" });
  const [showCreate, setShowCreate] = useState(false);
  const [open, setOpen] = useState<OpenTarget | null>(null);

  // Cada fonte tem a sua permissão; o servidor recorta a listagem pelo que o
  // usuário lê.
  const canRead = canReadIncidents(me) || canReadOpsReports(me);
  const canCreate = canCreateIncidents(me);
  const { cities, neighborhoodsOf } = useIncidentLocations(listOccurrenceLocations);

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
      const res = await listOccurrences({
        category: tab,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        source: filters.source || undefined,
        type: filters.type || undefined,
        means: filters.means || undefined,
        city: filters.city || undefined,
        neighborhood: filters.neighborhood || undefined,
        date_from: bounds.from || undefined,
        date_to: bounds.to || undefined,
        search: debouncedSearch || undefined,
        sort_by: (sort?.field as "occurred_on" | "type") || undefined,
        sort_dir: sort?.dir,
      });
      setData(res);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar");
    } finally {
      setLoading(false);
    }
  }, [
    canRead,
    tab,
    filters.source,
    filters.type,
    filters.means,
    filters.city,
    filters.neighborhood,
    bounds.from,
    bounds.to,
    debouncedSearch,
    page,
    sort,
  ]);

  useEffect(() => {
    reload();
  }, [reload]);

  const activeCount = incidentFilterCount(filters, DEFAULT_FILTERS);
  const filterSummary = incidentFilterSummary(filters, false);
  const homicides = tab === "homicidios";

  function switchTab(next: OccurrenceCategory) {
    if (next === tab) return;
    setTab(next);
    setPage(0);
    // Tipo, origem e meio pertencem à aba; território fica.
    setFilters((f) => ({ ...f, type: "", means: "", source: "" }));
    // A aba de homicídios não tem a coluna de tipo para ordenar.
    setSort((s) => (s?.field === "type" ? { field: "occurred_on", dir: "desc" } : s));
    // Some com as linhas da outra aba já na troca — as colunas mudam.
    setData((d) => (d ? { ...d, items: [], total: 0 } : d));
    setLoading(true);
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

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="screen-fill">
      <div className="toolbar">
        <div className="tabs">
          {OCCURRENCE_CATEGORIES.map((c) => (
            <button
              key={c}
              type="button"
              className={"tab" + (tab === c ? " tab-active" : "")}
              onClick={() => switchTab(c)}
            >
              {OCCURRENCE_CATEGORY_LABEL[c]}
              {data?.counts && <span className="muted"> {data.counts[c]}</span>}
            </button>
          ))}
        </div>
        <div className="toolbar-search">
          <Search size={14} strokeWidth={1.6} />
          <input
            type="text"
            value={search}
            onChange={(e) => {
              // A página 3 do recorte anterior não existe mais depois que o
              // termo muda.
              setSearch(e.target.value);
              setPage(0);
            }}
            placeholder="buscar por nome, CPF, natureza, descrição ou ficha CIOPS…"
          />
          {search && (
            <button
              type="button"
              className="action-btn"
              onClick={() => {
                setSearch("");
                setPage(0);
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
            setPage(0);
          }}
        />
        <FiltersButton count={activeCount} onClick={() => setShowFilters(true)} />
        <span className="muted filter-summary" title={filterSummary}>
          {filterSummary}
        </span>
        <div style={{ marginLeft: "auto" }} />
        {canCreate && (
          <button type="button" className="btn btn-primary" onClick={() => setShowCreate(true)}>
            <Plus size={14} strokeWidth={2} /> NOVA OCORRÊNCIA
          </button>
        )}
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                {/* Só o essencial para achar a ocorrência; o resto (descrição,
                    envolvidos, origem) está na ficha, a um clique. Em cada
                    aba uma coluna fica sem largura fixa e absorve a sobra. */}
                {!homicides && <SortHeader field="type" label="TIPO / NATUREZA" sort={sort} onChange={setSort} />}
                <SortHeader field="occurred_on" label="DATA / HORA" sort={sort} onChange={setSort} width={170} />
                {homicides && <th style={{ width: 160 }}>MEIO</th>}
                <th style={{ width: 170 }}>FICHA CIOPS</th>
                <th style={homicides ? undefined : { width: 320 }}>LOCAL</th>
                <th style={{ width: 170 }}>SITUAÇÃO</th>
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={5} className="muted" style={{ textAlign: "center", padding: 32 }}>
                    // CARREGANDO…
                  </td>
                </tr>
              )}
              {!loading && items.length === 0 && (
                <tr>
                  <td colSpan={5} className="muted" style={{ textAlign: "center", padding: 32 }}>
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

      {showFilters && (
        <IncidentFiltersModal
          title={"FILTROS · " + OCCURRENCE_CATEGORY_LABEL[tab]}
          value={filters}
          defaults={DEFAULT_FILTERS}
          cities={cities}
          neighborhoodsOf={neighborhoodsOf}
          // Homicídios: todo registro é do cadastro e do mesmo tipo — sobra o
          // meio utilizado. Produtividade: origem e tipo; meio é campo de CVLI.
          withSource={!homicides}
          types={homicides ? [] : PRODUCTIVITY_TYPES}
          withMeans={homicides}
          onApply={(f) => {
            setFilters(f);
            setPage(0);
            setShowFilters(false);
          }}
          onClose={() => setShowFilters(false)}
        />
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

      {open?.kind === "incident" && (
        <OcorrenciaDrawer incidentId={open.id} onClose={() => setOpen(null)} onChanged={reload} />
      )}
      {open?.kind === "ops" && (
        <OpsOccurrenceDrawer occurrenceId={open.id} onClose={() => setOpen(null)} onChanged={reload} />
      )}
    </div>
  );
}

function Row({ row, homicides, onOpen }: { row: OccurrenceRow; homicides: boolean; onOpen: () => void }) {
  const natures = row.natures.join(" · ");
  return (
    <tr onClick={onOpen} className="row-clickable">
      {!homicides && (
        <td>
          {row.type ? (
            <>
              <span className={"pill " + INCIDENT_TYPE_PILL[row.type]}>{INCIDENT_TYPE_LABEL[row.type]}</span>
              {natures && (
                <div className="muted tbl-nature" title={natures}>
                  {natures}
                </div>
              )}
            </>
          ) : (
            <div className="tbl-nature" style={{ color: "var(--fg-0)" }} title={natures}>
              {natures || "—"}
            </div>
          )}
        </td>
      )}
      <td style={{ whiteSpace: "nowrap" }}>
        {formatBRDate(row.occurred_on)}
        {row.time ? <span className="muted"> · {row.time}</span> : null}
      </td>
      {homicides && (
        <td
          className={row.means ? undefined : "muted"}
          title={row.means ? INCIDENT_MEANS_LABEL[row.means] : undefined}
        >
          {row.means ? INCIDENT_MEANS_SHORT[row.means] : "—"}
        </td>
      )}
      <td className="mono">{row.ciops_record || "—"}</td>
      <td>
        {row.city || row.neighborhood ? (
          <>
            <div style={{ color: "var(--fg-0)" }}>{row.neighborhood || "—"}</div>
            <div className="muted" style={{ fontSize: 11.5 }}>
              {row.city}
            </div>
          </>
        ) : (
          <span className="muted">—</span>
        )}
      </td>
      <td>
        <StatusBadge row={row} />
      </td>
    </tr>
  );
}

// Situação da ocorrência: quantas pendências tem para o envio ao SIPOM, como
// na fila de envio — âmbar quando alguma impede o envio; neutro quando são só
// avisos — ou VERIFICADA quando não há nenhuma. O detalhe de cada pendência
// fica no title e na ficha.
function StatusBadge({ row }: { row: OccurrenceRow }) {
  if (row.pendencias.length === 0) return <span className="pill active">VERIFICADA</span>;
  const blocking = row.pendencias.filter((p) => p.blocking);
  const shown = blocking.length > 0 ? blocking : row.pendencias;
  const noun = blocking.length > 0 ? "PENDÊNCIA" : "AVISO";
  // As que impedem o envio primeiro.
  const title = [...blocking, ...row.pendencias.filter((p) => !p.blocking)].map((p) => "• " + p.label).join("\n");
  return (
    <span className={"pill " + (blocking.length > 0 ? "hold" : "cold")} title={title}>
      {shown.length} {noun}
      {shown.length > 1 ? "S" : ""}
    </span>
  );
}
