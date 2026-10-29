"use client";

import { Fragment, useEffect, useState } from "react";
import { Check, Search } from "lucide-react";
import type { OpsOccurrence, SipomArma, SipomDroga, SipomRef, SipomVeiculo } from "@/lib/ops-reports-api";
import { formatGrams } from "@/lib/ops-reports-api";
import {
  getSipomCatalogo,
  searchSipomMarcasModelos,
  setOpsSipomMaterial,
  setOpsSipomProcedimento,
  setSipomVehiclePlate,
  setSipomVehicleYears,
  type SipomCatalogo,
  type SipomMaterialInput,
  type SipomProcedimentoInput,
} from "@/lib/sipom-api";
import Select from "../shared/Select";
import { OpsActions, OpsField, OpsFields } from "./OpsField";
import { AutoButton, type SipomEdit } from "./SipomSection";

// Listas do SIPOM (procedimentos, delegacias, delegados, armas, drogas,
// veículos): não mudam durante a sessão — uma busca serve às duas etapas e a
// todas as fichas.
let catalogoCache: Promise<SipomCatalogo> | null = null;
function loadCatalogo(): Promise<SipomCatalogo> {
  catalogoCache ??= getSipomCatalogo().catch((e) => {
    catalogoCache = null;
    throw e;
  });
  return catalogoCache;
}

function useCatalogo(editable: boolean) {
  const [cat, setCat] = useState<SipomCatalogo | null>(null);
  const [error, setError] = useState(false);
  useEffect(() => {
    if (!editable) return;
    loadCatalogo()
      .then(setCat)
      .catch(() => setError(true));
  }, [editable]);
  return { cat, error };
}

const CAT_ERROR = <div className="banner banner-error">⚠ NÃO FOI POSSÍVEL CARREGAR AS LISTAS DO SIPOM</div>;

type PartProps = { occ: OpsOccurrence; ed: SipomEdit };

/**
 * Fase 2 do envio, parte 1: o procedimento (tipo, número, delegacia, delegado)
 * nos códigos do SIPOM. O que o analista fixa vira termo aprendido e vale
 * para as próximas ocorrências com o mesmo texto.
 */
export function SipomProcedimento({ occ, ed }: PartProps) {
  const s = occ.sipom!;
  const { editable, busy, pending, manual } = ed;
  const { cat, error } = useCatalogo(editable);
  const p = s.procedimento;
  const setProc = (input: SipomProcedimentoInput) =>
    !!occ.id && ed.run(() => setOpsSipomProcedimento(occ.id!, input));
  const procManual = manual.has("procedimento") || manual.has("delegacia") || manual.has("delegado");
  // Ficha lida por um servidor anterior à lista: vale o procedimento único.
  const itens = p.itens?.length
    ? p.itens
    : [{ procedimento: p.procedimento, numero: p.numero, ano: p.ano, tipo_texto: p.tipo_texto, numero_texto: p.numero_texto }];
  const multi = itens.length > 1;

  return (
    <>
      {error && CAT_ERROR}
      {multi && (
        <div className="banner banner-info">
          A OCORRÊNCIA TEM {itens.length} PROCEDIMENTOS — TODOS VÃO AO SIPOM. TIPO E NÚMERO SÃO CASADOS PELA ORDEM EM QUE O
          RELATÓRIO OS ESCREVE; SE ESTIVEREM TROCADOS, AJUSTE EM CORRIGIR (BLOCO RELATÓRIO).
        </div>
      )}
      <OpsFields edit={editable}>
        {itens.map((it, i) => {
          const n = multi ? ` ${i + 1}` : "";
          // Com um procedimento só, o tipo é escolhível aqui; com vários, cada
          // tipo sai do texto do relatório.
          const pick = editable && !multi;
          return (
            <Fragment key={i}>
              <OpsField
                label={`TIPO${n}`}
                span={2}
                value={it.procedimento?.nome}
                missing={!it.procedimento}
                tag={!multi && manual.has("procedimento") ? "ANALISTA" : undefined}
              >
                {pick && (
                  <Select
                    value={it.procedimento ? String(it.procedimento.id) : ""}
                    placeholder="ESCOLHA O TIPO"
                    disabled={busy || !cat}
                    onChange={(v) => setProc({ procedimento_id: Number(v) })}
                    options={opts(cat?.procedimentos, it.procedimento)}
                  />
                )}
              </OpsField>
              <OpsField
                label={`NÚMERO / ANO${n}`}
                span={2}
                mono
                value={it.numero ? `${it.numero} / ${it.ano}` : ""}
                missing={!it.numero}
              />
            </Fragment>
          );
        })}
        <OpsField
          label="DELEGACIA"
          span={2}
          value={p.delegacia?.nome}
          missing={pending.has("delegacia")}
          tag={manual.has("delegacia") ? "ANALISTA" : undefined}
        >
          {editable && (
            <Select
              searchable
              value={p.delegacia ? String(p.delegacia.id) : ""}
              placeholder="ESCOLHA A DELEGACIA"
              disabled={busy || !cat}
              onChange={(v) => setProc({ delegacia_id: Number(v) })}
              options={opts(cat?.delegacias, p.delegacia)}
            />
          )}
        </OpsField>
        <OpsField
          label="DELEGADO (OPCIONAL)"
          span={2}
          value={p.delegado?.nome}
          missing={pending.has("delegado")}
          tag={manual.has("delegado") ? "ANALISTA" : undefined}
        >
          {editable && (
            <Select
              searchable
              value={p.delegado ? String(p.delegado.id) : ""}
              placeholder="BUSCAR DELEGADO NO CATÁLOGO"
              disabled={busy || !cat}
              onChange={(v) => setProc({ delegado_id: Number(v) })}
              options={opts(cat?.delegados, p.delegado)}
            />
          )}
        </OpsField>
      </OpsFields>
      {editable && p.delegado_candidates.length > 0 && !p.delegado && (
        <>
          <div className="ops-sub">DELEGADOS QUE CASAM COM O NOME DO RELATÓRIO</div>
          <div className="ops-chips">
            {p.delegado_candidates.map((c) => (
              <button key={c.id} type="button" className="btn btn-sm" disabled={busy} onClick={() => setProc({ delegado_id: c.id })}>
                <Check size={12} strokeWidth={2} /> {c.nome}
              </button>
            ))}
          </div>
        </>
      )}
      {editable && procManual && (
        <OpsActions>
          <AutoButton busy={busy} onClick={() => setProc({ reset: true })} label="VOLTAR O PROCEDIMENTO AO AUTOMÁTICO" />
        </OpsActions>
      )}
    </>
  );
}

/**
 * Fase 2 do envio, parte 2: os materiais (armas, drogas, veículos). Cada item
 * mostra o que o relatório trouxe em texto e, embaixo, a tradução para os
 * códigos do SIPOM.
 */
export function SipomMateriais({ occ, ed }: PartProps) {
  const s = occ.sipom!;
  const { editable, busy } = ed;
  const { cat, error } = useCatalogo(editable);
  const setMat = (index: number, input: SipomMaterialInput) =>
    !!occ.id && ed.run(() => setOpsSipomMaterial(occ.id!, index, input));
  const { armas, drogas, veiculos } = s.materiais;

  return (
    <>
      {error && CAT_ERROR}
      {armas.length > 0 && <div className="ops-sub">ARMAS ({armas.length})</div>}
      {armas.map((a) => (
        <ArmaRow key={"a" + a.index} a={a} raw={occ.weapons[a.index]} cat={cat} editable={editable} busy={busy} onSave={(input) => setMat(a.index, input)} />
      ))}
      {drogas.length > 0 && <div className="ops-sub">DROGAS ({drogas.length})</div>}
      {drogas.map((d) => (
        <DrogaRow key={"d" + d.index} d={d} raw={occ.drugs[d.index]} cat={cat} editable={editable} busy={busy} onSave={(input) => setMat(d.index, input)} />
      ))}
      {veiculos.length > 0 && <div className="ops-sub">VEÍCULOS ({veiculos.length})</div>}
      {veiculos.map((v) => (
        <Fragment key={"v" + v.index}>
          <VeiculoRow v={v} raw={occ.vehicles[v.index]} cat={cat} editable={editable} busy={busy} onSave={(input) => setMat(v.index, input)} />
          <VeiculoPlaca
            placa={occ.vehicles[v.index]?.plate ?? ""}
            editable={editable}
            busy={busy}
            onSave={(placa) => !!occ.id && ed.run(() => setSipomVehiclePlate(occ.id!, v.index, placa))}
          />
          <VeiculoAnos
            v={v}
            editable={editable}
            busy={busy}
            onSave={(fab, mod) => !!occ.id && ed.run(() => setSipomVehicleYears(occ.id!, v.index, fab, mod))}
          />
        </Fragment>
      ))}
    </>
  );
}

// ─── Linhas de material ───

type RowProps<T> = {
  cat: SipomCatalogo | null;
  editable: boolean;
  busy: boolean;
  onSave: (input: SipomMaterialInput) => unknown;
} & T;

/** Cabeçalho do item: o texto do relatório e a situação da tradução. */
function MaterialHead({ raw, ok, manual }: { raw: string; ok: boolean; manual: boolean }) {
  return (
    <div className="sipom-material-hd">
      <span className="sipom-material-lbl">RELATÓRIO</span>
      <span className="sipom-material-raw">{raw || "—"}</span>
      {manual && <span className="sipom-tag">ANALISTA</span>}
      <span className={"pill " + (ok ? "active" : "hold")}>{ok ? "OK" : "PENDENTE"}</span>
    </div>
  );
}

/** Tradução só de leitura (prévia da importação ou sem permissão). */
function MaterialRead({ fields }: { fields: [string, string | undefined][] }) {
  return (
    <OpsFields>
      {fields.map(([label, value]) => (
        <OpsField key={label} label={label} value={value} missing={!value} />
      ))}
    </OpsFields>
  );
}

function ArmaRow({ a, raw, cat, editable, busy, onSave }: RowProps<{ a: SipomArma; raw?: OpsOccurrence["weapons"][number] }>) {
  const [tipo, setTipo] = useState(a.tipo ? String(a.tipo.id) : "");
  const [marca, setMarca] = useState(a.marca ? String(a.marca.id) : "");
  const [calibre, setCalibre] = useState(a.calibre ? String(a.calibre.id) : "");
  useEffect(() => {
    setTipo(a.tipo ? String(a.tipo.id) : "");
    setMarca(a.marca ? String(a.marca.id) : "");
    setCalibre(a.calibre ? String(a.calibre.id) : "");
  }, [a.tipo, a.marca, a.calibre]);
  const rawText = raw ? [raw.kind, raw.brand, raw.model, raw.caliber, raw.serial].filter(Boolean).join(" · ") : "";
  const dirty =
    tipo !== (a.tipo ? String(a.tipo.id) : "") ||
    marca !== (a.marca ? String(a.marca.id) : "") ||
    calibre !== (a.calibre ? String(a.calibre.id) : "");
  return (
    <div className="sipom-material">
      <MaterialHead raw={rawText} ok={a.ok} manual={a.manual} />
      {editable ? (
        <>
          <OpsFields edit>
            <OpsField label="TIPO" missing={!tipo}>
              <Select value={tipo} placeholder="TIPO" disabled={busy || !cat} onChange={setTipo} options={opts(cat?.arma_tipos, a.tipo)} />
            </OpsField>
            <OpsField label="MARCA" span={2}>
              <Select searchable value={marca} placeholder="MARCA" disabled={busy || !cat} onChange={setMarca} options={opts(cat?.arma_marcas, a.marca)} />
            </OpsField>
            <OpsField label="CALIBRE">
              <Select value={calibre} placeholder="CALIBRE" disabled={busy || !cat} onChange={setCalibre} options={opts(cat?.arma_calibres, a.calibre)} />
            </OpsField>
          </OpsFields>
          <OpsActions>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              disabled={busy || !tipo || (!dirty && a.manual)}
              onClick={() =>
                onSave({
                  kind: "armas",
                  tipo_id: Number(tipo),
                  marca_id: marca ? Number(marca) : undefined,
                  calibre_id: calibre ? Number(calibre) : undefined,
                })
              }
            >
              <Check size={12} strokeWidth={2} /> GRAVAR
            </button>
            {a.manual && <AutoButton busy={busy} onClick={() => onSave({ kind: "armas", reset: true })} />}
          </OpsActions>
        </>
      ) : (
        <MaterialRead
          fields={[
            ["TIPO", a.tipo?.nome],
            ["MARCA", a.marca?.nome],
            ["CALIBRE", a.calibre?.nome],
          ]}
        />
      )}
    </div>
  );
}

function DrogaRow({ d, raw, cat, editable, busy, onSave }: RowProps<{ d: SipomDroga; raw?: OpsOccurrence["drugs"][number] }>) {
  const [droga, setDroga] = useState(d.droga ? String(d.droga.id) : "");
  const [qtd, setQtd] = useState(d.quantidade != null ? String(d.quantidade) : "");
  useEffect(() => {
    setDroga(d.droga ? String(d.droga.id) : "");
    setQtd(d.quantidade != null ? String(d.quantidade) : "");
  }, [d.droga, d.quantidade]);
  const unidade = cat?.drogas.find((x) => String(x.id) === droga)?.unidade ?? d.unidade;
  const rawText = raw
    ? [raw.description, raw.grams != null ? formatGrams(raw.grams) : "", raw.packages != null ? `${raw.packages} pacote(s)` : ""]
        .filter(Boolean)
        .join(" · ")
    : "";
  const q = Number(qtd.replace(",", "."));
  return (
    <div className="sipom-material">
      <MaterialHead raw={rawText} ok={d.ok} manual={d.manual} />
      {editable ? (
        <>
          <OpsFields edit>
            <OpsField label="DROGA" span={2} missing={!droga}>
              <Select
                value={droga}
                placeholder="DROGA"
                disabled={busy || !cat}
                onChange={setDroga}
                options={
                  cat
                    ? cat.drogas.map((x) => ({ value: String(x.id), label: `${x.nome} — ${x.unidade}` }))
                    : opts(undefined, d.droga)
                }
              />
            </OpsField>
            <OpsField label={unidade ? `QUANTIDADE EM ${unidade}` : "QUANTIDADE"} span={2} missing={!(q > 0)}>
              <input
                className="ops-input"
                type="text"
                inputMode="decimal"
                value={qtd}
                onChange={(e) => setQtd(e.target.value)}
                placeholder="0"
                disabled={busy}
              />
            </OpsField>
          </OpsFields>
          <OpsActions>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              disabled={busy || !droga || !(q > 0)}
              onClick={() => onSave({ kind: "drogas", droga_id: Number(droga), quantidade: q })}
            >
              <Check size={12} strokeWidth={2} /> GRAVAR
            </button>
            {d.manual && <AutoButton busy={busy} onClick={() => onSave({ kind: "drogas", reset: true })} />}
          </OpsActions>
        </>
      ) : (
        <MaterialRead
          fields={[
            ["DROGA", d.droga?.nome],
            ["QUANTIDADE", d.quantidade != null ? `${d.quantidade} ${d.unidade}` : undefined],
          ]}
        />
      )}
    </div>
  );
}

function VeiculoRow({ v, raw, cat, editable, busy, onSave }: RowProps<{ v: SipomVeiculo; raw?: OpsOccurrence["vehicles"][number] }>) {
  const [tipo, setTipo] = useState(v.tipo ? String(v.tipo.id) : "");
  const [cor, setCor] = useState(v.cor ? String(v.cor.id) : "");
  const [mm, setMm] = useState<SipomRef | null>(v.marca_modelo);
  const [situacao, setSituacao] = useState<1 | 2>(v.situacao === 2 ? 2 : 1);
  const [q, setQ] = useState("");
  const [found, setFound] = useState<SipomRef[]>([]);
  useEffect(() => {
    setTipo(v.tipo ? String(v.tipo.id) : "");
    setCor(v.cor ? String(v.cor.id) : "");
    setMm(v.marca_modelo);
    setSituacao(v.situacao === 2 ? 2 : 1);
  }, [v.tipo, v.cor, v.marca_modelo, v.situacao]);
  // Busca de marca/modelo conforme se digita (a tabela tem 40 mil linhas).
  useEffect(() => {
    const term = q.trim();
    if (term.length < 2) {
      setFound([]);
      return;
    }
    const h = window.setTimeout(() => {
      searchSipomMarcasModelos(term)
        .then((r) => setFound(r.items))
        .catch(() => setFound([]));
    }, 300);
    return () => window.clearTimeout(h);
  }, [q]);
  const rawText = raw ? [raw.kind, raw.brand, raw.model, raw.color, raw.plate].filter(Boolean).join(" · ") : "";
  const choices = found.length > 0 ? found : v.marca_modelo_candidates;
  // Tipo, cor e marca/modelo só são obrigatórios sem placa (vai como
  // MOTONETA) ou depois que o SIPOM recusou a placa.
  const semPlaca = !raw?.plate?.trim();
  // Sem placa, só ciclomotor/motoneta (tipos DENATRAN 2 e 3) vai ao SIPOM.
  const ciclomotor = !!v.tipo && [2, 3].includes(v.tipo.id);
  const required = v.cadastro_exigido || semPlaca;
  return (
    <div className="sipom-material">
      <MaterialHead raw={rawText} ok={v.ok} manual={v.manual} />
      {semPlaca && !v.cadastro_exigido && (ciclomotor ? (
        <div className="banner banner-info">SEM PLACA: VAI AO SIPOM COMO MOTONETA — INFORME COR E MARCA/MODELO</div>
      ) : (
        <div className="banner banner-error">⚠ SEM PLACA: SÓ CICLOMOTOR VAI AO SIPOM SEM PLACA — INFORME A PLACA DO VEÍCULO</div>
      ))}
      {v.cadastro_exigido && (
        <div className="banner banner-info">O SIPOM NÃO TEM ESTA PLACA: INFORME TIPO, COR, MARCA/MODELO E ANO DE FABRICAÇÃO E REENVIE</div>
      )}
      {editable ? (
        <>
          <OpsFields edit>
            <OpsField label="TIPO" missing={required && !tipo}>
              <Select value={tipo} placeholder="TIPO" disabled={busy || !cat} onChange={setTipo} options={opts(cat?.veiculo_tipos, v.tipo)} />
            </OpsField>
            <OpsField label="COR" missing={required && !cor}>
              <Select value={cor} placeholder="COR" disabled={busy || !cat} onChange={setCor} options={opts(cat?.veiculo_cores, v.cor)} />
            </OpsField>
            <OpsField label="SITUAÇÃO" span={2}>
              <div className="seg-row ops-seg" role="radiogroup" aria-label="Situação do veículo">
                {([1, 2] as const).map((x) => (
                  <button
                    key={x}
                    type="button"
                    role="radio"
                    aria-checked={situacao === x}
                    className={"seg-btn" + (situacao === x ? " seg-btn--on" : "")}
                    disabled={busy}
                    onClick={() => setSituacao(x)}
                  >
                    {x === 1 ? "APREENDIDO" : "RECUPERADO"}
                  </button>
                ))}
              </div>
            </OpsField>
            <OpsField label="MARCA / MODELO (TABELA DENATRAN)" span={2} missing={required && !mm} value={mm?.nome ?? "NÃO ESCOLHIDO"} />
            <OpsField label="BUSCAR MARCA / MODELO" span={2}>
              <div className="toolbar-search ops-search">
                <Search size={13} strokeWidth={1.6} />
                <input type="text" value={q} onChange={(e) => setQ(e.target.value)} placeholder="DIGITE PARA BUSCAR…" disabled={busy} />
              </div>
            </OpsField>
          </OpsFields>
          {choices.length > 0 && (
            <div className="ops-chips">
              {choices.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  className={"btn btn-sm" + (mm?.id === c.id ? " btn-active" : "")}
                  disabled={busy}
                  onClick={() => setMm(c)}
                >
                  {c.nome}
                </button>
              ))}
            </div>
          )}
          <OpsActions>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              disabled={busy || !tipo || !cor || !mm}
              onClick={() =>
                mm && onSave({ kind: "veiculos", tipo_codigo: Number(tipo), cor_codigo: Number(cor), marca_modelo_codigo: mm.id, situacao })
              }
            >
              <Check size={12} strokeWidth={2} /> GRAVAR
            </button>
            {v.manual && <AutoButton busy={busy} onClick={() => onSave({ kind: "veiculos", reset: true })} />}
          </OpsActions>
        </>
      ) : (
        <MaterialRead
          fields={[
            ["TIPO", v.tipo?.nome],
            ["MARCA / MODELO", v.marca_modelo?.nome],
            ["COR", v.cor?.nome],
            ["SITUAÇÃO", v.situacao === 2 ? "RECUPERADO" : v.situacao === 1 ? "APREENDIDO" : undefined],
          ]}
        />
      )}
    </div>
  );
}

// Placa do veículo: o relatório às vezes a omite ("ident. suprimida") ou a
// escreve errado. Sem ela, só o ciclomotor vai ao SIPOM.
function VeiculoPlaca({
  placa,
  editable,
  busy,
  onSave,
}: {
  placa: string;
  editable: boolean;
  busy: boolean;
  onSave: (placa: string) => void;
}) {
  const norm = (x: string) => x.toUpperCase().replace(/[^A-Z0-9]/g, "");
  const [val, setVal] = useState(norm(placa));
  useEffect(() => setVal(norm(placa)), [placa]);
  const valid = val === "" || /^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$/.test(val);
  if (!editable) {
    return <MaterialRead fields={[["PLACA", placa || undefined]]} />;
  }
  return (
    <OpsFields edit>
      <OpsField label="PLACA" missing={!val}>
        <input
          className="ops-input mono"
          maxLength={8}
          value={val}
          placeholder="ABC1D23"
          disabled={busy}
          onChange={(e) => setVal(norm(e.target.value))}
        />
      </OpsField>
      <OpsActions>
        <button
          type="button"
          className="btn btn-primary btn-sm"
          disabled={busy || val === norm(placa) || !valid}
          onClick={() => onSave(val)}
        >
          <Check size={12} strokeWidth={2} /> GRAVAR PLACA
        </button>
      </OpsActions>
    </OpsFields>
  );
}

// Ano de fabricação e do modelo do veículo: o relatório não traz, e o SIPOM
// exige o de fabricação para placa que ainda não está cadastrada lá — só
// falta de verdade depois que ele recusou a placa (cadastro_exigido).
function VeiculoAnos({
  v,
  editable,
  busy,
  onSave,
}: {
  v: SipomVeiculo;
  editable: boolean;
  busy: boolean;
  onSave: (fabricacao: number | null, modelo: number | null) => void;
}) {
  const [fab, setFab] = useState(v.ano_fabricacao ? String(v.ano_fabricacao) : "");
  const [mod, setMod] = useState(v.ano_modelo ? String(v.ano_modelo) : "");
  useEffect(() => {
    setFab(v.ano_fabricacao ? String(v.ano_fabricacao) : "");
    setMod(v.ano_modelo ? String(v.ano_modelo) : "");
  }, [v.ano_fabricacao, v.ano_modelo]);
  const year = (x: string) => (/^\d{4}$/.test(x) ? Number(x) : null);
  const dirty = fab !== (v.ano_fabricacao ? String(v.ano_fabricacao) : "") || mod !== (v.ano_modelo ? String(v.ano_modelo) : "");
  if (!editable) {
    return (
      <MaterialRead
        fields={[
          ["ANO DE FABRICAÇÃO", v.ano_fabricacao ? String(v.ano_fabricacao) : undefined],
          ["ANO DO MODELO", v.ano_modelo ? String(v.ano_modelo) : undefined],
        ]}
      />
    );
  }
  return (
    <OpsFields edit>
      <OpsField label="ANO DE FABRICAÇÃO" missing={v.cadastro_exigido && !v.ano_fabricacao}>
        <input
          className="ops-input mono"
          inputMode="numeric"
          maxLength={4}
          value={fab}
          placeholder="AAAA"
          disabled={busy}
          onChange={(e) => setFab(e.target.value.replace(/\D/g, ""))}
        />
      </OpsField>
      <OpsField label="ANO DO MODELO">
        <input
          className="ops-input mono"
          inputMode="numeric"
          maxLength={4}
          value={mod}
          placeholder="AAAA"
          disabled={busy}
          onChange={(e) => setMod(e.target.value.replace(/\D/g, ""))}
        />
      </OpsField>
      <OpsActions>
        <button
          type="button"
          className="btn btn-primary btn-sm"
          disabled={busy || !dirty || year(fab) === null || (mod !== "" && year(mod) === null)}
          onClick={() => onSave(year(fab), year(mod))}
        >
          <Check size={12} strokeWidth={2} /> GRAVAR ANOS
        </button>
      </OpsActions>
    </OpsFields>
  );
}

// `current`: o valor já gravado entra na lista mesmo antes de o catálogo
// carregar (ou se ele falhar) — o campo nunca aparece vazio tendo valor.
function opts(list: SipomRef[] | undefined, current?: SipomRef | null) {
  const out = (list ?? []).map((x) => ({ value: String(x.id), label: x.nome }));
  if (current && !out.some((o) => o.value === String(current.id))) {
    out.unshift({ value: String(current.id), label: current.nome });
  }
  return out;
}
