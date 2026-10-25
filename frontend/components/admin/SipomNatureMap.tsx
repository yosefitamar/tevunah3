"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { PlugZap, Save } from "lucide-react";
import {
  getSipomNatureMap,
  listSipomNaturezas,
  pingSipom,
  upsertSipomNatureRule,
  type SipomConfianca,
  type SipomNatureza,
} from "@/lib/sipom-api";
import type { ApiError } from "@/lib/api";
import Select from "../shared/Select";

const CONFIANCA_OPTIONS: { value: SipomConfianca; label: string }[] = [
  { value: "direta", label: "DIRETA" },
  { value: "com_arma", label: "DIRETA SE HOUVER ARMA" },
  { value: "com_droga", label: "DIRETA SE HOUVER DROGA" },
  { value: "sugerida", label: "SUGERIDA" },
];

// Linha editável: regra existente ou natureza nova (ainda sem regra).
type Row = {
  key: string;
  source: string;
  isNew: boolean;
  ocorrencias: number;
  naturezaId: string; // "" = sem correspondência
  confianca: SipomConfianca;
  prioridade: string;
  saved: { naturezaId: string; confianca: SipomConfianca; prioridade: string };
};

/**
 * Aba SIPOM do Admin: de-para entre as naturezas do relatório operacional e
 * as naturezas do SIPOM (destino do envio). Cada ocorrência vai com UMA
 * natureza; com várias na ficha, vale a de menor prioridade.
 *
 * DIRETA aplica sozinha; SUGERIDA aplica, mas deixa a ocorrência pendente
 * até o analista confirmar na ficha; DIRETA SE HOUVER ARMA/DROGA aplica
 * sozinha quando a ficha traz a apreensão correspondente. Gravar uma regra recalcula o acervo
 * (exceto as ocorrências em que o analista fixou a natureza à mão).
 */
export default function SipomNatureMap() {
  const [rows, setRows] = useState<Row[] | null>(null);
  const [naturezas, setNaturezas] = useState<SipomNatureza[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busyKey, setBusyKey] = useState<string | null>(null);

  const reload = useCallback(async () => {
    try {
      const [m, n] = await Promise.all([getSipomNatureMap(), listSipomNaturezas()]);
      setNaturezas(n.items);
      const fresh: Row[] = [
        ...m.unmapped.map((u) => {
          const saved = { naturezaId: "", confianca: "sugerida" as SipomConfianca, prioridade: "100" };
          return { key: "new:" + u.nature, source: u.nature, isNew: true, ocorrencias: u.ocorrencias, ...saved, saved };
        }),
        ...m.rules.map((r) => {
          const saved = {
            naturezaId: r.natureza ? String(r.natureza.id) : "",
            confianca: r.confianca,
            prioridade: String(r.prioridade),
          };
          return { key: r.id, source: r.source, isNew: false, ocorrencias: r.ocorrencias, ...saved, saved };
        }),
      ];
      setRows(fresh);
      setError(null);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar o de-para");
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const options = useMemo(
    () => [
      { value: "", label: "— SEM CORRESPONDÊNCIA —" },
      ...naturezas.map((n) => ({ value: String(n.id), label: `${n.rotulo} (${n.id})` })),
    ],
    [naturezas],
  );

  function patch(key: string, p: Partial<Row>) {
    setRows((rs) => rs?.map((r) => (r.key === key ? { ...r, ...p } : r)) ?? rs);
  }

  async function save(r: Row) {
    const prio = Number(r.prioridade);
    if (!Number.isInteger(prio) || prio < 1 || prio > 999) {
      setError("Prioridade deve ser um número entre 1 e 999");
      return;
    }
    setBusyKey(r.key);
    setError(null);
    setNotice(null);
    try {
      const res = await upsertSipomNatureRule({
        source: r.source,
        natureza_id: r.naturezaId ? Number(r.naturezaId) : null,
        confianca: r.confianca,
        prioridade: prio,
      });
      setNotice(
        `Regra gravada. Acervo recalculado: ${res.ready} ocorrência(s) pronta(s) para envio, ${res.pending} com pendência.`,
      );
      await reload();
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gravar a regra");
    } finally {
      setBusyKey(null);
    }
  }

  const dirty = (r: Row) =>
    r.isNew ||
    r.naturezaId !== r.saved.naturezaId ||
    r.confianca !== r.saved.confianca ||
    r.prioridade !== r.saved.prioridade;

  return (
    <div className="screen-fill intel-kw">
      <div className="section-title">
        DE-PARA DE NATUREZAS
        <span style={{ color: "var(--fg-2)" }}>· RELATÓRIO OPERACIONAL → SIPOM</span>
      </div>

      <p className="intel-kw-help">
        Cada ocorrência vai ao SIPOM com <strong>uma</strong> natureza do catálogo deles. Com várias naturezas na
        ficha, vale a de <strong>menor prioridade</strong> (a mais grave). <strong>Direta</strong> aplica sozinha;{" "}
        <strong>sugerida</strong> aplica, mas a ocorrência fica pendente até o analista confirmar na ficha.{" "}
        <strong>Direta se houver arma</strong> (ou <strong>droga</strong>) aplica sozinha quando a ficha traz a
        apreensão na tabela de armas de fogo (ou de drogas) e, sem ela, vale como sugerida. O artigo de
        lei é ignorado na comparação (&ldquo;FURTO - ART. 155/CPB&rdquo; = &ldquo;FURTO&rdquo;).
      </p>

      <SipomPing />

      {error && <div className="banner banner-error">⚠ {error}</div>}
      {notice && <div className="banner banner-info">{notice}</div>}
      {rows === null && !error && <div className="muted">// CARREGANDO…</div>}

      {rows && (
        <div className="panel panel--fill intel-kw-panel sipom-map-panel">
          <div className="table-scroll">
            <table className="tbl">
              <thead>
                <tr>
                  <th>NATUREZA NO RELATÓRIO</th>
                  <th style={{ width: 70, textAlign: "right" }}>OCORR.</th>
                  <th style={{ width: 380 }}>NATUREZA NO SIPOM</th>
                  <th style={{ width: 230 }}>CONFIANÇA</th>
                  <th style={{ width: 90 }}>PRIORIDADE</th>
                  <th style={{ width: 90 }} />
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.key}>
                    <td style={{ color: "var(--fg-0)" }}>
                      {r.source}
                      {r.isNew && <span className="pill hold" style={{ marginLeft: 8 }}>NOVA</span>}
                    </td>
                    <td className="mono" style={{ textAlign: "right" }}>
                      {r.ocorrencias}
                    </td>
                    <td>
                      <Select
                        searchable
                        value={r.naturezaId}
                        options={options}
                        placeholder="— SEM CORRESPONDÊNCIA —"
                        onChange={(v) => patch(r.key, { naturezaId: v })}
                      />
                    </td>
                    <td>
                      <Select
                        value={r.confianca}
                        options={CONFIANCA_OPTIONS}
                        onChange={(v) => patch(r.key, { confianca: v as SipomConfianca })}
                      />
                    </td>
                    <td>
                      <input
                        type="number"
                        min={1}
                        max={999}
                        className="intel-kw-input sipom-prio"
                        value={r.prioridade}
                        onChange={(e) => patch(r.key, { prioridade: e.target.value })}
                        aria-label={`Prioridade de ${r.source}`}
                      />
                    </td>
                    <td style={{ textAlign: "right" }}>
                      {dirty(r) && (
                        <button
                          type="button"
                          className="btn btn-primary btn-sm"
                          disabled={busyKey !== null}
                          onClick={() => save(r)}
                        >
                          <Save size={12} strokeWidth={2} /> {busyKey === r.key ? "…" : "SALVAR"}
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

// Teste da conexão com a API do SIPOM: confere chave, segredo e relógio do
// servidor sem gravar nada lá.
function SipomPing() {
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; text: string } | null>(null);
  async function run() {
    setBusy(true);
    setResult(null);
    try {
      const r = await pingSipom();
      if (!r.configurado) {
        setResult({ ok: false, text: "API DO SIPOM SEM CREDENCIAIS NO SERVIDOR — O ENVIO É MARCADO À MÃO" });
      } else if (r.ok) {
        setResult({ ok: true, text: `CONEXÃO OK · ${r.url} · CLIENTE ${r.cliente} · ${r.ms} MS` });
      } else {
        setResult({ ok: false, text: `${r.url}: ${r.erro}` });
      }
    } catch (e) {
      setResult({ ok: false, text: (e as ApiError).message || "Erro ao testar" });
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="ops-control-row">
      <button type="button" className="btn btn-sm" disabled={busy} onClick={run}>
        <PlugZap size={12} strokeWidth={2} /> {busy ? "TESTANDO…" : "TESTAR CONEXÃO COM O SIPOM"}
      </button>
      {result && <span className={result.ok ? "pill active" : "pill crit"}>{result.text}</span>}
    </div>
  );
}
