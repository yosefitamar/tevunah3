"use client";

import { useState, type FormEvent } from "react";
import { Check, Lock, Pencil, Plus, Trash2, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { createRole, deleteRole, updateRole } from "@/lib/roles-api";
import type { ApiError } from "@/lib/api";
import type { RoleRow } from "@/lib/types";

export default function RolesManagement() {
  const { roles, refreshRoles } = useAuth();
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const [editLabel, setEditLabel] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  async function run(fn: () => Promise<unknown>, ok: string) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
      await refreshRoles();
      setNotice(ok);
    } catch (e) {
      setError((e as ApiError).message || "Falha na operação");
    } finally {
      setBusy(false);
    }
  }

  function startEdit(r: RoleRow) {
    setEditing(r.code);
    setEditLabel(r.label);
    setConfirmDelete(null);
  }

  async function saveEdit(code: string) {
    await run(() => updateRole(code, { label: editLabel.trim() }), "Papel atualizado.");
    setEditing(null);
  }

  async function doDelete(code: string) {
    await run(() => deleteRole(code), "Papel excluído.");
    setConfirmDelete(null);
  }

  return (
    <div className="screen-fill">
      <div className="toolbar">
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => setCreating(true)}
          disabled={busy}
        >
          <Plus size={14} strokeWidth={1.8} /> NOVO PAPEL
        </button>
        <span className="muted" style={{ marginLeft: "auto" }}>
          {roles.length} papel{roles.length === 1 ? "" : "is"}
        </span>
      </div>

      {error && <div className="banner banner-error">⚠ {error}</div>}
      {notice && <div className="banner banner-info">✓ {notice}</div>}

      <div className="panel panel--fill">
        <div className="table-scroll">
          <table className="tbl">
            <thead>
              <tr>
                <th style={{ width: 200 }}>CÓDIGO</th>
                <th>RÓTULO</th>
                <th style={{ width: 120 }}>TIPO</th>
                <th style={{ width: 160 }}></th>
              </tr>
            </thead>
            <tbody>
              {roles.map((r) => {
                const isEditing = editing === r.code;
                const isConfirming = confirmDelete === r.code;
                return (
                  <tr key={r.code}>
                    <td className="mono" style={{ fontSize: 11 }}>{r.code}</td>
                    <td style={{ color: "var(--fg-0)", fontWeight: 600 }}>
                      {isEditing ? (
                        <input
                          type="text"
                          value={editLabel}
                          onChange={(e) => setEditLabel(e.target.value)}
                          autoFocus
                          style={{ width: "100%" }}
                        />
                      ) : (
                        r.label
                      )}
                    </td>
                    <td className="muted">
                      {r.is_system ? (
                        <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                          <Lock size={11} strokeWidth={1.8} /> SISTEMA
                        </span>
                      ) : (
                        "CUSTOMIZADO"
                      )}
                    </td>
                    <td>
                      {isEditing ? (
                        <span style={{ display: "inline-flex", gap: 6 }}>
                          <button
                            type="button"
                            className="action-btn"
                            title="Salvar"
                            disabled={busy || !editLabel.trim()}
                            onClick={() => saveEdit(r.code)}
                          >
                            <Check size={14} />
                          </button>
                          <button
                            type="button"
                            className="action-btn"
                            title="Cancelar"
                            onClick={() => setEditing(null)}
                          >
                            <X size={14} />
                          </button>
                        </span>
                      ) : isConfirming ? (
                        <span style={{ display: "inline-flex", gap: 6, alignItems: "center" }}>
                          <span className="muted" style={{ fontSize: 11 }}>Excluir?</span>
                          <button
                            type="button"
                            className="btn btn-danger"
                            disabled={busy}
                            onClick={() => doDelete(r.code)}
                          >
                            SIM
                          </button>
                          <button
                            type="button"
                            className="btn btn-ghost"
                            onClick={() => setConfirmDelete(null)}
                          >
                            NÃO
                          </button>
                        </span>
                      ) : (
                        <span style={{ display: "inline-flex", gap: 6 }}>
                          <button
                            type="button"
                            className="action-btn"
                            title="Renomear"
                            onClick={() => startEdit(r)}
                          >
                            <Pencil size={14} />
                          </button>
                          <button
                            type="button"
                            className="action-btn"
                            title={
                              r.is_system
                                ? "Papel de sistema não pode ser excluído"
                                : "Excluir papel"
                            }
                            disabled={r.is_system}
                            onClick={() => {
                              setConfirmDelete(r.code);
                              setEditing(null);
                            }}
                          >
                            <Trash2 size={14} />
                          </button>
                        </span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {creating && (
        <CreateRoleModal
          busy={busy}
          onClose={() => setCreating(false)}
          onCreate={async (code, label) => {
            await run(() => createRole({ code, label }), "Papel criado.");
            setCreating(false);
          }}
        />
      )}
    </div>
  );
}

function CreateRoleModal({
  busy,
  onClose,
  onCreate,
}: {
  busy: boolean;
  onClose: () => void;
  onCreate: (code: string, label: string) => void;
}) {
  const [code, setCode] = useState("");
  const [label, setLabel] = useState("");

  // Sugere um código a partir do rótulo (minúsculas, _ no lugar de não-alfanum).
  function onLabelChange(v: string) {
    const wasAuto =
      code === "" ||
      code === slug(label);
    setLabel(v);
    if (wasAuto) setCode(slug(v));
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    onCreate(code.trim(), label.trim());
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span>NOVO PAPEL</span>
          <button className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>

        <form className="modal-bd" onSubmit={submit} autoComplete="off">
          <label className="form-field">
            <span>RÓTULO</span>
            <input
              type="text"
              value={label}
              onChange={(e) => onLabelChange(e.target.value)}
              placeholder="Ex.: Analista de ContraInteligência"
              required
              autoFocus
            />
          </label>
          <label className="form-field">
            <span>CÓDIGO</span>
            <input
              type="text"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="ex.: analista_ci"
              pattern="[a-z][a-z0-9_]{1,31}"
              title="minúsculas, dígitos e _ (2–32, começando com letra)"
              required
            />
          </label>
          <div className="muted" style={{ fontSize: 11 }}>
            O papel nasce sem permissões. Conceda-as na aba MATRIZ RBAC.
          </div>

          <div className="modal-ft">
            <button type="button" className="btn btn-ghost" onClick={onClose}>
              CANCELAR
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy}>
              {busy ? "CRIANDO…" : "CRIAR"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function slug(s: string): string {
  return s
    .toLowerCase()
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "") // remove diacríticos (ê→e, ç→c)
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 32);
}
