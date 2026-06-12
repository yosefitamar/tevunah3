-- +goose Up
-- +goose StatementBegin

-- ─── incidents ────────────────────────────────────────────────────────
-- Ocorrência operacional alimentada diariamente pelos analistas. Tipo fixo
-- (homicídio, apreensão, prisão). MVP de tabela única: todos os tipos
-- compartilham os mesmos campos. Sem máquina de status e sem níveis de
-- sigilo nesta primeira versão — visível a quem tem incident.read.
-- Soft delete via deleted_at/deleted_by (sem hard delete, como o resto do app).
CREATE TABLE app.incidents (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  type                 text NOT NULL
                       CHECK (type IN ('homicidio', 'apreensao', 'prisao')),

  -- Quando ocorreu. Data obrigatória; hora opcional (nem sempre conhecida).
  occurred_on          date NOT NULL,
  occurred_time        time NULL,

  -- Ficha da CIOPS: número/referência do registro de origem (texto livre).
  ciops_record         text NOT NULL DEFAULT '',
  -- Houve participação da inteligência (INTEL) na ocorrência.
  intel_participation  boolean NOT NULL DEFAULT false,

  -- Foto única associada (filename canônico sob PHOTO_DIR). NULL = sem foto.
  photo_path           text NULL,

  -- Geolocalização (lat/long manuais; link externo pro Google Maps no front).
  latitude             double precision NULL,
  longitude            double precision NULL,

  description          text NOT NULL DEFAULT '',

  -- Auditoria/soft delete.
  created_at           timestamptz NOT NULL DEFAULT now(),
  created_by           uuid NOT NULL REFERENCES app.users(id),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  updated_by           uuid NULL REFERENCES app.users(id),
  deleted_at           timestamptz NULL,
  deleted_by           uuid NULL REFERENCES app.users(id)
);

CREATE INDEX incidents_type_idx       ON app.incidents (type)             WHERE deleted_at IS NULL;
CREATE INDEX incidents_occurred_idx   ON app.incidents (occurred_on DESC) WHERE deleted_at IS NULL;
CREATE INDEX incidents_created_by_idx ON app.incidents (created_by);

-- ─── incident_entities ────────────────────────────────────────────────
-- Vínculo N:N entre uma ocorrência e entidades já cadastradas (envolvidos:
-- preso, vítima, autor, etc.). `role` é texto livre — sugestões na UI.
CREATE TABLE app.incident_entities (
  incident_id  uuid NOT NULL REFERENCES app.incidents(id) ON DELETE CASCADE,
  entity_id    uuid NOT NULL REFERENCES app.entities(id),
  role         text NOT NULL DEFAULT '',
  added_at     timestamptz NOT NULL DEFAULT now(),
  added_by     uuid NOT NULL REFERENCES app.users(id),
  PRIMARY KEY (incident_id, entity_id)
);

CREATE INDEX incident_entities_entity_idx ON app.incident_entities (entity_id);

-- ─── Permissões ──────────────────────────────────────────────────────
-- Agente lê; analista/gestor/admin criam e editam; gestor/admin excluem.
INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('agente',        'incident.read',   true, false, NULL),
  ('analista',      'incident.read',   true, false, NULL),
  ('gestor',        'incident.read',   true, false, NULL),
  ('administrador', 'incident.read',   true, false, NULL),

  ('analista',      'incident.create', true, false, NULL),
  ('gestor',        'incident.create', true, false, NULL),
  ('administrador', 'incident.create', true, false, NULL),

  ('analista',      'incident.update', true, false, NULL),
  ('gestor',        'incident.update', true, false, NULL),
  ('administrador', 'incident.update', true, false, NULL),

  ('gestor',        'incident.delete', true, false, NULL),
  ('administrador', 'incident.delete', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions
 WHERE action IN (
   'incident.read','incident.create','incident.update','incident.delete'
 );
DROP TABLE IF EXISTS app.incident_entities;
DROP TABLE IF EXISTS app.incidents;
-- +goose StatementEnd
