"use client";

import { CE_MUNICIPIOS } from "@/lib/ce-municipios";
import Select from "../shared/Select";
import Combobox from "../shared/Combobox";

type Props = {
  city: string;
  neighborhood: string;
  onChangeCity: (v: string) => void;
  onChangeNeighborhood: (v: string) => void;
  /** Bairros já cadastrados no município selecionado (sugestões). */
  neighborhoodOptions: string[];
  /** Chamado ao sair do campo de bairro — usado pelo drawer, que persiste no blur. */
  onNeighborhoodBlur?: (v: string) => void;
  disabled?: boolean;
};

/**
 * Recorte territorial da ocorrência: município (lista fechada dos 184 do
 * Ceará) + bairro (texto livre com autocompletar sobre o que já existe no
 * acervo daquele município).
 *
 * O município é fechado porque é chave de agregação no mapa do crime —
 * "FORTALEZA" e "Fortaleza-CE" viram dois territórios na estatística. O
 * bairro não tem cadastro oficial utilizável, então fica livre, mas as
 * sugestões puxam para as grafias já usadas.
 */
export default function PlaceField({
  city,
  neighborhood,
  onChangeCity,
  onChangeNeighborhood,
  neighborhoodOptions,
  onNeighborhoodBlur,
  disabled,
}: Props) {
  return (
    <div className="form-grid-2">
      <div className="form-field">
        <span>MUNICÍPIO</span>
        <Select
          value={city}
          disabled={disabled}
          onChange={onChangeCity}
          placeholder="NÃO INFORMADO"
          options={[
            { value: "", label: "NÃO INFORMADO" },
            ...CE_MUNICIPIOS.map((m) => ({ value: m, label: m })),
          ]}
        />
      </div>
      <div className="form-field">
        <span>BAIRRO</span>
        <Combobox
          value={neighborhood}
          onChange={onChangeNeighborhood}
          onBlur={onNeighborhoodBlur}
          options={neighborhoodOptions}
          uppercase
          disabled={disabled}
          placeholder={city ? "bairro…" : "informe o município primeiro"}
        />
      </div>
    </div>
  );
}
