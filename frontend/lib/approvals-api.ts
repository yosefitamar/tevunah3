import { api } from "./api";

export type ApprovalStatus =
  | "pending"
  | "approved"
  | "rejected"
  | "expired"
  | "cancelled";

export type ApprovalAction =
  | "user.role.assign"
  | "user.clearance.set"
  | string;

export type Approval = {
  id: string;
  action: ApprovalAction;
  resource_type?: string;
  resource_id?: string;
  payload: unknown;
  requested_by: string;
  requested_at: string;
  required_approver_role: string;
  status: ApprovalStatus;
  decided_by?: string;
  decided_at?: string;
  decision_reason?: string;
  expires_at: string;
};

export type ApprovalsList = {
  items: Approval[];
  total: number;
};

export type ListApprovalsOpts = {
  mode?: "" | "mine" | "pending_for_me";
  status?: "" | ApprovalStatus;
};

function qs(opts: ListApprovalsOpts): string {
  const p = new URLSearchParams();
  if (opts.mode) p.set("mode", opts.mode);
  if (opts.status) p.set("status", opts.status);
  const s = p.toString();
  return s ? "?" + s : "";
}

export function listApprovals(opts: ListApprovalsOpts = {}) {
  return api<ApprovalsList>(`/api/approvals${qs(opts)}`);
}

export function getApproval(id: string) {
  return api<{ approval: Approval }>(`/api/approvals/${encodeURIComponent(id)}`);
}

export function approveApproval(id: string, reason?: string) {
  return api<{ approval: Approval }>(
    `/api/approvals/${encodeURIComponent(id)}/approve`,
    {
      method: "POST",
      body: JSON.stringify({ reason: reason ?? "" }),
    }
  );
}

export function rejectApproval(id: string, reason?: string) {
  return api<{ approval: Approval }>(
    `/api/approvals/${encodeURIComponent(id)}/reject`,
    {
      method: "POST",
      body: JSON.stringify({ reason: reason ?? "" }),
    }
  );
}

export function cancelApproval(id: string, reason?: string) {
  return api<{ approval: Approval }>(
    `/api/approvals/${encodeURIComponent(id)}/cancel`,
    {
      method: "POST",
      body: JSON.stringify({ reason: reason ?? "" }),
    }
  );
}

// ─────────── Disparadores de ações 4-eyes (geram pending OU executam direto) ───────────

export type RoleAssignResponse = {
  approval?: Approval;
  user?: unknown;
  note?: string;
};

export function requestSetRoles(userId: string, roles: string[]) {
  return api<RoleAssignResponse>(
    `/api/users/${encodeURIComponent(userId)}/roles`,
    {
      method: "POST",
      body: JSON.stringify({ roles }),
    }
  );
}

export function requestSetClearance(userId: string, clearance_level: number) {
  return api<RoleAssignResponse>(
    `/api/users/${encodeURIComponent(userId)}/clearance`,
    {
      method: "POST",
      body: JSON.stringify({ clearance_level }),
    }
  );
}

export const ACTION_LABEL: Record<string, string> = {
  "user.role.assign": "ALTERAR PAPEL",
  "user.clearance.set": "ALTERAR CLEARANCE",
};

export const STATUS_LABEL: Record<ApprovalStatus, string> = {
  pending: "PENDENTE",
  approved: "APROVADA",
  rejected: "REJEITADA",
  expired: "EXPIRADA",
  cancelled: "CANCELADA",
};

export const STATUS_PILL: Record<ApprovalStatus, string> = {
  pending: "hold",
  approved: "active",
  rejected: "cold",
  expired: "cold",
  cancelled: "cold",
};
