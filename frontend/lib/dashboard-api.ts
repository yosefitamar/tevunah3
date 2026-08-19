import { api } from "./api";
import type { IncidentMeans, IncidentType } from "./incidents-api";
import type { ReportStatus } from "./reports-api";
import type { EntityKind } from "./entities-types";

// Painel operacional. Um único GET devolve todos os blocos que o solicitante
// pode ver — cada bloco vem ausente quando falta a permissão de leitura do
// módulo (incident.read, report.read, informe.read, entity.list), então o
// componente checa presença em vez de repetir o gating de permissão.

export type DashPeriod = { from: string; to: string };

export type DashFacet = {
  name: string;
  /** Só nos bairros: o município desambigua homônimos entre cidades. */
  city?: string;
  count: number;
};

export type DashMonth = {
  month: string; // YYYY-MM
  homicidio: number;
  apreensao: number;
  prisao: number;
};

export type DashIncidents = {
  by_type: Record<IncidentType, number>;
  prev_by_type: Record<IncidentType, number>;
  total: number;
  prev_total: number;
  series: DashMonth[];
  means: DashFacet[];
  cities: DashFacet[];
  neighborhoods: DashFacet[];
  /** Ocorrências do período com coordenadas — cobertura do mapa do crime. */
  geocoded: number;
};

export type DashReports = {
  by_status: Record<ReportStatus, number>;
  created: number;
  diffused: number;
  prev_diffused: number;
};

export type DashInformes = {
  total: number;
  created: number;
  prev: number;
};

export type DashEntities = {
  by_kind: Record<EntityKind, number>;
  created: number;
  prev: number;
  deceased: number;
};

export type Dashboard = {
  period: DashPeriod;
  /** Ausente quando o recorte é aberto — sem base, não há variação. */
  previous?: DashPeriod;
  series_period: DashPeriod;
  incidents?: DashIncidents;
  reports?: DashReports;
  informes?: DashInformes;
  entities?: DashEntities;
};

export type GetDashboardOpts = {
  date_from?: string;
  date_to?: string;
  /** true = acervo inteiro, sem recorte temporal. */
  all?: boolean;
};

export function getDashboard(opts: GetDashboardOpts = {}) {
  const qs = new URLSearchParams();
  if (opts.all) qs.set("all", "1");
  else {
    if (opts.date_from) qs.set("date_from", opts.date_from);
    if (opts.date_to) qs.set("date_to", opts.date_to);
  }
  const q = qs.toString();
  return api<Dashboard>(`/api/dashboard${q ? `?${q}` : ""}`);
}
