-- +goose Up
-- +goose StatementBegin

-- Campos da pessoa que o RELATÓRIO DE CPF (consulta externa importada em PDF
-- na qualificação civil) traz e o cadastro ainda não tinha. Todos opcionais:
-- o cadastro manual continua funcionando sem eles.
--
--   father_name   nome do pai (mesma normalização do mother_name: MAIÚSCULAS)
--   birthplace    naturalidade, como vem no relatório ("FORTALEZA/CE")
--   nationality   nacionalidade ("BRASILEIRO")
--   rg_number     número do RG, só o identificador (sem órgão/UF)
--   rg_issuer     órgão emissor ("SSP")
--   rg_state      UF emissora (2 letras)
--   rg_issued_on  data de emissão do RG
--   phones        telefones só com dígitos, DDD incluído ("85989577505")
ALTER TABLE app.entity_persons
  ADD COLUMN father_name  text   NULL,
  ADD COLUMN birthplace   text   NULL,
  ADD COLUMN nationality  text   NULL,
  ADD COLUMN rg_number    text   NULL,
  ADD COLUMN rg_issuer    text   NULL,
  ADD COLUMN rg_state     text   NULL,
  ADD COLUMN rg_issued_on date   NULL,
  ADD COLUMN phones       text[] NOT NULL DEFAULT '{}';

CREATE INDEX entity_persons_rg_idx
  ON app.entity_persons (rg_number) WHERE rg_number IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.entity_persons_rg_idx;
ALTER TABLE app.entity_persons
  DROP COLUMN phones,
  DROP COLUMN rg_issued_on,
  DROP COLUMN rg_state,
  DROP COLUMN rg_issuer,
  DROP COLUMN rg_number,
  DROP COLUMN nationality,
  DROP COLUMN birthplace,
  DROP COLUMN father_name;
-- +goose StatementEnd
