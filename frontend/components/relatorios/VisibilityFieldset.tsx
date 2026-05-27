"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Plus, Search, X } from "lucide-react";
import { useModal } from "@/contexts/ModalContext";
import {
  type Report,
  type ReportVisibility,
  type ReportViewer,
  type UserLookup,
  listReportViewers,
  lookupUsers,
  setReportViewers,
  setReportVisibility,
} from "@/lib/reports-api";

type Props = {
  report: Report;
  /** ID do usuário logado — pra ocultá-lo do picker (autor não vira viewer). */
  meID: string;
  /** Autor/admin: pode ver a lista de viewers (endpoint restrito a esses papéis). */
  canManage: boolean;
  /** Autor/admin AND status='criado': pode mutar visibilidade/viewers. RI difundido é imutável. */
  canEdit: boolean;
  /** Notifica o pai que o relatório teve a visibilidade alterada (devolve o novo report). */
  onReportUpdated: (r: Report) => void;
};

// Sidebar "VISIBILIDADE": toggle aberto/restrito + lista de viewers explícitos.
// Quando restrito e a lista está vazia, o RI fica visível APENAS pro autor +
// admins. Adicionar viewers pela busca/typeahead alimentada pelo lookup leve.
export default function VisibilityFieldset({ report, meID, canManage, canEdit, onReportUpdated }: Props) {
  const modal = useModal();
  const [viewers, setViewers] = useState<ReportViewer[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);

  useEffect(() => {
    let alive = true;
    if (!canManage) {
      // Quem não é autor/admin não tem acesso à lista — endpoint devolve 403.
      // Esconde a sub-seção e segue.
      setLoaded(true);
      return;
    }
    setLoaded(false);
    (async () => {
      try {
        const r = await listReportViewers(report.id);
        if (alive) setViewers(r.viewers);
      } catch {
        // 403 esperado pra não-autor/não-admin.
      } finally {
        if (alive) setLoaded(true);
      }
    })();
    return () => {
      alive = false;
    };
  }, [report.id, canManage]);

  async function onToggleVisibility(next: ReportVisibility) {
    if (!canEdit) return; // defesa: backend bloqueia, mas evita request fadado a 409
    if (next === report.visibility) return;
    setBusy(true);
    try {
      const { report: updated } = await setReportVisibility(report.id, next);
      onReportUpdated(updated);
    } catch (e) {
      await modal.alert({
        variant: "error",
        title: "FALHA AO ALTERAR VISIBILIDADE",
        message: e instanceof Error ? e.message : "Erro desconhecido",
      });
    } finally {
      setBusy(false);
    }
  }

  async function saveViewers(next: ReportViewer[]) {
    if (!canEdit) return;
    setBusy(true);
    try {
      const { viewers: resolved } = await setReportViewers(
        report.id,
        next.map((v) => v.user_id),
      );
      setViewers(resolved);
    } catch (e) {
      await modal.alert({
        variant: "error",
        title: "FALHA AO ATUALIZAR VIEWERS",
        message: e instanceof Error ? e.message : "Erro desconhecido",
      });
    } finally {
      setBusy(false);
    }
  }

  function onRemove(userID: string) {
    saveViewers(viewers.filter((v) => v.user_id !== userID));
  }

  function onAdd(u: UserLookup) {
    if (viewers.some((v) => v.user_id === u.id)) return;
    // Optimistic add — saveViewers refetcha a lista resolvida do backend.
    saveViewers([
      ...viewers,
      {
        user_id: u.id,
        user_code: u.code,
        display_name: u.display_name,
        granted_by: meID,
        granted_at: new Date().toISOString(),
      },
    ]);
    setPickerOpen(false);
  }

  const restrito = report.visibility === "restrito";
  // RI já difundido/arquivado é imutável — mostra estado em read-only com aviso
  // explícito ao invés de toggle desabilitado (que parece clicável).
  const lockedByStatus = canManage && !canEdit;

  return (
    <fieldset className="form-fieldset">
      <legend>VISIBILIDADE</legend>

      {lockedByStatus ? (
        <div className="visibility-toggle">
          <span className={`vis-pill vis-pill--on vis-pill--readonly`}>
            {restrito ? "RESTRITO" : "ABERTO"}
          </span>
          <span className="visibility-hint">
            {restrito
              ? "VISÍVEL APENAS AO AUTOR + LISTA ABAIXO + ADMINS · "
              : "VISÍVEL A TODOS COM PERMISSÃO DE LEITURA · "}
            RI DIFUNDIDO — IMUTÁVEL
          </span>
        </div>
      ) : (
        <div className="visibility-toggle">
          <button
            type="button"
            className={`vis-pill${!restrito ? " vis-pill--on" : ""}`}
            disabled={!canEdit || busy}
            onClick={() => onToggleVisibility("aberto")}
          >
            ABERTO
          </button>
          <button
            type="button"
            className={`vis-pill${restrito ? " vis-pill--on" : ""}`}
            disabled={!canEdit || busy}
            onClick={() => onToggleVisibility("restrito")}
          >
            RESTRITO
          </button>
          <span className="visibility-hint">
            {restrito
              ? "VISÍVEL APENAS AO AUTOR + LISTA ABAIXO + ADMINS"
              : "VISÍVEL A TODOS COM PERMISSÃO DE LEITURA"}
          </span>
        </div>
      )}

      {restrito && canManage && (
        <div className="viewers-block">
          <div className="viewers-block-hd">
            <span>VIEWERS AUTORIZADOS · {viewers.length}</span>
            {canEdit && (
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                onClick={() => setPickerOpen(true)}
                disabled={busy}
              >
                <Plus size={12} /> ADICIONAR
              </button>
            )}
          </div>
          {!loaded ? (
            <div className="muted" style={{ fontSize: 11, padding: "6px 0" }}>
              // CARREGANDO…
            </div>
          ) : viewers.length === 0 ? (
            <div className="muted" style={{ fontSize: 11, padding: "6px 0" }}>
              // SOMENTE O AUTOR E ADMINS PODEM VER ESTE RELATÓRIO
            </div>
          ) : (
            <ul className="viewers-list">
              {viewers.map((v) => (
                <li key={v.user_id} className="viewer-chip">
                  <span className="viewer-code">{v.user_code}</span>
                  <span className="viewer-name">{v.display_name}</span>
                  {canEdit && (
                    <button
                      type="button"
                      className="viewer-remove"
                      title="Remover"
                      onClick={() => onRemove(v.user_id)}
                      disabled={busy}
                    >
                      <X size={11} />
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {pickerOpen && (
        <UserPickerModal
          excludeIDs={new Set([meID, ...viewers.map((v) => v.user_id)])}
          onPick={onAdd}
          onClose={() => setPickerOpen(false)}
        />
      )}
    </fieldset>
  );
}

// ─────────────────────────────────────────────────────────────────────────
// Picker modal: busca em /api/users/lookup com debounce 200ms.
function UserPickerModal({
  excludeIDs,
  onPick,
  onClose,
}: {
  excludeIDs: Set<string>;
  onPick: (u: UserLookup) => void;
  onClose: () => void;
}) {
  const [q, setQ] = useState("");
  const [items, setItems] = useState<UserLookup[]>([]);
  const [loading, setLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(async () => {
      setLoading(true);
      try {
        const r = await lookupUsers(q.trim());
        setItems(r.items);
      } finally {
        setLoading(false);
      }
    }, 200);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [q]);

  const filtered = useMemo(
    () => items.filter((u) => !excludeIDs.has(u.id)),
    [items, excludeIDs],
  );

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="picker-title"
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: 480 }}
      >
        <div className="modal-hd">
          <span id="picker-title">ADICIONAR VIEWER</span>
          <button
            type="button"
            className="action-btn"
            onClick={onClose}
            aria-label="Fechar"
          >
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd">
          <div className="picker-search">
            <Search size={13} />
            <input
              type="text"
              autoFocus
              placeholder="BUSCAR POR NOME OU CÓDIGO…"
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
          </div>
          <ul className="picker-results">
            {loading && (
              <li className="muted" style={{ fontSize: 11, padding: 10 }}>
                // BUSCANDO…
              </li>
            )}
            {!loading && filtered.length === 0 && (
              <li className="muted" style={{ fontSize: 11, padding: 10 }}>
                // NENHUM USUÁRIO ENCONTRADO
              </li>
            )}
            {!loading &&
              filtered.map((u) => (
                <li
                  key={u.id}
                  className="picker-item"
                  onClick={() => onPick(u)}
                >
                  <span className="picker-code">{u.code}</span>
                  <span className="picker-name">{u.display_name}</span>
                </li>
              ))}
          </ul>
        </div>
      </div>
    </div>
  );
}
