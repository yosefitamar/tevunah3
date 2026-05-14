"use client";

import { useEffect, useRef, useState } from "react";
import { Link2, Pencil, Plus, Trash2, X } from "lucide-react";
import TagInput from "../shared/TagInput";
import Combobox from "../shared/Combobox";
import { VEHICLE_BRANDS, modelsForBrand } from "@/lib/vehicle-catalog";
import { fetchAddressByCEP, formatCEP } from "@/lib/viacep";
import { useAuth } from "@/contexts/AuthContext";
import { useModal } from "@/contexts/ModalContext";
import {
  ENTITY_KIND_LABEL,
  GENDERS,
  GENDER_LABEL,
  RELATION_LABEL,
  RELATION_TYPES,
  isOrganization,
  isPerson,
  isPlace,
  isVehicle,
  linkOtherSide,
  orgPrimaryLabel,
  vehiclePrimaryLabel,
  type Entity,
  type EntityLink,
  type PersonAddress,
  type Gender,
  type OrganizationAttrs,
  type PersonAttrs,
  type PlaceAttrs,
  type RelationType,
  type VehicleAttrs,
} from "@/lib/entities-types";
import {
  createEntityLink,
  createPersonAddress,
  deleteEntity,
  deleteEntityLink,
  deleteEntityPhoto,
  deletePersonAddress,
  galleryPhotoURL,
  getEntity,
  listEntities,
  listEntityLinks,
  listPersonAddresses,
  photoURL,
  updateEntity,
  updatePersonAddress,
  uploadEntityPhoto,
} from "@/lib/entities-api";
import PrimaryPhotoPicker from "./PrimaryPhotoPicker";
import { PersistedGalleryEditor } from "./GalleryEditor";
import { canDeleteEntities, canEditEntities } from "@/lib/permissions";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";

type Props = {
  entityId: string;
  onClose: () => void;
  onChanged: () => void;
};

export default function EntidadeDrawer({ entityId, onClose, onChanged }: Props) {
  const { user: me } = useAuth();
  const modal = useModal();
  const [data, setData] = useState<Entity | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);

  const reload = () => {
    setLoading(true);
    setError(null);
    getEntity(entityId)
      .then((d) => setData(d.entity))
      .catch((e: ApiError) => setError(e.message || "Erro ao carregar"))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entityId]);

  async function handleDelete() {
    if (!data) return;
    const ok = await modal.confirm({
      variant: "error",
      title: "EXCLUIR ENTIDADE",
      message: `Confirme a exclusão de "${data.name.toUpperCase()}". O registro fica preservado no histórico de auditoria.`,
      confirm: "EXCLUIR",
      cancel: "CANCELAR",
      danger: true,
    });
    if (!ok) return;
    try {
      await deleteEntity(data.id);
      onChanged();
      onClose();
    } catch (e) {
      modal.alert({
        variant: "error",
        title: "ERRO",
        message: (e as ApiError).message || "Erro ao excluir",
      });
    }
  }

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <aside
        className="drawer drawer--wide"
        onClick={(e) => e.stopPropagation()}
        aria-label="Dossiê da entidade"
      >
        <div className="drawer-hd">
          <span>DOSSIÊ DA ENTIDADE</span>
          <button
            type="button"
            className="action-btn"
            onClick={onClose}
            aria-label="Fechar"
          >
            <X size={14} />
          </button>
        </div>

        <div className="drawer-bd">
          {loading && <div className="muted">// CARREGANDO…</div>}
          {error && <div className="banner banner-error">⚠ {error}</div>}

          {data && !editing && (
            <ViewMode
              data={data}
              canEdit={canEditEntities(me)}
              canDelete={canDeleteEntities(me)}
              onEdit={() => setEditing(true)}
              onDelete={handleDelete}
            />
          )}

          {data && editing && (
            <EditMode
              data={data}
              onCancel={() => setEditing(false)}
              onPhotoChanged={() => {
                reload();
                onChanged();
              }}
              onSaved={(updated) => {
                setData(updated);
                setEditing(false);
                onChanged();
              }}
              onConflict={() => {
                modal.alert({
                  variant: "warning",
                  title: "VERSÃO DESATUALIZADA",
                  message:
                    "A entidade foi modificada por outra pessoa enquanto você editava. Recarregando…",
                });
                reload();
                setEditing(false);
              }}
            />
          )}
        </div>
      </aside>
    </div>
  );
}

// ─────────────────────────── ViewMode ────────────────────────────

function ViewMode({
  data,
  canEdit,
  canDelete,
  onEdit,
  onDelete,
}: {
  data: Entity;
  canEdit: boolean;
  canDelete: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const hasPrimaryPhoto =
    (isPerson(data) && data.attrs?.has_photo) ||
    (isPlace(data) && data.attrs?.has_photo);

  return (
    <>
      <div className="dossier-head dossier-head--with-photo">
        {hasPrimaryPhoto && (
          <a
            className="dossier-photo"
            href={photoURL(data.id, data.version)}
            target="_blank"
            rel="noreferrer"
            title="Abrir foto em tamanho real"
          >
            <img
              src={photoURL(data.id, data.version)}
              alt={`foto de ${data.name}`}
            />
          </a>
        )}
        <div style={{ minWidth: 0, flex: 1 }}>
          <div className="dossier-code">{ENTITY_KIND_LABEL[data.kind]}</div>
          <div className="dossier-name">{data.name.toUpperCase()}</div>
          {data.description && (
            <div className="dossier-email">{data.description}</div>
          )}
          <div className="dossier-meta">
            <span>v{data.version}</span>
            <span>·</span>
            <span>ATUALIZADO {formatBR(data.updated_at)}</span>
          </div>
        </div>
      </div>

      {data.tags.length > 0 && (
        <div className="drawer-section">
          <div className="drawer-section-title">TAGS</div>
          <div className="tag-row">
            {data.tags.map((t) => (
              <span key={t} className="tag-chip">
                #{t}
              </span>
            ))}
          </div>
        </div>
      )}

      <div className="drawer-section">
        <div className="drawer-section-title">ATRIBUTOS</div>
        {isPerson(data) && <PersonView attrs={data.attrs} />}
        {isOrganization(data) && <OrganizationView attrs={data.attrs} />}
        {isPlace(data) && <PlaceView attrs={data.attrs} />}
        {isVehicle(data) && <VehicleView attrs={data.attrs} />}
      </div>

      {/* No dossiê (modo consulta) os vínculos e endereços são read-only.
          Adição/remoção fica disponível apenas no modo EDITAR. */}
      <LinksSection entityID={data.id} canManage={false} />

      {isPerson(data) && (
        <AddressesSection
          entityID={data.id}
          initial={data.attrs?.addresses ?? []}
          canManage={false}
        />
      )}


      {data.photos && data.photos.length > 0 && (
        <div className="drawer-section">
          <div className="drawer-section-title">OUTRAS FOTOS</div>
          <div className="gallery-grid">
            {data.photos.map((p) => (
              <div key={p.id} className="gallery-card">
                <a
                  className="gallery-thumb"
                  href={galleryPhotoURL(data.id, p.id, p.updated_at)}
                  target="_blank"
                  rel="noreferrer"
                >
                  <img
                    src={galleryPhotoURL(data.id, p.id, p.updated_at)}
                    alt={p.caption || "foto"}
                  />
                </a>
                {p.caption && (
                  <div
                    className="gallery-caption"
                    style={{ borderBottom: "none", textTransform: "uppercase" }}
                  >
                    {p.caption}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="drawer-section">
        <div className="drawer-section-title">REGISTRO</div>
        <dl className="dossier-list">
          <div>
            <dt>CRIADO EM</dt>
            <dd>{formatBR(data.created_at)}</dd>
          </div>
          <div>
            <dt>POR</dt>
            <dd className="id">{data.created_by}</dd>
          </div>
          <div>
            <dt>ATUALIZADO EM</dt>
            <dd>{formatBR(data.updated_at)}</dd>
          </div>
          <div>
            <dt>POR</dt>
            <dd className="id">{data.updated_by}</dd>
          </div>
        </dl>
      </div>

      <div className="drawer-actions">
        {canEdit && (
          <button type="button" className="btn btn-primary" onClick={onEdit}>
            <Pencil size={14} /> EDITAR
          </button>
        )}
        {canDelete && (
          <button type="button" className="btn btn-danger" onClick={onDelete}>
            <Trash2 size={14} /> EXCLUIR
          </button>
        )}
      </div>
    </>
  );
}

function PersonView({ attrs }: { attrs?: PersonAttrs }) {
  if (!attrs) return <div className="muted">// SEM DADOS</div>;
  const genderLabel = attrs.gender
    ? GENDER_LABEL[attrs.gender as Gender] ?? String(attrs.gender)
    : "—";
  return (
    <dl className="dossier-list">
      <Row label="NOME DA MÃE" value={attrs.mother_name ?? "—"} />
      <Row label="GÊNERO" value={genderLabel} />
      <Row label="NASCIMENTO" value={attrs.date_of_birth ?? "—"} />
      <Row label="CPF" value={attrs.cpf ? formatCpfMask(attrs.cpf) : "—"} />
      <Row label="ORCRIM" value={orcrimLabel(attrs)} />
      <Row label="APELIDOS" value={attrs.aliases?.join(", ") || "—"} />
    </dl>
  );
}

function OrganizationView({ attrs }: { attrs?: OrganizationAttrs }) {
  if (!attrs) return <div className="muted">// SEM DADOS</div>;
  return (
    <dl className="dossier-list">
      <Row label="SIGLA" value={attrs.aliases?.[0] ?? "—"} />
      <Row label="RAZÃO SOCIAL" value={attrs.legal_name ?? "—"} />
      <Row label="CNPJ / TAX ID" value={attrs.tax_id ?? "—"} />
      <Row label="FUNDADA EM" value={attrs.founded_at ?? "—"} />
    </dl>
  );
}

// Rótulo da orcrim no dossiê — sigla primária + nome quando ambos existem.
function orcrimLabel(attrs: PersonAttrs): string {
  if (!attrs.orcrim_name) return "—";
  if (attrs.orcrim_alias && attrs.orcrim_alias !== attrs.orcrim_name) {
    return `${attrs.orcrim_alias} · ${attrs.orcrim_name}`;
  }
  return attrs.orcrim_name;
}

function PlaceView({ attrs }: { attrs?: PlaceAttrs }) {
  if (!attrs) return <div className="muted">// SEM DADOS</div>;
  return (
    <dl className="dossier-list">
      <Row label="ENDEREÇO" value={attrs.address ?? "—"} />
      <Row label="PAÍS" value={attrs.country ?? "—"} />
      <Row label="REGIÃO" value={attrs.region ?? "—"} />
      <Row
        label="COORDENADAS"
        value={
          attrs.latitude != null && attrs.longitude != null
            ? `${attrs.latitude}, ${attrs.longitude}`
            : "—"
        }
      />
    </dl>
  );
}

function VehicleView({ attrs }: { attrs?: VehicleAttrs }) {
  if (!attrs) return <div className="muted">// SEM DADOS</div>;
  return (
    <dl className="dossier-list">
      <Row label="PLACA" value={attrs.plate ?? "—"} />
      <Row label="MARCA" value={attrs.brand ?? "—"} />
      <Row label="MODELO" value={attrs.model ?? "—"} />
      <Row label="COR" value={attrs.color ?? "—"} />
      <Row label="ANO" value={attrs.year != null ? String(attrs.year) : "—"} />
      <Row label="CHASSI" value={attrs.chassis ?? "—"} />
      <Row label="RENAVAM" value={attrs.renavam ?? "—"} />
    </dl>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function formatCpfMask(s: string): string {
  const d = s.replace(/\D/g, "").padStart(11, "0").slice(0, 11);
  return `${d.slice(0, 3)}.${d.slice(3, 6)}.${d.slice(6, 9)}-${d.slice(9)}`;
}

// ─────────────────────────── EditMode ────────────────────────────

function EditMode({
  data,
  onCancel,
  onSaved,
  onConflict,
  onPhotoChanged,
}: {
  data: Entity;
  onCancel: () => void;
  onSaved: (updated: Entity) => void;
  onConflict: () => void;
  onPhotoChanged: () => void;
}) {
  const [name, setName] = useState(data.name);
  const [description, setDescription] = useState(data.description ?? "");
  const [tags, setTags] = useState<string[]>(data.tags);

  const person = isPerson(data) ? data.attrs : undefined;
  const org = isOrganization(data) ? data.attrs : undefined;
  const place = isPlace(data) ? data.attrs : undefined;
  const vehicle = isVehicle(data) ? data.attrs : undefined;

  // Person
  const [aliases, setAliases] = useState<string[]>(person?.aliases ?? []);
  const [gender, setGender] = useState<Gender | "">(
    (person?.gender as Gender | undefined) ?? "",
  );
  const [dob, setDob] = useState(person?.date_of_birth ?? "");
  const [motherName, setMotherName] = useState(person?.mother_name ?? "");
  const [cpf, setCpf] = useState(person?.cpf ?? "");
  const [orcrimId, setOrcrimId] = useState(person?.orcrim_id ?? "");
  // Organization
  const [orgSigla, setOrgSigla] = useState(org?.aliases?.[0] ?? "");
  const [legalName, setLegalName] = useState(org?.legal_name ?? "");
  const [taxID, setTaxID] = useState(org?.tax_id ?? "");
  const [foundedAt, setFoundedAt] = useState(org?.founded_at ?? "");
  // Place
  const [address, setAddress] = useState(place?.address ?? "");
  const [country, setCountry] = useState(place?.country ?? "");
  const [region, setRegion] = useState(place?.region ?? "");
  const [latitude, setLatitude] = useState(
    place?.latitude != null ? String(place.latitude) : "",
  );
  const [longitude, setLongitude] = useState(
    place?.longitude != null ? String(place.longitude) : "",
  );
  // Vehicle
  const [vPlate, setVPlate] = useState(vehicle?.plate ?? "");
  const [vBrand, setVBrand] = useState(vehicle?.brand ?? "");
  const [vModel, setVModel] = useState(vehicle?.model ?? "");
  const [vColor, setVColor] = useState(vehicle?.color ?? "");
  const [vYear, setVYear] = useState(
    vehicle?.year != null ? String(vehicle.year) : "",
  );
  const [vChassis, setVChassis] = useState(vehicle?.chassis ?? "");
  const [vRenavam, setVRenavam] = useState(vehicle?.renavam ?? "");

  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    if (!name.trim()) {
      setErr("Nome é obrigatório");
      return;
    }
    setBusy(true);
    try {
      const payload = {
        version: data.version,
        // Veículo deriva o nome do backend (MARCA MODELO/placa). Não enviamos
        // pra que o re-derive a partir dos attrs atualizados.
        ...(data.kind !== "vehicle" ? { name: name.trim() } : {}),
        description: description.trim(),
        tags,
        person:
          data.kind === "person"
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
          data.kind === "organization"
            ? {
                aliases: orgSigla.trim() ? [orgSigla.trim()] : [],
                legal_name: legalName.trim() || undefined,
                tax_id: taxID.trim() || undefined,
                founded_at: foundedAt || undefined,
              }
            : undefined,
        place:
          data.kind === "place"
            ? {
                address: address.trim() || undefined,
                country: country.trim() || undefined,
                region: region.trim() || undefined,
                latitude: parseFloatOrUndef(latitude),
                longitude: parseFloatOrUndef(longitude),
              }
            : undefined,
        vehicle:
          data.kind === "vehicle"
            ? {
                plate: vPlate.trim() || undefined,
                brand: vBrand.trim() || undefined,
                model: vModel.trim() || undefined,
                color: vColor.trim() || undefined,
                year: parseIntOrUndef(vYear),
                chassis: vChassis.trim() || undefined,
                renavam: vRenavam.trim() || undefined,
              }
            : undefined,
      };
      const res = await updateEntity(data.id, payload);
      onSaved(res.entity);
    } catch (e) {
      const apiErr = e as ApiError;
      if (apiErr.status === 409) {
        onConflict();
        return;
      }
      setErr(apiErr.message || "Erro ao salvar");
    } finally {
      setBusy(false);
    }
  }

  const supportsPrimaryPhoto = data.kind === "person" || data.kind === "place";

  const hasPrimaryPhoto =
    (isPerson(data) && data.attrs?.has_photo) ||
    (isPlace(data) && data.attrs?.has_photo);

  async function uploadPrimary(file: File) {
    await uploadEntityPhoto(data.id, file);
    onPhotoChanged();
  }
  async function removePrimary() {
    await deleteEntityPhoto(data.id);
    onPhotoChanged();
  }

  return (
    <form className="entity-form" onSubmit={onSubmit} autoComplete="off">
      {supportsPrimaryPhoto && (
        <PrimaryPhotoPicker
          currentURL={hasPrimaryPhoto ? photoURL(data.id, data.version) : null}
          onUpload={uploadPrimary}
          onRemove={hasPrimaryPhoto ? removePrimary : undefined}
          label={data.kind === "person" ? "FOTO PRINCIPAL · 3X4" : "FOTO PRINCIPAL"}
        />
      )}

      {data.kind !== "vehicle" && (
        <label className="form-field">
          <span>NOME</span>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            autoFocus
          />
        </label>
      )}

      <label className="form-field">
        <span>DESCRIÇÃO</span>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={3}
        />
      </label>

      <label className="form-field">
        <span>TAGS</span>
        <TagInput value={tags} onChange={setTags} normalize="lower" />
      </label>

      {data.kind === "person" && (
        <fieldset className="form-fieldset">
          <legend>DADOS · PESSOA</legend>
          <label className="form-field">
            <span>NOME DA MÃE</span>
            <input
              type="text"
              value={motherName}
              onChange={(e) => setMotherName(e.target.value)}
              maxLength={200}
            />
          </label>
          <div className="form-grid-2">
            <label className="form-field">
              <span>GÊNERO</span>
              <select
                value={gender}
                onChange={(e) => setGender(e.target.value as Gender | "")}
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
              <span>NASCIMENTO</span>
              <input
                type="date"
                value={dob}
                onChange={(e) => setDob(e.target.value)}
              />
            </label>
            <label className="form-field" style={{ gridColumn: "1 / -1" }}>
              <span>CPF</span>
              <input
                type="text"
                value={cpf}
                onChange={(e) => setCpf(e.target.value)}
                inputMode="numeric"
                maxLength={14}
              />
            </label>
          </div>
          <label className="form-field">
            <span>APELIDOS</span>
            <TagInput value={aliases} onChange={setAliases} />
          </label>
          <DrawerOrcrimSelect value={orcrimId} onChange={setOrcrimId} />
        </fieldset>
      )}

      {data.kind === "organization" && (
        <fieldset className="form-fieldset">
          <legend>DADOS · ORGANIZAÇÃO</legend>
          <div className="form-grid-2">
            <label className="form-field">
              <span>SIGLA</span>
              <input
                type="text"
                value={orgSigla}
                onChange={(e) => setOrgSigla(e.target.value)}
                maxLength={20}
              />
            </label>
            <label className="form-field">
              <span>RAZÃO SOCIAL</span>
              <input
                type="text"
                value={legalName}
                onChange={(e) => setLegalName(e.target.value)}
              />
            </label>
            <label className="form-field">
              <span>CNPJ / TAX ID</span>
              <input
                type="text"
                value={taxID}
                onChange={(e) => setTaxID(e.target.value)}
              />
            </label>
            <label className="form-field">
              <span>FUNDADA EM</span>
              <input
                type="date"
                value={foundedAt}
                onChange={(e) => setFoundedAt(e.target.value)}
              />
            </label>
          </div>
        </fieldset>
      )}

      {data.kind === "place" && (
        <fieldset className="form-fieldset">
          <legend>DADOS · LUGAR</legend>
          <label className="form-field">
            <span>ENDEREÇO</span>
            <input
              type="text"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
            />
          </label>
          <div className="form-grid-2">
            <label className="form-field">
              <span>PAÍS</span>
              <input
                type="text"
                value={country}
                onChange={(e) => setCountry(e.target.value)}
              />
            </label>
            <label className="form-field">
              <span>REGIÃO</span>
              <input
                type="text"
                value={region}
                onChange={(e) => setRegion(e.target.value)}
              />
            </label>
            <label className="form-field">
              <span>LATITUDE</span>
              <input
                type="text"
                inputMode="decimal"
                value={latitude}
                onChange={(e) => setLatitude(e.target.value)}
              />
            </label>
            <label className="form-field">
              <span>LONGITUDE</span>
              <input
                type="text"
                inputMode="decimal"
                value={longitude}
                onChange={(e) => setLongitude(e.target.value)}
              />
            </label>
          </div>
        </fieldset>
      )}

      {data.kind === "vehicle" && (
        <fieldset className="form-fieldset">
          <legend>DADOS · VEÍCULO</legend>
          <div className="form-grid-2">
            <label className="form-field">
              <span>PLACA</span>
              <input
                type="text"
                value={vPlate}
                onChange={(e) => setVPlate(e.target.value.toUpperCase())}
                maxLength={10}
              />
            </label>
            <label className="form-field">
              <span>ANO</span>
              <input
                type="text"
                inputMode="numeric"
                value={vYear}
                onChange={(e) => setVYear(e.target.value)}
                maxLength={4}
              />
            </label>
            <label className="form-field">
              <span>MARCA</span>
              <Combobox
                value={vBrand}
                onChange={(v) => {
                  setVBrand(v);
                  if (vModel && !modelsForBrand(v).includes(vModel.toUpperCase())) {
                    setVModel("");
                  }
                }}
                options={VEHICLE_BRANDS}
                uppercase
              />
            </label>
            <label className="form-field">
              <span>MODELO</span>
              <Combobox
                value={vModel}
                onChange={setVModel}
                options={modelsForBrand(vBrand)}
                uppercase
                placeholder={vBrand ? "selecione ou digite" : "selecione a marca primeiro"}
              />
            </label>
            <label className="form-field">
              <span>COR</span>
              <input
                type="text"
                value={vColor}
                onChange={(e) => setVColor(e.target.value)}
                maxLength={30}
              />
            </label>
            <label className="form-field">
              <span>RENAVAM</span>
              <input
                type="text"
                inputMode="numeric"
                value={vRenavam}
                onChange={(e) => setVRenavam(e.target.value)}
                maxLength={11}
              />
            </label>
            <label className="form-field" style={{ gridColumn: "1 / -1" }}>
              <span>CHASSI</span>
              <input
                type="text"
                value={vChassis}
                onChange={(e) => setVChassis(e.target.value.toUpperCase())}
                maxLength={17}
              />
            </label>
          </div>
        </fieldset>
      )}

      <PersistedGalleryEditor
        entityID={data.id}
        photos={data.photos ?? []}
        onChanged={onPhotoChanged}
      />

      {/* Gerenciamento dos relacionamentos (vínculos e endereços) só fica
          disponível no modo EDITAR. Adições/remoções persistem imediatamente
          via API — independentes do SALVAR da entidade base. */}
      <LinksSection entityID={data.id} canManage={true} />

      {data.kind === "person" && (
        <AddressesSection
          entityID={data.id}
          initial={isPerson(data) ? data.attrs?.addresses ?? [] : []}
          canManage={true}
        />
      )}

      {err && <div className="banner banner-error">⚠ {err}</div>}

      <div className="drawer-actions">
        <button type="button" className="btn btn-ghost" onClick={onCancel} disabled={busy}>
          CANCELAR
        </button>
        <button type="submit" className="btn btn-primary" disabled={busy}>
          {busy ? "SALVANDO…" : "SALVAR"}
        </button>
      </div>
    </form>
  );
}

function parseIntOrUndef(s: string): number | undefined {
  if (s.trim() === "") return undefined;
  const n = parseInt(s, 10);
  return Number.isFinite(n) ? n : undefined;
}

function parseFloatOrUndef(s: string): number | undefined {
  if (s.trim() === "") return undefined;
  const n = Number(s);
  return Number.isFinite(n) ? n : undefined;
}

// ─────────────────────────── DrawerOrcrimSelect ────────────────────────────

function DrawerOrcrimSelect({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  // Lazy import circular-free: usa listEntities diretamente.
  const [options, setOptions] = useState<Entity[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    listEntities({ kind: "organization", tag: "orcrim", limit: 100 })
      .then((res) => {
        if (alive) setOptions(res.items);
      })
      .catch(() => {
        /* silencioso — campo é opcional */
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  const sorted = [...options]
    .map((o) => {
      const aliases = isOrganization(o) ? o.attrs?.aliases : undefined;
      return { entity: o, label: orgPrimaryLabel(o.name, aliases) };
    })
    .sort((a, b) => a.label.localeCompare(b.label));

  return (
    <label className="form-field">
      <span>ORCRIM</span>
      <select value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">{loading ? "CARREGANDO…" : "—"}</option>
        {sorted.map(({ entity: o, label }) => (
          <option key={o.id} value={o.id}>
            {label.toUpperCase()}
          </option>
        ))}
      </select>
    </label>
  );
}

// ─────────────────────────── LinksSection ───────────────────────────
//
// Lista os vínculos (saindo/chegando) da entidade aberta no drawer e
// permite criar/remover. Reload local — não recarrega o dossiê inteiro.

function LinksSection({
  entityID,
  canManage,
}: {
  entityID: string;
  canManage: boolean;
}) {
  const modal = useModal();
  const [links, setLinks] = useState<EntityLink[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [removingID, setRemovingID] = useState<string | null>(null);

  function reload() {
    setLoading(true);
    listEntityLinks(entityID)
      .then((r) => setLinks(r.items))
      .catch(() => setLinks([]))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entityID]);

  async function handleRemove(l: EntityLink) {
    const other = linkOtherSide(l);
    const ok = await modal.confirm({
      variant: "warning",
      title: "REMOVER VÍNCULO",
      message: (
        <>
          Remover vínculo <b>{RELATION_LABEL[l.relation_type]}</b> com{" "}
          <b>{other.name.toUpperCase()}</b>?
        </>
      ),
      confirm: "REMOVER",
      cancel: "CANCELAR",
      danger: true,
    });
    if (!ok) return;
    setRemovingID(l.id);
    try {
      await deleteEntityLink(entityID, l.id);
      reload();
    } catch (e) {
      modal.alert({
        variant: "error",
        title: "ERRO",
        message: (e as ApiError).message || "Erro ao remover vínculo",
      });
    } finally {
      setRemovingID(null);
    }
  }

  return (
    <div className="drawer-section">
      <div className="drawer-section-title" style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <span>VÍNCULOS{links && links.length > 0 ? ` · ${links.length}` : ""}</span>
        {canManage && (
          <button
            type="button"
            className="btn btn-ghost"
            style={{ height: 24, fontSize: 9.5, padding: "0 8px" }}
            onClick={() => setAdding(true)}
            title="Adicionar vínculo"
          >
            <Plus size={11} strokeWidth={2} /> ADICIONAR
          </button>
        )}
      </div>

      {loading && <div className="muted">// CARREGANDO…</div>}
      {!loading && (!links || links.length === 0) && (
        <div className="muted" style={{ fontSize: 11 }}>
          // sem vínculos cadastrados
        </div>
      )}
      {!loading && links && links.length > 0 && (
        <ul className="link-list">
          {links.map((l) => {
            const other = linkOtherSide(l);
            const arrow = l.direction === "out" ? "→" : "←";
            return (
              <li key={l.id} className="link-row">
                <span className="link-arrow" title={l.direction === "out" ? "sai daqui" : "chega aqui"}>
                  {arrow}
                </span>
                <span className="link-relation">{RELATION_LABEL[l.relation_type]}</span>
                <span className="link-other">
                  <Link2 size={11} strokeWidth={1.8} />{" "}
                  <span className="link-other-kind">{ENTITY_KIND_LABEL[other.kind]}</span>{" "}
                  <span className="link-other-name">{other.name.toUpperCase()}</span>
                </span>
                {l.note && <span className="link-note">· {l.note}</span>}
                {canManage && (
                  <button
                    type="button"
                    className="link-remove"
                    onClick={() => handleRemove(l)}
                    disabled={removingID === l.id}
                    title="Remover vínculo"
                  >
                    <Trash2 size={12} strokeWidth={1.8} />
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      )}

      {adding && (
        <AddLinkModal
          entityID={entityID}
          onClose={() => setAdding(false)}
          onCreated={() => {
            setAdding(false);
            reload();
          }}
        />
      )}
    </div>
  );
}

// ─────────────────────────── AddLinkModal ───────────────────────────

function AddLinkModal({
  entityID,
  onClose,
  onCreated,
}: {
  entityID: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const modal = useModal();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Entity[]>([]);
  const [searching, setSearching] = useState(false);
  const [picked, setPicked] = useState<Entity | null>(null);
  const [relation, setRelation] = useState<RelationType>("owns");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // Busca debounced. Só dispara com >=2 chars. Limita resultados via API.
  useEffect(() => {
    const q = query.trim();
    if (q.length < 2) {
      setResults([]);
      return;
    }
    let alive = true;
    setSearching(true);
    const handle = window.setTimeout(() => {
      listEntities({ search: q, limit: 10 })
        .then((r) => {
          if (!alive) return;
          // Esconde a própria entidade da lista — não permite auto-link.
          setResults(r.items.filter((it) => it.id !== entityID));
        })
        .catch(() => alive && setResults([]))
        .finally(() => alive && setSearching(false));
    }, 250);
    return () => {
      alive = false;
      window.clearTimeout(handle);
    };
  }, [query, entityID]);

  async function onSubmit() {
    if (!picked) {
      setErr("Selecione uma entidade alvo");
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      await createEntityLink(entityID, {
        to_entity_id: picked.id,
        relation_type: relation,
        note: note.trim() || undefined,
      });
      onCreated();
    } catch (e) {
      const apiErr = e as ApiError;
      if (apiErr.status === 409) {
        modal.alert({
          variant: "warning",
          title: "VÍNCULO JÁ EXISTE",
          message:
            "Este vínculo entre as duas entidades já está cadastrado com este tipo.",
        });
      } else {
        setErr(apiErr.message || "Erro ao criar vínculo");
      }
    } finally {
      setBusy(false);
    }
  }

  function entityLabel(e: Entity): string {
    if (e.kind === "organization") {
      const aliases = isOrganization(e) ? e.attrs?.aliases : undefined;
      return orgPrimaryLabel(e.name, aliases);
    }
    if (e.kind === "vehicle") {
      const plate = isVehicle(e) ? e.attrs?.plate : undefined;
      return vehiclePrimaryLabel(e.name, plate);
    }
    return e.name;
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <span>NOVO VÍNCULO</span>
          <button
            type="button"
            className="action-btn"
            onClick={onClose}
            aria-label="Fechar"
          >
            <X size={14} />
          </button>
        </div>

        <div className="modal-bd">
          <label className="form-field">
            <span>BUSCAR ENTIDADE ALVO</span>
            <input
              type="text"
              autoFocus
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setPicked(null);
              }}
              placeholder="digite nome, placa, sigla…"
            />
          </label>

          {!picked && query.trim().length >= 2 && (
            <div className="link-search-results">
              {searching && <div className="muted" style={{ fontSize: 11 }}>// buscando…</div>}
              {!searching && results.length === 0 && (
                <div className="muted" style={{ fontSize: 11 }}>// nenhum resultado</div>
              )}
              {!searching &&
                results.map((e) => (
                  <button
                    key={e.id}
                    type="button"
                    className="link-search-item"
                    onClick={() => {
                      setPicked(e);
                      setQuery(entityLabel(e));
                    }}
                  >
                    <span className="link-search-kind">{ENTITY_KIND_LABEL[e.kind]}</span>
                    <span className="link-search-name">{entityLabel(e).toUpperCase()}</span>
                  </button>
                ))}
            </div>
          )}

          {picked && (
            <div className="link-picked">
              SELECIONADO: <b>{ENTITY_KIND_LABEL[picked.kind]}</b> · {entityLabel(picked).toUpperCase()}
            </div>
          )}

          <label className="form-field">
            <span>TIPO DE RELAÇÃO</span>
            <select
              value={relation}
              onChange={(e) => setRelation(e.target.value as RelationType)}
            >
              {RELATION_TYPES.map((rt) => (
                <option key={rt} value={rt}>
                  {RELATION_LABEL[rt]}
                </option>
              ))}
            </select>
          </label>

          <label className="form-field">
            <span>NOTA (OPCIONAL)</span>
            <input
              type="text"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              maxLength={500}
            />
          </label>

          {err && <div className="banner banner-error">⚠ {err}</div>}

          <div className="modal-ft">
            <button type="button" className="btn btn-ghost" onClick={onClose}>
              CANCELAR
            </button>
            <button
              type="button"
              className="btn btn-primary"
              onClick={onSubmit}
              disabled={busy || !picked}
            >
              {busy ? "CRIANDO…" : "CRIAR VÍNCULO"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}



// ─────────────────────────── AddressesSection ───────────────────────────
//
// Endereços persistidos da pessoa. CRUD imediato contra os endpoints
// /api/entities/{id}/addresses. Render sempre na perspectiva ViewMode — a
// edição é inline por linha (botões EDITAR / REMOVER) sem trocar pra um
// EditMode global. Não interfere com a edição da entidade base.

type AddressEditState = {
  label: string;
  cep: string;
  street: string;
  number: string;
  complement: string;
  neighborhood: string;
  city: string;
  state: string;
};

function emptyAddressEdit(seed?: PersonAddress): AddressEditState {
  return {
    label: seed?.label ?? "",
    cep: seed?.cep ? formatCEP(seed.cep) : "",
    street: seed?.street ?? "",
    number: seed?.number ?? "",
    complement: seed?.complement ?? "",
    neighborhood: seed?.neighborhood ?? "",
    city: seed?.city ?? "",
    state: seed?.state ?? "",
  };
}

function addressEditToPayload(e: AddressEditState) {
  return {
    label: e.label.trim() || undefined,
    cep: e.cep.trim() || undefined,
    street: e.street.trim() || undefined,
    number: e.number.trim() || undefined,
    complement: e.complement.trim() || undefined,
    neighborhood: e.neighborhood.trim() || undefined,
    city: e.city.trim() || undefined,
    state: e.state.trim() || undefined,
  };
}

function AddressesSection({
  entityID,
  initial,
  canManage,
}: {
  entityID: string;
  initial: PersonAddress[];
  canManage: boolean;
}) {
  const modal = useModal();
  const [list, setList] = useState<PersonAddress[]>(initial);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);

  function reload() {
    listPersonAddresses(entityID)
      .then((r) => setList(r.items))
      .catch(() => {
        /* mantém estado atual */
      });
  }

  async function handleRemove(addr: PersonAddress) {
    const ok = await modal.confirm({
      variant: "warning",
      title: "REMOVER ENDEREÇO",
      message: (
        <>
          Remover endereço {addr.label ? <b>{addr.label.toUpperCase()}</b> : "sem rótulo"}?
        </>
      ),
      confirm: "REMOVER",
      cancel: "CANCELAR",
      danger: true,
    });
    if (!ok) return;
    try {
      await deletePersonAddress(entityID, addr.id);
      reload();
    } catch (e) {
      modal.alert({
        variant: "error",
        title: "ERRO",
        message: (e as ApiError).message || "Erro ao remover endereço",
      });
    }
  }

  function fmtAddress(a: PersonAddress): string {
    const parts: string[] = [];
    if (a.street) parts.push(a.street);
    if (a.number) parts.push(`Nº ${a.number}`);
    if (a.complement) parts.push(a.complement);
    if (a.neighborhood) parts.push(a.neighborhood);
    if (a.city) parts.push(a.state ? `${a.city}/${a.state}` : a.city);
    else if (a.state) parts.push(a.state);
    if (a.cep) parts.push(`CEP ${formatCEP(a.cep)}`);
    return parts.join(" · ") || "(sem dados)";
  }

  return (
    <div className="drawer-section">
      <div
        className="drawer-section-title"
        style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}
      >
        <span>ENDEREÇOS{list.length > 0 ? ` · ${list.length}` : ""}</span>
        {canManage && !adding && (
          <button
            type="button"
            className="btn btn-ghost"
            style={{ height: 24, fontSize: 9.5, padding: "0 8px" }}
            onClick={() => setAdding(true)}
          >
            <Plus size={11} strokeWidth={2} /> ADICIONAR
          </button>
        )}
      </div>

      {list.length === 0 && !adding && (
        <div className="muted" style={{ fontSize: 11 }}>
          // sem endereços cadastrados
        </div>
      )}

      <ul className="address-list">
        {list.map((a) => (
          <li key={a.id}>
            {editingID === a.id ? (
              <AddressEditor
                seed={a}
                onCancel={() => setEditingID(null)}
                onSave={async (payload) => {
                  await updatePersonAddress(entityID, a.id, payload);
                  setEditingID(null);
                  reload();
                }}
              />
            ) : (
              <div className="address-card">
                <div className="address-card-hd">
                  <span className="address-card-label">
                    {a.label ? a.label.toUpperCase() : "ENDEREÇO"}
                  </span>
                  {canManage && (
                    <div className="address-card-actions">
                      <button
                        type="button"
                        className="link-remove"
                        onClick={() => setEditingID(a.id)}
                        title="Editar endereço"
                      >
                        <Pencil size={12} strokeWidth={1.8} />
                      </button>
                      <button
                        type="button"
                        className="link-remove"
                        onClick={() => handleRemove(a)}
                        title="Remover endereço"
                      >
                        <Trash2 size={12} strokeWidth={1.8} />
                      </button>
                    </div>
                  )}
                </div>
                <div className="address-card-body">{fmtAddress(a)}</div>
              </div>
            )}
          </li>
        ))}
      </ul>

      {adding && (
        <AddressEditor
          onCancel={() => setAdding(false)}
          onSave={async (payload) => {
            await createPersonAddress(entityID, payload);
            setAdding(false);
            reload();
          }}
        />
      )}
    </div>
  );
}

// AddressEditor — formulário inline para criar ou editar um endereço. Faz
// a chamada via `onSave`, que decide POST/PATCH. CEP reativo via ViaCEP.
function AddressEditor({
  seed,
  onSave,
  onCancel,
}: {
  seed?: PersonAddress;
  onSave: (payload: ReturnType<typeof addressEditToPayload>) => Promise<void>;
  onCancel: () => void;
}) {
  const modal = useModal();
  const [state, setState] = useState<AddressEditState>(emptyAddressEdit(seed));
  const [busy, setBusy] = useState(false);
  const [cepLoading, setCepLoading] = useState(false);
  const [cepError, setCepError] = useState(false);
  const cepDebounce = useRef<number | null>(null);

  function patch(p: Partial<AddressEditState>) {
    setState((s) => ({ ...s, ...p }));
  }

  function handleCepInput(v: string) {
    const masked = formatCEP(v);
    patch({ cep: masked });
    setCepError(false);
    const digits = v.replace(/\D/g, "");
    if (cepDebounce.current) window.clearTimeout(cepDebounce.current);
    if (digits.length !== 8) return;
    cepDebounce.current = window.setTimeout(async () => {
      setCepLoading(true);
      const res = await fetchAddressByCEP(digits);
      setCepLoading(false);
      if (!res) {
        setCepError(true);
        return;
      }
      setState((s) => ({
        ...s,
        street: res.logradouro || s.street,
        neighborhood: res.bairro || s.neighborhood,
        city: res.localidade || s.city,
        state: res.uf || s.state,
      }));
    }, 350);
  }

  async function save() {
    setBusy(true);
    try {
      await onSave(addressEditToPayload(state));
    } catch (e) {
      modal.alert({
        variant: "error",
        title: "ERRO",
        message: (e as ApiError).message || "Erro ao salvar endereço",
      });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="address-row">
      <div className="address-row-hd">
        <input
          type="text"
          className="address-row-label"
          value={state.label}
          onChange={(e) => patch({ label: e.target.value })}
          placeholder='ex.: "casa", "casa da mãe", "trabalho"'
          maxLength={80}
        />
      </div>
      <div className="form-grid-2">
        <label className="form-field">
          <span>CEP</span>
          <input
            type="text"
            inputMode="numeric"
            value={state.cep}
            onChange={(e) => handleCepInput(e.target.value)}
            maxLength={9}
            placeholder="00000-000"
          />
          {cepLoading && <span className="muted" style={{ fontSize: 9.5 }}>// consultando…</span>}
          {cepError && <span style={{ fontSize: 9.5, color: "var(--warn)" }}>⚠ CEP não encontrado</span>}
        </label>
        <label className="form-field">
          <span>NÚMERO</span>
          <input
            type="text"
            value={state.number}
            onChange={(e) => patch({ number: e.target.value })}
            maxLength={20}
          />
        </label>
        <label className="form-field" style={{ gridColumn: "1 / -1" }}>
          <span>LOGRADOURO</span>
          <input
            type="text"
            value={state.street}
            onChange={(e) => patch({ street: e.target.value })}
            maxLength={200}
          />
        </label>
        <label className="form-field" style={{ gridColumn: "1 / -1" }}>
          <span>COMPLEMENTO</span>
          <input
            type="text"
            value={state.complement}
            onChange={(e) => patch({ complement: e.target.value })}
            maxLength={200}
          />
        </label>
        <label className="form-field">
          <span>BAIRRO</span>
          <input
            type="text"
            value={state.neighborhood}
            onChange={(e) => patch({ neighborhood: e.target.value })}
            maxLength={120}
          />
        </label>
        <label className="form-field">
          <span>CIDADE</span>
          <input
            type="text"
            value={state.city}
            onChange={(e) => patch({ city: e.target.value })}
            maxLength={120}
          />
        </label>
        <label className="form-field">
          <span>UF</span>
          <input
            type="text"
            value={state.state}
            onChange={(e) => patch({ state: e.target.value.toUpperCase() })}
            maxLength={2}
          />
        </label>
      </div>
      <div className="address-editor-actions">
        <button type="button" className="btn btn-ghost" onClick={onCancel} disabled={busy}>
          CANCELAR
        </button>
        <button type="button" className="btn btn-primary" onClick={save} disabled={busy}>
          {busy ? "SALVANDO…" : "SALVAR ENDEREÇO"}
        </button>
      </div>
    </div>
  );
}
