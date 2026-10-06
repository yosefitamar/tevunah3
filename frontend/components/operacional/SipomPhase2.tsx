"use client";

import { useEffect, useState } from "react";
import { Check, RotateCcw, Search } from "lucide-react";
import type { OpsOccurrence, SipomArma, SipomDroga, SipomRef, SipomVeiculo } from "@/lib/ops-reports-api";
import { formatGrams } from "@/lib/ops-reports-api";
import {
  getSipomCatalogo,
  searchSipomMarcasModelos,
  setOpsSipomMaterial,
  setOpsSipomProcedimento,
  type SipomCatalogo,
  type SipomMaterialInput,
  type SipomProcedimentoInput,
} from "@/lib/sipom-api";
import Select from "../shared/Select";

type Result = Awaited<ReturnType<typeof setOpsSipomProcedimento>>;

type Props = {
  occ: OpsOccurrence;
  editable: boolean;
  busy: boolean;
  /** Executa a gravação pelo controle de busy/erro da seção. */
  run: (fn: () => Promise<Result>) => Promise<boolean>;
};

/**
 * Fase 2 do envio: o procedimento (tipo, delegacia, delegado) e os materiais
 * (armas, drogas, veículos) nos códigos do SIPOM. O que o relatório trouxe em
 * texto fica ao lado da escolha; o que o analista fixa vira termo aprendido e
 * vale para as próximas ocorrências com o mesmo texto.
 */
export default function SipomPhase2({ occ, editable, busy, run }: Props) {
  const s = occ.sipom!;
  const [cat, setCat] = useState<SipomCatalogo | null>(null);
  const [catError, setCatError] = useState<string | null>(null);

  useEffect(() => {
    if (!editable) return;
    getSipomCatalogo()
      .then(setCat)
      .catch(() => setCatError("Não foi possível carregar as listas do SIPOM"));
  }, [editable]);

  const pending = new Set(s.pendencias.map((p) => p.code));
  const manual = new Set(s.manual);
  const p = s.procedimento;
  const setProc = (input: SipomProcedimentoInput) => occ.id && run(() => setOpsSipomProcedimento(occ.id!, input));
  const setMat = (index: number, input: SipomMaterialInput) =>
    occ.id && run(() => setOpsSipomMaterial(occ.id!, index, input));
  const procManual = manual.has("procedimento") || manual.has("delegacia") || manual.has("delegado");
  const opts = (list: SipomRef[] | undefined) => (list ?? []).map((x) => ({ value: String(x.id), label: x.nome }));

  return (
    <>
      <div className="sipom-sub">PROCEDIMENTO</div>
      {catError && <div className="banner banner-error">⚠ {catError}</div>}
      <dl className="ops-kv sipom-kv">
        <Field
          label="TIPO"
          value={p.procedimento?.nome ?? ""}
          hint={p.tipo_texto}
          missing={pending.has("procedimento") || pending.has("procedimento_tipo")}
          tag={manual.has("procedimento") ? "ANALISTA" : undefined}
        >
          {editable && (
            <Select
              value={p.procedimento ? String(p.procedimento.id) : ""}
              placeholder="ESCOLHA O TIPO"
              disabled={busy || !cat}
              onChange={(v) => setProc({ procedimento_id: Number(v) })}
              options={opts(cat?.procedimentos)}
            />
          )}
        </Field>
        <Field
          label="NÚMERO / ANO"
          value={p.numero ? `${p.numero} / ${p.ano}` : ""}
          hint={p.numero_texto}
          missing={pending.has("procedimento_numero")}
        />
        <Field
          label="DELEGACIA"
          value={p.delegacia?.nome ?? ""}
          hint={p.delegacia_texto}
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
              options={opts(cat?.delegacias)}
            />
          )}
        </Field>
        <Field
          label="DELEGADO (OPCIONAL)"
          value={p.delegado?.nome ?? ""}
          hint={p.delegado_texto}
          missing={pending.has("delegado")}
          tag={manual.has("delegado") ? "ANALISTA" : undefined}
        >
          {editable && (
            <div className="sipom-inline" style={{ flexDirection: "column", alignItems: "stretch", gap: 6 }}>
              {p.delegado_candidates.length > 0 && !p.delegado && (
                <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
                  {p.delegado_candidates.map((c) => (
                    <button
                      key={c.id}
                      type="button"
                      className="btn btn-sm"
                      disabled={busy}
                      onClick={() => setProc({ delegado_id: c.id })}
                      title="Casa com o nome abreviado do relatório"
                    >
                      <Check size={12} strokeWidth={2} /> {c.nome}
                    </button>
                  ))}
                </div>
              )}
              <Select
                searchable
                value={p.delegado ? String(p.delegado.id) : ""}
                placeholder="BUSCAR DELEGADO NO CATÁLOGO"
                disabled={busy || !cat}
                onChange={(v) => setProc({ delegado_id: Number(v) })}
                options={opts(cat?.delegados)}
              />
            </div>
          )}
        </Field>
      </dl>
      {editable && procManual && (
        <div style={{ marginTop: 4 }}>
          <AutoButton busy={busy} onClick={() => setProc({ reset: true })} label="PROCEDIMENTO AUTOMÁTICO" />
        </div>
      )}

      {(s.materiais.armas.length > 0 || s.materiais.drogas.length > 0 || s.materiais.veiculos.length > 0) && (
        <>
          <div className="sipom-sub">MATERIAIS</div>
          {s.materiais.armas.map((a) => (
            <ArmaRow
              key={"a" + a.index}
              a={a}
              raw={occ.weapons[a.index]}
              cat={cat}
              editable={editable}
              busy={busy}
              onSave={(input) => setMat(a.index, input)}
            />
          ))}
          {s.materiais.drogas.map((d) => (
            <DrogaRow
              key={"d" + d.index}
              d={d}
              raw={occ.drugs[d.index]}
              cat={cat}
              editable={editable}
              busy={busy}
              onSave={(input) => setMat(d.index, input)}
            />
          ))}
          {s.materiais.veiculos.map((v) => (
            <VeiculoRow
              key={"v" + v.index}
              v={v}
              raw={occ.vehicles[v.index]}
              cat={cat}
              editable={editable}
              busy={busy}
              onSave={(input) => setMat(v.index, input)}
            />
          ))}
        </>
      )}
    </>
  );
}

// ─── Linhas de material ───

type RowProps<T> = {
  cat: SipomCatalogo | null;
  editable: boolean;
  busy: boolean;
  onSave: (input: SipomMaterialInput) => Promise<boolean> | false | "" | undefined;
} & T;

function status(ok: boolean, manual: boolean) {
  return (
    <span className={"pill " + (ok ? "active" : "hold")} title={manual ? "Definido pelo analista" : undefined}>
      {ok ? "OK" : "PENDENTE"}
      {manual ? " · ANALISTA" : ""}
    </span>
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
  const dirty = tipo !== (a.tipo ? String(a.tipo.id) : "") || marca !== (a.marca ? String(a.marca.id) : "") || calibre !== (a.calibre ? String(a.calibre.id) : "");
  return (
    <div className="sipom-material">
      <div className="sipom-material-hd">
        <span className="sipom-material-kind">ARMA</span>
        <span className="muted">{rawText || "—"}</span>
        {status(a.ok, a.manual)}
      </div>
      {editable ? (
        <div className="sipom-material-edit">
          <Select value={tipo} placeholder="TIPO" disabled={busy || !cat} onChange={setTipo} options={opts(cat?.arma_tipos)} />
          <Select searchable value={marca} placeholder="MARCA" disabled={busy || !cat} onChange={setMarca} options={opts(cat?.arma_marcas)} />
          <Select value={calibre} placeholder="CALIBRE" disabled={busy || !cat} onChange={setCalibre} options={opts(cat?.arma_calibres)} />
          <button
            type="button"
            className="btn btn-primary btn-sm"
            disabled={busy || !tipo || !marca || !calibre || (!dirty && a.manual)}
            onClick={() => onSave({ kind: "armas", tipo_id: Number(tipo), marca_id: Number(marca), calibre_id: Number(calibre) })}
          >
            <Check size={12} strokeWidth={2} /> GRAVAR
          </button>
          {a.manual && <AutoButton busy={busy} onClick={() => onSave({ kind: "armas", reset: true })} />}
        </div>
      ) : (
        <div className="muted">{[a.tipo?.nome, a.marca?.nome, a.calibre?.nome].filter(Boolean).join(" · ") || "SEM TRADUÇÃO"}</div>
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
  const rawText = raw ? [raw.description, raw.grams != null ? formatGrams(raw.grams) : "", raw.packages != null ? `${raw.packages} pacote(s)` : ""].filter(Boolean).join(" · ") : "";
  const q = Number(qtd.replace(",", "."));
  return (
    <div className="sipom-material">
      <div className="sipom-material-hd">
        <span className="sipom-material-kind">DROGA</span>
        <span className="muted">{rawText || "—"}</span>
        {status(d.ok, d.manual)}
      </div>
      {editable ? (
        <div className="sipom-material-edit">
          <Select
            value={droga}
            placeholder="DROGA"
            disabled={busy || !cat}
            onChange={setDroga}
            options={(cat?.drogas ?? []).map((x) => ({ value: String(x.id), label: `${x.nome} — ${x.unidade}` }))}
          />
          <input
            type="text"
            inputMode="decimal"
            value={qtd}
            onChange={(e) => setQtd(e.target.value)}
            placeholder={unidade ? `QUANTIDADE EM ${unidade.toUpperCase()}` : "QUANTIDADE"}
            disabled={busy}
            style={{ width: 200 }}
          />
          <button
            type="button"
            className="btn btn-primary btn-sm"
            disabled={busy || !droga || !(q > 0)}
            onClick={() => onSave({ kind: "drogas", droga_id: Number(droga), quantidade: q })}
          >
            <Check size={12} strokeWidth={2} /> GRAVAR
          </button>
          {d.manual && <AutoButton busy={busy} onClick={() => onSave({ kind: "drogas", reset: true })} />}
        </div>
      ) : (
        <div className="muted">{d.droga ? `${d.droga.nome} · ${d.quantidade ?? "?"} ${d.unidade}` : "SEM TRADUÇÃO"}</div>
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
  return (
    <div className="sipom-material">
      <div className="sipom-material-hd">
        <span className="sipom-material-kind">VEÍCULO</span>
        <span className="muted">{rawText || "—"}</span>
        {status(v.ok, v.manual)}
      </div>
      {editable ? (
        <>
          <div className="sipom-material-edit">
            <Select value={tipo} placeholder="TIPO" disabled={busy || !cat} onChange={setTipo} options={opts(cat?.veiculo_tipos)} />
            <Select value={cor} placeholder="COR" disabled={busy || !cat} onChange={setCor} options={opts(cat?.veiculo_cores)} />
            <div className="seg-row" role="radiogroup" aria-label="Situação do veículo">
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
          </div>
          <div className="sipom-material-edit">
            <span className={"pill " + (mm ? "active" : "hold")} title="Marca/modelo na tabela DENATRAN">
              {mm ? mm.nome : "SEM MARCA/MODELO"}
            </span>
            <div className="toolbar-search" style={{ maxWidth: 320 }}>
              <Search size={13} strokeWidth={1.6} />
              <input type="text" value={q} onChange={(e) => setQ(e.target.value)} placeholder="buscar marca/modelo…" disabled={busy} />
            </div>
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
          </div>
          {choices.length > 0 && (
            <div style={{ display: "flex", flexWrap: "wrap", gap: 4, marginTop: 6 }}>
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
        </>
      ) : (
        <div className="muted">
          {[v.tipo?.nome, v.marca_modelo?.nome, v.cor?.nome, v.situacao === 2 ? "RECUPERADO" : v.situacao === 1 ? "APREENDIDO" : ""]
            .filter(Boolean)
            .join(" · ") || "SEM TRADUÇÃO"}
        </div>
      )}
    </div>
  );
}

// ─── Peças ───

function opts(list: SipomRef[] | undefined) {
  return (list ?? []).map((x) => ({ value: String(x.id), label: x.nome }));
}

function Field({
  label,
  value,
  hint,
  missing,
  tag,
  children,
}: {
  label: string;
  value: string;
  /** Como o relatório escreveu. */
  hint?: string;
  missing?: boolean;
  tag?: string;
  children?: React.ReactNode;
}) {
  return (
    <div className={missing ? "sipom-missing" : undefined}>
      <dt>
        {label}
        {tag && <span className="sipom-tag">{tag}</span>}
      </dt>
      <dd>
        {value || "—"}
        {hint && (
          <span className="muted" style={{ display: "block", fontSize: 11.5 }}>
            RELATÓRIO: {hint}
          </span>
        )}
      </dd>
      {children}
    </div>
  );
}

function AutoButton({ busy, onClick, label = "AUTOMÁTICO" }: { busy: boolean; onClick: () => void; label?: string }) {
  return (
    <button type="button" className="btn btn-ghost btn-sm" disabled={busy} onClick={onClick} title="Voltar ao cálculo automático">
      <RotateCcw size={12} strokeWidth={2} /> {label}
    </button>
  );
}
