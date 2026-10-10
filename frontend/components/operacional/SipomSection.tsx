"use client";

import { useEffect, useState } from "react";
import { AlertTriangle, Braces, Check, CheckCircle2, Pencil, RotateCcw } from "lucide-react";
import type { OpsOccurrence, OpsOfficer, SipomOccurrence, SipomRef } from "@/lib/ops-reports-api";
import {
  listSipomNaturezas,
  setOpsSipomField,
  setOpsSipomNatureza,
  type SipomAreaRule,
  type SipomFieldInput,
  type SipomNatureza,
} from "@/lib/sipom-api";
import { formatBRDate } from "@/lib/format";
import type { ApiError } from "@/lib/api";
import Select from "../shared/Select";
import { OpsActions, OpsField, OpsFields } from "./OpsField";
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

type SipomResult = { sipom: SipomOccurrence; officers?: OpsOfficer[]; area_rule?: SipomAreaRule | null };

/**
 * Estado das correções do envio ao SIPOM, compartilhado pelas etapas da ficha:
 * os campos obrigatórios do cadastro deles ficam espalhados por tema (fato,
 * local, efetivo, procedimento, materiais), mas a gravação, o erro e o
 * "ocupado" são um só. O que o analista fixa vale sobre o automático até ele
 * voltar ao automático.
 */
export type SipomEdit = {
  /** Ficha gravada + permissão de edição: as correções ficam disponíveis. */
  editable: boolean;
  busy: boolean;
  error: string | null;
  /** Referência de área que a última escolha do analista criou. */
  areaRule: SipomAreaRule | null;
  pending: Set<string>;
  manual: Set<string>;
  run: (fn: () => Promise<SipomResult>) => Promise<boolean>;
  setNatureza: (id: number | null) => Promise<boolean>;
  setField: (input: SipomFieldInput) => Promise<boolean>;
};

export function useSipomEdit(
  occ: OpsOccurrence,
  editable: boolean,
  onChange?: (s: SipomOccurrence, officers?: OpsOfficer[]) => void,
): SipomEdit {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [areaRule, setAreaRule] = useState<SipomAreaRule | null>(null);

  async function run(fn: () => Promise<SipomResult>) {
    setBusy(true);
    setError(null);
    setAreaRule(null);
    try {
      const r = await fn();
      setAreaRule(r.area_rule ?? null);
      onChange?.(r.sipom, r.officers);
      return true;
    } catch (e) {
      setError((e as ApiError).message || "Erro ao gravar");
      return false;
    } finally {
      setBusy(false);
    }
  }

  return {
    editable,
    busy,
    error,
    areaRule,
    pending: new Set(occ.sipom?.pendencias.map((p) => p.code)),
    manual: new Set(occ.sipom?.manual),
    run,
    setNatureza: async (id) => !!occ.id && run(() => setOpsSipomNatureza(occ.id!, id)),
    setField: async (input) => !!occ.id && run(() => setOpsSipomField(occ.id!, input)),
  };
}

type PartProps = { occ: OpsOccurrence; ed: SipomEdit };

/** Fato: natureza, data e hora e número da ocorrência como vão ao SIPOM. */
export function SipomFato({ occ, ed }: PartProps) {
  const s = occ.sipom!;
  const { editable, busy, pending, manual } = ed;
  const [naturezas, setNaturezas] = useState<SipomNatureza[]>([]);
  const [loadError, setLoadError] = useState(false);

  useEffect(() => {
    if (!editable) return;
    loadNaturezas()
      .then(setNaturezas)
      .catch(() => setLoadError(true));
  }, [editable]);

  const toConfirm = pending.has("natureza_confirmar");
  const dataHora = occ.start_time ? `${formatBRDate(occ.occurred_on)} ${occ.start_time}` : "";
  const origem = !s.natureza
    ? "SEM CORRESPONDÊNCIA"
    : manual.has("natureza")
      ? "ANALISTA"
      : toConfirm
        ? "SUGERIDA"
        : "DE-PARA";

  return (
    <>
      {loadError && <div className="banner banner-error">⚠ NÃO FOI POSSÍVEL CARREGAR AS NATUREZAS DO SIPOM</div>}
      <OpsFields edit={editable}>
        <OpsField
          label="NATUREZA"
          span={4}
          tag={origem}
          missing={pending.has("natureza") || toConfirm}
          value={s.natureza?.nome}
        >
          {editable && (
            <div className="ops-control-row">
              <Select
                searchable
                value={s.natureza ? String(s.natureza.id) : ""}
                placeholder="ESCOLHA A NATUREZA DO SIPOM"
                disabled={busy || naturezas.length === 0}
                onChange={(v) => ed.setNatureza(Number(v))}
                options={
                  naturezas.length || !s.natureza
                    ? naturezas.map((n) => ({ value: String(n.id), label: n.rotulo }))
                    : [{ value: String(s.natureza.id), label: s.natureza.nome }]
                }
              />
              {toConfirm && s.natureza && (
                <button type="button" className="btn btn-primary btn-sm" disabled={busy} onClick={() => ed.setNatureza(s.natureza!.id)}>
                  <Check size={12} strokeWidth={2} /> CONFIRMAR
                </button>
              )}
              {manual.has("natureza") && <AutoButton busy={busy} onClick={() => ed.setNatureza(null)} />}
            </div>
          )}
        </OpsField>
        <OpsField label="DATA E HORA" span={2} value={dataHora} missing={pending.has("data_hora")} />
        <OpsField label="Nº DA OCORRÊNCIA" span={2} mono value={occ.ciops_record} missing={pending.has("ficha")} />
      </OpsFields>
    </>
  );
}

/** Local: endereço nos códigos do SIPOM e a área da unidade militar. */
export function SipomLocal({ occ, ed }: PartProps) {
  const s = occ.sipom!;
  const { editable, busy, pending, manual, areaRule } = ed;
  const [editAddr, setEditAddr] = useState(false);
  const [log, setLog] = useState(s.logradouro);
  const [num, setNum] = useState(s.numeral);

  useEffect(() => {
    setLog(s.logradouro);
    setNum(s.numeral);
  }, [s.logradouro, s.numeral]);

  // A área aprendida também pode ser trocada: corrigir aqui corrige a
  // referência do lugar.
  const areaEditable =
    editable && (pending.has("area") || pending.has("area_ambigua") || manual.has("area") || s.area_learned);
  const areaChoices: SipomRef[] = s.area_options.length ? s.area_options : s.area_candidates;
  const addrTag = manual.has("endereco") ? "ANALISTA" : undefined;

  return (
    <>
      <OpsFields edit={editable}>
        <OpsField
          label="LOGRADOURO"
          span={3}
          value={s.logradouro}
          missing={pending.has("logradouro")}
          tag={addrTag}
          action={
            editable && !editAddr ? (
              <button type="button" className="ops-field-btn" onClick={() => setEditAddr(true)} title="Corrigir o endereço do envio">
                <Pencil size={11} /> CORRIGIR
              </button>
            ) : undefined
          }
        >
          {editAddr && (
            <input className="ops-input" type="text" value={log} onChange={(e) => setLog(e.target.value)} maxLength={200} autoFocus />
          )}
        </OpsField>
        <OpsField label="NÚMERO" value={s.numeral}>
          {editAddr && <input className="ops-input" type="text" value={num} onChange={(e) => setNum(e.target.value)} maxLength={60} />}
        </OpsField>
        {editAddr && (
          <OpsActions>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              disabled={busy || !log.trim()}
              onClick={async () => {
                if (await ed.setField({ field: "endereco", logradouro: log, numeral: num })) setEditAddr(false);
              }}
            >
              <Check size={12} strokeWidth={2} /> SALVAR ENDEREÇO
            </button>
            <button type="button" className="btn btn-ghost btn-sm" disabled={busy} onClick={() => setEditAddr(false)}>
              CANCELAR
            </button>
            {manual.has("endereco") && (
              <AutoButton
                busy={busy}
                onClick={async () => {
                  if (await ed.setField({ field: "endereco", reset: true })) setEditAddr(false);
                }}
              />
            )}
          </OpsActions>
        )}

        <OpsField label="BAIRRO" value={s.bairro?.nome ?? occ.place_neighborhood} missing={pending.has("bairro")} />
        <OpsField label="CIDADE" value={s.cidade?.nome ?? occ.place_city} missing={pending.has("cidade")} />
        <OpsField
          label="ÁREA DA UNIDADE MILITAR"
          span={2}
          missing={pending.has("area") || pending.has("area_ambigua")}
          tag={manual.has("area") ? "ANALISTA" : s.area_learned ? "REFERÊNCIA" : undefined}
          value={s.area?.nome}
        >
          {areaEditable && (
            <div className="ops-control-row">
              <Select
                value={s.area ? String(s.area.id) : ""}
                placeholder={areaChoices.length ? "ESCOLHA A ÁREA" : "SEM ÁREA NA CIDADE"}
                disabled={busy || areaChoices.length === 0}
                onChange={(v) => ed.setField({ field: "area", area_id: Number(v) })}
                options={areaChoices.map((a) => ({ value: String(a.id), label: a.nome }))}
              />
              {manual.has("area") && <AutoButton busy={busy} onClick={() => ed.setField({ field: "area", area_id: null })} />}
            </div>
          )}
        </OpsField>
      </OpsFields>
      {areaRule && (
        <div className="banner banner-info">
          REFERÊNCIA GRAVADA: {[areaRule.neighborhood || "SEM BAIRRO", areaRule.city].join(" · ")} → {areaRule.area}.
          AS PRÓXIMAS OCORRÊNCIAS DESTE LOCAL JÁ ENTRAM COM ESTA ÁREA
          {areaRule.resolved > 0 &&
            ` — ${areaRule.resolved} OUTRA${areaRule.resolved > 1 ? "S" : ""} DO ACERVO ${
              areaRule.resolved > 1 ? "FORAM RESOLVIDAS" : "FOI RESOLVIDA"
            }`}
          .
        </div>
      )}
    </>
  );
}

/** Efetivo: companhia que atendeu, tipo de policiamento e a equipe de cada policial. */
export function SipomEfetivo({ occ, ed }: PartProps) {
  const s = occ.sipom!;
  const { editable, busy, pending, manual } = ed;
  const opmEditable = editable && (pending.has("opm") || manual.has("opm"));
  const compEditable =
    editable && occ.officers.length > 0 && (pending.has("composicao_multi") || manual.has("composicao"));

  return (
    <>
      <OpsFields edit={editable}>
        <OpsField
          label="OPM QUE ATENDEU"
          span={2}
          value={s.opm?.nome}
          missing={pending.has("opm")}
          tag={manual.has("opm") ? "ANALISTA" : undefined}
        >
          {opmEditable && (
            <div className="ops-control-row">
              <Select
                value={s.opm ? String(s.opm.id) : ""}
                placeholder="ESCOLHA A COMPANHIA"
                disabled={busy || s.opm_candidates.length === 0}
                onChange={(v) => ed.setField({ field: "opm", opm_id: Number(v) })}
                options={s.opm_candidates.map((c) => ({ value: String(c.id), label: c.nome }))}
              />
              {manual.has("opm") && <AutoButton busy={busy} onClick={() => ed.setField({ field: "opm", opm_id: null })} />}
            </div>
          )}
        </OpsField>
        <OpsField
          label="TIPO DE POLICIAMENTO"
          span={2}
          value={occ.officers[0]?.sipom_policiamento}
          missing={pending.has("composicao") || pending.has("composicao_multi")}
          tag={manual.has("composicao") ? "ANALISTA" : undefined}
        />
      </OpsFields>
      {compEditable ? (
        <TeamsEditor
          officers={occ.officers}
          equipes={s.equipes}
          busy={busy}
          manual={manual.has("composicao")}
          onSave={(equipes) => ed.setField({ field: "composicao", equipes })}
          onReset={() => ed.setField({ field: "composicao", reset: true })}
        />
      ) : (
        <OfficersTable officers={occ.officers} />
      )}
    </>
  );
}

/** Composição da guarnição, com a função que cada policial leva ao SIPOM. */
export function OfficersTable({ officers }: { officers: OpsOfficer[] }) {
  if (officers.length === 0) return null;
  return (
    <table className="tbl ops-subtbl">
      <thead>
        <tr>
          <th style={{ width: 36 }}>#</th>
          <th>POSTO/GRAD.</th>
          <th>NOME DE GUERRA</th>
          <th>NÚMERO</th>
          <th>MATRÍCULA</th>
          <th>EQUIPE</th>
          <th>FUNÇÃO</th>
        </tr>
      </thead>
      <tbody>
        {officers.map((f, i) => (
          <tr key={i}>
            <td className="mono">{i + 1}</td>
            <td>{f.rank || "—"}</td>
            <td className="ops-strong">{f.war_name || "—"}</td>
            <td className="mono">{f.number || "—"}</td>
            <td className="mono">{f.registration || "—"}</td>
            <td>{f.sipom_equipe || "—"}</td>
            <td>{f.sipom_funcao || "—"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

/** Envolvidos como vão ao SIPOM (qualificação do dossiê, quando há vínculo). */
export function SipomEnvolvidos({ occ }: { occ: OpsOccurrence }) {
  const s = occ.sipom!;
  if (s.pessoas.length === 0) return <div className="ops-empty">NENHUM ENVOLVIDO VAI AO SIPOM</div>;
  return (
    <div className="ops-tbl-scroll">
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
              <td className="ops-strong">{p.nome || "—"}</td>
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
    </div>
  );
}

/** Selo de situação do envio: pronta, ou quantas pendências impedem. */
export function SipomReadyPill({ s }: { s: SipomOccurrence }) {
  const blocking = s.pendencias.filter((p) => p.blocking).length;
  return s.ready ? (
    <span className="pill active sipom-ready">
      <CheckCircle2 size={10} strokeWidth={2} /> PRONTA PARA O SIPOM
    </span>
  ) : (
    <span className="pill hold sipom-ready">
      <AlertTriangle size={10} strokeWidth={2} /> {blocking} PENDÊNCIA{blocking === 1 ? "" : "S"} PARA O SIPOM
    </span>
  );
}

/**
 * Conferência do envio: o que ainda impede, os avisos e a prévia do que será
 * enviado. `onGoTo` leva o analista à etapa onde a pendência se resolve.
 */
export function SipomEnvio({
  occ,
  stepOf,
  onGoTo,
}: {
  occ: OpsOccurrence;
  /** Etapa (rótulo) em que a pendência se resolve. */
  stepOf?: (code: string) => { key: string; label: string } | null;
  onGoTo?: (stepKey: string) => void;
}) {
  const s = occ.sipom!;
  const [showPayload, setShowPayload] = useState(false);
  const blocking = s.pendencias.filter((p) => p.blocking);
  const notes = s.pendencias.filter((p) => !p.blocking);

  const item = (p: (typeof s.pendencias)[number]) => {
    const step = stepOf?.(p.code);
    return (
      <li key={p.code} className={p.blocking ? "sipom-pending--block" : "sipom-pending--note"}>
        <span>{p.label}</span>
        {step && onGoTo && (
          <button type="button" className="ops-field-btn" onClick={() => onGoTo(step.key)}>
            IR PARA {step.label}
          </button>
        )}
      </li>
    );
  };

  return (
    <>
      <div className="sipom-status">
        <SipomReadyPill s={s} />
        {s.ready && occ.id && (
          <button type="button" className="btn btn-sm" onClick={() => setShowPayload(true)}>
            <Braces size={12} strokeWidth={2} /> PRÉVIA DO ENVIO
          </button>
        )}
      </div>
      {blocking.length > 0 && (
        <>
          <div className="ops-sub">IMPEDEM O ENVIO ({blocking.length})</div>
          <ul className="sipom-pending">{blocking.map(item)}</ul>
        </>
      )}
      {notes.length > 0 && (
        <>
          <div className="ops-sub">AVISOS ({notes.length})</div>
          <ul className="sipom-pending">{notes.map(item)}</ul>
        </>
      )}
      {blocking.length === 0 && notes.length === 0 && <div className="ops-empty">NENHUMA PENDÊNCIA</div>}
      {showPayload && occ.id && <SipomPayloadModal occurrenceId={occ.id} onClose={() => setShowPayload(false)} />}
    </>
  );
}

export function AutoButton({
  busy,
  onClick,
  label = "AUTOMÁTICO",
}: {
  busy: boolean;
  onClick: () => void;
  label?: string;
}) {
  return (
    <button type="button" className="btn btn-ghost btn-sm" disabled={busy} onClick={onClick} title="Voltar ao cálculo automático">
      <RotateCcw size={12} strokeWidth={2} /> {label}
    </button>
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
      <div className="ops-sub">
        EQUIPE DE CADA POLICIAL
        <span className="muted"> · {equipes.join(" · ")}</span>
      </div>
      <table className="tbl ops-subtbl">
        <thead>
          <tr>
            <th style={{ width: 36 }}>#</th>
            <th>POSTO/GRAD.</th>
            <th>NOME DE GUERRA</th>
            <th style={{ width: 220 }}>EQUIPE</th>
            <th style={{ width: 160 }}>FUNÇÃO</th>
          </tr>
        </thead>
        <tbody>
          {officers.map((o, i) => (
            <tr key={i}>
              <td className="mono">{i + 1}</td>
              <td>{o.rank || "—"}</td>
              <td className="ops-strong">{o.war_name}</td>
              <td>
                <Select
                  value={teams[i]}
                  placeholder="EQUIPE"
                  disabled={busy}
                  onChange={(v) => setTeams((cur) => cur.map((t, k) => (k === i ? v : t)))}
                  options={equipes.map((e) => ({ value: e, label: e }))}
                />
              </td>
              <td>{o.sipom_funcao || "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <OpsActions>
        <button type="button" className="btn btn-primary btn-sm" disabled={busy || !complete} onClick={() => onSave(teams)}>
          <Check size={12} strokeWidth={2} /> SALVAR EQUIPES
        </button>
        {manual && <AutoButton busy={busy} onClick={onReset} />}
      </OpsActions>
    </div>
  );
}

function formatCPF(d: string): string {
  return d.length === 11 ? `${d.slice(0, 3)}.${d.slice(3, 6)}.${d.slice(6, 9)}-${d.slice(9)}` : d;
}
