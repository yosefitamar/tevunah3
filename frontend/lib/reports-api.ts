import { api } from "./api";

export type ReportKind = "interno";
export type ReportStatus = "criado" | "difundido" | "arquivado";

export const REPORT_STATUS_LABEL: Record<ReportStatus, string> = {
  criado: "CRIADO",
  difundido: "DIFUNDIDO",
  arquivado: "ARQUIVADO",
};

// Para reuso com a CSS .pill (active/hold/cold).
export const REPORT_STATUS_PILL: Record<ReportStatus, string> = {
  criado: "hold",
  difundido: "active",
  arquivado: "cold",
};

export type Report = {
  id: string;
  kind: ReportKind;
  status: ReportStatus;
  number?: string;
  seq?: number;
  year?: number;
  doc_date: string;        // YYYY-MM-DD
  subject: string;
  origin: string;
  diffusion: string;
  prior_diffusion: string;
  reference: string;
  attachments: string;
  body_html: string;
  created_at: string;
  created_by: string;
  updated_at: string;
  updated_by?: string;
  diffused_at?: string;
  diffused_by?: string;
  archived_at?: string;
  archived_by?: string;
};

export type Qualification = {
  id: string;
  report_id: string;
  ord: number;
  kind: "militar" | "civil";
  entity_id?: string;
  data: Record<string, unknown>;
  source: string;
  consulted_at?: string;
  created_at: string;
};

export type ReportsList = {
  items: Report[];
  total: number;
  limit: number;
  offset: number;
};

export type ListReportsOpts = {
  limit?: number;
  offset?: number;
  status?: "" | ReportStatus;
  search?: string;
};

function qs(opts: ListReportsOpts): string {
  const p = new URLSearchParams();
  if (opts.limit) p.set("limit", String(opts.limit));
  if (opts.offset) p.set("offset", String(opts.offset));
  if (opts.status) p.set("status", opts.status);
  if (opts.search) p.set("search", opts.search);
  const s = p.toString();
  return s ? "?" + s : "";
}

export function listReports(opts: ListReportsOpts = {}) {
  return api<ReportsList>(`/api/reports${qs(opts)}`);
}

export type NewReportInput = {
  kind?: ReportKind;
  doc_date?: string;
  subject?: string;
  origin?: string;
  diffusion?: string;
};

export function createReport(input: NewReportInput) {
  return api<{ report: Report }>(`/api/reports`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getReport(id: string) {
  return api<{ report: Report; qualifications: Qualification[] }>(
    `/api/reports/${encodeURIComponent(id)}`,
  );
}

export type UpdateReportInput = {
  doc_date?: string;
  subject?: string;
  origin?: string;
  diffusion?: string;
  prior_diffusion?: string;
  reference?: string;
  attachments?: string;
  body_html?: string;
};

export function updateReport(id: string, input: UpdateReportInput) {
  return api<{ report: Report }>(`/api/reports/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function diffuseReport(id: string) {
  return api<{ report: Report }>(
    `/api/reports/${encodeURIComponent(id)}/diffuse`,
    { method: "POST", body: JSON.stringify({}) },
  );
}

export function archiveReport(id: string) {
  return api<{ report: Report }>(
    `/api/reports/${encodeURIComponent(id)}/archive`,
    { method: "POST", body: JSON.stringify({}) },
  );
}

export type NewQualificationInput = {
  kind: "militar" | "civil";
  entity_id?: string;
  data: Record<string, unknown>;
  source?: string;
  consulted_at?: string;
};

export function addQualification(reportId: string, input: NewQualificationInput) {
  return api<{ qualification: Qualification }>(
    `/api/reports/${encodeURIComponent(reportId)}/qualifications`,
    { method: "POST", body: JSON.stringify(input) },
  );
}

export function deleteQualification(reportId: string, qualifId: string) {
  return api<void>(
    `/api/reports/${encodeURIComponent(reportId)}/qualifications/${encodeURIComponent(qualifId)}`,
    { method: "DELETE" },
  );
}
