-- +goose Up
-- +goose StatementBegin

-- ─── Óbito da pessoa ──────────────────────────────────────────────────
-- Marcado quando a pessoa é vinculada como VÍTIMA de um homicídio (com
-- confirmação explícita do agente, pelo risco de homônimo).
--
-- death_incident_id aponta para a ocorrência que originou a marcação, mas é
-- ANULÁVEL sem desfazer o óbito: desvincular a vítima da ocorrência devolve
-- o registro ao estado "vítima não identificada" sem ressuscitar ninguém —
-- a morte é fato independente do vínculo. ON DELETE SET NULL cobre o mesmo
-- caso vindo do banco.
ALTER TABLE app.entity_persons
  ADD COLUMN deceased          boolean NOT NULL DEFAULT false,
  ADD COLUMN deceased_on       date    NULL,
  ADD COLUMN death_incident_id uuid    NULL
    REFERENCES app.incidents(id) ON DELETE SET NULL;

-- Coerência: sem óbito não há data nem ocorrência de morte.
ALTER TABLE app.entity_persons
  ADD CONSTRAINT entity_persons_death_check
  CHECK (deceased OR (deceased_on IS NULL AND death_incident_id IS NULL));

CREATE INDEX entity_persons_deceased_idx
  ON app.entity_persons (deceased) WHERE deceased;
CREATE INDEX entity_persons_death_incident_idx
  ON app.entity_persons (death_incident_id) WHERE death_incident_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.entity_persons_death_incident_idx;
DROP INDEX IF EXISTS app.entity_persons_deceased_idx;
ALTER TABLE app.entity_persons DROP CONSTRAINT IF EXISTS entity_persons_death_check;
ALTER TABLE app.entity_persons DROP COLUMN IF EXISTS death_incident_id;
ALTER TABLE app.entity_persons DROP COLUMN IF EXISTS deceased_on;
ALTER TABLE app.entity_persons DROP COLUMN IF EXISTS deceased;
-- +goose StatementEnd
