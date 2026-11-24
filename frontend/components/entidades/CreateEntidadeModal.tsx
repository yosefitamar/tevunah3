"use client";

import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { createPortal } from "react-dom";
import { AlertTriangle, Building2, Car, MapPin, Plus, Trash2, User, X } from "lucide-react";
import TagInput from "../shared/TagInput";
import Combobox from "../shared/Combobox";
import Select from "../shared/Select";
import DateInput from "../shared/DateInput";
import {
  VEHICLE_COLORS,
  brandsForCategory,
  isValidPlate,
  modelsForBrand,
  normalizePlateInput,
} from "@/lib/vehicle-catalog";
import { useModal } from "@/contexts/ModalContext";
import PrimaryPhotoPicker from "./PrimaryPhotoPicker";
import { PendingGalleryEditor, type PendingPhoto } from "./GalleryEditor";
import MotherField from "./MotherField";
import PersonPdfDrop from "./PersonPdfDrop";
import CriminalRecordsField from "./CriminalRecordsField";
import DuplicateResolver, { type MergePlan } from "./DuplicateResolver";
import { useAuth } from "@/contexts/AuthContext";
import { canEditEntities } from "@/lib/permissions";
import {
  createEntity,
  createEntityLink,
  createPersonAddress,
  findPersonDuplicates,
  listEntities,
  markEntityDeath,
  updateEntity,
  uploadEntityPhoto,
  uploadGalleryPhoto,
  type AddressPayload,
  type PersonPdfResult,
  type DuplicatesResult,
  type OpsEntityLinks,
  type OpsEntityMatch,
  type PersonDuplicate,
  type UpdateEntityInput,
} from "@/lib/entities-api";
import { linkOpsPerson } from "@/lib/ops-reports-api";
import { formatBRDate, formatPhoneBR } from "@/lib/format";
import {
  ENTITY_KIND_LABEL,
  FAMILY_OPTIONS,
  GENDERS,
  GENDER_LABEL,
  RELATION_LABEL,
  RELATION_TYPES,
  VEHICLE_CATEGORIES,
  VEHICLE_CATEGORY_LABEL,
  isOrganization,
  isPerson,
  isVehicle,
  orgPrimaryLabel,
  vehiclePrimaryLabel,
  type CriminalRecord,
  type Entity,
  type EntityKind,
  type FamilyOption,
  type Gender,
  type PersonAttrs,
  type RelationType,
  type VehicleCategory,
} from "@/lib/entities-types";
import { fetchAddressByCEP, formatCEP } from "@/lib/viacep";
import type { ApiError } from "@/lib/api";

type Props = {
  onClose: () => void;
  onCreated: (id: string) => void;
  /** Pula a etapa 1 (seletor de tipo) e abre direto o wizard pra esse kind.
   *  Usado por fluxos como "novo veículo inline" no wizard de pessoa. */
  initialKind?: EntityKind;
  /** Campos de pessoa já conhecidos — usado pela importação de relatório de
   *  ocorrência, que chega com nome, mãe e nascimento já lidos do texto. */
  initialPerson?: {
    name?: string;
    motherName?: string;
    dateOfBirth?: string;
    alias?: string;
    /** Texto de contexto (antecedentes, endereço) lido do relatório. */
    description?: string;
  };
};

export default function CreateEntidadeModal({
  onClose,
  onCreated,
  initialKind,
  initialPerson,
}: Props) {
  // Fluxo em duas etapas: na etapa 1 (kind === null) o modal mostra apenas o
  // seletor de tipo; ao escolher, kind passa a ter valor e o formulário
  // específico do tipo é renderizado. O botão "trocar tipo" devolve para a
  // etapa 1 preservando campos comuns já preenchidos.
  const [kind, setKind] = useState<EntityKind | null>(initialKind ?? null);
  // Tabs do wizard. Ordem e conjunto variam por kind. Vehicle pula a tab
  // "identity" (não tem nome) e move descrição/tags pra "observations" (última).
  type Tab = "identity" | "data" | "history" | "media" | "observations";
  const [activeTab, setActiveTab] = useState<Tab>(
    initialKind ? tabsForKind(initialKind)[0] : "identity",
  );
  const tabs = kind ? tabsForKind(kind) : (["identity", "data", "media"] as Tab[]);
  const tabLabels: Record<Tab, string> = {
    identity: "IDENTIFICAÇÃO",
    data: "DADOS",
    history: "HISTÓRICO",
    media: "MÍDIA",
    observations: "DESCRIÇÃO",
  };
  const [name, setName] = useState(initialPerson?.name ?? "");
  const [description, setDescription] = useState(initialPerson?.description ?? "");
  const [tags, setTags] = useState<string[]>([]);

  // Foto primária — aplicável a Pessoa e Lugar.
  const [photoFile, setPhotoFile] = useState<File | null>(null);
  // Galeria — pendentes em memória, enviadas após createEntity.
  const [galleryPending, setGalleryPending] = useState<PendingPhoto[]>([]);

  // Person
  const [aliases, setAliases] = useState<string[]>(
    initialPerson?.alias ? [initialPerson.alias] : [],
  );
  const [motherName, setMotherName] = useState(initialPerson?.motherName ?? "");
  // Quando o usuário escolhe uma pessoa já cadastrada como mãe, guardamos o
  // ID aqui. No submit, depois de criar a pessoa, abrimos um vínculo
  // mother_of automático (mãe → filho).
  const [motherEntityId, setMotherEntityId] = useState<string>("");
  const [gender, setGender] = useState<Gender | "">("");
  const [dob, setDob] = useState(initialPerson?.dateOfBirth ?? "");
  const [cpf, setCpf] = useState("");
  const [fatherName, setFatherName] = useState("");
  const [birthplace, setBirthplace] = useState("");
  const [nationality, setNationality] = useState("");
  const [rgNumber, setRgNumber] = useState("");
  const [rgIssuer, setRgIssuer] = useState("");
  const [rgState, setRgState] = useState("");
  const [rgIssuedOn, setRgIssuedOn] = useState("");
  const [phones, setPhones] = useState<string[]>([]);
  const [sipNumber, setSipNumber] = useState("");
  const [criminalRecords, setCriminalRecords] = useState<CriminalRecord[]>([]);
  const [deceasedOn, setDeceasedOn] = useState("");
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

  // Vehicle
  const [vCategory, setVCategory] = useState<VehicleCategory>("car");
  const [plate, setPlate] = useState("");
  const [brand, setBrand] = useState("");
  const [model, setModel] = useState("");
  const [color, setColor] = useState("");
  const [year, setYear] = useState("");
  const [chassis, setChassis] = useState("");
  const [renavam, setRenavam] = useState("");

  // Estado exclusivo do fluxo "criar pessoa" — entradas auxiliares aplicadas
  // após o createEntity bem-sucedido (endereços N-N, vínculos com veículos).
  const [pendingAddresses, setPendingAddresses] = useState<AddressDraft[]>([]);
  const [pendingVehicleLinks, setPendingVehicleLinks] = useState<
    { vehicle: Entity; relation: RelationType }[]
  >([]);
  const [pendingFamilyLinks, setPendingFamilyLinks] = useState<
    { person: Entity; option: FamilyOption }[]
  >([]);

  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // PDF de consulta: aviso de CPF bloqueado (pessoa excluída ou acima do
  // clearance — o cadastro seria recusado).
  const [cpfBlocked, setCpfBlocked] = useState(false);

  // Preenche o formulário com o que o PDF trouxe. Só preenche campo VAZIO:
  // o que o usuário digitou fica, e um segundo PDF (ex.: a CONSULTA
  // INTEGRADA depois do RELATÓRIO DE CPF) complementa o primeiro em vez de
  // sobrescrevê-lo. Listas (apelidos, telefones, endereços, histórico) somam
  // sem repetir. A descrição não é tocada — é o texto do analista.
  function applyPersonPdf(r: PersonPdfResult) {
    const rep = r.report;
    const fill = (v: string, set: (f: (prev: string) => string) => void) => {
      if (v) set((prev) => (prev.trim() ? prev : v));
    };
    fill(rep.name, setName);
    fill(rep.cpf, setCpf);
    fill(rep.mother_name, setMotherName);
    fill(rep.father_name, setFatherName);
    fill(rep.birth_date, setDob);
    if (rep.gender) setGender((prev) => prev || rep.gender);
    fill(rep.birthplace, setBirthplace);
    fill(rep.nationality, setNationality);
    fill(rep.sip_number, setSipNumber);
    const rg = rep.rgs[0];
    if (rg) {
      fill(rg.number, setRgNumber);
      fill(rg.issuer, setRgIssuer);
      fill(rg.state, setRgState);
      fill(rg.issued_on, setRgIssuedOn);
    }
    if (rep.aliases.length > 0) {
      setAliases((prev) => {
        const have = new Set(prev.map((a) => a.trim().toUpperCase()));
        return [...prev, ...rep.aliases.filter((a) => !have.has(a))];
      });
    }
    if (rep.phones.length > 0) {
      setPhones((prev) => {
        const have = new Set(prev.map((p) => p.replace(/\D/g, "")));
        return [...prev, ...rep.phones.filter((p) => !have.has(p)).map(formatPhoneBR)];
      });
    }
    if (rep.addresses.length > 0) {
      setPendingAddresses((prev) => {
        const key = (a: { street: string; number: string; cep: string }) =>
          [a.street.trim().toUpperCase(), a.number.trim().toUpperCase(), a.cep.replace(/\D/g, "")].join("|");
        const kept = prev.filter((d) => !addressDraftIsEmpty(d));
        const have = new Set(kept.map(key));
        const added = rep.addresses
          .filter((a) => !have.has(key(a)))
          .map((a) => ({ ...newAddressDraft(), ...a, cep: a.cep ? formatCEP(a.cep) : "" }));
        return [...kept, ...added];
      });
    }
    if (rep.criminal.length > 0) {
      setCriminalRecords((prev) => {
        const key = (c: CriminalRecord) =>
          [c.category, c.title, c.number, c.date].map((x) => x.trim().toUpperCase()).join("|");
        const have = new Set(prev.map(key));
        return [...prev, ...rep.criminal.filter((c) => !have.has(key(c)))];
      });
    }
    setCpfBlocked(r.cpf_blocked);
  }

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
  // Diálogo de duplicata: aberto no submit (homônimos), pelo botão do aviso
  // inline ou quando o CPF digitado já é de alguém. CPF igual = mesma
  // pessoa, então nesse caso não há "criar assim mesmo".
  const { user: me } = useAuth();
  const [resolver, setResolver] = useState<{
    candidates: PersonDuplicate[];
    allowCreate: boolean;
  } | null>(null);
  const lastCpfAlertRef = useRef<string | null>(null);
  useEffect(() => {
    if (!cpfTaken) {
      lastCpfAlertRef.current = null;
      return;
    }
    if (lastCpfAlertRef.current === cpfTaken.id) return;
    lastCpfAlertRef.current = cpfTaken.id;
    setResolver({ candidates: [cpfTaken], allowCreate: false });
  }, [cpfTaken]);

  // Plano de "atualizar o cadastro existente" a partir do formulário: só
  // preenche o que está em branco no cadastro; listas somam sem repetir.
  function planFor(existing: Entity): MergePlan {
    const cur = (existing.attrs ?? {}) as PersonAttrs;
    const norm = (v: string | undefined | null) =>
      (v ?? "").normalize("NFD").replace(/[\u0300-\u036f]/g, "").toUpperCase().replace(/\s+/g, " ").trim();
    const fills: MergePlan["fills"] = [];
    const conflicts: MergePlan["conflicts"] = [];
    const adds: string[] = [];
    const person: PersonAttrs = {};
    const update: UpdateEntityInput = { version: existing.version };

    const field = (
      label: string,
      key: keyof PersonAttrs,
      incoming: string,
      current: string | undefined,
      show: (v: string) => string = (v) => v,
      eq: (a: string, b: string) => boolean = (a, b) => norm(a) === norm(b),
    ) => {
      const v = incoming.trim();
      if (!v) return;
      if (!current || !current.trim()) {
        (person as Record<string, unknown>)[key] = v;
        fills.push({ label, value: show(v) });
      } else if (!eq(v, current)) {
        conflicts.push({ label, current: show(current), incoming: show(v) });
      }
    };
    const digitsEq = (a: string, b: string) => a.replace(/\W/g, "") === b.replace(/\W/g, "");

    if (norm(name) && norm(name) !== norm(existing.name)) {
      conflicts.push({ label: "NOME", current: existing.name, incoming: name.trim().toUpperCase() });
    }
    field("NOME DA MÃE", "mother_name", motherName, cur.mother_name);
    field("NOME DO PAI", "father_name", fatherName, cur.father_name);
    field("NASCIMENTO", "date_of_birth", dob, cur.date_of_birth, formatBRDate);
    field("GÊNERO", "gender", gender, cur.gender as string | undefined, (v) => GENDER_LABEL[v as Gender] ?? v);
    field("CPF", "cpf", cpf.replace(/\D/g, ""), cur.cpf, formatCpf, digitsEq);
    field("NATURALIDADE", "birthplace", birthplace, cur.birthplace);
    field("NACIONALIDADE", "nationality", nationality, cur.nationality);
    field("RG", "rg_number", rgNumber, cur.rg_number, (v) => v, digitsEq);
    field("RG · ÓRGÃO", "rg_issuer", rgIssuer, cur.rg_issuer);
    field("RG · UF", "rg_state", rgState, cur.rg_state);
    field("RG · EMISSÃO", "rg_issued_on", rgIssuedOn, cur.rg_issued_on, formatBRDate);
    field("Nº SIP", "sip_number", sipNumber, cur.sip_number, (v) => v, digitsEq);
    if (orcrimId && !cur.orcrim_id) {
      person.orcrim_id = orcrimId;
      fills.push({ label: "ORCRIM", value: "A SELECIONADA NO FORMULÁRIO" });
    } else if (orcrimId && cur.orcrim_id !== orcrimId) {
      conflicts.push({ label: "ORCRIM", current: cur.orcrim_name ?? "OUTRA", incoming: "A DO FORMULÁRIO" });
    }
    if (description.trim()) {
      if (!existing.description?.trim()) {
        update.description = description.trim();
        fills.push({ label: "DESCRIÇÃO", value: description.trim() });
      } else if (norm(description) !== norm(existing.description)) {
        conflicts.push({ label: "DESCRIÇÃO", current: existing.description, incoming: description.trim() });
      }
    }

    // Listas: soma o que falta e envia a lista inteira (PATCH substitui).
    const curAliases = cur.aliases ?? [];
    const newAliases = aliases.filter((a) => a.trim() && !curAliases.some((c) => norm(c) === norm(a)));
    if (newAliases.length) {
      person.aliases = [...curAliases, ...newAliases];
      adds.push(`APELIDOS: ${newAliases.map((a) => a.toUpperCase()).join(", ")}`);
    }
    const curPhones = cur.phones ?? [];
    const newPhones = phones.map((p) => p.replace(/\D/g, "")).filter((p) => p && !curPhones.includes(p));
    if (newPhones.length) {
      person.phones = [...curPhones, ...newPhones];
      adds.push(`TELEFONES: ${newPhones.map(formatPhoneBR).join(", ")}`);
    }
    const recKey = (c: CriminalRecord) =>
      [c.category, c.title, c.number, c.date].map((x) => norm(x)).join("|");
    const curRecs = cur.criminal_records ?? [];
    const haveRecs = new Set(curRecs.map(recKey));
    const newRecs = criminalRecords.filter((c) => !haveRecs.has(recKey(c)));
    if (newRecs.length) {
      person.criminal_records = [...curRecs, ...newRecs];
      adds.push(`HISTÓRICO CRIMINAL: ${newRecs.length} REGISTRO(S)`);
    }
    const curTags = existing.tags ?? [];
    const newTags = tags.filter((t) => !curTags.includes(t.toLowerCase()));
    if (newTags.length) {
      update.tags = [...curTags, ...newTags];
      adds.push(`TAGS: ${newTags.map((t) => "#" + t).join(" ")}`);
    }
    const newAddresses = mergeAddresses(existing).length;
    if (newAddresses) adds.push(`ENDEREÇOS: ${newAddresses}`);
    if (photoFile) {
      if (cur.has_photo) {
        conflicts.push({ label: "FOTO PRINCIPAL", current: "A ATUAL", incoming: "A ESCOLHIDA" });
      } else {
        adds.push("FOTO PRINCIPAL");
      }
    }
    if (galleryPending.length) adds.push(`GALERIA: ${galleryPending.length} FOTO(S)`);
    if (deceasedOn && !cur.deceased) fills.push({ label: "ÓBITO", value: formatBRDate(deceasedOn) });
    const links = (motherEntityId ? 1 : 0) + pendingFamilyLinks.length + pendingVehicleLinks.length;
    if (links) adds.push(`VÍNCULOS: ${links}`);

    if (Object.keys(person).length) update.person = person;
    const patching = !!update.person || update.description !== undefined || !!update.tags;
    return {
      update,
      fills,
      adds,
      conflicts,
      newAddresses,
      empty: !patching && adds.length === 0 && !(deceasedOn && !cur.deceased),
    };
  }

  // Endereços do formulário que o cadastro ainda não tem.
  function mergeAddresses(existing: Entity | null): AddressDraft[] {
    const drafts = pendingAddresses.filter((d) => !addressDraftIsEmpty(d));
    if (!existing) return drafts;
    const key = (a: { street?: string; number?: string; cep?: string }) =>
      [(a.street ?? "").trim().toUpperCase(), (a.number ?? "").trim().toUpperCase(), (a.cep ?? "").replace(/\D/g, "")].join("|");
    const have = new Set(((existing.attrs as PersonAttrs | undefined)?.addresses ?? []).map(key));
    return drafts.filter((d) => !have.has(key(d)));
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    // Quando este wizard é renderizado via portal dentro de outro form
    // (ex.: AddLinkModal no EditMode da entidade), o evento submit borbularia
    // pela árvore React até o form ancestral e dispararia o save dele.
    e.stopPropagation();
    setErr(null);
    if (kind === null) return; // não há form para submeter na etapa 1
    const orderedTabs = tabsForKind(kind);
    const isLast = orderedTabs[orderedTabs.length - 1] === activeTab;
    // Em tabs intermediárias, o submit (botão "PRÓXIMO" ou Enter) avança;
    // só na última tab é que de fato cria a entidade.
    if (!isLast) {
      if (activeTab === "identity" && !name.trim()) {
        setErr("Nome completo é obrigatório");
        return;
      }
      const idx = orderedTabs.indexOf(activeTab);
      setActiveTab(orderedTabs[idx + 1]);
      return;
    }
    // Defesa em profundidade: name obrigatório para tudo exceto vehicle.
    if (kind !== "vehicle" && !name.trim()) {
      setActiveTab("identity");
      setErr("Nome completo é obrigatório");
      return;
    }
    const supportsPrimaryPhoto =
      kind === "person" || kind === "place" || kind === "vehicle";
    if (supportsPrimaryPhoto && photoFile && photoFile.size > 5 * 1024 * 1024) {
      setErr("A foto excede 5 MiB");
      return;
    }
    // Placa, quando informada, precisa estar em formato BR válido.
    if (kind === "vehicle" && plate.trim() && !isValidPlate(plate)) {
      setActiveTab("data");
      setErr("Placa inválida — use o padrão antigo (ABC1234) ou Mercosul (ABC1D23)");
      return;
    }
    // CPF já cadastrado ou homônimos: o analista decide no diálogo entre
    // atualizar o cadastro existente, criar assim mesmo (só homônimo) ou
    // revisar.
    if (kind === "person" && cpfTaken) {
      setResolver({ candidates: [cpfTaken], allowCreate: false });
      return;
    }
    if (kind === "person" && homonyms.length > 0) {
      setResolver({ candidates: homonyms.slice(0, 5), allowCreate: true });
      return;
    }
    await saveEntity(null);
  }

  // Grava o formulário: cria a entidade (target nulo) ou atualiza o cadastro
  // existente escolhido no diálogo de duplicata, e depois aplica os passos
  // que dependem do id (foto, óbito, endereços, vínculos, galeria).
  async function saveEntity(target: { entity: Entity; plan: MergePlan } | null) {
    if (kind === null) return;
    const supportsPrimaryPhoto =
      kind === "person" || kind === "place" || kind === "vehicle";
    const existingAttrs = (target?.entity.attrs ?? {}) as PersonAttrs;
    const verb = target ? "Cadastro atualizado" : kind === "person" ? "Pessoa criada" : "Entidade criada";
    setResolver(null);
    setBusy(true);
    try {
      let entityId: string;
      let opsLinks: OpsEntityLinks | undefined;
      if (target) {
        if (target.plan.update.person || target.plan.update.description !== undefined || target.plan.update.tags) {
          await updateEntity(target.entity.id, target.plan.update);
        }
        entityId = target.entity.id;
      } else {
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
                  father_name: fatherName.trim() || undefined,
                  birthplace: birthplace.trim() || undefined,
                  nationality: nationality.trim() || undefined,
                  rg_number: rgNumber.trim() || undefined,
                  rg_issuer: rgIssuer.trim() || undefined,
                  rg_state: rgState.trim() || undefined,
                  rg_issued_on: rgIssuedOn || undefined,
                  phones: phones.map((p) => p.replace(/\D/g, "")).filter(Boolean),
                  sip_number: sipNumber.trim() || undefined,
                  criminal_records: criminalRecords,
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
          vehicle:
            kind === "vehicle"
              ? {
                  category: vCategory,
                  plate: plate.trim() || undefined,
                  brand: brand.trim() || undefined,
                  model: model.trim() || undefined,
                  color: color.trim() || undefined,
                  year: parseIntOrUndef(year),
                  chassis: chassis.trim() || undefined,
                  renavam: renavam.trim() || undefined,
                }
              : undefined,
        });
        entityId = res.entity.id;
        opsLinks = res.ops_links;
      }

      if (supportsPrimaryPhoto && photoFile && !existingAttrs.has_photo) {
        try {
          await uploadEntityPhoto(entityId, photoFile);
        } catch (e) {
          setErr(
            verb + ", mas o upload da foto principal falhou: " +
              ((e as ApiError).message ?? "erro desconhecido"),
          );
        }
      }
      // Pessoa: óbito informado à mão. Endpoint dedicado (auditoria própria),
      // aplicado depois da criação porque depende do id gerado.
      if (kind === "person" && deceasedOn && !existingAttrs.deceased) {
        try {
          await markEntityDeath(entityId, deceasedOn);
        } catch (e) {
          setErr(
            verb + ", mas o óbito não foi registrado: " +
              ((e as ApiError).message ?? "erro desconhecido"),
          );
        }
      }

      // Pessoa: persiste endereços pendentes (linhas vazias são ignoradas).
      const addressesToSave = mergeAddresses(target?.entity ?? null);
      if (kind === "person" && addressesToSave.length > 0) {
        const failures: string[] = [];
        for (const draft of addressesToSave) {
          try {
            await createPersonAddress(entityId, addressDraftToPayload(draft));
          } catch (e) {
            failures.push(
              `${draft.label || "endereço"}: ${(e as ApiError).message ?? "erro"}`,
            );
          }
        }
        if (failures.length > 0) {
          setErr(
            verb + ", mas falharam endereços: " + failures.join("; "),
          );
        }
      }

      // Pessoa: se a mãe foi pareada com uma entidade existente, cria o
      // vínculo mother_of(mãe → nova pessoa). Falha silenciosa caso o link
      // não consiga ser criado — não bloqueia a criação da pessoa.
      if (kind === "person" && motherEntityId) {
        try {
          await createEntityLink(motherEntityId, {
            to_entity_id: entityId,
            relation_type: "mother_of",
          });
        } catch (e) {
          // No cadastro já existente o vínculo pode já estar lá.
          const msg = (e as ApiError).message ?? "";
          if (!(target && /já existe|already/i.test(msg))) {
            setErr(verb + ", mas falhou o vínculo mãe→filho: " + (msg || "erro desconhecido"));
          }
        }
      }

      // Pessoa: persiste vínculos familiares/sociais. Cada item tem uma
      // FamilyOption que decide quem é `from` no insert e qual relation_type
      // canônico usar. Conflitos de unique (link já existente, ex.: mãe já
      // declarada via MotherField) são silenciosamente ignorados.
      if (kind === "person" && pendingFamilyLinks.length > 0) {
        const failures: string[] = [];
        for (const lk of pendingFamilyLinks) {
          const fromID = lk.option.anchorAsFrom ? entityId : lk.person.id;
          const toID = lk.option.anchorAsFrom ? lk.person.id : entityId;
          try {
            await createEntityLink(fromID, {
              to_entity_id: toID,
              relation_type: lk.option.relation,
            });
          } catch (e) {
            const msg = (e as ApiError).message ?? "";
            if (!/já existe|already/i.test(msg)) {
              failures.push(`${lk.person.name}: ${msg || "erro"}`);
            }
          }
        }
        if (failures.length > 0) {
          setErr(
            verb + ", mas falharam vínculos familiares: " +
              failures.join("; "),
          );
        }
      }

      // Pessoa: persiste vínculos com veículos selecionados/criados inline.
      if (kind === "person" && pendingVehicleLinks.length > 0) {
        const failures: string[] = [];
        for (const lk of pendingVehicleLinks) {
          try {
            await createEntityLink(entityId, {
              to_entity_id: lk.vehicle.id,
              relation_type: lk.relation,
            });
          } catch (e) {
            failures.push(
              `${lk.vehicle.name}: ${(e as ApiError).message ?? "erro"}`,
            );
          }
        }
        if (failures.length > 0) {
          setErr(
            verb + ", mas falharam vínculos: " + failures.join("; "),
          );
        }
      }

      // Galeria: upload sequencial; mantemos a ordem do array em ord 0..N.
      if (galleryPending.length > 0) {
        const failures: string[] = [];
        for (let i = 0; i < galleryPending.length; i++) {
          const p = galleryPending[i];
          try {
            await uploadGalleryPhoto(entityId, p.file, p.caption.trim(), i);
          } catch (e) {
            failures.push(`#${i + 1}: ${(e as ApiError).message ?? "erro"}`);
          }
        }
        if (failures.length > 0) {
          setErr(
            verb + ", mas falharam fotos da galeria: " + failures.join("; "),
          );
        }
      }
      // Pessoa: o servidor procurou o nome no relatório operacional. O que
      // casou por nome e mãe já está vinculado — o analista é avisado; o que
      // só tem o nome igual ele decide aqui.
      const ops = opsLinks;
      if (ops && ops.linked.length > 0) {
        await modal.alert({
          variant: "info",
          title: "VINCULADO AO RELATÓRIO OPERACIONAL",
          message: (
            <>
              <div style={{ marginBottom: 8 }}>
                O nome e a mãe batem com {ops.linked.length === 1 ? "um envolvido" : `${ops.linked.length} envolvidos`} de
                ocorrência já importada. O vínculo foi criado na ocorrência e na ficha da pessoa.
              </div>
              <OpsMatchList items={ops.linked} />
            </>
          ),
        });
      }
      if (ops && ops.suggestions.length > 0) {
        const ok = await modal.confirm({
          variant: "warning",
          title: "MESMO NOME NO RELATÓRIO OPERACIONAL",
          message: (
            <>
              <div style={{ marginBottom: 8 }}>
                Só o nome bate (sem o nome da mãe para confirmar). Vincular esta pessoa a
                {ops.suggestions.length === 1 ? " esta ocorrência" : " estas ocorrências"}?
              </div>
              <OpsMatchList items={ops.suggestions} />
            </>
          ),
          confirm: "VINCULAR",
          cancel: "NÃO VINCULAR",
        });
        if (ok) {
          const failures: string[] = [];
          for (const m of ops.suggestions) {
            try {
              await linkOpsPerson(m.occurrence_id, m.person_id, entityId);
            } catch (e) {
              failures.push(`${m.ciops_record || m.name}: ${(e as ApiError).message ?? "erro"}`);
            }
          }
          if (failures.length > 0) {
            setErr(verb + ", mas falharam vínculos com o relatório: " + failures.join("; "));
          }
        }
      }
      onCreated(entityId);
    } catch (e) {
      setErr((e as ApiError).message || (target ? "Erro ao atualizar o cadastro" : "Erro ao criar entidade"));
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
          <KindPicker
            onPick={(k) => {
              setKind(k);
              setActiveTab(tabsForKind(k)[0]);
            }}
          />
        ) : (
        <>
          <div className="modal-tabs" role="tablist">
            {tabs.map((t) => (
              <button
                key={t}
                type="button"
                role="tab"
                aria-selected={activeTab === t}
                className={"modal-tab" + (activeTab === t ? " modal-tab--on" : "")}
                onClick={() => setActiveTab(t)}
              >
                {tabLabels[t]}
              </button>
            ))}
          </div>

          <form className="modal-bd entity-form" onSubmit={onSubmit} autoComplete="off">
            {(activeTab === "identity" ||
              (kind === "vehicle" && activeTab === "observations")) && (
              <>
                {kind === "person" && (
                  <>
                    <PersonPdfDrop onParsed={applyPersonPdf} disabled={busy} />
                    {cpfBlocked && (
                      <div className="banner banner-error">
                        ⚠ ESTE CPF JÁ PERTENCE A UMA PESSOA EXCLUÍDA OU FORA DO SEU NÍVEL DE
                        ACESSO — O CADASTRO VAI SER RECUSADO. PEÇA A UM GESTOR PARA VERIFICAR.
                      </div>
                    )}
                  </>
                )}
                {kind !== "vehicle" && (
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
                )}

                <label className="form-field">
                  <span>DESCRIÇÃO {kind === "vehicle" && "(OPCIONAL)"}</span>
                  <textarea
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    rows={3}
                    maxLength={2000}
                  />
                </label>

                <label className="form-field">
                  <span>TAGS {kind === "vehicle" && "(OPCIONAL)"}</span>
                  <TagInput value={tags} onChange={setTags} normalize="lower" />
                </label>
              </>
            )}

            {activeTab === "history" && kind === "person" && (
              <CriminalRecordsField
                value={criminalRecords}
                onChange={setCriminalRecords}
                disabled={busy}
              />
            )}

            {activeTab === "data" && (
              <>
                {kind === "person" && (
                  <>
                    <PersonFields
                      aliases={aliases}
                      setAliases={setAliases}
                      motherName={motherName}
                      setMotherName={setMotherName}
                      motherEntityId={motherEntityId}
                      setMotherEntityId={setMotherEntityId}
                      gender={gender}
                      setGender={setGender}
                      dob={dob}
                      setDob={setDob}
                      cpf={cpf}
                      setCpf={setCpf}
                      fatherName={fatherName}
                      setFatherName={setFatherName}
                      birthplace={birthplace}
                      setBirthplace={setBirthplace}
                      nationality={nationality}
                      setNationality={setNationality}
                      rgNumber={rgNumber}
                      setRgNumber={setRgNumber}
                      rgIssuer={rgIssuer}
                      setRgIssuer={setRgIssuer}
                      rgState={rgState}
                      setRgState={setRgState}
                      rgIssuedOn={rgIssuedOn}
                      setRgIssuedOn={setRgIssuedOn}
                      phones={phones}
                      setPhones={setPhones}
                      sipNumber={sipNumber}
                      setSipNumber={setSipNumber}
                      deceasedOn={deceasedOn}
                      setDeceasedOn={setDeceasedOn}
                      orcrimId={orcrimId}
                      setOrcrimId={setOrcrimId}
                    />
                    <AddressFields
                      items={pendingAddresses}
                      onChange={setPendingAddresses}
                    />
                    <FamilyLinkPicker
                      items={pendingFamilyLinks}
                      onChange={setPendingFamilyLinks}
                    />
                    <VehicleLinkPicker
                      items={pendingVehicleLinks}
                      onChange={setPendingVehicleLinks}
                    />
                  </>
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
                {kind === "vehicle" && (
                  <VehicleFields
                    category={vCategory}
                    setCategory={(c) => {
                      setVCategory(c);
                      // Catálogos de marca/modelo diferem por categoria —
                      // limpa os campos pra não misturar carro com moto.
                      setBrand("");
                      setModel("");
                    }}
                    plate={plate}
                    setPlate={setPlate}
                    brand={brand}
                    setBrand={setBrand}
                    model={model}
                    setModel={setModel}
                    color={color}
                    setColor={setColor}
                    year={year}
                    setYear={setYear}
                    chassis={chassis}
                    setChassis={setChassis}
                    renavam={renavam}
                    setRenavam={setRenavam}
                  />
                )}
              </>
            )}

            {activeTab === "media" && (
              <>
                {(kind === "person" || kind === "place" || kind === "vehicle") && (
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
                <button
                  type="button"
                  className="btn btn-ghost banner-action"
                  onClick={() => setResolver({ candidates: homonyms.slice(0, 5), allowCreate: true })}
                >
                  VER / ATUALIZAR CADASTRO
                </button>
              </div>
            )}
            {resolver && kind === "person" && (
              <DuplicateResolver
                title={
                  !resolver.allowCreate
                    ? "CPF JÁ CADASTRADO"
                    : topScore >= 3
                      ? "MUITO PROVÁVEL DUPLICATA"
                      : topScore === 2
                        ? "PROVÁVEL DUPLICATA"
                        : "HOMÔNIMO ENCONTRADO"
                }
                variant={!resolver.allowCreate || topScore >= 3 ? "error" : "warning"}
                intro={
                  !resolver.allowCreate
                    ? "Este CPF já pertence a uma pessoa cadastrada — não é possível criar outra. Você pode completar o cadastro existente com os dados deste formulário."
                    : "Já existe cadastro com dados coincidentes. Você pode atualizar o cadastro existente com o que estiver em branco nele, ou criar uma pessoa nova."
                }
                candidates={resolver.candidates}
                allowCreate={resolver.allowCreate}
                canUpdate={canEditEntities(me)}
                planFor={planFor}
                busy={busy}
                onReview={() => setResolver(null)}
                onCreate={() => saveEntity(null)}
                onUpdate={(entity, plan) => saveEntity({ entity, plan })}
              />
            )}
            {dupesLoading && kind === "person" && (
              <div className="muted" style={{ fontSize: 11.5 }}>
                // verificando duplicates…
              </div>
            )}

            {err && <div className="banner banner-error">⚠ {err}</div>}

            <div className="modal-ft">
              <button type="button" className="btn btn-ghost" onClick={onClose}>
                CANCELAR
              </button>
              {tabs.indexOf(activeTab) > 0 && (
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={() => {
                    const idx = tabs.indexOf(activeTab);
                    if (idx > 0) setActiveTab(tabs[idx - 1]);
                  }}
                >
                  ← VOLTAR
                </button>
              )}
              <button type="submit" className="btn btn-primary" disabled={busy}>
                {tabs[tabs.length - 1] === activeTab
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
        <button
          type="button"
          className="kind-tile"
          onClick={() => onPick("vehicle")}
        >
          <Car size={44} strokeWidth={1.4} className="kind-tile-icon" />
          <span>VEÍCULO</span>
          <span className="kind-tile-sub">PLACA · MODELO</span>
        </button>
      </div>
    </div>
  );
}

// ────────────── helpers ──────────────

function tabsForKind(k: EntityKind): ("identity" | "data" | "history" | "media" | "observations")[] {
  if (k === "vehicle") return ["data", "media", "observations"];
  if (k === "person") return ["identity", "data", "history", "media"];
  return ["identity", "data", "media"];
}

function parseFloatOrUndef(s: string): number | undefined {
  if (s.trim() === "") return undefined;
  const n = Number(s);
  return Number.isFinite(n) ? n : undefined;
}

function parseIntOrUndef(s: string): number | undefined {
  if (s.trim() === "") return undefined;
  const n = parseInt(s, 10);
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
  motherEntityId: string;
  setMotherEntityId: (v: string) => void;
  gender: Gender | "";
  setGender: (v: Gender | "") => void;
  dob: string;
  setDob: (v: string) => void;
  cpf: string;
  setCpf: (v: string) => void;
  fatherName: string;
  setFatherName: (v: string) => void;
  birthplace: string;
  setBirthplace: (v: string) => void;
  nationality: string;
  setNationality: (v: string) => void;
  rgNumber: string;
  setRgNumber: (v: string) => void;
  rgIssuer: string;
  setRgIssuer: (v: string) => void;
  rgState: string;
  setRgState: (v: string) => void;
  rgIssuedOn: string;
  setRgIssuedOn: (v: string) => void;
  phones: string[];
  setPhones: (v: string[]) => void;
  sipNumber: string;
  setSipNumber: (v: string) => void;
  deceasedOn: string;
  setDeceasedOn: (v: string) => void;
  orcrimId: string;
  setOrcrimId: (v: string) => void;
}) {
  return (
    <fieldset className="form-fieldset">
      <legend>DADOS · PESSOA</legend>

      <MotherField
        name={props.motherName}
        setName={props.setMotherName}
        linkedId={props.motherEntityId}
        setLinkedId={props.setMotherEntityId}
      />

      <label className="form-field">
        <span>NOME DO PAI</span>
        <input
          type="text"
          value={props.fatherName}
          onChange={(e) => props.setFatherName(e.target.value)}
          maxLength={200}
        />
      </label>

      <div className="form-grid-2">
        <div className="form-field">
          <span>GÊNERO</span>
          <Select
            value={props.gender}
            onChange={(v) => props.setGender(v as Gender | "")}
            options={[
              { value: "", label: "—" },
              ...GENDERS.map((g) => ({ value: g, label: GENDER_LABEL[g] })),
            ]}
          />
        </div>
        <div className="form-field">
          <span>DATA DE NASCIMENTO</span>
          <DateInput value={props.dob} onChange={props.setDob} />
        </div>
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

      <div className="form-grid-2">
        <label className="form-field">
          <span>NATURALIDADE</span>
          <input
            type="text"
            value={props.birthplace}
            onChange={(e) => props.setBirthplace(e.target.value)}
            placeholder="CIDADE/UF"
          />
        </label>
        <label className="form-field">
          <span>NACIONALIDADE</span>
          <input
            type="text"
            value={props.nationality}
            onChange={(e) => props.setNationality(e.target.value)}
          />
        </label>
        <label className="form-field">
          <span>RG</span>
          <input
            type="text"
            value={props.rgNumber}
            onChange={(e) => props.setRgNumber(e.target.value)}
          />
        </label>
        <div className="form-grid-2">
          <label className="form-field">
            <span>ÓRGÃO</span>
            <input
              type="text"
              value={props.rgIssuer}
              onChange={(e) => props.setRgIssuer(e.target.value)}
            />
          </label>
          <label className="form-field">
            <span>UF</span>
            <input
              type="text"
              value={props.rgState}
              onChange={(e) => props.setRgState(e.target.value)}
              maxLength={2}
            />
          </label>
        </div>
        <div className="form-field">
          <span>EMISSÃO DO RG</span>
          <DateInput value={props.rgIssuedOn} onChange={props.setRgIssuedOn} />
        </div>
      </div>

      <div className="form-grid-2">
        <label className="form-field">
          <span>TELEFONES</span>
          <TagInput value={props.phones} onChange={props.setPhones} />
        </label>
        <label className="form-field">
          <span>Nº SIP</span>
          <input
            type="text"
            value={props.sipNumber}
            onChange={(e) => props.setSipNumber(e.target.value)}
            inputMode="numeric"
          />
        </label>
      </div>

      <label className="form-field">
        <span>APELIDOS</span>
        <TagInput value={props.aliases} onChange={props.setAliases} />
      </label>

      {/* Óbito conhecido sem ocorrência cadastrada. Quando a pessoa é
          vinculada como VÍTIMA de um homicídio, este campo é preenchido
          automaticamente com a data do fato. */}
      <div className="form-field">
        <span>DATA DO ÓBITO (OPCIONAL)</span>
        <DateInput value={props.deceasedOn} onChange={props.setDeceasedOn} />
        <small className="muted">// PREENCHER MARCA A PESSOA COMO ÓBITO</small>
      </div>

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
    <div className="form-field">
      <span>ORCRIM · ORGANIZAÇÕES MARCADAS COM #ORCRIM</span>
      <Select
        value={value}
        onChange={onChange}
        placeholder={loading ? "CARREGANDO…" : "—"}
        options={[
          { value: "", label: loading ? "CARREGANDO…" : "—" },
          ...sorted.map(({ entity: o, label }) => ({
            value: o.id,
            label: label.toUpperCase(),
          })),
        ]}
      />
      {err && (
        <div className="muted" style={{ fontSize: 11.5 }}>
          ⚠ não foi possível carregar orcrim: {err}
        </div>
      )}
      {!loading && sorted.length === 0 && !err && (
        <div className="muted" style={{ fontSize: 11.5 }}>
          Nenhuma organização com a tag #orcrim. Cadastre uma organização e marque-a com a tag.
        </div>
      )}
    </div>
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
        <div className="form-field">
          <span>FUNDADA EM</span>
          <DateInput value={props.foundedAt} onChange={props.setFoundedAt} />
        </div>
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

// ────────────── Vehicle fields ──────────────

function VehicleFields(props: {
  category: VehicleCategory;
  setCategory: (v: VehicleCategory) => void;
  plate: string;
  setPlate: (v: string) => void;
  brand: string;
  setBrand: (v: string) => void;
  model: string;
  setModel: (v: string) => void;
  color: string;
  setColor: (v: string) => void;
  year: string;
  setYear: (v: string) => void;
  chassis: string;
  setChassis: (v: string) => void;
  renavam: string;
  setRenavam: (v: string) => void;
}) {
  return (
    <fieldset className="form-fieldset">
      <legend>DADOS · VEÍCULO</legend>
      <div className="form-grid-2">
        <div className="form-field" style={{ gridColumn: "1 / -1" }}>
          <span>TIPO</span>
          <Select
            value={props.category}
            onChange={(v) => props.setCategory(v as VehicleCategory)}
            options={VEHICLE_CATEGORIES.map((c) => ({
              value: c,
              label: VEHICLE_CATEGORY_LABEL[c],
            }))}
          />
        </div>
        <label className="form-field">
          <span>PLACA</span>
          <input
            type="text"
            value={props.plate}
            onChange={(e) => props.setPlate(normalizePlateInput(e.target.value))}
            maxLength={7}
            placeholder="ABC1D23"
          />
          {props.plate.length > 0 && !isValidPlate(props.plate) && (
            <span style={{ fontSize: 11.5, color: "var(--crit)" }}>
              ⚠ formato inválido — use o padrão antigo (ABC1234) ou Mercosul (ABC1D23)
            </span>
          )}
        </label>
        <label className="form-field">
          <span>ANO</span>
          <input
            type="text"
            inputMode="numeric"
            value={props.year}
            onChange={(e) => props.setYear(e.target.value)}
            maxLength={4}
          />
        </label>
        <label className="form-field">
          <span>MARCA</span>
          <Combobox
            value={props.brand}
            onChange={(v) => {
              props.setBrand(v);
              // Trocar a marca invalida o modelo se já não pertencer à nova
              // lista — assim o usuário não fica com "GOL" sob marca "FIAT".
              if (
                props.model &&
                !modelsForBrand(v, props.category).includes(props.model.toUpperCase())
              ) {
                props.setModel("");
              }
            }}
            options={brandsForCategory(props.category)}
            uppercase
            placeholder={props.category === "motorcycle" ? "ex.: HONDA" : "ex.: VOLKSWAGEN"}
          />
        </label>
        <label className="form-field">
          <span>MODELO</span>
          <Combobox
            value={props.model}
            onChange={props.setModel}
            options={modelsForBrand(props.brand, props.category)}
            uppercase
            placeholder={
              props.brand
                ? "selecione ou digite"
                : "selecione a marca primeiro"
            }
          />
        </label>
        <div className="form-field">
          <span>COR</span>
          <Select
            value={props.color}
            onChange={props.setColor}
            options={[
              { value: "", label: "—" },
              ...(!VEHICLE_COLORS.includes(props.color) && props.color !== ""
                ? [{ value: props.color, label: props.color.toUpperCase() }]
                : []),
              ...VEHICLE_COLORS.map((c) => ({ value: c, label: c })),
            ]}
          />
        </div>
        <label className="form-field">
          <span>RENAVAM</span>
          <input
            type="text"
            inputMode="numeric"
            value={props.renavam}
            onChange={(e) => props.setRenavam(e.target.value)}
            maxLength={11}
          />
        </label>
        <label className="form-field" style={{ gridColumn: "1 / -1" }}>
          <span>CHASSI</span>
          <input
            type="text"
            value={props.chassis}
            onChange={(e) => props.setChassis(e.target.value.toUpperCase())}
            maxLength={17}
          />
        </label>
      </div>
    </fieldset>
  );
}

// ────────────── Address fields (pessoa) ──────────────

type AddressDraft = {
  tempId: string;
  label: string;
  cep: string;
  street: string;
  number: string;
  complement: string;
  neighborhood: string;
  city: string;
  state: string;
};

function newAddressDraft(): AddressDraft {
  return {
    tempId: `addr-${Math.random().toString(36).slice(2, 9)}`,
    label: "",
    cep: "",
    street: "",
    number: "",
    complement: "",
    neighborhood: "",
    city: "",
    state: "",
  };
}

function AddressFields({
  items,
  onChange,
}: {
  items: AddressDraft[];
  onChange: (items: AddressDraft[]) => void;
}) {
  function update(i: number, patch: Partial<AddressDraft>) {
    onChange(items.map((it, idx) => (idx === i ? { ...it, ...patch } : it)));
  }
  function remove(i: number) {
    onChange(items.filter((_, idx) => idx !== i));
  }
  function add() {
    onChange([...items, newAddressDraft()]);
  }
  return (
    <fieldset className="form-fieldset">
      <legend>ENDEREÇOS · OPCIONAL</legend>
      {items.length === 0 && (
        <div className="muted" style={{ fontSize: 13 }}>
          // nenhum endereço cadastrado
        </div>
      )}
      {items.map((a, i) => (
        <AddressRow
          key={a.tempId}
          draft={a}
          onChange={(patch) => update(i, patch)}
          onRemove={() => remove(i)}
        />
      ))}
      <button
        type="button"
        className="btn btn-ghost"
        style={{ alignSelf: "flex-start", marginTop: 4 }}
        onClick={add}
      >
        <Plus size={12} strokeWidth={2} /> ADICIONAR ENDEREÇO
      </button>
    </fieldset>
  );
}

function AddressRow({
  draft,
  onChange,
  onRemove,
}: {
  draft: AddressDraft;
  onChange: (patch: Partial<AddressDraft>) => void;
  onRemove: () => void;
}) {
  const [cepLoading, setCepLoading] = useState(false);
  const [cepError, setCepError] = useState(false);
  const cepDebounce = useRef<number | null>(null);

  function handleCepInput(v: string) {
    const masked = formatCEP(v);
    onChange({ cep: masked });
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
      onChange({
        cep: masked,
        street: res.logradouro || draft.street,
        neighborhood: res.bairro || draft.neighborhood,
        city: res.localidade || draft.city,
        state: res.uf || draft.state,
      });
    }, 350);
  }

  return (
    <div className="address-row">
      <div className="address-row-hd">
        <input
          type="text"
          className="address-row-label"
          value={draft.label}
          onChange={(e) => onChange({ label: e.target.value })}
          placeholder='ex.: "casa", "casa da mãe", "trabalho"'
          maxLength={80}
        />
        <button
          type="button"
          className="address-row-remove"
          onClick={onRemove}
          title="Remover endereço"
        >
          <Trash2 size={12} strokeWidth={1.8} />
        </button>
      </div>
      <div className="form-grid-2">
        <label className="form-field">
          <span>CEP</span>
          <input
            type="text"
            inputMode="numeric"
            value={draft.cep}
            onChange={(e) => handleCepInput(e.target.value)}
            maxLength={9}
            placeholder="00000-000"
          />
          {cepLoading && (
            <span className="muted" style={{ fontSize: 11.5 }}>
              // consultando…
            </span>
          )}
          {cepError && (
            <span style={{ fontSize: 11.5, color: "var(--warn)" }}>
              ⚠ CEP não encontrado
            </span>
          )}
        </label>
        <label className="form-field">
          <span>NÚMERO</span>
          <input
            type="text"
            value={draft.number}
            onChange={(e) => onChange({ number: e.target.value })}
            maxLength={20}
          />
        </label>
        <label className="form-field" style={{ gridColumn: "1 / -1" }}>
          <span>LOGRADOURO</span>
          <input
            type="text"
            value={draft.street}
            onChange={(e) => onChange({ street: e.target.value })}
            maxLength={200}
          />
        </label>
        <label className="form-field" style={{ gridColumn: "1 / -1" }}>
          <span>COMPLEMENTO</span>
          <input
            type="text"
            value={draft.complement}
            onChange={(e) => onChange({ complement: e.target.value })}
            maxLength={200}
          />
        </label>
        <label className="form-field">
          <span>BAIRRO</span>
          <input
            type="text"
            value={draft.neighborhood}
            onChange={(e) => onChange({ neighborhood: e.target.value })}
            maxLength={120}
          />
        </label>
        <label className="form-field">
          <span>CIDADE</span>
          <input
            type="text"
            value={draft.city}
            onChange={(e) => onChange({ city: e.target.value })}
            maxLength={120}
          />
        </label>
        <label className="form-field">
          <span>UF</span>
          <input
            type="text"
            value={draft.state}
            onChange={(e) => onChange({ state: e.target.value.toUpperCase() })}
            maxLength={2}
          />
        </label>
      </div>
    </div>
  );
}

function addressDraftToPayload(d: AddressDraft): AddressPayload {
  return {
    label: d.label.trim() || undefined,
    cep: d.cep.trim() || undefined,
    street: d.street.trim() || undefined,
    number: d.number.trim() || undefined,
    complement: d.complement.trim() || undefined,
    neighborhood: d.neighborhood.trim() || undefined,
    city: d.city.trim() || undefined,
    state: d.state.trim() || undefined,
  };
}

// Decide se a linha tem qualquer dado relevante para gravar. Linhas em branco
// são silenciosamente ignoradas no submit.
function addressDraftIsEmpty(d: AddressDraft): boolean {
  return (
    !d.label.trim() &&
    !d.cep.trim() &&
    !d.street.trim() &&
    !d.number.trim() &&
    !d.complement.trim() &&
    !d.neighborhood.trim() &&
    !d.city.trim() &&
    !d.state.trim()
  );
}

// ────────────── Family link picker (pessoa) ──────────────
//
// Vínculos familiares/sociais adicionados durante o cadastro da pessoa.
// O usuário pensa em rótulos cotidianos ("FILHO(A) DESTA MÃE") via
// FAMILY_OPTIONS — o picker resolve relation_type canônico e direção
// no submit.

function FamilyLinkPicker({
  items,
  onChange,
}: {
  items: { person: Entity; option: FamilyOption }[];
  onChange: (items: { person: Entity; option: FamilyOption }[]) => void;
}) {
  const [searchOpen, setSearchOpen] = useState(false);

  function add(person: Entity, optionId: string) {
    const option = FAMILY_OPTIONS.find((o) => o.id === optionId);
    if (!option) return;
    // Mesma pessoa pode aparecer com diferentes vínculos (ex.: PAI + SÓCIO),
    // mas evitamos duplicar exatamente o mesmo (pessoa+option).
    if (items.some((it) => it.person.id === person.id && it.option.id === option.id)) return;
    onChange([...items, { person, option }]);
  }
  function remove(i: number) {
    onChange(items.filter((_, idx) => idx !== i));
  }
  function setOption(i: number, optionId: string) {
    const option = FAMILY_OPTIONS.find((o) => o.id === optionId);
    if (!option) return;
    onChange(items.map((it, idx) => (idx === i ? { ...it, option } : it)));
  }

  return (
    <fieldset className="form-fieldset">
      <legend>VÍNCULOS FAMILIARES / SOCIAIS · OPCIONAL</legend>
      {items.length === 0 && (
        <div className="muted" style={{ fontSize: 13 }}>
          // nenhum vínculo
        </div>
      )}
      {items.map((it, i) => (
        <div key={`${it.person.id}-${it.option.id}-${i}`} className="vehicle-link-row">
          <Select
            value={it.option.id}
            onChange={(v) => setOption(i, v)}
            options={FAMILY_OPTIONS.map((o) => ({
              value: o.id,
              label: o.label,
            }))}
          />
          <span className="vehicle-link-label">{it.person.name.toUpperCase()}</span>
          <button
            type="button"
            className="address-row-remove"
            onClick={() => remove(i)}
            title="Remover vínculo"
          >
            <Trash2 size={12} strokeWidth={1.8} />
          </button>
        </div>
      ))}
      <div className="vehicle-link-actions">
        <button
          type="button"
          className="btn btn-ghost"
          onClick={() => setSearchOpen(true)}
        >
          <Plus size={12} strokeWidth={2} /> ADICIONAR VÍNCULO
        </button>
      </div>

      {searchOpen && (
        <FamilyAddPopover
          onSelect={(person, optionId) => {
            add(person, optionId);
            setSearchOpen(false);
          }}
          onClose={() => setSearchOpen(false)}
          excludeIds={items.map((it) => it.person.id)}
        />
      )}
    </fieldset>
  );
}

function FamilyAddPopover({
  onSelect,
  onClose,
  excludeIds,
}: {
  onSelect: (person: Entity, optionId: string) => void;
  onClose: () => void;
  excludeIds: string[];
}) {
  const [optionId, setOptionId] = useState<string>(FAMILY_OPTIONS[0].id);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Entity[]>([]);
  const [loading, setLoading] = useState(false);

  // excludeIds chega como array novo a cada render do pai — usa Set local
  // estável só pra filtrar no render. O effect depende somente de `query`,
  // evitando re-disparo a cada keystroke fora do input (causava o tremor).
  const excludeSet = useMemo(
    () => new Set(excludeIds),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [excludeIds.join("|")],
  );

  useEffect(() => {
    const q = query.trim();
    let alive = true;
    setLoading(true);
    const handle = window.setTimeout(() => {
      listEntities({ kind: "person", search: q || undefined, limit: 12 })
        .then((r) => {
          if (!alive) return;
          setResults(r.items);
        })
        .catch(() => alive && setResults([]))
        .finally(() => alive && setLoading(false));
    }, 250);
    return () => {
      alive = false;
      window.clearTimeout(handle);
    };
  }, [query]);

  const visible = results.filter((it) => !excludeSet.has(it.id));

  return (
    <div className="modal-backdrop" onClick={onClose} style={{ zIndex: 1400 }}>
      <div
        className="modal"
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: 480 }}
      >
        <div className="modal-hd">
          <span>ADICIONAR VÍNCULO</span>
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
          <div className="form-field">
            <span>TIPO DE VÍNCULO</span>
            <Select
              value={optionId}
              onChange={setOptionId}
              options={FAMILY_OPTIONS.map((o) => ({
                value: o.id,
                label: o.label,
              }))}
            />
          </div>
          <label className="form-field">
            <span>BUSCAR PESSOA (NOME, CPF, VULGO)</span>
            <input
              type="text"
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="digite parte do nome…"
            />
          </label>
          <div className="link-search-results">
            {loading && (
              <div className="muted" style={{ fontSize: 13, padding: 8 }}>
                // buscando…
              </div>
            )}
            {!loading && visible.length === 0 && (
              <div className="muted" style={{ fontSize: 13, padding: 8 }}>
                // nenhuma pessoa encontrada
              </div>
            )}
            {!loading &&
              visible.map((p) => {
                const alias =
                  isPerson(p) && p.attrs?.aliases && p.attrs.aliases.length > 0
                    ? p.attrs.aliases[0]
                    : undefined;
                return (
                  <button
                    key={p.id}
                    type="button"
                    className="link-search-item"
                    onClick={() => onSelect(p, optionId)}
                  >
                    <span>{p.name.toUpperCase()}</span>
                    {alias && (
                      <span className="muted" style={{ fontSize: 11.5 }}>
                        VULGO {alias.toUpperCase()}
                      </span>
                    )}
                  </button>
                );
              })}
          </div>
        </div>
      </div>
    </div>
  );
}

// ────────────── Vehicle link picker (pessoa) ──────────────

function VehicleLinkPicker({
  items,
  onChange,
}: {
  items: { vehicle: Entity; relation: RelationType }[];
  onChange: (items: { vehicle: Entity; relation: RelationType }[]) => void;
}) {
  const [searchOpen, setSearchOpen] = useState(false);
  const [newVehicleOpen, setNewVehicleOpen] = useState(false);

  function add(vehicle: Entity, relation: RelationType = "owns") {
    if (items.some((it) => it.vehicle.id === vehicle.id)) return;
    onChange([...items, { vehicle, relation }]);
  }
  function remove(i: number) {
    onChange(items.filter((_, idx) => idx !== i));
  }
  function setRelation(i: number, relation: RelationType) {
    onChange(items.map((it, idx) => (idx === i ? { ...it, relation } : it)));
  }

  return (
    <fieldset className="form-fieldset">
      <legend>VÍNCULOS COM VEÍCULOS · OPCIONAL</legend>
      {items.length === 0 && (
        <div className="muted" style={{ fontSize: 13 }}>
          // nenhum veículo vinculado
        </div>
      )}
      {items.map((it, i) => (
        <div key={it.vehicle.id} className="vehicle-link-row">
          <span className="vehicle-link-label">
            {vehiclePrimaryLabel(
              it.vehicle.name,
              isVehicle(it.vehicle) ? it.vehicle.attrs?.plate : undefined,
            ).toUpperCase()}
          </span>
          <Select
            value={it.relation}
            onChange={(v) => setRelation(i, v as RelationType)}
            options={RELATION_TYPES.map((rt) => ({
              value: rt,
              label: RELATION_LABEL[rt],
            }))}
          />
          <button
            type="button"
            className="address-row-remove"
            onClick={() => remove(i)}
            title="Remover vínculo"
          >
            <Trash2 size={12} strokeWidth={1.8} />
          </button>
        </div>
      ))}
      <div className="vehicle-link-actions">
        <button
          type="button"
          className="btn btn-ghost"
          onClick={() => setSearchOpen(true)}
        >
          <Plus size={12} strokeWidth={2} /> VINCULAR EXISTENTE
        </button>
        <button
          type="button"
          className="btn btn-ghost"
          onClick={() => setNewVehicleOpen(true)}
        >
          <Plus size={12} strokeWidth={2} /> NOVO VEÍCULO
        </button>
      </div>

      {searchOpen && (
        <VehicleSearchPopover
          onSelect={(v) => {
            add(v);
            setSearchOpen(false);
          }}
          onClose={() => setSearchOpen(false)}
          excludeIds={items.map((it) => it.vehicle.id)}
        />
      )}
      {/* Renderiza o wizard empilhado via Portal pra escapar do <form> pai.
          Sem isso, o <form> interno do CreateEntidadeModal fica aninhado e
          o React/Next.js falha na hidratação ("form cannot be descendant of form"). */}
      {newVehicleOpen &&
        typeof document !== "undefined" &&
        createPortal(
          <CreateEntidadeModal
            initialKind="vehicle"
            onClose={() => setNewVehicleOpen(false)}
            onCreated={async (id) => {
              setNewVehicleOpen(false);
              try {
                const { getEntity } = await import("@/lib/entities-api");
                const r = await getEntity(id);
                add(r.entity);
              } catch {
                /* silencioso — usuário pode vincular depois pelo dossiê */
              }
            }}
          />,
          document.body,
        )}
    </fieldset>
  );
}

function VehicleSearchPopover({
  onSelect,
  onClose,
  excludeIds,
}: {
  onSelect: (v: Entity) => void;
  onClose: () => void;
  excludeIds: string[];
}) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Entity[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const q = query.trim();
    let alive = true;
    setLoading(true);
    const handle = window.setTimeout(() => {
      listEntities({ kind: "vehicle", search: q || undefined, limit: 10 })
        .then((r) => {
          if (!alive) return;
          setResults(r.items.filter((it) => !excludeIds.includes(it.id)));
        })
        .catch(() => alive && setResults([]))
        .finally(() => alive && setLoading(false));
    }, 250);
    return () => {
      alive = false;
      window.clearTimeout(handle);
    };
  }, [query, excludeIds]);

  return (
    <div className="modal-backdrop" onClick={onClose} style={{ zIndex: 1400 }}>
      <div
        className="modal"
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: 460 }}
      >
        <div className="modal-hd">
          <span>VINCULAR VEÍCULO EXISTENTE</span>
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
            <span>BUSCAR (PLACA, MARCA, MODELO)</span>
            <input
              type="text"
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="digite parte da placa ou modelo…"
            />
          </label>
          <div className="link-search-results">
            {loading && (
              <div className="muted" style={{ fontSize: 13, padding: 8 }}>
                // buscando…
              </div>
            )}
            {!loading && results.length === 0 && (
              <div className="muted" style={{ fontSize: 13, padding: 8 }}>
                // nenhum veículo encontrado
              </div>
            )}
            {!loading &&
              results.map((v) => (
                <button
                  key={v.id}
                  type="button"
                  className="link-search-item"
                  onClick={() => onSelect(v)}
                >
                  <span className="link-search-kind">VEÍCULO</span>
                  <span className="link-search-name">
                    {vehiclePrimaryLabel(
                      v.name,
                      isVehicle(v) ? v.attrs?.plate : undefined,
                    ).toUpperCase()}
                  </span>
                </button>
              ))}
          </div>
          <div className="modal-ft">
            <button type="button" className="btn btn-ghost" onClick={onClose}>
              CANCELAR
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

/** Ocorrências do relatório operacional em que a pessoa aparece. */
function OpsMatchList({ items }: { items: OpsEntityMatch[] }) {
  return (
    <ul style={{ margin: 0, paddingLeft: 18 }}>
      {items.slice(0, 8).map((m) => (
        <li key={m.person_id} style={{ fontSize: 13 }}>
          {formatBRDate(m.occurred_on)} · FICHA {m.ciops_record || "—"} · {m.role}
          {m.natures.length > 0 ? ` · ${m.natures[0].toUpperCase()}` : ""}
          {m.approx ? ` · GRAFADO "${m.name.toUpperCase()}"` : ""}
        </li>
      ))}
      {items.length > 8 && <li style={{ fontSize: 13 }}>e mais {items.length - 8}…</li>}
    </ul>
  );
}
