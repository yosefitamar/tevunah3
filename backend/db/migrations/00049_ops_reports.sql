-- +goose Up
-- +goose StatementBegin

-- ─── Relatório Operacional ────────────────────────────────────────────
-- O Relatório Diário de Ocorrências do CPRAIO (PDF publicado todo dia, com
-- as ocorrências de todos os batalhões) é importado aqui — só as do 2º
-- BPRAIO. Domínio separado de app.incidents: lá fica o cadastro curado de
-- CVLI; aqui, a cópia fiel do que a tropa registrou (natureza, equipe,
-- apreensões, procedimento, composição).

-- Cada PDF importado. O sha256 impede importar o mesmo arquivo duas vezes;
-- o arquivo original fica em PHOTO_DIR/ops-reports/<sha256>.pdf.
CREATE TABLE app.ops_reports (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  report_date          date NULL,           -- data do cabeçalho (serviço relatado)
  file_sha256          text NOT NULL UNIQUE,
  file_name            text NOT NULL DEFAULT '',
  file_size            integer NOT NULL DEFAULT 0,
  -- Contagens da importação: total do PDF, do batalhão, gravadas e já
  -- existentes (mesma ficha CIOPS importada antes).
  total_occurrences    integer NOT NULL DEFAULT 0,
  unit_occurrences     integer NOT NULL DEFAULT 0,
  imported_occurrences integer NOT NULL DEFAULT 0,
  skipped_occurrences  integer NOT NULL DEFAULT 0,
  created_at           timestamptz NOT NULL DEFAULT now(),
  created_by           uuid NOT NULL REFERENCES app.users(id)
);

CREATE INDEX ops_reports_date_idx ON app.ops_reports (report_date DESC);

-- Uma ocorrência do relatório. Texto como veio do PDF (bairro/cidade/
-- naturezas em MAIÚSCULAS para agrupar; histórico e endereço fiéis).
CREATE TABLE app.ops_occurrences (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id             uuid NOT NULL REFERENCES app.ops_reports(id),
  source_page           integer NOT NULL DEFAULT 0,

  natures               text[] NOT NULL DEFAULT '{}',
  -- Unidade: linha "Base:" do relatório e suas partes.
  base_raw              text NOT NULL DEFAULT '',
  base_city             text NOT NULL DEFAULT '',
  cia                   text NOT NULL DEFAULT '',
  pel                   text NOT NULL DEFAULT '',
  bpm                   text NOT NULL DEFAULT '',

  occurred_on           date NOT NULL,
  start_time            time NULL,
  end_time              time NULL,
  teams                 text NOT NULL DEFAULT '',
  ciops_record          text NOT NULL DEFAULT '',

  place_address         text NOT NULL DEFAULT '',
  place_neighborhood    text NOT NULL DEFAULT '',
  place_city            text NOT NULL DEFAULT '',
  approach_address      text NOT NULL DEFAULT '',
  approach_neighborhood text NOT NULL DEFAULT '',
  approach_city         text NOT NULL DEFAULT '',

  police_station        text NOT NULL DEFAULT '',
  delegate              text NOT NULL DEFAULT '',
  procedure_type        text NOT NULL DEFAULT '',
  procedure_number      text NOT NULL DEFAULT '',

  seized_objects        text NOT NULL DEFAULT '',
  narrative             text NOT NULL DEFAULT '',

  created_at            timestamptz NOT NULL DEFAULT now(),
  created_by            uuid NOT NULL REFERENCES app.users(id),
  deleted_at            timestamptz NULL,
  deleted_by            uuid NULL REFERENCES app.users(id)
);

-- A ficha CIOPS identifica a ocorrência: o relatório do dia seguinte pode
-- repeti-la, e reimportar não pode duplicar.
CREATE UNIQUE INDEX ops_occurrences_ciops_uniq
  ON app.ops_occurrences (ciops_record)
  WHERE ciops_record <> '' AND deleted_at IS NULL;
CREATE INDEX ops_occurrences_date_idx   ON app.ops_occurrences (occurred_on DESC) WHERE deleted_at IS NULL;
CREATE INDEX ops_occurrences_report_idx ON app.ops_occurrences (report_id);

-- Acusados, vítimas e testemunhas. entity_id liga ao dossiê: 'auto' quando a
-- importação casou nome + mãe com um único dossiê; 'manual' quando o analista
-- vinculou.
CREATE TABLE app.ops_occurrence_people (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  occurrence_id  uuid NOT NULL REFERENCES app.ops_occurrences(id) ON DELETE CASCADE,
  position       integer NOT NULL,
  role           text NOT NULL CHECK (role IN ('ACUSADO', 'VÍTIMA', 'TESTEMUNHA')),
  name           text NOT NULL,
  mother_name    text NOT NULL DEFAULT '',
  age            smallint NULL,
  address        text NOT NULL DEFAULT '',
  note           text NOT NULL DEFAULT '',
  entity_id      uuid NULL REFERENCES app.entities(id),
  link_mode      text NOT NULL DEFAULT '' CHECK (link_mode IN ('', 'auto', 'manual')),
  linked_at      timestamptz NULL,
  linked_by      uuid NULL REFERENCES app.users(id)
);

CREATE INDEX ops_occurrence_people_occ_idx    ON app.ops_occurrence_people (occurrence_id, position);
CREATE INDEX ops_occurrence_people_entity_idx ON app.ops_occurrence_people (entity_id) WHERE entity_id IS NOT NULL;

CREATE TABLE app.ops_occurrence_weapons (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  occurrence_id  uuid NOT NULL REFERENCES app.ops_occurrences(id) ON DELETE CASCADE,
  position       integer NOT NULL,
  kind           text NOT NULL DEFAULT '',
  model          text NOT NULL DEFAULT '',
  brand          text NOT NULL DEFAULT '',
  caliber        text NOT NULL DEFAULT '',
  serial         text NOT NULL DEFAULT ''
);
CREATE INDEX ops_occurrence_weapons_occ_idx ON app.ops_occurrence_weapons (occurrence_id, position);

CREATE TABLE app.ops_occurrence_drugs (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  occurrence_id  uuid NOT NULL REFERENCES app.ops_occurrences(id) ON DELETE CASCADE,
  position       integer NOT NULL,
  description    text NOT NULL DEFAULT '',
  grams          numeric(14,3) NULL,
  packages       integer NULL
);
CREATE INDEX ops_occurrence_drugs_occ_idx ON app.ops_occurrence_drugs (occurrence_id, position);

CREATE TABLE app.ops_occurrence_vehicles (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  occurrence_id  uuid NOT NULL REFERENCES app.ops_occurrences(id) ON DELETE CASCADE,
  position       integer NOT NULL,
  kind           text NOT NULL DEFAULT '',
  brand          text NOT NULL DEFAULT '',
  model          text NOT NULL DEFAULT '',
  plate          text NOT NULL DEFAULT '',
  color          text NOT NULL DEFAULT ''
);
CREATE INDEX ops_occurrence_vehicles_occ_idx ON app.ops_occurrence_vehicles (occurrence_id, position);

-- Composição: matrícula, posto/graduação, numeral (praças) e nome de guerra.
CREATE TABLE app.ops_occurrence_officers (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  occurrence_id  uuid NOT NULL REFERENCES app.ops_occurrences(id) ON DELETE CASCADE,
  position       integer NOT NULL,
  registration   text NOT NULL DEFAULT '',
  rank           text NOT NULL DEFAULT '',
  number         text NOT NULL DEFAULT '',
  war_name       text NOT NULL DEFAULT '',
  raw            text NOT NULL DEFAULT ''
);
CREATE INDEX ops_occurrence_officers_occ_idx ON app.ops_occurrence_officers (occurrence_id, position);

-- ─── Permissões ──────────────────────────────────────────────────────
-- Agente lê; analista/gestor/admin importam e vinculam pessoas ao dossiê.
INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('agente',        'opsreport.read',   true, false, NULL),
  ('analista',      'opsreport.read',   true, false, NULL),
  ('gestor',        'opsreport.read',   true, false, NULL),
  ('administrador', 'opsreport.read',   true, false, NULL),

  ('analista',      'opsreport.import', true, false, NULL),
  ('gestor',        'opsreport.import', true, false, NULL),
  ('administrador', 'opsreport.import', true, false, NULL),

  ('analista',      'opsreport.update', true, false, NULL),
  ('gestor',        'opsreport.update', true, false, NULL),
  ('administrador', 'opsreport.update', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions
 WHERE action IN ('opsreport.read', 'opsreport.import', 'opsreport.update');
DROP TABLE IF EXISTS app.ops_occurrence_officers;
DROP TABLE IF EXISTS app.ops_occurrence_vehicles;
DROP TABLE IF EXISTS app.ops_occurrence_drugs;
DROP TABLE IF EXISTS app.ops_occurrence_weapons;
DROP TABLE IF EXISTS app.ops_occurrence_people;
DROP TABLE IF EXISTS app.ops_occurrences;
DROP TABLE IF EXISTS app.ops_reports;
-- +goose StatementEnd
