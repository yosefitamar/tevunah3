-- +goose Up
-- +goose StatementBegin

-- ─── Ocorrência enviada ao SIPOM ──────────────────────────────────────
-- Marca que a ocorrência do relatório operacional já foi lançada no SIPOM —
-- sai da fila de envio e fica sinalizada na listagem. Enquanto o SIPOM não
-- tem API, quem marca é o analista, depois de lançar por lá; quando houver o
-- envio automático, é ele quem grava aqui.
ALTER TABLE app.ops_occurrences
  ADD COLUMN sipom_sent_at timestamptz NULL,
  ADD COLUMN sipom_sent_by uuid NULL REFERENCES app.users(id);

-- A fila de envio é o que ainda não foi.
CREATE INDEX ops_occurrences_sipom_queue_idx
  ON app.ops_occurrences (occurred_on DESC)
  WHERE deleted_at IS NULL AND sipom_sent_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.ops_occurrences_sipom_queue_idx;
ALTER TABLE app.ops_occurrences
  DROP COLUMN IF EXISTS sipom_sent_by,
  DROP COLUMN IF EXISTS sipom_sent_at;
-- +goose StatementEnd
