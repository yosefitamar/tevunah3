-- +goose Up
-- +goose StatementBegin

-- ─── Histórico de inteligência ────────────────────────────────────────
-- Texto do analista para a aba Inteligência do SIPOM
-- (historico_inteligencia no envio). Não vem do relatório: o histórico da
-- tropa continua em narrative.
ALTER TABLE app.ops_occurrences
  ADD COLUMN intel_history text NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrences DROP COLUMN IF EXISTS intel_history;
-- +goose StatementEnd
