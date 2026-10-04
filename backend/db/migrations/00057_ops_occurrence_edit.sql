-- +goose Up
-- +goose StatementBegin

-- ─── Edição da ocorrência importada ───────────────────────────────────
-- A ocorrência do relatório operacional nasce como cópia fiel do PDF, mas o
-- PDF erra (data, hora, ficha, bairro) e o analista precisa consertar. A
-- edição passa a ser permitida a quem tem opsreport.update, sempre com o
-- antes e o depois no trilho de auditoria (opsreport.occurrence.update). O
-- PDF original continua guardado para conferência.
--
-- updated_at/updated_by marcam que a ficha já não é a cópia do PDF.
ALTER TABLE app.ops_occurrences
  ADD COLUMN updated_at timestamptz NULL,
  ADD COLUMN updated_by uuid NULL REFERENCES app.users(id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrences
  DROP COLUMN IF EXISTS updated_by,
  DROP COLUMN IF EXISTS updated_at;
-- +goose StatementEnd
