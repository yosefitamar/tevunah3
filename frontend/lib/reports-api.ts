import { api } from "./api";

export type ReportKind = "interno";
export type ReportStatus = "criado" | "difundido" | "arquivado";
export type ReportConfidentiality = "sigiloso" | "secreto" | "ultrassecreto";
export type ReportVisibility = "aberto" | "restrito";

export const CONFIDENTIALITY_LABEL: Record<ReportConfidentiality, string> = {
  sigiloso: "SIGILOSO",
  secreto: "SECRETO",
  ultrassecreto: "ULTRASSECRETO",
};

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
  confidentiality: ReportConfidentiality;
  visibility: ReportVisibility;
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
  confidentiality?: ReportConfidentiality;
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
  confidentiality?: ReportConfidentiality;
  body_html?: string;
};

export type ReportViewer = {
  user_id: string;
  user_code: string;
  display_name: string;
  granted_by: string;
  granted_at: string;
};

export function listReportViewers(id: string) {
  return api<{ viewers: ReportViewer[] }>(
    `/api/reports/${encodeURIComponent(id)}/viewers`,
  );
}

export function setReportViewers(id: string, userIDs: string[]) {
  return api<{ viewers: ReportViewer[] }>(
    `/api/reports/${encodeURIComponent(id)}/viewers`,
    { method: "PUT", body: JSON.stringify({ user_ids: userIDs }) },
  );
}

export function setReportVisibility(id: string, visibility: ReportVisibility) {
  return api<{ report: Report }>(
    `/api/reports/${encodeURIComponent(id)}/visibility`,
    { method: "PUT", body: JSON.stringify({ visibility }) },
  );
}

// Lookup leve de usuários pra picker de viewers. Não exige user.list —
// qualquer usuário autenticado pode consultar.
export type UserLookup = { id: string; code: string; display_name: string };
export function lookupUsers(search: string) {
  const qs = search ? `?search=${encodeURIComponent(search)}` : "";
  return api<{ items: UserLookup[] }>(`/api/users/lookup${qs}`);
}

export function updateReport(id: string, input: UpdateReportInput) {
  return api<{ report: Report }>(`/api/reports/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function destroyReport(id: string) {
  return api<void>(`/api/reports/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function diffuseReport(id: string) {
  return api<{ report: Report }>(
    `/api/reports/${encodeURIComponent(id)}/diffuse`,
    { method: "POST", body: JSON.stringify({}) },
  );
}

// Baixa o PDF do relatório. Não passa pelo helper `api` porque o corpo é
// binário (application/pdf) — fazemos fetch direto e devolvemos o Blob.
export async function downloadReportPDF(
  id: string,
): Promise<{ blob: Blob; filename: string; downloadID: string; sha256: string }> {
  const res = await fetch(`/api/reports/${encodeURIComponent(id)}/download`, {
    method: "GET",
    credentials: "include",
  });
  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      if (body && typeof body.message === "string") msg = body.message;
    } catch {
      // sem corpo JSON
    }
    const err = new Error(msg) as Error & { status: number };
    err.status = res.status;
    throw err;
  }
  // Content-Disposition: attachment; filename="RI_07_2026.pdf"
  let filename = `relatorio-${id}.pdf`;
  const cd = res.headers.get("Content-Disposition") || "";
  const m = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(cd);
  if (m && m[1]) filename = decodeURIComponent(m[1]);
  const downloadID = res.headers.get("X-Download-ID") ?? "";
  const sha256 = res.headers.get("X-PDF-SHA256") ?? "";
  const blob = await res.blob();
  return { blob, filename, downloadID, sha256 };
}

export function undiffuseReport(id: string, reason: string) {
  return api<{ report: Report }>(
    `/api/reports/${encodeURIComponent(id)}/undiffuse`,
    { method: "POST", body: JSON.stringify({ reason }) },
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

// URL para servir a foto da qualificação (militar). O backend manda no-store,
// então o `v` é só pra forçar o navegador a refetchar entre uploads na mesma view.
export function qualificationPhotoURL(
  reportId: string,
  qualifId: string,
  v?: string | number,
): string {
  const bust = v != null ? `?v=${encodeURIComponent(String(v))}` : "";
  return `/api/reports/${encodeURIComponent(reportId)}/qualifications/${encodeURIComponent(qualifId)}/photo${bust}`;
}

export async function uploadQualificationPhoto(
  reportId: string,
  qualifId: string,
  file: File,
): Promise<Qualification> {
  const fd = new FormData();
  fd.append("photo", file);
  const res = await fetch(
    `/api/reports/${encodeURIComponent(reportId)}/qualifications/${encodeURIComponent(qualifId)}/photo`,
    { method: "POST", credentials: "include", body: fd },
  );
  let body: {
    success: boolean;
    data?: { qualification: Qualification };
    message?: string;
  } = { success: false };
  try {
    body = await res.json();
  } catch {
    // sem corpo
  }
  if (!res.ok) {
    const err = new Error(body.message ?? `HTTP ${res.status}`) as Error & {
      status: number;
    };
    err.status = res.status;
    throw err;
  }
  return (body.data as { qualification: Qualification }).qualification;
}

export function deleteQualificationPhoto(reportId: string, qualifId: string) {
  return api<void>(
    `/api/reports/${encodeURIComponent(reportId)}/qualifications/${encodeURIComponent(qualifId)}/photo`,
    { method: "DELETE" },
  );
}
