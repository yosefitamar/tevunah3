-- +goose Up
-- +goose StatementBegin

-- Remoção de item de apreensão (arma, droga, veículo) da ocorrência: o item
-- lançado por engano, duplicado ou que não foi apreendido sai do envio ao
-- SIPOM, das pendências e das contagens — mas não é apagado (o schema nega
-- DELETE e as posições identificam os itens nas correções). Restaurar
-- limpa as três colunas.
ALTER TABLE app.ops_occurrence_weapons
  ADD COLUMN removed_at     timestamptz,
  ADD COLUMN removed_by     uuid REFERENCES app.users(id),
  ADD COLUMN removed_reason text NOT NULL DEFAULT '' CHECK (char_length(removed_reason) <= 500);
ALTER TABLE app.ops_occurrence_drugs
  ADD COLUMN removed_at     timestamptz,
  ADD COLUMN removed_by     uuid REFERENCES app.users(id),
  ADD COLUMN removed_reason text NOT NULL DEFAULT '' CHECK (char_length(removed_reason) <= 500);
ALTER TABLE app.ops_occurrence_vehicles
  ADD COLUMN removed_at     timestamptz,
  ADD COLUMN removed_by     uuid REFERENCES app.users(id),
  ADD COLUMN removed_reason text NOT NULL DEFAULT '' CHECK (char_length(removed_reason) <= 500);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrence_weapons  DROP COLUMN IF EXISTS removed_at, DROP COLUMN IF EXISTS removed_by, DROP COLUMN IF EXISTS removed_reason;
ALTER TABLE app.ops_occurrence_drugs    DROP COLUMN IF EXISTS removed_at, DROP COLUMN IF EXISTS removed_by, DROP COLUMN IF EXISTS removed_reason;
ALTER TABLE app.ops_occurrence_vehicles DROP COLUMN IF EXISTS removed_at, DROP COLUMN IF EXISTS removed_by, DROP COLUMN IF EXISTS removed_reason;
-- +goose StatementEnd
