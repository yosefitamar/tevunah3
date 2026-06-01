import { api } from "./api";
import type { RoleRow } from "./types";

export type RolesList = {
  items: RoleRow[];
  total: number;
};

export function listRoles() {
  return api<RolesList>(`/api/roles`);
}

export function createRole(input: { code: string; label: string }) {
  return api<{ role: RoleRow }>(`/api/admin/roles`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateRole(code: string, input: { label: string }) {
  return api<{ role: RoleRow }>(`/api/admin/roles/${encodeURIComponent(code)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deleteRole(code: string) {
  return api<{ deleted: string }>(`/api/admin/roles/${encodeURIComponent(code)}`, {
    method: "DELETE",
  });
}
