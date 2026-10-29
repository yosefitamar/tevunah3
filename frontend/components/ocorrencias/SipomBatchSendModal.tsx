"use client";

import { useEffect, useRef, useState } from "react";
import { Send, Square, X } from "lucide-react";
import { enviarSipom } from "@/lib/sipom-api";
import type { ApiError } from "@/lib/api";

/** Ocorrência escolhida na fila para o envio em lote; `sort` dá a ordem do fato. */
export type BatchItem = { id: string; ficha: string; date: string; sort: string };

type Props = {
  items: BatchItem[];
  onClose: () => void;
  /** Ao fechar depois de enviar algo: a fila precisa recarregar. */
  onDone: () => void;
};

// Intervalo entre um envio e o próximo. O SIPOM aceita 60 por minuto (por IP
// e endpoint); a pausa deixa o lote em ~20 por minuto, longe do limite e sem
// pesar no servidor deles.
const GAP_MS = 3000;
// O SIPOM pediu para esperar (429) ou está fora (503): espera e tenta a mesma
// ocorrência de novo, até RETRIES vezes.
const WAIT_MS = 60_000;
const RETRIES = 2;
// Falhas seguidas que não são recusa de campo (502 sem detalhes, 503): algo
// está errado no conjunto (credencial, SIPOM fora) — o lote para.
const MAX_STREAK = 3;

type Status = "fila" | "enviando" | "aguardando" | "ok" | "erro";
type Result = { status: Status; info?: string };

const sleep = (ms: number, stop: () => boolean) =>
  new Promise<void>((resolve) => {
    const t0 = Date.now();
    const tick = () => (stop() || Date.now() - t0 >= ms ? resolve() : window.setTimeout(tick, 250));
    tick();
  });

/**
 * Envio em lote das ocorrências prontas da fila: uma de cada vez, com pausa
 * entre elas, pelo mesmo envio da ficha (que recalcula, monta e registra na
 * auditoria). A recusa de uma ocorrência não para o lote; falhas seguidas do
 * SIPOM param. O analista pode parar a qualquer momento — a que está em
 * envio termina.
 */
export default function SipomBatchSendModal({ items, onClose, onDone }: Props) {
  const [results, setResults] = useState<Record<string, Result>>({});
  const [running, setRunning] = useState(false);
  const [finished, setFinished] = useState(false);
  const [stopReason, setStopReason] = useState<string | null>(null);
  const stopRef = useRef(false);

  // Fechar a aba no meio do lote interrompe o envio.
  useEffect(() => {
    if (!running) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [running]);

  const set = (id: string, r: Result) => setResults((cur) => ({ ...cur, [id]: r }));

  async function run() {
    stopRef.current = false;
    setRunning(true);
    setStopReason(null);
    let streak = 0;
    for (let i = 0; i < items.length; i++) {
      if (stopRef.current) break;
      const it = items[i];
      let done = false;
      for (let attempt = 0; !done; attempt++) {
        set(it.id, { status: "enviando" });
        try {
          const r = await enviarSipom(it.id);
          const env = r.sipom.envio;
          const avisos = env?.avisos?.length ? ` · ${env.avisos.length} aviso(s)` : "";
          set(it.id, { status: "ok", info: `${(env?.situacao || "enviado").toUpperCase()} · ${env?.identificador ?? ""}${avisos}` });
          streak = 0;
          done = true;
        } catch (e) {
          const err = e as ApiError;
          const transient = err.status === 429 || err.status === 503;
          if (transient && attempt < RETRIES && !stopRef.current) {
            set(it.id, { status: "aguardando", info: `${err.message} — nova tentativa em 1 min` });
            await sleep(WAIT_MS, () => stopRef.current);
            continue;
          }
          let info = err.message || "erro no envio";
          if (err.details && typeof err.details === "object" && !Array.isArray(err.details)) {
            info += " — " + Object.entries(err.details as Record<string, string>).map(([k, v]) => `${k}: ${v}`).join("; ");
          }
          set(it.id, { status: "erro", info });
          // Recusa desta ocorrência (pendência, campo inválido, excluída no
          // SIPOM) não diz nada do resto do lote.
          const ownFault = err.status === 409 || (err.status === 502 && !!err.details);
          streak = ownFault ? 0 : streak + 1;
          if (err.status === 403 || streak >= MAX_STREAK) {
            setStopReason(
              err.status === 403
                ? "Sem permissão para enviar — o lote parou."
                : `${MAX_STREAK} falhas seguidas do SIPOM — o lote parou. Confira a conexão com o SIPOM antes de continuar.`,
            );
            stopRef.current = true;
          }
          done = true;
        }
      }
      if (i < items.length - 1 && !stopRef.current) await sleep(GAP_MS, () => stopRef.current);
    }
    setRunning(false);
    setFinished(true);
  }

  const counts = Object.values(results).reduce(
    (acc, r) => ({ ...acc, [r.status]: (acc[r.status] ?? 0) + 1 }),
    {} as Partial<Record<Status, number>>,
  );
  const processed = (counts.ok ?? 0) + (counts.erro ?? 0);
  const minutes = Math.max(1, Math.ceil((items.length * (GAP_MS + 2000)) / 60_000));
  const close = () => {
    if (running) return;
    if (processed > 0) onDone();
    onClose();
  };

  return (
    <div className="modal-backdrop" onClick={close}>
      <div className="modal sipom-batch" onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span>ENVIAR AO SIPOM · {items.length} OCORRÊNCIA(S)</span>
          <button type="button" className="action-btn" onClick={close} aria-label="Fechar" disabled={running}>
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd">
          {!running && !finished && (
            <div className="banner banner-info">
              UMA OCORRÊNCIA A CADA ~{GAP_MS / 1000}S, ABAIXO DO LIMITE DO SIPOM (60/MIN) · TEMPO ESTIMADO: ~{minutes} MIN ·
              MANTENHA ESTA ABA ABERTA ATÉ O FIM
            </div>
          )}
          {(running || finished) && (
            <div className="banner banner-info" role="status">
              {running ? `ENVIANDO… ${processed} DE ${items.length}` : `CONCLUÍDO · ${processed} DE ${items.length} PROCESSADA(S)`}
              {" · "}
              {counts.ok ?? 0} ENVIADA(S) · {counts.erro ?? 0} COM ERRO
            </div>
          )}
          {stopReason && <div className="banner banner-error">⚠ {stopReason}</div>}
          <div className="sipom-batch-list">
            <table className="tbl">
              <thead>
                <tr>
                  <th style={{ width: 150 }}>FICHA</th>
                  <th style={{ width: 110 }}>DATA</th>
                  <th style={{ width: 120 }}>SITUAÇÃO</th>
                  <th>RESULTADO</th>
                </tr>
              </thead>
              <tbody>
                {items.map((it) => {
                  const r = results[it.id] ?? { status: "fila" as Status };
                  return (
                    <tr key={it.id}>
                      <td className="mono">{it.ficha || "—"}</td>
                      <td>{it.date}</td>
                      <td>
                        <span className={"pill " + PILL[r.status]}>{LABEL[r.status]}</span>
                      </td>
                      <td className={r.status === "erro" ? "sipom-batch-err" : "muted"}>{r.info ?? ""}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
        <div className="modal-ft">
          {running ? (
            <button type="button" className="btn" onClick={() => (stopRef.current = true)}>
              <Square size={12} strokeWidth={2} /> PARAR APÓS A ATUAL
            </button>
          ) : finished ? (
            <button type="button" className="btn btn-primary" onClick={close}>
              FECHAR
            </button>
          ) : (
            <>
              <button type="button" className="btn btn-ghost" onClick={close}>
                CANCELAR
              </button>
              <button type="button" className="btn btn-primary" onClick={run} disabled={items.length === 0}>
                <Send size={12} strokeWidth={2} /> ENVIAR {items.length} OCORRÊNCIA(S)
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

const LABEL: Record<Status, string> = {
  fila: "NA FILA",
  enviando: "ENVIANDO",
  aguardando: "AGUARDANDO",
  ok: "ENVIADA",
  erro: "ERRO",
};

const PILL: Record<Status, string> = {
  fila: "cold",
  enviando: "info",
  aguardando: "hold",
  ok: "active",
  erro: "crit",
};
