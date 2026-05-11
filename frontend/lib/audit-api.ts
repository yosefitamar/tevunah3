import { api } from "./api";
import type { AuditEntry } from "./types";

export type AuditList = {
  items: AuditEntry[];
  total: number;
  limit: number;
  offset: number;
};

export type ListAuditOpts = {
  limit?: number;
  offset?: number;
  action?: string;        // exato; sufixo '*' vira prefix match
  actor_id?: string;
  resource_type?: string;
  resource_id?: string;
  from?: string;          // ISO ou YYYY-MM-DD
  to?: string;
  search?: string;
  sort_by?: "id" | "ts" | "action" | "actor" | "resource";
  sort_dir?: "asc" | "desc";
};

function qs(o: ListAuditOpts): string {
  const p = new URLSearchParams();
  if (o.limit) p.set("limit", String(o.limit));
  if (o.offset) p.set("offset", String(o.offset));
  if (o.action) p.set("action", o.action);
  if (o.actor_id) p.set("actor_id", o.actor_id);
  if (o.resource_type) p.set("resource_type", o.resource_type);
  if (o.resource_id) p.set("resource_id", o.resource_id);
  if (o.from) p.set("from", o.from);
  if (o.to) p.set("to", o.to);
  if (o.search) p.set("search", o.search);
  if (o.sort_by) p.set("sort_by", o.sort_by);
  if (o.sort_dir) p.set("sort_dir", o.sort_dir);
  const s = p.toString();
  return s ? "?" + s : "";
}

export function listAudit(opts: ListAuditOpts = {}) {
  return api<AuditList>(`/api/audit${qs(opts)}`);
}

export function getAuditEntry(id: number) {
  return api<{ entry: AuditEntry }>(`/api/audit/${id}`);
}
