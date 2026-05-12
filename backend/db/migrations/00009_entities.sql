-- +goose Up
-- +goose StatementBegin

-- Tabela base de Entidades. Modelagem polimórfica: a base carrega campos
-- comuns e cada tipo (person/organization/place) tem uma tabela filha 1:1
-- via PK = FK. Não há hard delete — soft delete via deleted_at/deleted_by.
-- Sem compartimento ABAC: controle de acesso fica na combinação
-- permission(entity.*) + classification ≤ clearance_level do usuário.
CREATE TABLE app.entities (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind            text NOT NULL CHECK (kind IN ('person','organization','place')),
  name            text NOT NULL,
  description     text NULL,
  classification  smallint NOT NULL DEFAULT 1
                    CHECK (classification BETWEEN 1 AND 4),
  -- 1=public, 2=restricted, 3=confidential, 4=secret. Alinhado com
  -- app.users.clearance_level (1..5): leitura requer clearance >= classification.
  version         integer NOT NULL DEFAULT 1,
  created_at      timestamptz NOT NULL DEFAULT now(),
  created_by      uuid NOT NULL REFERENCES app.users(id),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  updated_by      uuid NOT NULL REFERENCES app.users(id),
  deleted_at      timestamptz NULL,
  deleted_by      uuid NULL REFERENCES app.users(id)
);

CREATE INDEX entities_kind_idx
  ON app.entities (kind) WHERE deleted_at IS NULL;
CREATE INDEX entities_classification_idx
  ON app.entities (classification) WHERE deleted_at IS NULL;
CREATE INDEX entities_name_lower_idx
  ON app.entities (lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX entities_created_at_idx
  ON app.entities (created_at DESC) WHERE deleted_at IS NULL;

-- Tabelas filhas polimórficas. PK = FK garante 1:1 e impede um id de
-- existir em duas filhas. ON DELETE não se aplica (não há hard delete na base).
CREATE TABLE app.entity_persons (
  entity_id      uuid PRIMARY KEY REFERENCES app.entities(id),
  aliases        text[] NOT NULL DEFAULT '{}',
  gender         text NULL,
  date_of_birth  date NULL,
  nationality    text NULL,
  doc_type       text NULL,
  doc_number     text NULL,
  photo_url      text NULL
);

CREATE TABLE app.entity_organizations (
  entity_id   uuid PRIMARY KEY REFERENCES app.entities(id),
  legal_name  text NULL,
  tax_id      text NULL,
  founded_at  date NULL,
  hq          text NULL
);

CREATE TABLE app.entity_places (
  entity_id   uuid PRIMARY KEY REFERENCES app.entities(id),
  address     text NULL,
  country     text NULL,
  region      text NULL,
  latitude    double precision NULL,
  longitude   double precision NULL
);

-- Tags livres. Tag normalizada em minúsculas pelo app (não há constraint
-- aqui para permitir display case em apresentação se decidirmos guardar).
CREATE TABLE app.entity_tags (
  entity_id  uuid NOT NULL REFERENCES app.entities(id),
  tag        text NOT NULL,
  added_at   timestamptz NOT NULL DEFAULT now(),
  added_by   uuid NOT NULL REFERENCES app.users(id),
  PRIMARY KEY (entity_id, tag)
);

CREATE INDEX entity_tags_tag_idx ON app.entity_tags (tag);

-- Grants já estão cobertos pelo ALTER DEFAULT PRIVILEGES da migration 00002
-- (SELECT/INSERT/UPDATE para tevunah_app). DELETE permanece negado — soft
-- delete via deleted_at é o único caminho.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.entity_tags;
DROP TABLE IF EXISTS app.entity_places;
DROP TABLE IF EXISTS app.entity_organizations;
DROP TABLE IF EXISTS app.entity_persons;
DROP TABLE IF EXISTS app.entities;
-- +goose StatementEnd
