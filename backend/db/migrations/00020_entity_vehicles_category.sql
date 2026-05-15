-- +goose Up
-- +goose StatementBegin

-- Distingue carro de moto no módulo Veículos. Conjunto fechado via CHECK —
-- migrations futuras expandem (caminhão, ônibus, etc.) se houver caso de uso.
-- Default 'car': registros pré-existentes (todos cadastrados como carro até
-- aqui) ficam coerentes sem backfill manual.
ALTER TABLE app.entity_vehicles
  ADD COLUMN category text NOT NULL DEFAULT 'car'
  CHECK (category IN ('car', 'motorcycle'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.entity_vehicles DROP COLUMN category;
-- +goose StatementEnd
