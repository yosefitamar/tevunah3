import { api } from "./api";

export type SystemSettings = {
  agency_name: string;
  document_title: string;
  brasao_path?: string;
};

export function getSystemSettings() {
  return api<{ settings: SystemSettings }>(`/api/system-settings`);
}

export type UpdateSystemSettingsInput = {
  agency_name: string;
  document_title: string;
};

export function updateSystemSettings(input: UpdateSystemSettingsInput) {
  return api<{ settings: SystemSettings }>(`/api/admin/system-settings`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

// Upload do brasão via multipart. Backend grava em PHOTO_DIR/logo-sai.<ext>
// pra o gerador de PDF consumir sem mudanças. Não usamos o wrapper api()
// porque ele força Content-Type: application/json (multipart precisa boundary
// auto-gerado pelo navegador) — replica o padrão de uploadEntityPhoto.
export async function uploadBrasao(file: File): Promise<{ brasao_path: string; mime: string }> {
  const fd = new FormData();
  fd.append("brasao", file);
  const res = await fetch(`/api/admin/system-settings/brasao`, {
    method: "PUT",
    credentials: "include",
    body: fd,
  });
  let body: { success: boolean; data?: { brasao_path: string; mime: string }; message?: string } = {
    success: false,
  };
  try {
    body = await res.json();
  } catch {
    // resposta sem corpo
  }
  if (!res.ok) {
    const err = new Error(body.message ?? `HTTP ${res.status}`) as Error & { status: number };
    err.status = res.status;
    throw err;
  }
  return body.data as { brasao_path: string; mime: string };
}

// URL absoluta usada pra preview do brasão na tela de admin (não há link
// público; só admin com permissão system.settings.update enxerga).
export function brasaoPreviewURL(): string {
  return `/api/admin/system-settings/brasao?t=${Date.now()}`;
}

// Upload do logo institucional (PMCE + CEARÁ). Grava em
// PHOTO_DIR/logo-instituicao2.<ext>; o gerador de PDF consome via
// loadAsset("logo-instituicao2"). Mesmo padrão multipart do brasão.
export async function uploadInstitutionalLogo(file: File): Promise<{ logo_path: string; mime: string }> {
  const fd = new FormData();
  fd.append("logo", file);
  const res = await fetch(`/api/admin/system-settings/logo`, {
    method: "PUT",
    credentials: "include",
    body: fd,
  });
  let body: { success: boolean; data?: { logo_path: string; mime: string }; message?: string } = {
    success: false,
  };
  try {
    body = await res.json();
  } catch {
    // resposta sem corpo
  }
  if (!res.ok) {
    const err = new Error(body.message ?? `HTTP ${res.status}`) as Error & { status: number };
    err.status = res.status;
    throw err;
  }
  return body.data as { logo_path: string; mime: string };
}

export function institutionalLogoPreviewURL(): string {
  return `/api/admin/system-settings/logo?t=${Date.now()}`;
}
