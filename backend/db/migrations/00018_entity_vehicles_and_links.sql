-- +goose Up
-- +goose StatementBegin

-- Phase 1 do módulo Veículos + Vínculos.
--
-- 1) Adiciona 'vehicle' como tipo polimórfico de Entity. Reusa toda a
--    infraestrutura existente (fotos, galeria, tags, classification, audit,
--    soft delete, lixeira, busca).
-- 2) Cria app.entity_vehicles 1:1 com app.entities (mesma estratégia das
--    outras tabelas filhas).
-- 3) Cria app.entity_links — grafo genérico de relações entre entidades.
--    Substitui o padrão "FK em attrs" (como orcrim_id em entity_persons) para
--    qualquer relação N-N. Tem direção (from→to), tipo, janela temporal,
--    nota livre e soft delete.

ALTER TABLE app.entities DROP CONSTRAINT entities_kind_check;
ALTER TABLE app.entities ADD CONSTRAINT entities_kind_check
  CHECK (kind IN ('person', 'organization', 'place', 'vehicle'));

CREATE TABLE app.entity_vehicles (
  entity_id  uuid PRIMARY KEY REFERENCES app.entities(id),
  plate      text NULL,           -- placa normalizada (uppercase, sem hífen/espaço)
  brand      text NULL,           -- marca (ex.: VOLKSWAGEN, FORD)
  model      text NULL,           -- modelo (ex.: GOL, FOCUS)
  color      text NULL,
  year       smallint NULL CHECK (year IS NULL OR (year BETWEEN 1900 AND 2100)),
  chassis    text NULL,           -- chassi VIN (17 caracteres em padrão moderno)
  renavam    text NULL            -- BR-specific, 11 dígitos
);

-- Placa é o identificador prático do veículo no Brasil; deve ser única
-- quando preenchida. Aceita NULL pra casos em que ainda não se tem a placa
-- (observação parcial em campo). Igual ao padrão de CPF da pessoa: unique
-- index parcial (inclui soft-deletados — placa raramente é reciclada).
CREATE UNIQUE INDEX entity_vehicles_plate_uniq
  ON app.entity_vehicles (plate)
  WHERE plate IS NOT NULL;

-- ─────────────────── entity_links ───────────────────

CREATE TABLE app.entity_links (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  from_entity_id  uuid NOT NULL REFERENCES app.entities(id),
  to_entity_id    uuid NOT NULL REFERENCES app.entities(id),
  relation_type   text NOT NULL,
  valid_from      date NULL,
  valid_to        date NULL,
  note            text NULL,
  attrs           jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at      timestamptz NOT NULL DEFAULT now(),
  created_by      uuid NOT NULL REFERENCES app.users(id),
  deleted_at      timestamptz NULL,
  deleted_by      uuid NULL REFERENCES app.users(id),

  -- Não permite auto-link (entidade ligada a si mesma).
  CHECK (from_entity_id <> to_entity_id),
  -- Janela temporal coerente quando ambas datas presentes.
  CHECK (valid_from IS NULL OR valid_to IS NULL OR valid_from <= valid_to),
  -- Phase 1: tipos de relação fechados via CHECK. Migrations futuras
  -- expandem o conjunto.
  CHECK (relation_type IN ('owns', 'associated_with'))
);

-- Uma mesma tripla (origem, destino, tipo) só pode existir uma vez como
-- ativa. Pode existir múltiplas vezes no histórico via soft delete.
CREATE UNIQUE INDEX entity_links_active_uniq
  ON app.entity_links (from_entity_id, to_entity_id, relation_type)
  WHERE deleted_at IS NULL;

-- Lookups frequentes: "links saindo de X" e "links chegando em X".
CREATE INDEX entity_links_from_idx
  ON app.entity_links (from_entity_id) WHERE deleted_at IS NULL;
CREATE INDEX entity_links_to_idx
  ON app.entity_links (to_entity_id) WHERE deleted_at IS NULL;
CREATE INDEX entity_links_relation_idx
  ON app.entity_links (relation_type) WHERE deleted_at IS NULL;

-- Grants: a app conecta como tevunah_app e segue o DEFAULT PRIVILEGES da
-- migration 00002 (SELECT/INSERT/UPDATE). DELETE é negado — soft delete via
-- deleted_at é o único caminho, mesmo nos links.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.entity_links;
DROP TABLE IF EXISTS app.entity_vehicles;

ALTER TABLE app.entities DROP CONSTRAINT entities_kind_check;
ALTER TABLE app.entities ADD CONSTRAINT entities_kind_check
  CHECK (kind IN ('person', 'organization', 'place'));
-- +goose StatementEnd
