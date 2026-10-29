-- +goose Up
-- +goose StatementBegin

-- ─── Veículo: o SIPOM pediu o cadastro ────────────────────────────────
-- O envio leva só placa e situação (o que o Tevunah já tiver de tipo, cor,
-- marca/modelo e ano vai junto): placa já cadastrada no SIPOM é só
-- vinculada. Placa nova o SIPOM recusa (422) pedindo os dados; aí o veículo
-- fica marcado e passa a exigi-los do analista antes de reenviar.
ALTER TABLE app.ops_occurrence_vehicles
  ADD COLUMN sipom_cadastro_exigido boolean NOT NULL DEFAULT false;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrence_vehicles DROP COLUMN IF EXISTS sipom_cadastro_exigido;
-- +goose StatementEnd
