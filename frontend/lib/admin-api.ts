import { api } from "./api";
import type { Permission, PermissionMatrix, RoleCode } from "./types";

export function listPermissions() {
  return api<PermissionMatrix>(`/api/admin/permissions`);
}

export type UpdatePermissionInput = Partial<{
  allowed: boolean;
  requires_dual_approval: boolean;
  approver_role: RoleCode | null;
}>;

export function updatePermission(
  roleCode: RoleCode,
  action: string,
  input: UpdatePermissionInput,
) {
  return api<{ permission: Permission }>(
    `/api/admin/permissions/${encodeURIComponent(roleCode)}/${encodeURIComponent(action)}`,
    {
      method: "PATCH",
      body: JSON.stringify(input),
    },
  );
}
