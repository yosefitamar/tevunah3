"use client";

import { useEffect, useState } from "react";
import { Pencil, Trash2, X } from "lucide-react";
import TagInput from "../shared/TagInput";
import { useAuth } from "@/contexts/AuthContext";
import { useModal } from "@/contexts/ModalContext";
import {
  ENTITY_KIND_LABEL,
  GENDERS,
  GENDER_LABEL,
  isOrganization,
  isPerson,
  isPlace,
  orgPrimaryLabel,
  type Entity,
  type Gender,
  type OrganizationAttrs,
  type PersonAttrs,
  type PlaceAttrs,
} from "@/lib/entities-types";
import {
  deleteEntity,
  deleteEntityPhoto,
  galleryPhotoURL,
  getEntity,
  listEntities,
  photoURL,
  updateEntity,
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
          <div className="dossier-photo">
            <img
              src={photoURL(data.id, data.version)}
              alt={`foto de ${data.name}`}
            />
          </div>
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
      </div>

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
        name: name.trim(),
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

      <PersistedGalleryEditor
        entityID={data.id}
        photos={data.photos ?? []}
        onChanged={onPhotoChanged}
      />

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

