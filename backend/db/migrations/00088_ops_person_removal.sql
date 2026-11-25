-- +goose Up
-- +goose StatementBegin

-- Remoção de envolvido da ocorrência: o que a importação leu a mais (linha
-- da tabela quebrada em duas, pessoa repetida) sai da ficha, do envio ao
-- SIPOM, das contagens e dos dossiês — mas não é apagado (o schema nega
-- DELETE). Restaurar limpa as três colunas.
ALTER TABLE app.ops_occurrence_people
  ADD COLUMN removed_at     timestamptz,
  ADD COLUMN removed_by     uuid REFERENCES app.users(id),
  ADD COLUMN removed_reason text NOT NULL DEFAULT '' CHECK (char_length(removed_reason) <= 500);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrence_people
  DROP COLUMN IF EXISTS removed_at, DROP COLUMN IF EXISTS removed_by, DROP COLUMN IF EXISTS removed_reason;
-- +goose StatementEnd
