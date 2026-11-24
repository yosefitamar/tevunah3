-- +goose Up
-- +goose StatementBegin

-- Recomendação do SIPOM: veículo sem placa (campo obrigatório lá) não vai
-- como material "Veículo" — vai como "Outros", com a descrição do que se
-- sabe dele e quantidade 1. Vale para todo veículo sem placa: ciclomotor
-- direto, carro e moto depois que o analista confirma que a placa não pôde
-- ser identificada. Vazio = descrição montada do relatório (tipo, marca,
-- modelo, cor).
ALTER TABLE app.ops_occurrence_vehicles
  ADD COLUMN sipom_outros_descricao text NOT NULL DEFAULT ''
    CHECK (char_length(sipom_outros_descricao) <= 500);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrence_vehicles DROP COLUMN IF EXISTS sipom_outros_descricao;
-- +goose StatementEnd
