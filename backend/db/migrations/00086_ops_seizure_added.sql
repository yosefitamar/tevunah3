-- +goose Up
-- +goose StatementBegin

-- Item de apreensão incluído pelo analista (o relatório não trouxe, ou trouxe
-- errado e o item foi removido): quem e quando. NULL = veio do relatório.
ALTER TABLE app.ops_occurrence_weapons
  ADD COLUMN added_by uuid REFERENCES app.users(id),
  ADD COLUMN added_at timestamptz;
ALTER TABLE app.ops_occurrence_drugs
  ADD COLUMN added_by uuid REFERENCES app.users(id),
  ADD COLUMN added_at timestamptz;
ALTER TABLE app.ops_occurrence_vehicles
  ADD COLUMN added_by uuid REFERENCES app.users(id),
  ADD COLUMN added_at timestamptz;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrence_weapons  DROP COLUMN IF EXISTS added_by, DROP COLUMN IF EXISTS added_at;
ALTER TABLE app.ops_occurrence_drugs    DROP COLUMN IF EXISTS added_by, DROP COLUMN IF EXISTS added_at;
ALTER TABLE app.ops_occurrence_vehicles DROP COLUMN IF EXISTS added_by, DROP COLUMN IF EXISTS added_at;
-- +goose StatementEnd
