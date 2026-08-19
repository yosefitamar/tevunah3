"use client";

import { useState, type FormEvent } from "react";
import { createPortal } from "react-dom";
import { ClipboardPaste, Plus, Trash2, UserPlus, X } from "lucide-react";
import {
  INCIDENT_TYPE_LABEL,
  INCIDENT_TYPES,
  INVOLVED_ROLES,
  createIncident,
  isVictimRole,
  type IncidentMeans,
  type IncidentType,
  type ParsedPerson,
  type ParsedPersonMatch,
  type ParsedReport,
} from "@/lib/incidents-api";
import { getEntity } from "@/lib/entities-api";
import type { PersonAttrs } from "@/lib/entities-types";
import type { ApiError } from "@/lib/api";
import { formatBRDate } from "@/lib/format";
import { useIncidentLocations } from "@/lib/useIncidentLocations";
import CreateEntidadeModal from "../entidades/CreateEntidadeModal";
import DateInput from "../shared/DateInput";
import Select from "../shared/Select";
import GeoField from "./GeoField";
import ImportReportModal from "./ImportReportModal";
import InvolvedPicker from "./InvolvedPicker";
import MeansField from "./MeansField";
import PlaceField from "./PlaceField";
import ConfirmVitimaModal, { type VictimCandidate } from "./ConfirmVitimaModal";

type Props = {
  onClose: () => void;
  onCreated: (id: string) => void;
};

type PendingInvolved = { entity_id: string; name: string; kind: string; role: string };

// Rótulos dos critérios que o backend usa para pontuar homônimos.
const MATCH_FIELD_LABEL: Record<string, string> = {
  name: "NOME",
  mother_name: "MÃE",
  date_of_birth: "NASCIMENTO",
};

export default function CreateOcorrenciaModal({ onClose, onCreated }: Props) {
  const today = new Date().toISOString().slice(0, 10);
  const [type, setType] = useState<IncidentType>("homicidio");
  const [occurredOn, setOccurredOn] = useState(today);
  const [occurredTime, setOccurredTime] = useState("");
  const [ciops, setCiops] = useState("");
  const [lat, setLat] = useState("");
  const [lng, setLng] = useState("");
  const [city, setCity] = useState("");
  const [neighborhood, setNeighborhood] = useState("");
  const [description, setDescription] = useState("");
  const [means, setMeans] = useState<IncidentMeans>("");
  const [meansDetail, setMeansDetail] = useState("");
  const [involved, setInvolved] = useState<PendingInvolved[]>([]);
  const [pendingVictim, setPendingVictim] = useState<{
    candidate: VictimCandidate;
    role: string;
    citedIdx?: number;
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const { neighborhoodsOf } = useIncidentLocations();

  // ── Importação de relatório ──
  const [importOpen, setImportOpen] = useState(false);
  // Pessoas citadas no texto que ainda não viraram vínculo. Ficam à parte do
  // formulário: o relatório cita, o analista decide se é a mesma pessoa.
  const [cited, setCited] = useState<ParsedPerson[]>([]);
  const [importWarnings, setImportWarnings] = useState<string[]>([]);
  const [creatingPerson, setCreatingPerson] = useState<ParsedPerson | null>(null);

  function commitInvolved(e: { id: string; name: string; kind: string }, role: string) {
    setInvolved((cur) =>
      cur.some((x) => x.entity_id === e.id)
        ? cur
        : [...cur, { entity_id: e.id, name: e.name, kind: e.kind, role }],
    );
  }

  function addInvolved(e: VictimCandidate, role: string, citedIdx?: number) {
    // Mesma confirmação do dossiê: aqui a marcação de óbito acontece no
    // POST, mas a decisão (é mesmo esta pessoa?) é a mesma.
    if (type === "homicidio" && isVictimRole(role) && e.kind === "person") {
      setPendingVictim({ candidate: e, role, citedIdx });
      return;
    }
    commitInvolved(e, role);
    if (citedIdx !== undefined) dropCited(citedIdx);
  }

  function dropCited(idx: number) {
    setCited((cur) => cur.filter((_, i) => i !== idx));
  }

  /**
   * Define o papel de um citado que o relatório não deixou claro — vítima
   * que sobreviveu, ou tentativa de homicídio. O parser recusa deduzir
   * porque VÍTIMA marca óbito no acervo inteiro.
   */
  function setCitedRole(idx: number, role: string) {
    setCited((cur) => cur.map((p, i) => (i === idx ? { ...p, role } : p)));
  }

  /**
   * Aplica o resultado da leitura do relatório. Só sobrescreve o que veio
   * reconhecido — campo não lido preserva o que o analista já tinha digitado.
   */
  function applyParsed(p: ParsedReport) {
    setImportOpen(false);
    if (p.type) setType(p.type);
    if (p.occurred_on) setOccurredOn(p.occurred_on);
    if (p.occurred_time) setOccurredTime(p.occurred_time);
    if (p.ciops_record) setCiops(p.ciops_record);
    if (p.means) setMeans(p.means);
    if (p.means_detail) setMeansDetail(p.means_detail);
    if (p.city) setCity(p.city);
    if (p.neighborhood) setNeighborhood(p.neighborhood);
    if (p.description) setDescription(p.description);
    if (p.latitude !== undefined && p.longitude !== undefined) {
      setLat(String(p.latitude));
      setLng(String(p.longitude));
    }
    setCited(p.people ?? []);
    setImportWarnings(p.warnings ?? []);
    setErr(null);
  }

  /** Vincula o citado a um dossiê já existente escolhido pelo analista. */
  async function linkCited(person: ParsedPerson, match: ParsedPersonMatch, idx: number) {
    try {
      // O match traz só o suficiente para comparar na lista; a confirmação de
      // vítima precisa da ficha inteira (CPF, foto, versão).
      const r = await getEntity(match.id);
      addInvolved(
        {
          id: r.entity.id,
          name: r.entity.name,
          kind: r.entity.kind,
          version: r.entity.version,
          attrs: (r.entity.attrs ?? undefined) as PersonAttrs | undefined,
        },
        person.role,
        idx,
      );
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao carregar o dossiê");
    }
  }

  /** Vincula a pessoa recém-cadastrada a partir do que o relatório trouxe. */
  async function linkNewPerson(id: string, person: ParsedPerson) {
    setCreatingPerson(null);
    try {
      const r = await getEntity(id);
      const idx = cited.findIndex((c) => c === person);
      addInvolved(
        {
          id: r.entity.id,
          name: r.entity.name,
          kind: r.entity.kind,
          version: r.entity.version,
          attrs: (r.entity.attrs ?? undefined) as PersonAttrs | undefined,
        },
        person.role,
        idx >= 0 ? idx : undefined,
      );
    } catch (e) {
      setErr((e as ApiError).message || "Pessoa cadastrada, mas o vínculo falhou");
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (occurredTime.trim() && !/^\d{1,2}:\d{2}$/.test(occurredTime.trim())) {
      setErr("Hora inválida — use HH:MM");
      return;
    }
    setBusy(true);
    try {
      const r = await createIncident({
        type,
        occurred_on: occurredOn,
        occurred_time: occurredTime.trim() || undefined,
        ciops_record: ciops.trim(),
        latitude: lat.trim() ? Number(lat) : undefined,
        longitude: lng.trim() ? Number(lng) : undefined,
        city,
        neighborhood: neighborhood.trim(),
        description: description.trim(),
        // Meio utilizado é campo de CVLI — não vai junto nos demais tipos.
        means: type === "homicidio" ? means : "",
        means_detail: type === "homicidio" && means === "outros" ? meansDetail.trim() : "",
        involved: involved.map((i) => ({ entity_id: i.entity_id, role: i.role })),
      });
      onCreated(r.incident.id);
    } catch (e) {
      setErr((e as ApiError).message || "Falha ao registrar ocorrência");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 600 }}>
        <div className="modal-hd">
          <span>NOVA OCORRÊNCIA</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <form className="modal-form" onSubmit={onSubmit}>
          <div className="modal-bd">
            <button
              type="button"
              className="btn btn-ghost"
              style={{ width: "100%", justifyContent: "center" }}
              onClick={() => setImportOpen(true)}
            >
              <ClipboardPaste size={13} />
              IMPORTAR RELATÓRIO DO GRUPO
            </button>

            {importWarnings.length > 0 && (
              <div className="banner banner-warn" style={{ marginTop: 8 }}>
                <span>
                  {importWarnings.map((wtext, i) => (
                    <span key={i} style={{ display: "block" }}>
                      {wtext}
                    </span>
                  ))}
                </span>
              </div>
            )}

            <div className="form-grid-2">
              <div className="form-field">
                <span>TIPO</span>
                <Select
                  value={type}
                  onChange={(v) => setType(v as IncidentType)}
                  options={INCIDENT_TYPES.map((t) => ({ value: t, label: INCIDENT_TYPE_LABEL[t] }))}
                />
              </div>
              <div className="form-field">
                <span>FICHA CIOPS</span>
                <input
                  type="text"
                  value={ciops}
                  onChange={(e) => setCiops(e.target.value)}
                  maxLength={60}
                  placeholder="nº / referência"
                />
              </div>
            </div>

            <div className="form-grid-2">
              <div className="form-field">
                <span>DATA</span>
                <DateInput value={occurredOn} onChange={setOccurredOn} max={today} />
              </div>
              <label className="form-field">
                <span>HORA (OPCIONAL)</span>
                <input
                  type="text"
                  value={occurredTime}
                  onChange={(e) => setOccurredTime(e.target.value)}
                  placeholder="HH:MM"
                  maxLength={5}
                  inputMode="numeric"
                />
              </label>
            </div>

            {type === "homicidio" && (
              <MeansField
                means={means}
                detail={meansDetail}
                onChangeMeans={setMeans}
                onChangeDetail={setMeansDetail}
              />
            )}

            <PlaceField
              city={city}
              neighborhood={neighborhood}
              onChangeCity={(v) => {
                setCity(v);
                // Bairro pertence ao município: trocar de cidade invalida o
                // que estava digitado.
                setNeighborhood("");
              }}
              onChangeNeighborhood={setNeighborhood}
              neighborhoodOptions={neighborhoodsOf(city)}
            />

            <GeoField lat={lat} lng={lng} onChange={(la, lo) => { setLat(la); setLng(lo); }} />

            <label className="form-field">
              <span>DESCRIÇÃO</span>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={5}
                placeholder="relato da ocorrência…"
              />
            </label>

            {cited.length > 0 && (
              <div className="form-field">
                <span>CITADOS NO RELATÓRIO</span>
                <div className="qual-picker-panel">
                  {cited.map((p, idx) => {
                    const qualif = [
                      p.mother_name && `MÃE: ${p.mother_name}`,
                      p.date_of_birth && `DN: ${formatBRDate(p.date_of_birth)}`,
                      p.cpf && `CPF: ${p.cpf}`,
                      p.alias && `VULGO: ${p.alias}`,
                    ]
                      .filter(Boolean)
                      .join(" · ");
                    return (
                      <div key={`${p.name}-${idx}`} className="cited-person">
                        <div className="cited-person-hd">
                          <span className="cited-person-name">{p.name}</span>
                          {p.role ? (
                            <span className="pill">{p.role}</span>
                          ) : (
                            <span className="pill hold">PAPEL A DEFINIR</span>
                          )}
                          <button
                            type="button"
                            className="action-btn"
                            aria-label="Descartar citado"
                            title="Descartar"
                            onClick={() => dropCited(idx)}
                          >
                            <Trash2 size={11} />
                          </button>
                        </div>
                        <div className="muted" style={{ fontSize: 10.5 }}>
                          {qualif || "// SEM QUALIFICAÇÃO NO TEXTO"}
                        </div>

                        {!p.role && (
                          <div className="form-field">
                            <span>PAPEL NA OCORRÊNCIA</span>
                            <Select
                              value={p.role}
                              onChange={(v) => setCitedRole(idx, v)}
                              placeholder="SELECIONE…"
                              options={[
                                { value: "", label: "SELECIONE…" },
                                ...INVOLVED_ROLES.map((r) => ({ value: r, label: r })),
                              ]}
                            />
                          </div>
                        )}

                        {p.matches.length > 0 ? (
                          <div className="qual-picker-results">
                            {p.matches.map((m) => (
                              <button
                                key={m.id}
                                type="button"
                                className="qual-picker-row"
                                disabled={!p.role}
                                title={p.role ? undefined : "Defina o papel antes de vincular"}
                                onClick={() => linkCited(p, m, idx)}
                              >
                                <span className="qual-picker-row-name">{m.name}</span>
                                <span className="qual-picker-row-meta">
                                  {m.score}/3 ·{" "}
                                  {m.matched_fields
                                    .map((f) => MATCH_FIELD_LABEL[f] ?? f.toUpperCase())
                                    .join(" + ")}
                                </span>
                                <span className="qual-picker-row-add">
                                  <Plus size={12} />
                                </span>
                              </button>
                            ))}
                          </div>
                        ) : (
                          <div className="muted" style={{ fontSize: 10.5 }}>
                            // NENHUM DOSSIÊ PARECIDO NO ACERVO
                          </div>
                        )}

                        <button
                          type="button"
                          className="btn btn-ghost"
                          disabled={!p.role}
                          title={p.role ? undefined : "Defina o papel antes de cadastrar"}
                          onClick={() => setCreatingPerson(p)}
                        >
                          <UserPlus size={12} />
                          CADASTRAR E VINCULAR
                        </button>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            <div className="form-field">
              <span>ENVOLVIDOS (OPCIONAL)</span>
              {involved.length > 0 && (
                <div className="tag-row" style={{ marginBottom: 8 }}>
                  {involved.map((i) => (
                    <span key={i.entity_id} className="tag-chip" style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                      {i.role ? `${i.role}: ` : ""}
                      {i.name.toUpperCase()}
                      <button
                        type="button"
                        className="action-btn"
                        aria-label="Remover"
                        onClick={() =>
                          setInvolved((cur) => cur.filter((x) => x.entity_id !== i.entity_id))
                        }
                      >
                        <Trash2 size={11} />
                      </button>
                    </span>
                  ))}
                </div>
              )}
              <InvolvedPicker
                exclude={involved.map((i) => i.entity_id)}
                onPick={(e, role) => addInvolved(e, role)}
              />
            </div>

            {err && <div className="banner banner-error">⚠ {err}</div>}

            {importOpen && (
              <ImportReportModal onClose={() => setImportOpen(false)} onApply={applyParsed} />
            )}

            {pendingVictim && (
              <ConfirmVitimaModal
                candidate={pendingVictim.candidate}
                occurredOn={occurredOn}
                onCancel={() => setPendingVictim(null)}
                onConfirm={() => {
                  commitInvolved(pendingVictim.candidate, pendingVictim.role);
                  if (pendingVictim.citedIdx !== undefined) dropCited(pendingVictim.citedIdx);
                  setPendingVictim(null);
                }}
              />
            )}
          </div>
          <div className="modal-ft">
            <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
              CANCELAR
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy}>
              {busy ? "REGISTRANDO…" : "REGISTRAR OCORRÊNCIA"}
            </button>
          </div>
        </form>

        {/* Portal: o wizard de entidade tem <form> próprio e não pode ficar
            aninhado no <form> desta tela (falha de hidratação). */}
        {creatingPerson &&
          typeof document !== "undefined" &&
          createPortal(
            <CreateEntidadeModal
              initialKind="person"
              initialPerson={{
                name: creatingPerson.name,
                motherName: creatingPerson.mother_name,
                dateOfBirth: creatingPerson.date_of_birth,
                alias: creatingPerson.alias,
                description: creatingPerson.notes,
              }}
              onClose={() => setCreatingPerson(null)}
              onCreated={(id) => linkNewPerson(id, creatingPerson)}
            />,
            document.body,
          )}
      </div>
    </div>
  );
}
