"use client";

import { useEffect, useState } from "react";
import { Pencil, Save } from "lucide-react";
import { setOpsIntelHistory, type OpsOccurrence } from "@/lib/ops-reports-api";
import type { ApiError } from "@/lib/api";
import { OpsActions } from "./OpsField";

const MAX = 20000;

type Props = {
  occ: OpsOccurrence;
  /** Chamado com a ficha regravada; sem ele, só leitura. */
  onChange?: (occ: OpsOccurrence) => void;
};

/**
 * Histórico de inteligência: o texto do analista que vai à aba Inteligência
 * do SIPOM. Só aparece com a participação da inteligência marcada. Não vem do
 * relatório; na ocorrência já enviada, vale no reenvio.
 */
export default function OpsIntelHistory({ occ, onChange }: Props) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(occ.intel_history);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!editing) setText(occ.intel_history);
  }, [occ.intel_history, editing]);

  async function save() {
    if (!occ.id) return;
    setBusy(true);
    setErr(null);
    try {
      const r = await setOpsIntelHistory(occ.id, text);
      setEditing(false);
      onChange?.(r.occurrence);
    } catch (e) {
      setErr((e as ApiError).message || "Erro ao gravar o histórico de inteligência");
    } finally {
      setBusy(false);
    }
  }

  if (!editing) {
    return (
      <>
        {occ.intel_history ? (
          <p className="ops-text ops-text--narrative">{occ.intel_history}</p>
        ) : (
          <div className="muted">
            {onChange ? "// NENHUM — O QUE FOR ESCRITO AQUI VAI PARA A ABA INTELIGÊNCIA DO SIPOM" : "—"}
          </div>
        )}
        {onChange && (
          <OpsActions>
            <button type="button" className="btn btn-sm" onClick={() => setEditing(true)}>
              <Pencil size={12} /> {occ.intel_history ? "EDITAR" : "ESCREVER"}
            </button>
          </OpsActions>
        )}
      </>
    );
  }

  return (
    <>
      <textarea
        className="ops-input ops-textarea"
        rows={8}
        maxLength={MAX}
        value={text}
        onChange={(e) => setText(e.target.value)}
        disabled={busy}
        autoFocus
      />
      {err && <div className="banner banner-error">⚠ {err}</div>}
      <OpsActions>
        <span className="muted ops-nowrap">
          {text.length}/{MAX}
          {occ.sipom?.sent_at ? " · JÁ ENVIADA: VAI NO REENVIO" : ""}
        </span>
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          disabled={busy}
          onClick={() => {
            setEditing(false);
            setErr(null);
          }}
        >
          CANCELAR
        </button>
        <button type="button" className="btn btn-primary btn-sm" disabled={busy} onClick={save}>
          <Save size={12} /> {busy ? "GRAVANDO…" : "GRAVAR"}
        </button>
      </OpsActions>
    </>
  );
}
