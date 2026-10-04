"use client";

import { useEffect, useState } from "react";
import { AlertTriangle, Braces, Check, CheckCircle2, Pencil, RotateCcw, Send } from "lucide-react";
import type { OpsOccurrence, OpsOfficer, SipomOccurrence, SipomRef } from "@/lib/ops-reports-api";
import {
  listSipomNaturezas,
  setOpsSipomField,
  setOpsSipomNatureza,
  type SipomFieldInput,
  type SipomNatureza,
} from "@/lib/sipom-api";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import Select from "../shared/Select";
import SipomPayloadModal from "./SipomPayloadModal";

// Catálogo de naturezas do SIPOM: 146 itens que não mudam — uma busca por
// sessão serve para todas as fichas.
let naturezasCache: Promise<SipomNatureza[]> | null = null;
function loadNaturezas(): Promise<SipomNatureza[]> {
  naturezasCache ??= listSipomNaturezas()
    .then((r) => r.items)
    .catch((e) => {
      naturezasCache = null;
      throw e;
    });
  return naturezasCache;
}

type Props = {
  occ: OpsOccurrence;
  /** Ficha gravada + permissão de edição: as correções ficam disponíveis. */
  editable?: boolean;
  onChange?: (s: SipomOccurrence, officers?: OpsOfficer[]) => void;
};

/**
 * Como a ocorrência vai para o SIPOM: os campos obrigatórios do cadastro
 * deles já nos códigos do destino, o que ainda impede o envio e, na ficha
 * gravada, as correções do analista. O que o analista fixa vale sobre o
 * automático até ele voltar ao automático.
 */
export default function SipomSection({ occ, editable = false, onChange }: Props) {
  const s = occ.sipom;
  const [naturezas, setNaturezas] = useState<SipomNatureza[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editAddr, setEditAddr] = useState(false);
  const [showPayload, setShowPayload] = useState(false);

  useEffect(() => {
    if (!editable) return;
    loadNaturezas()
      .then(setNaturezas)
      .catch(() => setError("Não foi possível carregar as naturezas do SIPOM"));
  }, [editable]);

  if (!s) return null;
  const blocking = s.pendencias.filter((p) => p.blocking);
  const notes = s.pendencias.filter((p) => !p.blocking);
  const pending = new Set(s.pendencias.map((p) => p.code));
  const manual = new Set(s.manual);
  const toConfirm = pending.has("natureza_confirmar");
  const dataHora = occ.start_time ? `${formatBRDate(occ.occurred_on)} ${occ.start_time}` : "";

  async function run(fn: () => Promise<{ sipom: SipomOccurrence; officers?: OpsOfficer[] }>) {
    setBusy(true);
    setError(null);
    try {
      const r = await fn();
      onChange?.(r.sipom, r.officers);
      return true;
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gravar");
      return false;
    } finally {
      setBusy(false);
    }
  }
  const setNatureza = (id: number | null) => occ.id && run(() => setOpsSipomNatureza(occ.id!, id));
  const setField = (input: SipomFieldInput) => occ.id && run(() => setOpsSipomField(occ.id!, input));

  const areaEditable = editable && (pending.has("area") || pending.has("area_ambigua") || manual.has("area"));
  const areaChoices: SipomRef[] = s.area_options.length ? s.area_options : s.area_candidates;
  const opmEditable = editable && (pending.has("opm") || manual.has("opm"));
  const compEditable =
    editable && occ.officers.length > 0 && (pending.has("composicao_multi") || manual.has("composicao"));

  return (
    <section className="ops-section sipom-section">
      <div className="ops-section-title sipom-title">
        <Send size={12} strokeWidth={2} /> ENVIO AO SIPOM
        {s.ready && occ.id && (
          <button type="button" className="btn btn-ghost btn-sm sipom-payload-btn" onClick={() => setShowPayload(true)}>
            <Braces size={12} strokeWidth={2} /> PRÉVIA DO ENVIO
          </button>
        )}
        {s.ready ? (
          <span className="pill active">
            <CheckCircle2 size={10} strokeWidth={2} /> PRONTA
          </span>
        ) : (
          <span className="pill hold">
            <AlertTriangle size={10} strokeWidth={2} /> {blocking.length} PENDÊNCIA{blocking.length === 1 ? "" : "S"}
          </span>
        )}
      </div>

      {/* Natureza */}
      <div className={"sipom-natureza" + (pending.has("natureza") || toConfirm ? " sipom-missing" : "")}>
        <span className="sipom-natureza-lbl">
          NATUREZA
          <span className="muted">
            {" · "}
            {!s.natureza
              ? "SEM CORRESPONDÊNCIA"
              : manual.has("natureza")
                ? "DEFINIDA PELO ANALISTA"
                : toConfirm
                  ? "SUGERIDA PELO DE-PARA"
                  : "PELO DE-PARA"}
          </span>
        </span>
        {editable ? (
          <div className="sipom-natureza-edit">
            <Select
              searchable
              value={s.natureza ? String(s.natureza.id) : ""}
              placeholder="ESCOLHA A NATUREZA DO SIPOM"
              disabled={busy || naturezas.length === 0}
              onChange={(v) => setNatureza(Number(v))}
              options={naturezas.map((n) => ({ value: String(n.id), label: n.rotulo }))}
            />
            {toConfirm && s.natureza && (
              <button type="button" className="btn btn-primary btn-sm" disabled={busy} onClick={() => setNatureza(s.natureza!.id)}>
                <Check size={12} strokeWidth={2} /> CONFIRMAR
              </button>
            )}
            {manual.has("natureza") && <AutoButton busy={busy} onClick={() => setNatureza(null)} />}
          </div>
        ) : (
          <span className="sipom-natureza-val">{s.natureza?.nome ?? "—"}</span>
        )}
      </div>
      {error && <div className="banner banner-error">⚠ {error}</div>}

      <dl className="ops-kv sipom-kv">
        <Field label="DATA E HORA" value={dataHora} missing={pending.has("data_hora")} />

        <Field
          label="ÁREA DA UNIDADE MILITAR"
          missing={pending.has("area") || pending.has("area_ambigua")}
          tag={manual.has("area") ? "ANALISTA" : undefined}
          value={s.area?.nome ?? ""}
        >
          {areaEditable && (
            <div className="sipom-inline">
              <Select
                value={s.area ? String(s.area.id) : ""}
                placeholder={areaChoices.length ? "ESCOLHA A ÁREA" : "SEM ÁREA NA CIDADE"}
                disabled={busy || areaChoices.length === 0}
                onChange={(v) => setField({ field: "area", area_id: Number(v) })}
                options={areaChoices.map((a) => ({ value: String(a.id), label: a.nome }))}
              />
              {manual.has("area") && <AutoButton busy={busy} onClick={() => setField({ field: "area", area_id: null })} />}
            </div>
          )}
        </Field>

        <Field
          label="LOGRADOURO"
          value={s.logradouro}
          missing={pending.has("logradouro")}
          tag={manual.has("endereco") ? "ANALISTA" : undefined}
          action={
            editable && !editAddr ? (
              <button type="button" className="action-btn" onClick={() => setEditAddr(true)} aria-label="Corrigir endereço" title="Corrigir endereço">
                <Pencil size={11} />
              </button>
            ) : undefined
          }
        />
        <Field label="NÚMERO" value={s.numeral || "—"} />
        {editAddr && (
          <AddressEditor
            logradouro={s.logradouro}
            numeral={s.numeral}
            busy={busy}
            manual={manual.has("endereco")}
            onCancel={() => setEditAddr(false)}
            onSave={async (logradouro, numeral) => {
              if (await setField({ field: "endereco", logradouro, numeral })) setEditAddr(false);
            }}
            onReset={async () => {
              if (await setField({ field: "endereco", reset: true })) setEditAddr(false);
            }}
          />
        )}

        <Field label="BAIRRO" value={s.bairro?.nome ?? occ.place_neighborhood} missing={pending.has("bairro")} />
        <Field label="CIDADE" value={s.cidade?.nome ?? occ.place_city} missing={pending.has("cidade")} />

        <Field label="OPM QUE ATENDEU" value={s.opm?.nome ?? ""} missing={pending.has("opm")} tag={manual.has("opm") ? "ANALISTA" : undefined}>
          {opmEditable && (
            <div className="sipom-inline">
              <Select
                value={s.opm ? String(s.opm.id) : ""}
                placeholder="ESCOLHA A COMPANHIA"
                disabled={busy || s.opm_candidates.length === 0}
                onChange={(v) => setField({ field: "opm", opm_id: Number(v) })}
                options={s.opm_candidates.map((c) => ({ value: String(c.id), label: c.nome }))}
              />
              {manual.has("opm") && <AutoButton busy={busy} onClick={() => setField({ field: "opm", opm_id: null })} />}
            </div>
          )}
        </Field>
        <Field label="Nº DA OCORRÊNCIA" value={occ.ciops_record} missing={pending.has("ficha")} />
      </dl>

      {compEditable && (
        <TeamsEditor
          officers={occ.officers}
          equipes={s.equipes}
          busy={busy}
          manual={manual.has("composicao")}
          onSave={(equipes) => setField({ field: "composicao", equipes })}
          onReset={() => setField({ field: "composicao", reset: true })}
        />
      )}

      {s.pessoas.length > 0 && (
        <>
          <div className="sipom-sub">ENVOLVIDOS ({s.pessoas.length})</div>
          <table className="tbl ops-subtbl sipom-people">
            <thead>
              <tr>
                <th>VÍNCULO</th>
                <th>NOME</th>
                <th>CPF</th>
                <th>SEXO</th>
                <th>NASCIMENTO</th>
                <th>MÃE</th>
                <th>MORTE</th>
                <th>FOTO</th>
                <th>ORIGEM</th>
              </tr>
            </thead>
            <tbody>
              {s.pessoas.map((p, i) => (
                <tr key={i}>
                  <td>{p.vinculo_label}</td>
                  <td style={{ color: "var(--fg-0)" }}>{p.nome || "—"}</td>
                  <td className="mono">{p.cpf ? formatCPF(p.cpf) : "—"}</td>
                  <td>{p.sexo === 1 ? "MASCULINO" : p.sexo === 2 ? "FEMININO" : "—"}</td>
                  <td>{p.nascimento ? formatBRDate(p.nascimento) : "—"}</td>
                  <td>{p.mae || "—"}</td>
                  <td>{p.morte ? <span className="pill deceased">SIM</span> : "NÃO"}</td>
                  <td>{p.foto ? "SIM" : "—"}</td>
                  <td>
                    <span className={"pill " + (p.fonte === "dossie" ? "active" : "hold")}>
                      {p.fonte === "dossie" ? "DOSSIÊ" : "SÓ RELATÓRIO"}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      {showPayload && occ.id && <SipomPayloadModal occurrenceId={occ.id} onClose={() => setShowPayload(false)} />}

      {(blocking.length > 0 || notes.length > 0) && (
        <ul className="sipom-pending">
          {blocking.map((p) => (
            <li key={p.code} className="sipom-pending--block">
              {p.label}
            </li>
          ))}
          {notes.map((p) => (
            <li key={p.code} className="sipom-pending--note">
              {p.label}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function Field({
  label,
  value,
  missing,
  tag,
  action,
  children,
}: {
  label: string;
  value: string;
  missing?: boolean;
  tag?: string;
  action?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <div className={missing ? "sipom-missing" : undefined}>
      <dt>
        {label}
        {tag && <span className="sipom-tag">{tag}</span>}
        {action}
      </dt>
      {children ?? <dd>{value || "—"}</dd>}
    </div>
  );
}

function AutoButton({ busy, onClick }: { busy: boolean; onClick: () => void }) {
  return (
    <button type="button" className="btn btn-ghost btn-sm" disabled={busy} onClick={onClick} title="Voltar ao cálculo automático">
      <RotateCcw size={12} strokeWidth={2} /> AUTOMÁTICO
    </button>
  );
}

function AddressEditor({
  logradouro,
  numeral,
  busy,
  manual,
  onSave,
  onCancel,
  onReset,
}: {
  logradouro: string;
  numeral: string;
  busy: boolean;
  manual: boolean;
  onSave: (logradouro: string, numeral: string) => void;
  onCancel: () => void;
  onReset: () => void;
}) {
  const [log, setLog] = useState(logradouro);
  const [num, setNum] = useState(numeral);
  return (
    <div className="ops-kv-wide sipom-addr-edit">
      <label className="form-field">
        <span>LOGRADOURO</span>
        <input type="text" value={log} onChange={(e) => setLog(e.target.value)} maxLength={200} />
      </label>
      <label className="form-field sipom-addr-num">
        <span>NÚMERO</span>
        <input type="text" value={num} onChange={(e) => setNum(e.target.value)} maxLength={60} />
      </label>
      <div className="sipom-inline">
        <button type="button" className="btn btn-primary btn-sm" disabled={busy || !log.trim()} onClick={() => onSave(log, num)}>
          SALVAR
        </button>
        <button type="button" className="btn btn-ghost btn-sm" disabled={busy} onClick={onCancel}>
          CANCELAR
        </button>
        {manual && <AutoButton busy={busy} onClick={onReset} />}
      </div>
    </div>
  );
}

// Ficha com mais de uma equipe: o analista diz em qual estava cada policial;
// tipo de policiamento e função saem da regra VTRA/RAIO dentro de cada equipe.
function TeamsEditor({
  officers,
  equipes,
  busy,
  manual,
  onSave,
  onReset,
}: {
  officers: OpsOfficer[];
  equipes: string[];
  busy: boolean;
  manual: boolean;
  onSave: (equipes: string[]) => void;
  onReset: () => void;
}) {
  const [teams, setTeams] = useState<string[]>(() => officers.map((o) => o.sipom_equipe || ""));
  useEffect(() => setTeams(officers.map((o) => o.sipom_equipe || "")), [officers]);
  const complete = teams.every((t) => t);
  return (
    <div className="sipom-teams">
      <div className="sipom-sub">
        EQUIPE DE CADA POLICIAL
        <span className="muted"> · {equipes.join(" · ")}</span>
      </div>
      <table className="tbl ops-subtbl">
        <tbody>
          {officers.map((o, i) => (
            <tr key={i}>
              <td className="mono" style={{ width: 28 }}>
                {i + 1}
              </td>
              <td>
                <span className="muted">{o.rank}</span> {o.war_name}
              </td>
              <td style={{ width: 200 }}>
                <Select
                  value={teams[i]}
                  placeholder="EQUIPE"
                  disabled={busy}
                  onChange={(v) => setTeams((cur) => cur.map((t, k) => (k === i ? v : t)))}
                  options={equipes.map((e) => ({ value: e, label: e }))}
                />
              </td>
              <td className="muted" style={{ width: 140 }}>
                {o.sipom_funcao ? o.sipom_funcao.toUpperCase() : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="sipom-inline" style={{ marginTop: 6 }}>
        <button type="button" className="btn btn-primary btn-sm" disabled={busy || !complete} onClick={() => onSave(teams)}>
          SALVAR EQUIPES
        </button>
        {manual && <AutoButton busy={busy} onClick={onReset} />}
      </div>
    </div>
  );
}

function formatCPF(d: string): string {
  return d.length === 11 ? `${d.slice(0, 3)}.${d.slice(3, 6)}.${d.slice(6, 9)}-${d.slice(9)}` : d;
}
