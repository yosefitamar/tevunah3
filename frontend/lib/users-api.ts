import { api } from "./api";
import type { RoleCode, User } from "./types";

export type UsersList = {
  items: User[];
  total: number;
  limit: number;
  offset: number;
};

export type ListUsersOpts = {
  limit?: number;
  offset?: number;
  search?: string;
  role?: RoleCode | "";
  status?: "" | "active" | "suspended" | "deactivated";
  clearance?: number; // 0 = todos
};

export type NewUserInput = {
  email: string;
  display_name: string;
  password: string;
  roles: RoleCode[];
  clearance_level: number;
};

export type CreatedUser = {
  user: User;
  totp_secret: string;
  note: string;
};

function qs(opts: ListUsersOpts): string {
  const p = new URLSearchParams();
  if (opts.limit) p.set("limit", String(opts.limit));
  if (opts.offset) p.set("offset", String(opts.offset));
  if (opts.search) p.set("search", opts.search);
  if (opts.role) p.set("role", opts.role);
  if (opts.status) p.set("status", opts.status);
  if (opts.clearance) p.set("clearance", String(opts.clearance));
  const s = p.toString();
  return s ? "?" + s : "";
}

export function listUsers(opts: ListUsersOpts = {}) {
  return api<UsersList>(`/api/users${qs(opts)}`);
}

export function getUser(id: string) {
  return api<{ user: User }>(`/api/users/${encodeURIComponent(id)}`);
}

export function createUser(input: NewUserInput) {
  return api<CreatedUser>(`/api/users`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateUserDisplayName(id: string, display_name: string) {
  return api<{ user: User }>(`/api/users/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ display_name }),
  });
}

export function deactivateUser(id: string, reason?: string) {
  return api<void>(`/api/users/${encodeURIComponent(id)}/deactivate`, {
    method: "POST",
    body: JSON.stringify({ reason: reason ?? "" }),
  });
}
