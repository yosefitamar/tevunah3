import { api } from "./api";

export type Informe = {
  id: string;
  occurred_on: string; // YYYY-MM-DD (QUANDO)
  location: string; // ONDE
  how: string; // COMO
  description: string; // DESCRIÇÃO
  has_photo: boolean;
  required_clearance: number; // 1..5
  version: number;
  created_at: string;
  created_by: string;
  created_by_code: string;
  created_by_name: string;
  updated_at: string;
};

export type InformesList = {
  items: Informe[];
  total: number;
  limit: number;
  offset: number;
};

export type ListInformesOpts = {
  limit?: number;
  offset?: number;
  search?: string;
  sort_by?: string;
  sort_dir?: "asc" | "desc";
};

function qs(opts: ListInformesOpts): string {
  const p = new URLSearchParams();
  if (opts.limit) p.set("limit", String(opts.limit));
  if (opts.offset) p.set("offset", String(opts.offset));
  if (opts.search) p.set("search", opts.search);
  if (opts.sort_by) p.set("sort_by", opts.sort_by);
  if (opts.sort_dir) p.set("sort_dir", opts.sort_dir);
  const s = p.toString();
  return s ? "?" + s : "";
}

export function listInformes(opts: ListInformesOpts = {}) {
  return api<InformesList>(`/api/informes${qs(opts)}`);
}

export type NewInformeInput = {
  occurred_on?: string;
  location?: string;
  how?: string;
  description?: string;
  required_clearance?: number;
};

export function createInforme(input: NewInformeInput) {
  return api<{ informe: Informe }>(`/api/informes`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getInforme(id: string) {
  return api<{ informe: Informe }>(`/api/informes/${encodeURIComponent(id)}`);
}

export type UpdateInformeInput = Partial<{
  occurred_on: string;
  location: string;
  how: string;
  description: string;
  required_clearance: number;
}>;

export function updateInforme(id: string, input: UpdateInformeInput) {
  return api<{ informe: Informe }>(`/api/informes/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deleteInforme(id: string) {
  return api<void>(`/api/informes/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

// URL pra exibir a foto (com cache-bust por version).
export function informePhotoURL(id: string, v?: string | number): string {
  const bust = v != null ? `?v=${encodeURIComponent(String(v))}` : "";
  return `/api/informes/${encodeURIComponent(id)}/photo${bust}`;
}

// Upload multipart (não usa api() porque precisa de boundary auto-gerado).
export async function uploadInformePhoto(id: string, file: File): Promise<Informe> {
  const fd = new FormData();
  fd.append("photo", file);
  const res = await fetch(`/api/informes/${encodeURIComponent(id)}/photo`, {
    method: "POST",
    credentials: "include",
    body: fd,
  });
  let body: { success: boolean; data?: { informe: Informe }; message?: string } = {
    success: false,
  };
  try {
    body = await res.json();
  } catch {
    // sem corpo
  }
  if (!res.ok) {
    const err = new Error(body.message ?? `HTTP ${res.status}`) as Error & { status: number };
    err.status = res.status;
    throw err;
  }
  return (body.data as { informe: Informe }).informe;
}

export function deleteInformePhoto(id: string) {
  return api<void>(`/api/informes/${encodeURIComponent(id)}/photo`, {
    method: "DELETE",
  });
}
