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
  sort_by?: "code" | "display_name" | "email" | "clearance_level" | "status" | "last_login_at";
  sort_dir?: "asc" | "desc";
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
  if (opts.sort_by) p.set("sort_by", opts.sort_by);
  if (opts.sort_dir) p.set("sort_dir", opts.sort_dir);
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

export type UpdateUserProfileInput = {
  display_name?: string;
  email?: string;
};

export function updateUserProfile(id: string, input: UpdateUserProfileInput) {
  return api<{ user: User }>(`/api/users/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deactivateUser(id: string, reason?: string) {
  return api<void>(`/api/users/${encodeURIComponent(id)}/deactivate`, {
    method: "POST",
    body: JSON.stringify({ reason: reason ?? "" }),
  });
}

// Admin → gera senha temporária pro agente; mostrada UMA vez. O agente
// recebe must_change_password=true e deve trocar no próximo login.
export function resetUserPassword(id: string) {
  return api<{ temp_password: string; note: string }>(
    `/api/users/${encodeURIComponent(id)}/password/reset`,
    { method: "POST", body: JSON.stringify({}) },
  );
}

// Admin → apaga o secret TOTP do agente e marca must_setup_totp=true.
// Admin nunca vê o novo secret — só o próprio agente, no próximo login.
export function resetUserTOTP(id: string) {
  return api<{ note: string }>(
    `/api/users/${encodeURIComponent(id)}/totp/reset`,
    { method: "POST", body: JSON.stringify({}) },
  );
}

// Self → troca a própria senha. Precisa da senha atual; valida tamanho
// mínimo no backend (12 chars).
export function changeOwnPassword(current_password: string, new_password: string) {
  return api<void>(`/api/auth/password/change`, {
    method: "POST",
    body: JSON.stringify({ current_password, new_password }),
  });
}

// Self → confirma o enrollment do TOTP após digitar o código gerado pelo
// authenticator. Backend já guardou o secret pendente no login.
export function setupTOTP(totp_code: string) {
  return api<void>(`/api/auth/totp/setup`, {
    method: "POST",
    body: JSON.stringify({ totp_code }),
  });
}
