-- +goose Up
-- +goose StatementBegin

-- Endereços associados a uma pessoa. Modelagem 1-N: cada pessoa pode ter
-- vários endereços com um rótulo livre ("casa", "casa da mãe", "trabalho").
-- Soft-delete consistente com o resto do app. Sem FK para um catálogo de
-- lugares (Place) — endereços de pessoas são entrada inline e não exigem
-- coordenação cruzada nesta fase. Migração futura pra grafo (resides_at via
-- entity_links + Place) permanece viável.

CREATE TABLE app.person_addresses (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  person_id    uuid NOT NULL REFERENCES app.entity_persons(entity_id),
  label        text NULL,          -- ex.: "casa", "casa da mãe", "trabalho"
  cep          text NULL,          -- 8 dígitos sem hífen
  street       text NULL,          -- logradouro (rua, avenida, etc.)
  number       text NULL,          -- número (string: aceita "S/N", "12-A")
  complement   text NULL,
  neighborhood text NULL,          -- bairro
  city         text NULL,
  state        text NULL,          -- UF (2 letras) ou nome
  created_at   timestamptz NOT NULL DEFAULT now(),
  created_by   uuid NOT NULL REFERENCES app.users(id),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  updated_by   uuid NOT NULL REFERENCES app.users(id),
  deleted_at   timestamptz NULL,
  deleted_by   uuid NULL REFERENCES app.users(id)
);

CREATE INDEX person_addresses_person_idx
  ON app.person_addresses (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_addresses_cep_idx
  ON app.person_addresses (cep) WHERE cep IS NOT NULL AND deleted_at IS NULL;

-- DELETE permanece negado pela política do schema (ALTER DEFAULT PRIVILEGES
-- em 00002). Toda exclusão é soft via deleted_at.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.person_addresses;
-- +goose StatementEnd
