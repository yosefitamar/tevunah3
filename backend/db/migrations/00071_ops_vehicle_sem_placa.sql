-- +goose Up
-- +goose StatementBegin

-- ─── Veículo sem placa confirmado ─────────────────────────────────────
-- Motocicleta ou automóvel pode ir ao SIPOM sem placa (com situação, tipo,
-- marca/modelo e cor), mas só depois que o analista confirma que a placa
-- não pôde ser identificada — o relatório costuma omiti-la ("ident.
-- suprimida") quando ela existe.
ALTER TABLE app.ops_occurrence_vehicles
  ADD COLUMN sipom_sem_placa_confirmada boolean NOT NULL DEFAULT false;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrence_vehicles DROP COLUMN IF EXISTS sipom_sem_placa_confirmada;
-- +goose StatementEnd
