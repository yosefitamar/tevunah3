"use client";

import { useEffect, useState } from "react";
import { AlertTriangle, Braces, Check, CheckCircle2, Pencil, Plus, RotateCcw, Send, Undo2, X } from "lucide-react";
import type {
  OpsOccurrence,
  OpsOfficer,
  SipomIntel,
  SipomIntelOfficer,
  SipomOccurrence,
  SipomPending,
  SipomRef,
} from "@/lib/ops-reports-api";
import {
  enviarSipom,
  listSipomBairros,
  listSipomIntelAgentes,
  listSipomNaturezas,
  setOpsSipomField,
  setOpsSipomNatureza,
  setSipomIntel,
  setSipomSent,
  setSipomSexo,
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
  // Bairro do relatório que não casa com o catálogo: o SIPOM exige o id, e
  // o analista escolhe entre os bairros da cidade.
  const bairroEditable = editable && !!s.cidade && (pending.has("bairro_catalogo") || manual.has("bairro"));
  const [bairros, setBairros] = useState<SipomRef[]>([]);
  const cidadeId = s.cidade?.id;
  useEffect(() => {
    if (!bairroEditable || !cidadeId) return;
    listSipomBairros(cidadeId)
      .then((r) => setBairros(r.items))
      .catch(() => setBairros([]));
  }, [bairroEditable, cidadeId]);

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

        <OpsField
          label="BAIRRO"
          span={bairroEditable ? 2 : 1}
          value={s.bairro?.nome ?? occ.place_neighborhood}
          missing={pending.has("bairro") || pending.has("bairro_catalogo")}
          tag={manual.has("bairro") ? "ANALISTA" : undefined}
        >
          {bairroEditable && (
            <div className="ops-control-row">
              <Select
                searchable
                value={s.bairro ? String(s.bairro.id) : ""}
                placeholder="ESCOLHA O BAIRRO DO SIPOM"
                disabled={busy || bairros.length === 0}
                onChange={(v) => ed.setField({ field: "bairro", bairro_id: Number(v) })}
                options={bairros.map((b) => ({ value: String(b.id), label: b.nome }))}
              />
              {manual.has("bairro") && <AutoButton busy={busy} onClick={() => ed.setField({ field: "bairro", bairro_id: null })} />}
            </div>
          )}
        </OpsField>
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
          REFERÊNCIA GRAVADA:{" "}
          {areaRule.city_wide
            ? `${areaRule.city} (CIDADE INTEIRA)`
            : [areaRule.neighborhood || "SEM BAIRRO", areaRule.city].join(" · ")}{" "}
          → {areaRule.area}.{" "}
          {areaRule.city_wide
            ? "AS PRÓXIMAS OCORRÊNCIAS DESTA CIDADE, EM QUALQUER BAIRRO, JÁ ENTRAM COM ESTA ÁREA"
            : "AS PRÓXIMAS OCORRÊNCIAS DESTE LOCAL JÁ ENTRAM COM ESTA ÁREA"}
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
      {s.inteligencia && <IntelComposition occ={occ} intel={s.inteligencia} ed={ed} />}
    </>
  );
}

/**
 * Composição da inteligência: com participação da inteligência, os agentes
 * da agência marcados no cadastro entram junto com a guarnição do relatório,
 * no tipo de policiamento Inteligência. O analista insere ou retira agentes
 * e troca a função antes do envio; a escolha fica gravada na ficha.
 */
function IntelComposition({ occ, intel, ed }: { occ: OpsOccurrence; intel: SipomIntel; ed: SipomEdit }) {
  const { editable, busy } = ed;
  const [candidates, setCandidates] = useState<SipomIntelOfficer[]>([]);
  const [funcoes, setFuncoes] = useState<SipomRef[]>([]);
  const [adding, setAdding] = useState("");

  useEffect(() => {
    if (!editable) return;
    listSipomIntelAgentes()
      .then((r) => {
        setCandidates(r.items);
        setFuncoes(r.funcoes);
      })
      .catch(() => setCandidates([]));
  }, [editable]);

  const save = (agentes: { user_id: string; funcao_id: number }[]) =>
    ed.run(() => setSipomIntel(occ.id!, { agentes }));
  const current = intel.agentes.map((a) => ({ user_id: a.user_id, funcao_id: a.funcao_id }));
  const available = candidates.filter((c) => !intel.agentes.some((a) => a.user_id === c.user_id));

  return (
    <div className="sipom-intel">
      <div className="ops-sub">
        INTELIGÊNCIA · AGENTES DA AGÊNCIA
        <span className="muted">
          {" "}
          · {intel.policiamento.toUpperCase()} · {intel.manual ? "DEFINIDA NA FICHA" : "CONFORME O CADASTRO DE AGENTES"}
        </span>
      </div>
      {intel.agentes.length === 0 ? (
        <div className="ops-empty">
          NENHUM AGENTE NA COMPOSIÇÃO DA INTELIGÊNCIA
          {!intel.manual && " — MARQUE \"ENTRAR NA COMPOSIÇÃO DO SIPOM\" NO CADASTRO DOS AGENTES"}
        </div>
      ) : (
        <table className="tbl ops-subtbl">
          <thead>
            <tr>
              <th style={{ width: 36 }}>#</th>
              <th>POSTO/GRAD.</th>
              <th>NOME DE GUERRA</th>
              <th>NÚMERO</th>
              <th>MATRÍCULA</th>
              <th style={{ width: editable ? 160 : undefined }}>FUNÇÃO</th>
              {editable && <th style={{ width: 40 }} />}
            </tr>
          </thead>
          <tbody>
            {intel.agentes.map((a, i) => (
              <tr key={a.user_id} className={a.na_guarnicao ? "muted" : undefined}>
                <td className="mono">{i + 1}</td>
                <td>{a.rank || "—"}</td>
                <td className="ops-strong">
                  {a.war_name || a.name.toUpperCase()}
                  {a.na_guarnicao && (
                    <span className="pill cold" title="A matrícula já está na guarnição do relatório: não entra de novo">
                      JÁ NA GUARNIÇÃO
                    </span>
                  )}
                </td>
                <td className="mono">{a.number || "—"}</td>
                <td className="mono">{a.registration || "—"}</td>
                <td>
                  {editable && funcoes.length > 0 ? (
                    <Select
                      value={String(a.funcao_id)}
                      disabled={busy}
                      onChange={(v) =>
                        save(current.map((c) => (c.user_id === a.user_id ? { ...c, funcao_id: Number(v) } : c)))
                      }
                      options={funcoes.map((f) => ({ value: String(f.id), label: f.nome.toUpperCase() }))}
                    />
                  ) : (
                    a.funcao.toUpperCase() || "—"
                  )}
                </td>
                {editable && (
                  <td>
                    <button
                      type="button"
                      className="action-btn"
                      disabled={busy}
                      title="Retirar da composição"
                      aria-label={`Retirar ${a.war_name || a.name} da composição`}
                      onClick={() => save(current.filter((c) => c.user_id !== a.user_id))}
                    >
                      <X size={13} />
                    </button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {editable && (
        <OpsActions>
          {available.length > 0 && (
            <div className="ops-control-row">
              <Select
                value={adding}
                placeholder="INSERIR AGENTE"
                disabled={busy}
                onChange={setAdding}
                options={available.map((c) => ({
                  value: c.user_id,
                  label: [c.rank, c.war_name || c.name.toUpperCase(), c.registration].filter(Boolean).join(" · "),
                }))}
              />
              <button
                type="button"
                className="btn btn-primary btn-sm ops-nowrap"
                disabled={busy || !adding}
                onClick={async () => {
                  const c = available.find((x) => x.user_id === adding);
                  if (c && (await save([...current, { user_id: c.user_id, funcao_id: c.funcao_id }]))) setAdding("");
                }}
              >
                <Plus size={12} strokeWidth={2} /> INSERIR
              </button>
            </div>
          )}
          {intel.manual && (
            <AutoButton busy={busy} label="CONFORME O CADASTRO" onClick={() => ed.run(() => setSipomIntel(occ.id!, { reset: true }))} />
          )}
        </OpsActions>
      )}
    </div>
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

const SEXO_LABEL: Record<number, string> = { 1: "MASCULINO", 2: "FEMININO" };
const SEXO_FONTE_LABEL: Record<string, string> = {
  nome: "PELO NOME",
  aprendido: "ESCOLHA ANTERIOR",
  historico: "PELO HISTÓRICO",
};

/**
 * Envolvidos como vão ao SIPOM (qualificação do dossiê, quando há vínculo).
 * O SIPOM exige o sexo de toda pessoa identificada: quando o dossiê não diz,
 * o analista informa aqui — ou confirma o sugerido (pelo nome, por escolha
 * anterior para o mesmo nome ou pelo histórico).
 */
export function SipomEnvolvidos({ occ, ed }: { occ: OpsOccurrence; ed?: SipomEdit }) {
  const s = occ.sipom!;
  const canSetSex = (p: (typeof s.pessoas)[number]) =>
    !!ed?.editable && !!occ.id && !!p.id && !!p.nome && (p.sexo === 0 || p.sexo_ficha);
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
              <td>
                {canSetSex(p) ? (
                  <div className="ops-control-row">
                    <Select
                      value={p.sexo ? String(p.sexo) : ""}
                      placeholder={p.sexo_sugerido ? `${SEXO_LABEL[p.sexo_sugerido]}?` : "INFORME"}
                      disabled={ed!.busy}
                      onChange={(v) => ed!.run(() => setSipomSexo(occ.id!, p.id, Number(v) as 1 | 2))}
                      options={[
                        { value: "1", label: "MASCULINO" },
                        { value: "2", label: "FEMININO" },
                      ]}
                    />
                    {!p.sexo && p.sexo_sugerido > 0 && (
                      <button
                        type="button"
                        className="btn btn-primary btn-sm ops-nowrap"
                        disabled={ed!.busy}
                        title={`Sugerido ${SEXO_FONTE_LABEL[p.sexo_fonte]?.toLowerCase() ?? ""}`}
                        onClick={() => ed!.run(() => setSipomSexo(occ.id!, p.id, p.sexo_sugerido as 1 | 2))}
                      >
                        <Check size={12} strokeWidth={2} /> {SEXO_LABEL[p.sexo_sugerido]} · {SEXO_FONTE_LABEL[p.sexo_fonte]}
                      </button>
                    )}
                  </div>
                ) : p.sexo === 1 ? (
                  "MASCULINO"
                ) : p.sexo === 2 ? (
                  "FEMININO"
                ) : (
                  "—"
                )}
              </td>
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
  if (s.envio?.excluida_em) {
    return (
      <span className="pill crit sipom-ready" title="O SIPOM informou que a ocorrência foi excluída lá e não a recria">
        <AlertTriangle size={10} strokeWidth={2} /> EXCLUÍDA NO SIPOM
      </span>
    );
  }
  if (s.sent_at) {
    const num = s.envio?.identificador;
    return (
      <span className="pill info sipom-ready" title={`Enviada em ${formatBRDate(s.sent_at.slice(0, 10))}`}>
        <Send size={10} strokeWidth={2} /> ENVIADA AO SIPOM{num ? ` · ${num}` : ""}
      </span>
    );
  }
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
  onSent,
}: {
  /** Quando presente, a ficha pode marcar a ocorrência como enviada. */
  onSent?: (s: SipomOccurrence) => void;
  occ: OpsOccurrence;
  /** Etapa (rótulo) em que a pendência se resolve. */
  stepOf?: (code: string) => { key: string; label: string } | null;
  onGoTo?: (stepKey: string) => void;
}) {
  const s = occ.sipom!;
  const [showPayload, setShowPayload] = useState(false);
  const [busySent, setBusySent] = useState(false);
  const [sentError, setSentError] = useState<string | null>(null);
  const [sendDetails, setSendDetails] = useState<string[]>([]);
  const blocking = s.pendencias.filter((p) => p.blocking);
  const envio = s.envio;

  // Envio pela API: monta o corpo no servidor (com as fotos) e manda. A
  // resposta traz a ficha já com o número da ocorrência no SIPOM.
  async function send() {
    if (!occ.id || busySent) return;
    setBusySent(true);
    setSentError(null);
    setSendDetails([]);
    try {
      const r = await enviarSipom(occ.id);
      onSent?.(r.sipom);
    } catch (e) {
      const err = e as ApiError;
      setSentError(err.message || "Erro no envio");
      // 409: pendências; 502 com 422 do SIPOM: erros por campo.
      if (Array.isArray(err.details)) {
        setSendDetails((err.details as SipomPending[]).map((p) => p.label || p.code));
      } else if (err.details && typeof err.details === "object") {
        setSendDetails(Object.entries(err.details as Record<string, string>).map(([k, v]) => `${k}: ${v}`));
      }
    } finally {
      setBusySent(false);
    }
  }

  // Sem a API configurada, quem diz que a ocorrência foi lançada lá é o
  // analista: marcar tira a ocorrência da fila de envio.
  async function markSent(sent: boolean) {
    if (!occ.id || busySent) return;
    setBusySent(true);
    setSentError(null);
    try {
      const r = await setSipomSent(occ.id, sent);
      onSent?.({ ...s, sent_at: r.sent_at });
    } catch (e) {
      setSentError((e as ApiError).message || "Erro ao gravar");
    } finally {
      setBusySent(false);
    }
  }
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
        {s.api && onSent && occ.id && s.ready && !envio?.excluida_em && (
          <button
            type="button"
            className={"btn btn-sm" + (s.sent_at ? "" : " btn-primary")}
            disabled={busySent}
            onClick={send}
            title={
              s.sent_at
                ? "Reenvia a ficha: o SIPOM atualiza o que mudou (ou responde que nada mudou)"
                : "Envia a ocorrência ao SIPOM pela API"
            }
          >
            <Send size={12} strokeWidth={2} /> {busySent ? "ENVIANDO…" : s.sent_at ? "REENVIAR AO SIPOM" : "ENVIAR AO SIPOM"}
          </button>
        )}
        {!s.api && onSent && occ.id && s.ready && !s.sent_at && (
          <button
            type="button"
            className="btn btn-sm"
            disabled={busySent}
            onClick={() => markSent(true)}
            title="Use depois de lançar a ocorrência no SIPOM: ela sai da fila de envio"
          >
            <Send size={12} strokeWidth={2} /> MARCAR COMO ENVIADA
          </button>
        )}
        {!s.api && onSent && occ.id && s.sent_at && (
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            disabled={busySent}
            onClick={() => markSent(false)}
            title="Devolve a ocorrência à fila de envio"
          >
            <Undo2 size={12} strokeWidth={2} /> DESFAZER
          </button>
        )}
      </div>
      {sentError && (
        <div className="banner banner-error">
          ⚠ {sentError}
          {sendDetails.map((d) => (
            <div key={d}>· {d}</div>
          ))}
        </div>
      )}
      {!sentError && envio?.erro && (
        <div className="banner banner-error">
          ⚠ ÚLTIMA TENTATIVA ({formatBRDate(envio.tentativa_em.slice(0, 10))}): {envio.erro}
        </div>
      )}
      {envio && !envio.erro && envio.identificador && (
        <div className="banner banner-info">
          OCORRÊNCIA Nº <span className="mono">{envio.identificador}</span> NO SIPOM ·{" "}
          {SITUACAO_LABEL[envio.situacao] ?? envio.situacao}
          {envio.avisos.map((a) => (
            <div key={a.campo + a.codigo}>
              ⚠ {AVISO_LABEL[a.codigo] ?? a.codigo}: <span className="mono">{a.campo}</span>
            </div>
          ))}
        </div>
      )}
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

const SITUACAO_LABEL: Record<string, string> = {
  criado: "CRIADA",
  atualizado: "ATUALIZADA",
  sem_alteracao: "SEM ALTERAÇÃO (O SIPOM JÁ TINHA ESTA VERSÃO)",
};

const AVISO_LABEL: Record<string, string> = {
  matricula_nao_encontrada: "MATRÍCULA NÃO ENCONTRADA NO SIPOM — O POLICIAL FICOU FORA DA COMPOSIÇÃO",
  numeral_nao_numerico: "NÚMERO DO ENDEREÇO NÃO NUMÉRICO — FICOU VAZIO NO SIPOM",
};

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
