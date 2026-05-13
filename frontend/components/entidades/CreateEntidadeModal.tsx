"use client";

import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { AlertTriangle, Building2, MapPin, User, X } from "lucide-react";
import TagInput from "../shared/TagInput";
import { useModal } from "@/contexts/ModalContext";
import PrimaryPhotoPicker from "./PrimaryPhotoPicker";
import { PendingGalleryEditor, type PendingPhoto } from "./GalleryEditor";
import {
  createEntity,
  findPersonDuplicates,
  listEntities,
  uploadEntityPhoto,
  uploadGalleryPhoto,
  type DuplicatesResult,
} from "@/lib/entities-api";
import {
  ENTITY_KIND_LABEL,
  GENDERS,
  GENDER_LABEL,
  isOrganization,
  orgPrimaryLabel,
  type Entity,
  type EntityKind,
  type Gender,
} from "@/lib/entities-types";
import type { ApiError } from "@/lib/api";

type Props = {
  onClose: () => void;
  onCreated: (id: string) => void;
};

export default function CreateEntidadeModal({ onClose, onCreated }: Props) {
  // Fluxo em duas etapas: na etapa 1 (kind === null) o modal mostra apenas o
  // seletor de tipo; ao escolher, kind passa a ter valor e o formulário
  // específico do tipo é renderizado. O botão "trocar tipo" devolve para a
  // etapa 1 preservando campos comuns já preenchidos.
  const [kind, setKind] = useState<EntityKind | null>(null);
  type Tab = "identity" | "data" | "media";
  const [activeTab, setActiveTab] = useState<Tab>("identity");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState<string[]>([]);

  // Foto primária — aplicável a Pessoa e Lugar.
  const [photoFile, setPhotoFile] = useState<File | null>(null);
  // Galeria — pendentes em memória, enviadas após createEntity.
  const [galleryPending, setGalleryPending] = useState<PendingPhoto[]>([]);

  // Person
  const [aliases, setAliases] = useState<string[]>([]);
  const [motherName, setMotherName] = useState("");
  const [gender, setGender] = useState<Gender | "">("");
  const [dob, setDob] = useState("");
  const [cpf, setCpf] = useState("");
  const [orcrimId, setOrcrimId] = useState("");

  // Organization
  const [orgSigla, setOrgSigla] = useState("");
  const [legalName, setLegalName] = useState("");
  const [taxID, setTaxID] = useState("");
  const [foundedAt, setFoundedAt] = useState("");

  // Place
  const [address, setAddress] = useState("");
  const [country, setCountry] = useState("");
  const [region, setRegion] = useState("");
  const [latitude, setLatitude] = useState("");
  const [longitude, setLongitude] = useState("");

  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const modal = useModal();
  const [dupes, setDupes] = useState<DuplicatesResult | null>(null);
  const [dupesLoading, setDupesLoading] = useState(false);

  // Debounced check de duplicates enquanto o usuário preenche o form (pessoa).
  // Só dispara com nome >= 3 chars. Reset quando muda de kind.
  useEffect(() => {
    if (kind !== "person") {
      setDupes(null);
      return;
    }
    const trimmedName = name.trim();
    const cpfDigits = cpf.replace(/\D/g, "");
    if (trimmedName.length < 3 && !cpfDigits) {
      setDupes(null);
      return;
    }
    const handle = window.setTimeout(() => {
      setDupesLoading(true);
      findPersonDuplicates({
        name: trimmedName || undefined,
        mother_name: motherName.trim() || undefined,
        date_of_birth: dob || undefined,
        cpf: cpfDigits || undefined,
      })
        .then(setDupes)
        .catch(() => {
          /* silencioso: alerta inline só aparece com dado válido */
        })
        .finally(() => setDupesLoading(false));
    }, 400);
    return () => window.clearTimeout(handle);
  }, [kind, name, motherName, dob, cpf]);

  const cpfTaken = dupes?.cpf_taken_by ?? null;
  const homonyms = dupes?.matches ?? [];
  const topScore = homonyms.reduce((m, x) => Math.max(m, x.score), 0);

  // Dispara modal de alerta uma única vez por colisão detectada. Quando o
  // usuário muda o CPF (saindo da colisão) e depois colide de novo com outro
  // registro, o modal abre novamente. O alerta inline foi removido — o modal
  // é a única forma de comunicar.
  const lastCpfAlertRef = useRef<string | null>(null);
  useEffect(() => {
    if (!cpfTaken) {
      lastCpfAlertRef.current = null;
      return;
    }
    if (lastCpfAlertRef.current === cpfTaken.id) return;
    lastCpfAlertRef.current = cpfTaken.id;
    modal.alert({
      variant: "error",
      title: "CPF JÁ CADASTRADO",
      message: (
        <>
          <div>
            Este CPF já está registrado para{" "}
            <b>{cpfTaken.name.toUpperCase()}</b>
            {cpfTaken.mother_name ? ` (mãe: ${cpfTaken.mother_name})` : ""}
            {cpfTaken.date_of_birth ? ` · nasc.: ${cpfTaken.date_of_birth}` : ""}.
          </div>
          <div style={{ marginTop: 8, fontSize: 11, color: "var(--fg-2)" }}>
            Não é possível duplicar. Edite o registro existente ou corrija o
            CPF informado.
          </div>
        </>
      ),
    });
  }, [cpfTaken, modal]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (kind === null) return; // não há form para submeter na etapa 1
    // Nas tabs IDENTIFICAÇÃO e DADOS o submit avança para a próxima tab; só na
    // MÍDIA (última) é que de fato cria a entidade. Isso casa com o botão
    // contextual do footer e com o Enter dentro dos inputs.
    if (activeTab !== "media") {
      if (activeTab === "identity" && !name.trim()) {
        setErr("Nome completo é obrigatório");
        return;
      }
      setActiveTab(activeTab === "identity" ? "data" : "media");
      return;
    }
    if (!name.trim()) {
      // Defesa em profundidade: name sempre é validado em IDENTIFICAÇÃO antes
      // de avançar, mas se algo escapar volta o usuário para a tab certa.
      setActiveTab("identity");
      setErr("Nome completo é obrigatório");
      return;
    }
    const supportsPrimaryPhoto = kind === "person" || kind === "place";
    if (supportsPrimaryPhoto && photoFile && photoFile.size > 5 * 1024 * 1024) {
      setErr("A foto excede 5 MiB");
      return;
    }
    // Bloqueio rígido: CPF tomado por outra pessoa — usuário já viu modal
    // inline; reabrimos no submit como reforço.
    if (kind === "person" && cpfTaken) {
      modal.alert({
        variant: "error",
        title: "CPF JÁ CADASTRADO",
        message: (
          <>
            Este CPF pertence a <b>{cpfTaken.name.toUpperCase()}</b>. Não é
            possível duplicar — corrija o CPF ou edite o registro existente.
          </>
        ),
      });
      return;
    }
    // Confirmação no submit quando há homônimos. Quanto maior o score, mais
    // alarmante a mensagem.
    if (kind === "person" && homonyms.length > 0) {
      const top = homonyms[0];
      const fields = top.matched_fields
        .map((f) =>
          f === "name" ? "nome" : f === "mother_name" ? "nome da mãe" : "data de nascimento",
        )
        .join(", ");
      const variant: "warning" | "error" = top.score >= 3 ? "error" : "warning";
      const ok = await modal.confirm({
        variant,
        title:
          top.score >= 3
            ? "MUITO PROVÁVEL DUPLICATA"
            : top.score === 2
              ? "PROVÁVEL DUPLICATA"
              : "HOMÔNIMO ENCONTRADO",
        message: (
          <>
            <div style={{ marginBottom: 8 }}>
              Já existe registro com critérios coincidentes ({fields}). Continuar
              com a criação assim mesmo?
            </div>
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {homonyms.slice(0, 5).map((h) => (
                <li key={h.id} style={{ fontSize: 11 }}>
                  {h.name.toUpperCase()}
                  {h.mother_name ? ` · mãe: ${h.mother_name}` : ""}
                  {h.date_of_birth ? ` · nasc.: ${h.date_of_birth}` : ""}
                </li>
              ))}
            </ul>
          </>
        ),
        confirm: "CRIAR ASSIM MESMO",
        cancel: "REVISAR",
        danger: top.score >= 3,
      });
      if (!ok) return;
    }
    setBusy(true);
    try {
      const res = await createEntity({
        kind,
        name: name.trim(),
        description: description.trim() || undefined,
        classification: 1, // default público — UI ainda não expõe a classificação
        tags,
        person:
          kind === "person"
            ? {
                aliases,
                gender: gender || undefined,
                date_of_birth: dob || undefined,
                mother_name: motherName.trim() || undefined,
                cpf: cpf.replace(/\D/g, "") || undefined,
                orcrim_id: orcrimId || undefined,
              }
            : undefined,
        organization:
          kind === "organization"
            ? {
                aliases: orgSigla.trim() ? [orgSigla.trim()] : [],
                legal_name: legalName.trim() || undefined,
                tax_id: taxID.trim() || undefined,
                founded_at: foundedAt || undefined,
              }
            : undefined,
        place:
          kind === "place"
            ? {
                address: address.trim() || undefined,
                country: country.trim() || undefined,
                region: region.trim() || undefined,
                latitude: parseFloatOrUndef(latitude),
                longitude: parseFloatOrUndef(longitude),
              }
            : undefined,
      });

      if (supportsPrimaryPhoto && photoFile) {
        try {
          await uploadEntityPhoto(res.entity.id, photoFile);
        } catch (e) {
          setErr(
            "Entidade criada, mas o upload da foto principal falhou: " +
              ((e as ApiError).message ?? "erro desconhecido"),
          );
        }
      }
      // Galeria: upload sequencial; mantemos a ordem do array em ord 0..N.
      if (galleryPending.length > 0) {
        const failures: string[] = [];
        for (let i = 0; i < galleryPending.length; i++) {
          const p = galleryPending[i];
          try {
            await uploadGalleryPhoto(res.entity.id, p.file, p.caption.trim(), i);
          } catch (e) {
            failures.push(`#${i + 1}: ${(e as ApiError).message ?? "erro"}`);
          }
        }
        if (failures.length > 0) {
          setErr(
            "Entidade criada, mas falharam fotos da galeria: " + failures.join("; "),
          );
        }
      }
      onCreated(res.entity.id);
    } catch (e) {
      setErr((e as ApiError).message || "Erro ao criar entidade");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className={"modal" + (kind === null ? "" : " modal--wide")}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-hd">
          <span>
            NOVA ENTIDADE
            {kind !== null && ` · ${ENTITY_KIND_LABEL[kind]}`}
          </span>
          <div className="modal-hd-actions">
            {kind !== null && (
              <button
                type="button"
                className="modal-hd-action"
                onClick={() => setKind(null)}
                title="Voltar para a escolha do tipo"
              >
                ← TROCAR TIPO
              </button>
            )}
            <button
              type="button"
              className="action-btn"
              onClick={onClose}
              aria-label="Fechar"
            >
              <X size={14} />
            </button>
          </div>
        </div>

        {kind === null ? (
          <KindPicker onPick={setKind} />
        ) : (
        <>
          <div className="modal-tabs" role="tablist">
            <button
              type="button"
              role="tab"
              aria-selected={activeTab === "identity"}
              className={"modal-tab" + (activeTab === "identity" ? " modal-tab--on" : "")}
              onClick={() => setActiveTab("identity")}
            >
              IDENTIFICAÇÃO
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={activeTab === "data"}
              className={"modal-tab" + (activeTab === "data" ? " modal-tab--on" : "")}
              onClick={() => setActiveTab("data")}
            >
              DADOS
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={activeTab === "media"}
              className={"modal-tab" + (activeTab === "media" ? " modal-tab--on" : "")}
              onClick={() => setActiveTab("media")}
            >
              MÍDIA
            </button>
          </div>

          <form className="modal-bd entity-form" onSubmit={onSubmit} autoComplete="off">
            {activeTab === "identity" && (
              <>
                <label className="form-field">
                  <span>{kind === "person" ? "NOME COMPLETO" : "NOME"}</span>
                  <input
                    type="text"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                    autoFocus
                    maxLength={200}
                  />
                </label>

                <label className="form-field">
                  <span>DESCRIÇÃO</span>
                  <textarea
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    rows={3}
                    maxLength={2000}
                  />
                </label>

                <label className="form-field">
                  <span>TAGS</span>
                  <TagInput value={tags} onChange={setTags} normalize="lower" />
                </label>
              </>
            )}

            {activeTab === "data" && (
              <>
                {kind === "person" && (
                  <PersonFields
                    aliases={aliases}
                    setAliases={setAliases}
                    motherName={motherName}
                    setMotherName={setMotherName}
                    gender={gender}
                    setGender={setGender}
                    dob={dob}
                    setDob={setDob}
                    cpf={cpf}
                    setCpf={setCpf}
                    orcrimId={orcrimId}
                    setOrcrimId={setOrcrimId}
                  />
                )}
                {kind === "organization" && (
                  <OrganizationFields
                    sigla={orgSigla}
                    setSigla={setOrgSigla}
                    legalName={legalName}
                    setLegalName={setLegalName}
                    taxID={taxID}
                    setTaxID={setTaxID}
                    foundedAt={foundedAt}
                    setFoundedAt={setFoundedAt}
                  />
                )}
                {kind === "place" && (
                  <PlaceFields
                    address={address}
                    setAddress={setAddress}
                    country={country}
                    setCountry={setCountry}
                    region={region}
                    setRegion={setRegion}
                    latitude={latitude}
                    setLatitude={setLatitude}
                    longitude={longitude}
                    setLongitude={setLongitude}
                  />
                )}
              </>
            )}

            {activeTab === "media" && (
              <>
                {(kind === "person" || kind === "place") && (
                  <PrimaryPhotoPicker
                    file={photoFile}
                    onFileChange={setPhotoFile}
                    label={kind === "person" ? "FOTO PRINCIPAL · 3X4" : "FOTO PRINCIPAL"}
                  />
                )}
                <PendingGalleryEditor
                  photos={galleryPending}
                  onChange={setGalleryPending}
                />
              </>
            )}

            {kind === "person" && homonyms.length > 0 && !cpfTaken && (
              <div
                className={
                  "banner " + (topScore >= 3 ? "banner-error" : "banner-warn")
                }
              >
                <AlertTriangle size={12} strokeWidth={2} />
                <span>
                  {topScore >= 3
                    ? "MUITO PROVÁVEL DUPLICATA"
                    : topScore === 2
                      ? "PROVÁVEL DUPLICATA"
                      : `${homonyms.length} HOMÔNIMO${homonyms.length > 1 ? "S" : ""}`}{" "}
                  · {homonyms.slice(0, 3).map((h) => h.name.toUpperCase()).join(" · ")}
                  {homonyms.length > 3 ? ` +${homonyms.length - 3}` : ""}
                </span>
              </div>
            )}
            {dupesLoading && kind === "person" && (
              <div className="muted" style={{ fontSize: 10 }}>
                // verificando duplicates…
              </div>
            )}

            {err && <div className="banner banner-error">⚠ {err}</div>}

            <div className="modal-ft">
              <button type="button" className="btn btn-ghost" onClick={onClose}>
                CANCELAR
              </button>
              {activeTab !== "identity" && (
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={() =>
                    setActiveTab(activeTab === "media" ? "data" : "identity")
                  }
                >
                  ← VOLTAR
                </button>
              )}
              <button type="submit" className="btn btn-primary" disabled={busy}>
                {activeTab === "media"
                  ? busy
                    ? "CRIANDO…"
                    : "CRIAR ENTIDADE"
                  : "PRÓXIMO →"}
              </button>
            </div>
          </form>
        </>
        )}
      </div>
    </div>
  );
}

// ────────────── KindPicker (etapa 1) ──────────────

function KindPicker({ onPick }: { onPick: (k: EntityKind) => void }) {
  return (
    <div className="modal-bd kind-picker-bd">
      <div className="kind-picker-hint">
        SELECIONE O TIPO DE ENTIDADE A CRIAR
      </div>
      <div className="kind-picker">
        <button
          type="button"
          className="kind-tile"
          onClick={() => onPick("person")}
          autoFocus
        >
          <User size={44} strokeWidth={1.4} className="kind-tile-icon" />
          <span>PESSOA</span>
          <span className="kind-tile-sub">INDIVÍDUO</span>
        </button>
        <button
          type="button"
          className="kind-tile"
          onClick={() => onPick("organization")}
        >
          <Building2 size={44} strokeWidth={1.4} className="kind-tile-icon" />
          <span>ORGANIZAÇÃO</span>
          <span className="kind-tile-sub">EMPRESA · ORCRIM</span>
        </button>
        <button
          type="button"
          className="kind-tile"
          onClick={() => onPick("place")}
        >
          <MapPin size={44} strokeWidth={1.4} className="kind-tile-icon" />
          <span>LUGAR</span>
          <span className="kind-tile-sub">ENDEREÇO · LOCAL</span>
        </button>
      </div>
    </div>
  );
}

// ────────────── helpers ──────────────

function parseFloatOrUndef(s: string): number | undefined {
  if (s.trim() === "") return undefined;
  const n = Number(s);
  return Number.isFinite(n) ? n : undefined;
}

function formatCpf(s: string): string {
  const d = s.replace(/\D/g, "").slice(0, 11);
  if (d.length <= 3) return d;
  if (d.length <= 6) return `${d.slice(0, 3)}.${d.slice(3)}`;
  if (d.length <= 9) return `${d.slice(0, 3)}.${d.slice(3, 6)}.${d.slice(6)}`;
  return `${d.slice(0, 3)}.${d.slice(3, 6)}.${d.slice(6, 9)}-${d.slice(9)}`;
}

// ────────────── Person fields ──────────────

function PersonFields(props: {
  aliases: string[];
  setAliases: (v: string[]) => void;
  motherName: string;
  setMotherName: (v: string) => void;
  gender: Gender | "";
  setGender: (v: Gender | "") => void;
  dob: string;
  setDob: (v: string) => void;
  cpf: string;
  setCpf: (v: string) => void;
  orcrimId: string;
  setOrcrimId: (v: string) => void;
}) {
  return (
    <fieldset className="form-fieldset">
      <legend>DADOS · PESSOA</legend>

      <label className="form-field">
        <span>NOME DA MÃE</span>
        <input
          type="text"
          value={props.motherName}
          onChange={(e) => props.setMotherName(e.target.value)}
          maxLength={200}
        />
      </label>

      <div className="form-grid-2">
        <label className="form-field">
          <span>GÊNERO</span>
          <select
            value={props.gender}
            onChange={(e) => props.setGender(e.target.value as Gender | "")}
          >
            <option value="">—</option>
            {GENDERS.map((g) => (
              <option key={g} value={g}>
                {GENDER_LABEL[g]}
              </option>
            ))}
          </select>
        </label>
        <label className="form-field">
          <span>DATA DE NASCIMENTO</span>
          <input
            type="date"
            value={props.dob}
            onChange={(e) => props.setDob(e.target.value)}
          />
        </label>
      </div>

      <label className="form-field">
        <span>CPF</span>
        <input
          type="text"
          value={formatCpf(props.cpf)}
          onChange={(e) => props.setCpf(e.target.value)}
          inputMode="numeric"
          maxLength={14}
        />
      </label>

      <label className="form-field">
        <span>APELIDOS</span>
        <TagInput value={props.aliases} onChange={props.setAliases} />
      </label>

      <OrcrimSelect value={props.orcrimId} onChange={props.setOrcrimId} />
    </fieldset>
  );
}

// ────────────── Orcrim select ──────────────

function OrcrimSelect({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  const [options, setOptions] = useState<Entity[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    listEntities({ kind: "organization", tag: "orcrim", limit: 100 })
      .then((res) => {
        if (alive) setOptions(res.items);
      })
      .catch((e) => {
        if (alive) setErr((e as Error).message);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  // Pré-computa o rótulo (sigla primária prioritária) e ordena por ele.
  const sorted = useMemo(() => {
    return [...options]
      .map((o) => {
        const aliases = isOrganization(o) ? o.attrs?.aliases : undefined;
        return { entity: o, label: orgPrimaryLabel(o.name, aliases) };
      })
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [options]);

  return (
    <label className="form-field">
      <span>ORCRIM · ORGANIZAÇÕES MARCADAS COM #ORCRIM</span>
      <select value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">{loading ? "CARREGANDO…" : "—"}</option>
        {sorted.map(({ entity: o, label }) => (
          <option key={o.id} value={o.id}>
            {label.toUpperCase()}
          </option>
        ))}
      </select>
      {err && (
        <div className="muted" style={{ fontSize: 10 }}>
          ⚠ não foi possível carregar orcrim: {err}
        </div>
      )}
      {!loading && sorted.length === 0 && !err && (
        <div className="muted" style={{ fontSize: 10 }}>
          Nenhuma organização com a tag #orcrim. Cadastre uma organização e marque-a com a tag.
        </div>
      )}
    </label>
  );
}

// ────────────── Organization fields ──────────────

function OrganizationFields(props: {
  sigla: string;
  setSigla: (v: string) => void;
  legalName: string;
  setLegalName: (v: string) => void;
  taxID: string;
  setTaxID: (v: string) => void;
  foundedAt: string;
  setFoundedAt: (v: string) => void;
}) {
  return (
    <fieldset className="form-fieldset">
      <legend>DADOS · ORGANIZAÇÃO</legend>
      <div className="form-grid-2">
        <label className="form-field">
          <span>SIGLA</span>
          <input
            type="text"
            value={props.sigla}
            onChange={(e) => props.setSigla(e.target.value)}
            maxLength={20}
          />
        </label>
        <label className="form-field">
          <span>RAZÃO SOCIAL</span>
          <input
            type="text"
            value={props.legalName}
            onChange={(e) => props.setLegalName(e.target.value)}
          />
        </label>
        <label className="form-field">
          <span>CNPJ / TAX ID</span>
          <input
            type="text"
            value={props.taxID}
            onChange={(e) => props.setTaxID(e.target.value)}
          />
        </label>
        <label className="form-field">
          <span>FUNDADA EM</span>
          <input
            type="date"
            value={props.foundedAt}
            onChange={(e) => props.setFoundedAt(e.target.value)}
          />
        </label>
      </div>
    </fieldset>
  );
}

// ────────────── Place fields ──────────────

function PlaceFields(props: {
  address: string;
  setAddress: (v: string) => void;
  country: string;
  setCountry: (v: string) => void;
  region: string;
  setRegion: (v: string) => void;
  latitude: string;
  setLatitude: (v: string) => void;
  longitude: string;
  setLongitude: (v: string) => void;
}) {
  return (
    <fieldset className="form-fieldset">
      <legend>DADOS · LUGAR</legend>
      <label className="form-field">
        <span>ENDEREÇO</span>
        <input
          type="text"
          value={props.address}
          onChange={(e) => props.setAddress(e.target.value)}
        />
      </label>
      <div className="form-grid-2">
        <label className="form-field">
          <span>PAÍS</span>
          <input
            type="text"
            value={props.country}
            onChange={(e) => props.setCountry(e.target.value)}
          />
        </label>
        <label className="form-field">
          <span>REGIÃO / ESTADO</span>
          <input
            type="text"
            value={props.region}
            onChange={(e) => props.setRegion(e.target.value)}
          />
        </label>
        <label className="form-field">
          <span>LATITUDE</span>
          <input
            type="text"
            inputMode="decimal"
            value={props.latitude}
            onChange={(e) => props.setLatitude(e.target.value)}
          />
        </label>
        <label className="form-field">
          <span>LONGITUDE</span>
          <input
            type="text"
            inputMode="decimal"
            value={props.longitude}
            onChange={(e) => props.setLongitude(e.target.value)}
          />
        </label>
      </div>
    </fieldset>
  );
}
