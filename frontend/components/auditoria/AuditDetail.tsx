"use client";

import { useEffect, useState } from "react";
import { Copy, X } from "lucide-react";
import { getAuditEntry } from "@/lib/audit-api";
import { actionGroup, summarizeUA, type AuditEntry } from "@/lib/types";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";

type Props = {
  entryId: number;
  onClose: () => void;
};

function prettyJSON(v: unknown): string {
  if (v === undefined || v === null) return "";
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
}

function copy(text: string) {
  if (typeof navigator !== "undefined" && navigator.clipboard) {
    navigator.clipboard.writeText(text).catch(() => {});
  }
}

export default function AuditDetail({ entryId, onClose }: Props) {
  const [data, setData] = useState<AuditEntry | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(null);
    getAuditEntry(entryId)
      .then((d) => {
        if (alive) setData(d.entry);
      })
      .catch((e: ApiError) => {
        if (alive) setError(e.message || "Erro ao carregar entrada");
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [entryId]);

  const beforeStr = data ? prettyJSON(data.before) : "";
  const afterStr = data ? prettyJSON(data.after) : "";
  const hasPayload = beforeStr !== "" || afterStr !== "";

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <aside
        className="drawer drawer--wide"
        onClick={(e) => e.stopPropagation()}
        aria-label="Entrada do trilho de auditoria"
      >
        <div className="drawer-hd">
          <span>ENTRADA #{entryId} · TRILHO DE AUDITORIA</span>
          <button
            type="button"
            className="action-btn"
            onClick={onClose}
            aria-label="Fechar"
          >
            <X size={14} />
          </button>
        </div>

        <div className="drawer-bd">
          {loading && <div className="muted">// CARREGANDO…</div>}
          {error && <div className="banner banner-error">⚠ {error}</div>}

          {data && (
            <>
              <div className="dossier-head">
                <div className="dossier-code">#{data.id}</div>
                <div className="dossier-name" style={{ fontFamily: "var(--font-mono)" }}>
                  {data.action}
                </div>
                <div className="dossier-meta">
                  <span className="pill info">{actionGroup(data.action)}</span>
                  <span>·</span>
                  <span>{formatBR(data.ts)}</span>
                </div>
              </div>

              <div className="drawer-section">
                <div className="drawer-section-title">ATOR</div>
                <dl className="dossier-list">
                  <DLRow label="USUÁRIO">
                    {data.actor_user_id
                      ? `${data.actor_user_code ?? "—"} · ${data.actor_display_name ?? "—"}`
                      : "// SISTEMA (sem ator autenticado)"}
                  </DLRow>
                  {data.actor_user_id && (
                    <DLRow label="ID INTERNO">
                      <span className="mono">{data.actor_user_id}</span>
                    </DLRow>
                  )}
                  <DLRow label="IP">{data.actor_ip ?? "—"}</DLRow>
                  <DLRow label="SESSÃO">
                    {data.actor_session_id ? (
                      <span className="mono" title={data.actor_session_id}>
                        {data.actor_session_id.slice(0, 16)}…
                      </span>
                    ) : (
                      "—"
                    )}
                  </DLRow>
                  <DLRow label="TERMINAL">{data.actor_terminal || "—"}</DLRow>
                  <DLRow label="CLIENTE">
                    {data.actor_user_agent
                      ? (() => {
                          const { os, browser } = summarizeUA(data.actor_user_agent);
                          return `${browser} · ${os}`;
                        })()
                      : "—"}
                  </DLRow>
                  {data.actor_user_agent && (
                    <DLRow label="USER-AGENT">
                      <span
                        className="mono mono-wrap"
                        style={{ fontSize: 9.5 }}
                        title={data.actor_user_agent}
                      >
                        {data.actor_user_agent}
                      </span>
                    </DLRow>
                  )}
                </dl>
              </div>

              <div className="drawer-section">
                <div className="drawer-section-title">RECURSO</div>
                <dl className="dossier-list">
                  <DLRow label="TIPO">{data.resource_type ?? "—"}</DLRow>
                  <DLRow label="ID">
                    {data.resource_id ? (
                      <span className="mono">{data.resource_id}</span>
                    ) : (
                      "—"
                    )}
                  </DLRow>
                  <DLRow label="CLASSIFICAÇÃO">
                    {data.resource_classification != null
                      ? `CL-${String(data.resource_classification).padStart(2, "0")}`
                      : "—"}
                  </DLRow>
                  <DLRow label="MOTIVO">{data.reason || "—"}</DLRow>
                </dl>
              </div>

              <div className="drawer-section">
                <div className="drawer-section-title">DELTA · BEFORE / AFTER</div>
                {!hasPayload ? (
                  <div className="muted" style={{ padding: "8px 0" }}>
                    // Nenhum payload para esta ação.
                  </div>
                ) : (
                  <div className="audit-diff">
                    <DiffPane title="BEFORE" body={beforeStr} />
                    <DiffPane title="AFTER" body={afterStr} />
                  </div>
                )}
              </div>

              <div className="drawer-section">
                <div className="drawer-section-title">CADEIA DE HASH</div>
                <dl className="dossier-list">
                  <DLRow label="PREV_HASH">
                    <span className="mono mono-wrap" title={data.prev_hash}>
                      {data.prev_hash}
                    </span>
                    <button
                      type="button"
                      className="action-btn"
                      onClick={() => copy(data.prev_hash)}
                      title="Copiar"
                      style={{ marginLeft: 6 }}
                    >
                      <Copy size={12} />
                    </button>
                  </DLRow>
                  <DLRow label="HASH">
                    <span className="mono mono-wrap" title={data.hash}>
                      {data.hash}
                    </span>
                    <button
                      type="button"
                      className="action-btn"
                      onClick={() => copy(data.hash)}
                      title="Copiar"
                      style={{ marginLeft: 6 }}
                    >
                      <Copy size={12} />
                    </button>
                  </DLRow>
                </dl>
                <div className="muted" style={{ fontSize: 9, marginTop: 6 }}>
                  // hash = sha256(prev_hash ∥ payload-canônico). Adulteração
                  desta linha ou de qualquer anterior quebra a cadeia.
                </div>
              </div>
            </>
          )}
        </div>
      </aside>
    </div>
  );
}

function DLRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

function DiffPane({ title, body }: { title: string; body: string }) {
  return (
    <div className="audit-diff-pane">
      <div className="audit-diff-hd">{title}</div>
      <pre className="audit-diff-bd">{body || "// vazio"}</pre>
    </div>
  );
}
