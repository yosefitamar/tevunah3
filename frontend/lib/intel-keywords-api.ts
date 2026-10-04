// Termos que marcam a participação da inteligência (SAI) nas ocorrências.
// Só o administrador gerencia — ver backend/cmd/server/intel_keywords.go.

import { api } from "./api";

export type IntelKeyword = {
  id: string;
  term: string;
  active: boolean;
  created_at: string;
  created_by_name: string;
};

export type IntelReapplyResult = { checked: number; marked: number; cleared: number };

export function listIntelKeywords() {
  return api<{ items: IntelKeyword[]; total: number }>("/api/admin/intel-keywords");
}

export function createIntelKeyword(term: string) {
  return api<IntelKeyword>("/api/admin/intel-keywords", {
    method: "POST",
    body: JSON.stringify({ term }),
  });
}

export function setIntelKeywordActive(id: string, active: boolean) {
  return api<IntelKeyword>(`/api/admin/intel-keywords/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ active }),
  });
}

export function deleteIntelKeyword(id: string) {
  return api<void>(`/api/admin/intel-keywords/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function reapplyIntelKeywords() {
  return api<IntelReapplyResult>("/api/admin/intel-keywords/reapply", { method: "POST" });
}
