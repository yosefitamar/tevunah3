"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Plus, Radar, RefreshCw, Trash2 } from "lucide-react";
import { useModal } from "@/contexts/ModalContext";
import {
  createIntelKeyword,
  deleteIntelKeyword,
  listIntelKeywords,
  reapplyIntelKeywords,
  setIntelKeywordActive,
  type IntelKeyword,
} from "@/lib/intel-keywords-api";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";

/**
 * Aba INTELIGÊNCIA do painel admin: termos que, encontrados no histórico ou
 * na equipe de uma ocorrência do relatório operacional, marcam a
 * participação da inteligência na importação. A comparação ignora caixa,
 * acento e pontuação, mas exige palavra inteira — "SAI do 2º BPRAIO" casa
 * com "sai do 2 bpraio", mas não com "saiu do 2º BPRAIO".
 *
 * Termos novos só valem para as próximas importações; REAPLICAR leva a regra
 * às ocorrências já gravadas (sem tocar nas marcadas à mão pelo analista).
 */
export default function IntelKeywords() {
  const modal = useModal();
  const [items, setItems] = useState<IntelKeyword[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [term, setTerm] = useState("");
  const [busy, setBusy] = useState(false);

  const reload = useCallback(async () => {
    try {
      const r = await listIntelKeywords();
      setItems(r.items);
      setError(null);
    } catch (e) {
      setError((e as ApiError).message || "Erro ao carregar os termos");
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  async function add(e: FormEvent) {
    e.preventDefault();
    if (!term.trim() || busy) return;
    setBusy(true);
    setError(null);
    try {
      await createIntelKeyword(term.trim());
      setTerm("");
      await reload();
    } catch (err) {
      setError((err as ApiError).message || "Erro ao cadastrar");
    } finally {
      setBusy(false);
    }
  }

  async function toggle(k: IntelKeyword) {
    setBusy(true);
    setError(null);
    try {
      await setIntelKeywordActive(k.id, !k.active);
      await reload();
    } catch (err) {
      setError((err as ApiError).message || "Erro ao atualizar");
    } finally {
      setBusy(false);
    }
  }

  async function remove(k: IntelKeyword) {
    const ok = await modal.confirm({
      title: "EXCLUIR TERMO",
      message: (
        <>
          Excluir o termo <strong>{k.term}</strong>? As ocorrências já marcadas por ele só mudam
          quando os termos forem reaplicados. Para suspender sem perder o cadastro, use DESATIVAR.
        </>
      ),
      confirm: "EXCLUIR",
      danger: true,
    });
    if (!ok) return;
    setBusy(true);
    setError(null);
    try {
      await deleteIntelKeyword(k.id);
      await reload();
    } catch (err) {
      setError((err as ApiError).message || "Erro ao excluir");
    } finally {
      setBusy(false);
    }
  }

  async function reapply() {
    const ok = await modal.confirm({
      title: "REAPLICAR TERMOS",
      message:
        "Reavaliar todas as ocorrências já importadas com os termos ativos? As que o analista " +
        "marcou ou desmarcou manualmente não serão alteradas.",
      variant: "info",
      confirm: "REAPLICAR",
    });
    if (!ok) return;
    setBusy(true);
    setError(null);
    try {
      const r = await reapplyIntelKeywords();
      await modal.alert({
        title: "TERMOS REAPLICADOS",
        variant: "success",
        autoClose: 0,
        message: `${r.checked} ocorrência(s) avaliada(s): ${r.marked} passaram a ter participação da inteligência e ${r.cleared} deixaram de ter.`,
      });
    } catch (err) {
      setError((err as ApiError).message || "Erro ao reaplicar");
    } finally {
      setBusy(false);
    }
  }

  const activeCount = items?.filter((k) => k.active).length ?? 0;

  return (
    <div className="screen-fill intel-kw">
      <div className="section-title">
        TERMOS DE INTELIGÊNCIA
        <span style={{ color: "var(--fg-2)" }}>· MARCAÇÃO AUTOMÁTICA NA IMPORTAÇÃO DO RELATÓRIO OPERACIONAL</span>
      </div>

      <p className="intel-kw-help">
        Quando o histórico ou a equipe de uma ocorrência contém um destes termos, a importação marca a
        participação da inteligência. Maiúsculas, acentos e pontuação não importam, mas o termo precisa
        aparecer como palavra inteira. Evite siglas soltas que também são palavras comuns (ex.: <em>SAI</em>,
        que casaria com o verbo “sai”).
      </p>

      <form className="intel-kw-add" onSubmit={add}>
        <input
          type="text"
          className="intel-kw-input"
          placeholder="NOVO TERMO — EX.: SAI DO 2º BPRAIO"
          value={term}
          maxLength={120}
          onChange={(e) => setTerm(e.target.value)}
          disabled={busy}
          aria-label="Novo termo"
        />
        <button type="submit" className="btn btn-primary" disabled={busy || !term.trim()}>
          <Plus size={14} strokeWidth={1.8} /> ADICIONAR
        </button>
        <div style={{ marginLeft: "auto" }} />
        <button type="button" className="btn" onClick={reapply} disabled={busy || activeCount === 0}>
          <RefreshCw size={14} strokeWidth={1.8} /> REAPLICAR ÀS OCORRÊNCIAS IMPORTADAS
        </button>
      </form>

      {error && <div className="banner banner-error">⚠ {error}</div>}

      {items === null && !error && <div className="muted">// CARREGANDO…</div>}

      {items && items.length === 0 && (
        <div className="intel-kw-empty">
          <Radar size={40} strokeWidth={1.2} />
          <div className="muted">Nenhum termo cadastrado — nenhuma ocorrência será marcada na importação.</div>
        </div>
      )}

      {items && items.length > 0 && (
        <div className="panel panel--fill intel-kw-panel">
          <div className="table-scroll">
            <table className="tbl">
              <thead>
                <tr>
                  <th>TERMO</th>
                  <th style={{ width: 110 }}>SITUAÇÃO</th>
                  <th style={{ width: 220 }}>CADASTRADO</th>
                  <th style={{ width: 170 }} />
                </tr>
              </thead>
              <tbody>
                {items.map((k) => (
                  <tr key={k.id} className={k.active ? undefined : "intel-kw-off"}>
                    <td style={{ color: "var(--fg-0)" }}>{k.term}</td>
                    <td>
                      <span className={"pill " + (k.active ? "active" : "cold")}>{k.active ? "ATIVO" : "INATIVO"}</span>
                    </td>
                    <td className="muted">
                      {formatBR(k.created_at)}
                      {k.created_by_name ? ` · ${k.created_by_name}` : " · CARGA INICIAL"}
                    </td>
                    <td style={{ textAlign: "right", whiteSpace: "nowrap" }}>
                      <button type="button" className="btn btn-ghost btn-sm" disabled={busy} onClick={() => toggle(k)}>
                        {k.active ? "DESATIVAR" : "ATIVAR"}
                      </button>
                      <button
                        type="button"
                        className="action-btn"
                        aria-label={`Excluir ${k.term}`}
                        title="Excluir"
                        disabled={busy}
                        onClick={() => remove(k)}
                      >
                        <Trash2 size={13} />
                      </button>
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
