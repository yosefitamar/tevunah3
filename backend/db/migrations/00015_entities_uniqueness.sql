-- +goose Up
-- +goose StatementBegin

-- Uniqueness por kind:
--
--  * Organizações: name único (case-insensitive) entre as não-deletadas.
--    Não usa o kind=person porque pessoas têm homônimos legítimos.
--
--  * Pessoas: CPF único quando preenchido. Cobre toda a tabela (inclusive
--    pessoas soft-deletadas) — liberação de CPF reusado fica como decisão
--    operacional manual; o índice trata o caso comum (CPF é identificador
--    nacional, dificilmente reciclado).

CREATE UNIQUE INDEX entities_organization_name_uniq
  ON app.entities (lower(name))
  WHERE kind = 'organization' AND deleted_at IS NULL;

CREATE UNIQUE INDEX entity_persons_cpf_uniq
  ON app.entity_persons (cpf)
  WHERE cpf IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.entity_persons_cpf_uniq;
DROP INDEX IF EXISTS app.entities_organization_name_uniq;
-- +goose StatementEnd
